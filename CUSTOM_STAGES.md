# TinyRed: Custom Stage Reference

> **Source:** These stages are project-specific extensions to the CodeCrafters "Build Your Own Redis" challenge. They are **not** part of the official CodeCrafters curriculum — see [CODECRAFTERS_REDIS_STAGES.md](CODECRAFTERS_REDIS_STAGES.md) for that.
> **Redis Protocol Spec:** [RESP2](https://redis.io/docs/latest/develop/reference/protocol-spec/)
> **Redis command reference:** [redis.io/commands](https://redis.io/commands/)

---
## How To Use This File

This file is written in the same format as `CODECRAFTERS_REDIS_STAGES.md` so you can work through it the same way: one stage at a time, one commit per stage. Each stage below contains:

- **What to implement** — the exact task
- **Tests** — what the black-box test suite checks (see the sibling `custom_*_stage_behavior_test.go` files)
- **Notes** — tips and gotchas

Unlike the CodeCrafters stages, there's no official tester binary for these — the Go test files in this repo (prefixed `custom_`) act as the tester. Each test file is gated behind its own environment variable (e.g. `TINYRED_CUSTOM_SETS_STAGE`), exactly like the CodeCrafters-phase test files, so tests skip until you bump the stage number as you implement each command.

Build each stage one at a time. One commit per stage.

---

# PHASE C1: SETS (5 stages)

Add a `set` data type to the store: an unordered collection of unique string members.

---

## Stage 1 — Create a set (SADD)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SETS_STAGE` |
| **Difficulty** | Easy |

### What to implement

The [`SADD`](https://redis.io/docs/latest/commands/sadd/) command adds one or more members to a set stored at `key`. If the key doesn't exist, a new set is created first.

```bash
> SADD myset "foo"
(integer) 1
> SADD myset "foo" "bar"
(integer) 1
```

`SADD` returns the number of members that were **newly added** (i.e., that weren't already present), as a RESP integer. Adding a member that's already in the set does not error and does not count towards the return value.

### Tests

```bash
$ redis-cli SADD myset foo
# Expects: :1\r\n  (new set, 1 new member)

$ redis-cli SADD myset foo bar
# Expects: :1\r\n  (foo already present, only bar is new)

$ redis-cli SADD myset foo
# Expects: :0\r\n  (foo already present, nothing new)
```

### Notes

- Store the set as `map[string]struct{}` internally, matching the type hint in the project README.
- `SADD` on a key that holds a non-set value (e.g. a string) should return a `WRONGTYPE` error, matching the existing `resp.ErrWrongType()` pattern used by list commands.

---

## Stage 2 — List members (SMEMBERS)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SETS_STAGE` |
| **Difficulty** | Easy |

### What to implement

The [`SMEMBERS`](https://redis.io/docs/latest/commands/smembers/) command returns all members of a set as a RESP array of bulk strings. Order is not guaranteed (Redis returns them in an implementation-defined order; tests must sort before comparing).

```bash
> SADD myset "foo" "bar" "baz"
(integer) 3
> SMEMBERS myset
1) "foo"
2) "bar"
3) "baz"
```

If the key doesn't exist, `SMEMBERS` returns an empty array (`*0\r\n`), not an error.

### Tests

```bash
$ redis-cli SADD myset a b c
$ redis-cli SMEMBERS myset
# Expects a RESP array containing exactly ["a","b","c"] in any order

$ redis-cli SMEMBERS missing_set
# Expects: *0\r\n
```

### Notes

- Sort both the expected and actual member lists before comparing in tests, since set iteration order is unspecified.

---

## Stage 3 — Check membership (SISMEMBER)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SETS_STAGE` |
| **Difficulty** | Easy |

### What to implement

The [`SISMEMBER`](https://redis.io/docs/latest/commands/sismember/) command checks whether `member` is in the set at `key`. Returns `:1\r\n` if present, `:0\r\n` otherwise (including when the key doesn't exist at all).

```bash
> SADD myset "foo"
(integer) 1
> SISMEMBER myset "foo"
(integer) 1
> SISMEMBER myset "bar"
(integer) 0
> SISMEMBER missing_set "foo"
(integer) 0
```

### Tests

```bash
$ redis-cli SADD myset foo
$ redis-cli SISMEMBER myset foo    # Expects: :1\r\n
$ redis-cli SISMEMBER myset bar    # Expects: :0\r\n
$ redis-cli SISMEMBER missing bar  # Expects: :0\r\n
```

---

## Stage 4 — Count members (SCARD)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SETS_STAGE` |
| **Difficulty** | Easy |

### What to implement

The [`SCARD`](https://redis.io/docs/latest/commands/scard/) command returns the number of members in a set as a RESP integer. Returns `0` for a missing key.

```bash
> SADD myset "a" "b" "c"
(integer) 3
> SCARD myset
(integer) 3
> SCARD missing_set
(integer) 0
```

### Tests

```bash
$ redis-cli SADD myset a b c
$ redis-cli SCARD myset      # Expects: :3\r\n
$ redis-cli SCARD missing    # Expects: :0\r\n
```

---

## Stage 5 — Remove members (SREM)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SETS_STAGE` |
| **Difficulty** | Easy |

### What to implement

The [`SREM`](https://redis.io/docs/latest/commands/srem/) command removes one or more members from a set, returning the count of members that were actually removed (members not present don't count).

```bash
> SADD myset "a" "b" "c"
(integer) 3
> SREM myset "b" "missing"
(integer) 1
> SMEMBERS myset
1) "a"
2) "c"
```

### Tests

```bash
$ redis-cli SADD myset a b c
$ redis-cli SREM myset b missing   # Expects: :1\r\n
$ redis-cli SMEMBERS myset         # Expects ["a","c"] in any order
$ redis-cli SREM missing_set x     # Expects: :0\r\n
```

### Notes

- Removing the last member of a set is up to your implementation discretion whether to delete the key entirely or leave an empty set behind — Redis deletes the key. Match that behavior: after removing all members, `EXISTS key`/`SCARD key` should behave as if the key never existed.

---

# PHASE C2: HASHES (6 stages)

Add a `hash` data type: a `map[string]string` stored under a key, for field/value pairs.

---

## Stage 1 — Set a hash field (HSET)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_HASHES_STAGE` |
| **Difficulty** | Easy |

### What to implement

The [`HSET`](https://redis.io/docs/latest/commands/hset/) command sets one or more field-value pairs in a hash stored at `key`, creating the hash if it doesn't exist. Returns the number of fields that were **newly added** (updating an existing field's value doesn't count).

```bash
> HSET myhash field1 "foo"
(integer) 1
> HSET myhash field1 "bar" field2 "baz"
(integer) 1
```

In the second call, `field1` already existed (its value is updated to `"bar"`) so it doesn't count; only `field2` is new, so the return value is `1`.

### Tests

```bash
$ redis-cli HSET myhash f1 v1          # Expects: :1\r\n
$ redis-cli HSET myhash f1 v2 f2 v3    # Expects: :1\r\n (f1 updated, f2 new)
```

---

## Stage 2 — Get a hash field (HGET)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_HASHES_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`HGET`](https://redis.io/docs/latest/commands/hget/) returns the value of a field in a hash as a bulk string, or a null bulk string (`$-1\r\n`) if the field or the key doesn't exist.

```bash
> HSET myhash field1 "foo"
(integer) 1
> HGET myhash field1
"foo"
> HGET myhash missing_field
(nil)
> HGET missing_hash field1
(nil)
```

### Tests

```bash
$ redis-cli HSET myhash f1 v1
$ redis-cli HGET myhash f1        # Expects: $2\r\nv1\r\n
$ redis-cli HGET myhash missing   # Expects: $-1\r\n
$ redis-cli HGET missing f1       # Expects: $-1\r\n
```

---

## Stage 3 — Get all fields and values (HGETALL)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_HASHES_STAGE` |
| **Difficulty** | Medium |

### What to implement

[`HGETALL`](https://redis.io/docs/latest/commands/hgetall/) returns all fields and values of a hash as a flat RESP array: `[field1, value1, field2, value2, ...]`. Order is unspecified. Returns an empty array for a missing key.

```bash
> HSET myhash f1 "v1" f2 "v2"
(integer) 2
> HGETALL myhash
1) "f1"
2) "v1"
3) "f2"
4) "v2"
```

### Tests

```bash
$ redis-cli HSET myhash f1 v1 f2 v2
$ redis-cli HGETALL myhash
# Expects a RESP array of 4 bulk strings that, when paired up
# (element[0]=element[1], element[2]=element[3], in any pair order),
# reconstructs {f1: v1, f2: v2}

$ redis-cli HGETALL missing_hash
# Expects: *0\r\n
```

### Notes

- Since field order is unspecified, tests should reconstruct a `map[string]string` from the flat array (pairing consecutive elements) and compare maps, not raw slices.

---

## Stage 4 — Check field existence (HEXISTS)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_HASHES_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`HEXISTS`](https://redis.io/docs/latest/commands/hexists/) returns `:1\r\n` if the field exists in the hash, `:0\r\n` otherwise (including for a missing key).

```bash
> HSET myhash field1 "foo"
(integer) 1
> HEXISTS myhash field1
(integer) 1
> HEXISTS myhash missing_field
(integer) 0
```

### Tests

```bash
$ redis-cli HSET myhash f1 v1
$ redis-cli HEXISTS myhash f1        # Expects: :1\r\n
$ redis-cli HEXISTS myhash missing   # Expects: :0\r\n
$ redis-cli HEXISTS missing_hash f1  # Expects: :0\r\n
```

---

## Stage 5 — Count fields (HLEN)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_HASHES_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`HLEN`](https://redis.io/docs/latest/commands/hlen/) returns the number of fields in a hash as a RESP integer. Returns `0` for a missing key.

```bash
> HSET myhash f1 v1 f2 v2 f3 v3
(integer) 3
> HLEN myhash
(integer) 3
> HLEN missing_hash
(integer) 0
```

### Tests

```bash
$ redis-cli HSET myhash f1 v1 f2 v2
$ redis-cli HLEN myhash        # Expects: :2\r\n
$ redis-cli HLEN missing_hash  # Expects: :0\r\n
```

---

## Stage 6 — Delete fields (HDEL)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_HASHES_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`HDEL`](https://redis.io/docs/latest/commands/hdel/) removes one or more fields from a hash, returning the count of fields actually removed.

```bash
> HSET myhash f1 v1 f2 v2 f3 v3
(integer) 3
> HDEL myhash f1 missing_field
(integer) 1
> HGETALL myhash
1) "f2"
2) "v2"
3) "f3"
4) "v3"
```

### Tests

```bash
$ redis-cli HSET myhash f1 v1 f2 v2
$ redis-cli HDEL myhash f1 missing   # Expects: :1\r\n
$ redis-cli HEXISTS myhash f1        # Expects: :0\r\n
$ redis-cli HDEL missing_hash f1     # Expects: :0\r\n
```

### Notes

- Like `SREM`, deleting all fields from a hash should remove the key entirely (matching real Redis).

---

# PHASE C3: KEY MANAGEMENT (5 stages)

General-purpose key commands that work across all data types (string, list, set, hash, stream, zset).

---

## Stage 1 — Delete keys (DEL)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_KEYMGMT_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`DEL`](https://redis.io/docs/latest/commands/del/) deletes one or more keys, returning the count of keys that actually existed and were removed.

```bash
> SET foo "bar"
OK
> DEL foo missing_key
(integer) 1
> GET foo
(nil)
```

### Tests

```bash
$ redis-cli SET foo bar
$ redis-cli DEL foo missing   # Expects: :1\r\n
$ redis-cli GET foo           # Expects: $-1\r\n
$ redis-cli DEL a b c         # (none exist) Expects: :0\r\n
```

---

## Stage 2 — Check existence (EXISTS)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_KEYMGMT_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`EXISTS`](https://redis.io/docs/latest/commands/exists/) accepts one or more keys and returns the count of them that exist, as a RESP integer. The **same key repeated** counts multiple times if it exists.

```bash
> SET foo "bar"
OK
> EXISTS foo foo missing_key
(integer) 2
```

### Tests

```bash
$ redis-cli SET foo bar
$ redis-cli EXISTS foo foo missing   # Expects: :2\r\n
$ redis-cli EXISTS missing           # Expects: :0\r\n
```

---

## Stage 3 — TYPE across all data types

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_KEYMGMT_STAGE` |
| **Difficulty** | Easy |

### What to implement

Extend the `TYPE` command (already implemented for `string`/`none`/`stream` in the CodeCrafters phases) to also report `list`, `set`, `hash`, and `zset` for the data types added in the custom phases.

```bash
> RPUSH mylist "a"
(integer) 1
> TYPE mylist
list

> SADD myset "a"
(integer) 1
> TYPE myset
set

> HSET myhash f v
(integer) 1
> TYPE myhash
hash
```

### Tests

```bash
$ redis-cli RPUSH l a       ; redis-cli TYPE l   # Expects: +list\r\n
$ redis-cli SADD s a        ; redis-cli TYPE s   # Expects: +set\r\n
$ redis-cli HSET h f v      ; redis-cli TYPE h   # Expects: +hash\r\n
$ redis-cli ZADD z 1 a      ; redis-cli TYPE z   # Expects: +zset\r\n
```

### Notes

- This stage only makes sense once the Sets and Hashes phases (and the CodeCrafters Sorted Sets phase) are implemented — order your work accordingly, or skip the sub-cases for types you haven't built yet.

---

## Stage 4 — Glob pattern matching (KEYS)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_KEYMGMT_STAGE` |
| **Difficulty** | Medium |

### What to implement

Extend the existing `KEYS` command (which currently only supports the literal pattern `*`) to support glob-style patterns: `*` (any sequence), `?` (any single character), and `[abc]` (character class), matching [Redis's glob-style pattern matching](https://redis.io/docs/latest/commands/keys/#pattern).

```bash
> MSET firstname Jack lastname Stuntman age 35
OK
> KEYS "*name*"
1) "firstname"
2) "lastname"
> KEYS "a??"
1) "age"
```

### Tests

```bash
$ redis-cli SET firstname Jack
$ redis-cli SET lastname Stuntman
$ redis-cli SET age 35
$ redis-cli KEYS "*name*"   # Expects ["firstname","lastname"] in any order
$ redis-cli KEYS "a??"      # Expects ["age"]
$ redis-cli KEYS "*"        # Expects all 3 keys (regression check against the existing behavior)
```

### Notes

- Go's `path.Match`/`filepath.Match` implements a compatible-enough glob syntax for `*`, `?`, and `[...]` for this stage's purposes.

---

## Stage 5 — Rename a key (RENAME)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_KEYMGMT_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`RENAME`](https://redis.io/docs/latest/commands/rename/) renames `key` to `newkey`. If `newkey` already exists, it is overwritten. Returns `+OK\r\n`. If the source key doesn't exist, returns an error containing `no such key`.

```bash
> SET foo "bar"
OK
> RENAME foo baz
OK
> GET baz
"bar"
> GET foo
(nil)

> RENAME missing_key other
(error) ERR no such key
```

### Tests

```bash
$ redis-cli SET foo bar
$ redis-cli RENAME foo baz   # Expects: +OK\r\n
$ redis-cli GET baz          # Expects: $3\r\nbar\r\n
$ redis-cli GET foo          # Expects: $-1\r\n
$ redis-cli RENAME missing other   # Expects error containing "no such key"
```

### Notes

- `RENAME` should preserve the value's type and any TTL set on the original key (see the TTL phase below) — renaming a key with an active expiry should carry the expiry over to `newkey`.

---

# PHASE C4: TTL COMMANDS (5 stages)

Generalize the existing ad-hoc `SET ... PX/EX` expiry into standalone TTL-management commands.

---

## Stage 1 — Set expiry in seconds (EXPIRE)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_TTL_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`EXPIRE`](https://redis.io/docs/latest/commands/expire/) sets a TTL (in seconds) on an existing key. Returns `:1\r\n` if the timeout was set, `:0\r\n` if the key doesn't exist.

```bash
> SET foo "bar"
OK
> EXPIRE foo 100
(integer) 1
> EXPIRE missing_key 100
(integer) 0
```

### Tests

```bash
$ redis-cli SET foo bar
$ redis-cli EXPIRE foo 100        # Expects: :1\r\n
$ redis-cli EXPIRE missing 100    # Expects: :0\r\n

# expiry actually takes effect:
$ redis-cli SET short bar
$ redis-cli EXPIRE short 1
$ sleep 1.2 && redis-cli GET short   # Expects: $-1\r\n
```

---

## Stage 2 — Set expiry in milliseconds (PEXPIRE)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_TTL_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`PEXPIRE`](https://redis.io/docs/latest/commands/pexpire/) is identical to `EXPIRE` but takes the TTL in milliseconds.

```bash
> SET foo "bar"
OK
> PEXPIRE foo 100
(integer) 1
```

### Tests

```bash
$ redis-cli SET foo bar
$ redis-cli PEXPIRE foo 150      # Expects: :1\r\n
$ redis-cli GET foo              # Expects value immediately
$ sleep 0.2 && redis-cli GET foo # Expects: $-1\r\n (expired)
$ redis-cli PEXPIRE missing 100  # Expects: :0\r\n
```

---

## Stage 3 — Query remaining TTL in seconds (TTL)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_TTL_STAGE` |
| **Difficulty** | Medium |

### What to implement

[`TTL`](https://redis.io/docs/latest/commands/ttl/) returns the remaining time to live of a key, in seconds, rounded (Redis rounds to the nearest second):

- `-2` if the key does not exist
- `-1` if the key exists but has no associated expiry
- Otherwise, the remaining TTL in seconds (as a positive integer)

```bash
> SET foo "bar"
OK
> TTL foo
(integer) -1
> EXPIRE foo 100
(integer) 1
> TTL foo
(integer) 100
> TTL missing_key
(integer) -2
```

### Tests

```bash
$ redis-cli SET foo bar
$ redis-cli TTL foo           # Expects: :-1\r\n
$ redis-cli EXPIRE foo 100
$ redis-cli TTL foo           # Expects a value in [99, 100] (allow 1s of test-execution slack)
$ redis-cli TTL missing       # Expects: :-2\r\n
```

---

## Stage 4 — Query remaining TTL in milliseconds (PTTL)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_TTL_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`PTTL`](https://redis.io/docs/latest/commands/pttl/) is identical to `TTL` but returns the remaining time in milliseconds. Same special values: `-2` (missing key), `-1` (no expiry).

```bash
> SET foo "bar" PX 10000
OK
> PTTL foo
(integer) 9987
```

### Tests

```bash
$ redis-cli SET foo bar PX 10000
$ redis-cli PTTL foo         # Expects a value in (0, 10000], with generous slack for test timing
$ redis-cli PTTL missing     # Expects: :-2\r\n

$ redis-cli SET bar baz
$ redis-cli PTTL bar         # Expects: :-1\r\n (no expiry)
```

---

## Stage 5 — Remove expiry (PERSIST)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_TTL_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`PERSIST`](https://redis.io/docs/latest/commands/persist/) removes any existing TTL from a key, making it persist forever. Returns `:1\r\n` if a TTL was removed, `:0\r\n` if the key had no TTL (or doesn't exist).

```bash
> SET foo "bar" EX 100
OK
> PERSIST foo
(integer) 1
> TTL foo
(integer) -1
> PERSIST foo
(integer) 0
```

### Tests

```bash
$ redis-cli SET foo bar EX 100
$ redis-cli PERSIST foo        # Expects: :1\r\n
$ redis-cli TTL foo            # Expects: :-1\r\n
$ redis-cli PERSIST foo        # Expects: :0\r\n (already persistent)
$ redis-cli PERSIST missing    # Expects: :0\r\n
```

---

# PHASE C5: BATCH STRING OPS (6 stages)

Additional string commands beyond `SET`/`GET`/`INCR`.

---

## Stage 1 — Set multiple keys (MSET)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_STRINGOPS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`MSET`](https://redis.io/docs/latest/commands/mset/) sets multiple key-value pairs atomically (from the perspective of other clients, all pairs become visible at once). Always returns `+OK\r\n`.

```bash
> MSET k1 "v1" k2 "v2" k3 "v3"
OK
> GET k1
"v1"
```

### Tests

```bash
$ redis-cli MSET a 1 b 2 c 3   # Expects: +OK\r\n
$ redis-cli GET a              # Expects: $1\r\n1\r\n
$ redis-cli GET b              # Expects: $1\r\n2\r\n
$ redis-cli GET c              # Expects: $1\r\n3\r\n
```

---

## Stage 2 — Get multiple keys (MGET)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_STRINGOPS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`MGET`](https://redis.io/docs/latest/commands/mget/) returns the values of multiple keys as a RESP array, with a null bulk string in place of any missing key (or any key holding a non-string value).

```bash
> SET k1 "v1"
OK
> MGET k1 missing_key
1) "v1"
2) (nil)
```

### Tests

```bash
$ redis-cli SET a 1
$ redis-cli MGET a missing b
# Expects RESP array: ["1", nil, nil] (b was never set either)
```

---

## Stage 3 — Append to a string (APPEND)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_STRINGOPS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`APPEND`](https://redis.io/docs/latest/commands/append/) appends a value to an existing string (creating the key with that value if it doesn't exist). Returns the length of the string **after** the append, as a RESP integer.

```bash
> SET foo "Hello "
OK
> APPEND foo "World"
(integer) 11
> GET foo
"Hello World"

> APPEND missing_key "abc"
(integer) 3
```

### Tests

```bash
$ redis-cli SET foo "Hello "
$ redis-cli APPEND foo World   # Expects: :11\r\n
$ redis-cli GET foo            # Expects: $11\r\nHello World\r\n
$ redis-cli APPEND newkey abc  # Expects: :3\r\n
$ redis-cli GET newkey         # Expects: $3\r\nabc\r\n
```

---

## Stage 4 — Decrement (DECR)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_STRINGOPS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`DECR`](https://redis.io/docs/latest/commands/decr/) decrements a key's integer value by 1, mirroring `INCR`'s already-implemented behavior (creates the key at `-1` if missing; errors with `ERR value is not an integer or out of range` if the existing value isn't an integer).

```bash
> SET foo "10"
OK
> DECR foo
(integer) 9
> DECR missing_key
(integer) -1
```

### Tests

```bash
$ redis-cli SET foo 10
$ redis-cli DECR foo         # Expects: :9\r\n
$ redis-cli DECR missing     # Expects: :-1\r\n
$ redis-cli SET bar xyz
$ redis-cli DECR bar         # Expects error containing "not an integer"
```

---

## Stage 5 — Increment by amount (INCRBY)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_STRINGOPS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`INCRBY`](https://redis.io/docs/latest/commands/incrby/) increments a key's integer value by the given amount (which may be negative).

```bash
> SET foo "10"
OK
> INCRBY foo 5
(integer) 15
> INCRBY foo -20
(integer) -5
```

### Tests

```bash
$ redis-cli SET foo 10
$ redis-cli INCRBY foo 5      # Expects: :15\r\n
$ redis-cli INCRBY foo -20    # Expects: :-5\r\n
$ redis-cli INCRBY missing 3  # Expects: :3\r\n (created at 0, then incremented)
```

---

## Stage 6 — Decrement by amount (DECRBY)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_STRINGOPS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`DECRBY`](https://redis.io/docs/latest/commands/decrby/) decrements a key's integer value by the given amount.

```bash
> SET foo "10"
OK
> DECRBY foo 3
(integer) 7
```

### Tests

```bash
$ redis-cli SET foo 10
$ redis-cli DECRBY foo 3      # Expects: :7\r\n
$ redis-cli DECRBY missing 3  # Expects: :-3\r\n
```

---

# PHASE C6: SHARDED STORE (1 stage)

Replace the single `map[string]*resp.Entry` + `sync.RWMutex` in `store.Store` with 64 independently-locked shards, to reduce lock contention under concurrent load.

---

## Stage 1 — 64-shard store with FNV-1a distribution

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SHARDING_STAGE` |
| **Difficulty** | Hard |

### What to implement

Replace the store's internal single map with 64 shards, each with its own `sync.RWMutex`. Route each key to a shard using the [FNV-1a hash](https://en.wikipedia.org/wiki/Fowler%E2%80%93Noll%E2%80%93Vo_hash_function) of the key, modulo 64 (Go's standard library provides this at `hash/fnv`).

This is purely an internal performance refactor — from the client's perspective, behavior must be **identical** to the single-map implementation. There is no new wire-protocol behavior to test; the only way to verify this stage from a black-box test is to hammer the store with concurrent operations across many keys and confirm:

- No data races (run with `go test -race`).
- No lost writes: every key that was set is later retrievable with its most-recently-written value.
- Existing single-shard behavior (SET/GET/DEL/EXPIRE/list & hash & set ops, etc.) is unaffected.

### Tests

The test suite for this stage does **not** inspect shard internals directly (they're a private implementation detail). Instead, it:

1. Starts the server.
2. Spawns N concurrent goroutines (e.g. 50), each opening its own connection and performing a sequence of `SET key<i> <goroutine-id>-<iteration>` / `GET key<i>` calls across a shared pool of ~200 keys, for a fixed number of iterations.
3. After all goroutines finish, verifies via `GET` that every key holds *some* value that was actually written by *some* goroutine (not corrupted, truncated, or a mix of two writes) — i.e., each final value exactly matches one of the literal strings that was sent for that key.
4. Runs the whole test binary with `-race` in CI (`go test -race ./...`) to catch shard-locking bugs.

### Notes

- Since this is purely internal, you can implement it however you like as long as the *effective* concurrency semantics stay correct (each key is still guarded by exactly one lock at a time — no torn reads/writes).
- A reasonable interface: keep `store.Store`'s public methods (`Get`, `Set`, `Update`, `Delete`, `Keys`, `ListPush`, `BLPOP`, ...) unchanged; only the internal storage swaps from one map to `[64]shard`.

---

# PHASE C7: ACTIVE EXPIRY (1 stage)

Currently, expired keys are only removed lazily, on access (see `HandleGet`'s expiry check in `server/commad_handlers.go`). This phase adds a background sweep so expired keys are removed proactively, even if nothing ever reads them again.

---

## Stage 1 — Background expiry sweep

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_ACTIVE_EXPIRY_STAGE` |
| **Difficulty** | Medium |

### What to implement

Run a background goroutine that, every 100ms, samples 20 random keys per shard (see Phase C6 — if sharding isn't implemented yet, sample 20 random keys from the whole store instead) and removes any that have expired. If more than 25% of the sampled keys in a shard were expired, immediately resample that shard (repeat until fewer than 25% are expired, to catch up quickly after a burst of short-TTL keys). This mirrors the [real Redis probabilistic expiry algorithm](https://redis.io/docs/latest/commands/expire/#how-redis-expires-keys).

The **observable** effect: a key set with a short TTL and never subsequently read should still disappear from `DBSIZE` / `KEYS *` shortly after expiring — not just when someone happens to `GET` it.

```bash
> SET foo "bar" PX 50
OK
> DBSIZE
(integer) 1
$ sleep 0.3
> DBSIZE
(integer) 0   # foo was actively swept, even though nobody called GET foo
```

### Tests

```bash
$ redis-cli SET foo bar PX 50
$ redis-cli DBSIZE            # Expects: :1\r\n (immediately)
$ sleep 0.3
$ redis-cli KEYS "*"          # Expects: *0\r\n — foo is gone from KEYS without ever being GET
$ redis-cli DBSIZE            # Expects: :0\r\n
```

The test never calls `GET foo` — that's the whole point: it isolates the *active* sweep from the pre-existing *lazy* expiry-on-read path (which is already covered by the base-stage expiry tests).

### Notes

- Depends on Phase C8's `DBSIZE` command (or you can test purely via `KEYS *` if you implement this before Server Commands).
- Keep the sweep interval and sample size configurable via constants so tests can use short TTLs without flaking; don't hardcode assumptions about real wall-clock Redis timing into the test — just assert "gone within some generous multiple of the sweep interval" (e.g. 500ms).

---

# PHASE C8: SERVER COMMANDS (4 stages)

Administrative commands that report on or reset the whole server/database.

---

## Stage 1 — Count keys (DBSIZE)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SERVERCMDS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`DBSIZE`](https://redis.io/docs/latest/commands/dbsize/) returns the number of keys in the database as a RESP integer. Keys that have already expired (whether or not the active sweep from Phase C7 has removed them yet) must **not** be counted.

```bash
> SET foo "bar"
OK
> SET baz "qux"
OK
> DBSIZE
(integer) 2
```

### Tests

```bash
$ redis-cli SET a 1
$ redis-cli SET b 2
$ redis-cli DBSIZE          # Expects: :2\r\n
$ redis-cli SET c 3 PX 50
$ sleep 0.2
$ redis-cli DBSIZE          # Expects: :2\r\n (c has expired and must not be counted, active-sweep or not)
```

---

## Stage 2 — Flush the database (FLUSHDB)

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SERVERCMDS_STAGE` |
| **Difficulty** | Easy |

### What to implement

[`FLUSHDB`](https://redis.io/docs/latest/commands/flushdb/) deletes every key in the database. Always returns `+OK\r\n`.

```bash
> SET foo "bar"
OK
> FLUSHDB
OK
> DBSIZE
(integer) 0
```

### Tests

```bash
$ redis-cli SET a 1
$ redis-cli SET b 2
$ redis-cli FLUSHDB       # Expects: +OK\r\n
$ redis-cli DBSIZE        # Expects: :0\r\n
$ redis-cli GET a         # Expects: $-1\r\n
```

---

## Stage 3 — Server INFO stats

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SERVERCMDS_STAGE` |
| **Difficulty** | Medium |

### What to implement

Extend the `INFO` command (already implemented for the `replication` section in the CodeCrafters replication phase) to support a `server` section reporting at least:

- `tcp_port:<port>` — the port the server is listening on.
- `connected_clients:<n>` — the current number of connected clients.
- `uptime_in_seconds:<n>` — seconds since the server started.

```bash
> INFO server
# Server
tcp_port:6379
connected_clients:1
uptime_in_seconds:42
```

Called with no argument, `INFO` should include all known sections (`server` and `replication`, and any others you've implemented).

### Tests

```bash
$ redis-cli -p <PORT> INFO server
# Response bulk string must contain "tcp_port:<PORT>" with the exact port the server was started on

$ redis-cli INFO server
# Response must contain "connected_clients:" followed by a non-negative integer
# Response must contain "uptime_in_seconds:" followed by a non-negative integer
```

### Notes

- `connected_clients` should reflect concurrently open connections at the time of the call — a test can open N extra idle connections, call `INFO server`, and check the reported count is at least N+1 (including the querying connection itself).

---

## Stage 4 — CONFIG GET for arbitrary known keys

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_SERVERCMDS_STAGE` |
| **Difficulty** | Easy |

### What to implement

The existing `CONFIG GET` handler only recognizes `dir` and `dbfilename`. Extend it to also serve the AOF-related keys from the CodeCrafters AOF phase (`appendonly`, `appenddirname`, `appendfilename`, `appendfsync`) and the replication `port` — i.e., make `CONFIG GET` a single general-purpose entry point instead of a hardcoded two-key switch, returning an empty array (not an error) for a key it truly doesn't recognize.

```bash
> CONFIG GET port
1) "port"
2) "6379"

> CONFIG GET totally-unknown-key
(empty array)
```

### Tests

```bash
$ redis-cli CONFIG GET port
# Expects RESP array ["port", "<the actual configured port>"]

$ redis-cli CONFIG GET nonexistent-key
# Expects: *0\r\n  (empty array, not an error)
```

---

# PHASE C9: GRACEFUL SHUTDOWN + DOCKER + MAKEFILE (3 stages)

Operational polish: clean process shutdown, containerization, and a standard build interface.

---

## Stage 1 — Graceful shutdown on SIGINT/SIGTERM

| | |
|---|---|
| **Env var** | `TINYRED_CUSTOM_GRACEFUL_SHUTDOWN_STAGE` |
| **Difficulty** | Medium |

### What to implement

On receiving `SIGINT` or `SIGTERM`, the server should:

1. Stop accepting new TCP connections.
2. Let in-flight commands on existing connections finish (don't abort mid-command).
3. If AOF is enabled, flush and close the AOF file cleanly.
4. Exit the process with status code `0`.

The server must not hang indefinitely waiting for idle-but-still-open client connections — a bounded drain timeout (e.g. a few seconds) is acceptable, after which remaining connections are force-closed.

### Tests

```bash
# Start the server as a subprocess.
# Open a client connection and leave it idle.
# Send SIGTERM to the server process.
# Assert: the process exits within a few seconds, with exit code 0.
```

A stronger version of this test: send a `SET` command, and while the response is in flight, send SIGTERM — assert the client still receives the `+OK\r\n` response before the connection closes (i.e., no command is aborted mid-flight).

### Notes

- In Go, use `signal.NotifyContext` (or `signal.Notify` + a done channel) to intercept `SIGINT`/`SIGTERM`, and thread a `context.Context` (or a "draining" flag) through `Serve`/`HandleConection` so the accept loop stops and existing connections are given a chance to finish.
- Since this changes process lifecycle rather than wire protocol, tests for this stage necessarily drive the server as an OS subprocess and send real signals (`syscall.SIGTERM` via `os.Process.Signal`), rather than talking over TCP alone.

---

## Stage 2 — Multi-stage Dockerfile

| | |
|---|---|
| **Env var** | N/A (not runtime-testable via `go test`) |
| **Difficulty** | Easy |

### What to implement

A multi-stage `Dockerfile` at the repo root:

- **Build stage:** a `golang:*` base image that compiles the `tinyred` binary.
- **Runtime stage:** a minimal base image (e.g. `alpine` or `scratch` + certs) that copies in just the compiled binary and runs it, exposing port `6379`.

### Tests

Automated `go test` coverage for this stage is limited to a lightweight presence/lint check (see `custom_graceful_shutdown_stage_behavior_test.go` or a dedicated small test) that verifies:

- `Dockerfile` exists at the repo root.
- It contains at least two `FROM` lines (multi-stage).
- It contains an `EXPOSE 6379` (or equivalent documented port) line.

A full `docker build .` is **not** run as part of the Go test suite (too slow/heavy for unit tests, and requires a Docker daemon) — verify that manually:

```bash
$ docker build -t tinyred .
$ docker run -p 6379:6379 tinyred
$ redis-cli PING   # Expects: PONG
```

---

## Stage 3 — Makefile with build/run/test targets

| | |
|---|---|
| **Env var** | N/A (not runtime-testable via `go test`) |
| **Difficulty** | Easy |

### What to implement

A `Makefile` at the repo root with at least three targets:

- `make build` — compiles the `tinyred` binary.
- `make run` — builds (if needed) and runs the server.
- `make test` — runs `go test ./...`.

### Tests

Like Stage 2, this is checked with a lightweight presence/lint test:

- `Makefile` exists at the repo root.
- It defines `build:`, `run:`, and `test:` targets (grep for lines matching `^build:`, `^run:`, `^test:`).

Actually invoking `make build`/`make run` is a manual verification step, not part of the automated Go test suite:

```bash
$ make build
$ make test
$ make run &
$ redis-cli PING   # Expects: PONG
```
