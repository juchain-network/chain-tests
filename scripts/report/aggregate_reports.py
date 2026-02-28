#!/usr/bin/env python3
import argparse
import json
import os
import re
from datetime import datetime, timezone


def read_json(path):
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return None


def parse_report_status(report_md):
    if not os.path.exists(report_md):
        return "UNKNOWN"
    try:
        with open(report_md, "r", encoding="utf-8") as f:
            text = f.read()
    except Exception:
        return "UNKNOWN"
    if "| FAIL |" in text or "Status: FAIL" in text:
        return "FAIL"
    if "| PASS |" in text or "Status: PASS" in text:
        return "PASS"
    return "UNKNOWN"


def collect_ci(ci_dir):
    out = []
    if not ci_dir or not os.path.isdir(ci_dir):
        return out
    for name in sorted(os.listdir(ci_dir)):
        if not name.startswith("ci_"):
            continue
        run_dir = os.path.join(ci_dir, name)
        if not os.path.isdir(run_dir):
            continue
        report_md = os.path.join(run_dir, "report.md")
        if not os.path.exists(report_md):
            continue
        summary = read_json(os.path.join(run_dir, "summary.json")) or {}
        manifest = read_json(os.path.join(run_dir, "manifest.json")) or {}
        status = summary.get("status") or parse_report_status(report_md)
        out.append(
            {
                "type": "ci",
                "id": name,
                "status": status,
                "report_md": report_md,
                "summary_json": os.path.join(run_dir, "summary.json") if os.path.exists(os.path.join(run_dir, "summary.json")) else "",
                "manifest_json": os.path.join(run_dir, "manifest.json") if os.path.exists(os.path.join(run_dir, "manifest.json")) else "",
                "mode": manifest.get("mode", summary.get("mode", "")),
            }
        )
    return out


def collect_fork(fork_dir):
    out = []
    if not fork_dir or not os.path.isdir(fork_dir):
        return out

    # Direct matrix under fork_dir.
    direct = read_json(os.path.join(fork_dir, "matrix.json"))
    if direct is not None:
        out.append(
            {
                "type": "fork",
                "id": os.path.basename(os.path.normpath(fork_dir)),
                "status": direct.get("status", "UNKNOWN"),
                "report_md": os.path.join(fork_dir, "matrix.md"),
                "summary_json": os.path.join(fork_dir, "matrix.json"),
                "manifest_json": "",
                "mode": "fork-matrix",
            }
        )

    # Nested matrix outputs.
    for root, _dirs, files in os.walk(fork_dir):
        if root == fork_dir:
            continue
        if "matrix.json" not in files:
            continue
        matrix_json = os.path.join(root, "matrix.json")
        matrix = read_json(matrix_json) or {}
        out.append(
            {
                "type": "fork",
                "id": os.path.relpath(root, fork_dir),
                "status": matrix.get("status", "UNKNOWN"),
                "report_md": os.path.join(root, "matrix.md"),
                "summary_json": matrix_json,
                "manifest_json": "",
                "mode": "fork-matrix",
            }
        )
    return out


def main():
    parser = argparse.ArgumentParser(description="Aggregate regression reports")
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--ci-dir", required=False, default="")
    parser.add_argument("--fork-dir", required=False, default="")
    args = parser.parse_args()

    output_dir = os.path.abspath(args.output_dir)
    os.makedirs(output_dir, exist_ok=True)

    records = []
    records.extend(collect_ci(args.ci_dir))
    records.extend(collect_fork(args.fork_dir))

    records.sort(key=lambda r: (r.get("type", ""), r.get("id", "")))
    total = len(records)
    failed = sum(1 for r in records if r.get("status") not in ("PASS", "pass"))

    index = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "total": total,
        "failed": failed,
        "status": "PASS" if failed == 0 else "FAIL",
        "reports": records,
    }

    index_json = os.path.join(output_dir, "index.json")
    with open(index_json, "w", encoding="utf-8") as f:
        json.dump(index, f, indent=2)
        f.write("\n")

    index_md = os.path.join(output_dir, "index.md")
    with open(index_md, "w", encoding="utf-8") as f:
        f.write("# Regression Report Index\n\n")
        f.write(f"- Generated: {index['generated_at']}\n")
        f.write(f"- Total Reports: {index['total']}\n")
        f.write(f"- Failed Reports: {index['failed']}\n")
        f.write(f"- Status: {index['status']}\n\n")
        f.write("| Type | ID | Mode | Status | Report | Summary | Manifest |\n")
        f.write("| --- | --- | --- | --- | --- | --- | --- |\n")
        for r in records:
            report = r.get("report_md", "")
            summary = r.get("summary_json", "")
            manifest = r.get("manifest_json", "")
            f.write(
                f"| {r.get('type','')} | {r.get('id','')} | {r.get('mode','')} | {r.get('status','')} | "
                f"{report} | {summary or '-'} | {manifest or '-'} |\n"
            )

    print(f"index markdown: {index_md}")
    print(f"index json: {index_json}")


if __name__ == "__main__":
    main()
