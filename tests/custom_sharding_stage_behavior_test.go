package tests

// PHASE C6: SHARDED STORE (custom stage, not part of CodeCrafters).
//
// store.Store currently guards a single map[string]*resp.Entry with one
// sync.RWMutex. The eventual goal of this phase is to replace that with 64
// independently-locked shards (keyed by FNV-1a hash of the key, mod 64) to
// reduce lock contention under concurrent load. From the client's
// perspective this is a pure internal refactor: there is no new
// wire-protocol behavior to assert on. These tests exist purely to hammer
// the store with concurrent SET/GET traffic across many keys and confirm
// there are no lost writes and no torn/corrupted values once the sharded
// rewrite lands.
//
// These tests are best run with `go test -race` so that any shard-locking
// bugs (e.g. a shard boundary being computed inconsistently, or two shards
// aliasing the same lock) surface as data races. They are also expected to
// pass without -race, just less reliably at catching subtle bugs, since
// the -race flag itself is a go-test-level flag this file cannot force on.
//
// Both tests below are gated behind the stage flag and SKIP by default, so
// running `go test .` (or `go test -race .`) without
// TINYRED_CUSTOM_SHARDING_STAGE set completes quickly without exercising
// the concurrency logic.

import (
	"fmt"
	"sync"
	"testing"
)

func requireCustomShardingStage(t *testing.T) {
	t.Helper()
	requirePhase(t, phaseCustomSharding)
}

// TestConcurrentSetGetAcrossManyKeysHasNoLostWrites_Stage01ShardedStore
// spawns many goroutines, each on its own connection, each exclusively
// owning a private set of keys. Every SET is immediately followed by a GET
// of that exact same key on the same connection, and the read-back value
// must exactly equal the value just written. Because each goroutine only
// ever writes to keys it exclusively owns, there is no ambiguity about
// "last writer wins" across goroutines -- any mismatch indicates a lost or
// torn write, which is exactly what a buggy shard-routing/locking scheme
// would produce under -race.
func TestConcurrentSetGetAcrossManyKeysHasNoLostWrites_Stage01ShardedStore(t *testing.T) {
	requireCustomShardingStage(t)
	// Scenario: 50 goroutines each on their own connection perform 20
	// SET-then-GET round trips against keys they exclusively own, stressing
	// the store's internal locking across up to 1000 distinct keys without
	// any lost or corrupted writes.
	sp := startTinyRed(t)

	const goroutines = 50
	const iterations = 20

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			conn, r := dialClient(t, sp)
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("shard_test_key_%d_%d", gid, i%20)
				value := fmt.Sprintf("g%d-i%d", gid, i)

				writeRESPArray(t, conn, "SET", key, value)
				if got := readLine(t, r); got != "+OK\r\n" {
					errCh <- fmt.Errorf("goroutine %d iter %d: SET %s expected +OK, got %q", gid, i, key, got)
					continue
				}

				writeRESPArray(t, conn, "GET", key)
				want := fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
				if got := readBulkString(t, r); got != want {
					errCh <- fmt.Errorf("goroutine %d iter %d: GET %s expected %q, got %q", gid, i, key, want, got)
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("%v", err)
	}
}

// TestConcurrentWritesToSharedKeysSurviveWithoutCorruption_Stage01ShardedStore
// has many goroutines writing overlapping keys from a small shared pool (8
// keys), with no reads mid-flight. Before spawning any goroutine, the test
// records exactly which literal values were intended to be written to each
// key. After all writers finish, each key's final GET result must equal
// one of the literal strings that was actually sent for that key -- proving
// the final value is a clean, complete write from some goroutine, never a
// torn/mixed value from two concurrent writers racing on the same shard
// lock.
func TestConcurrentWritesToSharedKeysSurviveWithoutCorruption_Stage01ShardedStore(t *testing.T) {
	requireCustomShardingStage(t)
	// Scenario: many goroutines write overlapping keys from a small shared
	// pool; after all writes finish, every key's final value must be one of
	// the values actually written to it (no torn/corrupted values).
	sp := startTinyRed(t)

	const sharedKeys = 8
	const goroutines = 50
	const iterations = 10

	keys := make([]string, sharedKeys)
	for i := range keys {
		keys[i] = fmt.Sprintf("shard_shared_key_%d", i)
	}

	// Record, before spawning any goroutine, exactly which values will be
	// sent to which key.
	intended := make(map[string][]string, sharedKeys)
	for _, k := range keys {
		intended[k] = nil
	}
	type write struct {
		key   string
		value string
	}
	var plan []write
	for g := 0; g < goroutines; g++ {
		for i := 0; i < iterations; i++ {
			key := keys[(g+i)%sharedKeys]
			value := fmt.Sprintf("shared-g%d-i%d", g, i)
			intended[key] = append(intended[key], value)
			plan = append(plan, write{key: key, value: value})
		}
	}

	writesByGoroutine := make([][]write, goroutines)
	for idx, w := range plan {
		g := idx / iterations
		writesByGoroutine[g] = append(writesByGoroutine[g], w)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			conn, r := dialClient(t, sp)
			for _, w := range writesByGoroutine[gid] {
				writeRESPArray(t, conn, "SET", w.key, w.value)
				if got := readLine(t, r); got != "+OK\r\n" {
					errCh <- fmt.Errorf("goroutine %d: SET %s expected +OK, got %q", gid, w.key, got)
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("%v", err)
	}

	conn, r := dialClient(t, sp)
	for _, k := range keys {
		writeRESPArray(t, conn, "GET", k)
		got := readBulkString(t, r)

		candidates := intended[k]
		matched := false
		for _, v := range candidates {
			want := fmt.Sprintf("$%d\r\n%s\r\n", len(v), v)
			if got == want {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("key %s: final value %q does not match any of the %d values written to it", k, got, len(candidates))
		}
	}
}
