package main

// Custom stage tests for PHASE C2: HASHES (see CUSTOM_STAGES.md).
// These stages are project-specific additions, not part of the CodeCrafters
// "Build Your Own Redis" challenge. None of HSET, HGET, HGETALL, HEXISTS,
// HLEN, HDEL are implemented yet, so every test below is expected to be
// skipped until TINYRED_CUSTOM_HASHES_STAGE is raised and the commands are
// implemented server-side.

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

const defaultMaxCustomHashesStage = 0

func maxCustomHashesStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_CUSTOM_HASHES_STAGE"))
	if raw == "" {
		return defaultMaxCustomHashesStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxCustomHashesStage
	}
	if v > 6 {
		return 6
	}
	return v
}

func requireCustomHashesStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxCustomHashesStage() {
		t.Skipf("skipping custom hashes stage %d test; set TINYRED_CUSTOM_HASHES_STAGE=%d (or higher) to run", stage, stage)
	}
}

// customHashesReadFlatMap reads a RESP array of the form
// [field1, value1, field2, value2, ...] and pairs up consecutive elements
// into a map for order-independent comparison (per CUSTOM_STAGES.md,
// HGETALL's field order is unspecified).
func customHashesReadFlatMap(t *testing.T, r *bufio.Reader) map[string]string {
	t.Helper()
	flat := listReadRESPArray(t, r)
	if len(flat)%2 != 0 {
		t.Fatalf("expected an even number of elements in flat hash array, got %d: %v", len(flat), flat)
	}
	m := make(map[string]string, len(flat)/2)
	for i := 0; i+1 < len(flat); i += 2 {
		m[flat[i]] = flat[i+1]
	}
	return m
}

// --- Stage 1: HSET ---

func TestHSetNewFieldReturnsOne_Stage01HSet(t *testing.T) {
	requireCustomHashesStage(t, 1)
	// Scenario: HSET on a brand-new hash with a single new field returns 1.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("HSET new field: expected 1, got %d", got)
	}
}

func TestHSetUpdateExistingAndAddNewCountsOnlyNew_Stage01HSet(t *testing.T) {
	requireCustomHashesStage(t, 1)
	// Scenario: HSET updating an existing field and adding a new one counts
	// only the newly added field.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("initial HSET: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v2", "f2", "v3")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("HSET update+new: expected 1 (only f2 is new), got %d", got)
	}
}

// --- Stage 2: HGET ---

func TestHGetReturnsStoredValue_Stage02HGet(t *testing.T) {
	requireCustomHashesStage(t, 2)
	// Scenario: HGET on an existing field returns its value as a bulk string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HGET", "myhash", "f1")
	if got := readBulkString(t, r); got != "$2\r\nv1\r\n" {
		t.Fatalf("HGET existing field: expected v1, got %q", got)
	}
}

func TestHGetMissingFieldReturnsNull_Stage02HGet(t *testing.T) {
	requireCustomHashesStage(t, 2)
	// Scenario: HGET on a missing field within an existing hash returns a
	// null bulk string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HGET", "myhash", "missing")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("HGET missing field: expected null bulk string, got %q", got)
	}
}

func TestHGetMissingHashReturnsNull_Stage02HGet(t *testing.T) {
	requireCustomHashesStage(t, 2)
	// Scenario: HGET on a hash key that doesn't exist returns a null bulk
	// string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HGET", "missing_hash", "f1")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("HGET missing hash: expected null bulk string, got %q", got)
	}
}

// --- Stage 3: HGETALL ---

func TestHGetAllReturnsAllFieldValuePairs_Stage03HGetAll(t *testing.T) {
	requireCustomHashesStage(t, 3)
	// Scenario: HGETALL returns a flat array of field/value pairs that, when
	// paired up, reconstructs the full hash regardless of order.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1", "f2", "v2")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HGETALL", "myhash")
	got := customHashesReadFlatMap(t, r)
	want := map[string]string{"f1": "v1", "f2": "v2"}
	if len(got) != len(want) {
		t.Fatalf("HGETALL: expected %v, got %v", want, got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("HGETALL: expected %v, got %v", want, got)
		}
	}
}

func TestHGetAllMissingHashReturnsEmptyArray_Stage03HGetAll(t *testing.T) {
	requireCustomHashesStage(t, 3)
	// Scenario: HGETALL on a missing key returns an empty array (*0\r\n).
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HGETALL", "missing_hash")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("HGETALL missing hash: expected empty array, got %v", elems)
	}
}

// --- Stage 4: HEXISTS ---

func TestHExistsExistingField_Stage04HExists(t *testing.T) {
	requireCustomHashesStage(t, 4)
	// Scenario: HEXISTS returns 1 when the field is present in the hash.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HEXISTS", "myhash", "f1")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("HEXISTS existing field: expected 1, got %d", got)
	}
}

func TestHExistsMissingField_Stage04HExists(t *testing.T) {
	requireCustomHashesStage(t, 4)
	// Scenario: HEXISTS returns 0 when the field is absent from an existing
	// hash.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HEXISTS", "myhash", "missing")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("HEXISTS missing field: expected 0, got %d", got)
	}
}

func TestHExistsMissingHash_Stage04HExists(t *testing.T) {
	requireCustomHashesStage(t, 4)
	// Scenario: HEXISTS returns 0 when the hash key itself doesn't exist.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HEXISTS", "missing_hash", "f1")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("HEXISTS missing hash: expected 0, got %d", got)
	}
}

// --- Stage 5: HLEN ---

func TestHLenExistingHash_Stage05HLen(t *testing.T) {
	requireCustomHashesStage(t, 5)
	// Scenario: HLEN returns the number of fields stored in the hash.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1", "f2", "v2")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HLEN", "myhash")
	got := readRESPInteger(t, r)
	if got != 2 {
		t.Fatalf("HLEN: expected 2, got %d", got)
	}
}

func TestHLenMissingHash_Stage05HLen(t *testing.T) {
	requireCustomHashesStage(t, 5)
	// Scenario: HLEN returns 0 for a hash key that doesn't exist.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HLEN", "missing_hash")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("HLEN missing hash: expected 0, got %d", got)
	}
}

// --- Stage 6: HDEL ---

func TestHDelRemovesOnlyExistingFields_Stage06HDel(t *testing.T) {
	requireCustomHashesStage(t, 6)
	// Scenario: HDEL removes the fields that exist and ignores missing ones,
	// returning the count of fields actually removed.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1", "f2", "v2")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HDEL", "myhash", "f1", "missing")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("HDEL: expected 1 (only f1 removed), got %d", got)
	}

	writeRESPArray(t, conn, "HEXISTS", "myhash", "f1")
	existsGot := readRESPInteger(t, r)
	if existsGot != 0 {
		t.Fatalf("HEXISTS after HDEL: expected 0, got %d", existsGot)
	}
}

func TestHDelMissingHashReturnsZero_Stage06HDel(t *testing.T) {
	requireCustomHashesStage(t, 6)
	// Scenario: HDEL on a hash key that doesn't exist returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HDEL", "missing_hash", "f1")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("HDEL missing hash: expected 0, got %d", got)
	}
}

func TestHDelAllFieldsRemovesKeyEntirely_Stage06HDel(t *testing.T) {
	requireCustomHashesStage(t, 6)
	// Scenario: per CUSTOM_STAGES.md notes, deleting all fields from a hash
	// removes the key entirely, matching real Redis (like SREM does for sets).
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f1", "v1", "f2", "v2")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "HDEL", "myhash", "f1", "f2")
	got := readRESPInteger(t, r)
	if got != 2 {
		t.Fatalf("HDEL all fields: expected 2, got %d", got)
	}

	writeRESPArray(t, conn, "HLEN", "myhash")
	lenGot := readRESPInteger(t, r)
	if lenGot != 0 {
		t.Fatalf("HLEN after removing all fields: expected 0 (key gone), got %d", lenGot)
	}

	writeRESPArray(t, conn, "HGETALL", "myhash")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("HGETALL after removing all fields: expected empty array, got %v", elems)
	}
}
