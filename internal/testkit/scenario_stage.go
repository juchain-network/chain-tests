package testkit

import (
	"os"
	"strings"
)

const scenarioStageFileEnv = "SCENARIO_STAGE_FILE"

func MarkScenarioStage(stage string) {
	stage = strings.TrimSpace(stage)
	if stage == "" {
		return
	}
	path := strings.TrimSpace(os.Getenv(scenarioStageFileEnv))
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte(stage+"\n"), 0o644)
}
