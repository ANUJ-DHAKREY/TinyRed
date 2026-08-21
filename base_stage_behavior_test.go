package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

const defaultMaxBaseStage = 7

func maxBaseStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_BASE_STAGE"))
	if raw == "" {
		return defaultMaxBaseStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxBaseStage
	}
	if v > 7 {
		return 7
	}
	return v
}

func requireStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxBaseStage() {
		t.Skipf("skipping stage %d test; set TINYRED_BASE_STAGE=%d (or higher) to run", stage, stage)
	}
}

type serverProc struct {
	port string
	cmd  *exec.Cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("TINYRED_HELPER_PROCESS") != "1" {
		t.Skip("helper process test")
	}

	args := os.Args
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || sep+1 >= len(args) {
		os.Exit(2)
	}

	serverArgs := append([]string{args[0]}, args[sep+1:]...)
	os.Args = serverArgs
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	main()
	os.Exit(0)
}

func startTinyRed(t *testing.T) *serverProc {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess", "--", "--port", strconv.Itoa(port))
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

func dialClient(t *testing.T, sp *serverProc) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", sp.port), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, bufio.NewReader(conn)
}

func writeRESPArray(t *testing.T, conn net.Conn, parts ...string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(parts))
	for _, p := range parts {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(p), p)
	}
	if _, err := io.WriteString(conn, b.String()); err != nil {
		t.Fatalf("failed to write command: %v", err)
	}
}

func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read line: %v", err)
	}
	return line
}

func readBulkString(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	header := readLine(t, r)
	if !strings.HasPrefix(header, "$") {
		t.Fatalf("expected bulk-string header, got %q", header)
	}
	if header == "$-1\r\n" {
		return header
	}

	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "$")))
	if err != nil {
		t.Fatalf("invalid bulk-string length header %q: %v", header, err)
	}
	body := make([]byte, n+2)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("failed reading bulk-string body: %v", err)
	}
	return header + string(body)
}

func TestAcceptsTCPConnectionOnConfiguredPort_Stage01BindToPort(t *testing.T) {
	requireStage(t, 1)
	// Scenario: server is started and should accept a TCP connection on its port.
	sp := startTinyRed(t)

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", sp.port), time.Second)
	if err != nil {
		t.Fatalf("expected server to accept TCP connections: %v", err)
	}
	_ = conn.Close()
}

func TestRespondsWithPongWhenPingIsReceived_Stage02RespondToPing(t *testing.T) {
	requireStage(t, 2)
	// Scenario: client sends PING and server replies with RESP simple string PONG.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	// Strict behavior check: server should only reply after a command is sent.
	_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, err := r.Peek(1)
	if err == nil {
		t.Fatalf("server sent data before receiving any command")
	}
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("expected read timeout before first command, got: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})

	writeRESPArray(t, conn, "PING")
	if got := readLine(t, r); got != "+PONG\r\n" {
		t.Fatalf("expected +PONG, got %q", got)
	}
}

func TestHandlesMultiplePingCommandsOnSameConnection_Stage03MultiplePings(t *testing.T) {
	requireStage(t, 3)
	// Scenario: two PING commands on one connection yield two independent PONG responses.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "PING")
	writeRESPArray(t, conn, "PING")

	if got := readLine(t, r); got != "+PONG\r\n" {
		t.Fatalf("first response: expected +PONG, got %q", got)
	}
	if got := readLine(t, r); got != "+PONG\r\n" {
		t.Fatalf("second response: expected +PONG, got %q", got)
	}
}

func TestServesConcurrentClientsWithoutDroppingResponses_Stage04ConcurrentClients(t *testing.T) {
	requireStage(t, 4)
	// Scenario: two clients concurrently send PING and each receives its own PONG.
	sp := startTinyRed(t)

	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", sp.port), time.Second)
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()

			r := bufio.NewReader(conn)
			writeRESPArray(t, conn, "PING")
			if got := readLine(t, r); got != "+PONG\r\n" {
				errCh <- fmt.Errorf("expected +PONG, got %q", got)
				return
			}
			errCh <- nil
		}()
	}

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent ping failed: %v", err)
		}
	}
}

func TestEchoReturnsArgumentAsBulkString_Stage05Echo(t *testing.T) {
	requireStage(t, 5)
	// Scenario: ECHO with one argument should return that exact payload as RESP bulk string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ECHO", "hello")
	if got := readBulkString(t, r); got != "$5\r\nhello\r\n" {
		t.Fatalf("expected echo bulk string, got %q", got)
	}
}

func TestSetStoresValueAndGetReturnsStoredOrNull_Stage06SetGet(t *testing.T) {
	requireStage(t, 6)
	// Scenario: SET stores key/value, GET returns stored value, and missing keys return null bulk string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$3\r\nbar\r\n" {
		t.Fatalf("GET existing expected bar, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "missing")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("GET missing expected null bulk string, got %q", got)
	}
}

func TestSetWithPxExpiresKeyAfterSpecifiedDuration_Stage07ExpiryPX(t *testing.T) {
	requireStage(t, 7)
	// Scenario: SET with PX allows immediate read, then key expires and GET returns null.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "ephemeral", "value", "PX", "100")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET PX expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "ephemeral")
	if got := readBulkString(t, r); got != "$5\r\nvalue\r\n" {
		t.Fatalf("immediate GET expected value, got %q", got)
	}

	time.Sleep(180 * time.Millisecond)
	writeRESPArray(t, conn, "GET", "ephemeral")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("expired GET expected null bulk string, got %q", got)
	}
}

func TestDoesNotSendDataBeforeAnyCommandIsReceived_Stage02NoUnsolicitedOutput(t *testing.T) {
	requireStage(t, 2)
	// Scenario: idle connection should not receive unsolicited bytes before any request.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	_ = conn.SetReadDeadline(time.Now().Add(120 * time.Millisecond))
	_, err := r.ReadByte()
	if err == nil {
		t.Fatalf("unexpected byte before first command")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("expected timeout before first command, got %v", err)
	}
}
