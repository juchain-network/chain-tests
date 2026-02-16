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

	if err := writeReport(reportPath, "groups", "config", "", "", "data/test_config.yaml", "/tmp/go-build", false, results); err != nil {
		t.Fatalf("writeReport failed: %v", err)
	}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "## Slow Tests (Top 20)") {
		t.Fatalf("slow tests section not found in report")
	}
	if !strings.Contains(text, "| 1 | TestB | 6s | PASS | group_config |") {
		t.Fatalf("expected sorted slow case row not found:\n%s", text)
	}
}
