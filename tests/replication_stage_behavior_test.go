package tests

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Replication is entirely unimplemented today (no --replicaof flag, no INFO
// command, no REPLCONF/PSYNC/WAIT), so every test below is gated behind
// TINYRED_REPLICATION_STAGE and skips by default until the corresponding
// server-side feature lands.
const defaultMaxReplicationStage = 0

func maxReplicationStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_REPLICATION_STAGE"))
	if raw == "" {
		return defaultMaxReplicationStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxReplicationStage
	}
	if v > 18 {
		return 18
	}
	return v
}

func requireReplicationStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxReplicationStage() {
		t.Skipf("skipping replication stage %d test; set TINYRED_REPLICATION_STAGE=%d (or higher) to run", stage, stage)
	}
}

// --- Local RESP / replication helpers ---
//
// These intentionally do not reuse identically-named helpers from sibling
// test files (listReadRESPArray, extractBulkValue, readRESPArray, ...) even
// though the logic overlaps, per the instruction to avoid redeclaring or
// colliding with helpers owned by other stage files.

// replExtractBulkPayload pulls the value out of a raw bulk-string response
// of the form "$N\r\nVALUE\r\n" as returned by readBulkString.
func replExtractBulkPayload(raw string) string {
	parts := strings.SplitN(raw, "\r\n", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// replReadPropagatedArray reads one RESP array of bulk strings off r and
// returns the decoded values (e.g. ["SET", "foo", "bar"]).
func replReadPropagatedArray(t *testing.T, r *bufio.Reader) []string {
	t.Helper()
	header := readLine(t, r)
	if !strings.HasPrefix(header, "*") {
		t.Fatalf("expected RESP array header, got %q", header)
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil {
		t.Fatalf("invalid RESP array length %q: %v", header, err)
	}
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		bs := readBulkString(t, r)
		p := strings.SplitN(bs, "\r\n", 3)
		if len(p) >= 2 {
			parts[i] = p[1]
		}
	}
	return parts
}

// respArrayBytes builds the exact RESP-array byte encoding for parts, the
// same way writeRESPArray does, so tests can compute byte-length offsets
// programmatically instead of hardcoding magic numbers.
func respArrayBytes(parts ...string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(parts))
	for _, p := range parts {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(p), p)
	}
	return []byte(b.String())
}

// writeRDBPayload writes the non-RESP "$<len>\r\n<bytes>" framing (no
// trailing CRLF after the binary payload) used to transfer an RDB file
// during a full resync.
func writeRDBPayload(t *testing.T, conn net.Conn, data []byte) {
	t.Helper()
	if _, err := io.WriteString(conn, fmt.Sprintf("$%d\r\n", len(data))); err != nil {
		t.Fatalf("failed to write RDB header: %v", err)
	}
	if len(data) > 0 {
		if _, err := conn.Write(data); err != nil {
			t.Fatalf("failed to write RDB payload: %v", err)
		}
	}
}

// readRDBPayload reads the "$<len>\r\n<bytes>" framing and returns the raw
// bytes (without consuming a trailing CRLF, since there isn't one).
func readRDBPayload(t *testing.T, r *bufio.Reader) []byte {
	t.Helper()
	header := readLine(t, r)
	if !strings.HasPrefix(header, "$") {
		t.Fatalf("expected RDB length header, got %q", header)
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "$")))
	if err != nil || n < 0 {
		t.Fatalf("invalid RDB length header %q: %v", header, err)
	}
	if n == 0 {
		return []byte{}
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("failed to read RDB payload of %d bytes: %v", n, err)
	}
	return buf
}

// startFakeMaster starts a listener that accepts exactly one connection and
// delivers it on the returned channel, letting a test play the role of a
// "master" server so the REPLICA side of the handshake can be exercised
// without a real master implementation.
func startFakeMaster(t *testing.T) (int, <-chan net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake master listener: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		connCh <- conn
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return port, connCh
}

// acceptFakeMasterConn waits (with a timeout) for the replica-under-test to
// connect to a fake master started via startFakeMaster.
func acceptFakeMasterConn(t *testing.T, connCh <-chan net.Conn) net.Conn {
	t.Helper()
	select {
	case conn := <-connCh:
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	case <-time.After(3 * time.Second):
		t.Fatalf("replica did not connect to fake master in time")
		return nil
	}
}

// replicaHandshakeAsClient drives the full replication handshake against sp
// (which is expected to act as a master), playing the role of a fake
// replica, and returns the connection/reader for further use (e.g. reading
// propagated commands or sending REPLCONF GETACK).
func replicaHandshakeAsClient(t *testing.T, sp *serverProc, listeningPort string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "PING")
	if got := readLine(t, r); got != "+PONG\r\n" {
		t.Fatalf("handshake PING: expected +PONG, got %q", got)
	}

	writeRESPArray(t, conn, "REPLCONF", "listening-port", listeningPort)
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("handshake REPLCONF listening-port: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "REPLCONF", "capa", "psync2")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("handshake REPLCONF capa: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "PSYNC", "?", "-1")
	line := readLine(t, r)
	if !strings.HasPrefix(line, "+FULLRESYNC ") {
		t.Fatalf("handshake PSYNC: expected +FULLRESYNC line, got %q", line)
	}
	_ = readRDBPayload(t, r)

	return conn, r
}

var replIDPattern = regexp.MustCompile(`master_replid:([A-Za-z0-9]{40})`)

// --- Stage 1 (doc 53): Configure listening port ---

func TestServerAcceptsConnectionOnConfiguredPort_Stage01ListeningPort(t *testing.T) {
	requireReplicationStage(t, 1)
	// Scenario: server started via startTinyRedWithArgs binds to a custom,
	// dynamically-chosen port and accepts a plain TCP connection on it.
	sp := startTinyRedWithArgs(t)

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", sp.port), time.Second)
	if err != nil {
		t.Fatalf("expected server to accept TCP connection on configured port: %v", err)
	}
	_ = conn.Close()
}

// --- Stage 2 (doc 54): The INFO command (master role) ---

func TestInfoReplicationReportsMasterRole_Stage02InfoMaster(t *testing.T) {
	requireReplicationStage(t, 2)
	// Scenario: a plain server (no --replicaof) reports role:master via
	// INFO replication.
	sp := startTinyRedWithArgs(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "INFO", "replication")
	raw := readBulkString(t, r)
	payload := replExtractBulkPayload(raw)
	if !strings.Contains(payload, "role:master") {
		t.Fatalf("expected INFO replication to contain role:master, got %q", payload)
	}
}

// --- Stage 3 (doc 55): The INFO command on a replica ---

func TestInfoReplicationReportsSlaveRoleWithReplicaOf_Stage03InfoReplica(t *testing.T) {
	requireReplicationStage(t, 3)
	// Scenario: a server started with --replicaof reports role:slave via
	// INFO replication, without needing a real master to be listening.
	sp := startTinyRedWithArgs(t, "--replicaof", "localhost 6379")
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "INFO", "replication")
	raw := readBulkString(t, r)
	payload := replExtractBulkPayload(raw)
	if !strings.Contains(payload, "role:slave") {
		t.Fatalf("expected INFO replication to contain role:slave, got %q", payload)
	}
}

func TestInfoReplicationStillReportsMasterRoleWithoutReplicaOf_Stage03InfoReplicaRegression(t *testing.T) {
	requireReplicationStage(t, 3)
	// Scenario: regression check that a plain server (no --replicaof) still
	// reports role:master once the replica-role handling is added.
	sp := startTinyRedWithArgs(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "INFO", "replication")
	raw := readBulkString(t, r)
	payload := replExtractBulkPayload(raw)
	if !strings.Contains(payload, "role:master") {
		t.Fatalf("expected INFO replication to contain role:master, got %q", payload)
	}
}

// --- Stage 4 (doc 56): Initial replication ID and offset ---

func TestInfoReplicationIncludesReplIdAndOffset_Stage04ReplIDOffset(t *testing.T) {
	requireReplicationStage(t, 4)
	// Scenario: a master's INFO replication output includes a 40-character
	// alphanumeric master_replid and a master_repl_offset of 0.
	sp := startTinyRedWithArgs(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "INFO", "replication")
	raw := readBulkString(t, r)
	payload := replExtractBulkPayload(raw)

	if !replIDPattern.MatchString(payload) {
		t.Fatalf("expected master_replid:<40 alphanumeric chars> in %q", payload)
	}
	if !strings.Contains(payload, "master_repl_offset:0") {
		t.Fatalf("expected master_repl_offset:0 in %q", payload)
	}
}

// --- Stage 5 (doc 57): Send handshake (1/3) ---

func TestReplicaSendsPingFirstDuringHandshake_Stage05SendPing(t *testing.T) {
	requireReplicationStage(t, 5)
	// Scenario: a server started with --replicaof connects to the master
	// and the very first bytes it sends encode a PING RESP array.
	fakeMasterPort, connCh := startFakeMaster(t)
	startTinyRedWithArgs(t, "--replicaof", fmt.Sprintf("127.0.0.1 %d", fakeMasterPort))

	conn := acceptFakeMasterConn(t, connCh)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	expected := respArrayBytes("PING")
	got := make([]byte, len(expected))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("failed to read PING from replica: %v", err)
	}
	if string(got) != string(expected) {
		t.Fatalf("expected first bytes to be %q, got %q", expected, got)
	}
}

// --- Stage 6 (doc 58): Send handshake (2/3) ---

func TestReplicaSendsReplconfAfterPing_Stage06SendReplconf(t *testing.T) {
	requireReplicationStage(t, 6)
	// Scenario: after PING/PONG, the replica sends REPLCONF listening-port
	// <its own port> and REPLCONF capa psync2, waiting for +OK after each.
	fakeMasterPort, connCh := startFakeMaster(t)
	sp := startTinyRedWithArgs(t, "--replicaof", fmt.Sprintf("127.0.0.1 %d", fakeMasterPort))

	conn := acceptFakeMasterConn(t, connCh)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)

	if got := replReadPropagatedArray(t, r); len(got) != 1 || got[0] != "PING" {
		t.Fatalf("expected [PING], got %v", got)
	}
	if _, err := io.WriteString(conn, "+PONG\r\n"); err != nil {
		t.Fatalf("failed to respond PONG: %v", err)
	}

	got := replReadPropagatedArray(t, r)
	if len(got) != 3 || !strings.EqualFold(got[0], "REPLCONF") || !strings.EqualFold(got[1], "listening-port") || got[2] != sp.port {
		t.Fatalf("expected [REPLCONF listening-port %s], got %v", sp.port, got)
	}
	if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
		t.Fatalf("failed to respond OK: %v", err)
	}

	got = replReadPropagatedArray(t, r)
	if len(got) != 3 || !strings.EqualFold(got[0], "REPLCONF") || !strings.EqualFold(got[1], "capa") || !strings.EqualFold(got[2], "psync2") {
		t.Fatalf("expected [REPLCONF capa psync2], got %v", got)
	}
	if _, err := io.WriteString(conn, "+OK\r\n"); err != nil {
		t.Fatalf("failed to respond OK: %v", err)
	}
}

// --- Stage 7 (doc 59): Send handshake (3/3) ---

func TestReplicaSendsPsyncAfterReplconf_Stage07SendPsync(t *testing.T) {
	requireReplicationStage(t, 7)
	// Scenario: after both REPLCONF/OK exchanges, the replica sends
	// PSYNC ? -1.
	fakeMasterPort, connCh := startFakeMaster(t)
	sp := startTinyRedWithArgs(t, "--replicaof", fmt.Sprintf("127.0.0.1 %d", fakeMasterPort))

	conn := acceptFakeMasterConn(t, connCh)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)

	_ = replReadPropagatedArray(t, r) // PING
	_, _ = io.WriteString(conn, "+PONG\r\n")

	got := replReadPropagatedArray(t, r) // REPLCONF listening-port <port>
	if len(got) != 3 || got[2] != sp.port {
		t.Fatalf("expected REPLCONF listening-port %s, got %v", sp.port, got)
	}
	_, _ = io.WriteString(conn, "+OK\r\n")

	_ = replReadPropagatedArray(t, r) // REPLCONF capa psync2
	_, _ = io.WriteString(conn, "+OK\r\n")

	got = replReadPropagatedArray(t, r)
	if len(got) != 3 || !strings.EqualFold(got[0], "PSYNC") || got[1] != "?" || got[2] != "-1" {
		t.Fatalf("expected [PSYNC ? -1], got %v", got)
	}
	// Replica is documented to ignore this response at this stage.
	_, _ = io.WriteString(conn, "+FULLRESYNC 0000000000000000000000000000000000000000 0\r\n")
}

// --- Stage 8 (doc 60): Receive handshake (1/2), master side ---

func TestMasterRespondsOkToReplconfCommands_Stage08ReceiveReplconf(t *testing.T) {
	requireReplicationStage(t, 8)
	// Scenario: a plain server (acting as master) responds +OK to both
	// REPLCONF commands sent by a connecting replica.
	sp := startTinyRedWithArgs(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "REPLCONF", "listening-port", "6380")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("REPLCONF listening-port: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "REPLCONF", "capa", "psync2")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("REPLCONF capa psync2: expected +OK, got %q", got)
	}
}

// --- Stage 9 (doc 61): Receive handshake (2/2) ---

func TestMasterRespondsFullresyncToPsync_Stage09ReceivePsync(t *testing.T) {
	requireReplicationStage(t, 9)
	// Scenario: after the two REPLCONF exchanges, PSYNC ? -1 gets a
	// +FULLRESYNC <40-char replid> 0 response.
	sp := startTinyRedWithArgs(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "REPLCONF", "listening-port", "6380")
	_ = readLine(t, r)
	writeRESPArray(t, conn, "REPLCONF", "capa", "psync2")
	_ = readLine(t, r)

	writeRESPArray(t, conn, "PSYNC", "?", "-1")
	line := readLine(t, r)

	fullresyncPattern := regexp.MustCompile(`^\+FULLRESYNC [A-Za-z0-9]{40} 0\r\n$`)
	if !fullresyncPattern.MatchString(line) {
		t.Fatalf("expected +FULLRESYNC <replid> 0, got %q", line)
	}
}

// --- Stage 10 (doc 62): Empty RDB transfer ---

func TestMasterSendsEmptyRDBAfterFullresync_Stage10EmptyRDBTransfer(t *testing.T) {
	requireReplicationStage(t, 10)
	// Scenario: after the FULLRESYNC line, the master sends an RDB file
	// using "$<len>\r\n<bytes>" framing (no trailing CRLF).
	sp := startTinyRedWithArgs(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "REPLCONF", "listening-port", "6380")
	_ = readLine(t, r)
	writeRESPArray(t, conn, "REPLCONF", "capa", "psync2")
	_ = readLine(t, r)
	writeRESPArray(t, conn, "PSYNC", "?", "-1")
	_ = readLine(t, r) // FULLRESYNC line

	// Safety net: don't hang forever against a future buggy implementation.
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	payload := readRDBPayload(t, r)
	if payload == nil {
		t.Fatalf("expected to read an RDB payload (possibly empty), got nil")
	}
}

// --- Stage 11 (doc 63): Single-replica propagation ---

func TestMasterPropagatesWriteCommandToSingleReplica_Stage11SingleReplicaPropagation(t *testing.T) {
	requireReplicationStage(t, 11)
	// Scenario: once a replica finishes the handshake, a SET issued by a
	// separate plain client is propagated to the replica connection.
	sp := startTinyRedWithArgs(t)
	replicaPort := strconv.Itoa(reserveFreePort(t))
	replicaConn, replicaReader := replicaHandshakeAsClient(t, sp, replicaPort)

	clientConn, clientReader := dialClient(t, sp)
	writeRESPArray(t, clientConn, "SET", "foo", "bar")
	if got := readLine(t, clientReader); got != "+OK\r\n" {
		t.Fatalf("SET foo bar: expected +OK, got %q", got)
	}

	_ = replicaConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := replReadPropagatedArray(t, replicaReader)
	expected := []string{"SET", "foo", "bar"}
	if len(got) != len(expected) {
		t.Fatalf("expected propagated %v, got %v", expected, got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("expected propagated %v, got %v", expected, got)
		}
	}
}

// --- Stage 12 (doc 64): Multi-replica propagation ---

func TestMasterPropagatesWriteCommandToMultipleReplicas_Stage12MultiReplicaPropagation(t *testing.T) {
	requireReplicationStage(t, 12)
	// Scenario: two independently-handshaken replicas both receive the same
	// propagated command in order.
	sp := startTinyRedWithArgs(t)

	replica1Conn, replica1Reader := replicaHandshakeAsClient(t, sp, strconv.Itoa(reserveFreePort(t)))
	replica2Conn, replica2Reader := replicaHandshakeAsClient(t, sp, strconv.Itoa(reserveFreePort(t)))

	clientConn, clientReader := dialClient(t, sp)
	writeRESPArray(t, clientConn, "SET", "foo", "bar")
	if got := readLine(t, clientReader); got != "+OK\r\n" {
		t.Fatalf("SET foo bar: expected +OK, got %q", got)
	}

	expected := []string{"SET", "foo", "bar"}
	for i, rc := range []struct {
		conn net.Conn
		r    *bufio.Reader
	}{
		{replica1Conn, replica1Reader},
		{replica2Conn, replica2Reader},
	} {
		_ = rc.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		got := replReadPropagatedArray(t, rc.r)
		if len(got) != len(expected) {
			t.Fatalf("replica %d: expected propagated %v, got %v", i+1, expected, got)
		}
		for j := range expected {
			if got[j] != expected[j] {
				t.Fatalf("replica %d: expected propagated %v, got %v", i+1, expected, got)
			}
		}
	}
}

// --- Stage 13 (doc 65): Command processing (replica side) ---

func TestReplicaAppliesPropagatedCommands_Stage13CommandProcessing(t *testing.T) {
	requireReplicationStage(t, 13)
	// Scenario: a replica applies commands propagated over the replication
	// connection (without replying to them), and later serves them via GET.
	fakeMasterPort, connCh := startFakeMaster(t)
	sp := startTinyRedWithArgs(t, "--replicaof", fmt.Sprintf("127.0.0.1 %d", fakeMasterPort))

	masterConn := acceptFakeMasterConn(t, connCh)
	_ = masterConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	masterReader := bufio.NewReader(masterConn)

	// Drive the handshake from the master's perspective.
	_ = replReadPropagatedArray(t, masterReader) // PING
	_, _ = io.WriteString(masterConn, "+PONG\r\n")
	_ = replReadPropagatedArray(t, masterReader) // REPLCONF listening-port <port>
	_, _ = io.WriteString(masterConn, "+OK\r\n")
	_ = replReadPropagatedArray(t, masterReader) // REPLCONF capa psync2
	_, _ = io.WriteString(masterConn, "+OK\r\n")
	_ = replReadPropagatedArray(t, masterReader) // PSYNC ? -1
	_, _ = io.WriteString(masterConn, "+FULLRESYNC 0000000000000000000000000000000000000000 0\r\n")
	writeRDBPayload(t, masterConn, []byte{})

	// Propagate a write directly on the replication connection.
	writeRESPArray(t, masterConn, "SET", "foo", "1")

	clientConn, clientReader := dialClient(t, sp)
	// Give the replica a moment to apply the propagated command.
	deadline := time.Now().Add(2 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		writeRESPArray(t, clientConn, "GET", "foo")
		got = replExtractBulkPayload(readBulkString(t, clientReader))
		if got == "1" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got != "1" {
		t.Fatalf("GET foo after propagation: expected \"1\", got %q", got)
	}
}

// --- Stage 14 (doc 66): ACKs with no commands ---

func TestReplicaRespondsToGetAckWithZeroOffset_Stage14AckNoCommands(t *testing.T) {
	requireReplicationStage(t, 14)
	// Scenario: right after the handshake, REPLCONF GETACK * gets
	// REPLCONF ACK 0 back on the same replication connection.
	fakeMasterPort, connCh := startFakeMaster(t)
	startTinyRedWithArgs(t, "--replicaof", fmt.Sprintf("127.0.0.1 %d", fakeMasterPort))

	masterConn := acceptFakeMasterConn(t, connCh)
	_ = masterConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	masterReader := bufio.NewReader(masterConn)

	_ = replReadPropagatedArray(t, masterReader) // PING
	_, _ = io.WriteString(masterConn, "+PONG\r\n")
	_ = replReadPropagatedArray(t, masterReader) // REPLCONF listening-port <port>
	_, _ = io.WriteString(masterConn, "+OK\r\n")
	_ = replReadPropagatedArray(t, masterReader) // REPLCONF capa psync2
	_, _ = io.WriteString(masterConn, "+OK\r\n")
	_ = replReadPropagatedArray(t, masterReader) // PSYNC ? -1
	_, _ = io.WriteString(masterConn, "+FULLRESYNC 0000000000000000000000000000000000000000 0\r\n")
	writeRDBPayload(t, masterConn, []byte{})

	writeRESPArray(t, masterConn, "REPLCONF", "GETACK", "*")

	got := replReadPropagatedArray(t, masterReader)
	if len(got) != 3 || !strings.EqualFold(got[0], "REPLCONF") || !strings.EqualFold(got[1], "ACK") || got[2] != "0" {
		t.Fatalf("expected [REPLCONF ACK 0], got %v", got)
	}
}

// --- Stage 15 (doc 67): ACKs with commands (offset tracking) ---

func TestReplicaTracksOffsetAcrossGetAcks_Stage15AckWithCommands(t *testing.T) {
	requireReplicationStage(t, 15)
	// Scenario: the replica's REPLCONF ACK offset only counts bytes of
	// commands processed strictly before the current GETACK request.
	fakeMasterPort, connCh := startFakeMaster(t)
	startTinyRedWithArgs(t, "--replicaof", fmt.Sprintf("127.0.0.1 %d", fakeMasterPort))

	masterConn := acceptFakeMasterConn(t, connCh)
	_ = masterConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	masterReader := bufio.NewReader(masterConn)

	_ = replReadPropagatedArray(t, masterReader) // PING
	_, _ = io.WriteString(masterConn, "+PONG\r\n")
	_ = replReadPropagatedArray(t, masterReader) // REPLCONF listening-port <port>
	_, _ = io.WriteString(masterConn, "+OK\r\n")
	_ = replReadPropagatedArray(t, masterReader) // REPLCONF capa psync2
	_, _ = io.WriteString(masterConn, "+OK\r\n")
	_ = replReadPropagatedArray(t, masterReader) // PSYNC ? -1
	_, _ = io.WriteString(masterConn, "+FULLRESYNC 0000000000000000000000000000000000000000 0\r\n")
	writeRDBPayload(t, masterConn, []byte{})

	expectAck := func(want int) {
		t.Helper()
		writeRESPArray(t, masterConn, "REPLCONF", "GETACK", "*")
		got := replReadPropagatedArray(t, masterReader)
		if len(got) != 3 || !strings.EqualFold(got[0], "REPLCONF") || !strings.EqualFold(got[1], "ACK") {
			t.Fatalf("expected REPLCONF ACK <offset>, got %v", got)
		}
		gotOffset, err := strconv.Atoi(got[2])
		if err != nil {
			t.Fatalf("invalid ACK offset %q: %v", got[2], err)
		}
		if gotOffset != want {
			t.Fatalf("expected ACK offset %d, got %d", want, gotOffset)
		}
	}

	getack := respArrayBytes("REPLCONF", "GETACK", "*")
	ping := respArrayBytes("PING")
	setFoo := respArrayBytes("SET", "foo", "1")
	setBar := respArrayBytes("SET", "bar", "2")

	running := 0

	// First GETACK: nothing processed before it yet.
	expectAck(running)
	running += len(getack)

	// PING is processed silently.
	if _, err := masterConn.Write(ping); err != nil {
		t.Fatalf("failed to send PING: %v", err)
	}
	running += len(ping)

	// Second GETACK reflects the first GETACK + the PING.
	expectAck(running)
	running += len(getack)

	// Two SETs processed silently.
	if _, err := masterConn.Write(setFoo); err != nil {
		t.Fatalf("failed to send SET foo: %v", err)
	}
	running += len(setFoo)
	if _, err := masterConn.Write(setBar); err != nil {
		t.Fatalf("failed to send SET bar: %v", err)
	}
	running += len(setBar)

	// Third GETACK reflects everything processed so far.
	expectAck(running)
}

// --- Stage 16 (doc 68): WAIT with no replicas ---

func TestWaitReturnsZeroImmediatelyWithNoReplicas_Stage16WaitNoReplicas(t *testing.T) {
	requireReplicationStage(t, 16)
	// Scenario: WAIT 0 <timeout> with zero connected replicas returns 0
	// immediately, well before the timeout elapses.
	sp := startTinyRedWithArgs(t)
	conn, r := dialClient(t, sp)

	start := time.Now()
	writeRESPArray(t, conn, "WAIT", "0", "60000")
	got := readRESPInteger(t, r)
	elapsed := time.Since(start)

	if got != 0 {
		t.Fatalf("WAIT 0 60000: expected 0, got %d", got)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("WAIT 0 60000 took too long: %v (expected near-immediate)", elapsed)
	}
}

// --- Stage 17 (doc 69): WAIT with no commands ---

func TestWaitReturnsConnectedReplicaCountWithNoCommands_Stage17WaitNoCommands(t *testing.T) {
	requireReplicationStage(t, 17)
	// Scenario: with N replicas fully handshaken but no writes issued yet,
	// WAIT returns the number of connected replicas regardless of the
	// requested count.
	sp := startTinyRedWithArgs(t)

	const numReplicas = 3
	for i := 0; i < numReplicas; i++ {
		replicaHandshakeAsClient(t, sp, strconv.Itoa(reserveFreePort(t)))
	}

	conn, r := dialClient(t, sp)
	writeRESPArray(t, conn, "WAIT", "5", "500")
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := readRESPInteger(t, r)
	if got != numReplicas {
		t.Fatalf("WAIT 5 500: expected %d (connected replicas), got %d", numReplicas, got)
	}
}

// --- Stage 18 (doc 70): WAIT with multiple commands ---

func TestWaitReturnsAckCountAfterWriteCommands_Stage18WaitMultipleCommands(t *testing.T) {
	requireReplicationStage(t, 18)
	// Scenario: after write commands, WAIT triggers REPLCONF GETACK under
	// the hood and returns a sane ack count bounded by the number of
	// connected replicas (the exact value vs. the requested count is
	// intentionally unconstrained per the spec).
	sp := startTinyRedWithArgs(t)

	const numReplicas = 2
	for i := 0; i < numReplicas; i++ {
		replicaHandshakeAsClient(t, sp, strconv.Itoa(reserveFreePort(t)))
	}

	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "123")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo 123: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "WAIT", "1", "1000")
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	firstWait := readRESPInteger(t, r)
	if firstWait < 1 {
		t.Fatalf("WAIT 1 1000: expected at least 1 acking replica, got %d", firstWait)
	}
	if firstWait > numReplicas {
		t.Fatalf("WAIT 1 1000: got %d, more than the %d connected replicas", firstWait, numReplicas)
	}

	writeRESPArray(t, conn, "SET", "bar", "456")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET bar 456: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "WAIT", "2", "1000")
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	secondWait := readRESPInteger(t, r)
	if secondWait < 0 || secondWait > numReplicas {
		t.Fatalf("WAIT 2 1000: expected a non-negative count <= %d connected replicas, got %d", numReplicas, secondWait)
	}
}
