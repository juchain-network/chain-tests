package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type runtimeCaseSupportReport struct {
	Topology       string `json:"topology"`
	Mode           string `json:"mode"`
	Target         string `json:"target"`
	RequiredFork   string `json:"required_fork"`
	NetworkMaxFork string `json:"network_max_fork"`
	Supported      bool   `json:"supported"`
	Nodes          []struct {
		Node    string `json:"node"`
		Impl    string `json:"impl"`
		Binary  string `json:"binary"`
		Version string `json:"version"`
		MaxFork string `json:"max_fork"`
	} `json:"nodes"`
}

func writeFakeBinary(t *testing.T, dir, name, output string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := fmt.Sprintf("#!/bin/sh\ncat <<'EOF'\n%s\nEOF\n", output)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary %s failed: %v", name, err)
	}
	return path
}

func runRuntimeCaseSupportReport(t *testing.T, cfgPath, topology, mode, target string) runtimeCaseSupportReport {
	t.Helper()
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}

	cmd := exec.Command(
		"bash", "-lc",
		fmt.Sprintf(
			"cd %q && source scripts/network/lib.sh && runtime_case_support_report %q %q %q %q",
			repoRoot, cfgPath, topology, mode, target,
		),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime_case_support_report failed: %v output=%s", err, string(out))
	}

	var report runtimeCaseSupportReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("unmarshal support report failed: %v raw=%s", err, string(out))
	}
	return report
}

func TestRuntimeCaseSupportReportMatchesWildcardVersionMatrix(t *testing.T) {
	tmp := t.TempDir()
	gethBinary := writeFakeBinary(t, tmp, "geth", "Geth\nVersion: 1.16.8-stable")
	cfgPath := filepath.Join(tmp, "config.yaml")
	cfg := fmt.Sprintf(`runtime:
  impl_mode: single
  impl: geth
binaries:
  geth_native: %q
runtime_capability:
  version_matrix:
    geth:
      "1.13.x": posa
      "1.16.x": osaka
network:
  node_count: 1
`, gethBinary)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	report := runRuntimeCaseSupportReport(t, cfgPath, "single", "upgrade", "osakaTime")
	if !report.Supported {
		t.Fatalf("expected wildcard version support, got unsupported report=%+v", report)
	}
	if report.NetworkMaxFork != "osaka" {
		t.Fatalf("unexpected network_max_fork: %s", report.NetworkMaxFork)
	}
	if len(report.Nodes) != 1 || report.Nodes[0].Version != "1.16.8" {
		t.Fatalf("unexpected nodes report: %+v", report.Nodes)
	}
}

func TestRuntimeCaseSupportReportPrefersExactVersionOverWildcard(t *testing.T) {
	tmp := t.TempDir()
	gethBinary := writeFakeBinary(t, tmp, "geth", "Geth\nVersion: 1.16.8-stable")
	cfgPath := filepath.Join(tmp, "config.yaml")
	cfg := fmt.Sprintf(`runtime:
  impl_mode: single
  impl: geth
binaries:
  geth_native: %q
runtime_capability:
  version_matrix:
    geth:
      "1.16.x": prague
      "1.16.8": osaka
network:
  node_count: 1
`, gethBinary)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	report := runRuntimeCaseSupportReport(t, cfgPath, "single", "upgrade", "osakaTime")
	if !report.Supported {
		t.Fatalf("expected exact version override to support osaka, got unsupported report=%+v", report)
	}
	if report.NetworkMaxFork != "osaka" {
		t.Fatalf("unexpected network_max_fork: %s", report.NetworkMaxFork)
	}
}

func TestRuntimeCaseSupportReportUsesWeakestNodeInMixedTopology(t *testing.T) {
	tmp := t.TempDir()
	gethBinary := writeFakeBinary(t, tmp, "geth", "Geth\nVersion: 1.16.8-stable")
	rethBinary := writeFakeBinary(t, tmp, "congress-node", "reth Version: 1.1.0-dev")
	cfgPath := filepath.Join(tmp, "config.yaml")
	cfg := fmt.Sprintf(`runtime:
  impl_mode: mixed
  impl: geth
binaries:
  geth_native: %q
  reth_native: %q
runtime_nodes:
  node0:
    impl: geth
  node1:
    impl: reth
runtime_capability:
  version_matrix:
    geth:
      "1.16.x": osaka
    reth:
      "1.1.x": posa
network:
  node_count: 2
`, gethBinary, rethBinary)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	report := runRuntimeCaseSupportReport(t, cfgPath, "multi", "smoke", "poa_shanghai_cancun_fixheader_posa_prague_osaka")
	if report.Supported {
		t.Fatalf("expected mixed topology weakest-node gating to reject osaka, got report=%+v", report)
	}
	if report.NetworkMaxFork != "posa" {
		t.Fatalf("unexpected weakest network fork: %s", report.NetworkMaxFork)
	}
	if len(report.Nodes) != 2 {
		t.Fatalf("unexpected nodes report: %+v", report.Nodes)
	}
}
