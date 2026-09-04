package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Custom stage: PHASE C5 — BATCH STRING OPS (not part of CodeCrafters).
// See CUSTOM_STAGES.md, "PHASE C5: BATCH STRING OPS" for the full spec.

const defaultMaxCustomStringOpsStage = 0

func maxCustomStringOpsStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_CUSTOM_STRINGOPS_STAGE"))
	if raw == "" {
		return defaultMaxCustomStringOpsStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxCustomStringOpsStage
	}
	if v > 6 {
		return 6
	}
	return v
}

func requireCustomStringOpsStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxCustomStringOpsStage() {
		t.Skipf("skipping custom stringops stage %d test; set TINYRED_CUSTOM_STRINGOPS_STAGE=%d (or higher) to run", stage, stage)
	}
}

// customStringOpsReadMGetArray reads a RESP array whose elements may be a mix
// of bulk strings and null bulk strings (as returned by MGET), returning a
// slice of *string where nil represents a null bulk string element.
func customStringOpsReadMGetArray(t *testing.T, r *bufio.Reader) []*string {
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
		return nil
	}
	elements := make([]*string, count)
	for i := 0; i < count; i++ {
		raw := readBulkString(t, r)
		if raw == "$-1\r\n" {
			elements[i] = nil
			continue
		}
		parts := strings.SplitN(raw, "\r\n", 3)
		if len(parts) < 2 {
			t.Fatalf("unexpected bulk string format: %q", raw)
		}
		val := parts[1]
		elements[i] = &val
	}
	return elements
}

// --- Stage 1: MSET ---

func TestMSetStoresMultipleKeys_Stage01MSet(t *testing.T) {
	requireCustomStringOpsStage(t, 1)
	// Scenario: MSET sets several key-value pairs at once, replying +OK, and
	// each key is subsequently retrievable via GET.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "MSET", "a", "1", "b", "2", "c", "3")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("MSET: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "a")
	if got := readBulkString(t, r); got != "$1\r\n1\r\n" {
		t.Fatalf("GET a: expected 1, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "b")
	if got := readBulkString(t, r); got != "$1\r\n2\r\n" {
		t.Fatalf("GET b: expected 2, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "c")
	if got := readBulkString(t, r); got != "$1\r\n3\r\n" {
		t.Fatalf("GET c: expected 3, got %q", got)
	}
}

// --- Stage 2: MGET ---

func TestMGetReturnsMixOfValuesAndNulls_Stage02MGet(t *testing.T) {
	requireCustomStringOpsStage(t, 2)
	// Scenario: MGET returns a RESP array with the value for an existing key
	// and null bulk strings for keys that were never set.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "a", "1")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET a: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "MGET", "a", "missing", "b")
	elems := customStringOpsReadMGetArray(t, r)
	if len(elems) != 3 {
		t.Fatalf("MGET: expected 3 elements, got %d (%v)", len(elems), elems)
	}
	if elems[0] == nil || *elems[0] != "1" {
		t.Fatalf("MGET element 0: expected \"1\", got %v", elems[0])
	}
	if elems[1] != nil {
		t.Fatalf("MGET element 1 (missing): expected nil, got %v", *elems[1])
	}
	if elems[2] != nil {
		t.Fatalf("MGET element 2 (b): expected nil, got %v", *elems[2])
	}
}

// --- Stage 3: APPEND ---

func TestAppendExtendsExistingAndCreatesMissingKey_Stage03Append(t *testing.T) {
	requireCustomStringOpsStage(t, 3)
	// Scenario: APPEND extends an existing string and returns the new total
	// length; APPEND on a missing key creates it with the appended value.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "Hello ")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "APPEND", "foo", "World")
	if got := readRESPInteger(t, r); got != 11 {
		t.Fatalf("APPEND foo World: expected 11, got %d", got)
	}

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$11\r\nHello World\r\n" {
		t.Fatalf("GET foo: expected \"Hello World\", got %q", got)
	}

	writeRESPArray(t, conn, "APPEND", "newkey", "abc")
	if got := readRESPInteger(t, r); got != 3 {
		t.Fatalf("APPEND newkey abc: expected 3, got %d", got)
	}

	writeRESPArray(t, conn, "GET", "newkey")
	if got := readBulkString(t, r); got != "$3\r\nabc\r\n" {
		t.Fatalf("GET newkey: expected \"abc\", got %q", got)
	}
}

// --- Stage 4: DECR ---

func TestDecrDecrementsCreatesAndErrorsOnNonInteger_Stage04Decr(t *testing.T) {
	requireCustomStringOpsStage(t, 4)
	// Scenario: DECR decrements an existing integer value, creates missing
	// keys at -1, and errors when the existing value isn't an integer.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "10")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "DECR", "foo")
	if got := readRESPInteger(t, r); got != 9 {
		t.Fatalf("DECR foo: expected 9, got %d", got)
	}

	writeRESPArray(t, conn, "DECR", "missing_key")
	if got := readRESPInteger(t, r); got != -1 {
		t.Fatalf("DECR missing_key: expected -1, got %d", got)
	}

	writeRESPArray(t, conn, "SET", "bar", "xyz")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET bar: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "DECR", "bar")
	got := readLine(t, r)
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("DECR bar (non-integer): expected error line, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "not an integer") {
		t.Fatalf("DECR bar (non-integer): expected error to mention 'not an integer', got %q", got)
	}
}

// --- Stage 5: INCRBY ---

func TestIncrByAppliesPositiveAndNegativeAmounts_Stage05IncrBy(t *testing.T) {
	requireCustomStringOpsStage(t, 5)
	// Scenario: INCRBY increments by the given (possibly negative) amount,
	// and creates missing keys starting from 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "10")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "INCRBY", "foo", "5")
	if got := readRESPInteger(t, r); got != 15 {
		t.Fatalf("INCRBY foo 5: expected 15, got %d", got)
	}

	writeRESPArray(t, conn, "INCRBY", "foo", "-20")
	if got := readRESPInteger(t, r); got != -5 {
		t.Fatalf("INCRBY foo -20: expected -5, got %d", got)
	}

	writeRESPArray(t, conn, "INCRBY", "missing_key", "3")
	if got := readRESPInteger(t, r); got != 3 {
		t.Fatalf("INCRBY missing_key 3: expected 3, got %d", got)
	}
}

// --- Stage 6: DECRBY ---

func TestDecrByAppliesAmountAndCreatesMissingKey_Stage06DecrBy(t *testing.T) {
	requireCustomStringOpsStage(t, 6)
	// Scenario: DECRBY decrements by the given amount, and creates missing
	// keys starting from 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "10")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "DECRBY", "foo", "3")
	if got := readRESPInteger(t, r); got != 7 {
		t.Fatalf("DECRBY foo 3: expected 7, got %d", got)
	}

	writeRESPArray(t, conn, "DECRBY", "missing_key", "3")
	if got := readRESPInteger(t, r); got != -3 {
		t.Fatalf("DECRBY missing_key 3: expected -3, got %d", got)
	}
}
