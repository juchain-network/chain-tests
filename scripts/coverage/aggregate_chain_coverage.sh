#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/coverage/lib.sh
source "$SCRIPT_DIR/lib.sh"

CONFIG_FILE="$(resolve_config_file "${TEST_ENV_CONFIG:-}")"

coverage_apply_session_defaults

coverage_scope >/dev/null
coverage_prepare_dirs

CHAIN_ROOT="$(coverage_chain_root "$CONFIG_FILE")"
mkdir -p "${GOCACHE:-/tmp/go-build}"
export GOCACHE="${GOCACHE:-/tmp/go-build}"

OUT_DIR="$(coverage_out_dir)"
RAW_ROOT="$(coverage_raw_root)"
MERGED_DIR="$OUT_DIR/merged"
PROFILE_FILE="$OUT_DIR/coverage.out"
FILTERED_PROFILE_FILE="$OUT_DIR/coverage_congress.out"
FUNC_FILE="$OUT_DIR/func.txt"
PACKAGE_PERCENT_FILE="$OUT_DIR/package_percent.txt"
SUMMARY_FILE="$OUT_DIR/summary.txt"
META_FILE="$OUT_DIR/meta.json"
KEEP_RAW="${CHAIN_COVERAGE_KEEP_RAW:-1}"

mkdir -p "$OUT_DIR"

declare -a INPUT_DIRS=()
while IFS= read -r dir; do
  [[ -n "$dir" ]] || continue
  if find "$dir" -type f | grep -q .; then
    INPUT_DIRS+=("$dir")
  fi
done < <(find "$RAW_ROOT" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort)

write_meta() {
  local status="$1"
  local total="$2"
  python3 - "$META_FILE" "$status" "$total" "$(coverage_scope)" "$OUT_DIR" "${CHAIN_COVERAGE_COMMAND:-}" <<'PY'
import json
import sys

path, status, total, scope, out_dir, command = sys.argv[1:7]
with open(path, "w", encoding="utf-8") as fh:
    json.dump(
        {
            "status": status,
            "scope": scope,
            "total_coverage": total,
            "out_dir": out_dir,
            "command": command,
        },
        fh,
        indent=2,
        sort_keys=True,
    )
PY
}

if [[ "${#INPUT_DIRS[@]}" -eq 0 ]]; then
  cat >"$SUMMARY_FILE" <<EOF_SUMMARY
scope: $(coverage_scope)
status: no_data
command: ${CHAIN_COVERAGE_COMMAND:-<unknown>}
out_dir: $OUT_DIR
note: no raw coverage files were produced by native geth processes
hint: check whether tests failed before/while starting the chain, or whether native stop flushed GOCOVERDIR successfully
EOF_SUMMARY
  write_meta "no_data" ""
  echo "[coverage] no raw coverage data found under $RAW_ROOT"
  exit 0
fi

rm -rf "$MERGED_DIR" "$PROFILE_FILE" "$FUNC_FILE" "$PACKAGE_PERCENT_FILE"
mkdir -p "$MERGED_DIR"

input_csv="$(IFS=,; echo "${INPUT_DIRS[*]}")"
go tool covdata merge -i="$input_csv" -o="$MERGED_DIR"
go tool covdata textfmt -i="$MERGED_DIR" -o="$PROFILE_FILE"

python3 - "$PROFILE_FILE" "$FILTERED_PROFILE_FILE" <<'PY'
import sys

src, dst = sys.argv[1], sys.argv[2]
with open(src, "r", encoding="utf-8") as fh:
    lines = fh.readlines()

with open(dst, "w", encoding="utf-8") as out:
    for line in lines:
        if line.startswith("mode:"):
            out.write(line)
            continue
        if "/consensus/congress/" in line:
            out.write(line)
PY

(cd "$CHAIN_ROOT" && go tool cover -func="$FILTERED_PROFILE_FILE") >"$FUNC_FILE"

python3 - "$FILTERED_PROFILE_FILE" "$PACKAGE_PERCENT_FILE" <<'PY'
import os
import sys

profile_path, out_path = sys.argv[1], sys.argv[2]
packages = {}

with open(profile_path, "r", encoding="utf-8") as fh:
    for line in fh:
        if line.startswith("mode:"):
            continue
        line = line.strip()
        if not line:
            continue
        fields = line.split()
        if len(fields) != 3:
            continue
        path_range, stmts_s, count_s = fields
        path = path_range.split(":", 1)[0]
        pkg = os.path.dirname(path)
        stmts = int(stmts_s)
        count = int(count_s)
        total, covered = packages.get(pkg, (0, 0))
        total += stmts
        if count > 0:
            covered += stmts
        packages[pkg] = (total, covered)

with open(out_path, "w", encoding="utf-8") as out:
    if not packages:
        out.write("no congress coverage entries found\n")
    else:
        for pkg in sorted(packages):
            total, covered = packages[pkg]
            pct = 0.0 if total == 0 else covered * 100.0 / total
            out.write(f"{pkg}\t{pct:.1f}%\n")
PY

total_cov="$(awk '/^total:/{print $3}' "$FUNC_FILE" | tail -n 1)"

{
  echo "scope: $(coverage_scope)"
  echo "status: ok"
  echo "command: ${CHAIN_COVERAGE_COMMAND:-<unknown>}"
  echo "config: $CONFIG_FILE"
  echo "out_dir: $OUT_DIR"
  echo "total_coverage: ${total_cov:-unknown}"
  echo "raw_dirs:"
  for dir in "${INPUT_DIRS[@]}"; do
    echo "  - $dir"
  done
  echo "package_percent: $PACKAGE_PERCENT_FILE"
  echo "function_report: $FUNC_FILE"
  echo "profile: $PROFILE_FILE"
  echo "filtered_profile: $FILTERED_PROFILE_FILE"
  echo "merged_dir: $MERGED_DIR"
} >"$SUMMARY_FILE"

write_meta "ok" "${total_cov:-}"

if ! coverage_bool_true "$KEEP_RAW"; then
  rm -rf "$RAW_ROOT"
fi

echo "[coverage] congress total coverage: ${total_cov:-unknown}"
echo "[coverage] summary: $SUMMARY_FILE"
