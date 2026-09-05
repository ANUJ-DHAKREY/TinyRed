package tests

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file covers PHASE C1: SETS from CUSTOM_STAGES.md, a project-specific
// custom phase (NOT part of the CodeCrafters "Build Your Own Redis" challenge).
// None of SADD, SMEMBERS, SISMEMBER, SCARD, SREM are implemented yet, so all
// tests here are gated behind TINYRED_CUSTOM_SETS_STAGE and skip by default.

const defaultMaxCustomSetsStage = 0

func maxCustomSetsStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_CUSTOM_SETS_STAGE"))
	if raw == "" {
		return defaultMaxCustomSetsStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxCustomSetsStage
	}
	if v > 5 {
		return 5
	}
	return v
}

func requireCustomSetsStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxCustomSetsStage() {
		t.Skipf("skipping custom sets stage %d test; set TINYRED_CUSTOM_SETS_STAGE=%d (or higher) to run", stage, stage)
	}
}

// customSetsSortedCopy returns a sorted copy of elems, leaving the input untouched.
func customSetsSortedCopy(elems []string) []string {
	out := make([]string, len(elems))
	copy(out, elems)
	sort.Strings(out)
	return out
}

// customSetsReadRESPError reads a RESP simple-error line (e.g. "-WRONGTYPE ...\r\n")
// and returns it verbatim.
func customSetsReadRESPError(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line := readLine(t, r)
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected RESP error, got %q", line)
	}
	return line
}

// --- Stage 1: SADD ---

func TestSAddNewMemberReturnsOne_Stage01SAdd(t *testing.T) {
	requireCustomSetsStage(t, 1)
	// Scenario: SADD on a brand-new set with a single member reports 1 new member.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "foo")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("SADD new member: expected 1, got %d", got)
	}
}

func TestSAddMixOfExistingAndNewMembersCountsOnlyNew_Stage01SAdd(t *testing.T) {
	requireCustomSetsStage(t, 1)
	// Scenario: SADD with a mix of an already-present member and a new one only
	// counts the newly added member.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "foo")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("first SADD: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "SADD", "myset", "foo", "bar")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("SADD foo bar: expected 1 (only bar is new), got %d", got)
	}
}

func TestSAddDuplicateMemberReturnsZero_Stage01SAdd(t *testing.T) {
	requireCustomSetsStage(t, 1)
	// Scenario: re-adding an already-present member reports zero newly added members.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "foo")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("first SADD: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "SADD", "myset", "foo")
	got = readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("SADD duplicate: expected 0, got %d", got)
	}
}

func TestSAddOnStringKeyReturnsWrongType_Stage01SAdd(t *testing.T) {
	requireCustomSetsStage(t, 1)
	// Scenario: SADD against a key holding a string value must fail with a
	// WRONGTYPE error rather than silently converting or corrupting the value.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "stringkey", "val")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "SADD", "stringkey", "member")
	errLine := customSetsReadRESPError(t, r)
	if !strings.Contains(errLine, "WRONGTYPE") {
		t.Fatalf("SADD on string key: expected WRONGTYPE error, got %q", errLine)
	}
}

// --- Stage 2: SMEMBERS ---

func TestSMembersReturnsAllAddedMembers_Stage02SMembers(t *testing.T) {
	requireCustomSetsStage(t, 2)
	// Scenario: SMEMBERS returns exactly the members added via SADD, order-independent.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "SMEMBERS", "myset")
	elems := listReadRESPArray(t, r)
	got := customSetsSortedCopy(elems)
	want := []string{"a", "b", "c"}
	if !sliceEqual(got, want) {
		t.Fatalf("SMEMBERS: expected sorted %v, got sorted %v (raw %v)", want, got, elems)
	}
}

func TestSMembersOnMissingSetReturnsEmptyArray_Stage02SMembers(t *testing.T) {
	requireCustomSetsStage(t, 2)
	// Scenario: SMEMBERS on a key that was never created returns an empty array,
	// not an error.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SMEMBERS", "missing_set")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("SMEMBERS missing set: expected empty array, got %v", elems)
	}
}

// --- Stage 3: SISMEMBER ---

func TestSIsMemberOnPresentMemberReturnsOne_Stage03SIsMember(t *testing.T) {
	requireCustomSetsStage(t, 3)
	// Scenario: SISMEMBER reports 1 for a member that was added to the set.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "foo")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("SADD: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "SISMEMBER", "myset", "foo")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("SISMEMBER present member: expected 1, got %d", got)
	}
}

func TestSIsMemberOnAbsentMemberReturnsZero_Stage03SIsMember(t *testing.T) {
	requireCustomSetsStage(t, 3)
	// Scenario: SISMEMBER reports 0 for a member never added to an existing set.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "foo")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "SISMEMBER", "myset", "bar")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("SISMEMBER absent member: expected 0, got %d", got)
	}
}

func TestSIsMemberOnMissingSetReturnsZero_Stage03SIsMember(t *testing.T) {
	requireCustomSetsStage(t, 3)
	// Scenario: SISMEMBER against a key that doesn't exist at all reports 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SISMEMBER", "missing_set", "foo")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("SISMEMBER missing set: expected 0, got %d", got)
	}
}

// --- Stage 4: SCARD ---

func TestSCardReturnsMemberCount_Stage04SCard(t *testing.T) {
	requireCustomSetsStage(t, 4)
	// Scenario: SCARD reports the number of distinct members in an existing set.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "SCARD", "myset")
	got := readRESPInteger(t, r)
	if got != 3 {
		t.Fatalf("SCARD: expected 3, got %d", got)
	}
}

func TestSCardOnMissingSetReturnsZero_Stage04SCard(t *testing.T) {
	requireCustomSetsStage(t, 4)
	// Scenario: SCARD on a nonexistent key reports 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SCARD", "missing_set")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("SCARD missing set: expected 0, got %d", got)
	}
}

// --- Stage 5: SREM ---

func TestSRemRemovesOnlyPresentMembers_Stage05SRem(t *testing.T) {
	requireCustomSetsStage(t, 5)
	// Scenario: SREM with one present and one absent member removes only the
	// present one and counts just that removal.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SADD", "myset", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "SREM", "myset", "b", "missing_member")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("SREM b missing_member: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "SMEMBERS", "myset")
	elems := listReadRESPArray(t, r)
	gotSorted := customSetsSortedCopy(elems)
	want := []string{"a", "c"}
	if !sliceEqual(gotSorted, want) {
		t.Fatalf("SMEMBERS after SREM: expected sorted %v, got sorted %v (raw %v)", want, gotSorted, elems)
	}
}

func TestSRemOnMissingSetReturnsZero_Stage05SRem(t *testing.T) {
	requireCustomSetsStage(t, 5)
	// Scenario: SREM against a key that doesn't exist reports 0 removed members.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SREM", "missing_set", "x")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("SREM missing set: expected 0, got %d", got)
	}
}
