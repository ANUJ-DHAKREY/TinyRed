package tests

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// defaultMaxCustomGracefulShutdownStage is 0 because none of the PHASE C9:
// GRACEFUL SHUTDOWN + DOCKER + MAKEFILE custom stages are implemented yet in
// this repo (no signal handling, no Dockerfile, no Makefile) — keeping this
// at 0 by default means `go test ./...` stays green until each stage is
// opted into explicitly via TINYRED_CUSTOM_GRACEFUL_SHUTDOWN_STAGE.
const defaultMaxCustomGracefulShutdownStage = 0

func maxCustomGracefulShutdownStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_CUSTOM_GRACEFUL_SHUTDOWN_STAGE"))
	if raw == "" {
		return defaultMaxCustomGracefulShutdownStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxCustomGracefulShutdownStage
	}
	if v > 3 {
		return 3
	}
	return v
}

func requireCustomGracefulShutdownStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxCustomGracefulShutdownStage() {
		t.Skipf("skipping custom graceful-shutdown stage %d test; set TINYRED_CUSTOM_GRACEFUL_SHUTDOWN_STAGE=%d (or higher) to run", stage, stage)
	}
}

// customGracefulShutdownWaitResult carries the outcome of a single Wait()
// call on the server subprocess, performed exactly once from a background
// goroutine so that startTinyRed's own t.Cleanup (which does its own
// Process.Kill + Wait on an already-exited process, which is a harmless
// no-op) never races with — or duplicates — the explicit Wait() done here.
type customGracefulShutdownWaitResult struct {
	err   error
	state *os.ProcessState
}

// customGracefulShutdownWaitForExit waits, with a bound, for cmd to exit by
// performing the one-and-only explicit Wait() call on it. Callers must not
// call cmd.Wait() themselves afterwards (Go's os/exec docs specify calling
// Wait twice is an error) — startTinyRed's t.Cleanup calling Wait again on an
// already-exited process is fine and returns an ignored error there.
func customGracefulShutdownWaitForExit(cmd *exec.Cmd, timeout time.Duration) (*os.ProcessState, error, bool) {
	resultCh := make(chan customGracefulShutdownWaitResult, 1)
	go func() {
		err := cmd.Wait()
		resultCh <- customGracefulShutdownWaitResult{err: err, state: cmd.ProcessState}
	}()

	select {
	case res := <-resultCh:
		return res.state, res.err, true
	case <-time.After(timeout):
		return nil, nil, false
	}
}

func TestServerExitsCleanlyOnSigterm_Stage01GracefulShutdown(t *testing.T) {
	requireCustomGracefulShutdownStage(t, 1)
	// Scenario: server is started, a client connects and stays idle (never
	// closed by the test), then the server process receives SIGTERM. The
	// process must exit within a bounded time with status code 0.
	sp := startTinyRed(t)

	// Open an idle client connection; intentionally leave it open so the
	// server must proactively stop serving it as part of shutdown.
	_, _ = dialClient(t, sp)

	if err := sp.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM to server process: %v", err)
	}

	state, waitErr, exited := customGracefulShutdownWaitForExit(sp.cmd, 5*time.Second)
	if !exited {
		t.Fatalf("server did not exit within 5s after SIGTERM")
	}
	if state == nil {
		t.Fatalf("process state is nil after wait (wait err: %v)", waitErr)
	}
	if code := state.ExitCode(); code != 0 {
		t.Fatalf("expected exit code 0 after graceful shutdown, got %d (wait err: %v)", code, waitErr)
	}
}

func TestInFlightSetCommandCompletesBeforeConnectionClosesOnSigterm_Stage01GracefulShutdownInFlight(t *testing.T) {
	requireCustomGracefulShutdownStage(t, 1)
	// Scenario: a SET command is written to the wire and SIGTERM is sent
	// immediately afterward, racing shutdown against the in-flight command.
	// The server must let the in-flight command finish and send the full
	// +OK\r\n response before the connection is torn down — no command
	// should be aborted mid-flight by shutdown.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if err := sp.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM to server process: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("expected in-flight SET to complete with +OK before shutdown, got %q", got)
	}

	state, waitErr, exited := customGracefulShutdownWaitForExit(sp.cmd, 5*time.Second)
	if !exited {
		t.Fatalf("server did not exit within 5s after SIGTERM")
	}
	if state == nil {
		t.Fatalf("process state is nil after wait (wait err: %v)", waitErr)
	}
	if code := state.ExitCode(); code != 0 {
		t.Fatalf("expected exit code 0 after graceful shutdown, got %d (wait err: %v)", code, waitErr)
	}
}

func TestDockerfileExistsWithMultiStageBuildAndExposedPort_Stage02Dockerfile(t *testing.T) {
	requireCustomGracefulShutdownStage(t, 2)
	// Scenario: a lightweight presence/lint check (not a real `docker build`)
	// verifying the repo-root Dockerfile is multi-stage and exposes 6379.
	data, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatalf("expected a Dockerfile at the repo root: %v", err)
	}

	fromCount := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "FROM ") {
			fromCount++
		}
	}
	if fromCount < 2 {
		t.Fatalf("expected at least 2 FROM lines for a multi-stage build, got %d", fromCount)
	}

	exposeRe := regexp.MustCompile(`(?i)expose\s+6379`)
	if !exposeRe.MatchString(string(data)) {
		t.Fatalf("expected Dockerfile to contain an EXPOSE 6379 line")
	}
}

func TestMakefileExistsWithBuildRunTestTargets_Stage03Makefile(t *testing.T) {
	requireCustomGracefulShutdownStage(t, 3)
	// Scenario: a lightweight presence/lint check (not an actual `make`
	// invocation) verifying the repo-root Makefile defines build/run/test
	// targets.
	data, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("expected a Makefile at the repo root: %v", err)
	}
	content := string(data)

	for _, target := range []string{"build:", "run:", "test:"} {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target))
		if !re.MatchString(content) {
			t.Fatalf("expected Makefile to define a %q target", target)
		}
	}
}
