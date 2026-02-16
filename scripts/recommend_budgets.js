#!/usr/bin/env node

const fs = require("fs");
const path = require("path");

const knownGroups = [
  "config",
  "governance",
  "staking",
  "delegation",
  "punish",
  "rewards",
  "epoch",
];

function parseArgs(argv) {
  const out = {
    reportsDir: "reports",
    recent: 60,
    groupQuantile: 0.9,
    groupHeadroom: 1.3,
    slowQuantile: 0.9,
    slowHeadroom: 1.4,
    format: "human",
    currentGroupThresholds: "",
    currentSlowThreshold: "",
    currentTestSlowThreshold: "",
    driftRatio: 0.25,
    driftMinMs: 15000,
    failOnDrift: false,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    const take = () => argv[++i];
    switch (arg) {
      case "--reports-dir":
        out.reportsDir = take();
        break;
      case "--recent":
        out.recent = Number(take());
        break;
      case "--group-quantile":
        out.groupQuantile = Number(take());
        break;
      case "--group-headroom":
        out.groupHeadroom = Number(take());
        break;
      case "--slow-quantile":
        out.slowQuantile = Number(take());
        break;
      case "--slow-headroom":
        out.slowHeadroom = Number(take());
        break;
      case "--format":
        out.format = String(take() || "").trim().toLowerCase();
        break;
      case "--current-group-thresholds":
        out.currentGroupThresholds = take();
        break;
      case "--current-slow-threshold":
        out.currentSlowThreshold = take();
        break;
      case "--current-test-slow-threshold":
        out.currentTestSlowThreshold = take();
        break;
      case "--drift-ratio":
        out.driftRatio = Number(take());
        break;
      case "--drift-min-ms":
        out.driftMinMs = Number(take());
        break;
      case "--fail-on-drift":
        out.failOnDrift = true;
        break;
      case "-h":
      case "--help":
        printUsage();
        process.exit(0);
      default:
        throw new Error(`unknown argument: ${arg}`);
    }
  }
  validateArgs(out);
  return out;
}

function printUsage() {
  console.log("Usage: node scripts/recommend_budgets.js [options]");
  console.log("");
  console.log("Options:");
  console.log("  --reports-dir <path>      Reports directory (default: reports)");
  console.log("  --recent <n>              Number of recent ci_* reports to scan (default: 60)");
  console.log("  --group-quantile <q>      Quantile for group durations (default: 0.9)");
  console.log("  --group-headroom <m>      Headroom multiplier for groups (default: 1.3)");
  console.log("  --slow-quantile <q>       Quantile for slow test durations (default: 0.9)");
  console.log("  --slow-headroom <m>       Headroom multiplier for slow tests (default: 1.4)");
  console.log("  --format <human|make>     Output format (default: human)");
  console.log("  --current-group-thresholds <csv>  Current group thresholds for drift check");
  console.log("  --current-slow-threshold <dur>    Current group slow threshold for drift check");
  console.log("  --current-test-slow-threshold <dur> Current tests slow threshold for drift check");
  console.log("  --drift-ratio <r>         Relative drift threshold (default: 0.25)");
  console.log("  --drift-min-ms <n>        Absolute drift floor in ms (default: 15000)");
  console.log("  --fail-on-drift           Exit non-zero when drift alert is detected");
}

function validateArgs(args) {
  if (!Number.isFinite(args.recent) || args.recent <= 0) {
    throw new Error("--recent must be a positive number");
  }
  for (const [name, val] of [
    ["group-quantile", args.groupQuantile],
    ["slow-quantile", args.slowQuantile],
  ]) {
    if (!Number.isFinite(val) || val < 0 || val > 1) {
      throw new Error(`--${name} must be between 0 and 1`);
    }
  }
  for (const [name, val] of [
    ["group-headroom", args.groupHeadroom],
    ["slow-headroom", args.slowHeadroom],
  ]) {
    if (!Number.isFinite(val) || val <= 0) {
      throw new Error(`--${name} must be > 0`);
    }
  }
  if (!Number.isFinite(args.driftRatio) || args.driftRatio < 0) {
    throw new Error("--drift-ratio must be >= 0");
  }
  if (!Number.isFinite(args.driftMinMs) || args.driftMinMs < 0) {
    throw new Error("--drift-min-ms must be >= 0");
  }
  if (!["human", "make"].includes(args.format)) {
    throw new Error("--format must be one of: human, make");
  }
}

function listRecentReportFiles(reportsDir, recent) {
  let entries = [];
  try {
    entries = fs.readdirSync(reportsDir, { withFileTypes: true });
  } catch (err) {
    throw new Error(`failed to read reports dir ${reportsDir}: ${err.message}`);
  }

  return entries
    .filter((e) => e.isDirectory() && /^ci_\d{8}_\d{6}$/.test(e.name))
    .map((e) => path.join(reportsDir, e.name, "report.md"))
    .filter((p) => fs.existsSync(p))
    .sort((a, b) => path.basename(path.dirname(b)).localeCompare(path.basename(path.dirname(a))))
    .slice(0, recent);
}

function parseMarkdownRow(line) {
  const raw = line.trim();
  if (!raw.startsWith("|") || !raw.endsWith("|")) {
    return null;
  }
  return raw
    .split("|")
    .slice(1, -1)
    .map((v) => v.trim());
}

function parseDurationMs(raw) {
  const value = String(raw || "").trim().toLowerCase();
  if (!value || value === "-") {
    return null;
  }
  const re = /([0-9]*\.?[0-9]+)(ms|s|m|h)/g;
  let match = null;
  let total = 0;
  let found = false;
  while ((match = re.exec(value)) !== null) {
    found = true;
    const num = Number(match[1]);
    if (!Number.isFinite(num)) {
      return null;
    }
    switch (match[2]) {
      case "ms":
        total += num;
        break;
      case "s":
        total += num * 1000;
        break;
      case "m":
        total += num * 60 * 1000;
        break;
      case "h":
        total += num * 60 * 60 * 1000;
        break;
      default:
        return null;
    }
  }
  if (!found) {
    return null;
  }
  return total;
}

function parseThresholdCsv(raw) {
  const out = {
    groups: {},
    defaultThreshold: null,
  };
  const text = String(raw || "").trim();
  if (!text) {
    return out;
  }
  const items = text.split(",").map((v) => v.trim()).filter(Boolean);
  for (const item of items) {
    const parts = item.split("=");
    if (parts.length !== 2) {
      throw new Error(`invalid threshold item: ${item}`);
    }
    const key = parts[0].trim().toLowerCase();
    const dur = parseDurationMs(parts[1].trim());
    if (!key || dur === null) {
      throw new Error(`invalid threshold item: ${item}`);
    }
    if (key === "default") {
      out.defaultThreshold = dur;
    } else {
      out.groups[key] = dur;
    }
  }
  return out;
}

function parseOptionalDuration(raw, flagName) {
  const text = String(raw || "").trim();
  if (!text) {
    return null;
  }
  const ms = parseDurationMs(text);
  if (ms === null) {
    throw new Error(`invalid ${flagName}: ${raw}`);
  }
  return ms;
}

function parseDurationFromGoTestLine(line) {
  const raw = String(line || "").trim();
  const prefixes = ["--- PASS: ", "--- FAIL: ", "--- SKIP: "];
  for (const prefix of prefixes) {
    if (!raw.startsWith(prefix)) {
      continue;
    }
    const value = raw.slice(prefix.length).trim();
    const l = value.lastIndexOf(" (");
    const r = value.lastIndexOf(")");
    if (l <= 0 || r <= l + 2) {
      return null;
    }
    return parseDurationMs(value.slice(l + 2, r));
  }
  return null;
}

function extractRowsAfterHeader(lines, headerLine) {
  const idx = lines.findIndex((l) => l.trim() === headerLine);
  if (idx < 0) {
    return [];
  }
  const rows = [];
  for (let i = idx + 2; i < lines.length; i += 1) {
    const line = lines[i].trim();
    if (!line.startsWith("|")) {
      break;
    }
    if (line.startsWith("| ---")) {
      continue;
    }
    rows.push(line);
  }
  return rows;
}

function inferGroupFromTestName(testName) {
  const m = /^Test([A-Z])/.exec(testName);
  if (!m) {
    return null;
  }
  switch (m[1]) {
    case "A":
      return "config";
    case "B":
      return "governance";
    case "C":
    case "D":
      return "staking";
    case "E":
      return "delegation";
    case "F":
    case "G":
      return "punish";
    case "H":
    case "I":
      return "rewards";
    case "Y":
    case "Z":
      return "epoch";
    default:
      return null;
  }
}

function inferGroupFromStep(step) {
  if (!step.startsWith("test_Test")) {
    return null;
  }
  const testName = step.slice("test_".length);
  return inferGroupFromTestName(testName);
}

function collectCaseDurationsFromLog(logPath) {
  let text = "";
  try {
    text = fs.readFileSync(logPath, "utf8");
  } catch (err) {
    return [];
  }
  const out = [];
  const lines = text.split(/\r?\n/);
  for (const line of lines) {
    const duration = parseDurationFromGoTestLine(line);
    if (duration !== null) {
      out.push(duration);
    }
  }
  return out;
}

function parseReport(reportPath) {
  const text = fs.readFileSync(reportPath, "utf8");
  const lines = text.split(/\r?\n/);
  const groupDurations = [];
  const slowDurations = [];
  const logPaths = [];

  const stepRows = extractRowsAfterHeader(lines, "| Step | Status | Duration | Skips | Log |");
  for (const line of stepRows) {
    const cols = parseMarkdownRow(line);
    if (!cols || cols.length < 5) {
      continue;
    }
    const step = cols[0];
    const duration = parseDurationMs(cols[2]);
    if (duration === null) {
      continue;
    }

    const logPath = cols[4];
    if (logPath && logPath !== "-") {
      logPaths.push(logPath);
    }

    if (step.startsWith("group_")) {
      const group = step.slice("group_".length).trim().toLowerCase();
      if (group) {
        groupDurations.push({ group, duration });
      }
      continue;
    }

    const inferred = inferGroupFromStep(step);
    if (inferred) {
      groupDurations.push({ group: inferred, duration });
    }
  }

  const slowRows = extractRowsAfterHeader(lines, "| Rank | Test | Duration | Status | Step |");
  for (const line of slowRows) {
    const cols = parseMarkdownRow(line);
    if (!cols || cols.length < 5) {
      continue;
    }
    const duration = parseDurationMs(cols[2]);
    if (duration !== null) {
      slowDurations.push(duration);
    }
  }

  return { groupDurations, slowDurations, logPaths };
}

function quantile(values, q) {
  if (!values || values.length === 0) {
    return null;
  }
  const arr = [...values].sort((a, b) => a - b);
  if (arr.length === 1) {
    return arr[0];
  }
  const pos = (arr.length - 1) * q;
  const base = Math.floor(pos);
  const rest = pos - base;
  if (arr[base + 1] === undefined) {
    return arr[base];
  }
  return arr[base] + rest * (arr[base + 1] - arr[base]);
}

function roundUp(ms, quantumMs) {
  return Math.ceil(ms / quantumMs) * quantumMs;
}

function roundGroupBudget(ms) {
  let v = Math.max(ms, 30 * 1000);
  if (v <= 2 * 60 * 1000) {
    return roundUp(v, 15 * 1000);
  }
  if (v <= 10 * 60 * 1000) {
    return roundUp(v, 30 * 1000);
  }
  return roundUp(v, 60 * 1000);
}

function roundSlowBudget(ms) {
  let v = Math.max(ms, 5 * 1000);
  if (v <= 30 * 1000) {
    return roundUp(v, 1000);
  }
  if (v <= 2 * 60 * 1000) {
    return roundUp(v, 5 * 1000);
  }
  return roundUp(v, 10 * 1000);
}

function formatDuration(ms) {
  let seconds = Math.ceil(ms / 1000);
  if (seconds <= 0) {
    return "0s";
  }
  const h = Math.floor(seconds / 3600);
  seconds -= h * 3600;
  const m = Math.floor(seconds / 60);
  seconds -= m * 60;
  let out = "";
  if (h > 0) out += `${h}h`;
  if (m > 0) out += `${m}m`;
  if (seconds > 0 || out === "") out += `${seconds}s`;
  return out;
}

function orderedGroupEntries(mapObj) {
  const entries = Object.entries(mapObj);
  const order = new Map(knownGroups.map((g, idx) => [g, idx]));
  entries.sort((a, b) => {
    const ai = order.has(a[0]) ? order.get(a[0]) : Number.MAX_SAFE_INTEGER;
    const bi = order.has(b[0]) ? order.get(b[0]) : Number.MAX_SAFE_INTEGER;
    if (ai !== bi) return ai - bi;
    return a[0].localeCompare(b[0]);
  });
  return entries;
}

function currentGroupThreshold(group, current) {
  const key = String(group || "").toLowerCase();
  if (Object.prototype.hasOwnProperty.call(current.groups, key)) {
    return current.groups[key];
  }
  return current.defaultThreshold;
}

function driftTriggered(currentMs, suggestedMs, ratioThreshold, minMs) {
  if (!Number.isFinite(currentMs) || currentMs <= 0 || !Number.isFinite(suggestedMs) || suggestedMs <= 0) {
    return false;
  }
  const diff = Math.abs(suggestedMs - currentMs);
  if (diff < minMs) {
    return false;
  }
  const ratio = diff / currentMs;
  return ratio >= ratioThreshold;
}

function driftPercent(currentMs, suggestedMs) {
  if (!Number.isFinite(currentMs) || currentMs <= 0 || !Number.isFinite(suggestedMs)) {
    return 0;
  }
  return ((suggestedMs - currentMs) / currentMs) * 100;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const currentGroupThresholds = parseThresholdCsv(args.currentGroupThresholds);
  const currentSlowThreshold = parseOptionalDuration(args.currentSlowThreshold, "--current-slow-threshold");
  const currentTestSlowThreshold = parseOptionalDuration(
    args.currentTestSlowThreshold,
    "--current-test-slow-threshold",
  );

  const reportFiles = listRecentReportFiles(args.reportsDir, args.recent);
  if (reportFiles.length === 0) {
    console.error(`No report files found under ${args.reportsDir}`);
    process.exit(1);
  }

  const groupSamples = new Map();
  const slowSamples = [];
  const seenLogs = new Set();

  for (const reportFile of reportFiles) {
    const parsed = parseReport(reportFile);
    for (const item of parsed.groupDurations) {
      if (!groupSamples.has(item.group)) {
        groupSamples.set(item.group, []);
      }
      groupSamples.get(item.group).push(item.duration);
    }
    slowSamples.push(...parsed.slowDurations);
    for (const logPath of parsed.logPaths) {
      const resolved = path.isAbsolute(logPath)
        ? logPath
        : path.join(path.dirname(reportFile), logPath);
      if (seenLogs.has(resolved)) {
        continue;
      }
      seenLogs.add(resolved);
      slowSamples.push(...collectCaseDurationsFromLog(resolved));
    }
  }

  const recommendedGroups = {};
  const groupLines = [];
  for (const [group, durations] of groupSamples.entries()) {
    const p = quantile(durations, args.groupQuantile);
    if (p === null) {
      continue;
    }
    const max = Math.max(...durations);
    const rec = roundGroupBudget(p * args.groupHeadroom);
    recommendedGroups[group] = rec;
    groupLines.push({
      group,
      samples: durations.length,
      p,
      max,
      rec,
    });
  }

  groupLines.sort((a, b) => {
    const order = new Map(knownGroups.map((g, idx) => [g, idx]));
    const ai = order.has(a.group) ? order.get(a.group) : Number.MAX_SAFE_INTEGER;
    const bi = order.has(b.group) ? order.get(b.group) : Number.MAX_SAFE_INTEGER;
    if (ai !== bi) return ai - bi;
    return a.group.localeCompare(b.group);
  });

  let defaultGroup = null;
  const recValues = Object.values(recommendedGroups);
  if (recValues.length > 0) {
    defaultGroup = roundGroupBudget(Math.max(...recValues));
  }

  let slowSuggested = null;
  if (slowSamples.length > 0) {
    const p = quantile(slowSamples, args.slowQuantile);
    slowSuggested = roundSlowBudget(p * args.slowHeadroom);
  }
  const testSlowSuggested = slowSuggested !== null
    ? Math.max(5000, Math.floor(slowSuggested / 2))
    : null;

  if (args.format !== "make") {
    console.log(`# Budget recommendation from ${reportFiles.length} report(s)`);
    console.log(`# Source dir: ${args.reportsDir}`);
    console.log("");

    if (groupLines.length === 0) {
      console.log("No group_* step duration samples found.");
    } else {
      console.log("## Group summary");
      console.log("| Group | Samples | q | qDuration | Max | Suggested |");
      console.log("| --- | ---: | ---: | --- | --- | --- |");
      for (const line of groupLines) {
        console.log(
          `| ${line.group} | ${line.samples} | ${args.groupQuantile.toFixed(2)} | ${formatDuration(line.p)} | ${formatDuration(line.max)} | ${formatDuration(line.rec)} |`,
        );
      }
      console.log("");
    }

    console.log(`Slow sample count: ${slowSamples.length}`);
    if (slowSuggested !== null) {
      const slowQ = quantile(slowSamples, args.slowQuantile);
      const slowMax = Math.max(...slowSamples);
      console.log(
        `Slow q${args.slowQuantile.toFixed(2)}=${formatDuration(slowQ)}, max=${formatDuration(slowMax)}, suggested=${formatDuration(slowSuggested)}`,
      );
    } else {
      console.log("Slow samples not found in scanned reports (kept existing default).");
    }
    console.log("");
  }

  const ordered = orderedGroupEntries(recommendedGroups)
    .map(([group, ms]) => `${group}=${formatDuration(ms)}`);
  if (defaultGroup !== null) {
    ordered.push(`default=${formatDuration(defaultGroup)}`);
  }

  const driftAlerts = [];
  for (const [group, suggested] of Object.entries(recommendedGroups)) {
    const current = currentGroupThreshold(group, currentGroupThresholds);
    if (!Number.isFinite(current) || current <= 0) {
      continue;
    }
    if (!driftTriggered(current, suggested, args.driftRatio, args.driftMinMs)) {
      continue;
    }
    driftAlerts.push({
      kind: "group",
      name: group,
      current,
      suggested,
      deltaPct: driftPercent(current, suggested),
    });
  }
  if (defaultGroup !== null && Number.isFinite(currentGroupThresholds.defaultThreshold) && currentGroupThresholds.defaultThreshold > 0) {
    if (driftTriggered(currentGroupThresholds.defaultThreshold, defaultGroup, args.driftRatio, args.driftMinMs)) {
      driftAlerts.push({
        kind: "group",
        name: "default",
        current: currentGroupThresholds.defaultThreshold,
        suggested: defaultGroup,
        deltaPct: driftPercent(currentGroupThresholds.defaultThreshold, defaultGroup),
      });
    }
  }
  if (slowSuggested !== null && Number.isFinite(currentSlowThreshold) && currentSlowThreshold > 0) {
    if (driftTriggered(currentSlowThreshold, slowSuggested, args.driftRatio, args.driftMinMs)) {
      driftAlerts.push({
        kind: "slow",
        name: "CI_BUDGET_SLOW_THRESHOLD",
        current: currentSlowThreshold,
        suggested: slowSuggested,
        deltaPct: driftPercent(currentSlowThreshold, slowSuggested),
      });
    }
  }
  if (testSlowSuggested !== null && Number.isFinite(currentTestSlowThreshold) && currentTestSlowThreshold > 0) {
    if (driftTriggered(currentTestSlowThreshold, testSlowSuggested, args.driftRatio, args.driftMinMs)) {
      driftAlerts.push({
        kind: "slow",
        name: "CI_BUDGET_TEST_SLOW_THRESHOLD",
        current: currentTestSlowThreshold,
        suggested: testSlowSuggested,
        deltaPct: driftPercent(currentTestSlowThreshold, testSlowSuggested),
      });
    }
  }

  if (args.format === "make") {
    if (ordered.length > 0) {
      console.log(`CI_BUDGET_GROUP_THRESHOLDS=${ordered.join(",")}`);
    }
    if (slowSuggested !== null) {
      console.log(`CI_BUDGET_SLOW_THRESHOLD=${formatDuration(slowSuggested)}`);
      console.log(`CI_BUDGET_TEST_SLOW_THRESHOLD=${formatDuration(testSlowSuggested)}`);
    }
  } else {
    console.log("## Recommended Make overrides");
    if (ordered.length > 0) {
      console.log(`CI_BUDGET_GROUP_THRESHOLDS=${ordered.join(",")}`);
    } else {
      console.log("# CI_BUDGET_GROUP_THRESHOLDS=<insufficient-data>");
    }
    if (slowSuggested !== null) {
      console.log(`CI_BUDGET_SLOW_THRESHOLD=${formatDuration(slowSuggested)}`);
      console.log(`CI_BUDGET_TEST_SLOW_THRESHOLD=${formatDuration(testSlowSuggested)}`);
    } else {
      console.log("# CI_BUDGET_SLOW_THRESHOLD=<insufficient-data>");
      console.log("# CI_BUDGET_TEST_SLOW_THRESHOLD=<insufficient-data>");
    }

    console.log("");
    console.log(`## Drift check (ratio>=${args.driftRatio}, min=${args.driftMinMs}ms)`);
    if (driftAlerts.length === 0) {
      console.log("No drift alert against provided current thresholds.");
    } else {
      console.log("| Kind | Key | Current | Suggested | Delta |");
      console.log("| --- | --- | --- | --- | ---: |");
      for (const item of driftAlerts) {
        const sign = item.deltaPct >= 0 ? "+" : "";
        console.log(
          `| ${item.kind} | ${item.name} | ${formatDuration(item.current)} | ${formatDuration(item.suggested)} | ${sign}${item.deltaPct.toFixed(1)}% |`,
        );
      }
    }
  }

  if (args.failOnDrift && driftAlerts.length > 0) {
    console.error(`Drift alerts detected: ${driftAlerts.length}`);
    process.exit(2);
  }
}

try {
  main();
} catch (err) {
  console.error(`Error: ${err.message}`);
  process.exit(1);
}
