package tests

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// defaultMaxOptimisticLockingStage is 0 because none of the WATCH/UNWATCH/
// optimistic-locking stages are implemented on the server yet. Every test in
// this file skips by default until a developer bumps TINYRED_OPTIMISTIC_LOCKING_STAGE
// as each stage gets implemented.
const defaultMaxOptimisticLockingStage = 0

func maxOptimisticLockingStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_OPTIMISTIC_LOCKING_STAGE"))
	if raw == "" {
		return defaultMaxOptimisticLockingStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxOptimisticLockingStage
	}
	if v > 8 {
		return 8
	}
	return v
}

func requireOptimisticLockingStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxOptimisticLockingStage() {
		t.Skipf("skipping optimistic locking stage %d test; set TINYRED_OPTIMISTIC_LOCKING_STAGE=%d (or higher) to run", stage, stage)
	}
}

// readOptimisticExecResult reads the response to an EXEC command, which is
// either a RESP null array (*-1\r\n) or an array whose elements can be simple
// strings, errors, integers, or bulk strings (mirroring the mixed results a
// queued command sequence can produce). It returns the decoded elements (as
// their raw textual value, prefix/type marker stripped) plus whether the
// array was null.
func readOptimisticExecResult(t *testing.T, r *bufio.Reader) (elements []string, isNull bool) {
	t.Helper()
	header := readLine(t, r)
	if !strings.HasPrefix(header, "*") {
		t.Fatalf("expected RESP array header for EXEC response, got %q", header)
	}
	if header == "*-1\r\n" {
		return nil, true
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil {
		t.Fatalf("invalid RESP array length %q: %v", header, err)
	}

	elements = make([]string, count)
	for i := 0; i < count; i++ {
		typeByte, err := r.Peek(1)
		if err != nil {
			t.Fatalf("failed to peek EXEC element %d type: %v", i, err)
		}
		switch typeByte[0] {
		case '+', '-', ':':
			line := readLine(t, r)
			elements[i] = strings.TrimSuffix(strings.TrimSuffix(line[1:], "\n"), "\r")
		case '$':
			raw := readBulkString(t, r)
			if raw == "$-1\r\n" {
				elements[i] = ""
				continue
			}
			parts := strings.SplitN(raw, "\r\n", 3)
			if len(parts) >= 2 {
				elements[i] = parts[1]
			}
		default:
			t.Fatalf("unexpected EXEC element %d type byte %q", i, string(typeByte[0]))
		}
	}
	return elements, false
}

// --- Stage 1: The WATCH command ---

// Scenario: WATCH on a single key, outside of any transaction, simply
// acknowledges with +OK\r\n and must not error or crash the server.
func TestWatchReturnsOK_Stage01WatchCommand(t *testing.T) {
	requireOptimisticLockingStage(t, 1)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "WATCH", "key")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("WATCH key: expected +OK, got %q", got)
	}
}

// --- Stage 2: WATCH inside transaction ---

// Scenario: once a connection has entered MULTI, calling WATCH must be
// rejected with a RESP error mentioning WATCH, MULTI, and "not allowed".
func TestWatchInsideMultiReturnsError_Stage02WatchInsideMulti(t *testing.T) {
	requireOptimisticLockingStage(t, 2)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "MULTI")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "WATCH", "somekey")
	got := readLine(t, r)
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("WATCH inside MULTI: expected RESP error, got %q", got)
	}
	upper := strings.ToUpper(got)
	if !strings.Contains(upper, "WATCH") {
		t.Fatalf("WATCH inside MULTI error missing 'WATCH': got %q", got)
	}
	if !strings.Contains(upper, "MULTI") {
		t.Fatalf("WATCH inside MULTI error missing 'MULTI': got %q", got)
	}
	if !strings.Contains(upper, "NOT ALLOWED") {
		t.Fatalf("WATCH inside MULTI error missing 'not allowed': got %q", got)
	}
}

// --- Stage 3: Tracking key modifications ---

// Scenario: Client1 watches foo, queues a write to bar inside MULTI. Client2
// modifies the watched key foo from a separate connection before EXEC runs.
// EXEC must abort (null array) and the queued write to bar must never apply.
func TestStage03aExecAbortsOnWatchedKeyModified(t *testing.T) {
	requireOptimisticLockingStage(t, 3)
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)
	conn2, r2 := dialClient(t, sp)

	writeRESPArray(t, conn1, "SET", "foo", "100")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET foo 100: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "200")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET bar 200: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "WATCH", "foo")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("WATCH foo: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "300")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("SET bar 300 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn2, "SET", "foo", "200")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("client2 SET foo 200: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems, isNull := readOptimisticExecResult(t, r1)
	if !isNull {
		t.Fatalf("EXEC after watched key modified: expected null array, got %v", elems)
	}

	writeRESPArray(t, conn1, "GET", "bar")
	got := readRESPBulkStringValue(t, r1)
	if got != "200" {
		t.Fatalf("GET bar after aborted transaction: expected unchanged '200', got %q", got)
	}
}

// Scenario: Client3 watches baz, queues a write to caz inside MULTI. Client4
// modifies a different, unwatched key (caz) before EXEC runs. EXEC must
// succeed (non-null array) and the queued write must take effect.
func TestStage03bExecSucceedsWhenUnwatchedKeyModified(t *testing.T) {
	requireOptimisticLockingStage(t, 3)
	sp := startTinyRed(t)
	conn3, r3 := dialClient(t, sp)
	conn4, r4 := dialClient(t, sp)

	writeRESPArray(t, conn3, "SET", "baz", "100")
	if got := readLine(t, r3); got != "+OK\r\n" {
		t.Fatalf("SET baz 100: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn3, "SET", "caz", "200")
	if got := readLine(t, r3); got != "+OK\r\n" {
		t.Fatalf("SET caz 200: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn3, "WATCH", "baz")
	if got := readLine(t, r3); got != "+OK\r\n" {
		t.Fatalf("WATCH baz: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn3, "MULTI")
	if got := readLine(t, r3); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn3, "SET", "caz", "400")
	if got := readLine(t, r3); got != "+QUEUED\r\n" {
		t.Fatalf("SET caz 400 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn4, "SET", "caz", "300")
	if got := readLine(t, r4); got != "+OK\r\n" {
		t.Fatalf("client4 SET caz 300: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn3, "EXEC")
	elems, isNull := readOptimisticExecResult(t, r3)
	if isNull {
		t.Fatalf("EXEC with unwatched key modified: expected non-null array, got null")
	}
	if len(elems) != 1 {
		t.Fatalf("EXEC result: expected 1 element, got %v", elems)
	}

	writeRESPArray(t, conn4, "GET", "caz")
	got := readRESPBulkStringValue(t, r4)
	if got != "400" {
		t.Fatalf("GET caz after successful transaction: expected '400', got %q", got)
	}
}

// --- Stage 4: Watching multiple keys ---

// Scenario: WATCH accepts multiple keys in one call. If any one of them is
// modified by another client before EXEC, the transaction aborts.
func TestExecAbortsWhenAnyWatchedKeyModified_Stage04WatchMultipleKeys(t *testing.T) {
	requireOptimisticLockingStage(t, 4)
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)
	conn2, r2 := dialClient(t, sp)

	writeRESPArray(t, conn1, "SET", "foo", "100")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET foo 100: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "200")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET bar 200: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "WATCH", "foo", "bar")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("WATCH foo bar: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "300")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("SET bar 300 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn2, "SET", "foo", "200")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("client2 SET foo 200: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems, isNull := readOptimisticExecResult(t, r1)
	if !isNull {
		t.Fatalf("EXEC after one of multiple watched keys modified: expected null array, got %v", elems)
	}

	writeRESPArray(t, conn2, "GET", "bar")
	got := readRESPBulkStringValue(t, r2)
	if got != "200" {
		t.Fatalf("GET bar after aborted transaction: expected unchanged '200', got %q", got)
	}
}

// --- Stage 5: Watching missing keys ---

// Scenario: WATCH on a key that doesn't exist yet still tracks it. If
// another client creates that key before EXEC, that counts as a
// modification and the transaction must abort.
func TestExecAbortsWhenWatchedMissingKeyIsCreated_Stage05WatchMissingKey(t *testing.T) {
	requireOptimisticLockingStage(t, 5)
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)
	conn2, r2 := dialClient(t, sp)

	writeRESPArray(t, conn1, "WATCH", "foo")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("WATCH foo (missing key): expected +OK, got %q", got)
	}

	writeRESPArray(t, conn2, "SET", "foo", "200")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("client2 SET foo 200: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "foo", "300")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("SET foo 300 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems, isNull := readOptimisticExecResult(t, r1)
	if !isNull {
		t.Fatalf("EXEC after watched missing key was created: expected null array, got %v", elems)
	}

	writeRESPArray(t, conn1, "GET", "foo")
	got := readRESPBulkStringValue(t, r1)
	if got != "200" {
		t.Fatalf("GET foo after aborted transaction: expected unchanged '200', got %q", got)
	}
}

// --- Stage 6: The UNWATCH command ---

// Scenario: UNWATCH clears all watched keys for the connection, so a
// subsequent transaction is unaffected by earlier modifications to those
// keys.
func TestUnwatchClearsWatchState_Stage06UnwatchCommand(t *testing.T) {
	requireOptimisticLockingStage(t, 6)
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)
	conn2, r2 := dialClient(t, sp)

	writeRESPArray(t, conn1, "SET", "foo", "100")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET foo 100: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "200")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET bar 200: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "WATCH", "foo", "bar")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("WATCH foo bar: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn2, "SET", "foo", "200")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("client2 SET foo 200: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "UNWATCH")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("UNWATCH: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "foo", "400")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("SET foo 400 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems, isNull := readOptimisticExecResult(t, r1)
	if isNull {
		t.Fatalf("EXEC after UNWATCH: expected non-null array, got null")
	}
	if len(elems) != 1 {
		t.Fatalf("EXEC result after UNWATCH: expected 1 element, got %v", elems)
	}

	writeRESPArray(t, conn1, "GET", "foo")
	got := readRESPBulkStringValue(t, r1)
	if got != "400" {
		t.Fatalf("GET foo after UNWATCH+EXEC: expected '400', got %q", got)
	}
}

// --- Stage 7: Unwatch on EXEC ---

// Scenario: after an EXEC (whether it aborts or succeeds), the connection's
// watch state must be cleared. A second transaction issued right after,
// without a new WATCH, must not be affected by earlier modifications.
func TestWatchStateClearedAfterExec_Stage07UnwatchOnExec(t *testing.T) {
	requireOptimisticLockingStage(t, 7)
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)
	conn2, r2 := dialClient(t, sp)

	writeRESPArray(t, conn1, "SET", "foo", "100")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET foo 100: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "200")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET bar 200: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "WATCH", "foo")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("WATCH foo: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "300")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("SET bar 300 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn2, "SET", "foo", "200")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("client2 SET foo 200: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems, isNull := readOptimisticExecResult(t, r1)
	if !isNull {
		t.Fatalf("first EXEC: expected null array (aborted), got %v", elems)
	}

	// Second transaction on the same connection, no new WATCH issued.
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("second MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "300")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("second SET bar 300 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems, isNull = readOptimisticExecResult(t, r1)
	if isNull {
		t.Fatalf("second EXEC: expected non-null array (watch state cleared by first EXEC), got null")
	}
	if len(elems) != 1 {
		t.Fatalf("second EXEC result: expected 1 element, got %v", elems)
	}

	writeRESPArray(t, conn1, "GET", "bar")
	got := readRESPBulkStringValue(t, r1)
	if got != "300" {
		t.Fatalf("GET bar after second EXEC: expected '300', got %q", got)
	}
}

// --- Stage 8: Unwatch on DISCARD ---

// Scenario: DISCARD aborts a transaction and, like EXEC, clears the
// connection's watch state. A subsequent transaction must not be affected by
// modifications that happened before the DISCARD.
func TestDiscardClearsWatchState_Stage08UnwatchOnDiscard(t *testing.T) {
	requireOptimisticLockingStage(t, 8)
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)
	conn2, r2 := dialClient(t, sp)

	writeRESPArray(t, conn1, "SET", "foo", "100")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET foo 100: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "200")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("SET bar 200: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "WATCH", "foo", "bar")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("WATCH foo bar: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "300")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("SET bar 300 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn2, "SET", "foo", "400")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("client2 SET foo 400: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "DISCARD")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("DISCARD: expected +OK, got %q", got)
	}

	// New transaction on the same connection, no new WATCH issued.
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("second MULTI: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn1, "SET", "bar", "300")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("second SET bar 300 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems, isNull := readOptimisticExecResult(t, r1)
	if isNull {
		t.Fatalf("EXEC after DISCARD: expected non-null array (watch state cleared by DISCARD), got null")
	}
	if len(elems) != 1 {
		t.Fatalf("EXEC result after DISCARD: expected 1 element, got %v", elems)
	}

	writeRESPArray(t, conn1, "GET", "bar")
	got := readRESPBulkStringValue(t, r1)
	if got != "300" {
		t.Fatalf("GET bar after DISCARD+EXEC: expected '300', got %q", got)
	}
}
