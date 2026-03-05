package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"gopkg.in/yaml.v3"
	intcfg "juchain.org/chain/tools/ci/internal/config"
)

type stepResult struct {
	Name      string
	Command   string
	LogPath   string
	Status    string
	Duration  time.Duration
	PassTests []string
	FailTests []string
	SkipTests []string
	Timed     []timedCase
}

type timedCase struct {
	Name     string
	Duration time.Duration
	Status   string
	Step     string
}

type groupDuration struct {
	Group     string
	Step      string
	Duration  time.Duration
	Threshold time.Duration
	Exceeded  bool
}

type reportOptions struct {
	SlowTop               int
	SlowThreshold         time.Duration
	GroupThresholds       map[string]time.Duration
	DefaultGroupThreshold time.Duration
}

type reportStats struct {
	SlowCaseAlerts int
	GroupAlerts    int
}

type summaryStep struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Duration  time.Duration `json:"duration"`
	PassCount int           `json:"pass_count"`
	FailCount int           `json:"fail_count"`
	SkipCount int           `json:"skip_count"`
	LogPath   string        `json:"log_path"`
}

type runSummary struct {
	GeneratedAt     string        `json:"generated_at"`
	Mode            string        `json:"mode"`
	Groups          []string      `json:"groups,omitempty"`
	Tests           []string      `json:"tests,omitempty"`
	RunPattern      string        `json:"run_pattern,omitempty"`
	ConfigPath      string        `json:"config_path"`
	ReportPath      string        `json:"report_path"`
	SlowCaseAlerts  int           `json:"slow_case_alerts"`
	GroupAlerts     int           `json:"group_alerts"`
	TotalPassTests  int           `json:"total_pass_tests"`
	TotalFailTests  int           `json:"total_fail_tests"`
	TotalSkipTests  int           `json:"total_skip_tests"`
	TotalStepCount  int           `json:"total_step_count"`
	FailedStepCount int           `json:"failed_step_count"`
	Status          string        `json:"status"`
	Steps           []summaryStep `json:"steps"`
}

type runManifest struct {
	GeneratedAt       string                   `json:"generated_at"`
	Mode              string                   `json:"mode"`
	RuntimeBackend    string                   `json:"runtime_backend"`
	RuntimeImpl       string                   `json:"runtime_impl"`
	RuntimeImplMode   string                   `json:"runtime_impl_mode"`
	RuntimeNodes      map[string]string        `json:"runtime_nodes,omitempty"`
	ValidatorAuthMode string                   `json:"validator_auth_mode,omitempty"`
	GitCommit         string                   `json:"git_commit"`
	GethVersion       string                   `json:"geth_version"`
	RethVersion       string                   `json:"reth_version"`
	GenesisHash       string                   `json:"genesis_hash"`
	ForkSchedule      map[string]any           `json:"fork_schedule"`
	CaseList          []string                 `json:"case_list"`
	ReproCommands     []string                 `json:"repro_commands"`
	ReportPath        string                   `json:"report_path"`
	SummaryPath       string                   `json:"summary_path"`
	Extra             map[string]any           `json:"extra,omitempty"`
	StepStatus        map[string]string        `json:"step_status"`
	StepLogs          map[string]string        `json:"step_logs"`
	StepDurationsSec  map[string]time.Duration `json:"step_durations"`
}

const nestedRunEnv = "CHAIN_TESTS_NESTED_RUN"

func main() {
	mode := flag.String("mode", "groups", "Run mode: groups or tests")
	groups := flag.String("groups", "config,governance,staking,delegation,punish,rewards,epoch", "Comma-separated group list")
	tests := flag.String("tests", "", "Comma-separated test names (e.g. TestB_Governance,TestZ_LastManStanding)")
	runPattern := flag.String("run", "", "go test -run pattern (used when -tests is empty)")
	pkgs := flag.String("pkgs", "./tests/...", "go test package pattern/path")
	timeout := flag.String("timeout", "30m", "go test timeout (tests mode only)")
	configPath := flag.String("config", "", "Path to test_config.yaml (default: test-integration/data/test_config.yaml)")
	reportDir := flag.String("report-dir", "reports", "Report output directory")
	gocache := flag.String("gocache", "/tmp/go-build", "GOCACHE path")
	debug := flag.Bool("debug", false, "Enable DEBUG logs (sets JUCHAIN_TEST_DEBUG=1)")
	skipSetup := flag.Bool("skip-setup", false, "Skip stop/clean/init/run/ready (tests mode with -run only)")
	sharedSetup := flag.Bool("shared-setup", false, "In groups mode, share one setup across explicit compatible groups")
	sharedGroups := flag.String("shared-groups", "", "Comma-separated state-compatible groups allowed to share setup (e.g. rewards,epoch)")
	runCiLog := flag.Bool("ci-log", false, "After grouped runs, execute make ci-log")
	slowTop := flag.Int("slow-top", 20, "Max rows in slow tests table")
	slowThresholdRaw := flag.String("slow-threshold", "0", "Mark tests >= duration as slow alert (e.g. 2s, 500ms); 0 disables")
	slowFail := flag.Bool("slow-fail", false, "Fail run when any test exceeds -slow-threshold")
	groupThresholdsRaw := flag.String("group-thresholds", "", "Comma-separated group thresholds (e.g. config=2m,rewards=3m,default=4m)")
	groupThresholdFail := flag.Bool("group-threshold-fail", false, "Fail run when any group duration exceeds configured threshold")
	maxSkips := flag.Int("max-skips", -1, "Maximum allowed skipped test count across all steps (-1 disables)")
	flag.Parse()

	slowThreshold, err := parseDurationFlag(*slowThresholdRaw)
	if err != nil {
		fmt.Printf("Invalid -slow-threshold: %v\n", err)
		os.Exit(1)
	}
	if *slowFail && slowThreshold <= 0 {
		fmt.Println("slow-fail requires -slow-threshold > 0")
		os.Exit(1)
	}
	groupThresholds, defaultGroupThreshold, err := parseGroupThresholds(*groupThresholdsRaw)
	if err != nil {
		fmt.Printf("Invalid -group-thresholds: %v\n", err)
		os.Exit(1)
	}
	if *groupThresholdFail && len(groupThresholds) == 0 && defaultGroupThreshold <= 0 {
		fmt.Println("group-threshold-fail requires -group-thresholds to be configured")
		os.Exit(1)
	}

	rootDir := callerDir()
	if rootDir == "" {
		fmt.Println("Failed to resolve working directory")
		os.Exit(1)
	}

	if !envTruthy(os.Getenv(nestedRunEnv)) {
		unlock, err := acquireRunLock(rootDir)
		if err != nil {
			fmt.Printf("Failed to acquire run lock: %v\n", err)
			os.Exit(1)
		}
		defer unlock()
	}

	if *configPath == "" {
		*configPath = filepath.Join(rootDir, "data", "test_config.yaml")
	} else if !filepath.IsAbs(*configPath) {
		*configPath = filepath.Join(rootDir, *configPath)
	}

	runID := time.Now().Format("20060102_150405")
	reportBase := *reportDir
	if !filepath.IsAbs(reportBase) {
		reportBase = filepath.Join(rootDir, reportBase)
	}
	runDir := filepath.Join(reportBase, "ci_"+runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		fmt.Printf("Failed to create report dir: %v\n", err)
		os.Exit(1)
	}

	env := append([]string{}, os.Environ()...)
	env = append(env, "GOCACHE="+*gocache)
	if *debug {
		env = append(env, "JUCHAIN_TEST_DEBUG=1")
	}
	sharedGroupSet := buildGroupSet(*sharedGroups)
	if *sharedSetup && len(sharedGroupSet) == 0 {
		fmt.Println("shared-setup enabled but shared-groups is empty; groups will run with isolated setup")
	}

	var results []stepResult
	var hadFailure bool

	switch strings.ToLower(*mode) {
	case "groups":
		groupList := splitList(*groups)
		sharedReady := false

		setupShared := func() bool {
			if sharedReady {
				return true
			}
			for _, step := range []struct {
				name string
				args []string
			}{
				{name: "shared_clean", args: []string{"clean"}},
				{name: "shared_init", args: []string{"init"}},
				{name: "shared_run", args: []string{"run"}},
			} {
				res := runStep(runDir, step.name, "make", step.args, env, rootDir)
				results = append(results, res)
				if res.Status != "PASS" {
					hadFailure = true
					return false
				}
			}
			sharedReady = true
			return true
		}

		teardownShared := func() {
			if !sharedReady {
				return
			}
			res := runStep(runDir, "shared_stop", "make", []string{"stop"}, env, rootDir)
			results = append(results, res)
			if res.Status != "PASS" {
				hadFailure = true
			}
			sharedReady = false
		}

		for _, group := range groupList {
			if group == "" {
				continue
			}

			useShared := *sharedSetup && isSharedSetupEligibleGroup(group, sharedGroupSet)
			groupEnv := env
			if useShared {
				if !setupShared() {
					break
				}
				groupEnv = append(append([]string{}, env...), "SKIP_SETUP=1")
			} else if sharedReady {
				teardownShared()
				if hadFailure {
					break
				}
			}

			name := "group_" + group
			cmd := "make"
			args := []string{"test-" + group}
			res := runStep(runDir, name, cmd, args, groupEnv, rootDir)
			results = append(results, res)
			if res.Status != "PASS" {
				hadFailure = true
				break
			}
		}
		if sharedReady {
			teardownShared()
		}
		if !hadFailure && *runCiLog {
			cmd := "make"
			args := []string{"ci-log"}
			res := runStep(runDir, "ci-log", cmd, args, env, filepath.Dir(rootDir))
			results = append(results, res)
			if res.Status != "PASS" {
				hadFailure = true
			}
		}
	case "tests":
		testList := splitList(*tests)
		if *skipSetup && len(testList) > 0 {
			fmt.Println("skip-setup can only be used with -run in tests mode")
			os.Exit(1)
		}
		if len(testList) == 0 && strings.TrimSpace(*runPattern) == "" {
			fmt.Println("tests mode requires -tests or -run")
			os.Exit(1)
		}

		if len(testList) > 0 {
			for _, testName := range testList {
				res := runSingleTest(runDir, testName, *pkgs, *timeout, *configPath, env, rootDir)
				results = append(results, res)
				if res.Status != "PASS" {
					hadFailure = true
					break
				}
			}
		} else {
			res := runPatternTest(runDir, *runPattern, *pkgs, *timeout, *configPath, env, rootDir, *skipSetup)
			results = append(results, res)
			if res.Status != "PASS" {
				hadFailure = true
			}
		}
	case "all":
		testList, err := discoverTests(rootDir)
		if err != nil {
			fmt.Printf("Failed to discover tests: %v\n", err)
			os.Exit(1)
		}
		if len(testList) == 0 {
			fmt.Println("No tests found")
			os.Exit(1)
		}
		for _, testName := range testList {
			res := runSingleTest(runDir, testName, *pkgs, *timeout, *configPath, env, rootDir)
			results = append(results, res)
			if res.Status != "PASS" {
				hadFailure = true
				break
			}
		}
	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		os.Exit(1)
	}

	reportPath := filepath.Join(runDir, "report.md")
	stats, err := writeReport(reportPath, *mode, *groups, *tests, *runPattern, *configPath, *gocache, *debug, results, reportOptions{
		SlowTop:               *slowTop,
		SlowThreshold:         slowThreshold,
		GroupThresholds:       groupThresholds,
		DefaultGroupThreshold: defaultGroupThreshold,
	})
	if err != nil {
		fmt.Printf("Failed to write report: %v\n", err)
		os.Exit(1)
	}

	if *slowFail && stats.SlowCaseAlerts > 0 {
		fmt.Printf("Slow threshold exceeded: %d test(s) >= %s\n", stats.SlowCaseAlerts, slowThreshold)
		hadFailure = true
	}
	if *groupThresholdFail && stats.GroupAlerts > 0 {
		fmt.Printf("Group threshold exceeded: %d group step(s)\n", stats.GroupAlerts)
		hadFailure = true
	}
	totalSkips := countTotalSkips(results)
	if *maxSkips >= 0 && totalSkips > *maxSkips {
		fmt.Printf("Skip budget exceeded: total skips=%d > max-skips=%d\n", totalSkips, *maxSkips)
		hadFailure = true
	}

	summaryPath := filepath.Join(runDir, "summary.json")
	summary := buildRunSummary(results, stats, *mode, *groups, *tests, *runPattern, *configPath, reportPath, hadFailure)
	if err := writeJSONFile(summaryPath, summary); err != nil {
		fmt.Printf("Failed to write summary: %v\n", err)
		os.Exit(1)
	}

	manifestPath := filepath.Join(runDir, "manifest.json")
	manifest := buildRunManifest(rootDir, results, *mode, *configPath, reportPath, summaryPath)
	if err := writeJSONFile(manifestPath, manifest); err != nil {
		fmt.Printf("Failed to write manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Report: %s\n", reportPath)
	fmt.Printf("Summary: %s\n", summaryPath)
	fmt.Printf("Manifest: %s\n", manifestPath)
	if hadFailure {
		os.Exit(1)
	}
}

func acquireRunLock(rootDir string) (func(), error) {
	lockPath := filepath.Join(rootDir, ".chain-tests-run.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lockFile.Close()
		return nil, fmt.Errorf("another ci/test run is in progress (%s)", lockPath)
	}

	return func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}, nil
}

func callerDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Dir(file)
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func buildGroupSet(raw string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, group := range splitList(raw) {
		key := strings.ToLower(strings.TrimSpace(group))
		if key != "" {
			out[key] = struct{}{}
		}
	}
	return out
}

func isSharedSetupEligibleGroup(group string, allowSet map[string]struct{}) bool {
	if len(allowSet) == 0 {
		return false
	}

	key := strings.ToLower(strings.TrimSpace(group))
	switch key {
	case "", "punish":
		// Punish keeps its split-test mode today.
		return false
	}
	_, ok := allowSet[key]
	return ok
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseDurationFlag(raw string) (time.Duration, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "", "0", "off", "none", "disabled":
		return 0, nil
	}
	dur, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if dur < 0 {
		return 0, fmt.Errorf("duration must be non-negative")
	}
	return dur, nil
}

func parseGroupThresholds(raw string) (map[string]time.Duration, time.Duration, error) {
	thresholds := make(map[string]time.Duration)
	var defaultThreshold time.Duration
	if strings.TrimSpace(raw) == "" {
		return thresholds, 0, nil
	}

	for _, item := range splitList(raw) {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, 0, fmt.Errorf("invalid item %q (expected key=duration)", item)
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		if key == "" {
			return nil, 0, fmt.Errorf("empty group key in %q", item)
		}
		dur, err := parseDurationFlag(parts[1])
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", key, err)
		}
		if key == "default" {
			defaultThreshold = dur
			continue
		}
		thresholds[key] = dur
	}
	return thresholds, defaultThreshold, nil
}

func discoverTests(rootDir string) ([]string, error) {
	testDir := filepath.Join(rootDir, "tests")
	var files []string
	err := filepath.WalkDir(testDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(testDir, path)
		if err == nil {
			rel = filepath.ToSlash(rel)
			if rel == "smoke" || strings.HasPrefix(rel, "smoke/") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	found := make(map[string]struct{})
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "func Test") {
				continue
			}
			name := strings.TrimSpace(strings.TrimPrefix(line, "func "))
			name = strings.SplitN(name, "(", 2)[0]
			if name == "TestMain" || name == "" {
				continue
			}
			found[name] = struct{}{}
		}
		_ = f.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}

	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func runSingleTest(runDir, testName, packagePattern, timeout, configPath string, env []string, workdir string) stepResult {
	name := "test_" + testName
	logPath := filepath.Join(runDir, name+".log")
	start := time.Now()
	status := "PASS"

	fmt.Printf("==> %s\n", name)

	logFile, err := os.Create(logPath)
	if err != nil {
		return stepResult{Name: name, Status: "FAIL", Command: "log create", LogPath: logPath}
	}
	defer logFile.Close()

	steps := [][]string{
		{"make", "clean"},
		{"make", "init"},
		{"make", "run"},
		{"go", "test", packagePattern, "-v", "-run", "^" + testName + "$", "-count=1", "-parallel=1", "-p", "1", "-timeout", timeout, "-config", configPath},
		{"make", "stop"},
	}

	for i, step := range steps {
		allowFail := step[0] == "make" && step[1] == "stop"
		if err := runLoggedCommand(logFile, step[0], step[1:], env, workdir, allowFail); err != nil {
			status = "FAIL"
			if i < len(steps)-1 {
				break
			}
		}
	}

	pass, fail, skip, timed := parseTestOutput(logPath)

	res := stepResult{
		Name:      name,
		Command:   "go test -run ^" + testName + "$",
		LogPath:   logPath,
		Status:    status,
		Duration:  time.Since(start),
		PassTests: pass,
		FailTests: fail,
		SkipTests: skip,
		Timed:     attachStepName(name, timed),
	}
	printResult(res)
	return res
}

func runPatternTest(runDir, pattern, packagePattern, timeout, configPath string, env []string, workdir string, skipSetup bool) stepResult {
	name := "test_run_pattern"
	logPath := filepath.Join(runDir, name+".log")
	start := time.Now()
	status := "PASS"

	fmt.Printf("==> %s\n", name)

	logFile, err := os.Create(logPath)
	if err != nil {
		return stepResult{Name: name, Status: "FAIL", Command: "log create", LogPath: logPath}
	}
	defer logFile.Close()

	steps := [][]string{
		{"go", "test", packagePattern, "-v", "-run", pattern, "-count=1", "-parallel=1", "-p", "1", "-timeout", timeout, "-config", configPath},
	}
	if !skipSetup {
		steps = [][]string{
			{"make", "clean"},
			{"make", "init"},
			{"make", "run"},
			steps[0],
			{"make", "stop"},
		}
	}

	for i, step := range steps {
		allowFail := step[0] == "make" && step[1] == "stop"
		if err := runLoggedCommand(logFile, step[0], step[1:], env, workdir, allowFail); err != nil {
			status = "FAIL"
			if i < len(steps)-1 {
				break
			}
		}
	}

	pass, fail, skip, timed := parseTestOutput(logPath)

	res := stepResult{
		Name:      name,
		Command:   "go test -run " + pattern,
		LogPath:   logPath,
		Status:    status,
		Duration:  time.Since(start),
		PassTests: pass,
		FailTests: fail,
		SkipTests: skip,
		Timed:     attachStepName(name, timed),
	}
	printResult(res)
	return res
}

func runStep(runDir, name, cmd string, args []string, env []string, workdir string) stepResult {
	logPath := filepath.Join(runDir, name+".log")
	start := time.Now()
	status := "PASS"

	fmt.Printf("==> %s\n", name)

	logFile, err := os.Create(logPath)
	if err != nil {
		return stepResult{Name: name, Status: "FAIL", Command: "log create", LogPath: logPath}
	}
	defer logFile.Close()

	if err := runLoggedCommand(logFile, cmd, args, env, workdir, false); err != nil {
		status = "FAIL"
	}

	pass, fail, skip, timed := parseTestOutput(logPath)

	res := stepResult{
		Name:      name,
		Command:   strings.Join(append([]string{cmd}, args...), " "),
		LogPath:   logPath,
		Status:    status,
		Duration:  time.Since(start),
		PassTests: pass,
		FailTests: fail,
		SkipTests: skip,
		Timed:     attachStepName(name, timed),
	}
	printResult(res)
	return res
}

func runLoggedCommand(logFile *os.File, cmd string, args []string, env []string, workdir string, allowFail bool) error {
	fullCmd := strings.Join(append([]string{cmd}, args...), " ")
	if _, err := fmt.Fprintf(logFile, ">>> %s\n", fullCmd); err != nil {
		return err
	}

	command := exec.Command(cmd, args...)
	command.Env = append(append([]string{}, env...), nestedRunEnv+"=1")
	command.Dir = workdir
	command.Stdout = logFile
	command.Stderr = logFile

	err := command.Run()
	if err != nil && !allowFail {
		return err
	}
	if allowFail && err != nil {
		_, _ = fmt.Fprintf(logFile, "⚠️  Ignored failure: %v\n", err)
	}
	return nil
}

func parseTestOutput(path string) ([]string, []string, []string, []timedCase) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, nil
	}
	defer file.Close()

	var pass []string
	var fail []string
	var skip []string
	var timed []timedCase

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "--- PASS: ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "--- PASS: "))
			pass = append(pass, value)
			if tc, ok := parseTimedCase(value, "PASS"); ok {
				timed = append(timed, tc)
			}
		} else if strings.HasPrefix(line, "--- FAIL: ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "--- FAIL: "))
			fail = append(fail, value)
			if tc, ok := parseTimedCase(value, "FAIL"); ok {
				timed = append(timed, tc)
			}
		} else if strings.HasPrefix(line, "--- SKIP: ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "--- SKIP: "))
			skip = append(skip, value)
			if tc, ok := parseTimedCase(value, "SKIP"); ok {
				timed = append(timed, tc)
			}
		}
	}
	return pass, fail, skip, timed
}

func parseTimedCase(raw, status string) (timedCase, bool) {
	l := strings.LastIndex(raw, " (")
	r := strings.LastIndex(raw, ")")
	if l <= 0 || r <= l+2 {
		return timedCase{}, false
	}
	name := strings.TrimSpace(raw[:l])
	durText := strings.TrimSpace(raw[l+2 : r])
	dur, err := time.ParseDuration(durText)
	if err != nil {
		return timedCase{}, false
	}
	return timedCase{Name: name, Duration: dur, Status: status}, true
}

func attachStepName(step string, items []timedCase) []timedCase {
	if len(items) == 0 {
		return nil
	}
	out := make([]timedCase, len(items))
	copy(out, items)
	for i := range out {
		out[i].Step = step
	}
	return out
}

func collectSlowCases(results []stepResult) []timedCase {
	var out []timedCase
	for _, res := range results {
		out = append(out, res.Timed...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Duration == out[j].Duration {
			if out[i].Step == out[j].Step {
				return out[i].Name < out[j].Name
			}
			return out[i].Step < out[j].Step
		}
		return out[i].Duration > out[j].Duration
	})
	return out
}

func collectSlowCaseAlerts(cases []timedCase, threshold time.Duration) []timedCase {
	if threshold <= 0 {
		return nil
	}
	var out []timedCase
	for _, tc := range cases {
		if tc.Duration >= threshold {
			out = append(out, tc)
		}
	}
	return out
}

func stepGroupName(step string) (string, bool) {
	if !strings.HasPrefix(step, "group_") {
		return "", false
	}
	name := strings.TrimSpace(strings.TrimPrefix(step, "group_"))
	if name == "" {
		return "", false
	}
	return name, true
}

func selectGroupThreshold(group string, thresholds map[string]time.Duration, defaultThreshold time.Duration) time.Duration {
	if len(thresholds) > 0 {
		if val, ok := thresholds[strings.ToLower(strings.TrimSpace(group))]; ok {
			return val
		}
	}
	return defaultThreshold
}

func collectGroupDurations(results []stepResult, thresholds map[string]time.Duration, defaultThreshold time.Duration) []groupDuration {
	var out []groupDuration
	for _, res := range results {
		group, ok := stepGroupName(res.Name)
		if !ok {
			continue
		}
		th := selectGroupThreshold(group, thresholds, defaultThreshold)
		item := groupDuration{
			Group:     group,
			Step:      res.Name,
			Duration:  res.Duration,
			Threshold: th,
			Exceeded:  th > 0 && res.Duration >= th,
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Duration == out[j].Duration {
			return out[i].Group < out[j].Group
		}
		return out[i].Duration > out[j].Duration
	})
	return out
}

func writeReport(path, mode, groups, tests, runPattern, configPath, gocache string, debug bool, results []stepResult, opts reportOptions) (reportStats, error) {
	if opts.SlowTop <= 0 {
		opts.SlowTop = 20
	}
	if opts.GroupThresholds == nil {
		opts.GroupThresholds = map[string]time.Duration{}
	}

	var sb strings.Builder
	stats := reportStats{}
	sb.WriteString("# Integration Test Report\n\n")
	sb.WriteString(fmt.Sprintf("- Generated: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- Mode: %s\n", mode))
	if strings.TrimSpace(groups) != "" {
		sb.WriteString(fmt.Sprintf("- Groups: %s\n", groups))
	}
	if strings.TrimSpace(tests) != "" {
		sb.WriteString(fmt.Sprintf("- Tests: %s\n", tests))
	}
	if strings.TrimSpace(runPattern) != "" {
		sb.WriteString(fmt.Sprintf("- Run Pattern: %s\n", runPattern))
	}
	sb.WriteString(fmt.Sprintf("- Config: %s\n", configPath))
	sb.WriteString(fmt.Sprintf("- GOCACHE: %s\n", gocache))
	sb.WriteString(fmt.Sprintf("- Debug: %t\n\n", debug))

	groupDurations := collectGroupDurations(results, opts.GroupThresholds, opts.DefaultGroupThreshold)
	if len(groupDurations) > 0 {
		sb.WriteString("## Group Runtime Profile\n\n")
		sb.WriteString("| Group | Step | Duration | Threshold | Budget |\n")
		sb.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, g := range groupDurations {
			thresholdText := "-"
			budget := "-"
			if g.Threshold > 0 {
				thresholdText = g.Threshold.String()
				if g.Exceeded {
					budget = "EXCEEDED"
					stats.GroupAlerts++
				} else {
					budget = "OK"
				}
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				g.Group,
				g.Step,
				g.Duration.Round(time.Second),
				thresholdText,
				budget,
			))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("| Step | Status | Duration | Skips | Log |\n")
	sb.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, res := range results {
		skips := "-"
		if len(res.SkipTests) > 0 {
			skips = fmt.Sprintf("%d", len(res.SkipTests))
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			res.Name,
			res.Status,
			res.Duration.Round(time.Second),
			skips,
			res.LogPath,
		))
	}

	slow := collectSlowCases(results)
	if len(slow) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Slow Tests (Top %d)\n\n", opts.SlowTop))
		sb.WriteString("| Rank | Test | Duration | Status | Step |\n")
		sb.WriteString("| --- | --- | --- | --- | --- |\n")
		limit := opts.SlowTop
		if len(slow) < limit {
			limit = len(slow)
		}
		for i := 0; i < limit; i++ {
			tc := slow[i]
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
				i+1,
				tc.Name,
				tc.Duration,
				tc.Status,
				tc.Step,
			))
		}
	}

	if opts.SlowThreshold > 0 {
		alerts := collectSlowCaseAlerts(slow, opts.SlowThreshold)
		stats.SlowCaseAlerts = len(alerts)
		sb.WriteString(fmt.Sprintf("\n## Slow Alerts (>= %s)\n\n", opts.SlowThreshold))
		if len(alerts) == 0 {
			sb.WriteString("No test case exceeded the slow threshold.\n")
		} else {
			sb.WriteString("| Rank | Test | Duration | Status | Step |\n")
			sb.WriteString("| --- | --- | --- | --- | --- |\n")
			for i, tc := range alerts {
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
					i+1,
					tc.Name,
					tc.Duration,
					tc.Status,
					tc.Step,
				))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n## Details\n\n")
	for _, res := range results {
		sb.WriteString(fmt.Sprintf("### %s\n\n", res.Name))
		sb.WriteString(fmt.Sprintf("- Status: %s\n", res.Status))
		sb.WriteString(fmt.Sprintf("- Duration: %s\n", res.Duration.Round(time.Second)))
		sb.WriteString(fmt.Sprintf("- Command: `%s`\n", res.Command))
		sb.WriteString(fmt.Sprintf("- Log: %s\n", res.LogPath))
		if len(res.FailTests) > 0 {
			sb.WriteString(fmt.Sprintf("- Failed Tests: %s\n", strings.Join(res.FailTests, ", ")))
		}
		if len(res.SkipTests) > 0 {
			sb.WriteString(fmt.Sprintf("- Skipped Tests: %s\n", strings.Join(res.SkipTests, ", ")))
		}
		if len(res.PassTests) > 0 {
			sb.WriteString(fmt.Sprintf("- Passed Tests: %d\n", len(res.PassTests)))
		}
		sb.WriteString("\n")
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return stats, err
	}
	return stats, nil
}

func printResult(res stepResult) {
	fmt.Printf("<== %s: %s (%s) log: %s\n", res.Name, res.Status, res.Duration.Round(time.Second), res.LogPath)
	printCaseList("PASS", res.PassTests)
	printCaseList("FAIL", res.FailTests)
	printCaseList("SKIP", res.SkipTests)
}

func printCaseList(label string, items []string) {
	if len(items) == 0 {
		return
	}
	list := append([]string{}, items...)
	sort.Strings(list)
	for _, item := range list {
		fmt.Printf("  %s: %s\n", label, item)
	}
}

func countTotalSkips(results []stepResult) int {
	total := 0
	for _, res := range results {
		total += len(res.SkipTests)
	}
	return total
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func buildRunSummary(results []stepResult, stats reportStats, mode, groups, tests, runPattern, configPath, reportPath string, failed bool) runSummary {
	summary := runSummary{
		GeneratedAt:    time.Now().Format(time.RFC3339),
		Mode:           mode,
		Groups:         splitList(groups),
		Tests:          splitList(tests),
		RunPattern:     strings.TrimSpace(runPattern),
		ConfigPath:     configPath,
		ReportPath:     reportPath,
		SlowCaseAlerts: stats.SlowCaseAlerts,
		GroupAlerts:    stats.GroupAlerts,
		TotalStepCount: len(results),
		Status:         "PASS",
	}
	if failed {
		summary.Status = "FAIL"
	}

	steps := make([]summaryStep, 0, len(results))
	for _, res := range results {
		steps = append(steps, summaryStep{
			Name:      res.Name,
			Status:    res.Status,
			Duration:  res.Duration.Round(time.Millisecond),
			PassCount: len(res.PassTests),
			FailCount: len(res.FailTests),
			SkipCount: len(res.SkipTests),
			LogPath:   res.LogPath,
		})
		summary.TotalPassTests += len(res.PassTests)
		summary.TotalFailTests += len(res.FailTests)
		summary.TotalSkipTests += len(res.SkipTests)
		if res.Status != "PASS" {
			summary.FailedStepCount++
		}
	}
	summary.Steps = steps
	return summary
}

func buildRunManifest(rootDir string, results []stepResult, mode, configPath, reportPath, summaryPath string) runManifest {
	caseSet := make(map[string]struct{})
	reproSet := make(map[string]struct{})
	stepStatus := make(map[string]string, len(results))
	stepLogs := make(map[string]string, len(results))
	stepDur := make(map[string]time.Duration, len(results))

	for _, res := range results {
		stepStatus[res.Name] = res.Status
		stepLogs[res.Name] = res.LogPath
		stepDur[res.Name] = res.Duration.Round(time.Millisecond)
		repro := strings.TrimSpace(res.Command)
		if repro != "" {
			reproSet["cd "+rootDir+" && "+repro] = struct{}{}
		}
		for _, name := range res.PassTests {
			caseSet[name] = struct{}{}
		}
		for _, name := range res.FailTests {
			caseSet[name] = struct{}{}
		}
		for _, name := range res.SkipTests {
			caseSet[name] = struct{}{}
		}
	}

	caseList := make([]string, 0, len(caseSet))
	for name := range caseSet {
		caseList = append(caseList, name)
	}
	sort.Strings(caseList)

	reproCommands := make([]string, 0, len(reproSet))
	for cmd := range reproSet {
		reproCommands = append(reproCommands, cmd)
	}
	sort.Strings(reproCommands)

	return runManifest{
		GeneratedAt:       time.Now().Format(time.RFC3339),
		Mode:              mode,
		RuntimeBackend:    detectRuntimeBackend(rootDir),
		RuntimeImpl:       detectRuntimeImpl(rootDir),
		RuntimeImplMode:   detectRuntimeImplMode(rootDir),
		RuntimeNodes:      detectRuntimeNodes(rootDir),
		ValidatorAuthMode: detectValidatorAuthMode(rootDir),
		GitCommit:         detectGitCommit(rootDir),
		GethVersion:       detectGethVersion(rootDir),
		RethVersion:       detectRethVersion(rootDir),
		GenesisHash:       detectGenesisHash(rootDir, configPath),
		ForkSchedule:      loadForkSchedule(configPath),
		CaseList:          caseList,
		ReproCommands:     reproCommands,
		ReportPath:        reportPath,
		SummaryPath:       summaryPath,
		StepStatus:        stepStatus,
		StepLogs:          stepLogs,
		StepDurationsSec:  stepDur,
	}
}

type testEnvLite struct {
	Runtime struct {
		Backend  string `yaml:"backend"`
		ImplMode string `yaml:"impl_mode"`
		Impl     string `yaml:"impl"`
	} `yaml:"runtime"`
	ValidatorAuth struct {
		Mode string `yaml:"mode"`
	} `yaml:"validator_auth"`
	RuntimeNodes map[string]string `yaml:"runtime_nodes"`
	Paths        struct {
		ChainRoot string `yaml:"chain_root"`
		RethRoot  string `yaml:"reth_root"`
	} `yaml:"paths"`
	Binaries struct {
		GethNative string `yaml:"geth_native"`
		RethNative string `yaml:"reth_native"`
	} `yaml:"binaries"`
}

func loadTestEnvLite(rootDir string) (*testEnvLite, error) {
	configPath := os.Getenv("TEST_ENV_CONFIG")
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(rootDir, "config", "test_env.yaml")
	}
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(rootDir, configPath)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg testEnvLite
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func detectRuntimeBackend(rootDir string) string {
	if v := strings.TrimSpace(os.Getenv("RUNTIME_BACKEND")); v != "" {
		return v
	}
	cfg, err := loadTestEnvLite(rootDir)
	if err != nil {
		return "native"
	}
	if v := strings.TrimSpace(cfg.Runtime.Backend); v != "" {
		return v
	}
	return "native"
}

func detectRuntimeImpl(rootDir string) string {
	cfg, err := loadTestEnvLite(rootDir)
	if err != nil {
		return "geth"
	}
	if v := strings.TrimSpace(cfg.Runtime.Impl); v != "" {
		return v
	}
	return "geth"
}

func detectRuntimeImplMode(rootDir string) string {
	cfg, err := loadTestEnvLite(rootDir)
	if err != nil {
		return "single"
	}
	if v := strings.TrimSpace(cfg.Runtime.ImplMode); v != "" {
		return v
	}
	return "single"
}

func detectRuntimeNodes(rootDir string) map[string]string {
	cfg, err := loadTestEnvLite(rootDir)
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(cfg.RuntimeNodes))
	for node, impl := range cfg.RuntimeNodes {
		node = strings.TrimSpace(node)
		impl = strings.TrimSpace(impl)
		if node == "" || impl == "" {
			continue
		}
		out[node] = impl
	}
	return out
}

func detectValidatorAuthMode(rootDir string) string {
	cfg, err := loadTestEnvLite(rootDir)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.ValidatorAuth.Mode)
}

func detectGitCommit(rootDir string) string {
	out, err := exec.Command("git", "-C", rootDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectGethVersion(rootDir string) string {
	cfg, err := loadTestEnvLite(rootDir)
	if err != nil {
		return ""
	}

	if explicit := strings.TrimSpace(cfg.Binaries.GethNative); explicit != "" {
		if !filepath.IsAbs(explicit) {
			explicit = filepath.Join(rootDir, explicit)
		}
		if version := detectBinaryVersion(explicit, "version"); version != "" {
			return firstLine(version)
		}
	}

	chainRoot := strings.TrimSpace(cfg.Paths.ChainRoot)
	if chainRoot == "" {
		chainRoot = "../chain-1.16/chain-1.16"
	}
	if !filepath.IsAbs(chainRoot) {
		chainRoot = filepath.Join(rootDir, chainRoot)
	}
	gethPath := filepath.Join(chainRoot, "build", "bin", "geth")
	out := detectBinaryVersion(gethPath, "version")
	if out == "" {
		return ""
	}
	return firstLine(out)
}

func detectRethVersion(rootDir string) string {
	cfg, err := loadTestEnvLite(rootDir)
	if err != nil {
		return ""
	}

	if explicit := strings.TrimSpace(cfg.Binaries.RethNative); explicit != "" {
		if !filepath.IsAbs(explicit) {
			explicit = filepath.Join(rootDir, explicit)
		}
		if version := detectBinaryVersion(explicit, "--version"); version != "" {
			return firstLine(version)
		}
	}

	rethRoot := strings.TrimSpace(cfg.Paths.RethRoot)
	if rethRoot == "" {
		rethRoot = "../rchain"
	}
	if !filepath.IsAbs(rethRoot) {
		rethRoot = filepath.Join(rootDir, rethRoot)
	}
	candidates := []string{
		filepath.Join(rethRoot, "target", "release", "congress-node"),
		filepath.Join(rethRoot, "target", "debug", "congress-node"),
	}
	for _, candidate := range candidates {
		if version := detectBinaryVersion(candidate, "--version"); version != "" {
			return firstLine(version)
		}
	}
	return ""
}

func detectBinaryVersion(bin string, args ...string) string {
	st, err := os.Stat(bin)
	if err != nil || st.Mode()&0o111 == 0 {
		return ""
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstLine(raw string) string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func detectGenesisHash(rootDir, configPath string) string {
	cfg, err := intcfg.LoadConfig(configPath)
	if err == nil && len(cfg.RPCs) > 0 {
		client, derr := ethclient.Dial(cfg.RPCs[0])
		if derr == nil {
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			block, berr := client.BlockByNumber(ctx, big.NewInt(0))
			if berr == nil && block != nil {
				return block.Hash().Hex()
			}
		}
	}
	genesisPath := filepath.Join(rootDir, "data", "genesis.json")
	return fileSHA256(genesisPath)
}

func fileSHA256(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func loadForkSchedule(configPath string) map[string]any {
	cfg, err := intcfg.LoadConfig(configPath)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{
		"mode":           cfg.Fork.Mode,
		"target":         cfg.Fork.Target,
		"scheduled_time": cfg.Fork.ScheduledTime,
		"delay_seconds":  cfg.Fork.DelaySeconds,
		"schedule": map[string]any{
			"shanghai_time":   cfg.Fork.Schedule.ShanghaiTime,
			"cancun_time":     cfg.Fork.Schedule.CancunTime,
			"fix_header_time": cfg.Fork.Schedule.FixHeaderTime,
			"posa_time":       cfg.Fork.Schedule.PosaTime,
		},
	}
	return out
}
