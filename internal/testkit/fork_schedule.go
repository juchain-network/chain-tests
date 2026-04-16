package testkit

import (
	"fmt"
	"strings"

	"juchain.org/chain/tools/ci/internal/config"
)

type forkSchedulePhase struct {
	name string
	at   int64
}

func UpgradeForkSchedulePhases(cfg *config.Config) []forkSchedulePhase {
	if cfg == nil {
		return nil
	}
	phases := []forkSchedulePhase{
		{name: "shanghai", at: cfg.Fork.Schedule.ShanghaiTime},
		{name: "cancun", at: cfg.Fork.Schedule.CancunTime},
		{name: "fixHeader", at: cfg.Fork.Schedule.FixHeaderTime},
		{name: "posa", at: cfg.Fork.Schedule.PosaTime},
		{name: "prague", at: cfg.Fork.Schedule.PragueTime},
		{name: "osaka", at: cfg.Fork.Schedule.OsakaTime},
	}
	out := make([]forkSchedulePhase, 0, len(phases))
	for _, phase := range phases {
		if phase.at > 0 {
			out = append(out, phase)
		}
	}
	return out
}

func ValidateUpgradeForkSchedule(cfg *config.Config) error {
	if cfg == nil || !strings.EqualFold(cfg.Fork.Mode, "upgrade") {
		return nil
	}
	phases := UpgradeForkSchedulePhases(cfg)
	if len(phases) == 0 {
		return fmt.Errorf("missing upgrade fork schedule")
	}
	for i := 1; i < len(phases); i++ {
		if phases[i-1].at > phases[i].at {
			return fmt.Errorf("invalid fork ordering: %s(%d) > %s(%d)", phases[i-1].name, phases[i-1].at, phases[i].name, phases[i].at)
		}
	}
	seenEnabled := false
	seenGap := false
	for _, phase := range []forkSchedulePhase{
		{name: "shanghai", at: cfg.Fork.Schedule.ShanghaiTime},
		{name: "cancun", at: cfg.Fork.Schedule.CancunTime},
		{name: "fixHeader", at: cfg.Fork.Schedule.FixHeaderTime},
		{name: "posa", at: cfg.Fork.Schedule.PosaTime},
		{name: "prague", at: cfg.Fork.Schedule.PragueTime},
		{name: "osaka", at: cfg.Fork.Schedule.OsakaTime},
	} {
		if phase.at <= 0 {
			if seenEnabled {
				seenGap = true
			}
			continue
		}
		if seenGap {
			return fmt.Errorf("invalid fork prefix: later phase %s(%d) is enabled after an earlier missing phase", phase.name, phase.at)
		}
		seenEnabled = true
	}
	return nil
}
