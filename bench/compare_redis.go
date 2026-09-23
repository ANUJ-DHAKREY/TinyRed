package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

type scenario struct {
	concurrency int
	pipeline    int
	label       string
}

type result struct {
	server string
	setRPS string
	getRPS string
}

const (
	tinyRedPort = "7999"
	redisPort   = "7998"
	requests    = "200000"
)

var scenarios = []scenario{
	{concurrency: 1, pipeline: 1, label: "single-client,no-pipeline"},
	{concurrency: 50, pipeline: 1, label: "50-clients,no-pipeline"},
	{concurrency: 200, pipeline: 1, label: "200-clients,no-pipeline"},
	{concurrency: 50, pipeline: 16, label: "50-clients,pipeline-16"},
}

func main() {
	runDir, err := os.MkdirTemp("", "tinyred-bench.")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(runDir)

	repoRoot, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	tinyRedBinary := filepath.Join(runDir, "tinyred")
	fmt.Printf("Building TinyRed from %s ...\n", repoRoot)
	if err := runCommand(repoRoot, "go", "build", "-o", tinyRedBinary, "."); err != nil {
		fatal(err)
	}

	redisPIDFile := filepath.Join(runDir, "redis.pid")
	redisLog := filepath.Join(runDir, "redis.log")
	fmt.Printf("Starting real redis-server on port %s ...\n", redisPort)
	if err := runCommand(repoRoot, "redis-server", "--port", redisPort, "--save", "", "--appendonly", "no", "--daemonize", "yes", "--pidfile", redisPIDFile, "--logfile", redisLog); err != nil {
		fatal(err)
	}
	defer func() { _ = runCommand(repoRoot, "redis-cli", "-p", redisPort, "shutdown", "nosave") }()
	if err := waitForRedis(repoRoot); err != nil {
		fatal(err)
	}

	tinyRedLog, err := os.OpenFile(filepath.Join(runDir, "tinyred.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fatal(err)
	}
	defer tinyRedLog.Close()

	fmt.Printf("Starting TinyRed on port %s ...\n", tinyRedPort)
	tinyRed := exec.Command(tinyRedBinary, "-port", tinyRedPort, "-dir", runDir)
	tinyRed.Stdout = tinyRedLog
	tinyRed.Stderr = tinyRedLog
	if err := tinyRed.Start(); err != nil {
		fatal(err)
	}
	defer func() {
		_ = tinyRed.Process.Kill()
		_ = tinyRed.Wait()
	}()
	time.Sleep(500 * time.Millisecond)

	fmt.Println()
	results := make([][]result, 0, len(scenarios))
	for _, current := range scenarios {
		fmt.Printf("=== Scenario: %s (-c %d -P %d) ===\n", current.label, current.concurrency, current.pipeline)
		fmt.Println("--- redis-server ---")
		redisOutput, err := benchmark(current, redisPort)
		if err != nil {
			fmt.Print(redisOutput)
			fmt.Printf("benchmark failed: %v\n", err)
		}
		fmt.Print(redisOutput)
		redisResult := result{server: "redis-server", setRPS: extractRPS(redisOutput, "SET"), getRPS: extractRPS(redisOutput, "GET")}

		fmt.Println("--- tinyred ---")
		if !processAlive(tinyRed.Process) {
			fmt.Println("TinyRed already crashed in a previous scenario, skipping.")
			results = append(results, []result{{server: "tinyred", setRPS: "CRASHED (earlier)", getRPS: "CRASHED (earlier)"}, redisResult})
			continue
		}
		tinyOutput, err := benchmark(current, tinyRedPort)
		if err != nil {
			fmt.Print(tinyOutput)
			fmt.Printf("benchmark failed: %v\n", err)
		}
		fmt.Print(tinyOutput)
		tinyResult := result{server: "tinyred", setRPS: extractRPS(tinyOutput, "SET"), getRPS: extractRPS(tinyOutput, "GET")}
		if !processAlive(tinyRed.Process) {
			tinyResult = result{server: "tinyred", setRPS: "CRASHED", getRPS: "CRASHED"}
			fmt.Printf("TinyRed crashed. See %s\n", filepath.Join(runDir, "tinyred.log"))
		}
		results = append(results, []result{tinyResult, redisResult})
	}

	fmt.Println("================================================================")
	fmt.Println("# TinyRed vs redis-server benchmark")
	fmt.Printf("\nRequests per scenario: %s (SET/GET) unless the server under test crashed first.\n\n", requests)
	fmt.Println("| Scenario | Server | SET rps | GET rps |")
	fmt.Println("|---|---|---|---|")
	for i, current := range scenarios {
		for _, item := range results[i] {
			fmt.Printf("| %s | %s | %s | %s |\n", current.label, item.server, item.setRPS, item.getRPS)
		}
	}
	fmt.Println("================================================================")
	fmt.Printf("Full logs kept in: %s\n", runDir)
}

func benchmark(current scenario, port string) (string, error) {
	command := exec.Command("redis-benchmark", "-p", port, "-t", "set,get", "-n", requests, "-c", strconv.Itoa(current.concurrency), "-P", strconv.Itoa(current.pipeline), "-q")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.String(), err
}

func extractRPS(output string, commandName string) string {
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(commandName) + `:.*?([0-9]+\.[0-9]+) requests per second`)
	match := pattern.FindStringSubmatch(output)
	if len(match) == 2 {
		return match[1]
	}
	return "n/a"
}

func waitForRedis(repoRoot string) error {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := runCommand(repoRoot, "redis-cli", "-p", redisPort, "ping"); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("redis-server did not become ready")
}

func processAlive(process *os.Process) bool {
	if process == nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func runCommand(dir string, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
