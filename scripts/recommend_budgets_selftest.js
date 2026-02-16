#!/usr/bin/env node

const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");
const assert = require("assert");

function runNode(scriptPath, args, opts = {}) {
  const res = spawnSync(process.execPath, [scriptPath, ...args], {
    encoding: "utf8",
    ...opts,
  });
  return res;
}

function writeFile(filePath, content) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, content, "utf8");
}

function createFixture(rootDir) {
  const reportsDir = path.join(rootDir, "reports");
  const runDir = path.join(reportsDir, "ci_20260101_000001");
  fs.mkdirSync(runDir, { recursive: true });

  const report = `# Integration Test Report

| Step | Status | Duration | Skips | Log |
| --- | --- | --- | --- | --- |
| group_config | PASS | 1m0s | - | test_group_config.log |
| test_TestB_Governance | PASS | 2m0s | - | test_TestB_Governance.log |

## Slow Tests (Top 20)

| Rank | Test | Duration | Status | Step |
| --- | --- | --- | --- | --- |
| 1 | TestB_Governance | 40s | PASS | test_TestB_Governance |
| 2 | TestA_SystemConfigSetup | 12s | PASS | group_config |
`;
  writeFile(path.join(runDir, "report.md"), report);

  const log1 = `>>> go test ./tests/config -v
--- PASS: TestA_SystemConfigSetup (12.00s)
PASS
`;
  const log2 = `>>> go test ./tests/governance -v
--- PASS: TestB_Governance (40.00s)
PASS
`;
  writeFile(path.join(runDir, "test_group_config.log"), log1);
  writeFile(path.join(runDir, "test_TestB_Governance.log"), log2);

  return reportsDir;
}

function testJsonOutput(scriptPath, reportsDir) {
  const res = runNode(scriptPath, [
    "--reports-dir", reportsDir,
    "--recent", "10",
    "--group-quantile", "0.9",
    "--group-headroom", "1.3",
    "--slow-quantile", "0.9",
    "--slow-headroom", "1.4",
    "--min-group-samples", "1",
    "--current-group-thresholds", "config=10m,governance=10m,default=10m",
    "--current-slow-threshold", "1m",
    "--current-test-slow-threshold", "30s",
    "--drift-ratio", "0.2",
    "--drift-min-ms", "1000",
    "--format", "json",
  ]);
  assert.strictEqual(res.status, 0, `json mode failed: ${res.stderr || res.stdout}`);
  const body = JSON.parse(res.stdout);
  assert.strictEqual(body.reportsAnalyzed, 1);
  assert.strictEqual(body.recommendation.groupThresholds.config, 90000); // 1m * 1.3 => 78s => round 15s => 90s
  assert.strictEqual(body.recommendation.groupThresholds.governance, 180000); // 2m * 1.3 => 156s => round 30s => 180s
  assert.strictEqual(body.recommendation.defaultGroupThreshold, 180000);
  assert.strictEqual(body.recommendation.slowThreshold, 55000); // q90(12s,40s)=37.2s -> *1.4=52.08s -> round 5s => 55s
  assert.strictEqual(body.recommendation.testSlowThreshold, 27500);
  assert.ok(body.drift.alertCount > 0, "expected drift alerts with oversized current thresholds");
}

function testMakeOutput(scriptPath, reportsDir) {
  const res = runNode(scriptPath, [
    "--reports-dir", reportsDir,
    "--recent", "10",
    "--min-group-samples", "1",
    "--format", "make",
  ]);
  assert.strictEqual(res.status, 0, `make mode failed: ${res.stderr || res.stdout}`);
  const lines = res.stdout.trim().split(/\r?\n/).filter(Boolean);
  assert.ok(lines.length >= 3, "expected at least 3 CI_BUDGET lines");
  for (const line of lines) {
    assert.ok(line.startsWith("CI_BUDGET_"), `non-variable line found in make output: ${line}`);
  }
}

function testFailOnDrift(scriptPath, reportsDir) {
  const res = runNode(scriptPath, [
    "--reports-dir", reportsDir,
    "--recent", "10",
    "--min-group-samples", "1",
    "--current-group-thresholds", "config=10m,governance=10m,default=10m",
    "--current-slow-threshold", "1m",
    "--current-test-slow-threshold", "30s",
    "--drift-ratio", "0.2",
    "--drift-min-ms", "1000",
    "--fail-on-drift",
  ]);
  assert.strictEqual(res.status, 2, `expected exit 2 on drift, got ${res.status}; output=${res.stdout}\n${res.stderr}`);
}

function main() {
  const scriptPath = path.join(__dirname, "recommend_budgets.js");
  const tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), "budget-selftest-"));
  try {
    const reportsDir = createFixture(tmpRoot);
    testJsonOutput(scriptPath, reportsDir);
    testMakeOutput(scriptPath, reportsDir);
    testFailOnDrift(scriptPath, reportsDir);
    console.log("recommend_budgets_selftest: PASS");
  } finally {
    fs.rmSync(tmpRoot, { recursive: true, force: true });
  }
}

try {
  main();
} catch (err) {
  console.error(`recommend_budgets_selftest: FAIL\n${err.stack || err.message}`);
  process.exit(1);
}
