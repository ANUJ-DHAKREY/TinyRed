package tests

import (
	"bufio"
	"strconv"
	"strings"
	"testing"
)

// defaultMaxTransactionStage is 0 because none of the PHASE 3: TRANSACTIONS
// stages (INCR, MULTI, EXEC, DISCARD) are implemented yet. Every test in this
// file skips by default until a developer bumps TINYRED_TRANSACTION_STAGE as
// they implement each stage.
func requireTransactionStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseTransactions)
}

// readTxnExecArrayRaw reads a generic RESP array header ("*N\r\n" or
// "*-1\r\n") and then, for each of the N elements, peeks the type byte and
// dispatches to the matching reader (simple string, error, integer, or bulk
// string). It returns the decoded string value of each element (not
// re-encoded RESP) and a bool indicating whether the array itself was null.
func readTxnExecArrayRaw(t *testing.T, r *bufio.Reader) ([]string, bool) {
	t.Helper()
	header := readLine(t, r)
	if !strings.HasPrefix(header, "*") {
		t.Fatalf("expected RESP array header, got %q", header)
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil {
		t.Fatalf("invalid RESP array length %q: %v", header, err)
	}
	if count < 0 {
		return nil, true
	}

	elements := make([]string, count)
	for i := 0; i < count; i++ {
		typeByte, err := r.Peek(1)
		if err != nil {
			t.Fatalf("failed to peek element %d type: %v", i, err)
		}
		switch typeByte[0] {
		case '+', '-':
			// Simple string or error: return the raw line as-is (including
			// the leading +/- and trailing \r\n) so callers can distinguish
			// errors from successes.
			elements[i] = readLine(t, r)
		case ':':
			elements[i] = strconv.Itoa(readRESPInteger(t, r))
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
			t.Fatalf("unexpected RESP type byte %q for element %d", typeByte[0], i)
		}
	}
	return elements, false
}

// --- Stage 1: INCR (1/3) Key Exists ---

// Scenario: INCR on an existing integer-valued key increments it and returns
// the new value as a RESP integer.
func TestIncrExistingIntegerKey_Stage01IncrKeyExists(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "41")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo 41: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "INCR", "foo")
	got := readRESPInteger(t, r)
	if got != 42 {
		t.Fatalf("INCR foo: expected 42, got %d", got)
	}
}

// --- Stage 2: INCR (2/3) Key Doesn't Exist ---

// Scenario: INCR on a missing key creates it with value 1, and a subsequent
// GET reflects that value as a string.
func TestIncrMissingKeyCreatesItAtOne_Stage02IncrKeyMissing(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "INCR", "missing_key")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("INCR missing_key: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GET", "missing_key")
	gotStr := readBulkString(t, r)
	if gotStr != "$1\r\n1\r\n" {
		t.Fatalf("GET missing_key: expected $1\\r\\n1\\r\\n, got %q", gotStr)
	}
}

// --- Stage 3: INCR (3/3) Non-Integer Value ---

// Scenario: INCR on a key whose value isn't an integer returns a RESP error
// mentioning that the value is not an integer.
func TestIncrNonIntegerValueReturnsError_Stage03IncrNotInteger(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "xyz")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo xyz: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "INCR", "foo")
	got := readLine(t, r)
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("INCR foo (non-integer): expected error line, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "not an integer") {
		t.Fatalf("INCR foo (non-integer): expected error to mention 'not an integer', got %q", got)
	}
}

// --- Stage 4: MULTI ---

// Scenario: MULTI starts a transaction and replies with a simple +OK.
func TestMultiStartsTransaction_Stage04Multi(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "MULTI")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}
}

// --- Stage 5: EXEC without MULTI ---

// Scenario: calling EXEC on a connection that never issued MULTI is an error.
func TestExecWithoutMultiReturnsError_Stage05ExecWithoutMulti(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "EXEC")
	got := readLine(t, r)
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("EXEC without MULTI: expected error line, got %q", got)
	}
	if !strings.Contains(got, "EXEC without MULTI") {
		t.Fatalf("EXEC without MULTI: expected error to mention 'EXEC without MULTI', got %q", got)
	}
}

// --- Stage 6: Empty Transaction ---

// Scenario: MULTI followed immediately by EXEC (nothing queued) returns an
// empty array, and the transaction is considered closed afterward, so a
// second EXEC on the same connection errors out.
func TestEmptyTransactionReturnsEmptyArray_Stage06EmptyTransaction(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "MULTI")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "EXEC")
	elems, isNull := readTxnExecArrayRaw(t, r)
	if isNull {
		t.Fatalf("EXEC empty transaction: expected empty array, got null array")
	}
	if len(elems) != 0 {
		t.Fatalf("EXEC empty transaction: expected 0 elements, got %d", len(elems))
	}

	writeRESPArray(t, conn, "EXEC")
	got := readLine(t, r)
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("second EXEC (no MULTI): expected error line, got %q", got)
	}
	if !strings.Contains(got, "EXEC without MULTI") {
		t.Fatalf("second EXEC (no MULTI): expected error to mention 'EXEC without MULTI', got %q", got)
	}
}

// --- Stage 7: Queueing Commands ---

// Scenario: commands issued inside a MULTI block are queued (+QUEUED) rather
// than executed immediately. A separate connection confirms the queued SET
// has not taken effect yet.
func TestQueuedCommandsDoNotTakeEffectUntilExec_Stage07QueueingCommands(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)

	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "SET", "foo", "41")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("SET foo 41 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn1, "INCR", "foo")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("INCR foo (queued): expected +QUEUED, got %q", got)
	}

	conn2, r2 := dialClient(t, sp)
	writeRESPArray(t, conn2, "GET", "foo")
	got := readBulkString(t, r2)
	if got != "$-1\r\n" {
		t.Fatalf("GET foo from separate connection before EXEC: expected null bulk string, got %q", got)
	}
}

// --- Stage 8: Executing a Transaction ---

// Scenario: EXEC runs all queued commands in order and returns their results
// as a single RESP array; effects are visible afterward on the connection.
func TestExecRunsQueuedCommandsInOrder_Stage08ExecutingTransaction(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "MULTI")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "SET", "foo", "6")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("SET foo 6 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "INCR", "foo")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("INCR foo (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "INCR", "bar")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("INCR bar (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "bar")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("GET bar (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "EXEC")
	elems, isNull := readTxnExecArrayRaw(t, r)
	if isNull {
		t.Fatalf("EXEC: expected array of results, got null array")
	}
	if len(elems) != 4 {
		t.Fatalf("EXEC: expected 4 results, got %d: %v", len(elems), elems)
	}
	if elems[0] != "+OK\r\n" {
		t.Fatalf("EXEC result[0] (SET foo 6): expected +OK\\r\\n, got %q", elems[0])
	}
	if elems[1] != "7" {
		t.Fatalf("EXEC result[1] (INCR foo): expected 7, got %q", elems[1])
	}
	if elems[2] != "1" {
		t.Fatalf("EXEC result[2] (INCR bar): expected 1, got %q", elems[2])
	}
	if elems[3] != "1" {
		t.Fatalf("EXEC result[3] (GET bar): expected 1, got %q", elems[3])
	}

	writeRESPArray(t, conn, "GET", "foo")
	got := readBulkString(t, r)
	if got != "$1\r\n7\r\n" {
		t.Fatalf("GET foo after EXEC: expected $1\\r\\n7\\r\\n, got %q", got)
	}
}

// --- Stage 9: DISCARD ---

// Scenario: DISCARD abandons a queued transaction without executing any of
// its commands, and calling DISCARD again with no active transaction errors.
func TestDiscardAbandonsQueuedTransaction_Stage09Discard(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "MULTI")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "SET", "foo", "41")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("SET foo 41 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "INCR", "foo")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("INCR foo (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "DISCARD")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("DISCARD: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "foo")
	got := readBulkString(t, r)
	if got != "$-1\r\n" {
		t.Fatalf("GET foo after DISCARD: expected null bulk string (never executed), got %q", got)
	}

	writeRESPArray(t, conn, "DISCARD")
	gotErr := readLine(t, r)
	if !strings.HasPrefix(gotErr, "-") {
		t.Fatalf("DISCARD without MULTI: expected error line, got %q", gotErr)
	}
	if !strings.Contains(gotErr, "DISCARD without MULTI") {
		t.Fatalf("DISCARD without MULTI: expected error to mention 'DISCARD without MULTI', got %q", gotErr)
	}
}

// --- Stage 10: Failures Within Transactions ---

// Scenario: when one queued command fails at execution time (e.g. INCR on a
// non-integer value), EXEC still runs the remaining queued commands and their
// successful effects persist; the failure is reported as an error element in
// the EXEC result array rather than aborting the whole transaction.
func TestExecContinuesAfterCommandFailure_Stage10FailuresWithinTransaction(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "xyz")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo xyz: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "SET", "bar", "41")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET bar 41: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "MULTI")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "INCR", "foo")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("INCR foo (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "INCR", "bar")
	if got := readLine(t, r); got != "+QUEUED\r\n" {
		t.Fatalf("INCR bar (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn, "EXEC")
	elems, isNull := readTxnExecArrayRaw(t, r)
	if isNull {
		t.Fatalf("EXEC: expected array of results, got null array")
	}
	if len(elems) != 2 {
		t.Fatalf("EXEC: expected 2 results, got %d: %v", len(elems), elems)
	}
	if !strings.HasPrefix(elems[0], "-") {
		t.Fatalf("EXEC result[0] (INCR foo): expected error, got %q", elems[0])
	}
	if !strings.Contains(strings.ToLower(elems[0]), "not an integer") {
		t.Fatalf("EXEC result[0] (INCR foo): expected error to mention 'not an integer', got %q", elems[0])
	}
	if elems[1] != "42" {
		t.Fatalf("EXEC result[1] (INCR bar): expected 42, got %q", elems[1])
	}

	writeRESPArray(t, conn, "GET", "bar")
	got := readBulkString(t, r)
	if got != "$2\r\n42\r\n" {
		t.Fatalf("GET bar after partial-failure EXEC: expected $2\\r\\n42\\r\\n, got %q", got)
	}
}

// --- Stage 11: Multiple Concurrent Transactions ---

// Scenario: two separate connections each maintain their own independent
// queued-command state; connection 2's transaction is opened only after
// connection 1's EXEC completes, so it observes connection 1's committed
// effect without racing on the shared key.
func TestConcurrentTransactionsHaveIndependentQueues_Stage11MultipleConcurrentTransactions(t *testing.T) {
	requireTransactionStage(t)
	sp := startTinyRed(t)

	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "MULTI")
	if got := readLine(t, r1); got != "+OK\r\n" {
		t.Fatalf("conn1 MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn1, "SET", "foo", "41")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("conn1 SET foo 41 (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn1, "INCR", "foo")
	if got := readLine(t, r1); got != "+QUEUED\r\n" {
		t.Fatalf("conn1 INCR foo (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn1, "EXEC")
	elems1, isNull1 := readTxnExecArrayRaw(t, r1)
	if isNull1 {
		t.Fatalf("conn1 EXEC: expected array of results, got null array")
	}
	if len(elems1) != 2 {
		t.Fatalf("conn1 EXEC: expected 2 results, got %d: %v", len(elems1), elems1)
	}
	if elems1[0] != "+OK\r\n" {
		t.Fatalf("conn1 EXEC result[0] (SET foo 41): expected +OK\\r\\n, got %q", elems1[0])
	}
	if elems1[1] != "42" {
		t.Fatalf("conn1 EXEC result[1] (INCR foo): expected 42, got %q", elems1[1])
	}

	// Only open connection 2 after connection 1's EXEC has fully returned, to
	// avoid racing on the shared "foo" key.
	conn2, r2 := dialClient(t, sp)
	writeRESPArray(t, conn2, "MULTI")
	if got := readLine(t, r2); got != "+OK\r\n" {
		t.Fatalf("conn2 MULTI: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn2, "INCR", "foo")
	if got := readLine(t, r2); got != "+QUEUED\r\n" {
		t.Fatalf("conn2 INCR foo (queued): expected +QUEUED, got %q", got)
	}

	writeRESPArray(t, conn2, "EXEC")
	elems2, isNull2 := readTxnExecArrayRaw(t, r2)
	if isNull2 {
		t.Fatalf("conn2 EXEC: expected array of results, got null array")
	}
	if len(elems2) != 1 {
		t.Fatalf("conn2 EXEC: expected 1 result, got %d: %v", len(elems2), elems2)
	}
	if elems2[0] != "43" {
		t.Fatalf("conn2 EXEC result[0] (INCR foo): expected 43, got %q", elems2[0])
	}
}
