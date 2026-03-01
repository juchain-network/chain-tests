package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	intcfg "juchain.org/chain/tools/ci/internal/config"
)

type sample struct {
	Timestamp        time.Time
	TierTPS          int
	Sent             int64
	Accepted         int64
	Failed           int64
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
	Failed              int64   `json:"failed"`
	SuccessRate         float64 `json:"success_rate"`
	StartHeight         uint64  `json:"start_height"`
	EndHeight           uint64  `json:"end_height"`
	HeightGrowth        uint64  `json:"height_growth"`
	MaxHeightLag        uint64  `json:"max_height_lag"`
	P95RPCLatencyMS     float64 `json:"p95_rpc_latency_ms"`
	MaxConsecutiveStall int     `json:"max_consecutive_stall"`
	Status              string  `json:"status"`
	Notes               string  `json:"notes,omitempty"`
}

type verdict struct {
	GeneratedAt    string         `json:"generated_at"`
	Mode           string         `json:"mode"`
	Config         string         `json:"config"`
	DataDir        string         `json:"data_dir"`
	Thresholds     map[string]any `json:"thresholds"`
	Tiers          []tierSummary  `json:"tiers"`
	FailedReasons  []string       `json:"failed_reasons,omitempty"`
	TopSlowWindows []slowWindow   `json:"top_slow_windows,omitempty"`
	ResourcePeaks  resourcePeaks  `json:"resource_peaks"`
	Status         string         `json:"status"`
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
	psCmd := exec.Command("/bin/sh", "-lc", `ps -Ao comm,pcpu,rss | awk '/(geth|juchain)/ {cpu+=$2; rss+=$3} END {printf "%.3f %.3f", cpu, rss/1024.0}'`)
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
		"timestamp", "tier_tps", "sent", "accepted", "failed", "primary_height", "max_height", "min_height", "height_lag", "rpc_latency_ms", "cpu_percent", "memory_mb", "disk_mb", "consecutive_stall",
	})
	for _, s := range samples {
		_ = w.Write([]string{
			s.Timestamp.Format(time.RFC3339),
			strconv.Itoa(s.TierTPS),
			strconv.FormatInt(s.Sent, 10),
			strconv.FormatInt(s.Accepted, 10),
			strconv.FormatInt(s.Failed, 10),
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

	b.WriteString("## Resource Peaks\n\n")
	b.WriteString("| CPU(%) | Memory(MB) | Disk(MB) |\n")
	b.WriteString("| --- | --- | --- |\n")
	b.WriteString(fmt.Sprintf("| %.3f | %.3f | %.3f |\n\n", v.ResourcePeaks.CPUPercent, v.ResourcePeaks.MemoryMB, v.ResourcePeaks.DiskMB))

	b.WriteString("## Tier Summary\n\n")
	b.WriteString("| TPS | Duration(s) | Sent | Accepted | Failed | SuccessRate | HeightGrowth | MaxLag | p95 RPC(ms) | MaxStall | Status |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, t := range v.Tiers {
		b.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d | %.4f | %d | %d | %.2f | %d | %s |\n",
			t.TPS, t.DurationSeconds, t.Sent, t.Accepted, t.Failed, t.SuccessRate, t.HeightGrowth, t.MaxHeightLag, t.P95RPCLatencyMS, t.MaxConsecutiveStall, t.Status,
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

func main() {
	configPath := flag.String("config", "data/test_config.yaml", "Path to generated test config")
	outDir := flag.String("out-dir", "reports/perf", "Output directory")
	mode := flag.String("mode", "tiers", "tiers|soak")
	tiersRaw := flag.String("tiers", "10,30,60", "Comma-separated TPS tiers")
	durationRaw := flag.String("duration", "90s", "Duration per tier")
	sampleIntervalRaw := flag.String("sample-interval", "2s", "Metrics sample interval")
	dataDir := flag.String("data-dir", "data", "Chain data directory for disk usage sampling")
	minSuccessRate := flag.Float64("min-success-rate", 0.99, "Minimum accepted tx success rate")
	maxStallSeconds := flag.Int("max-stall-seconds", 15, "Maximum consecutive no-growth sample window")
	maxHeightLag := flag.Uint64("max-height-lag", 8, "Maximum allowed node height lag")
	maxP95LatencyMS := flag.Float64("max-p95-latency-ms", 500, "Maximum allowed p95 RPC latency")
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

	tiers, err := parseTPSList(*tiersRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tiers parse failed: %v\n", err)
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

	// Include explicit node RPCs for lag checks if provided.
	lagClients := append([]*ethclient.Client{}, clients...)
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
		}
		defer func() {
			for _, c := range lagClients {
				c.Close()
			}
		}()
	}

	chainID, err := clients[0].ChainID(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "chain id query failed: %v\n", err)
		os.Exit(1)
	}

	from := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := clients[0].PendingNonceAt(context.Background(), from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pending nonce query failed: %v\n", err)
		os.Exit(1)
	}

	allSamples := make([]sample, 0, 4096)
	summaries := make([]tierSummary, 0, len(tiers))

	for _, tps := range tiers {
		interval := time.Second
		if tps > 0 {
			interval = time.Duration(float64(time.Second) / float64(tps))
			if interval <= 0 {
				interval = time.Millisecond
			}
		}

		start := time.Now()
		end := start.Add(duration)
		txTicker := time.NewTicker(interval)
		sampleTicker := time.NewTicker(sampleInterval)
		defer txTicker.Stop()
		defer sampleTicker.Stop()

		var sent, accepted, failed int64
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

		for time.Now().Before(end) {
			select {
			case <-txTicker.C:
				sent++
				gasPrice, gerr := clients[0].SuggestGasPrice(context.Background())
				if gerr != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
					gasPrice = big.NewInt(1_000_000_000)
				}
				tx := types.NewTransaction(nonce, from, big.NewInt(0), 21_000, gasPrice, nil)
				signed, serr := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
				if serr != nil {
					failed++
					continue
				}
				startSend := time.Now()
				errSend := clients[0].SendTransaction(context.Background(), signed)
				latencies = append(latencies, float64(time.Since(startSend).Milliseconds()))
				if errSend != nil {
					failed++
					msg := strings.ToLower(errSend.Error())
					if strings.Contains(msg, "nonce too low") || strings.Contains(msg, "replacement transaction underpriced") || strings.Contains(msg, "already known") {
						if refreshed, rerr := clients[0].PendingNonceAt(context.Background(), from); rerr == nil {
							nonce = refreshed
						}
					}
					continue
				}
				accepted++
				nonce++
			case <-sampleTicker.C:
				probeStart := time.Now()
				maxHeight, minHeight, primaryHeight, errHeights := queryHeights(lagClients)
				latMS := float64(time.Since(probeStart).Milliseconds())
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
				cpu, memMB, diskMB := sampleResources(*dataDir)
				allSamples = append(allSamples, sample{
					Timestamp:        time.Now().UTC(),
					TierTPS:          tps,
					Sent:             sent,
					Accepted:         accepted,
					Failed:           failed,
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
			}
		}

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

		successRate := 0.0
		if sent > 0 {
			successRate = float64(accepted) / float64(sent)
		}
		p95 := percentile95(latencies)
		status := "PASS"
		noteParts := []string{}
		if successRate < *minSuccessRate {
			status = "FAIL"
			noteParts = append(noteParts, fmt.Sprintf("success_rate %.4f < %.4f", successRate, *minSuccessRate))
		}
		if maxLag > *maxHeightLag {
			status = "FAIL"
			noteParts = append(noteParts, fmt.Sprintf("max_height_lag %d > %d", maxLag, *maxHeightLag))
		}
		if p95 > *maxP95LatencyMS {
			status = "FAIL"
			noteParts = append(noteParts, fmt.Sprintf("p95_latency %.2f > %.2f", p95, *maxP95LatencyMS))
		}
		if maxConsecutiveStall*int(sampleInterval.Seconds()) > *maxStallSeconds {
			status = "FAIL"
			noteParts = append(noteParts, fmt.Sprintf("stall_window %ds > %ds", maxConsecutiveStall*int(sampleInterval.Seconds()), *maxStallSeconds))
		}

		summaries = append(summaries, tierSummary{
			TPS:                 tps,
			DurationSeconds:     int64(duration.Seconds()),
			Sent:                sent,
			Accepted:            accepted,
			Failed:              failed,
			SuccessRate:         successRate,
			StartHeight:         startHeight,
			EndHeight:           endHeight,
			HeightGrowth:        endHeight - startHeight,
			MaxHeightLag:        maxLag,
			P95RPCLatencyMS:     p95,
			MaxConsecutiveStall: maxConsecutiveStall,
			Status:              status,
			Notes:               strings.Join(noteParts, "; "),
		})
	}

	failedReasons := collectFailedReasons(summaries)
	overallStatus := "PASS"
	if len(failedReasons) > 0 {
		overallStatus = "FAIL"
	}

	v := verdict{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Mode:        *mode,
		Config:      *configPath,
		DataDir:     *dataDir,
		Thresholds: map[string]any{
			"success_rate_min":         *minSuccessRate,
			"stall_window_seconds_max": *maxStallSeconds,
			"max_height_lag":           *maxHeightLag,
			"rpc_latency_p95_ms_max":   *maxP95LatencyMS,
		},
		Tiers:          summaries,
		FailedReasons:  failedReasons,
		TopSlowWindows: collectTopSlowWindows(allSamples, sampleInterval, 10),
		ResourcePeaks:  collectResourcePeaks(allSamples),
		Status:         overallStatus,
	}

	metricsCSV := filepath.Join(*outDir, "metrics.csv")
	if err := writeMetricsCSV(metricsCSV, allSamples); err != nil {
		fmt.Fprintf(os.Stderr, "write metrics csv failed: %v\n", err)
		os.Exit(1)
	}

	verdictPath := filepath.Join(*outDir, "verdict.json")
	data, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(verdictPath, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write verdict failed: %v\n", err)
		os.Exit(1)
	}

	summaryPath := filepath.Join(*outDir, "summary.md")
	if err := writeSummaryMD(summaryPath, v); err != nil {
		fmt.Fprintf(os.Stderr, "write summary failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("summary: %s\n", summaryPath)
	fmt.Printf("metrics: %s\n", metricsCSV)
	fmt.Printf("verdict: %s\n", verdictPath)
	if overallStatus != "PASS" {
		os.Exit(1)
	}
}
