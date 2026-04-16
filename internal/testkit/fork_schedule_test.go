package testkit

import (
	"strings"
	"testing"

	"juchain.org/chain/tools/ci/internal/config"
)

func testUpgradeConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Fork.Mode = "upgrade"
	return cfg
}

func TestValidateUpgradeForkScheduleAcceptsContiguousPrefix(t *testing.T) {
	cfg := testUpgradeConfig()
	cfg.Fork.Target = "pragueTime"
	cfg.Fork.Schedule.ShanghaiTime = 100
	cfg.Fork.Schedule.CancunTime = 100
	cfg.Fork.Schedule.FixHeaderTime = 100
	cfg.Fork.Schedule.PosaTime = 100
	cfg.Fork.Schedule.PragueTime = 100

	if err := ValidateUpgradeForkSchedule(cfg); err != nil {
		t.Fatalf("expected valid contiguous prefix, got: %v", err)
	}
}

func TestValidateUpgradeForkScheduleRejectsMissingPrerequisite(t *testing.T) {
	cfg := testUpgradeConfig()
	cfg.Fork.Target = "osakaTime"
	cfg.Fork.Schedule.ShanghaiTime = 100
	cfg.Fork.Schedule.CancunTime = 100
	cfg.Fork.Schedule.FixHeaderTime = 100
	cfg.Fork.Schedule.PosaTime = 100
	cfg.Fork.Schedule.OsakaTime = 100

	err := ValidateUpgradeForkSchedule(cfg)
	if err == nil {
		t.Fatalf("expected missing prerequisite to fail")
	}
	if !strings.Contains(err.Error(), "invalid fork prefix") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUpgradeForkScheduleRejectsOutOfOrderPrerequisite(t *testing.T) {
	cfg := testUpgradeConfig()
	cfg.Fork.Target = "pragueTime"
	cfg.Fork.Schedule.ShanghaiTime = 100
	cfg.Fork.Schedule.CancunTime = 100
	cfg.Fork.Schedule.FixHeaderTime = 100
	cfg.Fork.Schedule.PosaTime = 300
	cfg.Fork.Schedule.PragueTime = 200

	err := ValidateUpgradeForkSchedule(cfg)
	if err == nil {
		t.Fatalf("expected out-of-order schedule to fail")
	}
	if !strings.Contains(err.Error(), "invalid fork ordering") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateUpgradeForkScheduleRejectsOsakaWithoutPrague(t *testing.T) {
	cfg := testUpgradeConfig()
	cfg.Fork.Target = "osakaTime"
	cfg.Fork.Schedule.ShanghaiTime = 100
	cfg.Fork.Schedule.CancunTime = 100
	cfg.Fork.Schedule.FixHeaderTime = 100
	cfg.Fork.Schedule.PosaTime = 100
	cfg.Fork.Schedule.PragueTime = 0
	cfg.Fork.Schedule.OsakaTime = 100

	err := ValidateUpgradeForkSchedule(cfg)
	if err == nil {
		t.Fatalf("expected osaka without prague to fail")
	}
	if !strings.Contains(err.Error(), "invalid fork prefix") {
		t.Fatalf("unexpected error: %v", err)
	}
}
