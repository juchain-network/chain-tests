package testkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	testctx "juchain.org/chain/tools/ci/internal/context"
)

type HeadSample struct {
	Name     string
	URL      string
	Height   uint64
	Hash     common.Hash
	Coinbase common.Address
	Error    string
}

type HeadHashConflict struct {
	Height       uint64
	Hashes       []common.Hash
	Participants []string
}

type ValidatorLogEvidence struct {
	Validator               common.Address
	LogFile                 string
	CompetingBlockLines     []string
	SignedRecentlyWaitLines []string
}

type TimedHeadConflictSample struct {
	ObservedAt time.Time
	Samples    []HeadSample
	Conflicts  []HeadHashConflict
}

type ConsensusConflictEvidence struct {
	TargetHeight            uint64
	BeforeSamples           []HeightSample
	AfterSamples            []HeightSample
	TriggerHeadSamples      []HeadSample
	TriggerConflicts        []HeadHashConflict
	TriggerWindowSamples    []TimedHeadConflictSample
	PostStopWindowSamples   []TimedHeadConflictSample
	TriggerSurvivorHeads    []HeadSample
	PostStopSurvivorHeads   []HeadSample
	RecentCoinbases         []common.Address
	SameHeightMultiHash     bool
	PostStopMultiHash       bool
	SurvivorDistinctHeads   bool
	TargetIsNextInTurn      bool
	PostStopSameHeightSplit bool
	PostStopClassification  string
	SurvivorLogs            []ValidatorLogEvidence
	CompetingBlockObserved  bool
	SignedRecentlyObserved  bool
}

func observeRPCHead(name, rpcURL string) HeadSample {
	sample := HeadSample{
		Name: strings.TrimSpace(name),
		URL:  strings.TrimSpace(rpcURL),
	}
	if sample.Name == "" {
		sample.Name = sample.URL
	}
	if sample.URL == "" {
		sample.Error = "empty rpc"
		return sample
	}

	client, err := ethclient.Dial(sample.URL)
	if err != nil {
		sample.Error = err.Error()
		return sample
	}
	defer client.Close()

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		sample.Error = err.Error()
		return sample
	}
	if header == nil {
		sample.Error = "nil header"
		return sample
	}
	sample.Height = header.Number.Uint64()
	sample.Hash = header.Hash()
	sample.Coinbase = header.Coinbase
	return sample
}

func ObserveHeads(c *testctx.CIContext) []HeadSample {
	if c == nil {
		return nil
	}

	seen := make(map[string]bool)
	samples := make([]HeadSample, 0, 1)
	appendSample := func(name, rpcURL string) {
		rpcURL = strings.TrimSpace(rpcURL)
		if rpcURL == "" || seen[strings.ToLower(rpcURL)] {
			return
		}
		seen[strings.ToLower(rpcURL)] = true
		samples = append(samples, observeRPCHead(name, rpcURL))
	}

	if c.Config != nil {
		for idx, rpcURL := range c.Config.RPCs {
			appendSample(fmt.Sprintf("rpc[%d]", idx), rpcURL)
		}
		for idx, rpcURL := range c.Config.ValidatorRPCs {
			appendSample(fmt.Sprintf("validator[%d]", idx), rpcURL)
		}
		if syncRPC := strings.TrimSpace(c.Config.SyncRPC); syncRPC != "" {
			appendSample("sync", syncRPC)
		}
		for idx, nodeRPC := range c.Config.NodeRPCs {
			name := strings.TrimSpace(nodeRPC.Name)
			if name == "" {
				name = fmt.Sprintf("node_rpc[%d]", idx)
			}
			appendSample(name, nodeRPC.URL)
		}
	}

	return samples
}

func SameHeightHashConflicts(samples []HeadSample) []HeadHashConflict {
	type bucket struct {
		hashes       map[common.Hash]struct{}
		participants []string
	}

	byHeight := make(map[uint64]*bucket)
	for _, sample := range samples {
		if sample.Error != "" || sample.Height == 0 {
			continue
		}
		entry := byHeight[sample.Height]
		if entry == nil {
			entry = &bucket{hashes: make(map[common.Hash]struct{})}
			byHeight[sample.Height] = entry
		}
		entry.hashes[sample.Hash] = struct{}{}
		label := sample.Name
		if label == "" {
			label = sample.URL
		}
		entry.participants = append(entry.participants, label)
	}

	out := make([]HeadHashConflict, 0)
	for height, entry := range byHeight {
		if len(entry.hashes) <= 1 {
			continue
		}
		conflict := HeadHashConflict{
			Height:       height,
			Participants: append([]string(nil), entry.participants...),
		}
		for hash := range entry.hashes {
			conflict.Hashes = append(conflict.Hashes, hash)
		}
		sort.Slice(conflict.Hashes, func(i, j int) bool {
			return strings.ToLower(conflict.Hashes[i].Hex()) < strings.ToLower(conflict.Hashes[j].Hex())
		})
		sort.Strings(conflict.Participants)
		out = append(out, conflict)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Height < out[j].Height
	})
	return out
}

func ObserveHeadConflictSamples(c *testctx.CIContext, probes int, poll time.Duration) []TimedHeadConflictSample {
	if c == nil || probes <= 0 {
		return nil
	}
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}
	out := make([]TimedHeadConflictSample, 0, probes)
	for idx := 0; idx < probes; idx++ {
		samples := ObserveHeads(c)
		out = append(out, TimedHeadConflictSample{
			ObservedAt: time.Now(),
			Samples:    append([]HeadSample(nil), samples...),
			Conflicts:  SameHeightHashConflicts(samples),
		})
		if idx+1 < probes {
			time.Sleep(poll)
		}
	}
	return out
}

func SelectInTurnSignerAtHeight(signers []common.Address, height uint64) common.Address {
	if len(signers) == 0 {
		return common.Address{}
	}
	return signers[int(height%uint64(len(signers)))]
}

func ChooseCompetingWindowHeight(current, epoch uint64) uint64 {
	if epoch > 0 {
		nextCheckpoint := ((current / epoch) + 1) * epoch
		if nextCheckpoint > current && nextCheckpoint-current <= 6 && nextCheckpoint >= 1 {
			return nextCheckpoint - 1
		}
	}
	return current + 4
}

func CollectConsensusConflictEvidence(
	c *testctx.CIContext,
	survivors []common.Address,
	targetHeight uint64,
	beforeSamples, afterSamples []HeightSample,
	maxLines int,
) ConsensusConflictEvidence {
	if maxLines <= 0 {
		maxLines = 6
	}

	out := ConsensusConflictEvidence{
		TargetHeight:    targetHeight,
		BeforeSamples:   append([]HeightSample(nil), beforeSamples...),
		AfterSamples:    append([]HeightSample(nil), afterSamples...),
		RecentCoinbases: RecentCoinbases(c, 12),
	}

	for _, survivor := range survivors {
		item := ValidatorLogEvidence{
			Validator: survivor,
		}
		logFile := validatorCombinedLogPath(c, survivor)
		item.LogFile = logFile
		if logFile != "" {
			competing := tailMatchingLogLines(logFile, []string{"competing block"}, maxLines)
			recently := tailMatchingLogLines(logFile, []string{"signed recently", "must wait for others"}, maxLines)
			item.CompetingBlockLines = competing
			item.SignedRecentlyWaitLines = recently
			if len(competing) > 0 {
				out.CompetingBlockObserved = true
			}
			if len(recently) > 0 {
				out.SignedRecentlyObserved = true
			}
		}
		out.SurvivorLogs = append(out.SurvivorLogs, item)
	}

	return out
}

func validatorCombinedLogPath(c *testctx.CIContext, validator common.Address) string {
	if c == nil || c.Config == nil || strings.TrimSpace(c.Config.SourcePath) == "" {
		return ""
	}
	repoRoot := filepath.Dir(filepath.Dir(c.Config.SourcePath))
	logDir := filepath.Join(repoRoot, "data", "native-logs")

	for idx, item := range c.Config.Validators {
		if strings.EqualFold(strings.TrimSpace(item.Address), validator.Hex()) {
			path := filepath.Join(logDir, fmt.Sprintf("validator%d-combined.log", idx+1))
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func tailMatchingLogLines(path string, patterns []string, maxLines int) []string {
	if path == "" || maxLines <= 0 {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	matched := make([]string, 0, maxLines)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		ok := true
		for _, pattern := range patterns {
			if !strings.Contains(lower, strings.ToLower(pattern)) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		matched = append(matched, line)
		if len(matched) >= maxLines {
			break
		}
	}
	for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
		matched[i], matched[j] = matched[j], matched[i]
	}
	return matched
}
