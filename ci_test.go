package main

import (
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

	pass, fail, skip, timed := parseTestOutput(logPath)
	if len(pass) != 1 || !strings.Contains(pass[0], "TestA_SystemConfigSetup") {
		t.Fatalf("unexpected pass cases: %#v", pass)
	}
	if len(fail) != 1 || !strings.Contains(fail[0], "TestB_ConfigBoundaryChecks") {
		t.Fatalf("unexpected fail cases: %#v", fail)
	}
	if len(skip) != 1 || !strings.Contains(skip[0], "TestC_Skipped") {
		t.Fatalf("unexpected skip cases: %#v", skip)
	}
	if len(timed) != 3 {
		t.Fatalf("unexpected timed case count: %d", len(timed))
	}
	if timed[1].Status != "FAIL" {
		t.Fatalf("expected second timed case status FAIL, got %s", timed[1].Status)
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
	if !strings.Contains(text, "| 1 | TestB | 6s | PASS | group_config |") {
		t.Fatalf("expected sorted slow case row not found:\n%s", text)
	}
	if !strings.Contains(text, "## Group Runtime Profile") {
		t.Fatalf("group runtime profile section not found in report")
	}
	if !strings.Contains(text, "| config | group_config | 10s | 8s | EXCEEDED |") {
		t.Fatalf("group threshold row not found in report:\n%s", text)
	}
	if !strings.Contains(text, "## Slow Alerts (>= 5s)") {
		t.Fatalf("slow alerts section not found in report")
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
