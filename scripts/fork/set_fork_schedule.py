#!/usr/bin/env python3
import json
import sys
import time

FORK_FIELDS = (
    "shanghaiTime",
    "cancunTime",
    "fixHeaderTime",
    "posaTime",
    "pragueTime",
    "osakaTime",
    "bpo1Time",
    "bpo2Time",
)
POSA_BASE_FIELDS = ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime")
UPGRADE_DEPENDENCIES = {
    "shanghaiTime": ("shanghaiTime",),
    "cancunTime": ("shanghaiTime", "cancunTime"),
    "fixHeaderTime": ("shanghaiTime", "cancunTime", "fixHeaderTime"),
    "posaTime": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime"),
    "pragueTime": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime"),
    "osakaTime": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime", "osakaTime"),
    "bpo1Time": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime", "osakaTime", "bpo1Time"),
    "bpo2Time": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime", "osakaTime", "bpo1Time", "bpo2Time"),
}
STAGGER_STEP_SECONDS = 60
SMOKE_STATIC_CASES = {
    "poa": (),
    "poa_shanghai": ("shanghaiTime",),
    "poa_shanghai_cancun": ("shanghaiTime", "cancunTime"),
    "poa_shanghai_cancun_fixheader": ("shanghaiTime", "cancunTime", "fixHeaderTime"),
    "poa_shanghai_cancun_fixheader_posa": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime"),
    "poa_shanghai_cancun_fixheader_posa_prague": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime"),
    "poa_shanghai_cancun_fixheader_posa_prague_osaka": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime", "osakaTime"),
    "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime", "osakaTime", "bpo1Time"),
    "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2": ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime", "osakaTime", "bpo1Time", "bpo2Time"),
}
DEFAULT_BLOB_SCHEDULE = {
    "cancun": {
        "target": 3,
        "max": 6,
        "baseFeeUpdateFraction": 3338477,
    },
    "prague": {
        "target": 6,
        "max": 9,
        "baseFeeUpdateFraction": 5007716,
    },
    "osaka": {
        "target": 6,
        "max": 9,
        "baseFeeUpdateFraction": 5007716,
    },
    "bpo1": {
        "target": 10,
        "max": 15,
        "baseFeeUpdateFraction": 8346193,
    },
    "bpo2": {
        "target": 14,
        "max": 21,
        "baseFeeUpdateFraction": 11684671,
    },
}

FORK_ORDER = (
    "shanghaiTime",
    "cancunTime",
    "fixHeaderTime",
    "posaTime",
    "pragueTime",
    "osakaTime",
    "bpo1Time",
    "bpo2Time",
)


def canonical_target(raw: str) -> str:
    value = (raw or "").strip().lower()
    aliases = {
        "cancun": "cancunTime",
        "cancuntime": "cancunTime",
        "shanghai": "shanghaiTime",
        "shanghaitime": "shanghaiTime",
        "posa": "posaTime",
        "posatime": "posaTime",
        "prague": "pragueTime",
        "praguetime": "pragueTime",
        "osaka": "osakaTime",
        "osakatime": "osakaTime",
        "bpo1": "bpo1Time",
        "bpo1time": "bpo1Time",
        "bpo2": "bpo2Time",
        "bpo2time": "bpo2Time",
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
        "poa_shanghai_cancun_fixheader_posa_prague": "poa_shanghai_cancun_fixheader_posa_prague",
        "poa-shanghai-cancun-fixheader-posa-prague": "poa_shanghai_cancun_fixheader_posa_prague",
        "poa_shanghai_cancun_fixheader_posa_prague_osaka": "poa_shanghai_cancun_fixheader_posa_prague_osaka",
        "poa-shanghai-cancun-fixheader-posa-prague-osaka": "poa_shanghai_cancun_fixheader_posa_prague_osaka",
        "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1": "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1",
        "poa-shanghai-cancun-fixheader-posa-prague-osaka-bpo1": "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1",
        "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2": "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2",
        "poa-shanghai-cancun-fixheader-posa-prague-osaka-bpo1-bpo2": "poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2",
    }
    return aliases.get(value, "")


def ensure_blob_schedule(cfg: dict) -> None:
    blob_schedule = cfg.get("blobSchedule")
    if not isinstance(blob_schedule, dict):
        blob_schedule = {}
        cfg["blobSchedule"] = blob_schedule
    for fork_name, fork_cfg in DEFAULT_BLOB_SCHEDULE.items():
        if fork_name not in blob_schedule or not isinstance(blob_schedule[fork_name], dict):
            blob_schedule[fork_name] = dict(fork_cfg)


def validate_prefix_schedule(mode: str, cfg: dict) -> None:
    enabled = [key for key in FORK_ORDER if key in cfg]
    if mode == "poa":
        if enabled:
            raise SystemExit(f"poa mode must not enable forks, got: {enabled}")
        return
    if not enabled:
        raise SystemExit(f"{mode} mode must enable at least one fork in {FORK_ORDER}")
    expected_prefix = list(FORK_ORDER[:len(enabled)])
    if enabled != expected_prefix:
        raise SystemExit(
            f"invalid fork prefix: enabled={enabled}, expected_prefix={expected_prefix}"
        )
    for i in range(1, len(enabled)):
        prev_key = enabled[i - 1]
        curr_key = enabled[i]
        prev_value = int(cfg.get(prev_key, 0) or 0)
        curr_value = int(cfg.get(curr_key, 0) or 0)
        if prev_value > curr_value:
            raise SystemExit(
                f"invalid fork ordering: {prev_key}={prev_value} > {curr_key}={curr_value}"
            )


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
            cfg.pop(key, None)
        for key in POSA_BASE_FIELDS:
            cfg[key] = 0
        ensure_blob_schedule(cfg)
        effective_target = "all"
    elif mode == "upgrade":
        effective_target = canonical_target(target_raw)
        if not effective_target:
            print(
                "upgrade mode requires target in {shanghaiTime,cancunTime,fixHeaderTime,posaTime,pragueTime,osakaTime,bpo1Time,bpo2Time,allStaggered,allSame}",
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
            cfg["pragueTime"] = start_time + STAGGER_STEP_SECONDS*4
            cfg["osakaTime"] = start_time + STAGGER_STEP_SECONDS*5
            scheduled_time = cfg["osakaTime"]
            effective_delay_seconds = effective_delay_seconds + STAGGER_STEP_SECONDS*5
        elif effective_target == "allSame":
            for key in ("shanghaiTime", "cancunTime", "fixHeaderTime", "posaTime", "pragueTime", "osakaTime"):
                cfg[key] = start_time
            scheduled_time = start_time
        else:
            print(f"unsupported upgrade target: {effective_target}", file=sys.stderr)
            return 1
        if any(cfg.get(key, 0) > 0 for key in ("cancunTime", "pragueTime", "osakaTime")):
            ensure_blob_schedule(cfg)
    elif mode == "smoke":
        effective_target = canonical_smoke_case(target_raw)
        if not effective_target:
            print(
                "smoke mode requires target in {poa,poa_shanghai,poa_shanghai_cancun,poa_shanghai_cancun_fixheader,poa_shanghai_cancun_fixheader_posa,poa_shanghai_cancun_fixheader_posa_prague,poa_shanghai_cancun_fixheader_posa_prague_osaka,poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1,poa_shanghai_cancun_fixheader_posa_prague_osaka_bpo1_bpo2}",
                file=sys.stderr,
            )
            return 1
        for key in FORK_FIELDS:
            cfg.pop(key, None)
        cfg.pop("blobSchedule", None)

        for key in SMOKE_STATIC_CASES[effective_target]:
            cfg[key] = 0
        if any(key in SMOKE_STATIC_CASES[effective_target] for key in ("cancunTime", "pragueTime", "osakaTime", "bpo1Time", "bpo2Time")):
            ensure_blob_schedule(cfg)
        effective_delay_seconds = 0
    else:
        print(f"unsupported mode: {mode}", file=sys.stderr)
        return 1

    validate_prefix_schedule(mode, cfg)

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
