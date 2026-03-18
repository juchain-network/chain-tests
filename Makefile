SHELL := /bin/bash

.PHONY: all help init-config image init run ready reset stop clean logs status \
        precheck runtime-precheck \
        net-up net-down net-reset net-ready test \
        test-group test-smoke test-fork test-scenario test-regression test-perf \
        ci ci-tool ci-budget-suggest ci-budget-suggest-json ci-budget-suggest-save ci-budget-drift-check ci-budget-selftest ci-budget-enforced

PWD := $(shell pwd)
SCRIPTS_DIR := scripts
DATA_DIR := data
NETWORK_DISPATCH := scripts/network/dispatch.sh
CI_TOOL := go run ./ci.go
EPOCH_RESOLVER := $(SCRIPTS_DIR)/resolve_epoch.sh

# Runtime/backend config (docker/native selection)
TEST_ENV_CONFIG ?= config/test_env.yaml
RUNTIME_SESSION_FILE ?=

# Test runner config consumed by ci.go
TEST_CONFIG ?= data/test_config.yaml
GOCACHE ?=
REPORT_DIR ?=
DEBUG ?=
GROUPS ?=
GROUP ?=
TESTS ?=
RUN ?=
TIMEOUT ?=
CI_LOG ?=
PKGS ?=
ARGS ?=
EPOCH ?=
TOPOLOGY ?=
INIT_MODE ?=
INIT_TARGET ?=
INIT_DELAY_SECONDS ?=
FORK_CASES ?=
FORK_DELAY_SECONDS ?=
FORK_UPGRADE_STARTUP_BUFFER_SINGLE ?= 5
FORK_UPGRADE_STARTUP_BUFFER_MULTI ?= 30
FORK_TEST_TIMEOUT ?= 20m
FORK_REPORT_DIR ?=
MATRIX ?= 0
SMOKE_CASES ?= poa,poa_shanghai,poa_shanghai_cancun,poa_shanghai_cancun_fixheader,poa_shanghai_cancun_fixheader_posa
SMOKE_TOPOLOGY ?=
SMOKE_REPORT_DIR ?=
SMOKE_SINGLE_IMPL ?=
SMOKE_SINGLE_AUTH_MODE ?=
SMOKE_SINGLE_GENESIS_MODE ?=
SMOKE_SINGLE_FORK_TARGET ?=
SMOKE_SINGLE_OBSERVE_SECONDS ?=
SMOKE_SINGLE_TEST_TIMEOUT ?= 12m
PERF_TPS_TIERS ?= 10,30,60
PERF_TIER_DURATION ?= 90s
PERF_SAMPLE_INTERVAL ?= 2s
PERF_SOAK_DURATION ?= 24h
PERF_SOAK_TPS ?= 10
PERF_SOAK_RESTART_INTERVAL ?= 1h
SCENARIO ?=
CHECK ?= all
SCOPE ?= core
PROFILE ?=
MODE ?=
BUDGET ?= 0
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
MAX_SKIPS ?=
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

CI_COMMON_FLAGS := $(if $(DEBUG),-debug,) $(if $(GOCACHE),-gocache $(GOCACHE),) $(if $(TEST_CONFIG),-config $(TEST_CONFIG),) $(if $(REPORT_DIR),-report-dir $(REPORT_DIR),) $(if $(filter 1 true yes,$(SKIP_SETUP)),-skip-setup,) $(if $(filter 1 true yes,$(SHARED_SETUP)),-shared-setup,) $(if $(SHARED_GROUPS),-shared-groups $(SHARED_GROUPS),) $(if $(SLOW_TOP),-slow-top $(SLOW_TOP),) $(if $(SLOW_THRESHOLD),-slow-threshold $(SLOW_THRESHOLD),) $(if $(filter 1 true yes,$(SLOW_FAIL)),-slow-fail,) $(if $(GROUP_THRESHOLDS),-group-thresholds $(GROUP_THRESHOLDS),) $(if $(filter 1 true yes,$(GROUP_THRESHOLD_FAIL)),-group-threshold-fail,) $(if $(MAX_SKIPS),-max-skips $(MAX_SKIPS),)

backend_cmd = RUNTIME_BACKEND="$${RUNTIME_BACKEND:-$$(awk '/^[[:space:]]*backend:[[:space:]]*/{print $$2; exit}' "$(TEST_ENV_CONFIG)" 2>/dev/null | sed 's/\"//g')}"; \
	if [ -z "$$RUNTIME_BACKEND" ]; then RUNTIME_BACKEND=native; fi

all: help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Network Commands:"
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
	@echo "Direct Backend-Routed Commands:"
	@echo "  net-up net-down net-reset net-ready"
	@echo ""
	@echo "Primary Test Commands:"
	@echo "  test            - Run the prepared network in a single go test pass (expects ready network)"
	@echo "  test-group      - Run one business group: GROUP=config|governance|staking|delegation|punish|rewards|epoch|all"
	@echo "  test-smoke      - Smoke runs: TOPOLOGY=single|multi|all MATRIX=0|1 (default: multi, MATRIX=0)"
	@echo "  test-fork       - Fork matrix runs: TOPOLOGY=single|multi|all (default: multi)"
	@echo "  test-scenario   - Scenario runs: SCENARIO=posa|interop CHECK=sync|state-root|all"
	@echo "  test-regression - Regression bundles: SCOPE=core|full (default: core)"
	@echo "  test-perf       - Perf/soak runs: MODE=tiers|soak"
	@echo ""
	@echo "CI Commands:"
	@echo "  ci              - PROFILE=pr|nightly|release|weekly-soak or MODE=groups|tests [BUDGET=1]"
	@echo "  ci-tool         - Pass raw flags to ci.go via ARGS=..."
	@echo ""
	@echo "Utilities:"
	@echo "  ci-budget-suggest ci-budget-suggest-json ci-budget-suggest-save"
	@echo "  ci-budget-drift-check ci-budget-selftest ci-budget-enforced"
	@echo ""
	@echo "Key Variables:"
	@echo "  TEST_ENV_CONFIG=$(TEST_ENV_CONFIG)"
	@echo "  RUNTIME_SESSION_FILE=$(RUNTIME_SESSION_FILE) # optional override for runtime session snapshot path"
	@echo "  TEST_CONFIG=$(TEST_CONFIG)"
	@echo "  GROUP=$(GROUP)                   # test-group selector"
	@echo "  GROUPS=$(GROUPS)                 # ci MODE=groups group list override"
	@echo "  TOPOLOGY=$(TOPOLOGY)             # init/test-smoke/test-fork: single|multi|all"
	@echo "  MATRIX=$(MATRIX)                 # test-smoke: 0|1"
	@echo "  SCENARIO=$(SCENARIO)             # test-scenario: posa|interop"
	@echo "  CHECK=$(CHECK)                   # test-scenario interop check: sync|state-root|all"
	@echo "  SCOPE=$(SCOPE)                   # test-regression: core|full"
	@echo "  PROFILE=$(PROFILE)               # ci profile: pr|nightly|release|weekly-soak"
	@echo "  MODE=$(MODE)                     # ci/test-perf mode selector"
	@echo "  BUDGET=$(BUDGET)                 # ci MODE=groups|tests with budget gate enabled"
	@echo "  EPOCH=$(EPOCH)                   # optional runtime epoch override for init/test commands"
	@echo "  INIT_MODE=$(INIT_MODE)           # init-only: poa|posa|smoke|upgrade"
	@echo "  INIT_TARGET=$(INIT_TARGET)       # init-only: smoke/upgrade target case"
	@echo "  INIT_DELAY_SECONDS=$(INIT_DELAY_SECONDS) # init-only: upgrade delay seconds"
	@echo "  RUN=$(RUN) TESTS=$(TESTS) PKGS=$(PKGS) TIMEOUT=$(TIMEOUT)"
	@echo "  FORK_CASES=$(FORK_CASES)         # e.g. poa,upgrade:shanghaiTime,upgrade:allStaggered,upgrade:allSame,posa"
	@echo "  FORK_DELAY_SECONDS=$(FORK_DELAY_SECONDS) # optional override; empty -> use config network.fork_delay_seconds"
	@echo "  FORK_TEST_TIMEOUT=$(FORK_TEST_TIMEOUT) FORK_REPORT_DIR=$(FORK_REPORT_DIR)"
	@echo "  SMOKE_CASES=$(SMOKE_CASES) SMOKE_REPORT_DIR=$(SMOKE_REPORT_DIR)"
	@echo "  SMOKE_SINGLE_IMPL=$(SMOKE_SINGLE_IMPL) # optional override: geth|reth; empty -> use config runtime.*"
	@echo "  SMOKE_SINGLE_AUTH_MODE=$(SMOKE_SINGLE_AUTH_MODE) # optional override: auto|private_key|keystore; empty -> use config validator_auth.mode"
	@echo "  SMOKE_SINGLE_GENESIS_MODE=$(SMOKE_SINGLE_GENESIS_MODE) # optional: poa|posa|smoke|upgrade"
	@echo "  SMOKE_SINGLE_FORK_TARGET=$(SMOKE_SINGLE_FORK_TARGET) # required when SMOKE_SINGLE_GENESIS_MODE=smoke|upgrade"
	@echo "  SMOKE_SINGLE_OBSERVE_SECONDS=$(SMOKE_SINGLE_OBSERVE_SECONDS) # optional override; empty -> use config tests.smoke.observe_seconds"
	@echo "  SMOKE_SINGLE_TEST_TIMEOUT=$(SMOKE_SINGLE_TEST_TIMEOUT) # single-smoke go test timeout"
	@echo "  PERF_TPS_TIERS=$(PERF_TPS_TIERS) PERF_TIER_DURATION=$(PERF_TIER_DURATION)"
	@echo "  PERF_SOAK_DURATION=$(PERF_SOAK_DURATION) PERF_SOAK_TPS=$(PERF_SOAK_TPS)"
	@echo "  REPORT_DIR=$(REPORT_DIR) REGRESSION_REPORT_DIR=$(REGRESSION_REPORT_DIR)"
	@echo "  See README.md for the full variable reference."
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
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
		TEST_NETWORK_EPOCH="$(EPOCH)" \
		TOPOLOGY="$(TOPOLOGY)" \
		INIT_MODE="$(INIT_MODE)" \
		INIT_TARGET="$(INIT_TARGET)" \
		INIT_DELAY_SECONDS="$(INIT_DELAY_SECONDS)" \
		RUNTIME_BACKEND="$(RUNTIME_BACKEND)" \
		bash $(SCRIPTS_DIR)/gen_network_config.sh; \
	session_cfg="$${RUNTIME_SESSION_FILE:-data/runtime_session.yaml}"; \
	TEST_ENV_CONFIG="$$session_cfg" RUNTIME_SESSION_FILE="$$session_cfg" "$(NETWORK_DISPATCH)" init

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
	resolved_backend="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" resolve-backend)"; \
	session_cfg="$${RUNTIME_SESSION_FILE:-data/runtime_session.yaml}"; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" RUNTIME_SESSION_REQUIRED=1 bash $(SCRIPTS_DIR)/runtime_precheck.sh; \
	echo "🚀 Starting network backend=$$resolved_backend"; \
	if [ "$$resolved_backend" = "docker" ]; then \
		TEST_ENV_CONFIG="$$session_cfg" bash $(SCRIPTS_DIR)/build_docker.sh; \
	fi; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" up; \
	if [ "$$resolved_backend" = "docker" ]; then \
		bash $(SCRIPTS_DIR)/ensure_miners.sh || true; \
	fi; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" ready

ready:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" ready

stop:
	@set -e; \
	resolved_backend="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" resolve-backend)"; \
	echo "🛑 Stopping network backend=$$resolved_backend"; \
	TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" down

reset: clean init run ready

clean:
	@$(MAKE) --no-print-directory stop >/dev/null 2>&1 || true
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
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" NODE="$(NODE)" "$(NETWORK_DISPATCH)" logs

status:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" status

net-up:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" up

net-down:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" down

net-reset:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" reset

net-ready:
	@TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" RUNTIME_SESSION_FILE="$(RUNTIME_SESSION_FILE)" "$(NETWORK_DISPATCH)" ready

test-group:
	@set -e; \
	group="$(GROUP)"; \
	if [ -z "$$group" ]; then \
		echo "Set GROUP=<config|governance|staking|delegation|punish|rewards|epoch|all>"; \
		exit 1; \
	fi; \
	case "$$group" in \
		all) \
			$(CI_TOOL) -mode groups $(CI_COMMON_FLAGS) -groups "$(CI_DEFAULT_GROUPS)" $(if $(CI_LOG),-ci-log,); \
			;; \
		config) \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups config)"; \
			echo "⏱ config epoch=$$epoch"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/config -run "TestA_SystemConfigSetup|TestB_ConfigBoundaryChecks"; \
			;; \
		governance) \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups governance)"; \
			echo "⏱ governance epoch=$$epoch"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/governance -run "TestB_Governance.*"; \
			;; \
		staking) \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups staking)"; \
			echo "⏱ staking epoch=$$epoch"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/staking -run "TestC_Staking.*|TestD_Staking.*"; \
			;; \
		delegation) \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups delegation)"; \
			echo "⏱ delegation epoch=$$epoch"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/delegation -run "TestE_Delegation.*"; \
			;; \
		punish) \
			echo "🧪 Running Punishment Test Group..."; \
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
			fi; \
			;; \
		rewards) \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups rewards)"; \
			echo "⏱ rewards epoch=$$epoch"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/rewards -run "TestH_Robustness|TestI_ConsensusRewards|TestI_PublicQueryCoverage|TestI_ValidatorExtras"; \
			;; \
		epoch) \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups epoch)"; \
			echo "⏱ epoch group epoch=$$epoch"; \
			echo "⏱ epoch phase-1: non-destructive checks"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/epoch -run "TestY_UpdateActiveValidatorSet|TestZ_UpgradesAndInitGuards|TestZ_SystemInitSecurityGuards"; \
			echo "⏱ epoch phase-2: destructive last-man-standing"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/epoch -run "TestZ_LastManStanding"; \
			;; \
		*) \
			echo "Unsupported GROUP=$$group"; \
			echo "Expected one of: config governance staking delegation punish rewards epoch all"; \
			exit 1; \
			;; \
	esac

test-smoke:
	@set -e; \
	topology="$(if $(TOPOLOGY),$(TOPOLOGY),multi)"; \
	matrix="$(if $(MATRIX),$(MATRIX),0)"; \
	case "$$matrix" in \
		1|true|yes|on) matrix=1 ;; \
		0|false|no|off|"") matrix=0 ;; \
		*) echo "MATRIX must be 0|1|true|false"; exit 1 ;; \
	esac; \
	case "$$topology" in \
		single|multi|all) ;; \
		*) echo "TOPOLOGY must be single|multi|all"; exit 1 ;; \
	esac; \
	if [ "$$matrix" = "1" ]; then \
		if [ "$$topology" = "all" ]; then \
			report_root="$(if $(SMOKE_REPORT_DIR),$(SMOKE_REPORT_DIR),reports/smoke_matrix_$$(date +%Y%m%d_%H%M%S))"; \
			echo "📦 smoke matrix report dir=$$report_root"; \
			$(MAKE) SMOKE_REPORT_DIR="$$report_root/single" TOPOLOGY=single MATRIX=1 test-smoke; \
			$(MAKE) SMOKE_REPORT_DIR="$$report_root/multi" TOPOLOGY=multi MATRIX=1 test-smoke; \
		elif [ "$$topology" = "single" ]; then \
			TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
			SMOKE_CASES="$(SMOKE_CASES)" \
			SMOKE_TOPOLOGY="single" \
			SMOKE_REPORT_DIR="$(SMOKE_REPORT_DIR)" \
			bash ./scripts/smoke/run_matrix.sh single; \
		else \
			TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
			SMOKE_CASES="$(SMOKE_CASES)" \
			SMOKE_TOPOLOGY="multi" \
			SMOKE_REPORT_DIR="$(SMOKE_REPORT_DIR)" \
			bash ./scripts/smoke/run_matrix.sh multi; \
		fi; \
	else \
		if [ "$$topology" = "all" ]; then \
			echo "TOPOLOGY=all is only supported when MATRIX=1"; \
			exit 1; \
		elif [ "$$topology" = "single" ]; then \
			TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
			SMOKE_SINGLE_IMPL="$(SMOKE_SINGLE_IMPL)" \
			SMOKE_SINGLE_AUTH_MODE="$(SMOKE_SINGLE_AUTH_MODE)" \
			SMOKE_SINGLE_GENESIS_MODE="$(SMOKE_SINGLE_GENESIS_MODE)" \
			SMOKE_SINGLE_FORK_TARGET="$(SMOKE_SINGLE_FORK_TARGET)" \
			SMOKE_SINGLE_OBSERVE_SECONDS="$(SMOKE_SINGLE_OBSERVE_SECONDS)" \
			SMOKE_SINGLE_TEST_TIMEOUT="$(SMOKE_SINGLE_TEST_TIMEOUT)" \
			bash ./scripts/smoke/run_single.sh; \
		else \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups smoke)"; \
			echo "⏱ smoke epoch=$$epoch"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/smoke -run "TestS_SmokeChainLivenessAllNodes"; \
		fi; \
	fi

test-fork:
	@set -e; \
	topology="$(if $(TOPOLOGY),$(TOPOLOGY),multi)"; \
	case "$$topology" in \
		single|multi|all) ;; \
		*) echo "TOPOLOGY must be single|multi|all"; exit 1 ;; \
	esac; \
	if [ "$$topology" = "all" ]; then \
		report_root="$(if $(FORK_REPORT_DIR),$(FORK_REPORT_DIR),reports/fork_$$(date +%Y%m%d_%H%M%S))"; \
		echo "📦 fork matrix report dir=$$report_root"; \
		$(MAKE) FORK_REPORT_DIR="$$report_root/single" TOPOLOGY=single test-fork; \
		$(MAKE) FORK_REPORT_DIR="$$report_root/multi" TOPOLOGY=multi test-fork; \
	else \
		TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
		FORK_CASES="$(FORK_CASES)" \
		FORK_DELAY_SECONDS="$(FORK_DELAY_SECONDS)" \
		FORK_UPGRADE_STARTUP_BUFFER_SINGLE="$(FORK_UPGRADE_STARTUP_BUFFER_SINGLE)" \
		FORK_UPGRADE_STARTUP_BUFFER_MULTI="$(FORK_UPGRADE_STARTUP_BUFFER_MULTI)" \
		FORK_TEST_TIMEOUT="$(FORK_TEST_TIMEOUT)" \
		FORK_REPORT_DIR="$(FORK_REPORT_DIR)" \
		bash ./scripts/fork/run_matrix.sh "$$topology"; \
	fi

test-scenario:
	@set -e; \
	scenario="$(SCENARIO)"; \
	check="$(if $(CHECK),$(CHECK),all)"; \
	if [ -z "$$scenario" ]; then \
		echo "Set SCENARIO=<posa|interop>"; \
		exit 1; \
	fi; \
	case "$$scenario" in \
		posa) \
			epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups posa)"; \
			echo "⏱ posa epoch=$$epoch"; \
			EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/posa -run "TestP_.*"; \
			bash ./scripts/report/assert_chain_health.sh; \
			;; \
		interop) \
			case "$$check" in \
				all) \
					$(MAKE) SCENARIO=interop CHECK=sync test-scenario; \
					$(MAKE) SCENARIO=interop CHECK=state-root test-scenario; \
					;; \
				sync) \
					epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups smoke)"; \
					echo "⏱ interop-sync epoch=$$epoch"; \
					EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/interop -run "TestI_SyncCatchUp"; \
					;; \
				state-root) \
					epoch="$$(TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" EPOCH="$(EPOCH)" bash $(EPOCH_RESOLVER) groups smoke)"; \
					echo "⏱ interop-state-root epoch=$$epoch"; \
					EPOCH="$$epoch" $(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -pkgs ./tests/interop -run "TestI_StateRootCheckpoint"; \
					;; \
				*) \
					echo "Unsupported CHECK=$$check"; \
					echo "Expected one of: sync state-root all"; \
					exit 1; \
					;; \
			esac; \
			;; \
		*) \
			echo "Unsupported SCENARIO=$$scenario"; \
			echo "Expected one of: posa interop"; \
			exit 1; \
			;; \
	esac

test-regression:
	@set -e; \
	scope="$(if $(SCOPE),$(SCOPE),core)"; \
	case "$$scope" in \
		core) \
			$(CI_TOOL) -mode all $(CI_COMMON_FLAGS); \
			;; \
		full) \
			reg_id="$$(date +%Y%m%d_%H%M%S)"; \
			reg_dir="$(if $(REGRESSION_REPORT_DIR),$(REGRESSION_REPORT_DIR),reports/regression_$$reg_id)"; \
			ci_dir="$$reg_dir/ci"; \
			fork_dir="$$reg_dir/fork"; \
			echo "📦 regression report dir=$$reg_dir"; \
			mkdir -p "$$ci_dir" "$$fork_dir"; \
			$(MAKE) REPORT_DIR="$$ci_dir" TOPOLOGY=multi MATRIX=0 test-smoke; \
			$(MAKE) REPORT_DIR="$$ci_dir" test-group GROUP=all; \
			$(MAKE) FORK_REPORT_DIR="$$fork_dir" TOPOLOGY=all test-fork; \
			$(MAKE) REPORT_DIR="$$ci_dir" test-scenario SCENARIO=posa; \
			$(MAKE) REPORT_DIR="$$ci_dir" test-scenario SCENARIO=interop CHECK=all; \
			python3 ./scripts/report/aggregate_reports.py --output-dir "$$reg_dir" --ci-dir "$$ci_dir" --fork-dir "$$fork_dir"; \
			;; \
		*) \
			echo "SCOPE must be core|full"; \
			exit 1; \
			;; \
	esac

test-perf:
	@set -e; \
	mode="$(MODE)"; \
	if [ -z "$$mode" ]; then \
		echo "Set MODE=<tiers|soak>"; \
		exit 1; \
	fi; \
	case "$$mode" in \
		tiers) \
			TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
			PERF_TPS_TIERS="$(PERF_TPS_TIERS)" \
			PERF_TIER_DURATION="$(PERF_TIER_DURATION)" \
			PERF_SAMPLE_INTERVAL="$(PERF_SAMPLE_INTERVAL)" \
			bash ./scripts/perf/run_tps_tiers.sh; \
			;; \
		soak) \
			TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" \
			PERF_SOAK_DURATION="$(PERF_SOAK_DURATION)" \
			PERF_SOAK_TPS="$(PERF_SOAK_TPS)" \
			PERF_SAMPLE_INTERVAL="$(PERF_SAMPLE_INTERVAL)" \
			PERF_SOAK_RESTART_INTERVAL="$(PERF_SOAK_RESTART_INTERVAL)" \
			bash ./scripts/perf/run_soak.sh; \
			;; \
		*) \
			echo "MODE must be tiers|soak for test-perf"; \
			exit 1; \
			;; \
	esac

test: ready
	@echo "🧪 Running Integration Tests (Single Pass)..."
	@$(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) -run "." -skip-setup

ci:
	@set -e; \
	profile="$(PROFILE)"; \
	mode="$(MODE)"; \
	budget="$(if $(BUDGET),$(BUDGET),0)"; \
	case "$$budget" in \
		1|true|yes|on) budget=1 ;; \
		0|false|no|off|"") budget=0 ;; \
		*) echo "BUDGET must be 0|1|true|false"; exit 1 ;; \
	esac; \
	if [ -n "$$profile" ] && [ -n "$$mode" ]; then \
		echo "Set either PROFILE or MODE, not both"; \
		exit 1; \
	fi; \
	if [ -n "$$profile" ]; then \
		case "$$profile" in \
			pr|nightly|release|weekly-soak) \
				TEST_ENV_CONFIG="$(TEST_ENV_CONFIG)" CI_PR_GROUPS="$(CI_PR_GROUPS)" CI_NIGHTLY_GROUPS="$(CI_NIGHTLY_GROUPS)" bash ./scripts/ci/run_profile.sh "$$profile"; \
				;; \
			*) \
				echo "PROFILE must be pr|nightly|release|weekly-soak"; \
				exit 1; \
				;; \
		esac; \
	elif [ -n "$$mode" ]; then \
		case "$$mode" in \
			groups) \
				if [ "$$budget" = "1" ]; then \
					$(CI_TOOL) -mode groups $(CI_COMMON_FLAGS) \
						-groups "$(if $(GROUPS),$(GROUPS),$(CI_DEFAULT_GROUPS))" \
						$(if $(CI_LOG),-ci-log,) \
						-slow-top $(if $(SLOW_TOP),$(SLOW_TOP),$(CI_BUDGET_SLOW_TOP)) \
						-slow-threshold $(if $(SLOW_THRESHOLD),$(SLOW_THRESHOLD),$(CI_BUDGET_SLOW_THRESHOLD)) \
						-slow-fail \
						-group-thresholds "$(if $(GROUP_THRESHOLDS),$(GROUP_THRESHOLDS),$(CI_BUDGET_GROUP_THRESHOLDS))" \
						-group-threshold-fail; \
				else \
					$(CI_TOOL) -mode groups $(CI_COMMON_FLAGS) -groups "$(if $(GROUPS),$(GROUPS),$(CI_DEFAULT_GROUPS))" $(if $(CI_LOG),-ci-log,); \
				fi; \
				;; \
			tests) \
				if [ -z "$(TESTS)" ] && [ -z "$(RUN)" ]; then \
					echo "Set TESTS or RUN"; \
					exit 1; \
				fi; \
				if [ "$$budget" = "1" ]; then \
					$(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) \
						$(if $(PKGS),-pkgs "$(PKGS)",) \
						$(if $(TESTS),-tests "$(TESTS)",) \
						$(if $(RUN),-run "$(RUN)",) \
						$(if $(TIMEOUT),-timeout $(TIMEOUT),) \
						-slow-top $(if $(SLOW_TOP),$(SLOW_TOP),$(CI_BUDGET_SLOW_TOP)) \
						-slow-threshold $(if $(SLOW_THRESHOLD),$(SLOW_THRESHOLD),$(CI_BUDGET_TEST_SLOW_THRESHOLD)) \
						-slow-fail; \
				else \
					$(CI_TOOL) -mode tests $(CI_COMMON_FLAGS) $(if $(PKGS),-pkgs "$(PKGS)",) $(if $(TESTS),-tests "$(TESTS)",) $(if $(RUN),-run "$(RUN)",) $(if $(TIMEOUT),-timeout $(TIMEOUT),); \
				fi; \
				;; \
			*) \
				echo "MODE must be groups|tests for ci"; \
				exit 1; \
				;; \
		esac; \
	else \
		echo "Set PROFILE=<pr|nightly|release|weekly-soak> or MODE=<groups|tests>"; \
		exit 1; \
	fi

ci-tool:
	@$(CI_TOOL) $(CI_COMMON_FLAGS) $(ARGS)

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

ci-budget-enforced: image ci-budget-drift-check
	@$(MAKE) ci MODE=groups BUDGET=1
