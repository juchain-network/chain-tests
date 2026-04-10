#!/usr/bin/env python3
import json
import sys
import time

FORK_FIELDS = ("cancunTime", "shanghaiTime", "posaTime", "fixHeaderTime")
UPGRADE_DEPENDENCIES = {
    "shanghaiTime": ("shanghaiTime",),
    "cancunTime": ("shanghaiTime", "cancunTime"),
    "fixHeaderTime": ("shanghaiTime", "cancunTime", "fixHeaderTime"),
    "posaTime": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime"),
}
STAGGER_STEP_SECONDS = 60
SMOKE_STATIC_CASES = {
    "poa": (),
    "poa_shanghai": ("shanghaiTime",),
    "poa_shanghai_cancun": ("shanghaiTime", "cancunTime"),
    "poa_shanghai_cancun_fixheader": ("shanghaiTime", "cancunTime", "fixHeaderTime"),
    "poa_shanghai_cancun_fixheader_posa": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime"),
}
DEFAULT_BLOB_SCHEDULE = {
    "cancun": {
        "target": 3,
        "max": 6,
        "baseFeeUpdateFraction": 3338477,
    }
}


def canonical_target(raw: str) -> str:
    value = (raw or "").strip().lower()
    aliases = {
        "cancun": "cancunTime",
        "cancuntime": "cancunTime",
        "shanghai": "shanghaiTime",
        "shanghaitime": "shanghaiTime",
        "posa": "posaTime",
        "posatime": "posaTime",
        "fixheader": "fixHeaderTime",
        "fixheadertime": "fixHeaderTime",
        "staggered": "allStaggered",
        "allstaggered": "allStaggered",
        "all_staggered": "allStaggered",
        "all-staggered": "allStaggered",
        "staggered1m": "allStaggered",
        "allsame": "allSame",
        "all_same": "allSame",
        "all-same": "allSame",
    }
    return aliases.get(value, "")


def canonical_smoke_case(raw: str) -> str:
    value = (raw or "").strip().lower()
    aliases = {
        "poa": "poa",
        "poa_shanghai": "poa_shanghai",
        "poa-shanghai": "poa_shanghai",
        "poa_shanghai_cancun": "poa_shanghai_cancun",
        "poa-shanghai-cancun": "poa_shanghai_cancun",
        "poa_shanghai_cancun_fixheader": "poa_shanghai_cancun_fixheader",
        "poa-shanghai-cancun-fixheader": "poa_shanghai_cancun_fixheader",
        "poa_shanghai_cancun_fixheader_posa": "poa_shanghai_cancun_fixheader_posa",
        "poa-shanghai-cancun-fixheader-posa": "poa_shanghai_cancun_fixheader_posa",
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
            "usage: set_fork_schedule.py <genesis.json> <poa|posa|upgrade|smoke> [target] [delay_sec]",
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
    effective_delay_seconds = max(delay, 0)

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
                "upgrade mode requires target in {shanghaiTime,cancunTime,posaTime,fixHeaderTime,allStaggered,allSame}",
                file=sys.stderr,
            )
            return 1
        for key in FORK_FIELDS:
            cfg.pop(key, None)
        cfg.pop("blobSchedule", None)
        start_time = now + max(delay, 0)
        scheduled_time = start_time
        dependencies = UPGRADE_DEPENDENCIES.get(effective_target, ())
        if len(dependencies) > 0:
            for key in dependencies:
                cfg[key] = scheduled_time
        elif effective_target == "allStaggered":
            cfg["shanghaiTime"] = start_time
            cfg["cancunTime"] = start_time + STAGGER_STEP_SECONDS
            cfg["fixHeaderTime"] = start_time + STAGGER_STEP_SECONDS*2
            cfg["posaTime"] = start_time + STAGGER_STEP_SECONDS*3
            scheduled_time = cfg["posaTime"]
            effective_delay_seconds = effective_delay_seconds + STAGGER_STEP_SECONDS*3
        elif effective_target == "allSame":
            for key in ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime"):
                cfg[key] = start_time
            scheduled_time = start_time
        else:
            print(f"unsupported upgrade target: {effective_target}", file=sys.stderr)
            return 1
        if cfg.get("cancunTime", 0) > 0:
            ensure_blob_schedule(cfg)
    elif mode == "smoke":
        effective_target = canonical_smoke_case(target_raw)
        if not effective_target:
            print(
                "smoke mode requires target in {poa,poa_shanghai,poa_shanghai_cancun,poa_shanghai_cancun_fixheader,poa_shanghai_cancun_fixheader_posa}",
                file=sys.stderr,
            )
            return 1
        for key in FORK_FIELDS:
            cfg.pop(key, None)
        cfg.pop("blobSchedule", None)

        for key in SMOKE_STATIC_CASES[effective_target]:
            cfg[key] = 0
        if "cancunTime" in SMOKE_STATIC_CASES[effective_target]:
            ensure_blob_schedule(cfg)
        effective_delay_seconds = 0
    else:
        print(f"unsupported mode: {mode}", file=sys.stderr)
        return 1

    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)
        f.write("\n")

    schedule = {}
    for key in FORK_FIELDS:
        value = cfg.get(key, 0)
        if isinstance(value, int):
            schedule[key] = value
        else:
            schedule[key] = 0

    print(
        json.dumps(
            {
                "mode": mode,
                "target": effective_target,
                "scheduled_time": scheduled_time,
                "effective_delay_seconds": effective_delay_seconds,
                "schedule": schedule,
            }
        )
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
