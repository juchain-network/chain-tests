package testkit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	testctx "juchain.org/chain/tools/ci/internal/context"
)

type LoopbackNetemConfig struct {
	Enabled bool
	Delay   string
	Jitter  string
	Loss    string
}

func LoopbackNetemConfigFromEnv() LoopbackNetemConfig {
	cfg := LoopbackNetemConfig{
		Delay:  "200ms",
		Jitter: "100ms",
		Loss:   "2%",
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LIVENESS_REPRO_NETEM_ENABLE"))) {
	case "1", "true", "yes", "on":
		cfg.Enabled = true
	}
	if raw := strings.TrimSpace(os.Getenv("LIVENESS_REPRO_NETEM_DELAY")); raw != "" {
		cfg.Delay = raw
	}
	if raw := strings.TrimSpace(os.Getenv("LIVENESS_REPRO_NETEM_JITTER")); raw != "" {
		cfg.Jitter = raw
	}
	if raw := strings.TrimSpace(os.Getenv("LIVENESS_REPRO_NETEM_LOSS")); raw != "" {
		cfg.Loss = raw
	}
	return cfg
}

func (c LoopbackNetemConfig) Summary() string {
	return fmt.Sprintf("delay=%s jitter=%s loss=%s", c.Delay, c.Jitter, c.Loss)
}

func nativeRepoRoot(c *testctx.CIContext) (string, error) {
	if c == nil || c.Config == nil {
		return "", fmt.Errorf("context not initialized")
	}
	if strings.TrimSpace(c.Config.SourcePath) == "" {
		return "", fmt.Errorf("config source path is empty")
	}
	return filepath.Dir(filepath.Dir(c.Config.SourcePath)), nil
}

func validatorP2PPort(c *testctx.CIContext, validator common.Address) (int, error) {
	idx, _, _, err := runtimeNodeForValidator(c, validator)
	if err != nil {
		return 0, err
	}
	repoRoot, err := nativeRepoRoot(c)
	if err != nil {
		return 0, err
	}
	nativeEnvFile := filepath.Join(repoRoot, "data", "native", ".env")
	raw := strings.TrimSpace(envValueFromFile(nativeEnvFile, fmt.Sprintf("VALIDATOR%d_P2P_PORT", idx+1)))
	if raw == "" {
		return 0, fmt.Errorf("validator p2p port missing for %s", validator.Hex())
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("invalid validator p2p port %q for %s", raw, validator.Hex())
	}
	return port, nil
}

func ApplyLoopbackNetemForValidators(c *testctx.CIContext, validators []common.Address, cfg LoopbackNetemConfig) (func() error, []int, error) {
	if c == nil || c.Config == nil {
		return nil, nil, fmt.Errorf("context not initialized")
	}
	if !cfg.Enabled {
		return func() error { return nil }, nil, nil
	}

	repoRoot, err := nativeRepoRoot(c)
	if err != nil {
		return nil, nil, err
	}

	ports := make([]int, 0, len(validators))
	seen := make(map[int]struct{}, len(validators))
	for _, validator := range validators {
		port, err := validatorP2PPort(c, validator)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	if len(ports) == 0 {
		return nil, nil, fmt.Errorf("no validator p2p ports resolved for loopback netem")
	}

	portArgs := make([]string, 0, len(ports))
	for _, port := range ports {
		portArgs = append(portArgs, strconv.Itoa(port))
	}

	script := filepath.Join(repoRoot, "scripts", "repro", "loopback_netem.sh")
	stateFile := filepath.Join(repoRoot, "data", "native", "loopback_netem.state")
	applyCmd := exec.Command(script,
		"apply",
		"--state-file", stateFile,
		"--ports", strings.Join(portArgs, ","),
		"--delay", cfg.Delay,
		"--jitter", cfg.Jitter,
		"--loss", cfg.Loss,
	)
	applyCmd.Dir = repoRoot
	if out, err := applyCmd.CombinedOutput(); err != nil {
		return nil, ports, fmt.Errorf("apply loopback netem failed: %w output=%s", err, strings.TrimSpace(string(out)))
	}

	cleanup := func() error {
		clearCmd := exec.Command(script, "clear", "--state-file", stateFile)
		clearCmd.Dir = repoRoot
		if out, err := clearCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("clear loopback netem failed: %w output=%s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return cleanup, ports, nil
}
