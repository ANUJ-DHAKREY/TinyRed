package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startTinyRedWithArgs starts the server on a freshly reserved port plus any
// extra CLI flags (e.g. "--replicaof", "host port", "--appendonly", "yes").
// It mirrors startTinyRed/startTinyRedWithRDB but is generic over flags so
// AOF, transaction, pub/sub, and replication tests can all reuse it.
func startTinyRedWithArgs(t *testing.T, extraArgs ...string) *serverProc {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	args := append([]string{"--port", strconv.Itoa(port)}, extraArgs...)
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=TestHelperProcess", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "TINYRED_HELPER_PROCESS=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start tinyred: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(3 * time.Second)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return &serverProc{port: strconv.Itoa(port), cmd: cmd}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("server did not become ready in time; output:\n%s", out.String())
	return nil
}

// reserveFreePort reserves and immediately releases a free TCP port, useful
// for handing a predetermined port to a server-under-test (e.g. --replicaof
// pointing at a fake master, or the port a fake replica listens on).
func reserveFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// readN reads exactly n bytes from r, failing the test on error.
func readN(t *testing.T, r *bufio.Reader, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := readFull(r, buf); err != nil {
		t.Fatalf("failed to read %d bytes: %v", n, err)
	}
	return buf
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// splitArgs is a tiny helper so callers can build extraArgs slices inline
// without importing strings in every test file.
func splitArgs(s string) []string {
	return strings.Fields(s)
}
