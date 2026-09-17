package tests

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func requireStreamsStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseStreams)
}

// streamsDecodeRESP recursively decodes a single RESP value. Simple strings,
// errors and integers are returned as their (unprefixed) string content. Bulk
// strings are returned as plain strings, with a null bulk string ($-1\r\n)
// represented as a nil any. Arrays are returned as []any, with a null array
// (*-1\r\n) also represented as a nil any. This is pragmatic, not a general
// RESP decoder: it only needs to handle what XADD/XRANGE/XREAD/TYPE produce.
func streamsDecodeRESP(t *testing.T, r *bufio.Reader) any {
	t.Helper()

	line := readLine(t, r)
	trimmed := strings.TrimRight(line, "\r\n")
	if len(trimmed) == 0 {
		t.Fatalf("streamsDecodeRESP: empty RESP line")
	}

	prefix := trimmed[0]
	content := trimmed[1:]

	switch prefix {
	case '+', '-', ':':
		return content
	case '$':
		n, err := strconv.Atoi(content)
		if err != nil {
			t.Fatalf("streamsDecodeRESP: invalid bulk-string length %q: %v", content, err)
		}
		if n < 0 {
			return nil
		}
		body := make([]byte, n+2)
		if _, err := io.ReadFull(r, body); err != nil {
			t.Fatalf("streamsDecodeRESP: failed reading bulk-string body: %v", err)
		}
		return string(body[:n])
	case '*':
		n, err := strconv.Atoi(content)
		if err != nil {
			t.Fatalf("streamsDecodeRESP: invalid array length %q: %v", content, err)
		}
		if n < 0 {
			return nil
		}
		arr := make([]any, n)
		for i := 0; i < n; i++ {
			arr[i] = streamsDecodeRESP(t, r)
		}
		return arr
	default:
		t.Fatalf("streamsDecodeRESP: unexpected RESP type byte %q in line %q", prefix, line)
		return nil
	}
}

// streamsAsArray type-asserts a decoded RESP value into a []any, failing the
// test with a helpful message if the shape doesn't match.
func streamsAsArray(t *testing.T, v any) []any {
	t.Helper()
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("expected RESP array, got %T (%v)", v, v)
	}
	return arr
}

// streamsAsString type-asserts a decoded RESP value into a string.
func streamsAsString(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected RESP string, got %T (%v)", v, v)
	}
	return s
}

// streamsFieldsAsStrings converts a decoded flat field/value array into a []string.
func streamsFieldsAsStrings(t *testing.T, v any) []string {
	t.Helper()
	arr := streamsAsArray(t, v)
	out := make([]string, len(arr))
	for i, e := range arr {
		out[i] = streamsAsString(t, e)
	}
	return out
}

// streamsStringSlicesEqual compares two string slices for equality.
func streamsStringSlicesEqual(a, b []string) bool {
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

// streamsExpectedEntry describes one expected stream entry: an ID plus its
// flat field/value pairs, in the order they were added.
type streamsExpectedEntry struct {
	id     string
	fields []string
}

// streamsAssertEntry asserts that a decoded entry (a 2-element array of
// [id, [field, value, ...]]) matches the expected id and fields.
func streamsAssertEntry(t *testing.T, entry any, wantID string, wantFields []string) {
	t.Helper()
	e := streamsAsArray(t, entry)
	if len(e) != 2 {
		t.Fatalf("expected entry array of length 2, got %d: %v", len(e), e)
	}
	gotID := streamsAsString(t, e[0])
	if gotID != wantID {
		t.Fatalf("entry id: expected %q, got %q", wantID, gotID)
	}
	gotFields := streamsFieldsAsStrings(t, e[1])
	if !streamsStringSlicesEqual(gotFields, wantFields) {
		t.Fatalf("entry fields for id %s: expected %v, got %v", wantID, wantFields, gotFields)
	}
}

// streamsAssertEntries asserts that a decoded XRANGE-style response (an
// array of entries) matches the expected list of entries, in order.
func streamsAssertEntries(t *testing.T, v any, want []streamsExpectedEntry) {
	t.Helper()
	arr := streamsAsArray(t, v)
	if len(arr) != len(want) {
		t.Fatalf("expected %d entries, got %d: %v", len(want), len(arr), arr)
	}
	for i, e := range want {
		streamsAssertEntry(t, arr[i], e.id, e.fields)
	}
}

// streamsAssertStream asserts that a decoded XREAD-style stream element (a
// 2-element array of [key, [entry, ...]]) matches the expected key and entries.
func streamsAssertStream(t *testing.T, streamVal any, wantKey string, want []streamsExpectedEntry) {
	t.Helper()
	s := streamsAsArray(t, streamVal)
	if len(s) != 2 {
		t.Fatalf("expected stream array of length 2, got %d: %v", len(s), s)
	}
	gotKey := streamsAsString(t, s[0])
	if gotKey != wantKey {
		t.Fatalf("stream key: expected %q, got %q", wantKey, gotKey)
	}
	entries := streamsAsArray(t, s[1])
	if len(entries) != len(want) {
		t.Fatalf("stream %s: expected %d entries, got %d: %v", wantKey, len(want), len(entries), entries)
	}
	for i, e := range want {
		streamsAssertEntry(t, entries[i], e.id, e.fields)
	}
}

// streamsReadErrorLine reads a RESP simple-error line (e.g.
// "-ERR something\r\n") and returns its message content, without the leading
// '-' or trailing line terminator. Fails the test if the line isn't an error.
func streamsReadErrorLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line := readLine(t, r)
	if !strings.HasPrefix(line, "-") {
		t.Fatalf("expected RESP error, got %q", line)
	}
	return strings.TrimRight(strings.TrimPrefix(line, "-"), "\r\n")
}

// streamsBuildRESPArray renders a RESP array-of-bulk-strings command, matching
// the wire format produced by writeRESPArray.
func streamsBuildRESPArray(parts ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(parts))
	for _, p := range parts {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(p), p)
	}
	return b.String()
}

// streamsFireCommand opens a fresh connection to the server, sends a single
// command, and closes the connection shortly after. It deliberately avoids
// taking a *testing.T so it is safe to call from a background goroutine
// (testing.T's Fatal family may only be called from the test's own
// goroutine); callers should propagate the returned error back to the main
// test goroutine (e.g. via a channel) and fail there instead.
func streamsFireCommand(sp *serverProc, parts ...string) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", sp.port), 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := io.WriteString(conn, streamsBuildRESPArray(parts...)); err != nil {
		return err
	}

	// Give the server a moment to process and reply before we tear the
	// connection down, without needing to fully parse the response here.
	r := bufio.NewReader(conn)
	_, _ = r.ReadString('\n')
	return nil
}

// --- Stage 1 (doc 71): TYPE command ---

func TestTypeReturnsStringForExistingStringKey_Stage01TypeCommand(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: SET creates a string key; TYPE reports "string" as a RESP simple string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "some_key", "foo")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "TYPE", "some_key")
	if got := readLine(t, r); got != "+string\r\n" {
		t.Fatalf("TYPE some_key: expected +string, got %q", got)
	}
}

func TestTypeReturnsNoneForMissingKey_Stage01TypeCommand(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: TYPE on a key that was never set reports "none".
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "TYPE", "missing_key")
	if got := readLine(t, r); got != "+none\r\n" {
		t.Fatalf("TYPE missing_key: expected +none, got %q", got)
	}
}

// --- Stage 2 (doc 72): Create a stream via XADD ---

func TestXaddCreatesStreamAndTypeReportsStream_Stage02CreateStream(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XADD on a new key creates a stream and returns the assigned ID;
	// TYPE on that key subsequently reports "stream".
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "0-1", "foo", "bar")
	if got := readRESPBulkStringValue(t, r); got != "0-1" {
		t.Fatalf("XADD stream_key 0-1: expected id 0-1, got %q", got)
	}

	writeRESPArray(t, conn, "TYPE", "stream_key")
	if got := readLine(t, r); got != "+stream\r\n" {
		t.Fatalf("TYPE stream_key: expected +stream, got %q", got)
	}
}

// --- Stage 3 (doc 73): Validating entry IDs ---

func TestXaddValidatesEntryIds_Stage03ValidateEntryIds(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XADD rejects explicit IDs that are not strictly greater than the
	// stream's current top ID, and rejects 0-0 outright.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "1-1", "foo", "bar")
	if got := readRESPBulkStringValue(t, r); got != "1-1" {
		t.Fatalf("XADD 1-1: expected 1-1, got %q", got)
	}

	writeRESPArray(t, conn, "XADD", "stream_key", "1-2", "bar", "baz")
	if got := readRESPBulkStringValue(t, r); got != "1-2" {
		t.Fatalf("XADD 1-2: expected 1-2, got %q", got)
	}

	writeRESPArray(t, conn, "XADD", "stream_key", "1-2", "baz", "foo")
	if got := streamsReadErrorLine(t, r); !strings.Contains(got, "equal or smaller than the target stream top item") {
		t.Fatalf("XADD 1-2 (duplicate id): expected 'equal or smaller' error, got %q", got)
	}

	writeRESPArray(t, conn, "XADD", "stream_key", "0-3", "baz", "foo")
	if got := streamsReadErrorLine(t, r); !strings.Contains(got, "equal or smaller than the target stream top item") {
		t.Fatalf("XADD 0-3 (smaller time part): expected 'equal or smaller' error, got %q", got)
	}

	writeRESPArray(t, conn, "XADD", "stream_key", "0-0", "baz", "foo")
	if got := streamsReadErrorLine(t, r); !strings.Contains(got, "must be greater than 0-0") {
		t.Fatalf("XADD 0-0: expected 'must be greater than 0-0' error, got %q", got)
	}
}

// --- Stage 4 (doc 74): Partially auto-generated IDs (<ms>-*) ---

func TestXaddPartiallyAutoGeneratedSequenceNumber_Stage04PartialAutoId(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XADD with "<ms>-*" auto-generates the sequence number: it starts
	// at 1 when the time part is literally 0, otherwise starts at 0 and increments.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "0-*", "foo", "bar")
	if got := readRESPBulkStringValue(t, r); got != "0-1" {
		t.Fatalf("XADD 0-*: expected 0-1, got %q", got)
	}

	writeRESPArray(t, conn, "XADD", "stream_key", "5-*", "foo", "bar")
	if got := readRESPBulkStringValue(t, r); got != "5-0" {
		t.Fatalf("XADD 5-* (first at time 5): expected 5-0, got %q", got)
	}

	writeRESPArray(t, conn, "XADD", "stream_key", "5-*", "bar", "baz")
	if got := readRESPBulkStringValue(t, r); got != "5-1" {
		t.Fatalf("XADD 5-* (second at time 5): expected 5-1, got %q", got)
	}
}

// --- Stage 5 (doc 75): Fully auto-generated IDs (*) ---

func TestXaddFullyAutoGeneratedId_Stage05FullAutoId(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XADD with "*" auto-generates both the time part (current unix ms)
	// and the sequence number.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	before := time.Now().UnixMilli()
	writeRESPArray(t, conn, "XADD", "stream_key", "*", "foo", "bar")
	got := readRESPBulkStringValue(t, r)
	after := time.Now().UnixMilli()

	if !regexp.MustCompile(`^\d+-\d+$`).MatchString(got) {
		t.Fatalf("XADD *: expected id matching ^\\d+-\\d+$, got %q", got)
	}

	msPart := strings.SplitN(got, "-", 2)[0]
	ms, err := strconv.ParseInt(msPart, 10, 64)
	if err != nil {
		t.Fatalf("XADD *: failed to parse ms part of id %q: %v", got, err)
	}

	const slackMillis = 5000
	if ms < before-slackMillis || ms > after+slackMillis {
		t.Fatalf("XADD *: id ms part %d not within sane range of now (before=%d after=%d)", ms, before, after)
	}
}

// --- Stage 6 (doc 76): XRANGE with both bounds explicit ---

func TestXrangeWithExplicitBounds_Stage06XrangeExplicitBounds(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XRANGE with two explicit IDs returns the inclusive range of entries.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "0-1", "foo", "bar")
	_ = readRESPBulkStringValue(t, r)
	writeRESPArray(t, conn, "XADD", "stream_key", "0-2", "bar", "baz")
	_ = readRESPBulkStringValue(t, r)
	writeRESPArray(t, conn, "XADD", "stream_key", "0-3", "baz", "foo")
	_ = readRESPBulkStringValue(t, r)

	writeRESPArray(t, conn, "XRANGE", "stream_key", "0-2", "0-3")
	got := streamsDecodeRESP(t, r)
	streamsAssertEntries(t, got, []streamsExpectedEntry{
		{"0-2", []string{"bar", "baz"}},
		{"0-3", []string{"baz", "foo"}},
	})
}

// --- Stage 7 (doc 77): XRANGE with "-" as start ---

func TestXrangeWithDashAsStart_Stage07XrangeDashStart(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XRANGE with "-" as the start ID returns entries from the very
	// beginning of the stream through the given end ID.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "0-1", "foo", "bar")
	_ = readRESPBulkStringValue(t, r)
	writeRESPArray(t, conn, "XADD", "stream_key", "0-2", "bar", "baz")
	_ = readRESPBulkStringValue(t, r)
	writeRESPArray(t, conn, "XADD", "stream_key", "0-3", "baz", "foo")
	_ = readRESPBulkStringValue(t, r)

	writeRESPArray(t, conn, "XRANGE", "stream_key", "-", "0-2")
	got := streamsDecodeRESP(t, r)
	streamsAssertEntries(t, got, []streamsExpectedEntry{
		{"0-1", []string{"foo", "bar"}},
		{"0-2", []string{"bar", "baz"}},
	})
}

// --- Stage 8 (doc 78): XRANGE with "+" as end ---

func TestXrangeWithPlusAsEnd_Stage08XrangePlusEnd(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XRANGE with "+" as the end ID returns entries from the given
	// start ID through the very end of the stream.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "0-1", "foo", "bar")
	_ = readRESPBulkStringValue(t, r)
	writeRESPArray(t, conn, "XADD", "stream_key", "0-2", "bar", "baz")
	_ = readRESPBulkStringValue(t, r)
	writeRESPArray(t, conn, "XADD", "stream_key", "0-3", "baz", "foo")
	_ = readRESPBulkStringValue(t, r)

	writeRESPArray(t, conn, "XRANGE", "stream_key", "0-2", "+")
	got := streamsDecodeRESP(t, r)
	streamsAssertEntries(t, got, []streamsExpectedEntry{
		{"0-2", []string{"bar", "baz"}},
		{"0-3", []string{"baz", "foo"}},
	})
}

// --- Stage 9 (doc 79): XREAD single stream ---

func TestXreadSingleStream_Stage09XreadSingleStream(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XREAD STREAMS <key> <id> returns entries newer than <id> for one stream.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "0-1", "temperature", "96")
	_ = readRESPBulkStringValue(t, r)

	writeRESPArray(t, conn, "XREAD", "STREAMS", "stream_key", "0-0")
	got := streamsDecodeRESP(t, r)
	streams := streamsAsArray(t, got)
	if len(streams) != 1 {
		t.Fatalf("XREAD single stream: expected 1 stream in response, got %d: %v", len(streams), streams)
	}
	streamsAssertStream(t, streams[0], "stream_key", []streamsExpectedEntry{
		{"0-1", []string{"temperature", "96"}},
	})
}

// --- Stage 10 (doc 80): XREAD multiple streams ---

func TestXreadMultipleStreams_Stage10XreadMultipleStreams(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XREAD STREAMS <key1> <key2> <id1> <id2> returns results for each
	// stream, in the same order as requested.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "stream_key", "0-1", "temperature", "95")
	_ = readRESPBulkStringValue(t, r)
	writeRESPArray(t, conn, "XADD", "other_stream_key", "0-2", "humidity", "97")
	_ = readRESPBulkStringValue(t, r)

	writeRESPArray(t, conn, "XREAD", "STREAMS", "stream_key", "other_stream_key", "0-0", "0-1")
	got := streamsDecodeRESP(t, r)
	streams := streamsAsArray(t, got)
	if len(streams) != 2 {
		t.Fatalf("XREAD multiple streams: expected 2 streams in response, got %d: %v", len(streams), streams)
	}
	streamsAssertStream(t, streams[0], "stream_key", []streamsExpectedEntry{
		{"0-1", []string{"temperature", "95"}},
	})
	streamsAssertStream(t, streams[1], "other_stream_key", []streamsExpectedEntry{
		{"0-2", []string{"humidity", "97"}},
	})
}

// --- Stage 11 (doc 81): Blocking reads with timeout ---

func TestXreadBlockingWithTimeoutUnblocksWithNewEntry_Stage11XreadBlockTimeout(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: a blocking XREAD unblocks and returns the newly added entry when
	// another connection XADDs before the timeout expires.
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)

	writeRESPArray(t, conn1, "XADD", "stream_key", "0-1", "temperature", "96")
	_ = readRESPBulkStringValue(t, r1)

	_ = conn1.SetReadDeadline(time.Now().Add(5 * time.Second))
	writeRESPArray(t, conn1, "XREAD", "BLOCK", "1000", "streams", "stream_key", "0-1")

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		errCh <- streamsFireCommand(sp, "XADD", "stream_key", "0-2", "temperature", "95")
	}()

	got := streamsDecodeRESP(t, r1)
	if err := <-errCh; err != nil {
		t.Fatalf("background XADD failed: %v", err)
	}

	streams := streamsAsArray(t, got)
	if len(streams) != 1 {
		t.Fatalf("XREAD BLOCK unblock: expected 1 stream in response, got %d: %v", len(streams), streams)
	}
	streamsAssertStream(t, streams[0], "stream_key", []streamsExpectedEntry{
		{"0-2", []string{"temperature", "95"}},
	})
}

func TestXreadBlockingWithTimeoutExpiresReturnsNullArray_Stage11XreadBlockTimeout(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: a blocking XREAD returns a null array once its timeout elapses
	// with no new entry added.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "k", "0-1", "f", "v")
	_ = readRESPBulkStringValue(t, r)

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	writeRESPArray(t, conn, "XREAD", "BLOCK", "300", "streams", "k", "0-1")
	if got := readLine(t, r); got != "*-1\r\n" {
		t.Fatalf("XREAD BLOCK timeout: expected null array (*-1\\r\\n), got %q", got)
	}
}

// --- Stage 12 (doc 82): Blocking reads without timeout (BLOCK 0) ---

func TestXreadBlockingIndefinitelyUnblocksWithNewEntry_Stage12XreadBlockForever(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XREAD BLOCK 0 blocks indefinitely (doesn't spuriously return) and
	// unblocks only once a new entry is added, however long that takes.
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)

	writeRESPArray(t, conn1, "XADD", "stream_key", "0-1", "temperature", "96")
	_ = readRESPBulkStringValue(t, r1)

	// Safety net only: a correct implementation should not hit this deadline
	// since BLOCK 0 has no timeout.
	_ = conn1.SetReadDeadline(time.Now().Add(5 * time.Second))
	writeRESPArray(t, conn1, "XREAD", "BLOCK", "0", "streams", "stream_key", "0-1")

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		errCh <- streamsFireCommand(sp, "XADD", "stream_key", "0-2", "temperature", "95")
	}()

	got := streamsDecodeRESP(t, r1)
	if err := <-errCh; err != nil {
		t.Fatalf("background XADD failed: %v", err)
	}

	streams := streamsAsArray(t, got)
	if len(streams) != 1 {
		t.Fatalf("XREAD BLOCK 0: expected 1 stream in response, got %d: %v", len(streams), streams)
	}
	streamsAssertStream(t, streams[0], "stream_key", []streamsExpectedEntry{
		{"0-2", []string{"temperature", "95"}},
	})
}

// --- Stage 13 (doc 83): Blocking reads using "$" ---

func TestXreadBlockingWithDollarReturnsOnlyNewEntries_Stage13XreadBlockDollar(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XREAD BLOCK 0 STREAMS <key> $ only returns entries added after
	// the command was issued, not the pre-existing entry.
	sp := startTinyRed(t)
	conn1, r1 := dialClient(t, sp)

	writeRESPArray(t, conn1, "XADD", "stream_key", "0-1", "temperature", "96")
	_ = readRESPBulkStringValue(t, r1)

	_ = conn1.SetReadDeadline(time.Now().Add(5 * time.Second))
	writeRESPArray(t, conn1, "XREAD", "BLOCK", "0", "streams", "stream_key", "$")

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		errCh <- streamsFireCommand(sp, "XADD", "stream_key", "0-2", "temperature", "95")
	}()

	got := streamsDecodeRESP(t, r1)
	if err := <-errCh; err != nil {
		t.Fatalf("background XADD failed: %v", err)
	}

	streams := streamsAsArray(t, got)
	if len(streams) != 1 {
		t.Fatalf("XREAD BLOCK $ : expected 1 stream in response, got %d: %v", len(streams), streams)
	}
	streamsAssertStream(t, streams[0], "stream_key", []streamsExpectedEntry{
		{"0-2", []string{"temperature", "95"}},
	})
}

func TestXreadBlockingWithDollarAndTimeoutExpiresReturnsNullArray_Stage13XreadBlockDollar(t *testing.T) {
	requireStreamsStage(t)
	// Scenario: XREAD BLOCK <ms> STREAMS <key> $ returns a null array once the
	// timeout elapses with no new entry added after the command was issued.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "XADD", "k", "0-1", "f", "v")
	_ = readRESPBulkStringValue(t, r)

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	writeRESPArray(t, conn, "XREAD", "BLOCK", "300", "streams", "k", "$")
	if got := readLine(t, r); got != "*-1\r\n" {
		t.Fatalf("XREAD BLOCK $ timeout: expected null array (*-1\\r\\n), got %q", got)
	}
}
