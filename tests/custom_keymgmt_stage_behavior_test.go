package tests

import (
	"sort"
	"strings"
	"testing"
)

// PHASE C3: KEY MANAGEMENT (custom, not part of CodeCrafters).
// See CUSTOM_STAGES.md, section "PHASE C3: KEY MANAGEMENT", for the full spec.
//
// Stage 1 — DEL
// Stage 2 — EXISTS
// Stage 3 — TYPE across all data types
// Stage 4 — KEYS with glob patterns
// Stage 5 — RENAME

func requireCustomKeymgmtStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseCustomKeyManagement)
}

// customKeymgmtSortedStrings returns a sorted copy of the given slice.
func customKeymgmtSortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// --- Stage 1: DEL ---

func TestDelRemovesExistingKeyAndSkipsMissing_Stage01Del(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: SET foo bar; DEL foo missing_key deletes only foo and returns
	// the count of keys that actually existed (1); GET foo then confirms deletion.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo bar: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "DEL", "foo", "missing_key")
	if got := readRESPInteger(t, r); got != 1 {
		t.Fatalf("DEL foo missing_key: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("GET foo after DEL: expected null bulk string, got %q", got)
	}
}

func TestDelWithNoExistingKeysReturnsZero_Stage01Del(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: DEL on keys that never existed returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "DEL", "a", "b", "c")
	if got := readRESPInteger(t, r); got != 0 {
		t.Fatalf("DEL a b c: expected 0, got %d", got)
	}
}

// --- Stage 2: EXISTS ---

func TestExistsCountsRepeatedKeyMultipleTimes_Stage02Exists(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: SET foo bar; EXISTS foo foo missing_key counts foo twice
	// (once per occurrence) since it exists both times it's checked, while
	// missing_key contributes nothing.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo bar: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "EXISTS", "foo", "foo", "missing_key")
	if got := readRESPInteger(t, r); got != 2 {
		t.Fatalf("EXISTS foo foo missing_key: expected 2, got %d", got)
	}
}

func TestExistsWithOnlyMissingKeysReturnsZero_Stage02Exists(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: EXISTS on a key that was never set returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "EXISTS", "missing")
	if got := readRESPInteger(t, r); got != 0 {
		t.Fatalf("EXISTS missing: expected 0, got %d", got)
	}
}

// --- Stage 3: TYPE across all data types ---

func TestTypeReportsListForListKey_Stage03Type(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: RPUSH mylist a; TYPE mylist should report "list" as a RESP
	// simple string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "TYPE", "mylist")
	if got := readLine(t, r); got != "+list\r\n" {
		t.Fatalf("TYPE mylist: expected +list, got %q", got)
	}
}

func TestTypeReportsSetForSetKey_Stage03Type(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: SADD myset a; TYPE myset should report "set" as a RESP simple
	// string. Depends on the custom Sets phase; gated at this same stage for
	// consistency of phase numbering.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "a")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "TYPE", "myset")
	if got := readLine(t, r); got != "+set\r\n" {
		t.Fatalf("TYPE myset: expected +set, got %q", got)
	}
}

func TestTypeReportsHashForHashKey_Stage03Type(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: HSET myhash f v; TYPE myhash should report "hash" as a RESP
	// simple string. Depends on the custom Hashes phase; gated at this same
	// stage for consistency of phase numbering.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "HSET", "myhash", "f", "v")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "TYPE", "myhash")
	if got := readLine(t, r); got != "+hash\r\n" {
		t.Fatalf("TYPE myhash: expected +hash, got %q", got)
	}
}

func TestTypeReportsZsetForSortedSetKey_Stage03Type(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: ZADD myzset 1 a; TYPE myzset should report "zset" as a RESP
	// simple string. Depends on the CodeCrafters Sorted Sets phase; gated at
	// this same stage for consistency of phase numbering.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ZADD", "myzset", "1", "a")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "TYPE", "myzset")
	if got := readLine(t, r); got != "+zset\r\n" {
		t.Fatalf("TYPE myzset: expected +zset, got %q", got)
	}
}

// --- Stage 4: KEYS with glob patterns ---

func TestKeysGlobPatternMatchesSubstring_Stage04KeysGlob(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: three keys are set; KEYS "*name*" should match only the two
	// keys containing "name", regardless of order.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "firstname", "Jack")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET firstname: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn, "SET", "lastname", "Stuntman")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET lastname: expected +OK, got %q", got)
	}
	writeRESPArray(t, conn, "SET", "age", "35")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET age: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "KEYS", "*name*")
	got := listReadRESPArray(t, r)
	want := []string{"firstname", "lastname"}
	if !sliceEqual(customKeymgmtSortedStrings(got), customKeymgmtSortedStrings(want)) {
		t.Fatalf("KEYS *name*: expected %v (any order), got %v", want, got)
	}
}

func TestKeysGlobPatternMatchesSingleCharWildcards_Stage04KeysGlob(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: KEYS "a??" should match only "age" (3-char key starting with
	// "a") among the three keys set.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "firstname", "Jack")
	_ = readLine(t, r)
	writeRESPArray(t, conn, "SET", "lastname", "Stuntman")
	_ = readLine(t, r)
	writeRESPArray(t, conn, "SET", "age", "35")
	_ = readLine(t, r)

	writeRESPArray(t, conn, "KEYS", "a??")
	got := listReadRESPArray(t, r)
	want := []string{"age"}
	if !sliceEqual(customKeymgmtSortedStrings(got), customKeymgmtSortedStrings(want)) {
		t.Fatalf("KEYS a??: expected %v, got %v", want, got)
	}
}

func TestKeysWildcardStillMatchesAllKeys_Stage04KeysGlob(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: regression check that the bare "*" pattern still returns all
	// keys once glob support is added.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "firstname", "Jack")
	_ = readLine(t, r)
	writeRESPArray(t, conn, "SET", "lastname", "Stuntman")
	_ = readLine(t, r)
	writeRESPArray(t, conn, "SET", "age", "35")
	_ = readLine(t, r)

	writeRESPArray(t, conn, "KEYS", "*")
	got := listReadRESPArray(t, r)
	want := []string{"firstname", "lastname", "age"}
	if !sliceEqual(customKeymgmtSortedStrings(got), customKeymgmtSortedStrings(want)) {
		t.Fatalf("KEYS *: expected %v (any order), got %v", want, got)
	}
}

// --- Stage 5: RENAME ---

func TestRenameMovesValueToNewKey_Stage05Rename(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: SET foo bar; RENAME foo baz returns +OK, GET baz returns the
	// value, and GET foo confirms the old key no longer exists.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET foo bar: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "RENAME", "foo", "baz")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("RENAME foo baz: expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "baz")
	if got := readBulkString(t, r); got != "$3\r\nbar\r\n" {
		t.Fatalf("GET baz after RENAME: expected bar, got %q", got)
	}

	writeRESPArray(t, conn, "GET", "foo")
	if got := readBulkString(t, r); got != "$-1\r\n" {
		t.Fatalf("GET foo after RENAME: expected null bulk string, got %q", got)
	}
}

func TestRenameMissingKeyReturnsError_Stage05Rename(t *testing.T) {
	requireCustomKeymgmtStage(t)
	// Scenario: RENAME on a key that doesn't exist returns a RESP error
	// whose text mentions "no such key".
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RENAME", "missing_key", "other")
	got := readLine(t, r)
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("RENAME missing_key: expected RESP error, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "no such key") {
		t.Fatalf("RENAME missing_key: expected error to mention 'no such key', got %q", got)
	}
}
