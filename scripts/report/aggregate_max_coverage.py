#!/usr/bin/env python3
import argparse
import csv
import json
import os
from collections import defaultdict
from datetime import datetime, timezone


def read_steps(path: str):
    rows = []
    with open(path, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f, delimiter="\t")
        for row in reader:
            try:
                row["rc"] = int(row.get("rc", "0") or 0)
            except ValueError:
                row["rc"] = 1
            try:
                row["duration_sec"] = int(row.get("duration_sec", "0") or 0)
            except ValueError:
                row["duration_sec"] = 0
            rows.append(row)
    return rows


def display_status(status: str) -> str:
    normalized = (status or "").strip().upper()
    if normalized == "PASS":
        return "🟢 PASS"
    if normalized == "FAIL":
        return "🔴 FAIL"
    if normalized == "TIMEOUT":
        return "🟠 TIMEOUT"
    if normalized == "SKIP":
        return "🟡 SKIP"
    return status or "UNKNOWN"


def discover_artifacts(report_dir: str):
    if not report_dir or not os.path.isdir(report_dir):
        return []

    targets = {
        "index.md",
        "index.json",
        "matrix.md",
        "matrix.json",
        "report.md",
        "summary.json",
        "manifest.json",
    }
    artifacts = []
    for root, _dirs, files in os.walk(report_dir):
        for file_name in sorted(files):
            if file_name not in targets:
                continue
            artifacts.append(os.path.join(root, file_name))
        if len(artifacts) >= 8:
            break
    artifacts.sort()
    return artifacts[:8]


def main():
    parser = argparse.ArgumentParser(description="Aggregate max coverage run results")
    parser.add_argument("--steps", required=True, help="Path to steps.tsv")
    parser.add_argument("--output-dir", required=True, help="Coverage run output directory")
    args = parser.parse_args()

    steps_file = os.path.abspath(args.steps)
    output_dir = os.path.abspath(args.output_dir)

    if not os.path.exists(steps_file):
        raise FileNotFoundError(f"steps file not found: {steps_file}")
    os.makedirs(output_dir, exist_ok=True)

    steps = read_steps(steps_file)
    by_category = defaultdict(list)
    for item in steps:
        by_category[item.get("category", "unknown")].append(item)

    pass_count = sum(1 for s in steps if (s.get("status") or "").upper() == "PASS")
    fail_count = sum(1 for s in steps if (s.get("status") or "").upper() == "FAIL")
    timeout_count = sum(1 for s in steps if (s.get("status") or "").upper() == "TIMEOUT")
    total = len(steps)
    duration_total = sum(int(s.get("duration_sec", 0) or 0) for s in steps)

    status = "PASS" if fail_count == 0 and timeout_count == 0 else "FAIL"
    generated_at = datetime.now(timezone.utc).isoformat()

    for item in steps:
        item["artifacts"] = discover_artifacts(item.get("report_dir", ""))

    summary = {
        "generated_at": generated_at,
        "status": status,
        "total_steps": total,
        "pass_steps": pass_count,
        "fail_steps": fail_count,
        "timeout_steps": timeout_count,
        "duration_sec": duration_total,
        "categories": {
            cat: {
                "total": len(items),
                "pass": sum(1 for i in items if (i.get("status") or "").upper() == "PASS"),
                "fail": sum(1 for i in items if (i.get("status") or "").upper() == "FAIL"),
                "timeout": sum(1 for i in items if (i.get("status") or "").upper() == "TIMEOUT"),
                "duration_sec": sum(int(i.get("duration_sec", 0) or 0) for i in items),
            }
            for cat, items in sorted(by_category.items(), key=lambda x: x[0])
        },
        "steps": steps,
    }

    json_path = os.path.join(output_dir, "index.json")
    with open(json_path, "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2)
        f.write("\n")

    md_path = os.path.join(output_dir, "index.md")
    with open(md_path, "w", encoding="utf-8") as f:
        f.write("# Max Coverage Test Report\n\n")
        f.write(f"- Generated: {generated_at}\n")
        f.write(f"- Status: {display_status(status)}\n")
        f.write(f"- Total Steps: {total}\n")
        f.write(f"- Passed: {pass_count}\n")
        f.write(f"- Failed: {fail_count}\n")
        f.write(f"- Timeout: {timeout_count}\n")
        f.write(f"- Total Duration(s): {duration_total}\n\n")

        f.write("## Category Summary\n\n")
        f.write("| Category | Total | Pass | Fail | Timeout | Duration(s) |\n")
        f.write("| --- | ---: | ---: | ---: | ---: | ---: |\n")
        for category, item in summary["categories"].items():
            f.write(
                f"| {category} | {item['total']} | {item['pass']} | {item['fail']} | {item['timeout']} | {item['duration_sec']} |\n"
            )
        f.write("\n")

        f.write("## Step Results\n\n")
        f.write("| Step | Category | Status | RC | Duration(s) | Report Dir | Artifacts | Log |\n")
        f.write("| --- | --- | --- | ---: | ---: | --- | --- | --- |\n")
        for item in steps:
            artifacts = item.get("artifacts", [])
            artifact_text = "<br/>".join(artifacts) if artifacts else "-"
            f.write(
                f"| {item.get('step','')} | {item.get('category','')} | {display_status(item.get('status',''))} | "
                f"{item.get('rc', 0)} | {item.get('duration_sec', 0)} | {item.get('report_dir','-')} | "
                f"{artifact_text} | {item.get('log_file','-')} |\n"
            )

    print(f"max coverage report markdown: {md_path}")
    print(f"max coverage report json: {json_path}")


if __name__ == "__main__":
    main()
