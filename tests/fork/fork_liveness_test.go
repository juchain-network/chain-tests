package tests

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"juchain.org/chain/tools/ci/internal/config"
	"juchain.org/chain/tools/ci/internal/testkit"
)

const (
	forkTxInterval      = 400 * time.Millisecond
	noForkObserveWindow = 45 * time.Second
	forkLeadWindow      = 5 * time.Second
	minPreForkWindow    = 12 * time.Second
	forkPostWindow      = 30 * time.Second
	maxStallWindow      = 15 * time.Second
	minTxSent           = 8
	minGrowthPerNode    = uint64(3)
	maxHeightLag        = uint64(8)
)

type forkNode struct {
	name   string
	rpcURL string
	client *ethclient.Client
}

type trafficStats struct {
	sent   int64
	failed int64
	last   atomic.Value
}

func (s *trafficStats) setErr(err error) {
	if err == nil {
		return
	}
	s.last.Store(err.Error())
}

func (s *trafficStats) lastErr() string {
	if v := s.last.Load(); v != nil {
		if msg, ok := v.(string); ok {
			return msg
		}
	}
	return ""
}

func forkScheduleSummary() string {
	return fmt.Sprintf(
		"target=%s scheduled=%d schedule[shanghai=%d cancun=%d fixHeader=%d posa=%d]",
		cfg.Fork.Target,
		cfg.Fork.ScheduledTime,
		cfg.Fork.Schedule.ShanghaiTime,
		cfg.Fork.Schedule.CancunTime,
		cfg.Fork.Schedule.FixHeaderTime,
		cfg.Fork.Schedule.PosaTime,
	)
}

type forkPhase struct {
	name string
	at   int64
}

func forkPhases() []forkPhase {
	if cfg == nil {
		return nil
	}
	schedule := cfg.Fork.Schedule
	phases := []forkPhase{
		{name: "shanghai", at: schedule.ShanghaiTime},
		{name: "cancun", at: schedule.CancunTime},
		{name: "fixHeader", at: schedule.FixHeaderTime},
		{name: "posa", at: schedule.PosaTime},
	}
	out := make([]forkPhase, 0, len(phases))
	for _, phase := range phases {
		if phase.at > 0 {
			out = append(out, phase)
		}
	}
	return out
}

func validateForkSchedule() error {
	if cfg == nil || !strings.EqualFold(cfg.Fork.Mode, "upgrade") {
		return nil
	}
	phases := forkPhases()
	if len(phases) == 0 {
		return fmt.Errorf("missing upgrade fork schedule")
	}
	for i := 1; i < len(phases); i++ {
		if phases[i-1].at > phases[i].at {
			return fmt.Errorf("invalid fork ordering: %s(%d) > %s(%d)", phases[i-1].name, phases[i-1].at, phases[i].name, phases[i].at)
		}
	}
	if strings.EqualFold(cfg.Fork.Target, "allStaggered") {
		for i := 1; i < len(phases); i++ {
			if phases[i-1].at >= phases[i].at {
				return fmt.Errorf("allStaggered requires strict fork ordering: %s(%d) >= %s(%d)", phases[i-1].name, phases[i-1].at, phases[i].name, phases[i].at)
			}
		}
	}
	last := phases[len(phases)-1].at
	if cfg.Fork.ScheduledTime != last {
		return fmt.Errorf("scheduled_time mismatch: have=%d want=%d", cfg.Fork.ScheduledTime, last)
	}
	return nil
}

func forkPhaseAt(ts uint64) string {
	phases := forkPhases()
	if len(phases) == 0 {
		return "no-fork"
	}
	for i, phase := range phases {
		if int64(ts) < phase.at {
			if i == 0 {
				return fmt.Sprintf("before %s", phase.name)
			}
			return fmt.Sprintf("between %s and %s", phases[i-1].name, phase.name)
		}
	}
	return fmt.Sprintf("after %s", phases[len(phases)-1].name)
}

func forkLeadPhaseLabel() string {
	if cfg == nil || cfg.Fork.ScheduledTime <= 0 {
		return "fork boundary"
	}
	targetTs := uint64(cfg.Fork.ScheduledTime)
	if targetTs >= uint64(forkLeadWindow.Seconds()) {
		targetTs -= uint64(forkLeadWindow.Seconds())
	} else {
		targetTs = 0
	}
	return forkPhaseAt(targetTs)
}

func TestF_ForkLiveness(t *testing.T) {
	if cfg == nil || funderKey == nil {
		t.Fatalf("fork test context not initialized")
	}
	if err := validateForkSchedule(); err != nil {
		t.Fatalf("invalid fork schedule: %v (%s)", err, forkScheduleSummary())
	}

	nodes, err := openForkNodes(cfg)
	if err != nil {
		t.Fatalf("open nodes: %v", err)
	}
	defer closeForkNodes(nodes)

	if len(nodes) == 0 {
		t.Fatalf("no RPC nodes resolved")
	}

	primary := nodes[0].client
	chainID, err := primary.ChainID(context.Background())
	if err != nil {
		t.Fatalf("query chain id: %v", err)
	}
	t.Logf("fork schedule: %s", forkScheduleSummary())

	startHeights, err := collectHeights(nodes)
	if err != nil {
		failForkTest(t, nodes, "collect start heights: %v", err)
	}
	maxHeights := cloneHeights(startHeights)

	sendCtx, cancelSend := context.WithCancel(context.Background())
	defer cancelSend()
	stats := &trafficStats{}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runTraffic(sendCtx, stats, primary, chainID, funderKey)
	}()

	head, err := latestHead(primary)
	if err != nil {
		cancelSend()
		wg.Wait()
		failForkTest(t, nodes, "read initial head: %v", err)
	}
	lastAdvance := time.Now()
	lastHeadNumber := head.number

	if cfg.Fork.ScheduledTime > 0 {
		if head.timestamp >= uint64(cfg.Fork.ScheduledTime) {
			cancelSend()
			wg.Wait()
			failForkTest(t, nodes, "fork already passed before test start: headTime=%d scheduled=%d mode=%s target=%s", head.timestamp, cfg.Fork.ScheduledTime, cfg.Fork.Mode, cfg.Fork.Target)
		}
		preStart := head.number
		preStartTime := head.timestamp
		preDeadline := time.Now().Add(time.Duration(cfg.Fork.DelaySeconds+60) * time.Second)
		if cfg.Fork.DelaySeconds <= 0 {
			preDeadline = time.Now().Add(90 * time.Second)
		}

		for {
			if time.Now().After(preDeadline) {
				cancelSend()
				wg.Wait()
				failForkTest(t, nodes, "timeout waiting for fork boundary: %s current=%d", forkScheduleSummary(), head.timestamp)
			}
			time.Sleep(1 * time.Second)
			head, err = latestHead(primary)
			if err != nil {
				continue
			}
			if head.number > lastHeadNumber {
				lastHeadNumber = head.number
				lastAdvance = time.Now()
			}
			if time.Since(lastAdvance) > maxStallWindow {
				cancelSend()
				wg.Wait()
				failForkTest(t, nodes, "chain stalled %s for %s: %s current_head[number=%d timestamp=%d hash=%s]", forkLeadPhaseLabel(), maxStallWindow, forkScheduleSummary(), head.number, head.timestamp, head.hash)
			}
			current, err := collectHeights(nodes)
			if err == nil {
				mergeMax(maxHeights, current)
			}
			if head.timestamp+uint64(forkLeadWindow.Seconds()) >= uint64(cfg.Fork.ScheduledTime) {
				break
			}
		}

		preEnd := head.number
		remainingPreFork := int64(cfg.Fork.ScheduledTime) - int64(preStartTime)
		requireStrictPreGrowth := remainingPreFork >= int64(minPreForkWindow.Seconds())
		if requireStrictPreGrowth && preEnd <= preStart {
			cancelSend()
			wg.Wait()
			failForkTest(t, nodes, "no pre-fork block growth: start=%d end=%d target=%s", preStart, preEnd, cfg.Fork.Target)
		}
		if !requireStrictPreGrowth {
			t.Logf("skip strict pre-fork growth check: target=%s pre_window=%ds", cfg.Fork.Target, remainingPreFork)
		}

		crossDeadline := time.Now().Add(40 * time.Second)
		for head.timestamp < uint64(cfg.Fork.ScheduledTime) {
			if time.Now().After(crossDeadline) {
				cancelSend()
				wg.Wait()
				failForkTest(t, nodes, "fork boundary not reached in time: %s current=%d", forkScheduleSummary(), head.timestamp)
			}
			time.Sleep(1 * time.Second)
			head, err = latestHead(primary)
			if err == nil && head.number > lastHeadNumber {
				lastHeadNumber = head.number
				lastAdvance = time.Now()
			}
			if time.Since(lastAdvance) > maxStallWindow {
				cancelSend()
				wg.Wait()
				failForkTest(t, nodes, "chain stalled while crossing %s for %s: %s current_head[number=%d timestamp=%d hash=%s]", forkPhaseAt(uint64(cfg.Fork.ScheduledTime)), maxStallWindow, forkScheduleSummary(), head.number, head.timestamp, head.hash)
			}
			current, err := collectHeights(nodes)
			if err == nil {
				mergeMax(maxHeights, current)
			}
		}

		postStart := head.number
		postEndTime := time.Now().Add(forkPostWindow)
		for time.Now().Before(postEndTime) {
			time.Sleep(1 * time.Second)
			head, err = latestHead(primary)
			if err == nil && head.number > lastHeadNumber {
				lastHeadNumber = head.number
				lastAdvance = time.Now()
			}
			if time.Since(lastAdvance) > maxStallWindow {
				cancelSend()
				wg.Wait()
				failForkTest(t, nodes, "chain stalled %s for %s: %s current_head[number=%d timestamp=%d hash=%s]", forkPhaseAt(head.timestamp), maxStallWindow, forkScheduleSummary(), head.number, head.timestamp, head.hash)
			}
			current, err := collectHeights(nodes)
			if err == nil {
				mergeMax(maxHeights, current)
			}
		}

		postEnd := head.number
		if postEnd <= postStart {
			cancelSend()
			wg.Wait()
			failForkTest(t, nodes, "no post-fork block growth: start=%d end=%d target=%s", postStart, postEnd, cfg.Fork.Target)
		}
	} else {
		deadline := time.Now().Add(noForkObserveWindow)
		for time.Now().Before(deadline) {
			time.Sleep(1 * time.Second)
			head, err = latestHead(primary)
			if err == nil && head.number > lastHeadNumber {
				lastHeadNumber = head.number
				lastAdvance = time.Now()
			}
			if time.Since(lastAdvance) > maxStallWindow {
				cancelSend()
				wg.Wait()
				failForkTest(t, nodes, "chain stalled for %s in non-upgrade mode", maxStallWindow)
			}
			current, err := collectHeights(nodes)
			if err == nil {
				mergeMax(maxHeights, current)
			}
		}
	}

	cancelSend()
	wg.Wait()

	sent := atomic.LoadInt64(&stats.sent)
	failed := atomic.LoadInt64(&stats.failed)
	if sent < minTxSent {
		failForkTest(t, nodes, "traffic too low: sent=%d failed=%d lastErr=%s", sent, failed, stats.lastErr())
	}
	if sent == 0 && failed > 0 {
		failForkTest(t, nodes, "traffic had no success: failed=%d lastErr=%s", failed, stats.lastErr())
	}

	finalHeights, err := collectHeights(nodes)
	if err != nil {
		failForkTest(t, nodes, "collect final heights: %v", err)
	}
	mergeMax(maxHeights, finalHeights)

	var maxFinal uint64
	minFinal := ^uint64(0)
	for _, n := range nodes {
		start := startHeights[n.name]
		end := maxHeights[n.name]
		if end < start {
			failForkTest(t, nodes, "height regressed on %s: start=%d end=%d", n.name, start, end)
		}
		growth := end - start
		if growth < minGrowthPerNode {
			failForkTest(t, nodes, "height growth too low on %s: start=%d end=%d growth=%d", n.name, start, end, growth)
		}
		if end > maxFinal {
			maxFinal = end
		}
		if end < minFinal {
			minFinal = end
		}
		t.Logf("fork node=%s mode=%s target=%s start=%d end=%d growth=%d", n.name, cfg.Fork.Mode, cfg.Fork.Target, start, end, growth)
	}

	if len(nodes) > 1 && maxFinal-minFinal > maxHeightLag {
		failForkTest(t, nodes, "node height lag too large: max=%d min=%d lag=%d", maxFinal, minFinal, maxFinal-minFinal)
	}

	verifyHistoricalBlocks(t, nodes[0], startHeights[nodes[0].name], maxHeights[nodes[0].name])
	verifyCancunFields(t, nodes[0])
	t.Logf("fork traffic summary: sent=%d failed=%d mode=%s target=%s", sent, failed, cfg.Fork.Mode, cfg.Fork.Target)
}

func runTraffic(ctx context.Context, stats *trafficStats, client *ethclient.Client, chainID *big.Int, key *ecdsa.PrivateKey) {
	if key == nil {
		return
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		stats.setErr(err)
		return
	}
	ticker := time.NewTicker(forkTxInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		gasPrice, err := client.SuggestGasPrice(context.Background())
		if err != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
			gasPrice = big.NewInt(1_000_000_000)
		}
		tx := types.NewTransaction(nonce, from, big.NewInt(0), 21_000, gasPrice, nil)
		signed, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
		if err != nil {
			atomic.AddInt64(&stats.failed, 1)
			stats.setErr(err)
			continue
		}
		if err := client.SendTransaction(context.Background(), signed); err != nil {
			atomic.AddInt64(&stats.failed, 1)
			stats.setErr(err)
			switch {
			case strings.Contains(err.Error(), "nonce too low"):
				if refreshed, nerr := client.PendingNonceAt(context.Background(), from); nerr == nil {
					nonce = refreshed
				}
			case strings.Contains(err.Error(), "already known"):
				nonce++
			case strings.Contains(err.Error(), "replacement transaction underpriced"):
				nonce++
			}
			continue
		}
		atomic.AddInt64(&stats.sent, 1)
		nonce++
	}
}

func openForkNodes(cfg *config.Config) ([]forkNode, error) {
	type endpoint struct {
		name string
		url  string
	}

	if cfg == nil {
		return nil, errors.New("nil config")
	}
	seen := make(map[string]struct{})
	var endpoints []endpoint
	add := func(name, url string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		endpoints = append(endpoints, endpoint{name: name, url: url})
	}

	for _, n := range cfg.NodeRPCs {
		add(n.Name, n.URL)
	}
	if len(endpoints) == 0 {
		for i, rpc := range cfg.RPCs {
			add(fmt.Sprintf("rpc%d", i+1), rpc)
		}
	}

	nodes := make([]forkNode, 0, len(endpoints))
	for _, ep := range endpoints {
		dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err := ethclient.DialContext(dialCtx, ep.url)
		cancel()
		if err != nil {
			closeForkNodes(nodes)
			return nil, fmt.Errorf("dial %s (%s): %w", ep.name, ep.url, err)
		}
		nodes = append(nodes, forkNode{name: ep.name, rpcURL: ep.url, client: client})
	}
	return nodes, nil
}

func closeForkNodes(nodes []forkNode) {
	for _, n := range nodes {
		if n.client != nil {
			n.client.Close()
		}
	}
}

func collectHeights(nodes []forkNode) (map[string]uint64, error) {
	out := make(map[string]uint64, len(nodes))
	for _, n := range nodes {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		height, err := n.client.BlockNumber(ctx)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("node %s (%s) blockNumber: %w", n.name, n.rpcURL, err)
		}
		out[n.name] = height
	}
	return out, nil
}

func cloneHeights(src map[string]uint64) map[string]uint64 {
	dst := make(map[string]uint64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mergeMax(target, current map[string]uint64) {
	for name, height := range current {
		if old, ok := target[name]; !ok || height > old {
			target[name] = height
		}
	}
}

type headInfo struct {
	number    uint64
	timestamp uint64
	hash      string
}

func latestHead(client *ethclient.Client) (headInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return headInfo{}, err
	}
	if header == nil {
		return headInfo{}, errors.New("nil latest header")
	}
	return headInfo{
		number:    header.Number.Uint64(),
		timestamp: header.Time,
		hash:      header.Hash().Hex(),
	}, nil
}

type nodeHeadSnapshot struct {
	name      string
	rpcURL    string
	number    uint64
	timestamp uint64
	hash      string
	err       string
}

func failForkTest(t *testing.T, nodes []forkNode, format string, args ...any) {
	t.Helper()
	logForkDiagnostics(t, nodes)
	t.Fatalf(format, args...)
}

func logForkDiagnostics(t *testing.T, nodes []forkNode) {
	t.Helper()
	if len(nodes) == 0 {
		t.Log("fork diag: no nodes available")
		return
	}

	snapshots := collectNodeHeadSnapshots(nodes)
	refIdx := referenceSnapshotIndex(snapshots)
	if refIdx < 0 {
		for _, snap := range snapshots {
			t.Logf("fork diag node=%s rpc=%s err=%s", snap.name, snap.rpcURL, snap.err)
		}
		t.Log("fork diag: all nodes unreachable")
		return
	}

	ref := snapshots[refIdx]
	refLabel := "primary"
	if refIdx != 0 {
		refLabel = fmt.Sprintf("fallback:%s", ref.name)
	}
	t.Logf("fork diag reference=%s head[number=%d timestamp=%d hash=%s]", refLabel, ref.number, ref.timestamp, ref.hash)

	for i, snap := range snapshots {
		if snap.err != "" {
			t.Logf("fork diag node=%s rpc=%s err=%s", snap.name, snap.rpcURL, snap.err)
			continue
		}

		relation := "reference"
		if i != refIdx {
			switch {
			case snap.number < ref.number:
				relation = fmt.Sprintf("behind_by=%d", ref.number-snap.number)
			case snap.number > ref.number:
				relation = fmt.Sprintf("ahead_by=%d", snap.number-ref.number)
			case snap.hash != ref.hash:
				relation = "same_height_hash_mismatch"
			default:
				relation = "matched_hash"
			}
		}

		t.Logf(
			"fork diag node=%s rpc=%s head[number=%d timestamp=%d hash=%s] relation=%s",
			snap.name,
			snap.rpcURL,
			snap.number,
			snap.timestamp,
			snap.hash,
			relation,
		)
	}

	recent := recentCanonicalCoinbases(nodes[refIdx].client, 16)
	if len(recent) > 0 {
		t.Logf("fork diag recent_coinbases=%v", recent)
	}

	if checkpoint := latestCheckpointHeaderSummary(nodes[refIdx].client, ref.number); checkpoint != "" {
		t.Logf("fork diag %s", checkpoint)
	}
}

func collectNodeHeadSnapshots(nodes []forkNode) []nodeHeadSnapshot {
	out := make([]nodeHeadSnapshot, 0, len(nodes))
	for _, n := range nodes {
		snap := nodeHeadSnapshot{name: n.name, rpcURL: n.rpcURL}
		if n.client == nil {
			snap.err = "nil client"
			out = append(out, snap)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		header, err := n.client.HeaderByNumber(ctx, nil)
		cancel()
		if err != nil {
			snap.err = err.Error()
			out = append(out, snap)
			continue
		}
		if header == nil {
			snap.err = "nil latest header"
			out = append(out, snap)
			continue
		}

		snap.number = header.Number.Uint64()
		snap.timestamp = header.Time
		snap.hash = header.Hash().Hex()
		out = append(out, snap)
	}
	return out
}

func referenceSnapshotIndex(snaps []nodeHeadSnapshot) int {
	if len(snaps) == 0 {
		return -1
	}
	if snaps[0].err == "" {
		return 0
	}
	for i := range snaps {
		if snaps[i].err == "" {
			return i
		}
	}
	return -1
}

func recentCanonicalCoinbases(client *ethclient.Client, limit int) []string {
	if client == nil || limit <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	latest, err := client.BlockNumber(ctx)
	cancel()
	if err != nil {
		return []string{fmt.Sprintf("blockNumber_err=%v", err)}
	}

	start := uint64(1)
	if latest >= uint64(limit) {
		start = latest - uint64(limit) + 1
	}

	items := make([]string, 0, latest-start+1)
	for height := start; height <= latest; height++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		header, err := client.HeaderByNumber(ctx, new(big.Int).SetUint64(height))
		cancel()
		if err != nil {
			items = append(items, fmt.Sprintf("%d:error=%v", height, err))
			continue
		}
		if header == nil {
			items = append(items, fmt.Sprintf("%d:nil_header", height))
			continue
		}
		items = append(items, fmt.Sprintf("%d:%s", height, header.Coinbase.Hex()))
	}
	return items
}

func latestCheckpointHeaderSummary(client *ethclient.Client, headNumber uint64) string {
	if client == nil || cfg == nil || cfg.Network.Epoch == 0 || headNumber == 0 {
		return ""
	}

	checkpoint := (headNumber / cfg.Network.Epoch) * cfg.Network.Epoch
	if checkpoint == 0 {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	header, err := client.HeaderByNumber(ctx, new(big.Int).SetUint64(checkpoint))
	cancel()
	if err != nil {
		return fmt.Sprintf("checkpoint_header block=%d err=%v", checkpoint, err)
	}
	if header == nil {
		return fmt.Sprintf("checkpoint_header block=%d err=nil_header", checkpoint)
	}

	signers, err := testkit.ParseHeaderExtraSigners(header.Extra)
	if err != nil {
		return fmt.Sprintf(
			"checkpoint_header block=%d hash=%s coinbase=%s parse_extra_err=%v",
			checkpoint,
			header.Hash().Hex(),
			header.Coinbase.Hex(),
			err,
		)
	}

	return fmt.Sprintf(
		"checkpoint_header block=%d distance_from_head=%d timestamp=%d hash=%s coinbase=%s extra_signers=%v",
		checkpoint,
		headNumber-checkpoint,
		header.Time,
		header.Hash().Hex(),
		header.Coinbase.Hex(),
		signers,
	)
}

func parseUint64Hex(raw string) (uint64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "0x" {
		return 0, nil
	}
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		return strconv.ParseUint(value[2:], 16, 64)
	}
	return strconv.ParseUint(value, 10, 64)
}

func parseCheckpointHeights(start, end uint64) ([]uint64, error) {
	spec := strings.TrimSpace(os.Getenv("FORK_HISTORY_CHECKPOINTS"))
	if spec == "" {
		mid := start
		if end > start {
			mid = start + (end-start)/2
		}
		return []uint64{0, mid, end}, nil
	}

	parts := strings.Split(spec, ",")
	heights := make([]uint64, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, "latest") {
			heights = append(heights, end)
			continue
		}
		height, err := strconv.ParseUint(item, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid checkpoint %q: %w", item, err)
		}
		heights = append(heights, height)
	}
	if len(heights) == 0 {
		return nil, errors.New("no checkpoint heights resolved")
	}
	return heights, nil
}

func verifyHistoricalBlocks(t *testing.T, node forkNode, startHeight, endHeight uint64) {
	t.Helper()

	checkpoints, err := parseCheckpointHeights(startHeight, endHeight)
	if err != nil {
		t.Fatalf("parse fork history checkpoints failed: %v", err)
	}

	rpcClient, err := rpc.DialContext(context.Background(), node.rpcURL)
	if err != nil {
		t.Fatalf("dial rpc for history checks failed: %v", err)
	}
	defer rpcClient.Close()

	for _, height := range checkpoints {
		tag := fmt.Sprintf("0x%x", height)
		var block map[string]any
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", tag, false)
		cancel()
		if err != nil {
			t.Fatalf("eth_getBlockByNumber(%s) failed: %v", tag, err)
		}
		if block == nil {
			t.Fatalf("eth_getBlockByNumber(%s) returned nil", tag)
		}
		hash, _ := block["hash"].(string)
		stateRoot, _ := block["stateRoot"].(string)
		if strings.TrimSpace(hash) == "" || strings.EqualFold(hash, "null") {
			t.Fatalf("missing block hash at height=%d", height)
		}
		if strings.TrimSpace(stateRoot) == "" || strings.EqualFold(stateRoot, "null") {
			t.Fatalf("missing stateRoot at height=%d", height)
		}
	}
}

func verifyCancunFields(t *testing.T, node forkNode) {
	t.Helper()

	expectCancun := false
	expectFixHeader := false
	if strings.EqualFold(os.Getenv("EXPECT_CANCUN_FIELDS"), "1") || strings.EqualFold(os.Getenv("EXPECT_CANCUN_FIELDS"), "true") {
		expectCancun = true
	}
	if strings.EqualFold(os.Getenv("EXPECT_FIXHEADER_FIELDS"), "1") || strings.EqualFold(os.Getenv("EXPECT_FIXHEADER_FIELDS"), "true") {
		expectFixHeader = true
	}
	if cfg != nil && cfg.Fork.Schedule.CancunTime > 0 {
		expectCancun = true
	}
	if !expectCancun {
		return
	}

	rpcClient, err := rpc.DialContext(context.Background(), node.rpcURL)
	if err != nil {
		t.Fatalf("dial rpc for cancun checks failed: %v", err)
	}
	defer rpcClient.Close()

	var block map[string]any
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", "latest", false)
	cancel()
	if err != nil {
		t.Fatalf("eth_getBlockByNumber(latest) failed: %v", err)
	}
	if block == nil {
		t.Fatalf("latest block response is nil")
	}

	tsRaw, _ := block["timestamp"].(string)
	ts, parseErr := parseUint64Hex(tsRaw)
	if parseErr != nil {
		t.Fatalf("parse latest block timestamp failed: %v", parseErr)
	}

	if cfg != nil && cfg.Fork.Schedule.CancunTime > 0 {
		if int64(ts) < cfg.Fork.Schedule.CancunTime {
			t.Logf("skip cancun field assertion: latest timestamp=%d < cancun_time=%d", ts, cfg.Fork.Schedule.CancunTime)
			return
		}
	}
	if cfg != nil && cfg.Fork.Schedule.FixHeaderTime > 0 && int64(ts) >= cfg.Fork.Schedule.FixHeaderTime {
		expectFixHeader = true
	}

	required := []string{"blobGasUsed", "excessBlobGas"}
	for _, field := range required {
		value, exists := block[field]
		if !exists || value == nil {
			t.Fatalf("missing Cancun field %s in latest block", field)
		}
		str, _ := value.(string)
		if strings.TrimSpace(str) == "" || strings.EqualFold(str, "null") {
			t.Fatalf("empty Cancun field %s in latest block", field)
		}
	}

	if !expectFixHeader {
		return
	}

	value, exists := block["parentBeaconBlockRoot"]
	if !exists || value == nil {
		t.Fatalf("missing fixHeader field parentBeaconBlockRoot in latest block")
	}
	root, _ := value.(string)
	if strings.TrimSpace(root) == "" || strings.EqualFold(root, "null") {
		t.Fatalf("empty fixHeader field parentBeaconBlockRoot in latest block")
	}
	if !strings.EqualFold(root, "0x0000000000000000000000000000000000000000000000000000000000000000") {
		t.Fatalf("invalid parentBeaconBlockRoot post-fixHeaderTime: have=%s", root)
	}
}
