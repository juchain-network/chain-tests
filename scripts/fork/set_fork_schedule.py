#!/usr/bin/env python3
import json
import sys
import time

FORK_FIELDS = ("cancunTime", "shanghaiTime", "posaTime", "fixHeaderTime")
DEFAULT_BLOB_SCHEDULE = {
    "cancun": {
        "target": 3,
        "max": 6,
        "baseFeeUpdateFraction": 3338477,
    }
}


def canonical_target(raw: str) -> str:
    value = (raw or "").strip()
    aliases = {
        "cancun": "cancunTime",
        "cancuntime": "cancunTime",
        "shanghai": "shanghaiTime",
        "shanghaitime": "shanghaiTime",
        "posa": "posaTime",
        "posatime": "posaTime",
        "fixheader": "fixHeaderTime",
        "fixheadertime": "fixHeaderTime",
        "cancunTime": "cancunTime",
        "shanghaiTime": "shanghaiTime",
        "posaTime": "posaTime",
        "fixHeaderTime": "fixHeaderTime",
    }
    return aliases.get(value, "")


def ensure_blob_schedule(cfg: dict) -> None:
    blob_schedule = cfg.get("blobSchedule")
    if not isinstance(blob_schedule, dict):
        blob_schedule = {}
        cfg["blobSchedule"] = blob_schedule
    if "cancun" not in blob_schedule or not isinstance(blob_schedule["cancun"], dict):
        blob_schedule["cancun"] = dict(DEFAULT_BLOB_SCHEDULE["cancun"])


def main() -> int:
    if len(sys.argv) < 3:
        print(
            "usage: set_fork_schedule.py <genesis.json> <poa|posa|upgrade> [target] [delay_sec]",
            file=sys.stderr,
        )
        return 1

    path = sys.argv[1]
    mode = sys.argv[2].strip().lower()
    target_raw = sys.argv[3] if len(sys.argv) > 3 else ""
    delay = int(sys.argv[4]) if len(sys.argv) > 4 and sys.argv[4] else 120

    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)

    cfg = data.setdefault("config", {})
    now = int(time.time())
    scheduled_time = 0
    effective_target = ""

    if mode == "poa":
        for key in FORK_FIELDS:
            cfg.pop(key, None)
        cfg.pop("blobSchedule", None)
    elif mode == "posa":
        for key in FORK_FIELDS:
            cfg[key] = 0
        ensure_blob_schedule(cfg)
        effective_target = "all"
    elif mode == "upgrade":
        effective_target = canonical_target(target_raw)
        if not effective_target:
            print(
                "upgrade mode requires target in {shanghaiTime,cancunTime,posaTime,fixHeaderTime}",
                file=sys.stderr,
            )
            return 1
        for key in FORK_FIELDS:
            cfg.pop(key, None)
        cfg.pop("blobSchedule", None)
        scheduled_time = now + max(delay, 0)
        cfg[effective_target] = scheduled_time
        if effective_target == "cancunTime":
            ensure_blob_schedule(cfg)
    else:
        print(f"unsupported mode: {mode}", file=sys.stderr)
        return 1

    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)
        f.write("\n")

    print(
        json.dumps(
            {
                "mode": mode,
                "target": effective_target,
                "scheduled_time": scheduled_time,
            }
        )
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
