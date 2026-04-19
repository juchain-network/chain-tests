package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTimedCase(t *testing.T) {
	tc, ok := parseTimedCase("TestA_SystemConfigSetup (1.23s)", "PASS")
	if !ok {
		t.Fatalf("expected parseTimedCase to parse valid duration")
	}
	if tc.Name != "TestA_SystemConfigSetup" {
		t.Fatalf("unexpected test name: %s", tc.Name)
	}
	if tc.Duration != 1230*time.Millisecond {
		t.Fatalf("unexpected duration: %s", tc.Duration)
	}
	if tc.Status != "PASS" {
		t.Fatalf("unexpected status: %s", tc.Status)
	}
}

func TestDisplayStatus(t *testing.T) {
	cases := map[string]string{
		"PASS":     "🟢 PASS",
		"FAIL":     "🔴 FAIL",
		"SKIP":     "🟡 SKIP",
		"DEFERRED": "🟠 DEFERRED",
	}
	for raw, want := range cases {
		if got := displayStatus(raw); got != want {
			t.Fatalf("displayStatus(%q)=%q want %q", raw, got, want)
		}
	}
}

func TestParseTestOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "go_test.log")
	content := strings.Join([]string{
		"=== RUN   TestA_SystemConfigSetup",
		"--- PASS: TestA_SystemConfigSetup (1.23s)",
		"=== RUN   TestB_ConfigBoundaryChecks",
		"--- FAIL: TestB_ConfigBoundaryChecks (2.50s)",
		"=== RUN   TestC_Skipped",
		"--- SKIP: TestC_Skipped (0.01s)",
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	pass, fail, skip, deferred, timed := parseTestOutput(logPath)
	if len(pass) != 1 || !strings.Contains(pass[0], "TestA_SystemConfigSetup") {
		t.Fatalf("unexpected pass cases: %#v", pass)
	}
	if len(fail) != 1 || !strings.Contains(fail[0], "TestB_ConfigBoundaryChecks") {
		t.Fatalf("unexpected fail cases: %#v", fail)
	}
	if len(skip) != 1 || !strings.Contains(skip[0], "TestC_Skipped") {
		t.Fatalf("unexpected skip cases: %#v", skip)
	}
	if len(deferred) != 0 {
		t.Fatalf("unexpected deferred cases: %#v", deferred)
	}
	if len(timed) != 3 {
		t.Fatalf("unexpected timed case count: %d", len(timed))
	}
	if timed[1].Status != "FAIL" {
		t.Fatalf("expected second timed case status FAIL, got %s", timed[1].Status)
	}
}

func TestParseTestOutputHandlesIndentedAndProgressPrefixedLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "go_test_prefixed.log")
	content := strings.Join([]string{
		".......--- PASS: TestF2_QuickReEntry (22.96s)",
		"    --- PASS: TestF4_MiscExit/P-09_MinerOnlyPunish (6.14s)",
		"..--- SKIP: TestG_PunishPaths/P-24_ExecutePendingAutoByConsensus (195.28s)",
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write prefixed test log: %v", err)
	}

	pass, fail, skip, deferred, timed := parseTestOutput(logPath)
	if len(pass) != 2 {
		t.Fatalf("unexpected pass case count: %#v", pass)
	}
	if len(fail) != 0 {
		t.Fatalf("unexpected fail cases: %#v", fail)
	}
	if len(skip) != 1 {
		t.Fatalf("unexpected skip cases: %#v", skip)
	}
	if len(deferred) != 0 {
		t.Fatalf("unexpected deferred cases: %#v", deferred)
	}
	if pass[0] != "TestF2_QuickReEntry (22.96s)" {
		t.Fatalf("unexpected first pass case: %q", pass[0])
	}
	if pass[1] != "TestF4_MiscExit/P-09_MinerOnlyPunish (6.14s)" {
		t.Fatalf("unexpected second pass case: %q", pass[1])
	}
	if skip[0] != "TestG_PunishPaths/P-24_ExecutePendingAutoByConsensus (195.28s)" {
		t.Fatalf("unexpected skip case: %q", skip[0])
	}
	if len(timed) != 3 {
		t.Fatalf("unexpected timed case count: %d", len(timed))
	}
}

func TestParseTestOutputHandlesNestedCIRenderedStatusLines(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nested_ci.log")
	content := strings.Join([]string{
		"  🟢 PASS: TestF1_ExitFlow (28.80s)",
		"  🔴 FAIL: TestB_ConfigBoundaryChecks (2.50s)",
		"  🟡 SKIP: TestG_DoubleSign/P-23_MultiValidatorDoubleSign (98.16s)",
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write nested ci log: %v", err)
	}

	pass, fail, skip, deferred, timed := parseTestOutput(logPath)
	if len(pass) != 1 || pass[0] != "TestF1_ExitFlow (28.80s)" {
		t.Fatalf("unexpected pass cases: %#v", pass)
	}
	if len(fail) != 1 || fail[0] != "TestB_ConfigBoundaryChecks (2.50s)" {
		t.Fatalf("unexpected fail cases: %#v", fail)
	}
	if len(skip) != 1 || skip[0] != "TestG_DoubleSign/P-23_MultiValidatorDoubleSign (98.16s)" {
		t.Fatalf("unexpected skip cases: %#v", skip)
	}
	if len(deferred) != 0 {
		t.Fatalf("unexpected deferred cases: %#v", deferred)
	}
	if len(timed) != 3 {
		t.Fatalf("unexpected timed case count: %d", len(timed))
	}
}

func TestParseTestOutputSeparatesDeferredFromSkip(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "go_test_deferred.log")
	content := strings.Join([]string{
		"=== RUN   TestK_ForkcapCapability_BlobTxDeferred",
		"    blob_placeholder_test.go:16: Deferred: blob transactions are temporarily disabled at the txpool layer on this chain.",
		"--- SKIP: TestK_ForkcapCapability_BlobTxDeferred (0.00s)",
		"=== RUN   TestOther_Skipped",
		"--- SKIP: TestOther_Skipped (0.01s)",
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write deferred test log: %v", err)
	}

	pass, fail, skip, deferred, timed := parseTestOutput(logPath)
	if len(pass) != 0 || len(fail) != 0 {
		t.Fatalf("unexpected pass/fail cases: pass=%#v fail=%#v", pass, fail)
	}
	if len(skip) != 1 || skip[0] != "TestOther_Skipped (0.01s)" {
		t.Fatalf("unexpected skip cases: %#v", skip)
	}
	if len(deferred) != 1 || deferred[0] != "TestK_ForkcapCapability_BlobTxDeferred (0.00s)" {
		t.Fatalf("unexpected deferred cases: %#v", deferred)
	}
	if len(timed) != 2 {
		t.Fatalf("unexpected timed case count: %d", len(timed))
	}
	if timed[0].Status != "DEFERRED" {
		t.Fatalf("expected first timed case status DEFERRED, got %s", timed[0].Status)
	}
}

func TestAttributeImplicitFailures(t *testing.T) {
	fail := attributeImplicitFailures("FAIL", nil, nil, "TestG_PunishPaths/P-24_ExecutePendingAutoByConsensus", "test_run_pattern")
	if len(fail) != 1 || fail[0] != "TestG_PunishPaths/P-24_ExecutePendingAutoByConsensus" {
		t.Fatalf("unexpected implicit failures: %#v", fail)
	}

	fail = attributeImplicitFailures("FAIL", nil, []string{"TestA_SystemConfigSetup"}, "", "test_TestA_SystemConfigSetup")
	if len(fail) != 1 || fail[0] != "TestA_SystemConfigSetup" {
		t.Fatalf("unexpected test fallback failures: %#v", fail)
	}
}

func TestCollectCasesFromSummaryUsesFallbackCounts(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "inner.log")
	if err := os.WriteFile(logPath, []byte("panic: test timed out after 30m0s\n"), 0o644); err != nil {
		t.Fatalf("failed to write inner log: %v", err)
	}

	summaryPath := filepath.Join(dir, "summary.json")
	summary := runSummary{
		RunPattern: "TestG_PunishPaths/P-24_ExecutePendingAutoByConsensus",
		Steps: []summaryStep{
			{
				Name:      "test_run_pattern",
				Status:    "FAIL",
				FailCount: 1,
				LogPath:   logPath,
			},
		},
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal summary: %v", err)
	}
	if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
		t.Fatalf("failed to write summary: %v", err)
	}

	pass, fail, skip, deferred, timed, err := collectCasesFromSummary(summaryPath)
	if err != nil {
		t.Fatalf("collectCasesFromSummary failed: %v", err)
	}
	if len(pass) != 0 || len(skip) != 0 || len(deferred) != 0 {
		t.Fatalf("unexpected pass/skip/deferred cases: pass=%#v skip=%#v deferred=%#v", pass, skip, deferred)
	}
	if len(fail) != 1 || !strings.Contains(fail[0], "TestG_PunishPaths/P-24_ExecutePendingAutoByConsensus") {
		t.Fatalf("unexpected fail cases: %#v", fail)
	}
	if len(timed) != 0 {
		t.Fatalf("unexpected timed cases: %#v", timed)
	}
}

func TestCollectNestedSummaryCasesAggregatesMultipleInnerRuns(t *testing.T) {
	dir := t.TempDir()
	childLog1 := filepath.Join(dir, "child1.log")
	childLog2 := filepath.Join(dir, "child2.log")
	if err := os.WriteFile(childLog1, []byte("--- PASS: TestA_SystemConfigSetup (1.23s)\n"), 0o644); err != nil {
		t.Fatalf("failed to write child log1: %v", err)
	}
	if err := os.WriteFile(childLog2, []byte("--- SKIP: TestZ_LastManStanding (0.00s)\n"), 0o644); err != nil {
		t.Fatalf("failed to write child log2: %v", err)
	}

	childSummary1 := filepath.Join(dir, "child1-summary.json")
	childSummary2 := filepath.Join(dir, "child2-summary.json")
	for _, item := range []struct {
		path    string
		status  string
		logPath string
		pass    int
		skip    int
	}{
		{path: childSummary1, status: "PASS", logPath: childLog1, pass: 1},
		{path: childSummary2, status: "PASS", logPath: childLog2, skip: 1},
	} {
		summary := runSummary{
			Steps: []summaryStep{
				{
					Name:      "test_run_pattern",
					Status:    item.status,
					LogPath:   item.logPath,
					PassCount: item.pass,
					SkipCount: item.skip,
				},
			},
		}
		data, err := json.Marshal(summary)
		if err != nil {
			t.Fatalf("failed to marshal child summary: %v", err)
		}
		if err := os.WriteFile(item.path, data, 0o644); err != nil {
			t.Fatalf("failed to write child summary: %v", err)
		}
	}

	parentLog := filepath.Join(dir, "parent.log")
	parentContent := strings.Join([]string{
		"Summary: " + childSummary1,
		"Summary: " + childSummary2,
	}, "\n")
	if err := os.WriteFile(parentLog, []byte(parentContent), 0o644); err != nil {
		t.Fatalf("failed to write parent log: %v", err)
	}

	pass, fail, skip, deferred, timed, ok := collectNestedSummaryCases(parentLog)
	if !ok {
		t.Fatalf("expected nested summary aggregation to succeed")
	}
	if len(pass) != 1 || pass[0] != "TestA_SystemConfigSetup (1.23s)" {
		t.Fatalf("unexpected pass cases: %#v", pass)
	}
	if len(fail) != 0 {
		t.Fatalf("unexpected fail cases: %#v", fail)
	}
	if len(skip) != 1 || skip[0] != "TestZ_LastManStanding (0.00s)" {
		t.Fatalf("unexpected skip cases: %#v", skip)
	}
	if len(deferred) != 0 {
		t.Fatalf("unexpected deferred cases: %#v", deferred)
	}
	if len(timed) != 2 {
		t.Fatalf("unexpected timed cases: %#v", timed)
	}
}

func TestCollectSlowCasesSortByDurationDesc(t *testing.T) {
	results := []stepResult{
		{
			Name: "group_config",
			Timed: []timedCase{
				{Name: "TestA", Duration: 2 * time.Second, Status: "PASS", Step: "group_config"},
				{Name: "TestB", Duration: 5 * time.Second, Status: "PASS", Step: "group_config"},
			},
		},
		{
			Name: "group_rewards",
			Timed: []timedCase{
				{Name: "TestC", Duration: 3 * time.Second, Status: "FAIL", Step: "group_rewards"},
			},
		},
	}

	slow := collectSlowCases(results)
	if len(slow) != 3 {
		t.Fatalf("unexpected slow case count: %d", len(slow))
	}
	if slow[0].Name != "TestB" || slow[1].Name != "TestC" || slow[2].Name != "TestA" {
		t.Fatalf("unexpected order: %#v", slow)
	}
}

func TestParseDurationFlag(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
	}{
		{raw: "0", want: 0},
		{raw: "off", want: 0},
		{raw: "500ms", want: 500 * time.Millisecond},
		{raw: "2s", want: 2 * time.Second},
	}
	for _, c := range cases {
		got, err := parseDurationFlag(c.raw)
		if err != nil {
			t.Fatalf("parseDurationFlag(%q) failed: %v", c.raw, err)
		}
		if got != c.want {
			t.Fatalf("parseDurationFlag(%q)=%s want %s", c.raw, got, c.want)
		}
	}
	if _, err := parseDurationFlag("bad"); err == nil {
		t.Fatalf("expected parseDurationFlag to fail for invalid duration")
	}
}

func TestParseGroupThresholds(t *testing.T) {
	thresholds, defaultVal, err := parseGroupThresholds("config=2m,rewards=3m,default=5m")
	if err != nil {
		t.Fatalf("parseGroupThresholds failed: %v", err)
	}
	if thresholds["config"] != 2*time.Minute {
		t.Fatalf("unexpected config threshold: %s", thresholds["config"])
	}
	if thresholds["rewards"] != 3*time.Minute {
		t.Fatalf("unexpected rewards threshold: %s", thresholds["rewards"])
	}
	if defaultVal != 5*time.Minute {
		t.Fatalf("unexpected default threshold: %s", defaultVal)
	}
	if _, _, err := parseGroupThresholds("config2m"); err == nil {
		t.Fatalf("expected parseGroupThresholds to fail for invalid expression")
	}
}

func TestEnvTruthy(t *testing.T) {
	trueValues := []string{"1", "true", "TRUE", " yes ", "on"}
	for _, v := range trueValues {
		if !envTruthy(v) {
			t.Fatalf("expected envTruthy(%q)=true", v)
		}
	}
	falseValues := []string{"", "0", "false", "no", "off", "random"}
	for _, v := range falseValues {
		if envTruthy(v) {
			t.Fatalf("expected envTruthy(%q)=false", v)
		}
	}
}

func TestCollectGroupDurations(t *testing.T) {
	results := []stepResult{
		{Name: "group_config", Duration: 90 * time.Second},
		{Name: "group_rewards", Duration: 4 * time.Minute},
		{Name: "test_run_pattern", Duration: 5 * time.Second},
	}
	items := collectGroupDurations(results, map[string]time.Duration{
		"config": 2 * time.Minute,
	}, 3*time.Minute)

	if len(items) != 2 {
		t.Fatalf("unexpected group duration item count: %d", len(items))
	}
	if items[0].Group != "rewards" || !items[0].Exceeded {
		t.Fatalf("expected rewards group to exceed default threshold: %#v", items[0])
	}
	if items[1].Group != "config" || items[1].Exceeded {
		t.Fatalf("expected config group within threshold: %#v", items[1])
	}
}

func TestGroupMakeArgsUsesUnifiedEntryPoint(t *testing.T) {
	args := groupMakeArgs("config")
	if len(args) != 2 {
		t.Fatalf("unexpected arg count: %#v", args)
	}
	if args[0] != "test-group" {
		t.Fatalf("unexpected target: %#v", args)
	}
	if args[1] != "GROUP=config" {
		t.Fatalf("unexpected group selector: %#v", args)
	}
}

func TestWriteReportIncludesSlowTestsSection(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "report.md")
	results := []stepResult{
		{
			Name:     "group_config",
			Command:  "go test ./tests/config",
			LogPath:  "reports/sample.log",
			Status:   "PASS",
			Duration: 10 * time.Second,
			Timed: []timedCase{
				{Name: "TestA", Duration: 4 * time.Second, Status: "PASS", Step: "group_config"},
				{Name: "TestB", Duration: 6 * time.Second, Status: "PASS", Step: "group_config"},
			},
		},
	}

	stats, err := writeReport(reportPath, "groups", "config", "", "", "data/test_config.yaml", "/tmp/go-build", false, results, reportOptions{
		SlowTop:       10,
		SlowThreshold: 5 * time.Second,
		GroupThresholds: map[string]time.Duration{
			"config": 8 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("writeReport failed: %v", err)
	}
	if stats.SlowCaseAlerts != 1 {
		t.Fatalf("unexpected slow alert count: %d", stats.SlowCaseAlerts)
	}
	if stats.GroupAlerts != 1 {
		t.Fatalf("unexpected group alert count: %d", stats.GroupAlerts)
	}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "## Slow Tests (Top 10)") {
		t.Fatalf("slow tests section not found in report")
	}
	if !strings.Contains(text, "| 1 | TestB | 6s | 🟢 PASS | group_config |") {
		t.Fatalf("expected sorted slow case row not found:\n%s", text)
	}
	if !strings.Contains(text, "## Group Runtime Profile") {
		t.Fatalf("group runtime profile section not found in report")
	}
	if !strings.Contains(text, "| config | group_config | 10s | 8s | EXCEEDED |") {
		t.Fatalf("group threshold row not found in report:\n%s", text)
	}
	if !strings.Contains(text, "- Totals: pass=0 fail=0 skip=0") {
		t.Fatalf("report totals line not found in report")
	}
	if !strings.Contains(text, "## Slow Alerts (>= 5s)") {
		t.Fatalf("slow alerts section not found in report")
	}
	if !strings.Contains(text, "- Status: 🟢 PASS") {
		t.Fatalf("details status line not decorated:\n%s", text)
	}
}

func TestBuildRunSummaryAccumulatesSkipCounts(t *testing.T) {
	results := []stepResult{
		{
			Name:      "group_punish",
			Status:    "PASS",
			PassTests: []string{"TestF1_ExitFlow (1.00s)"},
			SkipTests: []string{"TestG_DoubleSign/P-23_MultiValidatorDoubleSign (98.16s)"},
		},
	}

	summary := buildRunSummary(results, reportStats{}, "groups", "punish", "", "", "data/test_config.yaml", "reports/report.md", false)
	if summary.TotalPassTests != 1 {
		t.Fatalf("unexpected total pass count: %d", summary.TotalPassTests)
	}
	if summary.TotalSkipTests != 1 {
		t.Fatalf("unexpected total skip count: %d", summary.TotalSkipTests)
	}
	if len(summary.Steps) != 1 || summary.Steps[0].SkipCount != 1 {
		t.Fatalf("unexpected step skip counts: %#v", summary.Steps)
	}
}

func TestDiscoverTestTargetsMapsTestsToOwningPackage(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "tests", "epoch"), 0o755); err != nil {
		t.Fatalf("failed to create epoch test dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rootDir, "tests", "smoke"), 0o755); err != nil {
		t.Fatalf("failed to create smoke test dir: %v", err)
	}

	epochTest := []byte("package epoch\n\nfunc TestZ_SystemInitSecurityGuards(t *testing.T) {}\n")
	if err := os.WriteFile(filepath.Join(rootDir, "tests", "epoch", "system_init_security_test.go"), epochTest, 0o644); err != nil {
		t.Fatalf("failed to write epoch test file: %v", err)
	}

	smokeTest := []byte("package smoke\n\nfunc TestSmoke_ShouldBeSkipped(t *testing.T) {}\n")
	if err := os.WriteFile(filepath.Join(rootDir, "tests", "smoke", "smoke_test.go"), smokeTest, 0o644); err != nil {
		t.Fatalf("failed to write smoke test file: %v", err)
	}

	targets, err := discoverTestTargets(rootDir)
	if err != nil {
		t.Fatalf("discoverTestTargets failed: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("unexpected target count: %d", len(targets))
	}
	if targets[0].Name != "TestZ_SystemInitSecurityGuards" {
		t.Fatalf("unexpected target name: %#v", targets[0])
	}
	if targets[0].Package != "./tests/epoch" {
		t.Fatalf("unexpected target package: %#v", targets[0])
	}
}
