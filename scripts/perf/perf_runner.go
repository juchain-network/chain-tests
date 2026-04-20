package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	intcfg "juchain.org/chain/tools/ci/internal/config"
)

type sample struct {
	Timestamp        time.Time
	TierTPS          int
	Sent             int64
	Accepted         int64
	Confirmed        int64
	Failed           int64
	BackpressureDrop int64
	PrimaryHeight    uint64
	MaxHeight        uint64
	MinHeight        uint64
	HeightLag        uint64
	RPCLatencyMS     float64
	CPUPercent       float64
	MemoryMB         float64
	DiskMB           float64
	ConsecutiveStall int
}

type tierSummary struct {
	TPS                 int     `json:"tps"`
	DurationSeconds     int64   `json:"duration_seconds"`
	Sent                int64   `json:"sent"`
	Accepted            int64   `json:"accepted"`
	Confirmed           int64   `json:"confirmed"`
	Failed              int64   `json:"failed"`
	BackpressureDrop    int64   `json:"backpressure_drop,omitempty"`
	AcceptedRate        float64 `json:"accepted_rate"`
	ConfirmedRate       float64 `json:"confirmed_rate"`
	AcceptedTPS         float64 `json:"accepted_tps"`
	ConfirmedTPS        float64 `json:"confirmed_tps"`
	StartHeight         uint64  `json:"start_height"`
	EndHeight           uint64  `json:"end_height"`
	HeightGrowth        uint64  `json:"height_growth"`
	MaxHeightLag        uint64  `json:"max_height_lag"`
	P95RPCLatencyMS     float64 `json:"p95_rpc_latency_ms"`
	MaxConsecutiveStall int     `json:"max_consecutive_stall"`
	Status              string  `json:"status"`
	Notes               string  `json:"notes,omitempty"`
}

type maxSearchResult struct {
	BaseTPS          int    `json:"base_tps"`
	StepTPS          int    `json:"step_tps"`
	TargetTPS        int    `json:"target_tps"`
	StepDurationSecs int64  `json:"step_duration_seconds"`
	LastStableTPS    int    `json:"last_stable_tps,omitempty"`
	FirstUnstableTPS int    `json:"first_unstable_tps,omitempty"`
	StopReason       string `json:"stop_reason,omitempty"`
}

type verdict struct {
	GeneratedAt    string           `json:"generated_at"`
	Mode           string           `json:"mode"`
	Scope          string           `json:"scope"`
	Topology       string           `json:"topology,omitempty"`
	IngressRPC     string           `json:"ingress_rpc,omitempty"`
	Config         string           `json:"config"`
	DataDir        string           `json:"data_dir"`
	SenderAccounts int              `json:"sender_accounts,omitempty"`
	Warmup         *warmupResult    `json:"warmup,omitempty"`
	Thresholds     map[string]any   `json:"thresholds"`
	MaxSearch      *maxSearchResult `json:"max_search,omitempty"`
	Tiers          []tierSummary    `json:"tiers"`
	FailedReasons  []string         `json:"failed_reasons,omitempty"`
	TopSlowWindows []slowWindow     `json:"top_slow_windows,omitempty"`
	ResourcePeaks  resourcePeaks    `json:"resource_peaks"`
	Status         string           `json:"status"`
}

type warmupResult struct {
	Enabled               bool   `json:"enabled"`
	Status                string `json:"status"`
	DurationSeconds       int64  `json:"duration_seconds"`
	StableSamplesRequired int    `json:"stable_samples_required"`
	StableSamplesObserved int    `json:"stable_samples_observed"`
	MaxHeight             uint64 `json:"max_height"`
	MinHeight             uint64 `json:"min_height"`
	Lag                   uint64 `json:"lag"`
	Notes                 string `json:"notes,omitempty"`
}

type resourcePeaks struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   float64 `json:"memory_mb"`
	DiskMB     float64 `json:"disk_mb"`
}

type slowWindow struct {
	Timestamp        string  `json:"timestamp"`
	TPS              int     `json:"tps"`
	RPCLatencyMS     float64 `json:"rpc_latency_ms"`
	StallSeconds     int     `json:"stall_seconds"`
	HeightLag        uint64  `json:"height_lag"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryMB         float64 `json:"memory_mb"`
	DiskMB           float64 `json:"disk_mb"`
	PrimaryHeight    uint64  `json:"primary_height"`
	ConsecutiveStall int     `json:"consecutive_stall"`
}

func parseTPSList(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("invalid tps value: %q", p)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty tps tiers")
	}
	return out, nil
}

func percentile95(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)
	idx := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func queryHeights(clients []*ethclient.Client) (max uint64, min uint64, primary uint64, err error) {
	if len(clients) == 0 {
		return 0, 0, 0, fmt.Errorf("no clients")
	}
	min = ^uint64(0)
	for i, c := range clients {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		h, e := c.BlockNumber(ctx)
		cancel()
		if e != nil {
			return 0, 0, 0, e
		}
		if i == 0 {
			primary = h
		}
		if h > max {
			max = h
		}
		if h < min {
			min = h
		}
	}
	return max, min, primary, nil
}

func sampleResources(dataDir string) (cpu float64, memMB float64, diskMB float64) {
	psCmd := exec.Command("/bin/sh", "-lc", `ps -Ao comm,pcpu,rss | awk '/(geth|juchain|congress-node|reth)/ {cpu+=$2; rss+=$3} END {printf "%.3f %.3f", cpu, rss/1024.0}'`)
	if out, err := psCmd.Output(); err == nil {
		fields := strings.Fields(string(out))
		if len(fields) >= 2 {
			if v, e := strconv.ParseFloat(fields[0], 64); e == nil {
				cpu = v
			}
			if v, e := strconv.ParseFloat(fields[1], 64); e == nil {
				memMB = v
			}
		}
	}
	duCmd := exec.Command("du", "-sk", dataDir)
	if out, err := duCmd.Output(); err == nil {
		fields := strings.Fields(string(out))
		if len(fields) >= 1 {
			if kb, e := strconv.ParseFloat(fields[0], 64); e == nil {
				diskMB = kb / 1024.0
			}
		}
	}
	return cpu, memMB, diskMB
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeMetricsCSV(path string, samples []sample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{
		"timestamp", "tier_tps", "sent", "accepted", "confirmed", "failed", "backpressure_drop", "primary_height", "max_height", "min_height", "height_lag", "rpc_latency_ms", "cpu_percent", "memory_mb", "disk_mb", "consecutive_stall",
	})
	for _, s := range samples {
		_ = w.Write([]string{
			s.Timestamp.Format(time.RFC3339),
			strconv.Itoa(s.TierTPS),
			strconv.FormatInt(s.Sent, 10),
			strconv.FormatInt(s.Accepted, 10),
			strconv.FormatInt(s.Confirmed, 10),
			strconv.FormatInt(s.Failed, 10),
			strconv.FormatInt(s.BackpressureDrop, 10),
			strconv.FormatUint(s.PrimaryHeight, 10),
			strconv.FormatUint(s.MaxHeight, 10),
			strconv.FormatUint(s.MinHeight, 10),
			strconv.FormatUint(s.HeightLag, 10),
			fmt.Sprintf("%.2f", s.RPCLatencyMS),
			fmt.Sprintf("%.3f", s.CPUPercent),
			fmt.Sprintf("%.3f", s.MemoryMB),
			fmt.Sprintf("%.3f", s.DiskMB),
			strconv.Itoa(s.ConsecutiveStall),
		})
	}
	return nil
}

func writeSummaryMD(path string, v verdict) error {
	var b strings.Builder
	b.WriteString("# Performance Summary\n\n")
	b.WriteString(fmt.Sprintf("- Generated: %s\n", v.GeneratedAt))
	b.WriteString(fmt.Sprintf("- Mode: %s\n", v.Mode))
	b.WriteString(fmt.Sprintf("- Topology: %s\n", v.Topology))
	b.WriteString(fmt.Sprintf("- Scope: %s\n", v.Scope))
	if strings.TrimSpace(v.IngressRPC) != "" {
		b.WriteString(fmt.Sprintf("- Ingress RPC: %s\n", v.IngressRPC))
	}
	if v.SenderAccounts > 0 {
		b.WriteString(fmt.Sprintf("- Sender accounts: %d\n", v.SenderAccounts))
	}
	b.WriteString(fmt.Sprintf("- Config: %s\n", v.Config))
	b.WriteString(fmt.Sprintf("- DataDir: %s\n", v.DataDir))
	b.WriteString(fmt.Sprintf("- Status: %s\n\n", v.Status))
	b.WriteString("## Go/No-Go\n\n")
	b.WriteString(fmt.Sprintf("- Verdict: **%s**\n", v.Status))
	if len(v.FailedReasons) == 0 {
		b.WriteString("- Failed reasons: none\n\n")
	} else {
		b.WriteString("- Failed reasons:\n")
		for _, reason := range v.FailedReasons {
			b.WriteString(fmt.Sprintf("  - %s\n", reason))
		}
		b.WriteString("\n")
	}

	if v.Warmup != nil {
		b.WriteString("## Warmup\n\n")
		b.WriteString("| Scope | Status | Duration(s) | StableSamples | Observed | MaxHeight | MinHeight | Lag |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %d | %d | %d |\n",
			v.Scope, v.Warmup.Status, v.Warmup.DurationSeconds, v.Warmup.StableSamplesRequired, v.Warmup.StableSamplesObserved, v.Warmup.MaxHeight, v.Warmup.MinHeight, v.Warmup.Lag,
		))
		if strings.TrimSpace(v.Warmup.Notes) != "" {
			b.WriteString(fmt.Sprintf("\n- Notes: %s\n", v.Warmup.Notes))
		}
		b.WriteString("\n")
	}

	if v.MaxSearch != nil {
		b.WriteString("## Max TPS Search\n\n")
		b.WriteString("| BaseTPS | StepTPS | TargetTPS | StepDuration(s) | LastStable | FirstUnstable | StopReason |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		b.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d | %d | %s |\n\n",
			v.MaxSearch.BaseTPS,
			v.MaxSearch.StepTPS,
			v.MaxSearch.TargetTPS,
			v.MaxSearch.StepDurationSecs,
			v.MaxSearch.LastStableTPS,
			v.MaxSearch.FirstUnstableTPS,
			v.MaxSearch.StopReason,
		))
	}

	b.WriteString("## Resource Peaks\n\n")
	b.WriteString("| CPU(%) | Memory(MB) | Disk(MB) |\n")
	b.WriteString("| --- | --- | --- |\n")
	b.WriteString(fmt.Sprintf("| %.3f | %.3f | %.3f |\n\n", v.ResourcePeaks.CPUPercent, v.ResourcePeaks.MemoryMB, v.ResourcePeaks.DiskMB))

	b.WriteString("## Tier Summary\n\n")
	b.WriteString("| TPS | Duration(s) | Sent | Accepted | Confirmed | Failed | AcceptedRate | ConfirmedRate | AcceptedTPS | ConfirmedTPS | HeightGrowth | MaxLag | p95 RPC(ms) | MaxStall | Status |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, t := range v.Tiers {
		b.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d | %d | %.4f | %.4f | %.2f | %.2f | %d | %d | %.2f | %d | %s |\n",
			t.TPS, t.DurationSeconds, t.Sent, t.Accepted, t.Confirmed, t.Failed, t.AcceptedRate, t.ConfirmedRate, t.AcceptedTPS, t.ConfirmedTPS, t.HeightGrowth, t.MaxHeightLag, t.P95RPCLatencyMS, t.MaxConsecutiveStall, t.Status,
		))
	}
	b.WriteString("\n## Top Slow Windows\n\n")
	if len(v.TopSlowWindows) == 0 {
		b.WriteString("No sample windows captured.\n")
	} else {
		b.WriteString("| Timestamp | TPS | RPC(ms) | Stall(s) | HeightLag | CPU(%) | Memory(MB) | Disk(MB) |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		for _, w := range v.TopSlowWindows {
			b.WriteString(fmt.Sprintf("| %s | %d | %.2f | %d | %d | %.3f | %.3f | %.3f |\n",
				w.Timestamp, w.TPS, w.RPCLatencyMS, w.StallSeconds, w.HeightLag, w.CPUPercent, w.MemoryMB, w.DiskMB,
			))
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func collectFailedReasons(summaries []tierSummary) []string {
	if len(summaries) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, s := range summaries {
		if s.Status == "PASS" {
			continue
		}
		notes := strings.Split(s.Notes, ";")
		found := false
		for _, note := range notes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			item := fmt.Sprintf("tier %d: %s", s.TPS, note)
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
			found = true
		}
		if !found {
			item := fmt.Sprintf("tier %d: unknown threshold violation", s.TPS)
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func collectResourcePeaks(samples []sample) resourcePeaks {
	var peaks resourcePeaks
	for _, s := range samples {
		if s.CPUPercent > peaks.CPUPercent {
			peaks.CPUPercent = s.CPUPercent
		}
		if s.MemoryMB > peaks.MemoryMB {
			peaks.MemoryMB = s.MemoryMB
		}
		if s.DiskMB > peaks.DiskMB {
			peaks.DiskMB = s.DiskMB
		}
	}
	return peaks
}

func collectTopSlowWindows(samples []sample, sampleInterval time.Duration, limit int) []slowWindow {
	if len(samples) == 0 || limit <= 0 {
		return nil
	}
	ordered := append([]sample{}, samples...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].RPCLatencyMS == ordered[j].RPCLatencyMS {
			stallI := ordered[i].ConsecutiveStall
			stallJ := ordered[j].ConsecutiveStall
			if stallI == stallJ {
				if ordered[i].HeightLag == ordered[j].HeightLag {
					return ordered[i].Timestamp.After(ordered[j].Timestamp)
				}
				return ordered[i].HeightLag > ordered[j].HeightLag
			}
			return stallI > stallJ
		}
		return ordered[i].RPCLatencyMS > ordered[j].RPCLatencyMS
	})
	if len(ordered) < limit {
		limit = len(ordered)
	}
	out := make([]slowWindow, 0, limit)
	for _, s := range ordered[:limit] {
		stallSeconds := int(math.Round(float64(s.ConsecutiveStall) * sampleInterval.Seconds()))
		out = append(out, slowWindow{
			Timestamp:        s.Timestamp.Format(time.RFC3339),
			TPS:              s.TierTPS,
			RPCLatencyMS:     s.RPCLatencyMS,
			StallSeconds:     stallSeconds,
			HeightLag:        s.HeightLag,
			CPUPercent:       s.CPUPercent,
			MemoryMB:         s.MemoryMB,
			DiskMB:           s.DiskMB,
			PrimaryHeight:    s.PrimaryHeight,
			ConsecutiveStall: s.ConsecutiveStall,
		})
	}
	return out
}

func waitForMultiScopeWarmup(clients []*ethclient.Client, sampleInterval time.Duration, maxHeightLag uint64, timeout time.Duration, stableSamplesRequired int) warmupResult {
	result := warmupResult{
		Enabled:               true,
		Status:                "PASS",
		StableSamplesRequired: stableSamplesRequired,
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if sampleInterval <= 0 {
		sampleInterval = 2 * time.Second
	}
	if stableSamplesRequired <= 0 {
		stableSamplesRequired = 3
		result.StableSamplesRequired = stableSamplesRequired
	}

	start := time.Now()
	deadline := start.Add(timeout)
	stableSamples := 0
	lastNote := ""

	for {
		maxHeight, minHeight, _, err := queryHeights(clients)
		if err != nil {
			lastNote = fmt.Sprintf("warmup height query failed: %v", err)
			stableSamples = 0
		} else {
			lag := uint64(0)
			if maxHeight >= minHeight {
				lag = maxHeight - minHeight
			}
			result.MaxHeight = maxHeight
			result.MinHeight = minHeight
			result.Lag = lag

			if minHeight > 0 && lag <= maxHeightLag {
				stableSamples++
				result.StableSamplesObserved = stableSamples
				if stableSamples >= stableSamplesRequired {
					result.DurationSeconds = int64(time.Since(start).Seconds())
					result.Notes = fmt.Sprintf("multi-scope warmup converged within lag threshold %d", maxHeightLag)
					return result
				}
			} else {
				stableSamples = 0
				result.StableSamplesObserved = 0
				lastNote = fmt.Sprintf("waiting for convergence: max=%d min=%d lag=%d threshold=%d", maxHeight, minHeight, lag, maxHeightLag)
			}
		}

		if time.Now().After(deadline) {
			result.Status = "FAIL"
			result.DurationSeconds = int64(time.Since(start).Seconds())
			result.StableSamplesObserved = stableSamples
			if strings.TrimSpace(lastNote) == "" {
				lastNote = fmt.Sprintf("multi-scope warmup timeout: max=%d min=%d lag=%d threshold=%d", result.MaxHeight, result.MinHeight, result.Lag, maxHeightLag)
			} else {
				lastNote = fmt.Sprintf("multi-scope warmup timeout: %s", lastNote)
			}
			result.Notes = lastNote
			return result
		}

		time.Sleep(sampleInterval)
	}
}

func writeVerdictArtifacts(outDir string, v verdict, samples []sample, sampleInterval time.Duration) error {
	metricsCSV := filepath.Join(outDir, "metrics.csv")
	if err := writeMetricsCSV(metricsCSV, samples); err != nil {
		return fmt.Errorf("write metrics csv failed: %w", err)
	}

	verdictPath := filepath.Join(outDir, "verdict.json")
	data, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(verdictPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write verdict failed: %w", err)
	}

	summaryPath := filepath.Join(outDir, "summary.md")
	if err := writeSummaryMD(summaryPath, v); err != nil {
		return fmt.Errorf("write summary failed: %w", err)
	}

	fmt.Printf("summary: %s\n", summaryPath)
	fmt.Printf("metrics: %s\n", metricsCSV)
	fmt.Printf("verdict: %s\n", verdictPath)
	return nil
}

func plannedMaxTPS(mode string, tiers []int, target int) int {
	maxTPS := target
	for _, tps := range tiers {
		if tps > maxTPS {
			maxTPS = tps
		}
	}
	if strings.EqualFold(mode, "soak") && len(tiers) > 0 && tiers[0] > maxTPS {
		maxTPS = tiers[0]
	}
	return maxTPS
}

func runTier(tps int, duration time.Duration, sampleInterval time.Duration, lagClients []*ethclient.Client, dataDir string, generator *loadGenerator, minSuccessRate float64, maxStallSeconds int, maxHeightLag uint64, maxP95LatencyMS float64, allSamples *[]sample) tierSummary {
	start := time.Now()
	end := start.Add(duration)
	sampleTicker := time.NewTicker(sampleInterval)
	defer sampleTicker.Stop()

	fmt.Printf("▶ starting tier=%d duration=%s sample_interval=%s\n", tps, duration, sampleInterval)

	var latencies []float64
	var maxLag uint64
	var startHeight, endHeight uint64
	var lastPrimary uint64
	consecutiveStall := 0
	maxConsecutiveStall := 0

	maxH, minH, primaryH, err := queryHeights(lagClients)
	if err == nil {
		startHeight = primaryH
		lastPrimary = primaryH
		if maxH >= minH {
			maxLag = maxH - minH
		}
	}

	generator.Start()
	for time.Now().Before(end) {
		<-sampleTicker.C
		probeStart := time.Now()
		maxHeight, minHeight, primaryHeight, errHeights := queryHeights(lagClients)
		latMS := float64(time.Since(probeStart).Milliseconds())
		latencies = append(latencies, latMS)
		if errHeights != nil {
			maxHeight, minHeight, primaryHeight = 0, 0, 0
		}
		if primaryHeight > 0 {
			if primaryHeight == lastPrimary {
				consecutiveStall++
			} else {
				consecutiveStall = 0
				lastPrimary = primaryHeight
			}
			if consecutiveStall > maxConsecutiveStall {
				maxConsecutiveStall = consecutiveStall
			}
		}
		lag := uint64(0)
		if maxHeight >= minHeight {
			lag = maxHeight - minHeight
			if lag > maxLag {
				maxLag = lag
			}
		}
		snap := generator.Snapshot()
		cpu, memMB, diskMB := sampleResources(dataDir)
		*allSamples = append(*allSamples, sample{
			Timestamp:        time.Now().UTC(),
			TierTPS:          tps,
			Sent:             snap.Sent,
			Accepted:         snap.Accepted,
			Confirmed:        snap.Confirmed,
			Failed:           snap.Failed,
			BackpressureDrop: snap.BackpressureDrop,
			PrimaryHeight:    primaryHeight,
			MaxHeight:        maxHeight,
			MinHeight:        minHeight,
			HeightLag:        lag,
			RPCLatencyMS:     latMS,
			CPUPercent:       cpu,
			MemoryMB:         memMB,
			DiskMB:           diskMB,
			ConsecutiveStall: consecutiveStall,
		})
		elapsed := time.Since(start).Round(time.Second)
		fmt.Printf("  progress tier=%d elapsed=%s/%s sent=%d accepted=%d confirmed=%d failed=%d lag=%d primary_height=%d\n",
			tps,
			elapsed,
			duration,
			snap.Sent,
			snap.Accepted,
			snap.Confirmed,
			snap.Failed,
			lag,
			primaryHeight,
		)
	}
	generator.Stop()
	drain := generator.DrainConfirmations(defaultDrainTimeout)

	maxH2, minH2, primaryEnd, err2 := queryHeights(lagClients)
	if err2 == nil {
		endHeight = primaryEnd
		if maxH2 >= minH2 {
			lag := maxH2 - minH2
			if lag > maxLag {
				maxLag = lag
			}
		}
	}

	acceptedRate := 0.0
	confirmedRate := 0.0
	if drain.Sent > 0 {
		acceptedRate = float64(drain.Accepted) / float64(drain.Sent)
		confirmedRate = float64(drain.Confirmed) / float64(drain.Sent)
	}
	p95 := percentile95(latencies)
	status := "PASS"
	noteParts := []string{}
	if acceptedRate < minSuccessRate {
		status = "FAIL"
		noteParts = append(noteParts, fmt.Sprintf("accepted_rate %.4f < %.4f", acceptedRate, minSuccessRate))
	}
	if confirmedRate < minSuccessRate {
		status = "FAIL"
		noteParts = append(noteParts, fmt.Sprintf("confirmed_rate %.4f < %.4f", confirmedRate, minSuccessRate))
	}
	if drain.PendingBacklog > 0 {
		status = "FAIL"
		noteParts = append(noteParts, fmt.Sprintf("pending_backlog %d remained after drain", drain.PendingBacklog))
	}
	if maxLag > maxHeightLag {
		status = "FAIL"
		noteParts = append(noteParts, fmt.Sprintf("max_height_lag %d > %d", maxLag, maxHeightLag))
	}
	if p95 > maxP95LatencyMS {
		status = "FAIL"
		noteParts = append(noteParts, fmt.Sprintf("p95_latency %.2f > %.2f", p95, maxP95LatencyMS))
	}
	stallWindowSeconds := maxConsecutiveStall * int(math.Max(sampleInterval.Seconds(), 1))
	if stallWindowSeconds > maxStallSeconds {
		status = "FAIL"
		noteParts = append(noteParts, fmt.Sprintf("stall_window %ds > %ds", stallWindowSeconds, maxStallSeconds))
	}
	if drain.BackpressureDrop > 0 {
		noteParts = append(noteParts, fmt.Sprintf("sender_backpressure_drop=%d", drain.BackpressureDrop))
	}

	acceptedTPS := 0.0
	confirmedTPS := 0.0
	if duration > 0 {
		acceptedTPS = float64(drain.Accepted) / duration.Seconds()
		confirmedTPS = float64(drain.Confirmed) / duration.Seconds()
	}

	result := tierSummary{
		TPS:                 tps,
		DurationSeconds:     int64(duration.Seconds()),
		Sent:                drain.Sent,
		Accepted:            drain.Accepted,
		Confirmed:           drain.Confirmed,
		Failed:              drain.Failed,
		BackpressureDrop:    drain.BackpressureDrop,
		AcceptedRate:        acceptedRate,
		ConfirmedRate:       confirmedRate,
		AcceptedTPS:         acceptedTPS,
		ConfirmedTPS:        confirmedTPS,
		StartHeight:         startHeight,
		EndHeight:           endHeight,
		HeightGrowth:        endHeight - startHeight,
		MaxHeightLag:        maxLag,
		P95RPCLatencyMS:     p95,
		MaxConsecutiveStall: maxConsecutiveStall,
		Status:              status,
		Notes:               strings.Join(noteParts, "; "),
	}
	fmt.Printf("✔ finished tier=%d status=%s accepted=%d confirmed=%d failed=%d accepted_tps=%.2f confirmed_tps=%.2f max_lag=%d notes=%s\n",
		result.TPS,
		result.Status,
		result.Accepted,
		result.Confirmed,
		result.Failed,
		result.AcceptedTPS,
		result.ConfirmedTPS,
		result.MaxHeightLag,
		strings.TrimSpace(result.Notes),
	)
	return result
}

func main() {
	configPath := flag.String("config", "data/test_config.yaml", "Path to generated test config")
	outDir := flag.String("out-dir", "reports/perf", "Output directory")
	mode := flag.String("mode", "tiers", "tiers|max|soak")
	scope := flag.String("scope", "single", "Lag validation scope: single|multi")
	topology := flag.String("topology", "single", "Network topology under test: single|multi")
	tiersRaw := flag.String("tiers", "10,30,60", "Comma-separated TPS tiers")
	durationRaw := flag.String("duration", "90s", "Duration per tier")
	sampleIntervalRaw := flag.String("sample-interval", "2s", "Metrics sample interval")
	dataDir := flag.String("data-dir", "data", "Chain data directory for disk usage sampling")
	minSuccessRate := flag.Float64("min-success-rate", 0.99, "Minimum accepted/confirmed tx success rate")
	maxStallSeconds := flag.Int("max-stall-seconds", 15, "Maximum consecutive no-growth sample window")
	maxHeightLag := flag.Uint64("max-height-lag", 8, "Maximum allowed node height lag")
	maxP95LatencyMS := flag.Float64("max-p95-latency-ms", 500, "Maximum allowed p95 RPC latency")
	multiWarmupTimeoutRaw := flag.String("multi-warmup-timeout", "60s", "Maximum warmup wait before multi-scope lag checks start")
	multiWarmupStableSamples := flag.Int("multi-warmup-stable-samples", 3, "Consecutive in-threshold samples required before multi-scope measurement starts")
	senderAccountsFlag := flag.Int("sender-accounts", 0, "Number of funded sender accounts; 0 means auto-size by target TPS")
	maxBaseTPS := flag.Int("max-base-tps", 1000, "MODE=max starting TPS")
	maxStepTPS := flag.Int("max-step", 100, "MODE=max TPS increment per step")
	maxTargetTPS := flag.Int("max-target-tps", 5000, "MODE=max upper bound; stop once reached stably")
	flag.Parse()

	cfg, err := intcfg.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}
	if len(cfg.RPCs) == 0 {
		fmt.Fprintln(os.Stderr, "no rpcs configured")
		os.Exit(1)
	}
	if cfg.Funder.PrivateKey == "" {
		fmt.Fprintln(os.Stderr, "missing funder private key")
		os.Exit(1)
	}
	key, err := crypto.HexToECDSA(cfg.Funder.PrivateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid funder private key: %v\n", err)
		os.Exit(1)
	}

	modeName := strings.ToLower(strings.TrimSpace(*mode))
	switch modeName {
	case "tiers", "max", "soak":
	default:
		fmt.Fprintf(os.Stderr, "invalid mode %q: expected tiers|max|soak\n", *mode)
		os.Exit(1)
	}

	var tiers []int
	switch modeName {
	case "max":
		tiers, err = buildMaxTPSSteps(*maxBaseTPS, *maxStepTPS, *maxTargetTPS)
	case "soak", "tiers":
		tiers, err = parseTPSList(*tiersRaw)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "tier setup failed: %v\n", err)
		os.Exit(1)
	}

	duration, err := time.ParseDuration(*durationRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "duration parse failed: %v\n", err)
		os.Exit(1)
	}
	sampleInterval, err := time.ParseDuration(*sampleIntervalRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sample-interval parse failed: %v\n", err)
		os.Exit(1)
	}
	if sampleInterval <= 0 {
		sampleInterval = 2 * time.Second
	}
	if duration <= 0 {
		duration = 90 * time.Second
	}
	multiWarmupTimeout, err := time.ParseDuration(*multiWarmupTimeoutRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "multi warmup timeout parse failed: %v\n", err)
		os.Exit(1)
	}
	if *multiWarmupStableSamples <= 0 {
		fmt.Fprintln(os.Stderr, "multi warmup stable samples must be > 0")
		os.Exit(1)
	}

	if err := ensureDir(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	clients := make([]*ethclient.Client, 0, len(cfg.RPCs))
	for _, rpc := range cfg.RPCs {
		c, err := ethclient.Dial(rpc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dial %s failed: %v\n", rpc, err)
			os.Exit(1)
		}
		clients = append(clients, c)
	}
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	scopeName := strings.ToLower(strings.TrimSpace(*scope))
	switch scopeName {
	case "", "single":
		scopeName = "single"
	case "multi":
	default:
		fmt.Fprintf(os.Stderr, "invalid scope %q: expected single|multi\n", *scope)
		os.Exit(1)
	}
	topologyName := strings.ToLower(strings.TrimSpace(*topology))
	switch topologyName {
	case "", "single":
		topologyName = "single"
	case "multi":
	default:
		fmt.Fprintf(os.Stderr, "invalid topology %q: expected single|multi\n", *topology)
		os.Exit(1)
	}
	if topologyName == "multi" && scopeName == "single" {
		scopeName = "multi"
	}

	lagClients := append([]*ethclient.Client{}, clients[:1]...)
	var extraLagClients []*ethclient.Client
	if scopeName == "multi" {
		if len(cfg.NodeRPCs) > 0 {
			lagClients = lagClients[:0]
			seen := make(map[string]struct{})
			for _, n := range cfg.NodeRPCs {
				url := strings.TrimSpace(n.URL)
				if url == "" {
					continue
				}
				if _, ok := seen[url]; ok {
					continue
				}
				seen[url] = struct{}{}
				c, err := ethclient.Dial(url)
				if err != nil {
					fmt.Fprintf(os.Stderr, "dial node rpc %s failed: %v\n", url, err)
					os.Exit(1)
				}
				lagClients = append(lagClients, c)
				extraLagClients = append(extraLagClients, c)
			}
		} else {
			lagClients = append([]*ethclient.Client{}, clients...)
		}
	}
	defer func() {
		for _, c := range extraLagClients {
			c.Close()
		}
	}()

	chainID, err := clients[0].ChainID(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "chain id query failed: %v\n", err)
		os.Exit(1)
	}

	allSamples := make([]sample, 0, 4096)
	summaries := make([]tierSummary, 0, len(tiers))
	var warmup *warmupResult
	var maxSearch *maxSearchResult

	if scopeName == "multi" {
		result := waitForMultiScopeWarmup(lagClients, sampleInterval, *maxHeightLag, multiWarmupTimeout, *multiWarmupStableSamples)
		warmup = &result
	}

	thresholds := map[string]any{
		"success_rate_min":             *minSuccessRate,
		"stall_window_seconds_max":     *maxStallSeconds,
		"max_height_lag":               *maxHeightLag,
		"rpc_latency_p95_ms_max":       *maxP95LatencyMS,
		"multi_warmup_timeout_seconds": int64(multiWarmupTimeout.Seconds()),
		"multi_warmup_stable_samples":  *multiWarmupStableSamples,
	}

	failedReasons := make([]string, 0, len(tiers)+1)
	if warmup != nil && warmup.Status != "PASS" {
		failedReasons = append(failedReasons, fmt.Sprintf("warmup: %s", warmup.Notes))
		v := verdict{
			GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
			Mode:           modeName,
			Scope:          scopeName,
			Topology:       topologyName,
			IngressRPC:     cfg.RPCs[0],
			Config:         *configPath,
			DataDir:        *dataDir,
			Warmup:         warmup,
			Thresholds:     thresholds,
			Tiers:          summaries,
			FailedReasons:  failedReasons,
			TopSlowWindows: collectTopSlowWindows(allSamples, sampleInterval, 10),
			ResourcePeaks:  collectResourcePeaks(allSamples),
			Status:         "FAIL",
		}
		if err := writeVerdictArtifacts(*outDir, v, allSamples, sampleInterval); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		os.Exit(1)
	}

	accountCount := *senderAccountsFlag
	if accountCount <= 0 {
		accountCount = recommendedSenderAccountCount(plannedMaxTPS(modeName, tiers, *maxTargetTPS))
	}
	thresholds["sender_accounts"] = accountCount

	senderAccounts, err := prepareSenderAccounts(clients[0], key, chainID, accountCount)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare sender accounts failed: %v\n", err)
		os.Exit(1)
	}

	for _, tps := range tiers {
		if err := syncSenderAccountNonces(clients[0], senderAccounts); err != nil {
			fmt.Fprintf(os.Stderr, "sync sender nonces failed: %v\n", err)
			os.Exit(1)
		}
		summary := runTier(tps, duration, sampleInterval, lagClients, *dataDir, newLoadGenerator(clients[0], chainID, senderAccounts, tps), *minSuccessRate, *maxStallSeconds, *maxHeightLag, *maxP95LatencyMS, &allSamples)
		summaries = append(summaries, summary)
		if modeName == "max" && maxSearch == nil {
			maxSearch = &maxSearchResult{
				BaseTPS:          *maxBaseTPS,
				StepTPS:          *maxStepTPS,
				TargetTPS:        *maxTargetTPS,
				StepDurationSecs: int64(duration.Seconds()),
			}
		}
		if summary.Status == "PASS" {
			if maxSearch != nil {
				maxSearch.LastStableTPS = summary.TPS
				if summary.TPS >= *maxTargetTPS {
					maxSearch.StopReason = "target_reached"
					break
				}
			}
			continue
		}
		if maxSearch != nil {
			maxSearch.FirstUnstableTPS = summary.TPS
			maxSearch.StopReason = "first_unstable_tps"
			break
		}
	}
	if maxSearch != nil && maxSearch.StopReason == "" {
		maxSearch.StopReason = "exhausted_search_range"
	}

	failedReasons = append(failedReasons, collectFailedReasons(summaries)...)
	overallStatus := "PASS"
	if len(failedReasons) > 0 {
		overallStatus = "FAIL"
	}

	v := verdict{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Mode:           modeName,
		Scope:          scopeName,
		Topology:       topologyName,
		IngressRPC:     cfg.RPCs[0],
		Config:         *configPath,
		DataDir:        *dataDir,
		SenderAccounts: accountCount,
		Warmup:         warmup,
		Thresholds:     thresholds,
		MaxSearch:      maxSearch,
		Tiers:          summaries,
		FailedReasons:  failedReasons,
		TopSlowWindows: collectTopSlowWindows(allSamples, sampleInterval, 10),
		ResourcePeaks:  collectResourcePeaks(allSamples),
		Status:         overallStatus,
	}

	if err := writeVerdictArtifacts(*outDir, v, allSamples, sampleInterval); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if overallStatus != "PASS" {
		os.Exit(1)
	}
}
