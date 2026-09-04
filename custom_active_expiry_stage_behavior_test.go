package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const defaultMaxCustomActiveExpiryStage = 0

func maxCustomActiveExpiryStage() int {
	raw := strings.TrimSpace(os.Getenv("TINYRED_CUSTOM_ACTIVE_EXPIRY_STAGE"))
	if raw == "" {
		return defaultMaxCustomActiveExpiryStage
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultMaxCustomActiveExpiryStage
	}
	if v > 1 {
		return 1
	}
	return v
}

func requireCustomActiveExpiryStage(t *testing.T, stage int) {
	t.Helper()
	if stage > maxCustomActiveExpiryStage() {
		t.Skipf("skipping custom active-expiry stage %d test; set TINYRED_CUSTOM_ACTIVE_EXPIRY_STAGE=%d (or higher) to run", stage, stage)
	}
}

// customActiveExpiryContains reports whether needle is present in haystack.
func customActiveExpiryContains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func TestExpiredKeyDisappearsFromKeysWithoutBeingRead_Stage01BackgroundSweep(t *testing.T) {
	requireCustomActiveExpiryStage(t, 1)
	// Scenario: a key set with a short TTL should be proactively removed by a background
	// sweep and vanish from KEYS * even though it is never read (GET is never called here) —
	// proving removal isn't just the pre-existing lazy expiry-on-access path.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	writeRESPArray(t, conn, "SET", "foo", "bar", "PX", "50")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET expected +OK, got %q", got)
	}

	writeRESPArray(t, conn, "KEYS", "*")
	before := listReadRESPArray(t, r)
	if !customActiveExpiryContains(before, "foo") {
		t.Fatalf("expected foo present in KEYS * immediately after SET, got %v", before)
	}

	time.Sleep(300 * time.Millisecond)

	writeRESPArray(t, conn, "KEYS", "*")
	after := listReadRESPArray(t, r)
	if customActiveExpiryContains(after, "foo") {
		t.Fatalf("expected foo to be actively swept from KEYS * after expiry, got %v", after)
	}
}

func TestActiveSweepOnlyRemovesExpiredKeysAndLeavesOthersIntact_Stage01SelectiveSweep(t *testing.T) {
	requireCustomActiveExpiryStage(t, 1)
	// Scenario: mix short-TTL keys with persistent (no expiry) and longer-TTL keys. After
	// waiting past the short TTLs, the sweep should have removed only the short-TTL keys,
	// leaving persistent and longer-TTL keys present in KEYS *.
	sp := startTinyRed(t)
	conn, r := dialClient(t, sp)

	shortLived := []string{"short1", "short2"}
	for _, k := range shortLived {
		writeRESPArray(t, conn, "SET", k, "v", "PX", "50")
		if got := readLine(t, r); got != "+OK\r\n" {
			t.Fatalf("SET %s expected +OK, got %q", k, got)
		}
	}

	writeRESPArray(t, conn, "SET", "persistent1", "val")
	if got := readLine(t, r); got != "+OK\r\n" {
		t.Fatalf("SET persistent1 expected +OK, got %q", got)
	}

	longLived := []string{"long1", "long2"}
	for _, k := range longLived {
		writeRESPArray(t, conn, "SET", k, "v", "PX", "5000")
		if got := readLine(t, r); got != "+OK\r\n" {
			t.Fatalf("SET %s expected +OK, got %q", k, got)
		}
	}

	time.Sleep(300 * time.Millisecond)

	writeRESPArray(t, conn, "KEYS", "*")
	after := listReadRESPArray(t, r)

	for _, k := range shortLived {
		if customActiveExpiryContains(after, k) {
			t.Fatalf("expected short-TTL key %q to be swept away, got %v", k, after)
		}
	}
	if !customActiveExpiryContains(after, "persistent1") {
		t.Fatalf("expected persistent1 (no expiry) to remain, got %v", after)
	}
	for _, k := range longLived {
		if !customActiveExpiryContains(after, k) {
			t.Fatalf("expected longer-TTL key %q to remain, got %v", k, after)
		}
	}
}
