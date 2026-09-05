package tests

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const defaultMaxCustomTTLStage = 0

func maxCustomTTLStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_CUSTOM_TTL_STAGE"))
	if raw == "" {
		return defaultMaxCustomTTLStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxCustomTTLStage
	}
	if v > 5 {
		return 5
	}
	return v
}

func requireCustomTTLStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxCustomTTLStage() {
		t.Skipf("skipping custom TTL stage %d test; set TINYRED_CUSTOM_TTL_STAGE=%d (or higher) to run", stage, stage)
	}
}

// customTTLReadRESPIntegerInRange reads a RESP integer and asserts it falls within [min, max] inclusive.
func customTTLReadRESPIntegerInRange(t *testing.T, r *bufio.Reader, min, max int, label string) int {
	t.Helper()
	got := readRESPInteger(t, r)
	if got < min || got > max {
		t.Fatalf("%s: expected value in [%d, %d], got %d", label, min, max, got)
	}
	return got
}

// --- Stage 1: EXPIRE ---

func TestExpireSetsTTLOnExistingKey_Stage01Expire(t *testing.T) {
	requireCustomTTLStage(t, 1)
	// Scenario: EXPIRE on an existing key sets a TTL and returns 1.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "EXPIRE", "foo", "100")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("EXPIRE existing key: expected 1, got %d", got)
	}
}

func TestExpireOnMissingKeyReturnsZero_Stage01Expire(t *testing.T) {
	requireCustomTTLStage(t, 1)
	// Scenario: EXPIRE on a non-existent key returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "EXPIRE", "missing_key", "100")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("EXPIRE missing key: expected 0, got %d", got)
	}
}

func TestExpireInSecondsActuallyExpiresKey_Stage01Expire(t *testing.T) {
	requireCustomTTLStage(t, 1)
	// Scenario: EXPIRE's unit is genuinely seconds; after the TTL elapses the key is gone.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "short", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "EXPIRE", "short", "1")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("EXPIRE short: expected 1, got %d", got)
	}

	time.Sleep(1200 * time.Millisecond)

	writeRESPArray(t, conn, "GET", "short")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("GET after EXPIRE elapsed: expected null bulk string, got %q", got)
	}
}

// --- Stage 2: PEXPIRE ---

func TestPExpireSetsTTLOnExistingKey_Stage02PExpire(t *testing.T) {
	requireCustomTTLStage(t, 2)
	// Scenario: PEXPIRE on an existing key sets a millisecond TTL and returns 1.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "PEXPIRE", "foo", "150")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("PEXPIRE existing key: expected 1, got %d", got)
	}
}

func TestPExpireExpiresKeyAfterMilliseconds_Stage02PExpire(t *testing.T) {
	requireCustomTTLStage(t, 2)
	// Scenario: key is present immediately after PEXPIRE, then expires once the millisecond TTL elapses.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "PEXPIRE", "foo", "150")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("PEXPIRE: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$3\r\nbar\r\n" {
		t.Fatalf("immediate GET after PEXPIRE: expected bar, got %q", got)
	}

	time.Sleep(200 * time.Millisecond)

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("GET after PEXPIRE elapsed: expected null bulk string, got %q", got)
	}
}

func TestPExpireOnMissingKeyReturnsZero_Stage02PExpire(t *testing.T) {
	requireCustomTTLStage(t, 2)
	// Scenario: PEXPIRE on a non-existent key returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "PEXPIRE", "missing_key", "100")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("PEXPIRE missing key: expected 0, got %d", got)
	}
}

// --- Stage 3: TTL ---

func TestTTLReportsNoExpiryThenSecondsAfterExpire_Stage03TTL(t *testing.T) {
	requireCustomTTLStage(t, 3)
	// Scenario: TTL is -1 before any expiry is set, then reflects the remaining seconds after EXPIRE.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "TTL", "foo")
	got := readRESPInteger(t, r)
	if got != -1 {
		t.Fatalf("TTL with no expiry: expected -1, got %d", got)
	}

	writeRESPArray(t, conn, "EXPIRE", "foo", "100")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("EXPIRE: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "TTL", "foo")
	customTTLReadRESPIntegerInRange(t, r, 99, 100, "TTL after EXPIRE 100")
}

func TestTTLOnMissingKeyReturnsNegativeTwo_Stage03TTL(t *testing.T) {
	requireCustomTTLStage(t, 3)
	// Scenario: TTL on a non-existent key returns -2.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "TTL", "missing_key")
	got := readRESPInteger(t, r)
	if got != -2 {
		t.Fatalf("TTL missing key: expected -2, got %d", got)
	}
}

// --- Stage 4: PTTL ---

func TestPTTLReportsRemainingMillisecondsForKeyWithPXExpiry_Stage04PTTL(t *testing.T) {
	requireCustomTTLStage(t, 4)
	// Scenario: PTTL on a key set with SET ... PX returns the remaining TTL in milliseconds.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar", "PX", "10000")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET PX expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "PTTL", "foo")
	val := readRESPInteger(t, r)
	if !(val > 0 && val <= 10000) {
		t.Fatalf("PTTL foo: expected value in (0, 10000], got %d", val)
	}
}

func TestPTTLOnMissingKeyReturnsNegativeTwo_Stage04PTTL(t *testing.T) {
	requireCustomTTLStage(t, 4)
	// Scenario: PTTL on a non-existent key returns -2.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "PTTL", "missing_key")
	got := readRESPInteger(t, r)
	if got != -2 {
		t.Fatalf("PTTL missing key: expected -2, got %d", got)
	}
}

func TestPTTLOnKeyWithoutExpiryReturnsNegativeOne_Stage04PTTL(t *testing.T) {
	requireCustomTTLStage(t, 4)
	// Scenario: PTTL on a key with no expiry returns -1.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "bar", "baz")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "PTTL", "bar")
	got := readRESPInteger(t, r)
	if got != -1 {
		t.Fatalf("PTTL no expiry: expected -1, got %d", got)
	}
}

// --- Stage 5: PERSIST ---

func TestPersistRemovesExpiryFromKey_Stage05Persist(t *testing.T) {
	requireCustomTTLStage(t, 5)
	// Scenario: PERSIST removes a key's TTL, confirmed via TTL returning -1 afterward.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar", "EX", "100")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET EX expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "PERSIST", "foo")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("PERSIST on key with expiry: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "TTL", "foo")
	got = readRESPInteger(t, r)
	if got != -1 {
		t.Fatalf("TTL after PERSIST: expected -1, got %d", got)
	}
}

func TestPersistOnAlreadyPersistentKeyReturnsZero_Stage05Persist(t *testing.T) {
	requireCustomTTLStage(t, 5)
	// Scenario: calling PERSIST again on a key with no expiry left to remove returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar", "EX", "100")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET EX expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "PERSIST", "foo")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("first PERSIST: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "PERSIST", "foo")
	got = readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("second PERSIST: expected 0, got %d", got)
	}
}

func TestPersistOnMissingKeyReturnsZero_Stage05Persist(t *testing.T) {
	requireCustomTTLStage(t, 5)
	// Scenario: PERSIST on a non-existent key returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "PERSIST", "missing_key")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("PERSIST missing key: expected 0, got %d", got)
	}
}
