#!/usr/bin/env bash
set -euo pipefail

RESULTS_FILE="${1:-}"
REPORT_DIR="${2:-}"

if [[ -z "$RESULTS_FILE" || -z "$REPORT_DIR" ]]; then
  echo "usage: scripts/fork/collect_matrix_report.sh <matrix_results.tsv> <report_dir>" >&2
  exit 1
fi

[[ -f "$RESULTS_FILE" ]] || { echo "results file not found: $RESULTS_FILE" >&2; exit 1; }
mkdir -p "$REPORT_DIR"

python3 - "$RESULTS_FILE" "$REPORT_DIR" <<'PY'
import csv
import json
import os
import sys
from datetime import datetime, timezone

results_file = sys.argv[1]
report_dir = sys.argv[2]

rows = []
with open(results_file, "r", encoding="utf-8") as f:
    reader = csv.DictReader(f, delimiter="\t")
    for row in reader:
        row["rc"] = int(row.get("rc", "0") or 0)
        rows.append(row)

rows.sort(key=lambda r: (r.get("topology", ""), r.get("case", "")))

passed = sum(1 for r in rows if r.get("status") == "PASS")
failed = sum(1 for r in rows if r.get("status") != "PASS")

def display_status(status: str) -> str:
    normalized = (status or "").strip().upper()
    if normalized == "PASS":
        return "🟢 PASS"
    if normalized == "FAIL":
        return "🔴 FAIL"
    if normalized == "SKIP":
        return "🟡 SKIP"
    return status or "UNKNOWN"

matrix = {
    "generated_at": datetime.now(timezone.utc).isoformat(),
    "total": len(rows),
    "passed": passed,
    "failed": failed,
    "status": "PASS" if failed == 0 else "FAIL",
    "cases": rows,
}

json_path = os.path.join(report_dir, "matrix.json")
with open(json_path, "w", encoding="utf-8") as f:
    json.dump(matrix, f, indent=2)
    f.write("\n")

md_path = os.path.join(report_dir, "matrix.md")
with open(md_path, "w", encoding="utf-8") as f:
    f.write("# Fork Matrix Report\n\n")
    f.write(f"- Generated: {matrix['generated_at']}\n")
    f.write(f"- Total: {matrix['total']}\n")
    f.write(f"- Passed: {matrix['passed']}\n")
    f.write(f"- Failed: {matrix['failed']}\n")
    f.write(f"- Status: {display_status(matrix['status'])}\n\n")
    f.write("| Topology | Case | Mode | Target | Status | RC | Log | Repro |\n")
    f.write("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
    for r in rows:
        f.write(
            f"| {r.get('topology','')} | {r.get('case','')} | {r.get('mode','')} | "
            f"{r.get('target','')} | {display_status(r.get('status',''))} | {r.get('rc',0)} | "
            f"{r.get('log','')} | `{r.get('repro','')}` |\n"
        )

print(f"matrix report markdown: {md_path}")
print(f"matrix report json: {json_path}")
PY
