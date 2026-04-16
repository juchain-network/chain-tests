#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
usage:
  loopback_netem.sh apply --state-file <path> --ports <p1,p2,...> [--delay 200ms] [--jitter 100ms] [--loss 2%]
  loopback_netem.sh clear --state-file <path>

Notes:
  - repro-only helper for native localhost validator P2P ports
  - requires root because it mutates tc qdisc on lo
  - intentionally coarse-grained: it matches loopback traffic by TCP sport/dport
EOF
}

need_root() {
  if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    echo "loopback netem requires root; re-run with sudo" >&2
    exit 1
  fi
}

need_tc() {
  if ! command -v tc >/dev/null 2>&1; then
    echo "tc command not found" >&2
    exit 1
  fi
}

load_state() {
  if [ -f "$STATE_FILE" ]; then
    # shellcheck disable=SC1090
    source "$STATE_FILE"
  fi
}

save_state() {
  mkdir -p "$(dirname "$STATE_FILE")"
  cat >"$STATE_FILE" <<EOF
PORTS="${PORTS}"
ROOT_CREATED="${ROOT_CREATED}"
DELAY="${DELAY}"
JITTER="${JITTER}"
LOSS="${LOSS}"
EOF
}

ensure_supported_root_qdisc() {
  local root_line
  root_line="$(tc qdisc show dev lo | head -n1 || true)"
  if [[ -z "$root_line" || "$root_line" == *"qdisc noqueue"* ]]; then
    tc qdisc replace dev lo root handle 1: prio bands 4
    ROOT_CREATED=1
    return
  fi
  if [[ "$root_line" == *"qdisc prio 1: root"* ]]; then
    ROOT_CREATED=0
    return
  fi
  echo "unsupported existing root qdisc on lo: $root_line" >&2
  exit 1
}

delete_filters_for_ports() {
  local ports_csv="${1:-}"
  local port
  local idx=0
  local pref_in
  local pref_out
  IFS=',' read -r -a _ports <<<"$ports_csv"
  for port in "${_ports[@]}"; do
    port="${port// /}"
    [ -n "$port" ] || continue
    idx=$((idx + 1))
    pref_in=$((100 + idx))
    pref_out=$((200 + idx))
    tc filter del dev lo parent 1: protocol ip pref "$pref_in" u32 2>/dev/null || true
    tc filter del dev lo parent 1: protocol ip pref "$pref_out" u32 2>/dev/null || true
  done
}

apply_filters() {
  local port
  local idx=0
  local pref_in
  local pref_out
  IFS=',' read -r -a _ports <<<"$PORTS"
  for port in "${_ports[@]}"; do
    port="${port// /}"
    [ -n "$port" ] || continue
    idx=$((idx + 1))
    pref_in=$((100 + idx))
    pref_out=$((200 + idx))
    tc filter add dev lo protocol ip parent 1: prio "$pref_in" u32 \
      match ip dport "$port" 0xffff flowid 1:4
    tc filter add dev lo protocol ip parent 1: prio "$pref_out" u32 \
      match ip sport "$port" 0xffff flowid 1:4
  done
}

ACTION="${1:-}"
shift || true

STATE_FILE=""
PORTS=""
PORTS_ARG=""
DELAY="200ms"
JITTER="100ms"
LOSS="2%"
DELAY_ARG="200ms"
JITTER_ARG="100ms"
LOSS_ARG="2%"
ROOT_CREATED=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --state-file)
      STATE_FILE="${2:-}"
      shift 2
      ;;
    --ports)
      PORTS_ARG="${2:-}"
      shift 2
      ;;
    --delay)
      DELAY_ARG="${2:-}"
      shift 2
      ;;
    --jitter)
      JITTER_ARG="${2:-}"
      shift 2
      ;;
    --loss)
      LOSS_ARG="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

[ -n "$STATE_FILE" ] || { echo "--state-file is required" >&2; exit 1; }
need_root
need_tc

case "$ACTION" in
  apply)
    [ -n "$PORTS_ARG" ] || { echo "--ports is required for apply" >&2; exit 1; }
    load_state
    if [ -n "${PORTS:-}" ]; then
      delete_filters_for_ports "$PORTS"
    fi
    tc qdisc del dev lo parent 1:4 handle 40: netem 2>/dev/null || true
    PORTS="$PORTS_ARG"
    DELAY="$DELAY_ARG"
    JITTER="$JITTER_ARG"
    LOSS="$LOSS_ARG"
    ROOT_CREATED=0
    delete_filters_for_ports "$PORTS"
    ;;
  clear)
    load_state
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac

if [ "$ACTION" = "apply" ]; then
  ensure_supported_root_qdisc
  tc qdisc replace dev lo parent 1:4 handle 40: netem delay "$DELAY" "$JITTER" loss "$LOSS"
  apply_filters
  save_state
  echo "applied loopback netem on lo ports=$PORTS delay=$DELAY jitter=$JITTER loss=$LOSS root_created=$ROOT_CREATED"
  exit 0
fi

delete_filters_for_ports "${PORTS:-}"
tc qdisc del dev lo parent 1:4 handle 40: netem 2>/dev/null || true
if [ "${ROOT_CREATED:-0}" = "1" ]; then
  tc qdisc del dev lo root 2>/dev/null || true
fi
rm -f "$STATE_FILE"
echo "cleared loopback netem"
