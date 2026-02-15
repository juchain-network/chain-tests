package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
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
}

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
	runCiLog := flag.Bool("ci-log", false, "After grouped runs, execute make ci-log")
	flag.Parse()

	rootDir := callerDir()
	if rootDir == "" {
		fmt.Println("Failed to resolve working directory")
		os.Exit(1)
	}

	unlock, err := acquireRunLock(rootDir)
	if err != nil {
		fmt.Printf("Failed to acquire run lock: %v\n", err)
		os.Exit(1)
	}
	defer unlock()

	if *configPath == "" {
		*configPath = filepath.Join(rootDir, "data", "test_config.yaml")
	} else if !filepath.IsAbs(*configPath) {
		*configPath = filepath.Join(rootDir, *configPath)
	}

	runID := time.Now().Format("20060102_150405")
	runDir := filepath.Join(rootDir, *reportDir, "ci_"+runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		fmt.Printf("Failed to create report dir: %v\n", err)
		os.Exit(1)
	}

	env := append([]string{}, os.Environ()...)
	env = append(env, "GOCACHE="+*gocache)
	if *debug {
		env = append(env, "JUCHAIN_TEST_DEBUG=1")
	}

	var results []stepResult
	var hadFailure bool

	switch strings.ToLower(*mode) {
	case "groups":
		for _, group := range splitList(*groups) {
			if group == "" {
				continue
			}
			name := "group_" + group
			cmd := "make"
			args := []string{"test-" + group}
			res := runStep(runDir, name, cmd, args, env, rootDir)
			results = append(results, res)
			if res.Status != "PASS" {
				hadFailure = true
				break
			}
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
	if err := writeReport(reportPath, *mode, *groups, *tests, *runPattern, *configPath, *gocache, *debug, results); err != nil {
		fmt.Printf("Failed to write report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Report: %s\n", reportPath)
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

func discoverTests(rootDir string) ([]string, error) {
	testDir := filepath.Join(rootDir, "tests")
	var files []string
	err := filepath.WalkDir(testDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
		{"make", "stop"},
		{"make", "clean"},
		{"make", "init"},
		{"make", "run"},
		{"make", "ready"},
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

	pass, fail, skip := parseTestOutput(logPath)

	res := stepResult{
		Name:      name,
		Command:   "go test -run ^" + testName + "$",
		LogPath:   logPath,
		Status:    status,
		Duration:  time.Since(start),
		PassTests: pass,
		FailTests: fail,
		SkipTests: skip,
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
			{"make", "stop"},
			{"make", "clean"},
			{"make", "init"},
			{"make", "run"},
			{"make", "ready"},
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

	pass, fail, skip := parseTestOutput(logPath)

	res := stepResult{
		Name:      name,
		Command:   "go test -run " + pattern,
		LogPath:   logPath,
		Status:    status,
		Duration:  time.Since(start),
		PassTests: pass,
		FailTests: fail,
		SkipTests: skip,
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

	pass, fail, skip := parseTestOutput(logPath)

	res := stepResult{
		Name:      name,
		Command:   strings.Join(append([]string{cmd}, args...), " "),
		LogPath:   logPath,
		Status:    status,
		Duration:  time.Since(start),
		PassTests: pass,
		FailTests: fail,
		SkipTests: skip,
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
	command.Env = env
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

func parseTestOutput(path string) ([]string, []string, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, nil
	}
	defer file.Close()

	var pass []string
	var fail []string
	var skip []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "--- PASS: ") {
			pass = append(pass, strings.TrimSpace(strings.TrimPrefix(line, "--- PASS: ")))
		} else if strings.HasPrefix(line, "--- FAIL: ") {
			fail = append(fail, strings.TrimSpace(strings.TrimPrefix(line, "--- FAIL: ")))
		} else if strings.HasPrefix(line, "--- SKIP: ") {
			skip = append(skip, strings.TrimSpace(strings.TrimPrefix(line, "--- SKIP: ")))
		}
	}
	return pass, fail, skip
}

func writeReport(path, mode, groups, tests, runPattern, configPath, gocache string, debug bool, results []stepResult) error {
	var sb strings.Builder
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

	return os.WriteFile(path, []byte(sb.String()), 0o644)
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
