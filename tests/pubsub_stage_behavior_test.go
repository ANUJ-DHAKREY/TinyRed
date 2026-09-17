package tests

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func requirePubSubStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phasePubSub)
}

// readPubSubArrayHeader reads a RESP array header line (e.g. "*3\r\n") and returns
// the declared element count.
func readPubSubArrayHeader(t *testing.T, r *bufio.Reader) int {
	t.Helper()
	header := readLine(t, r)
	if !strings.HasPrefix(header, "*") {
		t.Fatalf("expected RESP array header, got %q", header)
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil {
		t.Fatalf("invalid RESP array length header %q: %v", header, err)
	}
	return n
}

// readPubSubElement reads a single RESP element of mixed type (bulk string or
// integer), as found inside pub/sub reply arrays, and returns its value as a string.
func readPubSubElement(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	peek, err := r.Peek(1)
	if err != nil {
		t.Fatalf("failed to peek pub/sub element type: %v", err)
	}
	switch peek[0] {
	case '$':
		return readRESPBulkStringValue(t, r)
	case ':':
		return strconv.Itoa(readRESPInteger(t, r))
	default:
		t.Fatalf("unexpected pub/sub element type byte %q", string(peek[0]))
		return ""
	}
}

// readPubSubArray reads a full RESP array with mixed-type elements (as produced
// by SUBSCRIBE/UNSUBSCRIBE confirmations and PUBLISH message pushes) and returns
// the element values as strings.
func readPubSubArray(t *testing.T, r *bufio.Reader) []string {
	t.Helper()
	n := readPubSubArrayHeader(t, r)
	elems := make([]string, n)
	for i := 0; i < n; i++ {
		elems[i] = readPubSubElement(t, r)
	}
	return elems
}

// readRawBytes reads exactly n raw bytes from the connection and returns them
// as a string, for asserting exact wire-format replies byte-for-byte.
func readRawBytes(t *testing.T, r *bufio.Reader, n int) string {
	t.Helper()
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("failed reading raw bytes: %v", err)
	}
	return string(buf)
}

// --- Stage 1: Subscribe to a Channel ---

func TestSubscribeToChannelReturnsConfirmation_Stage01Subscribe(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: client sends SUBSCRIBE for a single channel and receives an exact
	// three-element subscribe confirmation array: ["subscribe", "foo", 1].
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SUBSCRIBE", "foo")
	expected := "*3\r\n$9\r\nsubscribe\r\n$3\r\nfoo\r\n:1\r\n"
	got := readRawBytes(t, r, len(expected))
	if got != expected {
		t.Fatalf("SUBSCRIBE foo: expected %q, got %q", expected, got)
	}
}

// --- Stage 2: Subscribe to Multiple Channels ---

func TestSubscribeToMultipleChannelsIncrementsCount_Stage02MultipleChannels(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: subscribing to multiple channels on one connection increments the
	// per-client channel count; re-subscribing to an already-subscribed channel
	// does not increase the count further.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SUBSCRIBE", "foo")
	if got := readPubSubArray(t, r); !sliceEqual(got, []string{"subscribe", "foo", "1"}) {
		t.Fatalf("SUBSCRIBE foo: expected [subscribe foo 1], got %v", got)
	}

	writeRESPArray(t, conn, "SUBSCRIBE", "bar")
	if got := readPubSubArray(t, r); !sliceEqual(got, []string{"subscribe", "bar", "2"}) {
		t.Fatalf("SUBSCRIBE bar: expected [subscribe bar 2], got %v", got)
	}

	writeRESPArray(t, conn, "SUBSCRIBE", "bar")
	if got := readPubSubArray(t, r); !sliceEqual(got, []string{"subscribe", "bar", "2"}) {
		t.Fatalf("SUBSCRIBE bar (already subscribed): expected [subscribe bar 2], got %v", got)
	}
}

func TestSubscribeCountIsPerClientNotGlobal_Stage02MultipleChannels(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: a second, independent connection subscribing to "foo" must get its
	// own count of 1, rather than continuing some shared/global counter.
	sp := startTinyRed(t)

	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "SUBSCRIBE", "foo")
	if got := readPubSubArray(t, r1); !sliceEqual(got, []string{"subscribe", "foo", "1"}) {
		t.Fatalf("client1 SUBSCRIBE foo: expected [subscribe foo 1], got %v", got)
	}
	writeRESPArray(t, conn1, "SUBSCRIBE", "bar")
	if got := readPubSubArray(t, r1); !sliceEqual(got, []string{"subscribe", "bar", "2"}) {
		t.Fatalf("client1 SUBSCRIBE bar: expected [subscribe bar 2], got %v", got)
	}

	conn2, r2 := dialClient(t, sp)
	writeRESPArray(t, conn2, "SUBSCRIBE", "foo")
	if got := readPubSubArray(t, r2); !sliceEqual(got, []string{"subscribe", "foo", "1"}) {
		t.Fatalf("client2 SUBSCRIBE foo: expected independent count [subscribe foo 1], got %v", got)
	}
}

// --- Stage 3: Enter Subscribed Mode ---

func TestDisallowedCommandInSubscribedModeReturnsError_Stage03SubscribedMode(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: once a connection has entered subscribed mode via SUBSCRIBE,
	// sending a disallowed command such as ECHO must be rejected with a RESP
	// error naming the offending command and stating it isn't allowed.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SUBSCRIBE", "somechan")
	_ = readPubSubArray(t, r)

	writeRESPArray(t, conn, "ECHO", "hey")
	got := readLine(t, r)
	if !strings.HasPrefix(got, "-") {
		t.Fatalf("expected RESP error for ECHO in subscribed mode, got %q", got)
	}
	lower := strings.ToLower(got)
	if !strings.Contains(lower, "echo") {
		t.Fatalf("expected error text to mention 'echo', got %q", got)
	}
	if !strings.Contains(lower, "allowed") {
		t.Fatalf("expected error text to mention 'allowed', got %q", got)
	}
}

// --- Stage 4: PING in Subscribed Mode ---

func TestPingInSubscribedModeReturnsArrayResponse_Stage04PingSubscribed(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: PING sent on a connection that is in subscribed mode must return
	// a two-element RESP array ["pong", ""] instead of the plain simple string.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SUBSCRIBE", "somechan")
	_ = readPubSubArray(t, r)

	writeRESPArray(t, conn, "PING")
	expected := "*2\r\n$4\r\npong\r\n$0\r\n\r\n"
	got := readRawBytes(t, r, len(expected))
	if got != expected {
		t.Fatalf("PING in subscribed mode: expected %q, got %q", expected, got)
	}
}

func TestPingOnFreshConnectionStillReturnsSimplePong_Stage04PingSubscribed(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: regression check -- a connection that has never subscribed must
	// still receive the plain +PONG simple string reply for PING.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "PING")
	if got := readLine(t, r); got != "+PONG\r\n" {
		t.Fatalf("expected +PONG on non-subscribed connection, got %q", got)
	}
}

// --- Stage 5: Publish a Message ---

func TestPublishReturnsSubscriberCount_Stage05Publish(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: PUBLISH returns an integer equal to the number of clients
	// currently subscribed to the target channel.
	sp := startTinyRed(t)

	connA, rA := dialClient(t, sp)
	writeRESPArray(t, connA, "SUBSCRIBE", "bar")
	_ = readPubSubArray(t, rA)

	connB, rB := dialClient(t, sp)
	writeRESPArray(t, connB, "SUBSCRIBE", "bar")
	_ = readPubSubArray(t, rB)

	connC, rC := dialClient(t, sp)
	writeRESPArray(t, connC, "PUBLISH", "bar", "somemsg")
	got := readRESPInteger(t, rC)
	if got != 2 {
		t.Fatalf("PUBLISH bar: expected 2 subscribers, got %d", got)
	}
}

func TestPublishToChannelWithNoSubscribersReturnsZero_Stage05Publish(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: publishing to a channel with no subscribers returns zero.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "PUBLISH", "nosuchchannel", "msg")
	got := readRESPInteger(t, r)
	if got != 0 {
		t.Fatalf("PUBLISH nosuchchannel: expected 0, got %d", got)
	}
}

// --- Stage 6: Deliver Messages ---

func TestPublishDeliversMessageToSubscriber_Stage06DeliverMessages(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: a published message is pushed, unsolicited, only to connections
	// subscribed to that exact channel; a connection subscribed to a different
	// channel must receive nothing for that publish.
	sp := startTinyRed(t)

	connA, rA := dialClient(t, sp)
	writeRESPArray(t, connA, "SUBSCRIBE", "channel_1")
	_ = readPubSubArray(t, rA)

	connB, rB := dialClient(t, sp)
	writeRESPArray(t, connB, "SUBSCRIBE", "channel_2")
	_ = readPubSubArray(t, rB)

	connC, rC := dialClient(t, sp)
	writeRESPArray(t, connC, "PUBLISH", "channel_1", "hello")
	if got := readRESPInteger(t, rC); got != 1 {
		t.Fatalf("PUBLISH channel_1: expected 1 subscriber, got %d", got)
	}

	// connA (subscribed to channel_1) should receive the pushed message.
	push := readPubSubArray(t, rA)
	expected := []string{"message", "channel_1", "hello"}
	if !sliceEqual(push, expected) {
		t.Fatalf("connA message push: expected %v, got %v", expected, push)
	}

	// connB (subscribed to a different channel) must not receive anything.
	_ = connB.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, err := rB.ReadByte()
	if err == nil {
		t.Fatalf("connB unexpectedly received data for a channel it is not subscribed to")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("expected read timeout on connB, got %v", err)
	}
}

// --- Stage 7: Unsubscribe ---

func TestUnsubscribeRemovesChannelSubscription_Stage07Unsubscribe(t *testing.T) {
	requirePubSubStage(t)
	// Scenario: UNSUBSCRIBE removes a channel from a client's subscriptions.
	// PUBLISH afterward must reflect the reduced subscriber count for the
	// unsubscribed channel, and the unsubscribing client must no longer receive
	// pushes for it, while still receiving pushes for channels it remains
	// subscribed to.
	sp := startTinyRed(t)

	conn1, r1 := dialClient(t, sp)
	writeRESPArray(t, conn1, "SUBSCRIBE", "foo")
	if got := readPubSubArray(t, r1); !sliceEqual(got, []string{"subscribe", "foo", "1"}) {
		t.Fatalf("SUBSCRIBE foo: expected [subscribe foo 1], got %v", got)
	}
	writeRESPArray(t, conn1, "SUBSCRIBE", "bar")
	if got := readPubSubArray(t, r1); !sliceEqual(got, []string{"subscribe", "bar", "2"}) {
		t.Fatalf("SUBSCRIBE bar: expected [subscribe bar 2], got %v", got)
	}
	writeRESPArray(t, conn1, "UNSUBSCRIBE", "foo")
	if got := readPubSubArray(t, r1); !sliceEqual(got, []string{"unsubscribe", "foo", "1"}) {
		t.Fatalf("UNSUBSCRIBE foo: expected [unsubscribe foo 1], got %v", got)
	}

	conn2, r2 := dialClient(t, sp)

	// Publishing to "foo" should now have zero subscribers, and conn1 must not
	// receive any push for it.
	writeRESPArray(t, conn2, "PUBLISH", "foo", "msg1")
	if got := readRESPInteger(t, r2); got != 0 {
		t.Fatalf("PUBLISH foo after unsubscribe: expected 0 subscribers, got %d", got)
	}
	_ = conn1.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, err := r1.ReadByte()
	if err == nil {
		t.Fatalf("conn1 unexpectedly received a push for unsubscribed channel foo")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("expected read timeout on conn1 for foo publish, got %v", err)
	}
	_ = conn1.SetReadDeadline(time.Time{})

	// Publishing to "bar" (still subscribed) should still reach conn1.
	writeRESPArray(t, conn2, "PUBLISH", "bar", "msg2")
	if got := readRESPInteger(t, r2); got != 1 {
		t.Fatalf("PUBLISH bar: expected 1 subscriber, got %d", got)
	}
	push := readPubSubArray(t, r1)
	expected := []string{"message", "bar", "msg2"}
	if !sliceEqual(push, expected) {
		t.Fatalf("conn1 message push: expected %v, got %v", expected, push)
	}
}
