package tests

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const defaultMaxListStage = 11

func maxListStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_LIST_STAGE"))
	if raw == "" {
		return defaultMaxListStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxListStage
	}
	if v > 11 {
		return 11
	}
	return v
}

func requireListStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxListStage() {
		t.Skipf("skipping list stage %d test; set TINYRED_LIST_STAGE=%d (or higher) to run", stage, stage)
	}
}

// readRESPInteger reads a RESP integer response (e.g. ":3\r\n") and returns the integer value.
func readRESPInteger(t *testing.T, r *bufio.Reader) int {
	t.Helper()
	line := readLine(t, r)
	if !strings.HasPrefix(line, ":") {
		t.Fatalf("expected RESP integer, got %q", line)
	}
	val, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, ":")))
	if err != nil {
		t.Fatalf("invalid RESP integer %q: %v", line, err)
	}
	return val
}

// listReadRESPArray reads a RESP array and returns the bulk string elements.
// Returns nil for null arrays (*-1\r\n).
func listReadRESPArray(t *testing.T, r *bufio.Reader) []string {
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
	elements := make([]string, count)
	for i := 0; i < count; i++ {
		bs := readBulkString(t, r)
		// Parse out just the value from "$N\r\nVALUE\r\n"
		parts := strings.SplitN(bs, "\r\n", 3)
		if len(parts) >= 2 {
			elements[i] = parts[1]
		}
	}
	return elements
}

// readRESPArrayRaw reads a RESP array header and returns all raw bulk strings.
func readRESPBulkStringValue(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	raw := readBulkString(t, r)
	if raw == "$-1\r\n" {
		return ""
	}
	parts := strings.SplitN(raw, "\r\n", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	t.Fatalf("unexpected bulk string format: %q", raw)
	return ""
}

// --- Stage 1: RPUSH creates a new list ---

func TestRPushCreatesNewListWithSingleElement_Stage01(t *testing.T) {
	requireListStage(t, 1)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "element")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("RPUSH new list: expected 1, got %d", got)
	}
}

func TestRPushCreatesNewListDifferentKeys_Stage01(t *testing.T) {
	requireListStage(t, 1)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "list1", "a")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("RPUSH list1: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "RPUSH", "list2", "b")
	got = readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("RPUSH list2: expected 1, got %d", got)
	}
}

// --- Stage 2: RPUSH appends to an existing list ---

func TestRPushAppendsToExistingList_Stage02(t *testing.T) {
	requireListStage(t, 2)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "element1")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("first RPUSH: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "RPUSH", "mylist", "element2")
	got = readRESPInteger(t, r)
	if got != 2 {
		t.Fatalf("second RPUSH: expected 2, got %d", got)
	}

	writeRESPArray(t, conn, "RPUSH", "mylist", "element3")
	got = readRESPInteger(t, r)
	if got != 3 {
		t.Fatalf("third RPUSH: expected 3, got %d", got)
	}
}

func TestRPushAppendsMultipleCallsSameList_Stage02(t *testing.T) {
	requireListStage(t, 2)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	for i := 1; i <= 5; i++ {
		writeRESPArray(t, conn, "RPUSH", "counter", fmt.Sprintf("item%d", i))
		got := readRESPInteger(t, r)
		if got != i {
			t.Fatalf("RPUSH call %d: expected %d, got %d", i, i, got)
		}
	}
}

// --- Stage 3: RPUSH with multiple elements ---

func TestRPushMultipleElementsNewList_Stage03(t *testing.T) {
	requireListStage(t, 3)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c")
	got := readRESPInteger(t, r)
	if got != 3 {
		t.Fatalf("RPUSH multiple to new list: expected 3, got %d", got)
	}
}

func TestRPushMultipleElementsExistingList_Stage03(t *testing.T) {
	requireListStage(t, 3)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c")
	got := readRESPInteger(t, r)
	if got != 3 {
		t.Fatalf("first RPUSH: expected 3, got %d", got)
	}

	writeRESPArray(t, conn, "RPUSH", "mylist", "d", "e")
	got = readRESPInteger(t, r)
	if got != 5 {
		t.Fatalf("second RPUSH: expected 5, got %d", got)
	}
}

// --- Stage 4: LRANGE with positive indexes ---

func TestLRangeBasicSubset_Stage04(t *testing.T) {
	requireListStage(t, 4)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c", "d", "e")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "0", "1")
	elems := listReadRESPArray(t, r)
	expected := []string{"a", "b"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE 0 1: expected %v, got %v", expected, elems)
	}
}

func TestLRangeMiddleSubset_Stage04(t *testing.T) {
	requireListStage(t, 4)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c", "d", "e")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "2", "4")
	elems := listReadRESPArray(t, r)
	expected := []string{"c", "d", "e"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE 2 4: expected %v, got %v", expected, elems)
	}
}

func TestLRangeStopBeyondLength_Stage04(t *testing.T) {
	requireListStage(t, 4)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "0", "100")
	elems := listReadRESPArray(t, r)
	expected := []string{"a", "b", "c"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE 0 100: expected %v, got %v", expected, elems)
	}
}

func TestLRangeStartBeyondLength_Stage04(t *testing.T) {
	requireListStage(t, 4)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "10", "20")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("LRANGE start beyond length: expected empty, got %v", elems)
	}
}

func TestLRangeStartGreaterThanStop_Stage04(t *testing.T) {
	requireListStage(t, 4)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "3", "1")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("LRANGE start > stop: expected empty, got %v", elems)
	}
}

func TestLRangeNonExistentList_Stage04(t *testing.T) {
	requireListStage(t, 4)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "LRANGE", "nolist", "0", "10")
	elems := listReadRESPArray(t, r)
	if len(elems) != 0 {
		t.Fatalf("LRANGE non-existent: expected empty, got %v", elems)
	}
}

// --- Stage 5: LRANGE with negative indexes ---

func TestLRangeNegativeEnd_Stage05(t *testing.T) {
	requireListStage(t, 5)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c", "d", "e")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "-2", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"d", "e"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE -2 -1: expected %v, got %v", expected, elems)
	}
}

func TestLRangePositiveStartNegativeEnd_Stage05(t *testing.T) {
	requireListStage(t, 5)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c", "d", "e")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "0", "-3")
	elems := listReadRESPArray(t, r)
	expected := []string{"a", "b", "c"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE 0 -3: expected %v, got %v", expected, elems)
	}
}

func TestLRangeMixedStartNegativeEnd_Stage05(t *testing.T) {
	requireListStage(t, 5)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c", "d", "e")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "2", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"c", "d", "e"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE 2 -1: expected %v, got %v", expected, elems)
	}
}

func TestLRangeNegativeOutOfRange_Stage05(t *testing.T) {
	requireListStage(t, 5)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c", "d", "e")
	_ = readRESPInteger(t, r)

	// -10 is out of range for a 5-element list; should be treated as 0
	writeRESPArray(t, conn, "LRANGE", "mylist", "-10", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"a", "b", "c", "d", "e"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE -10 -1: expected %v, got %v", expected, elems)
	}
}

func TestLRangeAllElementsWithNegativeIndex_Stage05(t *testing.T) {
	requireListStage(t, 5)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "x", "y", "z")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "0", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"x", "y", "z"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE 0 -1: expected %v, got %v", expected, elems)
	}
}

// --- Stage 6: LPUSH ---

func TestLPushSingleElement_Stage06(t *testing.T) {
	requireListStage(t, 6)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "LPUSH", "mylist", "a")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("LPUSH single: expected 1, got %d", got)
	}
}

func TestLPushMultipleElements_Stage06(t *testing.T) {
	requireListStage(t, 6)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "LPUSH", "mylist", "c")
	got := readRESPInteger(t, r)
	if got != 1 {
		t.Fatalf("LPUSH first: expected 1, got %d", got)
	}

	writeRESPArray(t, conn, "LPUSH", "mylist", "b", "a")
	got = readRESPInteger(t, r)
	if got != 3 {
		t.Fatalf("LPUSH multi: expected 3, got %d", got)
	}

	// Verify order: elements are prepended in reverse, so list should be [a, b, c]
	writeRESPArray(t, conn, "LRANGE", "mylist", "0", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"a", "b", "c"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LPUSH order: expected %v, got %v", expected, elems)
	}
}

func TestLPushCreatesNewList_Stage06(t *testing.T) {
	requireListStage(t, 6)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "LPUSH", "newlist", "x", "y", "z")
	got := readRESPInteger(t, r)
	if got != 3 {
		t.Fatalf("LPUSH new list: expected 3, got %d", got)
	}

	// Order: z was pushed last (prepended last), so list is [z, y, x]
	writeRESPArray(t, conn, "LRANGE", "newlist", "0", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"z", "y", "x"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LPUSH new list order: expected %v, got %v", expected, elems)
	}
}

// --- Stage 7: LLEN ---

func TestLLenExistingList_Stage07(t *testing.T) {
	requireListStage(t, 7)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c", "d")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LLEN", "mylist")
	got := readRESPInteger(t, r)
	if got != 4 {
		t.Fatalf("LLEN: expected 4, got %d", got)
	}
}

func TestLLenNonExistentList_Stage07(t *testing.T) {
	requireListStage(t, 7)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "LLEN", "missing_key")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("LLEN non-existent: expected 0, got %d", got)
	}
}

func TestLLenAfterMultiplePushes_Stage07(t *testing.T) {
	requireListStage(t, 7)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPUSH", "mylist", "x", "y")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LLEN", "mylist")
	got := readRESPInteger(t, r)
	if got != 4 {
		t.Fatalf("LLEN after mixed pushes: expected 4, got %d", got)
	}
}

// --- Stage 8: LPOP single element ---

func TestLPopSingleElement_Stage08(t *testing.T) {
	requireListStage(t, 8)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "one", "two", "three", "four", "five")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPOP", "mylist")
	got := readRESPBulkStringValue(t, r)
	if got != "one" {
		t.Fatalf("LPOP: expected 'one', got %q", got)
	}
}

func TestLPopVerifiesRemainingElements_Stage08(t *testing.T) {
	requireListStage(t, 8)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "one", "two", "three", "four", "five")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPOP", "mylist")
	_ = readBulkString(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "0", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"two", "three", "four", "five"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE after LPOP: expected %v, got %v", expected, elems)
	}
}

func TestLPopEmptyList_Stage08(t *testing.T) {
	requireListStage(t, 8)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "LPOP", "nonexistent")
	raw := readBulkString(t, r)
	if raw != "$-1\r\n" {
		t.Fatalf("LPOP non-existent: expected null bulk string, got %q", raw)
	}
}

func TestLPopMultipleCallsDrainsList_Stage08(t *testing.T) {
	requireListStage(t, 8)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPOP", "mylist")
	got := readRESPBulkStringValue(t, r)
	if got != "a" {
		t.Fatalf("first LPOP: expected 'a', got %q", got)
	}

	writeRESPArray(t, conn, "LPOP", "mylist")
	got = readRESPBulkStringValue(t, r)
	if got != "b" {
		t.Fatalf("second LPOP: expected 'b', got %q", got)
	}

	writeRESPArray(t, conn, "LPOP", "mylist")
	got = readRESPBulkStringValue(t, r)
	if got != "c" {
		t.Fatalf("third LPOP: expected 'c', got %q", got)
	}

	writeRESPArray(t, conn, "LPOP", "mylist")
	raw := readBulkString(t, r)
	if raw != "$-1\r\n" {
		t.Fatalf("LPOP empty: expected null bulk string, got %q", raw)
	}
}

// --- Stage 9: LPOP with count argument ---

func TestLPopWithCount_Stage09(t *testing.T) {
	requireListStage(t, 9)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "one", "two", "three", "four", "five")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPOP", "mylist", "2")
	elems := listReadRESPArray(t, r)
	expected := []string{"one", "two"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LPOP 2: expected %v, got %v", expected, elems)
	}
}

func TestLPopWithCountVerifiesRemaining_Stage09(t *testing.T) {
	requireListStage(t, 9)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "one", "two", "three", "four", "five")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPOP", "mylist", "2")
	_ = listReadRESPArray(t, r)

	writeRESPArray(t, conn, "LRANGE", "mylist", "0", "-1")
	elems := listReadRESPArray(t, r)
	expected := []string{"three", "four", "five"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LRANGE after LPOP 2: expected %v, got %v", expected, elems)
	}
}

func TestLPopCountExceedsLength_Stage09(t *testing.T) {
	requireListStage(t, 9)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "a", "b", "c")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPOP", "mylist", "10")
	elems := listReadRESPArray(t, r)
	expected := []string{"a", "b", "c"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LPOP count > length: expected %v, got %v", expected, elems)
	}
}

func TestLPopCountAll_Stage09(t *testing.T) {
	requireListStage(t, 9)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "RPUSH", "mylist", "x", "y", "z")
	_ = readRESPInteger(t, r)

	writeRESPArray(t, conn, "LPOP", "mylist", "3")
	elems := listReadRESPArray(t, r)
	expected := []string{"x", "y", "z"}
	if !sliceEqual(elems, expected) {
		t.Fatalf("LPOP all: expected %v, got %v", expected, elems)
	}

	// List should now be empty
	writeRESPArray(t, conn, "LLEN", "mylist")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("LLEN after full LPOP: expected 0, got %d", got)
	}
}

// --- Stage 10: BLPOP with timeout 0 (blocks indefinitely) ---

func TestBLPopBlocksAndReceivesElement_Stage10(t *testing.T) {
	requireListStage(t, 10)
	sp := startTinyRed(t)

	// Client 1: sends BLPOP and blocks
	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "BLPOP", "blocklist", "0")

	// Give the server time to register the blocking client
	time.Sleep(100 * time.Millisecond)

	// Client 2: pushes an element to unblock client 1
	conn2, r2 := dialClient(t, sp)
	writeRESPArray(t, conn2, "RPUSH", "blocklist", "hello")
	pushResult := readRESPInteger(t, r2)
	if pushResult != 0 && pushResult != 1 {
		// RPUSH may return 0 if element was consumed immediately, or 1 if queued
		// Either is acceptable depending on implementation
	}
	_ = pushResult

	// Client 1 should receive the response
	_ = conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	elems := listReadRESPArray(t, r1)
	if len(elems) != 2 {
		t.Fatalf("BLPOP response: expected 2-element array, got %v", elems)
	}
	if elems[0] != "blocklist" {
		t.Fatalf("BLPOP key: expected 'blocklist', got %q", elems[0])
	}
	if elems[1] != "hello" {
		t.Fatalf("BLPOP value: expected 'hello', got %q", elems[1])
	}
}

func TestBLPopServesLongestWaitingClient_Stage10(t *testing.T) {
	requireListStage(t, 10)
	sp := startTinyRed(t)

	// Client 1 blocks first
	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "BLPOP", "sharedlist", "0")
	time.Sleep(50 * time.Millisecond)

	// Client 2 blocks second
	conn2, _ := dialClient(t, sp)
	writeRESPArray(t, conn2, "BLPOP", "sharedlist", "0")
	time.Sleep(50 * time.Millisecond)

	// Client 3 pushes one element
	conn3, r3 := dialClient(t, sp)
	writeRESPArray(t, conn3, "RPUSH", "sharedlist", "first")
	_ = readLine(t, r3) // consume RPUSH response

	// Client 1 should get the element (it was waiting longer)
	_ = conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	elems := listReadRESPArray(t, r1)
	if len(elems) != 2 {
		t.Fatalf("BLPOP first client: expected 2-element array, got %v", elems)
	}
	if elems[0] != "sharedlist" {
		t.Fatalf("BLPOP key: expected 'sharedlist', got %q", elems[0])
	}
	if elems[1] != "first" {
		t.Fatalf("BLPOP value: expected 'first', got %q", elems[1])
	}
}

func TestBLPopExistingListReturnsImmediately_Stage10(t *testing.T) {
	requireListStage(t, 10)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	// Pre-populate the list
	writeRESPArray(t, conn, "RPUSH", "ready", "existing_value")
	_ = readRESPInteger(t, r)

	// BLPOP on a non-empty list should return immediately
	writeRESPArray(t, conn, "BLPOP", "ready", "0")
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	elems := listReadRESPArray(t, r)
	if len(elems) != 2 {
		t.Fatalf("BLPOP existing: expected 2-element array, got %v", elems)
	}
	if elems[0] != "ready" {
		t.Fatalf("BLPOP key: expected 'ready', got %q", elems[0])
	}
	if elems[1] != "existing_value" {
		t.Fatalf("BLPOP value: expected 'existing_value', got %q", elems[1])
	}
}

// --- Stage 11: BLPOP with non-zero timeout ---

func TestBLPopTimeoutExpires_Stage11(t *testing.T) {
	requireListStage(t, 11)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	start := time.Now()
	writeRESPArray(t, conn, "BLPOP", "emptylist", "0.5")
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Should receive null array after timeout
	header := readLine(t, r)
	elapsed := time.Since(start)

	if header != "*-1\r\n" {
		t.Fatalf("BLPOP timeout: expected null array (*-1\\r\\n), got %q", header)
	}
	if elapsed < 400*time.Millisecond {
		t.Fatalf("BLPOP returned too quickly: %v (expected ~500ms)", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("BLPOP took too long: %v (expected ~500ms)", elapsed)
	}
}

func TestBLPopTimeoutElementArrivesBeforeDeadline_Stage11(t *testing.T) {
	requireListStage(t, 11)
	sp := startTinyRed(t)

	// Client 1: blocks with a 2-second timeout
	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "BLPOP", "timedlist", "2")

	// Wait briefly, then push an element
	time.Sleep(100 * time.Millisecond)
	conn2, r2 := dialClient(t, sp)
	writeRESPArray(t, conn2, "RPUSH", "timedlist", "arrived")
	_ = readLine(t, r2)

	// Client 1 should receive the element before timeout
	_ = conn1.SetReadDeadline(time.Now().Add(3 * time.Second))
	elems := listReadRESPArray(t, r1)
	if len(elems) != 2 {
		t.Fatalf("BLPOP before timeout: expected 2-element array, got %v", elems)
	}
	if elems[0] != "timedlist" {
		t.Fatalf("BLPOP key: expected 'timedlist', got %q", elems[0])
	}
	if elems[1] != "arrived" {
		t.Fatalf("BLPOP value: expected 'arrived', got %q", elems[1])
	}
}

func TestBLPopShortTimeout_Stage11(t *testing.T) {
	requireListStage(t, 11)
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	start := time.Now()
	writeRESPArray(t, conn, "BLPOP", "nope", "0.1")
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	header := readLine(t, r)
	elapsed := time.Since(start)

	if header != "*-1\r\n" {
		t.Fatalf("BLPOP short timeout: expected null array, got %q", header)
	}
	if elapsed < 80*time.Millisecond {
		t.Fatalf("BLPOP returned too quickly: %v", elapsed)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("BLPOP took too long: %v", elapsed)
	}
}

// --- Helpers ---

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
