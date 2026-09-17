package tests

import (
	"bufio"
	"math"
	"strconv"
	"strings"
	"testing"
)

func requireSortedSetStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseSortedSets)
}

// zsetReadIntOrNilBulk reads a response that is either a RESP integer (e.g.
// ":3\r\n") or a null bulk string ("$-1\r\n"). It returns the integer value
// and whether the response was nil.
func zsetReadIntOrNilBulk(t *testing.T, r *bufio.Reader) (val int, isNil bool) {
	t.Helper()
	prefix, err := r.Peek(1)
	if err != nil {
		t.Fatalf("failed to peek response type: %v", err)
	}
	switch prefix[0] {
	case ':':
		return readRESPInteger(t, r), false
	case '$':
		raw := readBulkString(t, r)
		if raw != "$-1\r\n" {
			t.Fatalf("expected null bulk string, got %q", raw)
		}
		return 0, true
	default:
		t.Fatalf("expected RESP integer or null bulk string, got prefix %q", prefix[0])
		return 0, false
	}
}

// zsetReadBulkOrNil reads a bulk-string response and reports whether it was
// the null bulk string ("$-1\r\n"). When not nil, it returns the decoded
// value.
func zsetReadBulkOrNil(t *testing.T, r *bufio.Reader) (val string, isNil bool) {
	t.Helper()
	raw := readBulkString(t, r)
	if raw == "$-1\r\n" {
		return "", true
	}
	parts := strings.SplitN(raw, "\r\n", 3)
	if len(parts) < 2 {
		t.Fatalf("unexpected bulk string format: %q", raw)
	}
	return parts[1], false
}

// zsetAssertScoreEqual compares a decoded score string against an expected
// float using a small epsilon, to avoid brittleness from formatting
// differences (e.g. "30.10" vs "30.1").
func zsetAssertScoreEqual(t *testing.T, got string, want float64) {
	t.Helper()
	gotVal, err := strconv.ParseFloat(got, 64)
	if err != nil {
		t.Fatalf("failed to parse score %q as float: %v", got, err)
	}
	if math.Abs(gotVal-want) > 1e-9 {
		t.Fatalf("expected score %v, got %v (raw %q)", want, gotVal, got)
	}
}

// --- Stage 1: ZADD creates a new sorted set ---

func TestZAddCreatesNewSortedSet_Stage01ZAddCreate(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZADD on a brand-new key creates the sorted set and reports one new member added.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ZADD", "zset_key", "10.0", "zset_member")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("ZADD new sorted set: expected 1, got %d", got)
	}
}

// --- Stage 2: ZADD adds members to an existing sorted set ---

func TestZAddAddsNewMembersAndUpdatesExistingMember_Stage02ZAddMultipleMembers(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZADD reports 1 for each newly added member, and 0 when only updating an existing member's score.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	members := []struct {
		score  string
		member string
	}{
		{"20.0", "m1"},
		{"30.1", "m2"},
		{"40.2", "m3"},
		{"50.3", "m4"},
	}
	for _, m := range members {
		writeRESPArray(t, conn, "ZADD", "zset_key", m.score, m.member)
		got := readRESPInteger(t, r)
		if got != 1 {
			t.Fatalf("ZADD new member %q: expected 1, got %d", m.member, got)
		}
	}

	writeRESPArray(t, conn, "ZADD", "zset_key", "100.0", "m1")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("ZADD update existing member: expected 0, got %d", got)
	}
}

// --- Stage 3: ZRANK ---

func TestZRankOrdersByScoreWithLexicographicTiebreak_Stage03ZRank(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZRANK returns the 0-based rank ordered by ascending score, ties broken lexicographically.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	members := []struct {
		score  string
		member string
	}{
		{"100.0", "foo"},
		{"100.0", "bar"},
		{"20.0", "baz"},
		{"30.1", "caz"},
		{"40.2", "paz"},
	}
	for _, m := range members {
		writeRESPArray(t, conn, "ZADD", "zset_key", m.score, m.member)
		got := readRESPInteger(t, r)
		if got != 1 {
			t.Fatalf("ZADD member %q: expected 1, got %d", m.member, got)
		}
	}

	expectedRanks := map[string]int{
		"baz": 0,
		"caz": 1,
		"paz": 2,
		"bar": 3,
		"foo": 4,
	}
	for _, member := range []string{"caz", "baz", "foo", "bar"} {
		writeRESPArray(t, conn, "ZRANK", "zset_key", member)
		val, isNil := zsetReadIntOrNilBulk(t, r)
		if isNil {
			t.Fatalf("ZRANK %q: expected integer, got nil", member)
		}
		if val != expectedRanks[member] {
			t.Fatalf("ZRANK %q: expected %d, got %d", member, expectedRanks[member], val)
		}
	}
}

func TestZRankReturnsNullForMissingMemberOrKey_Stage03ZRank(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZRANK returns a null bulk string when the member or the sorted set itself is missing.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ZADD", "zset_key", "100.0", "foo")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "ZRANK", "zset_key", "missing_member")
	_, isNil := zsetReadIntOrNilBulk(t, r)
	if !isNil {
		t.Fatalf("ZRANK missing member: expected nil")
	}

	writeRESPArray(t, conn, "ZRANK", "missing_key", "member")
	_, isNil = zsetReadIntOrNilBulk(t, r)
	if !isNil {
		t.Fatalf("ZRANK missing key: expected nil")
	}
}

// --- Stage 4: ZRANGE with non-negative indexes ---

func TestZRangeWithNonNegativeIndexesReturnsSliceOrderedByScore_Stage04ZRange(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZRANGE with non-negative start/stop returns members ordered by ascending score (ties lexicographic).
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	members := []struct {
		score  string
		member string
	}{
		{"100.0", "foo"},
		{"100.0", "bar"},
		{"20.0", "baz"},
		{"30.1", "caz"},
		{"40.2", "paz"},
	}
	for _, m := range members {
		writeRESPArray(t, conn, "ZADD", "zset_key", m.score, m.member)
		_ = readRESPInteger(t, r)
	}

	writeRESPArray(t, conn, "ZRANGE", "zset_key", "2", "4")
	elems := listReadRESPArray(t, r)
	expected := []string{"paz", "bar", "foo"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("ZRANGE 2 4: expected %v, got %v", expected, elems)
	}
}

func TestZRangeOnMissingKeyReturnsEmptyArray_Stage04ZRange(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZRANGE on a non-existent key returns an empty array.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ZRANGE", "missing_key", "0", "-1")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("ZRANGE missing_key: expected empty array, got %v", elems)
	}
}

// --- Stage 5: ZRANGE with negative indexes ---

func TestZRangeWithNegativeIndexes_Stage05ZRange(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZRANGE supports negative indexes counting from the end of the ordered set.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	members := []struct {
		score  string
		member string
	}{
		{"20.0", "foo"},
		{"30.1", "bar"},
		{"40.2", "baz"},
		{"25.0", "paz"},
		{"25.0", "caz"},
	}
	for _, m := range members {
		writeRESPArray(t, conn, "ZADD", "zset_key", m.score, m.member)
		_ = readRESPInteger(t, r)
	}

	writeRESPArray(t, conn, "ZRANGE", "zset_key", "2", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"paz", "bar", "baz"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("ZRANGE 2 -1: expected %v, got %v", expected, elems)
	}
}

// --- Stage 6: ZCARD ---

func TestZCardReturnsMemberCountAndIsUnaffectedByScoreUpdates_Stage06ZCard(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZCARD returns the number of members, which does not change when an existing member's score is updated.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	members := []struct {
		score  string
		member string
	}{
		{"20.0", "m1"},
		{"30.1", "m2"},
		{"40.2", "m3"},
		{"50.3", "m4"},
	}
	for _, m := range members {
		writeRESPArray(t, conn, "ZADD", "zset_key", m.score, m.member)
		_ = readRESPInteger(t, r)
	}

	writeRESPArray(t, conn, "ZCARD", "zset_key")
	got := readRESPInteger(t, r)
	if got != 4 {
		t.Fatalf("ZCARD: expected 4, got %d", got)
	}

	writeRESPArray(t, conn, "ZADD", "zset_key", "100.0", "m1")
	addGot := readRESPInteger(t, r)
	if addGot != 0 {
		t.Fatalf("ZADD update existing member: expected 0, got %d", addGot)
	}

	writeRESPArray(t, conn, "ZCARD", "zset_key")
	got = readRESPInteger(t, r)
	if got != 4 {
		t.Fatalf("ZCARD after update: expected 4, got %d", got)
	}
}

func TestZCardOnMissingKeyReturnsZero_Stage06ZCard(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZCARD on a non-existent key returns 0.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ZCARD", "missing_key")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("ZCARD missing_key: expected 0, got %d", got)
	}
}

// --- Stage 7: ZSCORE ---

func TestZScoreReturnsMemberScoreAndReflectsUpdates_Stage07ZScore(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZSCORE returns the current score of a member, reflecting subsequent score updates.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	members := []struct {
		score  string
		member string
	}{
		{"20.0", "m1"},
		{"30.1", "m2"},
		{"40.2", "m3"},
		{"50.3", "m4"},
	}
	for _, m := range members {
		writeRESPArray(t, conn, "ZADD", "zset_key", m.score, m.member)
		_ = readRESPInteger(t, r)
	}

	writeRESPArray(t, conn, "ZSCORE", "zset_key", "m2")
	val, isNil := zsetReadBulkOrNil(t, r)
	if isNil {
		t.Fatalf("ZSCORE m2: expected a value, got nil")
	}
	zsetAssertScoreEqual(t, val, 30.1)

	writeRESPArray(t, conn, "ZADD", "zset_key", "100.99", "m2")
	addGot := readRESPInteger(t, r)
	if addGot != 0 {
		t.Fatalf("ZADD update m2: expected 0, got %d", addGot)
	}

	writeRESPArray(t, conn, "ZSCORE", "zset_key", "m2")
	val, isNil = zsetReadBulkOrNil(t, r)
	if isNil {
		t.Fatalf("ZSCORE m2 after update: expected a value, got nil")
	}
	zsetAssertScoreEqual(t, val, 100.99)
}

func TestZScoreReturnsNullForMissingMemberOrKey_Stage07ZScore(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZSCORE returns a null bulk string when the member or the sorted set itself is missing.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "ZADD", "zset_key", "20.0", "m1")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "ZSCORE", "zset_key", "missing_member")
	_, isNil := zsetReadBulkOrNil(t, r)
	if !isNil {
		t.Fatalf("ZSCORE missing member: expected nil")
	}

	writeRESPArray(t, conn, "ZSCORE", "missing_key", "member")
	_, isNil = zsetReadBulkOrNil(t, r)
	if !isNil {
		t.Fatalf("ZSCORE missing key: expected nil")
	}
}

// --- Stage 8: ZREM ---

func TestZRemRemovesMemberAndReturnsZeroForMissingMember_Stage08ZRem(t *testing.T) {
	requireSortedSetStage(t)
	// Scenario: ZREM removes an existing member (returning 1), updates ordering seen via ZRANGE, and returns 0 for a member that doesn't exist.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	members := []struct {
		score  string
		member string
	}{
		{"80.5", "foo"},
		{"50.3", "baz"},
		{"80.5", "bar"},
	}
	for _, m := range members {
		writeRESPArray(t, conn, "ZADD", "zset_key", m.score, m.member)
		got := readRESPInteger(t, r)
		if got != 1 {
			t.Fatalf("ZADD member %q: expected 1, got %d", m.member, got)
		}
	}

	writeRESPArray(t, conn, "ZREM", "zset_key", "baz")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("ZREM existing member: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "ZRANGE", "zset_key", "0", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"bar", "foo"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("ZRANGE after ZREM: expected %v, got %v", expected, elems)
	}

	writeRESPArray(t, conn, "ZREM", "zset_key", "missing_member")
	got = readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("ZREM missing member: expected 0, got %d", got)
	}
}
