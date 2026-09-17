package tests

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func requireCustomServerCmdsStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseCustomServerCommands)
}

// --- Stage 1: DBSIZE ---

func TestDbsizeCountsKeysExcludingExpired_Stage01Dbsize(t *testing.T) {
	requireCustomServerCmdsStage(t)
	// Scenario: DBSIZE counts live keys and must not count an expired-but-not-yet-swept key.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "a", "1")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET a expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "SET", "b", "2")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET b expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "DBSIZE")
	if got := readRESPInteger(t, r); got != 2 {
		t.Fatalf("DBSIZE after two sets: expected 2, got %d", got)
	}

	writeRESPArray(t, conn, "SET", "c", "3", "PX", "50")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET c PX expected +OK, got %q", got)
	}

	time.Sleep(200 * time.Millisecond)

	writeRESPArray(t, conn, "DBSIZE")
	if got := readRESPInteger(t, r); got != 2 {
		t.Fatalf("DBSIZE after expiry: expected 2 (expired key excluded), got %d", got)
	}
}

// --- Stage 2: FLUSHDB ---

func TestFlushdbRemovesAllKeys_Stage02Flushdb(t *testing.T) {
	requireCustomServerCmdsStage(t)
	// Scenario: FLUSHDB clears all keys, resetting DBSIZE to 0 and making prior keys unreadable.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "a", "1")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET a expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "SET", "b", "2")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET b expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "FLUSHDB")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("FLUSHDB expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "DBSIZE")
	if got := readRESPInteger(t, r); got != 0 {
		t.Fatalf("DBSIZE after FLUSHDB: expected 0, got %d", got)
	}

	writeRESPArray(t, conn, "GET", "a")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("GET a after FLUSHDB: expected null bulk string, got %q", got)
	}
}

// --- Stage 3: INFO server ---

var (
	customServerCmdsConnectedClientsRE = regexp.MustCompile(`connected_clients:(\d+)`)
	customServerCmdsUptimeRE           = regexp.MustCompile(`uptime_in_seconds:(\d+)`)
)

func TestInfoServerReportsPortClientsAndUptime_Stage03InfoServer(t *testing.T) {
	requireCustomServerCmdsStage(t)
	// Scenario: INFO server reports the exact listening port, a connected-clients count
	// that reflects concurrently open connections, and a parseable uptime.
	sp := startTinyRed(t)

	// Two extra idle connections in addition to the one issuing the command.
	idle1, _ := dialClient(t, sp)
	idle2, _ := dialClient(t, sp)
	_ = idle1
	_ = idle2

	conn, r := dialClient(t, sp)
	writeRESPArray(t, conn, "INFO", "server")
	raw := readBulkString(t, r)

	payload := customServerCmdsBulkPayload(t, raw)

	wantPort := "tcp_port:" + sp.port
	if !strings.Contains(payload, wantPort) {
		t.Fatalf("INFO server: expected payload to contain %q, got %q", wantPort, payload)
	}

	ccMatch := customServerCmdsConnectedClientsRE.FindStringSubmatch(payload)
	if ccMatch == nil {
		t.Fatalf("INFO server: expected connected_clients:<n>, got %q", payload)
	}
	connectedClients, err := strconv.Atoi(ccMatch[1])
	if err != nil || connectedClients < 0 {
		t.Fatalf("INFO server: invalid connected_clients value %q: %v", ccMatch[1], err)
	}
	if connectedClients < 3 {
		t.Fatalf("INFO server: expected connected_clients >= 3 (2 idle + 1 caller), got %d", connectedClients)
	}

	upMatch := customServerCmdsUptimeRE.FindStringSubmatch(payload)
	if upMatch == nil {
		t.Fatalf("INFO server: expected uptime_in_seconds:<n>, got %q", payload)
	}
	uptime, err := strconv.Atoi(upMatch[1])
	if err != nil || uptime < 0 {
		t.Fatalf("INFO server: invalid uptime_in_seconds value %q: %v", upMatch[1], err)
	}
}

// customServerCmdsBulkPayload extracts the payload from a raw bulk-string response
// of the form "$N\r\n<payload>\r\n".
func customServerCmdsBulkPayload(t *testing.T, raw string) string {
	t.Helper()
	if !strings.HasPrefix(raw, "$") {
		t.Fatalf("expected bulk string, got %q", raw)
	}
	idx := strings.Index(raw, "\r\n")
	if idx == -1 {
		t.Fatalf("malformed bulk string %q", raw)
	}
	body := raw[idx+2:]
	body = strings.TrimSuffix(body, "\r\n")
	return body
}

// --- Stage 4: CONFIG GET for arbitrary keys ---

func TestConfigGetKnownPortReturnsActualPort_Stage04ConfigGetPort(t *testing.T) {
	requireCustomServerCmdsStage(t)
	// Scenario: CONFIG GET port returns a two-element array with the exact listening port.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "CONFIG", "GET", "port")
	elems := listReadRESPArray(t, r)
	if len(elems) != 2 {
		t.Fatalf("CONFIG GET port: expected 2-element array, got %v", elems)
	}
	if elems[0] != "port" {
		t.Fatalf("CONFIG GET port: expected key 'port', got %q", elems[0])
	}
	if elems[1] != sp.port {
		t.Fatalf("CONFIG GET port: expected value %q, got %q", sp.port, elems[1])
	}
}

func TestConfigGetUnknownKeyReturnsEmptyArrayNotError_Stage04ConfigGetUnknown(t *testing.T) {
	requireCustomServerCmdsStage(t)
	// Scenario: CONFIG GET for a key the server doesn't recognize returns an empty
	// RESP array, never a RESP error.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "CONFIG", "GET", "totally-unknown-key-xyz")

	header := readLine(t, r)
	if strings.HasPrefix(header, "-") {
		t.Fatalf("CONFIG GET unknown key: expected non-error response, got RESP error %q", header)
	}
	if header != "*0\r\n" {
		t.Fatalf("CONFIG GET unknown key: expected empty array header *0\\r\\n, got %q", header)
	}

	// Re-issue and confirm via the array-reading helper for a zero-length slice.
	writeRESPArray(t, conn, "CONFIG", "GET", "totally-unknown-key-xyz")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("CONFIG GET unknown key: expected empty array, got %v", elems)
	}
}
