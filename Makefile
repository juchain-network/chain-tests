SHELL := /bin/bash

.PHONY: all help init-config image init run ready reset stop clean logs status \
        precheck runtime-precheck \
        net-up net-down net-reset net-ready test test-all test-all-legacy \
        test-smoke test-config test-governance test-staking test-delegation test-punish \
        test-rewards test-epoch test-fork-single test-fork-multi test-fork-all \
        test-posa-multi test-regression-all test-perf-tiers test-soak-24h \
        ci ci-tool ci-groups ci-groups-budget ci-tests ci-tests-budget ci-budget-suggest ci-budget-suggest-json ci-budget-suggest-save ci-budget-drift-check ci-budget-selftest ci-budget-enforced \
        ci-pr-gate ci-nightly-full ci-weekly-soak

PWD := $(shell pwd)
SCRIPTS_DIR := scripts
DATA_DIR := data
NETWORK_DISPATCH := scripts/network/dispatch.sh
CI_TOOL := go run ./ci.go
EPOCH_RESOLVER := $(SCRIPTS_DIR)/resolve_epoch.sh

# Runtime/backend config (docker/native selection)
TEST_ENV_CONFIG ?= config/test_env.yaml

# Test runner config consumed by ci.go
TEST_CONFIG ?= data/test_config.yaml
GOCACHE ?=
REPORT_DIR ?=
DEBUG ?=
GROUPS ?=
TESTS ?=
RUN ?=
TIMEOUT ?=
CI_LOG ?=
PKGS ?=
ARGS ?=
EPOCH ?=
FORK_CASES ?=
FORK_DELAY_SECONDS ?= 120
FORK_UPGRADE_STARTUP_BUFFER_SINGLE ?= 5
FORK_UPGRADE_STARTUP_BUFFER_MULTI ?= 30
FORK_TEST_TIMEOUT ?= 20m
FORK_REPORT_DIR ?=
PERF_TPS_TIERS ?= 10,30,60
PERF_TIER_DURATION ?= 90s
PERF_SAMPLE_INTERVAL ?= 2s
PERF_SOAK_DURATION ?= 24h
PERF_SOAK_TPS ?= 10
PERF_SOAK_RESTART_INTERVAL ?= 1h
REGRESSION_REPORT_DIR ?=
CI_PR_GROUPS ?= config,governance,staking,punish,epoch
CI_NIGHTLY_GROUPS ?= config,governance,staking,delegation,punish,rewards,epoch
SKIP_PRECHECK ?=
SKIP_SETUP ?=
SHARED_SETUP ?=
SHARED_GROUPS ?=
SLOW_TOP ?=
SLOW_THRESHOLD ?=
SLOW_FAIL ?=
GROUP_THRESHOLDS ?=
GROUP_THRESHOLD_FAIL ?=
CI_DEFAULT_GROUPS ?= config,governance,staking,delegation,punish,rewards,epoch
CI_BUDGET_GROUP_THRESHOLDS ?= config=6m,governance=15m,staking=12m,delegation=12m,punish=16m,rewards=14m,epoch=18m,default=15m
CI_BUDGET_SLOW_THRESHOLD ?= 45s
CI_BUDGET_SLOW_TOP ?= 30
CI_BUDGET_TEST_SLOW_THRESHOLD ?= 20s
BUDGET_RECOMMEND_RECENT ?= 120
BUDGET_RECOMMEND_GROUP_QUANTILE ?= 0.90
BUDGET_RECOMMEND_GROUP_HEADROOM ?= 1.30
BUDGET_RECOMMEND_SLOW_QUANTILE ?= 0.90
BUDGET_RECOMMEND_SLOW_HEADROOM ?= 1.40
BUDGET_RECOMMEND_MIN_GROUP_SAMPLES ?= 2
BUDGET_DRIFT_RATIO ?= 0.25
BUDGET_DRIFT_MIN_MS ?= 15000

# Optional local override generated from historical report analysis
-include config/ci_budget.local.mk

CI_COMMON_FLAGS := $(if $(DEBUG),-debug,) $(if $(GOCACHE),-gocache $(GOCACHE),) $(if $(TEST_CONFIG),-config $(TEST_CONFIG),) $(if $(REPORT_DIR),-report-dir $(REPORT_DIR),) $(if $(filter 1 true yes,$(SKIP_SETUP)),-skip-setup,) $(if $(filter 1 true yes,$(SHARED_SETUP)),-shared-setup,) $(if $(SHARED_GROUPS),-shared-groups $(SHARED_GROUPS),) $(if $(SLOW_TOP),-slow-top $(SLOW_TOP),) $(if $(SLOW_THRESHOLD),-slow-threshold $(SLOW_THRESHOLD),) $(if $(filter 1 true yes,$(SLOW_FAIL)),-slow-fail,) $(if $(GROUP_THRESHOLDS),-group-thresholds $(GROUP_THRESHOLDS),) $(if $(filter 1 true yes,$(GROUP_THRESHOLD_FAIL)),-group-threshold-fail,)

backend_cmd = RUNTIME_BACKEND="$${RUNTIME_BACKEND:-$$(awk '/^[[:space:]]*backend:[[:space:]]*/{print $$2; exit}' "$(TEST_ENV_CONFIG)" 2>/dev/null | sed 's/\"//g')}"; \
	if [ -z "$$RUNTIME_BACKEND" ]; then RUNTIME_BACKEND=native; fi

all: help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Network Targets:"
	@echo "  init-config     - Create config/test_env.yaml from example if missing"
	@echo "  image           - Build juchain binary for docker runtime"
	@echo "  init            - Generate genesis, keys, and runtime config"
	@echo "  run             - Start network (auto backend: docker/native)"
	@echo "  precheck        - Compile-only precheck for tests/tooling (cached by source fingerprint)"
	@echo "  runtime-precheck - Validate contract/congress/geth consistency before startup"
	@echo "  ready           - Wait for RPC readiness"
	@echo "  stop            - Stop network"
	@echo "  reset           - clean + init + run + ready"
	@echo "  clean           - Stop network and remove local runtime data"
	@echo "  logs            - View runtime logs (NODE=... optional)"
	@echo "  status          - Show runtime status"
	@echo ""
	@echo "Direct Backend-Routed Targets:"
	@echo "  net-up net-down net-reset net-ready"
	@echo ""
	@echo "Test Targets:"
	@echo "  test            - Run full suite in single pass (no setup)"
	@echo "  test-all        - Run all non-smoke tests with isolated reset per test"
	@echo "  test-smoke      - Quick smoke test (continuous tx + multi-node height growth)"
	@echo "  test-fork-single - Fork liveness matrix on native single-node topology"
	@echo "  test-fork-multi - Fork liveness matrix on configured multi-node backend"
	@echo "  test-fork-all   - Run fork liveness matrix for single and multi topology"
	@echo "  test-posa-multi - Deep PoSA multi-node regression scenarios"
	@echo "  test-regression-all - One-shot full regression orchestration + aggregate report"
	@echo "  test-perf-tiers - Run TPS tier perf profile and summary"
	@echo "  test-soak-24h   - Run long-soak profile and verdict report"
	@echo "  test-config     - System config tests"
	@echo "  test-governance - Governance tests"
	@echo "  test-staking    - Staking tests"
	@echo "  test-delegation - Delegation tests"
	@echo "  test-punish     - Punish/exit tests"
	@echo "  test-rewards    - Rewards/query tests"
	@echo "  test-epoch      - Epoch/upgrade tests"
	@echo ""
	@echo "CI Targets:"
	@echo "  ci ci-tool ci-groups ci-groups-budget ci-tests ci-tests-budget ci-budget-suggest ci-budget-suggest-json ci-budget-drift-check ci-budget-selftest ci-budget-enforced"
	@echo "  ci-pr-gate      - PR gate profile (smoke + key groups)"
	@echo "  ci-nightly-full - Nightly profile (full groups + fork-all + posa)"
	@echo "  ci-weekly-soak  - Weekly long-soak profile"
	@echo "  ci-groups-budget - Run group mode with default runtime budget gates enabled"
	@echo "  ci-tests-budget  - Run tests mode with default slow-test budget gate enabled"
	@echo "  ci-budget-suggest - Suggest budget thresholds from historical reports"
	@echo "  ci-budget-suggest-json - Output budget suggestion as machine-readable JSON"
	@echo "  ci-budget-suggest-save - Suggest and write CI_BUDGET_* overrides to config/ci_budget.local.mk"
	@echo "  ci-budget-drift-check - Compare suggestions with current CI_BUDGET_* and fail on large drift"
	@echo "  ci-budget-selftest - Run built-in self checks for budget recommendation script"
	@echo "  ci-budget-enforced - image + drift check + grouped budget-gated test run"
	@echo ""
	@echo "Variables:"
	@echo "  TEST_ENV_CONFIG=$(TEST_ENV_CONFIG)"
	@echo "  TEST_CONFIG=$(TEST_CONFIG)"
	@echo "  EPOCH=$(EPOCH)                     # optional runtime epoch override for init/reset"
	@echo "                                    # also overrides group/special epoch config when set"
	@echo "                                    # test-* epoch order: EPOCH > tests.epoch_overrides > profile.epoch > network.epoch"
	@echo "  FORK_CASES=$(FORK_CASES)         # e.g. poa,upgrade:shanghaiTime,upgrade:allStaggered,upgrade:allSame,posa"
	@echo "  FORK_DELAY_SECONDS=$(FORK_DELAY_SECONDS)"
	@echo "  FORK_UPGRADE_STARTUP_BUFFER_SINGLE=$(FORK_UPGRADE_STARTUP_BUFFER_SINGLE)"
	@echo "  FORK_UPGRADE_STARTUP_BUFFER_MULTI=$(FORK_UPGRADE_STARTUP_BUFFER_MULTI)"
	@echo "  FORK_TEST_TIMEOUT=$(FORK_TEST_TIMEOUT)"
	@echo "  FORK_REPORT_DIR=$(FORK_REPORT_DIR)"
	@echo "  PERF_TPS_TIERS=$(PERF_TPS_TIERS)"
	@echo "  PERF_TIER_DURATION=$(PERF_TIER_DURATION)"
	@echo "  PERF_SAMPLE_INTERVAL=$(PERF_SAMPLE_INTERVAL)"
	@echo "  PERF_SOAK_DURATION=$(PERF_SOAK_DURATION)"
	@echo "  PERF_SOAK_TPS=$(PERF_SOAK_TPS)"
	@echo "  PERF_SOAK_RESTART_INTERVAL=$(PERF_SOAK_RESTART_INTERVAL)"
	@echo "  REGRESSION_REPORT_DIR=$(REGRESSION_REPORT_DIR)"
	@echo "  SKIP_PRECHECK=$(SKIP_PRECHECK)     # set to 1 to bypass precheck before run"
	@echo "  SKIP_SETUP=$(SKIP_SETUP)           # set to 1 to skip clean/init/run/stop in tests mode (-run)"
	@echo "  SHARED_SETUP=$(SHARED_SETUP)       # set to 1 to share setup across compatible groups in ci-groups"
	@echo "  SHARED_GROUPS=$(SHARED_GROUPS)     # comma list of state-compatible groups allowed to share setup"
	@echo "  SLOW_TOP=$(SLOW_TOP)               # top-N slow tests in CI report"
	@echo "  SLOW_THRESHOLD=$(SLOW_THRESHOLD)   # duration threshold for slow alerts (e.g. 2s)"
	@echo "  SLOW_FAIL=$(SLOW_FAIL)             # 1/true/yes -> fail when slow threshold exceeded"
	@echo "  GROUP_THRESHOLDS=$(GROUP_THRESHOLDS) # e.g. config=2m,rewards=3m,default=4m"
	@echo "  GROUP_THRESHOLD_FAIL=$(GROUP_THRESHOLD_FAIL) # 1/true/yes -> fail on group overrun"
	@echo "  CI_BUDGET_GROUP_THRESHOLDS=$(CI_BUDGET_GROUP_THRESHOLDS)"
	@echo "  CI_BUDGET_SLOW_THRESHOLD=$(CI_BUDGET_SLOW_THRESHOLD)"
	@echo "  CI_BUDGET_SLOW_TOP=$(CI_BUDGET_SLOW_TOP)"
	@echo "  CI_BUDGET_TEST_SLOW_THRESHOLD=$(CI_BUDGET_TEST_SLOW_THRESHOLD)"
	@echo "  BUDGET_RECOMMEND_RECENT=$(BUDGET_RECOMMEND_RECENT)"
	@echo "  BUDGET_RECOMMEND_GROUP_QUANTILE=$(BUDGET_RECOMMEND_GROUP_QUANTILE)"
	@echo "  BUDGET_RECOMMEND_GROUP_HEADROOM=$(BUDGET_RECOMMEND_GROUP_HEADROOM)"
	@echo "  BUDGET_RECOMMEND_SLOW_QUANTILE=$(BUDGET_RECOMMEND_SLOW_QUANTILE)"
	@echo "  BUDGET_RECOMMEND_SLOW_HEADROOM=$(BUDGET_RECOMMEND_SLOW_HEADROOM)"
	@echo "  BUDGET_RECOMMEND_MIN_GROUP_SAMPLES=$(BUDGET_RECOMMEND_MIN_GROUP_SAMPLES)"
	@echo "  BUDGET_DRIFT_RATIO=$(BUDGET_DRIFT_RATIO)"
	@echo "  BUDGET_DRIFT_MIN_MS=$(BUDGET_DRIFT_MIN_MS)"
	@echo "  RUNTIME_BACKEND=(native|docker)  # optional override"

init-config:
	@if [ ! -f "$(TEST_ENV_CONFIG)" ]; then \
		if [ -f "config/test_env.yaml.example" ]; then \
			cp "config/test_env.yaml.example" "$(TEST_ENV_CONFIG)"; \
			echo "Created $(TEST_ENV_CONFIG) from example"; \
		else \
			echo "Missing config/test_env.yaml.example"; \
			exit 1; \
		fi; \
	else \
		echo "Config already exists: $(TEST_ENV_CONFIG)"; \
	fi

image:
	@echo "🚀 Building juchain binary for docker runtime..."
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" bash $(SCRIPTS_DIR)/build_docker.sh

init:
	@echo "⚙️  Generating network config/genesis..."
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" TEST_NETWORK_EPOCH="$(EPOCH)" bash $(SCRIPTS_DIR)/gen_network_config.sh

precheck:
	@echo "🔎 Running compile precheck..."
	@GOCACHE="$(if $(GOCACHE),$(GOCACHE),/tmp/go-build)" bash $(SCRIPTS_DIR)/precheck.sh

runtime-precheck:
	@$(backend_cmd); \
	echo "🔍 Running runtime consistency precheck..."; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" bash $(SCRIPTS_DIR)/runtime_precheck.sh

run:
	@set -e; \
	if [ -z "$(SKIP_PRECHECK)" ]; then \
		$(MAKE) precheck; \
	fi; \
	$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" bash $(SCRIPTS_DIR)/runtime_precheck.sh; \
	echo "🚀 Starting network backend=$$RUNTIME_BACKEND"; \
	if [ "$$RUNTIME_BACKEND" = "docker" ]; then \
		bash $(SCRIPTS_DIR)/build_docker.sh; \
	fi; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" up; \
	if [ "$$RUNTIME_BACKEND" = "docker" ]; then \
		bash $(SCRIPTS_DIR)/ensure_miners.sh || true; \
	fi; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" ready

ready:
	@$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" ready

stop:
	@$(backend_cmd); \
	echo "🛑 Stopping network backend=$$RUNTIME_BACKEND"; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" down

reset: clean init run ready

clean: stop
	@echo "🧹 Cleaning local runtime artifacts..."
	@if [ -d "$(DATA_DIR)" ]; then \
		rm -rf "$(DATA_DIR)" 2>/dev/null || true; \
		if [ -d "$(DATA_DIR)" ] && command -v docker >/dev/null 2>&1; then \
			docker run --rm -v "$(PWD)":/work alpine sh -c "rm -rf /work/$(DATA_DIR)" || true; \
		fi; \
	fi
	@rm -f tests.test
	@echo "✅ Clean complete."

logs:
	@$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" NODE="$(NODE)" "$(NETWORK_DISPATCH)" logs

status:
	@$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" status

net-up:
	@$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" up

net-down:
	@$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" down

net-reset:
	@$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" reset

net-ready:
	@$(backend_cmd); \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_BACKEND="$$RUNTIME_BACKEND" "$(NETWORK_DISPATCH)" ready

# Punish tests run in two isolated chunks to balance startup overhead and state stability.
test-punish:
	@echo "🧪 Running Punishment Test Group..."
	@set -e; \
	group_epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups punish)"; \
	paths_epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) specials punish_paths punish)"; \
	double_sign_epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) specials punish_double_sign punish)"; \
	if [ -z "$$paths_epoch" ]; then paths_epoch="$$group_epoch"; fi; \
	if [ -z "$$double_sign_epoch" ]; then double_sign_epoch="$$group_epoch"; fi; \
	echo "⏱ punish epochs: paths=$$paths_epoch double_sign=$$double_sign_epoch"; \
	if [ "$$paths_epoch" = "$$double_sign_epoch" ]; then \
		echo "⏱ punish running in single pass (shared epoch=$$paths_epoch)"; \
		EPOCH="$$paths_epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/punish -run "TestF1_ExitFlow|TestF2_QuickReEntry|TestF3_WithdrawProfits|TestF4_MiscExit|TestF5_RoleChange|TestF6_DoubleSignWindow|TestF7_PunishedRedemption|TestG_PunishPaths|TestG_DoubleSign"; \
	else \
		EPOCH="$$paths_epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/punish -run "TestF1_ExitFlow|TestF2_QuickReEntry|TestF3_WithdrawProfits|TestF4_MiscExit|TestF5_RoleChange|TestF6_DoubleSignWindow|TestF7_PunishedRedemption|TestG_PunishPaths"; \
		EPOCH="$$double_sign_epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/punish -run "TestG_DoubleSign"; \
	fi

test-config:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups config)"; \
	echo "⏱ config epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/config -run "TestA_SystemConfigSetup|TestB_ConfigBoundaryChecks"

test-smoke:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups smoke)"; \
	echo "⏱ smoke epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/smoke -run "TestS_SmokeChainLivenessAllNodes"

test-fork-single:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
		FORK_CASES="$(FORK_CASES)" \
		FORK_DELAY_SECONDS="$(FORK_DELAY_SECONDS)" \
		FORK_UPGRADE_STARTUP_BUFFER_SINGLE="$(FORK_UPGRADE_STARTUP_BUFFER_SINGLE)" \
		FORK_UPGRADE_STARTUP_BUFFER_MULTI="$(FORK_UPGRADE_STARTUP_BUFFER_MULTI)" \
		FORK_TEST_TIMEOUT="$(FORK_TEST_TIMEOUT)" \
		FORK_REPORT_DIR="$(FORK_REPORT_DIR)" \
		bash ./scripts/fork/run_matrix.sh single

test-fork-multi:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
		FORK_CASES="$(FORK_CASES)" \
		FORK_DELAY_SECONDS="$(FORK_DELAY_SECONDS)" \
		FORK_UPGRADE_STARTUP_BUFFER_SINGLE="$(FORK_UPGRADE_STARTUP_BUFFER_SINGLE)" \
		FORK_UPGRADE_STARTUP_BUFFER_MULTI="$(FORK_UPGRADE_STARTUP_BUFFER_MULTI)" \
		FORK_TEST_TIMEOUT="$(FORK_TEST_TIMEOUT)" \
		FORK_REPORT_DIR="$(FORK_REPORT_DIR)" \
		bash ./scripts/fork/run_matrix.sh multi

test-fork-all: test-fork-single test-fork-multi

test-governance:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups governance)"; \
	echo "⏱ governance epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/governance -run "TestB_Governance.*"

test-staking:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups staking)"; \
	echo "⏱ staking epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/staking -run "TestC_Staking.*|TestD_Staking.*"

test-delegation:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups delegation)"; \
	echo "⏱ delegation epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/delegation -run "TestE_Delegation.*"

test-rewards:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups rewards)"; \
	echo "⏱ rewards epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/rewards -run "TestH_Robustness|TestI_ConsensusRewards|TestI_PublicQueryCoverage|TestI_ValidatorExtras"

test-epoch:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups epoch)"; \
	echo "⏱ epoch group epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/epoch -run "TestY_UpdateActiveValidatorSet|TestZ_LastManStanding|TestZ_UpgradesAndInitGuards|TestZ_SystemInitSecurityGuards"

test-posa-multi:
	@set -e; \
	epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups posa)"; \
	echo "⏱ posa epoch=$$epoch"; \
	EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/posa -run "TestP_.*"

test-perf-tiers:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
		PERF_TPS_TIERS="$(PERF_TPS_TIERS)" \
		PERF_TIER_DURATION="$(PERF_TIER_DURATION)" \
		PERF_SAMPLE_INTERVAL="$(PERF_SAMPLE_INTERVAL)" \
		bash ./scripts/perf/run_tps_tiers.sh

test-soak-24h:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
		PERF_SOAK_DURATION="$(PERF_SOAK_DURATION)" \
		PERF_SOAK_TPS="$(PERF_SOAK_TPS)" \
		PERF_SAMPLE_INTERVAL="$(PERF_SAMPLE_INTERVAL)" \
		PERF_SOAK_RESTART_INTERVAL="$(PERF_SOAK_RESTART_INTERVAL)" \
		bash ./scripts/perf/run_soak.sh

test-regression-all:
	@set -e; \
	reg_id="$$(date +%Y%m%d_%H%M%S)"; \
	reg_dir="$(if $(REGRESSION_REPORT_DIR),$(REGRESSION_REPORT_DIR),reports/regression_$$reg_id)"; \
	ci_dir="$$reg_dir/ci"; \
	fork_dir="$$reg_dir/fork"; \
	echo "📦 regression report dir=$$reg_dir"; \
	mkdir -p "$$ci_dir" "$$fork_dir"; \
	$(MAKE) REPORT_DIR="$$ci_dir" test-smoke; \
	$(MAKE) REPORT_DIR="$$ci_dir" ci-groups GROUPS="$(CI_DEFAULT_GROUPS)"; \
	$(MAKE) FORK_REPORT_DIR="$$fork_dir" test-fork-all; \
	$(MAKE) REPORT_DIR="$$ci_dir" test-posa-multi; \
	python3 ./scripts/report/aggregate_reports.py --output-dir "$$reg_dir" --ci-dir "$$ci_dir" --fork-dir "$$fork_dir"

test-all:
	@$(CI_TOOL) -mode all $(CI_COMMON_FLAGS)

test-all-legacy:
	@$(CI_TOOL) -mode groups $(CI_COMMON_FLAGS) -groups $(CI_DEFAULT_GROUPS)

test: ready
	@echo "🧪 Running Integration Tests (Single Pass)..."
	@$(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -run "." -skip-setup

ci: image test-all

ci-tool:
	@$(CI_TOOL) $(CI_COMMON_FLAGS) $(ARGS)

ci-groups:
	@$(CI_TOOL) -mode groups $(CI_COMMON_FLAGS) -groups "$(if $(GROUPS),$(GROUPS),$(CI_DEFAULT_GROUPS))" $(if $(CI_LOG),-ci-log,)

ci-groups-budget:
	@$(CI_TOOL) -mode groups $(CI_COMMON_FLAGS) \
		-groups "$(if $(GROUPS),$(GROUPS),$(CI_DEFAULT_GROUPS))" \
		$(if $(CI_LOG),-ci-log,) \
		-slow-top $(if $(SLOW_TOP),$(SLOW_TOP),$(CI_BUDGET_SLOW_TOP)) \
		-slow-threshold $(if $(SLOW_THRESHOLD),$(SLOW_THRESHOLD),$(CI_BUDGET_SLOW_THRESHOLD)) \
		-slow-fail \
		-group-thresholds "$(if $(GROUP_THRESHOLDS),$(GROUP_THRESHOLDS),$(CI_BUDGET_GROUP_THRESHOLDS))" \
		-group-threshold-fail

ci-tests:
	@if [ -z "$(TESTS)" ] && [ -z "$(RUN)" ]; then echo "Set TESTS or RUN"; exit 1; fi
	@$(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) $(if $(PKGS),-pkgs "$(PKGS)",) $(if $(TESTS),-tests "$(TESTS)",) $(if $(RUN),-run "$(RUN)",) $(if $(TIMEOUT),-timeout $(TIMEOUT),)

ci-tests-budget:
	@if [ -z "$(TESTS)" ] && [ -z "$(RUN)" ]; then echo "Set TESTS or RUN"; exit 1; fi
	@$(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) \
		$(if $(PKGS),-pkgs "$(PKGS)",) \
		$(if $(TESTS),-tests "$(TESTS)",) \
		$(if $(RUN),-run "$(RUN)",) \
		$(if $(TIMEOUT),-timeout $(TIMEOUT),) \
		-slow-top $(if $(SLOW_TOP),$(SLOW_TOP),$(CI_BUDGET_SLOW_TOP)) \
		-slow-threshold $(if $(SLOW_THRESHOLD),$(SLOW_THRESHOLD),$(CI_BUDGET_TEST_SLOW_THRESHOLD)) \
		-slow-fail

ci-budget-suggest:
	@node $(SCRIPTS_DIR)/recommend_budgets.js \
		--reports-dir reports \
		--recent $(BUDGET_RECOMMEND_RECENT) \
		--group-quantile $(BUDGET_RECOMMEND_GROUP_QUANTILE) \
		--group-headroom $(BUDGET_RECOMMEND_GROUP_HEADROOM) \
		--min-group-samples $(BUDGET_RECOMMEND_MIN_GROUP_SAMPLES) \
		--slow-quantile $(BUDGET_RECOMMEND_SLOW_QUANTILE) \
		--slow-headroom $(BUDGET_RECOMMEND_SLOW_HEADROOM) \
		--current-group-thresholds "$(CI_BUDGET_GROUP_THRESHOLDS)" \
		--current-slow-threshold "$(CI_BUDGET_SLOW_THRESHOLD)" \
		--current-test-slow-threshold "$(CI_BUDGET_TEST_SLOW_THRESHOLD)" \
		--drift-ratio $(BUDGET_DRIFT_RATIO) \
		--drift-min-ms $(BUDGET_DRIFT_MIN_MS)

ci-budget-suggest-json:
	@node $(SCRIPTS_DIR)/recommend_budgets.js \
		--reports-dir reports \
		--recent $(BUDGET_RECOMMEND_RECENT) \
		--group-quantile $(BUDGET_RECOMMEND_GROUP_QUANTILE) \
		--group-headroom $(BUDGET_RECOMMEND_GROUP_HEADROOM) \
		--min-group-samples $(BUDGET_RECOMMEND_MIN_GROUP_SAMPLES) \
		--slow-quantile $(BUDGET_RECOMMEND_SLOW_QUANTILE) \
		--slow-headroom $(BUDGET_RECOMMEND_SLOW_HEADROOM) \
		--current-group-thresholds "$(CI_BUDGET_GROUP_THRESHOLDS)" \
		--current-slow-threshold "$(CI_BUDGET_SLOW_THRESHOLD)" \
		--current-test-slow-threshold "$(CI_BUDGET_TEST_SLOW_THRESHOLD)" \
		--drift-ratio $(BUDGET_DRIFT_RATIO) \
		--drift-min-ms $(BUDGET_DRIFT_MIN_MS) \
		--format json

ci-budget-suggest-save:
	@set -e; \
	mkdir -p config; \
	tmp="$$(mktemp)"; \
	node $(SCRIPTS_DIR)/recommend_budgets.js \
		--reports-dir reports \
		--recent $(BUDGET_RECOMMEND_RECENT) \
		--group-quantile $(BUDGET_RECOMMEND_GROUP_QUANTILE) \
		--group-headroom $(BUDGET_RECOMMEND_GROUP_HEADROOM) \
		--min-group-samples $(BUDGET_RECOMMEND_MIN_GROUP_SAMPLES) \
		--slow-quantile $(BUDGET_RECOMMEND_SLOW_QUANTILE) \
		--slow-headroom $(BUDGET_RECOMMEND_SLOW_HEADROOM) \
		--current-group-thresholds "$(CI_BUDGET_GROUP_THRESHOLDS)" \
		--current-slow-threshold "$(CI_BUDGET_SLOW_THRESHOLD)" \
		--current-test-slow-threshold "$(CI_BUDGET_TEST_SLOW_THRESHOLD)" \
		--drift-ratio $(BUDGET_DRIFT_RATIO) \
		--drift-min-ms $(BUDGET_DRIFT_MIN_MS) \
		--format make > "$$tmp"; \
	{ \
		echo "# Auto-generated by: make ci-budget-suggest-save"; \
		echo "# Generated at: $$(date -u +'%Y-%m-%dT%H:%M:%SZ')"; \
		cat "$$tmp"; \
	} > config/ci_budget.local.mk; \
	rm -f "$$tmp"; \
	echo "Saved budget overrides to config/ci_budget.local.mk"; \
	cat config/ci_budget.local.mk

ci-budget-drift-check:
	@node $(SCRIPTS_DIR)/recommend_budgets.js \
		--reports-dir reports \
		--recent $(BUDGET_RECOMMEND_RECENT) \
		--group-quantile $(BUDGET_RECOMMEND_GROUP_QUANTILE) \
		--group-headroom $(BUDGET_RECOMMEND_GROUP_HEADROOM) \
		--min-group-samples $(BUDGET_RECOMMEND_MIN_GROUP_SAMPLES) \
		--slow-quantile $(BUDGET_RECOMMEND_SLOW_QUANTILE) \
		--slow-headroom $(BUDGET_RECOMMEND_SLOW_HEADROOM) \
		--current-group-thresholds "$(CI_BUDGET_GROUP_THRESHOLDS)" \
		--current-slow-threshold "$(CI_BUDGET_SLOW_THRESHOLD)" \
		--current-test-slow-threshold "$(CI_BUDGET_TEST_SLOW_THRESHOLD)" \
		--drift-ratio $(BUDGET_DRIFT_RATIO) \
		--drift-min-ms $(BUDGET_DRIFT_MIN_MS) \
		--fail-on-drift

ci-budget-selftest:
	@node $(SCRIPTS_DIR)/recommend_budgets_selftest.js

ci-budget-enforced: image ci-budget-drift-check ci-groups-budget

ci-pr-gate:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" CI_PR_GROUPS="$(CI_PR_GROUPS)" bash ./scripts/ci/run_profile.sh pr

ci-nightly-full:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" CI_NIGHTLY_GROUPS="$(CI_NIGHTLY_GROUPS)" bash ./scripts/ci/run_profile.sh nightly

ci-weekly-soak:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" bash ./scripts/ci/run_profile.sh weekly_soak
