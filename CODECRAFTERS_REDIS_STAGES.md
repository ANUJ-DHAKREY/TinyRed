# CodeCrafters: Build Your Own Redis — Complete Stage Reference

> **Source:** [codecrafters-io/build-your-own-redis](https://github.com/codecrafters-io/build-your-own-redis) (public repo)
> **Stage descriptions:** [stage_descriptions/](https://github.com/codecrafters-io/build-your-own-redis/tree/main/stage_descriptions)
> **Redis Protocol Spec:** [RESP2](https://redis.io/docs/latest/develop/reference/protocol-spec/)

---
## How To Use This File

Each stage below contains:
- **What to implement** — the exact task
- **Tests** — what the tester checks (write your own tests to match these)
- **Notes** — tips and gotchas
- **Links** — to Redis docs and the original stage description on GitHub

Build each stage one at a time. One commit per stage.

---

# PHASE 1: BASE STAGES (7 stages)

These are mandatory. Every CodeCrafters participant completes these first, in this exact order.

---

## Stage 1 — Bind to a Port

| | |
|---|---|
| **Slug** | `jm1` |
| **Difficulty** | Very Easy |
| **Source** | [base-01-jm1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/base-01-jm1.md) |

### What to implement

Redis servers communicate over TCP. Start a TCP server that listens on port **6379**.

- Accept a connection
- You can close it immediately for now — handling commands comes in later stages
- You only need to handle a single client in this stage

### Tests

```bash
$ ./your_program.sh
# Tester connects to port 6379 via TCP
# Expects the connection to be accepted
```

### Notes

- Use `net.Listen("tcp", ":6379")` in Go
- Accept connections in a loop even if you only handle one for now

### Knowledge needed

- [Go TCP server basics](https://pkg.go.dev/net#Listen)
- What TCP is and how client-server connections work

---

## Stage 2 — Respond to PING

| | |
|---|---|
| **Slug** | `rg2` |
| **Difficulty** | Easy |
| **Source** | [base-02-rg2.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/base-02-rg2.md) |

### What to implement

`PING` is the simplest Redis command — used to check if a server is alive.

When a client sends `PING`, respond with `+PONG\r\n` (the RESP simple string encoding of "PONG").

For this stage, **hardcode** the response. You don't need to parse input yet.

### Tests

```bash
$ ./your_program.sh
$ redis-cli PING
# Expects: +PONG\r\n
```

### Notes

- The raw bytes the client sends will be `*1\r\n$4\r\nPING\r\n` (RESP array encoding), but you can ignore the input for now
- Just write `+PONG\r\n` to the connection

### Knowledge needed

- [RESP protocol: Simple Strings](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings)
- [PING command](https://redis.io/commands/ping)

---

## Stage 3 — Respond to Multiple PINGs

| | |
|---|---|
| **Slug** | `wy1` |
| **Difficulty** | Easy |
| **Source** | [base-03-wy1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/base-03-wy1.md) |

### What to implement

A Redis server listens for the next command as soon as it's done responding to the previous one. Run a **loop** that continuously reads commands and responds with `+PONG\r\n`.

### Tests

```bash
$ ./your_program.sh
$ echo -e "PING\nPING" | redis-cli
# Expects +PONG\r\n for each PING sent via the SAME connection
```

### Notes

- The exact bytes received will be `*1\r\n$4\r\nPING\r\n` (RESP encoding), not plain `PING`
- You can still hardcode `+PONG\r\n` — parsing comes later
- Multiple PINGs come over the **same** connection (not separate connections)

---

## Stage 4 — Handle Concurrent Clients

| | |
|---|---|
| **Slug** | `zu2` |
| **Difficulty** | Medium |
| **Source** | [base-04-zu2.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/base-04-zu2.md) |

### What to implement

Handle **multiple clients at once**. In Go, spawn a goroutine per connection:

```go
for {
    conn, _ := listener.Accept()
    go handleConnection(conn)
}
```

### Tests

```bash
$ ./your_program.sh
# Two concurrent redis-cli PING commands from different connections
$ redis-cli PING &
$ redis-cli PING &
# Expects two separate +PONG\r\n responses
```

### Notes

- Since you're only handling `PING` right now, you can still hardcode the response
- The key change is spawning a goroutine (or thread) per connection

### Knowledge needed

- [Goroutines](https://go.dev/tour/concurrency/1)
- [Event Loop](https://en.wikipedia.org/wiki/Event_loop) (what Redis actually uses)

---

## Stage 5 — Implement the ECHO Command

| | |
|---|---|
| **Slug** | `qq0` |
| **Difficulty** | Medium |
| **Source** | [base-05-qq0.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/base-05-qq0.md) |

### What to implement

The `ECHO` command accepts a single argument and sends it back as a RESP bulk string.

```bash
$ redis-cli ECHO hey
"hey"
```

You need to:
1. **Parse the RESP input** to extract the command name and argument
2. Return the argument as a bulk string: `$3\r\nhey\r\n`

### Tests

```bash
$ ./your_program.sh
$ redis-cli ECHO hey
# Raw bytes received: *2\r\n$4\r\nECHO\r\n$3\r\nhey\r\n
# Expected response: $3\r\nhey\r\n
```

### Notes

- **Implement a proper RESP parser at this stage** — you'll reuse it everywhere
- Command names are **case-insensitive**: `ECHO`, `echo`, `EcHo` all work
- The argument will be random, so you can't hardcode the response

### Knowledge needed

- [RESP protocol: Arrays](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays)
- [RESP protocol: Bulk Strings](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings)
- [ECHO command](https://redis.io/commands/echo)

---

## Stage 6 — Implement SET & GET Commands

| | |
|---|---|
| **Slug** | `la7` |
| **Difficulty** | Medium |
| **Source** | [base-06-la7.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/base-06-la7.md) |

### What to implement

**SET** stores a key-value pair. **GET** retrieves it.

```bash
$ redis-cli SET foo bar    →  +OK\r\n
$ redis-cli GET foo        →  $3\r\nbar\r\n
$ redis-cli GET missing    →  $-1\r\n  (null bulk string)
```

### Tests

```bash
$ ./your_program.sh
$ redis-cli SET foo bar
# Expects: +OK\r\n (simple string)

$ redis-cli GET foo
# Expects: $3\r\nbar\r\n (bulk string)
```

### Notes

- Keys and values are random — can't hardcode
- If a key doesn't exist, GET returns null bulk string `$-1\r\n`
- No expiry support yet (that's next stage)
- If you built a RESP parser in the previous stage, reuse it here

### Knowledge needed

- [SET command](https://redis.io/commands/set)
- [GET command](https://redis.io/commands/get)
- [Null Bulk Strings](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-bulk-strings)

---

## Stage 7 — Expiry

| | |
|---|---|
| **Slug** | `yz1` |
| **Difficulty** | Medium |
| **Source** | [base-07-yz1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/base-07-yz1.md) |

### What to implement

Support the **PX** option on SET — expiry in milliseconds.

```bash
$ redis-cli SET foo bar PX 100    →  +OK\r\n
$ redis-cli GET foo               →  $3\r\nbar\r\n   (immediately)
$ sleep 0.2 && redis-cli GET foo  →  $-1\r\n          (after expiry)
```

### Tests

```bash
$ ./your_program.sh
$ redis-cli SET foo bar PX 100
# Expects: +OK\r\n

$ redis-cli GET foo              # immediately
# Expects: $3\r\nbar\r\n

$ sleep 0.2 && redis-cli GET foo # after 200ms
# Expects: $-1\r\n (null bulk string — key expired)
```

### Notes

- Also support `EX` (seconds): `SET key value EX 10`
- Arguments are **case-insensitive**: `PX`, `px`, `pX` all valid
- Keys, values, and expiry times are random

### Knowledge needed

- [SET command options](https://redis.io/docs/latest/commands/set/#options)
- `time.Now()` and `time.After()` in Go for tracking expiry

---

# PHASE 2: LISTS (11 stages)

---

## Stage 8 — Create a List (RPUSH)

| | |
|---|---|
| **Slug** | `mh6` |
| **Difficulty** | Easy |
| **Source** | [lists-01-mh6.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-01-mh6.md) |

### What to implement

`RPUSH` appends elements to a list. If the list doesn't exist, create it first.

```bash
> RPUSH list_key "foo"
(integer) 1
```

Return the list length as a RESP integer (`:1\r\n`).

### Tests

```bash
$ redis-cli RPUSH list_key "element"
# Expects: :1\r\n
```

### Notes

- Only handle creating a new list with a single element in this stage
- [RPUSH docs](https://redis.io/docs/latest/commands/rpush/)

---

## Stage 9 — Append an Element (RPUSH existing)

| | |
|---|---|
| **Slug** | `tn7` |
| **Difficulty** | Easy |
| **Source** | [lists-02-tn7.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-02-tn7.md) |

### What to implement

When `RPUSH` is called on an existing list, append to the end. Return new length.

```bash
$ redis-cli RPUSH list_key "element1"    →  :1\r\n
$ redis-cli RPUSH list_key "element2"    →  :2\r\n
```

### Tests

Multiple RPUSH commands on the same list. Each returns updated length as RESP integer.

---

## Stage 10 — Append Multiple Elements (RPUSH)

| | |
|---|---|
| **Slug** | `lx4` |
| **Difficulty** | Easy |
| **Source** | [lists-03-lx4.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-03-lx4.md) |

### What to implement

`RPUSH` accepts multiple elements at once:

```bash
> RPUSH list_key "element1" "element2" "element3"
(integer) 3
```

### Tests

```bash
$ redis-cli RPUSH list_key "element1" "element2" "element3"
# Expects: :3\r\n

$ redis-cli RPUSH list_key "element4" "element5"
# Expects: :5\r\n
```

---

## Stage 11 — List Elements: Positive Indexes (LRANGE)

| | |
|---|---|
| **Slug** | `sf6` |
| **Difficulty** | Easy |
| **Source** | [lists-04-sf6.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-04-sf6.md) |

### What to implement

`LRANGE key start stop` returns elements from index `start` to `stop` (inclusive).

```bash
> RPUSH list_key "a" "b" "c" "d" "e"
(integer) 5

> LRANGE list_key 0 1
1) "a"
2) "b"

> LRANGE list_key 2 4
1) "c"
2) "d"
3) "e"
```

Edge cases:
- List doesn't exist → empty array `*0\r\n`
- `start` ≥ list length → empty array
- `stop` ≥ list length → treat as last element
- `start` > `stop` → empty array

### Tests

Creates list with RPUSH, then sends LRANGE commands. Expects RESP arrays.

### Notes

- Only positive indexes in this stage. Negative indexes come next.
- [LRANGE docs](https://redis.io/docs/latest/commands/lrange/)

---

## Stage 12 — List Elements: Negative Indexes (LRANGE)

| | |
|---|---|
| **Slug** | `ri1` |
| **Difficulty** | Easy |
| **Source** | [lists-05-ri1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-05-ri1.md) |

### What to implement

`-1` = last element, `-2` = second-to-last, etc.

```bash
> LRANGE list_key -2 -1
1) "d"
2) "e"

> LRANGE list_key 0 -3
1) "a"
2) "b"
3) "c"
```

- Negative index out of range (e.g. `-6` on list of 5) → treat as `0`

### Tests

```bash
$ redis-cli LRANGE list_key 2 -1
# Expects: ["c", "d", "e"] as RESP array
```

---

## Stage 13 — Prepend Elements (LPUSH)

| | |
|---|---|
| **Slug** | `gu5` |
| **Difficulty** | Easy |
| **Source** | [lists-06-gu5.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-06-gu5.md) |

### What to implement

`LPUSH` inserts elements at the **start** of the list. Elements are added in reverse order.

```bash
> LPUSH list_key "a" "b" "c"
(integer) 3

> LRANGE list_key 0 -1
1) "c"
2) "b"
3) "a"
```

### Tests

Sends LPUSH commands, then verifies order with LRANGE.

- [LPUSH docs](https://redis.io/docs/latest/commands/lpush/)

---

## Stage 14 — Query List Length (LLEN)

| | |
|---|---|
| **Slug** | `fv6` |
| **Difficulty** | Easy |
| **Source** | [lists-07-fv6.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-07-fv6.md) |

### What to implement

`LLEN key` returns the length of the list as a RESP integer. Non-existent list returns `:0\r\n`.

```bash
> RPUSH list_key "a" "b" "c" "d"
(integer) 4
> LLEN list_key
(integer) 4
> LLEN missing_list_key
(integer) 0
```

- [LLEN docs](https://redis.io/docs/latest/commands/llen/)

---

## Stage 15 — Remove an Element (LPOP)

| | |
|---|---|
| **Slug** | `ef1` |
| **Difficulty** | Easy |
| **Source** | [lists-08-ef1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-08-ef1.md) |

### What to implement

`LPOP key` removes and returns the first element. Returns null bulk string if list is empty or doesn't exist.

```bash
> RPUSH list_key "a" "b" "c" "d"
(integer) 4
> LPOP list_key
"a"
```

### Tests

After LPOP, verifies remaining elements with LRANGE.

- [LPOP docs](https://redis.io/docs/latest/commands/lpop/)

---

## Stage 16 — Remove Multiple Elements (LPOP count)

| | |
|---|---|
| **Slug** | `jp1` |
| **Difficulty** | Easy |
| **Source** | [lists-09-jp1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-09-jp1.md) |

### What to implement

`LPOP key count` removes and returns `count` elements as a RESP array.

```bash
> RPUSH list_key "a" "b" "c" "d"
(integer) 4
> LPOP list_key 2
1) "a"
2) "b"
```

If count > list length, return all elements.

---

## Stage 17 — Blocking Retrieval (BLPOP)

| | |
|---|---|
| **Slug** | `ec3` |
| **Difficulty** | Medium |
| **Source** | [lists-10-ec3.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-10-ec3.md) |

### What to implement

`BLPOP key timeout` blocks until an element is available, then pops it. With timeout `0`, blocks indefinitely.

```bash
# Client 1 (blocks):
> BLPOP list_key 0

# Client 2 (pushes):
> RPUSH list_key "foobar"

# Client 1 receives:
1) "list_key"
2) "foobar"
```

Response is a RESP array of [key_name, popped_element]. If multiple clients block on same list, serve the one that's been waiting longest.

### Tests

- BLPOP with timeout 0, then RPUSH from another client
- Multiple blocking clients — first one waiting gets served first

### Notes

- Only timeout `0` (indefinite) in this stage. Non-zero timeout comes next.
- [BLPOP docs](https://redis.io/docs/latest/commands/blpop/)

---

## Stage 18 — Blocking Retrieval with Timeout (BLPOP)

| | |
|---|---|
| **Slug** | `xj7` |
| **Difficulty** | Medium |
| **Source** | [lists-11-xj7.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/lists-11-xj7.md) |

### What to implement

BLPOP with a non-zero timeout (in seconds, can be fractional like `0.5`).

- If timeout expires with no element → return null array `*-1\r\n`
- If element pushed before timeout → return `[key, element]` as usual

### Tests

```bash
$ redis-cli BLPOP list_key 0.1
# (Blocks for 0.1 seconds, then returns *-1\r\n)
```

---

# PHASE 3: TRANSACTIONS (11 stages)

---

## Stage 19 — The INCR Command (1/3): Key Exists

| | |
|---|---|
| **Slug** | `si4` |
| **Difficulty** | Easy |
| **Source** | [transactions-01-si4.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-01-si4.md) |

### What to implement

`INCR key` increments a key's value by 1. Key must exist and have a numerical value.

```bash
> SET foo 41
OK
> INCR foo
(integer) 42
```

### Tests

```bash
$ redis-cli SET foo 41
# Expects: +OK\r\n
$ redis-cli INCR foo
# Expects: :42\r\n
```

- [INCR docs](https://redis.io/docs/latest/commands/incr/)

---

## Stage 20 — The INCR Command (2/3): Key Doesn't Exist

| | |
|---|---|
| **Slug** | `lz8` |
| **Difficulty** | Easy |
| **Source** | [transactions-02-lz8.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-02-lz8.md) |

### What to implement

When key doesn't exist, INCR sets the value to 1.

```bash
> INCR missing_key
(integer) 1
> GET missing_key
"1"
```

---

## Stage 21 — The INCR Command (3/3): Non-Integer Value

| | |
|---|---|
| **Slug** | `mk1` |
| **Difficulty** | Easy |
| **Source** | [transactions-03-mk1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-03-mk1.md) |

### What to implement

When key exists but value isn't a number, return an error.

```bash
> SET foo xyz
OK
> INCR foo
(error) ERR value is not an integer or out of range
```

### Tests

```bash
$ redis-cli SET foo xyz
# Expects: +OK\r\n
$ redis-cli INCR foo
# Expects: -ERR value is not an integer or out of range\r\n
```

---

## Stage 22 — The MULTI Command

| | |
|---|---|
| **Slug** | `pn0` |
| **Difficulty** | Easy |
| **Source** | [transactions-04-pn0.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-04-pn0.md) |

### What to implement

`MULTI` starts a transaction. Simply return `+OK\r\n`. Queueing comes in later stages.

### Tests

```bash
$ redis-cli MULTI
# Expects: +OK\r\n
```

- [MULTI docs](https://redis.io/docs/latest/commands/multi/)

---

## Stage 23 — The EXEC Command (without MULTI)

| | |
|---|---|
| **Slug** | `lo4` |
| **Difficulty** | Easy |
| **Source** | [transactions-05-lo4.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-05-lo4.md) |

### What to implement

`EXEC` without a preceding `MULTI` returns an error.

```bash
> EXEC
(error) ERR EXEC without MULTI
```

### Tests

```bash
$ redis-cli EXEC
# Expects: -ERR EXEC without MULTI\r\n
```

- [EXEC docs](https://redis.io/docs/latest/commands/exec/)

---

## Stage 24 — Empty Transaction

| | |
|---|---|
| **Slug** | `we1` |
| **Difficulty** | Hard |
| **Source** | [transactions-06-we1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-06-we1.md) |

### What to implement

`MULTI` followed immediately by `EXEC` (no commands queued) returns an empty array.

```bash
> MULTI
OK
> EXEC
(empty array)
```

After EXEC, the transaction ends. A second EXEC should return the error.

### Tests

```bash
> MULTI         # Expects: +OK\r\n
> EXEC          # Expects: *0\r\n (empty array)
> EXEC          # Expects: -ERR EXEC without MULTI\r\n
```

---

## Stage 25 — Queueing Commands

| | |
|---|---|
| **Slug** | `rs9` |
| **Difficulty** | Medium |
| **Source** | [transactions-07-rs9.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-07-rs9.md) |

### What to implement

After `MULTI`, any command is queued (not executed). Respond with `+QUEUED\r\n`.

```bash
> MULTI
OK
> SET foo 41
QUEUED
> INCR foo
QUEUED
```

Queued commands **must not** alter the database until EXEC.

### Tests

```bash
# Connection 1:
> MULTI
> SET foo 41     # Expects: +QUEUED\r\n
> INCR foo       # Expects: +QUEUED\r\n

# Connection 2 (separate):
> GET foo         # Expects: $-1\r\n (key doesn't exist yet)
```

---

## Stage 26 — Executing a Transaction

| | |
|---|---|
| **Slug** | `fy6` |
| **Difficulty** | Hard |
| **Source** | [transactions-08-fy6.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-08-fy6.md) |

### What to implement

`EXEC` executes all queued commands and returns a RESP array of their responses.

```bash
> MULTI
OK
> SET foo 41
QUEUED
> INCR foo
QUEUED
> EXEC
1) OK
2) (integer) 42
```

### Tests

```bash
> MULTI
> SET foo 6       # QUEUED
> INCR foo        # QUEUED
> INCR bar        # QUEUED
> GET bar         # QUEUED
> EXEC            # Expects array: [OK, 7, 1, "1"]

# After transaction:
> GET foo          # Expects: "7"
```

---

## Stage 27 — The DISCARD Command

| | |
|---|---|
| **Slug** | `rl9` |
| **Difficulty** | Easy |
| **Source** | [transactions-09-rl9.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-09-rl9.md) |

### What to implement

`DISCARD` aborts a transaction. Clears the queue, returns `+OK\r\n`. DISCARD without MULTI returns an error.

```bash
> MULTI
OK
> SET foo 41
QUEUED
> DISCARD
OK
> DISCARD
(error) ERR DISCARD without MULTI
```

### Tests

```bash
> MULTI
> SET foo 41     # QUEUED
> INCR foo       # QUEUED
> DISCARD        # Expects: +OK\r\n
> GET foo        # Expects: $-1\r\n (not set — transaction was discarded)
> DISCARD        # Expects: -ERR DISCARD without MULTI\r\n
```

- [DISCARD docs](https://redis.io/docs/latest/commands/discard/)

---

## Stage 28 — Failures Within Transactions

| | |
|---|---|
| **Slug** | `sg9` |
| **Difficulty** | Medium |
| **Source** | [transactions-10-sg9.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-10-sg9.md) |

### What to implement

If a command in a transaction fails, the error is returned in the EXEC response array. Other commands still execute.

```bash
> SET foo xyz
OK
> SET bar 41
OK
> MULTI
OK
> INCR foo        # will fail (not an integer)
QUEUED
> INCR bar        # will succeed
QUEUED
> EXEC
1) (error) ERR value is not an integer or out of range
2) (integer) 42
```

### Tests

EXEC returns array with error for failed command, success for others. Verifies with GET afterward.

- [Redis transactions: errors](https://redis.io/docs/latest/develop/interact/transactions/#errors-inside-a-transaction)

---

## Stage 29 — Multiple Concurrent Transactions

| | |
|---|---|
| **Slug** | `jf8` |
| **Difficulty** | Medium |
| **Source** | [transactions-11-jf8.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/transactions-11-jf8.md) |

### What to implement

Multiple connections can each have their own open transaction with separate command queues.

```bash
# Connection 1:
> MULTI → SET foo 41 → INCR foo → EXEC
# Returns: [OK, 42]

# Connection 2:
> MULTI → INCR foo → EXEC
# Returns: [43]   (foo was 42 from conn 1's transaction)
```

### Tests

Multiple redis-cli clients send MULTI/EXEC concurrently. Each has its own queue.

---

# PHASE 4: PUB/SUB (7 stages)

---

## Stage 30 — Subscribe to a Channel

| | |
|---|---|
| **Slug** | `mx3` |
| **Difficulty** | Easy |
| **Source** | [pub-sub-01-mx3.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/pub-sub-01-mx3.md) |

### What to implement

`SUBSCRIBE channel` registers the client as a subscriber. Response is a RESP array:

```bash
> SUBSCRIBE mychan
1) "subscribe"
2) "mychan"
3) (integer) 1
```

Array elements: `["subscribe", channel_name, subscribed_count]`

### Tests

```bash
$ redis-cli SUBSCRIBE foo
# Expects RESP: *3\r\n$9\r\nsubscribe\r\n$3\r\nfoo\r\n:1\r\n
```

- [SUBSCRIBE docs](https://redis.io/docs/latest/commands/subscribe/)

---

## Stage 31 — Subscribe to Multiple Channels

| | |
|---|---|
| **Slug** | `zc8` |
| **Difficulty** | Easy |
| **Source** | [pub-sub-02-zc8.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/pub-sub-02-zc8.md) |

### What to implement

A client can send SUBSCRIBE multiple times. The count increments per unique channel.

```bash
> SUBSCRIBE foo      →  ["subscribe", "foo", 1]
> SUBSCRIBE bar      →  ["subscribe", "bar", 2]
> SUBSCRIBE bar      →  ["subscribe", "bar", 2]   (same channel, count unchanged)
```

Counts are **per-client**.

---

## Stage 32 — Enter Subscribed Mode

| | |
|---|---|
| **Slug** | `aw8` |
| **Difficulty** | Medium |
| **Source** | [pub-sub-03-aw8.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/pub-sub-03-aw8.md) |

### What to implement

After SUBSCRIBE, the client enters "subscribed mode". Only these commands are allowed:
- `SUBSCRIBE`, `UNSUBSCRIBE`, `PSUBSCRIBE`, `PUNSUBSCRIBE`, `PING`, `QUIT`

All other commands return an error:

```bash
> ECHO hey
(error) ERR Can't execute 'echo': only (P|S)SUBSCRIBE / (P|S)UNSUBSCRIBE / PING / QUIT / RESET are allowed in this context
```

### Tests

Sends SUBSCRIBE then tries disallowed commands. Checks error starts with `ERR Can't execute '<command>'`.

---

## Stage 33 — PING in Subscribed Mode

| | |
|---|---|
| **Slug** | `lf1` |
| **Difficulty** | Easy |
| **Source** | [pub-sub-04-lf1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/pub-sub-04-lf1.md) |

### What to implement

In subscribed mode, PING responds with `["pong", ""]` (RESP array), **not** `+PONG\r\n`.

```
*2\r\n$4\r\npong\r\n$0\r\n\r\n
```

Non-subscribed clients still get `+PONG\r\n`.

---

## Stage 34 — Publish a Message

| | |
|---|---|
| **Slug** | `hf2` |
| **Difficulty** | Easy |
| **Source** | [pub-sub-05-hf2.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/pub-sub-05-hf2.md) |

### What to implement

`PUBLISH channel message` returns the number of clients subscribed to that channel as a RESP integer.

```bash
$ redis-cli PUBLISH bar "msg"
(integer) 2      # 2 clients subscribed to "bar"
```

In this stage, you don't need to deliver the message to subscribers yet — just return the count.

### Tests

Spawns multiple clients subscribing to channels, then PUBLISH and checks the count.

- [PUBLISH docs](https://redis.io/docs/latest/commands/publish/)

---

## Stage 35 — Deliver Messages

| | |
|---|---|
| **Slug** | `dn4` |
| **Difficulty** | Hard |
| **Source** | [pub-sub-06-dn4.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/pub-sub-06-dn4.md) |

### What to implement

When PUBLISH is called, actually **deliver the message** to all subscribed clients.

Each subscriber receives a RESP array:

```
1) "message"
2) "channel_1"
3) "hello"
```

RESP encoding:
```
*3\r\n$7\r\nmessage\r\n$9\r\nchannel_1\r\n$5\r\nhello\r\n
```

### Tests

- Multiple clients subscribe to different channels
- PUBLISH sends to a channel
- Verifies subscribed clients receive `["message", channel, content]`
- Verifies non-subscribed clients do NOT receive anything

### Notes

This is the **hardest pub/sub stage**. You need a dedicated goroutine per subscriber to forward messages from an internal channel to the TCP connection.

---

## Stage 36 — Unsubscribe

| | |
|---|---|
| **Slug** | `ze9` |
| **Difficulty** | Medium |
| **Source** | [pub-sub-07-ze9.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/pub-sub-07-ze9.md) |

### What to implement

`UNSUBSCRIBE channel` removes the client from a channel. Response:

```
1) "unsubscribe"
2) "foo"
3) (integer) 1       # remaining subscribed channels
```

Unsubscribing from a channel not subscribed to → returns the array with unchanged count.

### Tests

- Subscribe to multiple channels, unsubscribe from one, publish to both
- Verifies messages only go to still-subscribed channels
- [UNSUBSCRIBE docs](https://redis.io/docs/latest/commands/unsubscribe/)

---

# PHASE 5: AOF PERSISTENCE (10 stages)

---

## Stage 37 — Default AOF Options

| | |
|---|---|
| **Slug** | `uj3` |
| **Difficulty** | Easy |
| **Source** | [aof-01-uj3.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-01-uj3.md) |

### What to implement

Set up default values for AOF config options, retrievable via `CONFIG GET`:

| Option | Default |
|---|---|
| `dir` | Current working directory at startup |
| `appendonly` | `no` |
| `appenddirname` | `appendonlydir` |
| `appendfilename` | `appendonly.aof` |
| `appendfsync` | `everysec` |

### Tests

```bash
$ redis-cli CONFIG GET appendonly
1) "appendonly"
2) "no"
```

Each CONFIG GET returns a 2-element RESP array [name, value].

### Notes

- No actual persistence logic yet. Just store and return defaults.

---

## Stage 38 — AOF Options from Flags

| | |
|---|---|
| **Slug** | `vd9` |
| **Difficulty** | Easy |
| **Source** | [aof-02-vd9.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-02-vd9.md) |

### What to implement

Accept `--dir`, `--appendonly`, `--appenddirname`, `--appendfilename`, `--appendfsync` CLI flags. Override defaults.

### Tests

```bash
$ ./your_program.sh --appendonly yes --appenddirname mydir
$ redis-cli CONFIG GET appendonly
# Expects: ["appendonly", "yes"]
```

---

## Stage 39 — Create Append-Only Directory

| | |
|---|---|
| **Slug** | `fm0` |
| **Difficulty** | Easy |
| **Source** | [aof-03-fm0.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-03-fm0.md) |

### What to implement

When started with `--appendonly yes`, create the directory `<dir>/<appenddirname>/` at startup.

If `--appendonly` is not `yes`, do NOT create it.

### Tests

Verifies directory exists before any client commands are sent.

---

## Stage 40 — Create Append-Only File

| | |
|---|---|
| **Slug** | `dw4` |
| **Difficulty** | Easy |
| **Source** | [aof-04-dw4.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-04-dw4.md) |

### What to implement

Create an empty AOF file at startup: `<dir>/<appenddirname>/<appendfilename>.1.incr.aof`

### Tests

Verifies the file exists and is empty.

---

## Stage 41 — Create Manifest File

| | |
|---|---|
| **Slug** | `pb9` |
| **Difficulty** | Easy |
| **Source** | [aof-05-pb9.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-05-pb9.md) |

### What to implement

Create `<appendfilename>.manifest` alongside the AOF file. Contents:

```
file <appendfilename>.1.incr.aof seq 1 type i
```

### Tests

Verifies manifest file exists and contains the correct line (ending with newline).

---

## Stage 42 — Write a Single Command

| | |
|---|---|
| **Slug** | `dc8` |
| **Difficulty** | Hard |
| **Source** | [aof-06-dc8.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-06-dc8.md) |

### What to implement

When a write command (like SET) is processed, append it to the AOF file in RESP format.

```bash
$ redis-cli SET foo 100
```

AOF file gets: `*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\n100\r\n`

With `--appendfsync always`: flush to disk before responding to the client.

### Tests

- Tester creates its own manifest + AOF file with a custom filename
- Your server must **read the manifest** to find which file to write to
- Verifies command appears in the AOF file in valid RESP

### Notes

- Must read manifest to find filename — tester uses non-default names intentionally
- Only `always` fsync policy is tested

---

## Stage 43 — Write Multiple Commands

| | |
|---|---|
| **Slug** | `fi1` |
| **Difficulty** | Medium |
| **Source** | [aof-07-fi1.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-07-fi1.md) |

### What to implement

Append multiple commands in order. No separators between them — RESP framing is enough.

### Tests

Two SET commands → both appear in AOF file in order.

---

## Stage 44 — Filter Write Commands

| | |
|---|---|
| **Slug** | `ep6` |
| **Difficulty** | Easy |
| **Source** | [aof-08-ep6.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-08-ep6.md) |

### What to implement

Only **write commands** go to AOF. `PING`, `GET`, `ECHO`, `CONFIG GET` etc. are NOT logged.

### Tests

Mix of SET, GET, PING, ECHO commands → only SET commands appear in AOF file.

---

## Stage 45 — Replay a Single Command

| | |
|---|---|
| **Slug** | `xz2` |
| **Difficulty** | Hard |
| **Source** | [aof-09-xz2.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-09-xz2.md) |

### What to implement

On startup, if AOF directory exists:
1. Read manifest file
2. Find the `type i` entry to get the AOF filename
3. Parse the RESP-encoded commands from the file
4. Execute each command to restore state

### Tests

- Tester pre-creates AOF file with a SET command + manifest
- Starts your server
- Sends GET → expects the value from the AOF file

### Notes

- Reuse your existing RESP parser to read the file
- Must follow the manifest for the filename

---

## Stage 46 — Replay Multiple Commands

| | |
|---|---|
| **Slug** | `kn2` |
| **Difficulty** | Medium |
| **Source** | [aof-10-kn2.md](https://github.com/codecrafters-io/build-your-own-redis/blob/main/stage_descriptions/aof-10-kn2.md) |

### What to implement

Same as above, but the AOF file has multiple commands. Parse until EOF.

### Tests

Multiple SET commands in AOF → multiple GET commands verify all restored.

### Notes

- RESP is self-framing (`*n\r\n` tells you command length) — just keep parsing until EOF

---

# Official Stages Missing From This Document (Fetched in Order)

These are now expanded in the same detailed format as the first 46 stages.

## Stage 47 — RDB file config

| | |
|---|---|
| **Slug** | `zg5` |
| **Difficulty** | Easy |
| **Source** | [persistence-rdb-01-zg5.md](stage_descriptions/persistence-rdb-01-zg5.md) |

### What to implement

Welcome to the RDB Persistence Extension! In this extension, you'll add support for reading [RDB files](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/) (Redis Database files).

In this stage, you'll add support for two configuration parameters related to RDB persistence, as well as the [CONFIG GET](https://redis.io/docs/latest/commands/config-get/) command.

### RDB files

An RDB file is a point-in-time snapshot of a Redis dataset. When RDB persistence is enabled, the Redis server syncs its in-memory state with an RDB file, by doing the following:

1. On startup, the Redis server loads the data from the RDB file.
2. While running, the Redis server periodically takes new snapshots of the dataset, in order to update the RDB file.

### `dir` and `dbfilename`

The configuration parameters `dir` and `dbfilename` specify where an RDB file is stored:
- `dir` - the path to the directory where the RDB file is stored (example: `/tmp/redis-data`)
- `dbfilename` - the name of the RDB file (example: `rdbfile`)

### The `CONFIG GET` command

The [`CONFIG GET`](https://redis.io/docs/latest/commands/config-get/) command returns the values of configuration parameters.

It takes in one or more configuration parameters and returns a [RESP array](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays) of key-value pairs:

```bash
$ redis-cli CONFIG GET dir
1) "dir"
2) "/tmp/redis-data"
```

Although `CONFIG GET` can fetch multiple parameters at a time, the tester will only send `CONFIG GET` commands with one parameter at a time.


### Tests


The tester will execute your program like this:

```bash
./your_program.sh --dir /tmp/redis-files --dbfilename dump.rdb
```

It'll then send the following commands:

```bash
$ redis-cli CONFIG GET dir
$ redis-cli CONFIG GET dbfilename
```

Your server must respond to each `CONFIG GET` command with a RESP array containing two elements:

1. The parameter **name**, encoded as a [RESP Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings)
2. The parameter **value**, encoded as a RESP Bulk string

For example, if the value of `dir` is `/tmp/redis-files`, then the expected response to `CONFIG GET dir` is:

```bash
*2\r\n$3\r\ndir\r\n$16\r\n/tmp/redis-files\r\n
```


### Notes


- You don't need to read the RDB file in this stage, you only need to store `dir` and `dbfilename`. Reading from the file will be covered in later stages.
- If your repository was created before 5th Oct 2023, it's possible that your `./your_program.sh` script is not passing arguments to your program. To fix this, you'll need to edit `./your_program.sh`. Check the [update CLI args PR](https://github.com/codecrafters-io/build-your-own-redis/pull/89/files) for details on how to do this.

---

## Stage 48 — Read a key

| | |
|---|---|
| **Slug** | `jz6` |
| **Difficulty** | Medium |
| **Source** | [persistence-rdb-02-jz6.md](stage_descriptions/persistence-rdb-02-jz6.md) |

### What to implement

In this stage, you'll add support for reading a single key from an RDB file.

### RDB file format

<details>
  <summary>Click to expand/collapse</summary>
#### RDB file format overview

Here are the different sections of the RDB file, in order:

1.  Header section
2.  Metadata section
3.  Database section
4.  End of file section

RDB files use special encodings to store different types of data. The ones relevant to this stage are "size encoding" and "string encoding." These are explained near the end of this page.

The following breakdown of the RDB file format is based on [Redis RDB File Format](https://rdb.fnordig.de/file_format.html) by Jan-Erik Rediger. We’ve only included the parts that are relevant to this stage.

#### Header section

RDB files begin with a header section, which looks something like this:
```
52 45 44 49 53 30 30 31 31  // Magic string + version number (ASCII): "REDIS0011".
```

The header contains the magic string `REDIS`, followed by a four-character RDB version number. In this challenge, the test RDB files all use version 11. So, the header is always `REDIS0011`.

#### Metadata section

Next is the metadata section. It contains zero or more "metadata subsections," which each specify a single metadata attribute. Here's an example of a metadata subsection that specifies `redis-ver`:
```
FA                             // Indicates the start of a metadata subsection.
09 72 65 64 69 73 2D 76 65 72  // The name of the metadata attribute (string encoded): "redis-ver".
06 36 2E 30 2E 31 36           // The value of the metadata attribute (string encoded): "6.0.16".
```

The metadata name and value are always string encoded.

#### Database section

Next is the database section. It contains zero or more "database subsections," which each describe a single database. Here's an example of a database subsection:
```
FE                       // Indicates the start of a database subsection.
00                       /* The index of the database (size encoded).
                            Here, the index is 0. */

FB                       // Indicates that hash table size information follows.
03                       /* The size of the hash table that stores the keys and values (size encoded).
                            Here, the total key-value hash table size is 3. */
02                       /* The size of the hash table that stores the expires of the keys (size encoded).
                            Here, the number of keys with an expiry is 2. */
```

```
00                       /* The 1-byte flag that specifies the value’s type and encoding.
                            Here, the flag is 0, which means "string." */
06 66 6F 6F 62 61 72     // The name of the key (string encoded). Here, it's "foobar".
06 62 61 7A 71 75 78     // The value (string encoded). Here, it's "bazqux".
```

```
FC                       /* Indicates that this key ("foo") has an expire,
                            and that the expire timestamp is expressed in milliseconds. */
15 72 E7 07 8F 01 00 00  /* The expire timestamp, expressed in Unix time,
                            stored as an 8-byte unsigned long, in little-endian (read right-to-left).
                            Here, the expire timestamp is 1713824559637. */
00                       // Value type is string.
03 66 6F 6F              // Key name is "foo".
03 62 61 72              // Value is "bar".
```

```
FD                       /* Indicates that this key ("baz") has an expire,
                            and that the expire timestamp is expressed in seconds. */
52 ED 2A 66              /* The expire timestamp, expressed in Unix time,
                            stored as an 4-byte unsigned integer, in little-endian (read right-to-left).
                            Here, the expire timestamp is 1714089298. */
00                       // Value type is string.
03 62 61 7A              // Key name is "baz".
03 71 75 78              // Value is "qux".
```

Here's a more formal description of how each key-value pair is stored:

1. Optional expire information (one of the following):
    * Timestamp in seconds:
          1.  `FD`
          2.  Expire timestamp in seconds (4-byte unsigned integer)
    * Timestamp in milliseconds:
          1.  `FC`
          2.  Expire timestamp in milliseconds (8-byte unsigned long)
2. Value type (1-byte flag)
3. Key (string encoded)
4. Value (encoding depends on value type)

#### End of file section

This section marks the end of the file. It looks something like this:
```
FF                       /* Indicates that the file is ending,
                            and that the checksum follows. */
89 3b b7 4e f8 0f 77 19  // An 8-byte CRC64 checksum of the entire file.
```

#### Length encoding

Length-encoded values specify the length or size of something. Here are some examples:
- The database indexes and hash table sizes are length encoded.
- String encoding begins with a length-encoded value that specifies the number of characters in the string.
- List encoding begins with a length-encoded value that specifies the number of elements in the list.

The first (most significant) two bits of a length-encoded value indicate how the value should be parsed. Here's a guide (bits are shown in both hexadecimal and binary):
```
/* If the first two bits are 0b00:
   The length is the remaining 6 bits of the byte.
   In this example, the length is 10: */
0A
00001010

/* If the first two bits are 0b01:
   The length is the next 14 bits
   (remaining 6 bits in the first byte, combined with the next byte),
   in big-endian (read left-to-right).
   In this example, the length is 700: */
42 BC
01000010 10111100

/* If the first two bits are 0b10:
   Ignore the remaining 6 bits of the first byte.
   The length is the next 4 bytes, in big-endian (read left-to-right).
   In this example, the length is 17000: */
80 00 00 42 68
10000000 00000000 00000000 01000010 01101000

/* If the first two bits are 0b11:
   The remaining 6 bits specify a type of string encoding.
   See string encoding section. */
```

#### String encoding

A string-encoded value consists of two parts:
1.  The size of the string (size encoded).
2.  The string.

Here's an example:
```
/* The 0x0D size specifies that the string is 13 characters long.
   The remaining characters spell out "Hello, World!". */
0D 48 65 6C 6C 6F 2C 20 57 6F 72 6C 64 21
```

For sizes that begin with `0b11`, the remaining 6 bits indicate a type of string format:
```
/* The 0xC0 size indicates the string is an 8-bit integer.
   In this example, the string is "123". */
C0 7B

/* The 0xC1 size indicates the string is a 16-bit integer.
   The remaining bytes are in little-endian (read right-to-left).
   In this example, the string is "12345". */
C1 39 30

/* The 0xC2 size indicates the string is a 32-bit integer.
   The remaining bytes are in little-endian (read right-to-left),
   In this example, the string is "1234567". */
C2 87 D6 12 00

/* The 0xC3 size indicates that the string is compressed with the LZF algorithm.
   You will not encounter LZF-compressed strings in this challenge. */
C3 ...
```
</details>


### The `KEYS` command
<details>
  <summary>Click to expand/collapse</summary>

The [`KEYS command`](https://redis.io/docs/latest/commands/keys/) returns all the keys that match a given pattern, as a RESP array:
```
$ redis-cli SET foo bar
OK
$ redis-cli SET baz qux
OK
$ redis-cli KEYS "f*"
1) "foo"
```

When the pattern is `*`, the command returns all the keys in the database:
```
$ redis-cli KEYS "*"
1) "baz"
2) "foo"
```

In this stage, you must add support for the `KEYS` command. However, you only need to support the `*` pattern.
</details>


### Tests


The tester will create an RDB file with a single key and execute your program like this:
```
$ ./your_program.sh --dir <dir> --dbfilename <filename>
```

It'll then send a `KEYS "*"` command to your server.
```
$ redis-cli KEYS "*"
```

Your server must respond with a RESP array that contains the key from the RDB file:
```
*1\r\n$3\r\nfoo\r\n
```


### Notes


- The RDB file provided by `--dir` and `--dbfilename` might not exist. If the file doesn't exist, your program must treat the database as empty.
- RDB files use both little-endian and big-endian to store numbers. See the [MDN article on endianness](https://developer.mozilla.org/en-US/docs/Glossary/Endianness) to learn more.
- To generate an RDB file, use the [`SAVE` command](https://redis.io/docs/latest/commands/save/).

---

## Stage 49 — Read a string value

| | |
|---|---|
| **Slug** | `gc6` |
| **Difficulty** | Medium |
| **Source** | [persistence-rdb-03-gc6.md](stage_descriptions/persistence-rdb-03-gc6.md) |

### What to implement

In this stage, you'll add support for reading the value corresponding to a key from an RDB file.

Just like with previous stages, we'll stick to supporting RDB files that contain a single key for now.

The tester will create an RDB file with a single key and execute your program like this:

```
./your_program.sh --dir <dir> --dbfilename <filename>
```

It'll then send a `GET <key>` command to your server.

```bash
$ redis-cli GET "foo"
```

The response to `GET <key>` should be a RESP bulk string with the value of the key.

For example, let's say the RDB file contains a key called `foo` with the value `bar`. The expected response will be `$3\r\nbar\r\n`.

Strings can be encoded in three different ways in the RDB file format:

- Length-prefixed strings
- Integers as strings
- Compressed strings

In this stage, you only need to support length-prefixed strings. We won't cover the other two types in this challenge.

We recommend using [this blog post](https://rdb.fnordig.de/file_format.html) as a reference when working on this stage.

### Tests

Refer to the stage description for exact tester behavior.

### Notes

No additional notes for this stage.

---

## Stage 50 — Read multiple keys

| | |
|---|---|
| **Slug** | `jw4` |
| **Difficulty** | Medium |
| **Source** | [persistence-rdb-04-jw4.md](stage_descriptions/persistence-rdb-04-jw4.md) |

### What to implement

In this stage, you'll add support for reading multiple keys from an RDB file.

The tester will create an RDB file with multiple keys and execute your program like this:

```bash
$ ./your_program.sh --dir <dir> --dbfilename <filename>
```

It'll then send a `KEYS *` command to your server.

```bash
$ redis-cli KEYS "*"
```

The response to `KEYS *` should be a RESP array with the keys as elements.

For example, let's say the RDB file contains two keys: `foo` and `bar`. The expected response will be:

```
*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
```

- `*2\r\n` indicates that the array has two elements
- `$3\r\nfoo\r\n` indicates that the first element is a bulk string with the value `foo`
- `$3\r\nbar\r\n` indicates that the second element is a bulk string with the value `bar`

### Tests

Refer to the stage description for exact tester behavior.

### Notes

No additional notes for this stage.

---

## Stage 51 — Read multiple string values

| | |
|---|---|
| **Slug** | `dq3` |
| **Difficulty** | Medium |
| **Source** | [persistence-rdb-05-dq3.md](stage_descriptions/persistence-rdb-05-dq3.md) |

### What to implement

In this stage, you'll add support for reading multiple string values from an RDB file.

The tester will create an RDB file with multiple keys and execute your program like this:

```bash
$ ./your_program.sh --dir <dir> --dbfilename <filename>
```

It'll then send multiple `GET <key>` commands to your server.

```bash
$ redis-cli GET "foo"
$ redis-cli GET "bar"
```

The response to each `GET <key>` command should be a RESP bulk string with the value corresponding to the key.

### Tests

Refer to the stage description for exact tester behavior.

### Notes

No additional notes for this stage.

---

## Stage 52 — Read value with expiry

| | |
|---|---|
| **Slug** | `sm4` |
| **Difficulty** | Medium |
| **Source** | [persistence-rdb-06-sm4.md](stage_descriptions/persistence-rdb-06-sm4.md) |

### What to implement

In this stage, you'll add support for reading values that have an expiry set.

The tester will create an RDB file with multiple keys. Some of these keys will have an expiry set, and some won't. The expiry timestamps
will also be random, some will be in the past and some will be in the future.

The tester will execute your program like this:

```bash
$ ./your_program.sh --dir <dir> --dbfilename <filename>
```

It'll then send multiple `GET <key>` commands to your server.

```bash
$ redis-cli GET "foo"
$ redis-cli GET "bar"
```

When a key has expired, the expected response is `$-1\r\n` (a "null bulk string").

When a key hasn't expired, the expected response is a RESP bulk string with the value corresponding to the key.

### Tests

Refer to the stage description for exact tester behavior.

### Notes

No additional notes for this stage.

---

## Stage 53 — Configure listening port

| | |
|---|---|
| **Slug** | `bw1` |
| **Difficulty** | Easy |
| **Source** | [replication-01-bw1.md](stage_descriptions/replication-01-bw1.md) |

### What to implement

In this stage, you'll add support for starting the Redis server on a custom port.

### Leader-Follower Replication

The [leader-follower replication](https://redis.io/docs/latest/operate/oss_and_stack/management/replication/) is a pattern where one server (the "master") handles all write operations, and one or more servers (the "replicas") maintain copies of the master's data. When the master changes data, it automatically copies those changes to the replicas. This system provides data redundancy and improves read performance.

### Custom Port Support

Since replication requires running multiple Redis servers simultaneously, each instance needs its own port. This means a Redis server must be able to start on a port other than the default `6379`.

The `--port` flag passes the port number to the Redis server:

```bash
./your_program.sh --port <port_number>
```

The server then parses this argument and starts a TCP server on the specified port.

If you don’t provide a `--port` flag, the Redis server defaults to port `6379`.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port 6380
```

It'll then try to connect to your TCP server on the specified port number. If the connection succeeds, you'll pass this stage.


### Notes


- The tester will pass a random port number to your program, so you can't hardcode the port number from the example above.
- If your repository was created before 5th Oct 2023, it's possible that your `./your_program.sh` script
might not be passing arguments on to your program. You'll need to edit `./your_program.sh` to fix this, check
[this PR](https://github.com/codecrafters-io/build-your-own-redis/pull/89/files) for details.

---

## Stage 54 — The INFO command

| | |
|---|---|
| **Slug** | `ye5` |
| **Difficulty** | Easy |
| **Source** | [replication-02-ye5.md](stage_descriptions/replication-02-ye5.md) |

### What to implement

In this stage, you'll add support for responding to the [INFO](https://redis.io/commands/info/) command as a master server.

### The `INFO` Command

The `INFO` command returns information and statistics about a running Redis server. For example, a client can get information about a server like this:

```bash
$ redis-cli INFO
# Server
redis_version:7.2.4
...
# Clients
connected_clients:1
...
# Memory
used_memory:859944
...
# Replication
role:master
...
```

The server then responds with a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings), where each line is a key-value pair separated by a colon (`:`). The string can also contain section header lines (starting with `#`) and blank lines.

The `INFO` command also accepts an optional parameter to specify which section of information to display, such as `server`, `memory`, or `replication`. For this stage, we'll only focus on the `replication` section.

### The `replication` Section

When you run the `INFO` command with the `replication` argument, the server returns only the details concerning its replication setup:

```
$ redis-cli INFO replication
# Replication
role:master
connected_slaves:0
master_replid:8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb
master_repl_offset:0
second_repl_offset:-1
repl_backlog_active:0
repl_backlog_size:1048576
repl_backlog_first_byte_offset:0
repl_backlog_histlen:
```

Here are what some of the important fields mean:

- `role`: The role of the server (either `master` or `slave`).
- `connected_slaves`: The number of connected replica servers.
- `master_replid`: The replication ID of the master.
- `master_repl_offset`: The replication offset of the master.

In this stage, you'll only need to support the `role` key. We'll add support for other keys in later stages.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT>
```

It will then send the `INFO` command with `replication` as an argument.

```bash
$ redis-cli -p <PORT> info replication
```

Your server should respond with a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings) where each line
is a key value pair separated by a colon (`:`). The tester will only look for the `role` key and assert that the value is `master`.


### Notes


- In the response for the `INFO` command, you only need to support the `role` key for this stage. We'll add support for the other keys in later stages.
- The `# Replication` heading in the response is optional, and you can ignore it.
- The response to `INFO` needs to be encoded as a bulk string.
  - An example valid response would be `$11\r\nrole:master\r\n` (the string `role:master` encoded as a bulk string)

---

## Stage 55 — The INFO command on a replica

| | |
|---|---|
| **Slug** | `hc6` |
| **Difficulty** | Medium |
| **Source** | [replication-03-hc6.md](stage_descriptions/replication-03-hc6.md) |

### What to implement

In this stage, you'll extend the [INFO](https://redis.io/commands/info/) command to reflect a server's role as a replica.

### The `--replicaof` Flag

By default, your Redis server assumes the master role. When you pass the `--replicaof` flag, the server assumes the replica role instead.

For example:

```
./your_program.sh --port 6380 --replicaof "localhost 6379"
```

Here, we use the `--replicaof` flag to start a Redis server as a replica. The server will listen for connections on port `6380`, but it will also connect to a master (another Redis server) running on `localhost:6379` and replicate all its changes.

We'll learn more about how this replication works in later stages. 

For this stage, your primary task is to update the `INFO replication` command handler to check the server's configuration:

- If the user does not include the `--replicaof` flag, respond with `role:master`.
- If the user includes the `--replicaof` flag, respond with `role:slave`.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT> --replicaof "<MASTER_HOST> <MASTER_PORT>"
```

It will then send the `INFO` command with a `replication` argument to your server.

```bash
$ redis-cli -p <PORT> INFO replication
```

Your program should respond with a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings) where each line
is a key-value pair separated by a colon (`:`). The tester will only look for the `role` key and assert that the value is `slave`.


### Notes


- Your program still needs to pass the tests for previous stages, so if `--replicaof` isn't specified, you should default to the `master` role.
- Just like the previous stages, you only need to support the `role` key in the response for this stage. We'll add support for the other keys in later stages.
- You don't need to actually connect to the master server specified via `--replicaof` in this stage. We'll get to that in later stages.

---

## Stage 56 — Initial replication ID and offset

| | |
|---|---|
| **Slug** | `xc1` |
| **Difficulty** | Easy |
| **Source** | [replication-04-xc1.md](stage_descriptions/replication-04-xc1.md) |

### What to implement

In this stage, you'll extend your `INFO` command to return the `master_replid` and `master_repl_offset` values.

### The Replication ID and Offset

Every Redis master maintains two key pieces of information for managing replication: the **replication ID** and the **replication offset**.

The replication ID is a large pseudo-random string. This ID identifies the current history of the master's dataset. When a master server boots for the first time or restarts, it resets its ID.

The replication offset tracks the number of bytes of commands the master has streamed to its replicas. This value is used to update the state of the replicas with changes made to the dataset. The offset starts at `0` when a master boots up and no replicas have connected yet.

In this stage, you'll initialize a replication ID and offset for the master server:

- The ID can be any pseudo-random alphanumeric string of `40` characters.
  - For this challenge, you don't need to generate a random string. You can hardcode it instead.
  - As an example, you can hardcode `8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb` as the replication ID.
- The offset should be `0`.

These two values should be returned as part of the `INFO` command output, under the `master_replid` and `master_repl_offset` keys, respectively.


### Tests


The tester will execute your program like this:

```
./your_program.sh
```

It will then send the `INFO` command with the `replication` option to your server.

```bash
$ redis-cli INFO replication
```

Your program should respond with a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings) where each line is a key-value pair separated by a colon (`:`). The tester will look for the following key-value pairs:

- `role`: `master`
- `master_replid`: A 40-character alphanumeric string
- `master_repl_offset`: `0`


### Notes


- Your code must pass the tests for previous stages, meaning you should still return the correct `role` key.

---

## Stage 57 — Send handshake (1/3)

| | |
|---|---|
| **Slug** | `gl7` |
| **Difficulty** | Easy |
| **Source** | [replication-05-gl7.md](stage_descriptions/replication-05-gl7.md) |

### What to implement

In this stage, you'll implement the first step of the replication handshake.

### Handshake

When a replica connects to a master, it needs to go through a "handshake" before receiving updates from the master.

There are three steps to this handshake:

1. The replica sends a `PING` to the master.
2. The replica sends `REPLCONF` twice to the master.
3. The replica sends `PSYNC` to the master.

We'll learn more about `REPLCONF` and `PSYNC` in later stages. For now, we'll focus on the first part of the handshake.

When your server starts in replica mode, it must connect to the specified master host and port, and then send the `PING` command.

The `PING` command must be sent encoded as a [RESP array](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays).


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT> --replicaof "<MASTER_HOST> <MASTER_PORT>"
```

It will then assert that the replica connects to the master and sends the `PING` command as a RESP array (`*1\r\n$4\r\nPING\r\n`).

### Notes

No additional notes for this stage.

---

## Stage 58 — Send handshake (2/3)

| | |
|---|---|
| **Slug** | `eh4` |
| **Difficulty** | Easy |
| **Source** | [replication-06-eh4.md](stage_descriptions/replication-06-eh4.md) |

### What to implement

In this stage, you'll implement the second step of the replication handshake.

### Handshake (Recap)

As a recap, there are three steps to the handshake:

1. The replica sends a `PING` to the master (Handled in previous stages)
2. The replica sends `REPLCONF` twice to the master
3. The replica sends `PSYNC` to the master

For this stage, you'll handle the second step of this process.

### The `REPLCONF` Command

The `REPLCONF` command is used to configure a connected replica. After receiving a response to `PING`, the replica sends two `REPLCONF` commands one-by-one, waiting for a response after each.

1. `REPLCONF listening-port <PORT>`: This tells the master which port the replica is listening on. This value is used for [monitoring and logging](https://github.com/redis/redis/blob/90178712f6eccf1e5b61daa677c5c103114bda3a/src/replication.c#L107-L130), not for replication itself.
2. `REPLCONF capa psync2`: This notifies the master of the replica's capabilities.
   - `capa` stands for "capabilities". It indicates that the next argument is a feature the replica supports.
   - `psync2` signals that the replica supports the PSYNC2 protocol. PSYNC2 is an improved version of the [partial synchronization](https://redis.io/docs/latest/operate/oss_and_stack/management/replication/) feature used to resynchronize a replica with its master.
   - You can safely hardcode `capa psync2` for now.

Both commands should be sent as RESP arrays, so the exact bytes will look something like this:

```
# REPLCONF listening-port <PORT>
*3\r\n$8\r\nREPLCONF\r\n$14\r\nlistening-port\r\n$4\r\n6380\r\n

# REPLCONF capa psync2
*3\r\n$8\r\nREPLCONF\r\n$4\r\ncapa\r\n$6\r\npsync2\r\n
```

For both commands, the master will respond with `+OK\r\n`. That's the string `OK` encoded as a [simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings). The replica should wait for this response before sending the next command.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT> --replicaof "<MASTER_HOST> <MASTER_PORT>"
```

It will then assert that the replica connects to the master and sends the following:

1. The `PING` command
2. The `REPLCONF` command with `listening-port` and `<PORT>` as the arguments
3. The `REPLCONF` command with `capa psync2` as the arguments

**Notes**

- The response to `REPLCONF` will always be `+OK\r\n`.

### Notes

No additional notes for this stage.

---

## Stage 59 — Send handshake (3/3)

| | |
|---|---|
| **Slug** | `ju6` |
| **Difficulty** | Medium |
| **Source** | [replication-07-ju6.md](stage_descriptions/replication-07-ju6.md) |

### What to implement

In this stage, you'll implement the third step of the replication handshake.

### Handshake (Recap)

As a recap, there are three steps to the handshake:

- The replica sends a `PING` to the master (Handled in earlier stages)
- The replica sends `REPLCONF` twice to the master (Handled in previous stages)
- The replica sends `PSYNC` to the master

### The `PSYNC` Command

After receiving a response to the second `REPLCONF`, the replica sends a [`PSYNC`](https://redis.io/commands/psync/) command to the master. 

The `PSYNC` command is used to synchronize the state of the replica with the master. The command format is:

```bash
PSYNC <replication_id> <offset>
```

The command takes two arguments: the master's current replication ID and the replica's current offset.

For the replica's first connection to the master:

- The replication ID will be `?` because the replica doesn't know the master's ID yet.
- The offset will be `-1` since the replica has no data from the master yet.

So the final command sent will be `PSYNC ? -1`, encoded as a RESP array:

```
*3\r\n$5\r\nPSYNC\r\n$1\r\n?\r\n$2\r\n-1\r\n
```

The master will respond with a [simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings) that looks like this:

```
+FULLRESYNC <REPL_ID> 0\r\n
```

You can ignore this response for now. We'll get to handling it in later stages.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT> --replicaof "<MASTER_HOST> <MASTER_PORT>"
```

It will then assert that the replica connects to the master and sends the following commands:

1. `PING`
2. `REPLCONF` with `listening-port` and `<PORT>` as arguments
3. `REPLCONF` with `capa psync2` as arguments
4. `PSYNC` with `? -1` as arguments

### Notes

No additional notes for this stage.

---

## Stage 60 — Receive handshake (1/2)

| | |
|---|---|
| **Slug** | `fj0` |
| **Difficulty** | Easy |
| **Source** | [replication-08-fj0.md](stage_descriptions/replication-08-fj0.md) |

### What to implement

In this stage, we'll start implementing support for receiving a replication handshake as a master.

### Handshake (Recap)

Up until now, we've been implementing the handshake from the replica's perspective. Now we'll implement the same handshake on the master side.

As a recap, the master receives the following from the replica during the handshake:

1. A `PING` command
2. Two `REPLCONF` commands
3. A `PSYNC` command

Your Redis server already supports the `PING` command, so there's no additional work to do for the first step.

In this stage, you'll add support for receiving the two `REPLCONF` commands as a master.

For the purposes of this challenge, you can safely ignore the arguments for both commands and simply respond with `+OK\r\n`. That's the string `OK` encoded as a [simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings)


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT>
```

It will then send the following commands:

1. `PING` — expecting `+PONG\r\n`
2. `REPLCONF listening-port <PORT>` — expecting `+OK\r\n`
3. `REPLCONF capa psync2` — expecting `+OK\r\n` 

### Notes

No additional notes for this stage.

---

## Stage 61 — Receive handshake (2/2)

| | |
|---|---|
| **Slug** | `vm3` |
| **Difficulty** | Easy |
| **Source** | [replication-09-vm3.md](stage_descriptions/replication-09-vm3.md) |

### What to implement

In this stage, you'll add support for receiving the [`PSYNC`](https://redis.io/commands/psync/) command from the replica.

### Handshake (Recap)

As a recap, the master receives the following for the handshake:

1. A `PING` from the replica
2. `REPLCONF` twice from the replica
3. `PSYNC` from the replica

After the replica sends `REPLCONF` twice, it will send a `PSYNC` command with the arguments `? -1` to the master:

- The replication ID is `?` because the replica doesn't know the master's ID yet.
- The offset is `-1` since the replica has no data from the master yet.

The final command you'll receive will look something like this:

```
*3\r\n$5\r\nPSYNC\r\n$1\r\n?\r\n$2\r\n-1\r\n
```

That's `["PSYNC", "?", "-1"]` encoded as a RESP array.

The master needs to respond with `+FULLRESYNC <REPL_ID> 0\r\n`, which is `FULLRESYNC <REPL_ID> 0` encoded as a simple string. Here's what the response means:

- `FULLRESYNC` means that the master cannot perform an incremental update to the replica, and will start a full resynchronization.
- `<REPL_ID>` is the replication ID of the master.
- `0` is the replication offset of the master.

For example, if your replication ID is `8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb`, you'd respond with:
```bash
+FULLRESYNC 8371b4fb1155b71f4a04d3e1bc3e18c4a990aeeb 0\r\n
```


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT>
```

It will then connect to your TCP server as a replica and send the following commands:

1. `PING` - expecting `+PONG\r\n` back
2. `REPLCONF listening-port <PORT>` - expecting `+OK\r\n` back
3. `REPLCONF capa psync2` - expecting `+OK\r\n` back
4. `PSYNC ? -1` - expecting `+FULLRESYNC <REPL_ID> 0\r\n` back


### Notes


- In the response, `<REPL_ID>` needs to be replaced with the replication ID of the master, which you've initialized in previous stages.

---

## Stage 62 — Empty RDB transfer

| | |
|---|---|
| **Slug** | `cf8` |
| **Difficulty** | Easy |
| **Source** | [replication-10-cf8.md](stage_descriptions/replication-10-cf8.md) |

### What to implement

In this stage, you'll add support for sending an empty RDB file as a master.

### Full Resynchronization

When a replica connects to a master for the first time, it sends a `PSYNC ? -1` command. This is the replica's way of telling the master that it doesn't have any data yet and needs to be fully resynchronized.

The master responds in two steps:

- It acknowledges with a `FULLRESYNC` response (Handled in previous stages)
- It sends a snapshot of its current state as an [RDB file](https://rdb.fnordig.de/file_format.html).

The replica is expected to load the file into memory and replace its current state with the master's data.

For this challenge, you don’t need to build an RDB file yourself. Instead, you can hardcode an empty RDB file, since we’ll assume the master’s database is always empty.

You can find the hex and base64 representation of an empty RDB file [here](https://github.com/codecrafters-io/redis-tester/blob/main/internal/assets/empty_rdb_hex.md). You need to decode these into binary contents before sending them to the replica.

The file is sent using the following format:

```
$<length_of_file>\r\n<binary_contents_of_file>
```

This is similar to how [bulk strings](https://redis.io/topics/protocol#resp-bulk-strings) are encoded, but **without the trailing `\r\n`**.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT>
```

It will then connect to your TCP server as a replica and execute the following commands:

1. `PING` - expecting `+PONG\r\n`
2. `REPLCONF listening-port <PORT>` - expecting `+OK\r\n`
3. `REPLCONF capa eof capa psync2` - expecting `+OK\r\n`
4. `PSYNC ? -1` - expecting `+FULLRESYNC <REPL_ID> 0\r\n`

After the last response, the tester will expect to receive an empty RDB file from your server.

The tester will accept any valid RDB file that is empty.


### Notes


- The RDB file should be sent like this: `$<length>\r\n<contents>`
  - `<length>` is the length of the file in bytes
  - `<contents>` is the binary contents of the file
  - Note that this is NOT a RESP bulk string and doesn't contain a `\r\n` at the end.
- If you want to learn more about the RDB file format, read [this blog post](https://rdb.fnordig.de/file_format.html). This challenge
  has a separate extension dedicated to reading RDB files.

---

## Stage 63 — Single-replica propagation

| | |
|---|---|
| **Slug** | `zn8` |
| **Difficulty** | Medium |
| **Source** | [replication-11-zn8.md](stage_descriptions/replication-11-zn8.md) |

### What to implement

In this stage, you'll add support for propagating write commands to a single replica as a master.

### Command Propagation

After the replication handshake is complete and the master has sent the RDB file to the replica, the master starts propagating "write" commands to the replica.

Write commands are commands that modify the master's dataset, such as `SET` and `DEL`. Commands like `PING`, `ECHO`, etc., are not considered "write" commands, so they aren't propagated.

### The Propagation Process

Command propagation happens over the replication connection. This is the same connection that was used for the handshake.

The propagated commands are sent as RESP arrays. For example, if the master receives `SET foo bar` as a command from a client, it'll send `*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n` to all connected replicas over their respective replication connections.

Replicas process commands received over the connection just like they would process commands received from a client, but with one difference: they don't send responses back to the master. They just process the command silently and update their state.

Similarly, the master doesn't wait for a response from the replica when propagating commands. It just sends the commands as they come in.

There is one exception to this "no response" rule: the `REPLCONF GETACK` command. We'll learn about this command in later stages.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT>
```

It will then connect to your TCP server as a replica and complete the full handshake sequence covered in previous stages.

The tester will then wait for your server to send an RDB file.

Once the RDB file is received, the tester will send a series of write commands to your program (as a separate Redis client).

```bash
$ redis-cli SET foo 1
$ redis-cli SET bar 2
$ redis-cli SET baz 3
```

It will then assert that these commands were propagated to the replica in the correct order. The tester will expect to receive these commands: 

- Encoded as RESP arrays.
- Sent on the same connection used for the handshake (replication connection).


### Notes


- Although replicas provide a `listening-port` during the handshake, it’s used only for [monitoring/logging purposes](https://github.com/redis/redis/blob/90178712f6eccf1e5b61daa677c5c103114bda3a/src/replication.c#L107-L130), not for propagation. Redis propagates commands over the same TCP connection that the replica initiated during the handshake.
- A true implementation would buffer the commands so that they can be sent to the replica after it loads the RDB file. For the purposes of this challenge, you can assume that the replica is ready to receive commands immediately after receiving the RDB file.

---

## Stage 64 — Multi-replica propagation

| | |
|---|---|
| **Slug** | `hd5` |
| **Difficulty** | Hard |
| **Source** | [replication-12-hd5.md](stage_descriptions/replication-12-hd5.md) |

### What to implement

In this stage, you'll extend your implementation of the master to support propagating commands to multiple replicas.

### Command Propagation (Recap)

Once a replica completes the handshake and loads the RDB file, it is ready to start receiving live updates from the master. 

Every write command executed on the master must be forwarded to all connected replicas, not just one. This ensures that every replica stays in sync with the master.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT>
```

It will then start **multiple** replicas that will each connect to your server, complete the handshake, and receive the initial RDB file.

Next, the tester will send `SET` commands to the master from a separate client.

```bash
$ redis-cli SET foo 1
$ redis-cli SET bar 2
$ redis-cli SET baz 3
```

It will then assert that each replica received those commands in the correct order.

### Notes

No additional notes for this stage.

---

## Stage 65 — Command processing

| | |
|---|---|
| **Slug** | `yg4` |
| **Difficulty** | Hard |
| **Source** | [replication-13-yg4.md](stage_descriptions/replication-13-yg4.md) |

### What to implement

In this stage, you'll implement the processing of propagated commands as a replica.

### Command Processing

After the replica receives a command from the master, it processes it and applies it to its own state. This will work exactly like a regular command sent by a client. The key difference is that the replica **must not send a response** back to the master.

For example, if a master propagates `SET foo 1` to a replica:

- The replica must update its database to set the value of `foo` to `1`.
- Unlike commands from regular clients, the replica does not reply with `+OK\r\n`.


### Tests


The tester will spawn a Redis master and execute your program as a replica like this:

```
./your_program.sh --port <PORT> --replicaof "<MASTER_HOST> <MASTER_PORT>"
```

Just like in the previous stages, your replica should complete the handshake with the master and receive an empty RDB file.

Once the RDB file is received, the master will propagate a series of write commands to your program:

```bash
SET foo 1 # propagated from master to replica
SET bar 2 # propagated from master to replica
SET baz 3 # propagated from master to replica
```

The tester will then issue `GET` commands to your program to check if the commands were processed correctly.

```bash
$ redis-cli GET foo # expecting `1` back
$ redis-cli GET bar # expecting `2` back
# ... and so on
```


### Notes


- The propagated commands are sent as RESP arrays. So the command `SET foo 1` will be sent as `*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$1\r\n1\r\n`.
- It is **not** guaranteed that propagated commands will be sent one at a time. One TCP segment might contain bytes for multiple commands.

---

## Stage 66 — ACKs with no commands

| | |
|---|---|
| **Slug** | `xv6` |
| **Difficulty** | Easy |
| **Source** | [replication-14-xv6.md](stage_descriptions/replication-14-xv6.md) |

### What to implement

In this stage, you'll implement support for responding to the `REPLCONF GETACK` command as a replica.

### ACKs

Normally, a replica processes propagated commands silently. However, the master needs a way to verify that a replica is "in sync" and hasn't fallen behind. This is done using ACKs (acknowledgements).

Redis masters periodically ask replicas to send ACKs to check how much of the replication stream they’ve processed.

### The `REPLCONF GETACK` command

When the master wants an update, it sends the command:

```bash
REPLCONF GETACK *
```

The exact command received by the replica will look something like this: `*3\r\n$8\r\nreplconf\r\n$6\r\ngetack\r\n$1\r\n*\r\n`. That's `["replconf", "getack", "*"]` encoded as a [RESP array](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays).

The replica receives this command over the replication connection (i.e., the connection used for the replication handshake) and responds with:

```bash
REPLCONF ACK <offset>
```

The offset is the number of bytes of commands processed by the replica. For this stage, you can hardcode the offset to `0`. We'll learn how to track offsets and update them in later stages.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT> --replicaof "<HOST> <PORT>"
```

Just like in the previous stages, your replica should complete the handshake with the master and receive an empty RDB file.

The tester will then send `REPLCONF GETACK *` to your replica. 

It will expect to receive `REPLCONF ACK 0` encoded as a RESP array (`*3\r\n$8\r\nREPLCONF\r\n$3\r\nACK\r\n$1\r\n0\r\n`).


### Notes


- After the master-replica handshake is complete, a replica should **only** send responses to `REPLCONF GETACK` commands. All other propagated commands (like `PING`, `SET`, etc.) should be read and processed, but a response should not be sent back to the master.

---

## Stage 67 — ACKs with commands

| | |
|---|---|
| **Slug** | `yd3` |
| **Difficulty** | Medium |
| **Source** | [replication-15-yd3.md](stage_descriptions/replication-15-yd3.md) |

### What to implement

In this stage, you'll extend your `REPLCONF GETACK` implementation to respond with the number of bytes of commands processed by the replica.

### ACKs (Recap)

As a recap, a master uses ACKs to verify that its replicas are in sync with it and haven't fallen behind. Each ACK contains an offset — the number of bytes of commands processed by the replica.

### Offset tracking

A replica keeps its offset updated by tracking the total byte size of every command received from its master. This includes both write commands (like `SET`, `DEL`) and non-write commands (like `PING`, `REPLCONF GETACK *`).

After processing the received command (e.g., `["SET", "foo", "bar]`), it adds the full RESP array byte length to its running offset.

An important rule for this process is that the offset should only include commands processed **before** the current `REPLCONF GETACK *` request.

For example:

- A replica connects, completes the handshake, and the master sends `REPLCONF GETACK *`.
  - The replica responds with `REPLCONF ACK 0` since no commands had been processed before this request.
- Next, the master sends another `REPLCONF GETACK *`.
  - The replica responds with `REPLCONF ACK 37`, because the previous `REPLCONF` command consumed 37 bytes.
- The master then sends a `PING` command.
  - The replica silently processes it, increments its offset by 14, and sends no response.
- The next `REPLCONF GETACK *` arrives.
  - The replica responds with `REPLCONF ACK 88` — that’s 37 (for the first `REPLCONF`), +37 (for the second `REPLCONF`), +14 (for the `PING`).

Notice that the current `GETACK` request itself is not included in the offset value.


### Tests


The tester will execute your program like this:

```
./your_program.sh --port <PORT> --replicaof "<HOST> <PORT>"
```

Just like in the previous stages, your replica should complete the handshake with the master and receive an empty RDB file.

The master will then propagate a series of commands to your replica. These commands will be interleaved with `REPLCONF GETACK *` commands.

```bash
REPLCONF GETACK *    # expect: REPLCONF ACK 0

PING                 # replica processes silently
REPLCONF GETACK *    # expect: REPLCONF ACK 51
# 51 = 37 (first REPLCONF) + 14 (PING)

SET foo 1             # replica processes silently
SET bar 2             # replica processes silently
REPLCONF GETACK *    # expect: REPLCONF ACK 146
# 146 = 51 + 37 (second REPLCONF) + 29 (SET foo) + 29 (SET bar)
```

Your replica must calculate and return the exact offset at each step in the `REPLCONF ACK <offset>` response. Your response should also be encoded as a [RESP array](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays).


### Notes


- The offset should only include the number of bytes of commands processed **before** receiving the current `REPLCONF GETACK` command.
- Although masters don't propagate `PING` commands when received from clients (since they aren't "write" commands), they may send `PING` commands to replicas to notify replicas that the master is still alive.

---

## Stage 68 — WAIT with no replicas

| | |
|---|---|
| **Slug** | `my8` |
| **Difficulty** | Medium |
| **Source** | [replication-16-my8.md](stage_descriptions/replication-16-my8.md) |

### What to implement

In this stage, you’ll implement support for the `WAIT` command on the master.

### The `WAIT` command

The `WAIT` command is used to check how many replicas have acknowledged all previous write commands. This allows a client to measure the durability of a write command before considering it successful.

The command format is:

```bash
WAIT <numreplicas> <timeout>
```

Here's what each argument means:

- `<numreplicas>`: The minimum number of replicas that must acknowledge all previous write commands.
- `<timeout>`: The maximum time (in milliseconds) the client is willing to wait.

For example:

```bash
$ redis-cli WAIT 3 5000
(integer) 2
```

Here, the client is asking the master to wait for `3` replicas (with a maximum timeout of 5000 ms). After the timeout passes, the master has only `2` replicas connected, so it immediately replies with `2` as a [RESP integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integers).

For now, we’ll handle the simplest case: when the client needs `0` replicas and the master also has no replicas connected. In this case, `WAIT` should immediately return `0`.

We'll get to tracking the number of replicas and responding accordingly in later stages.


### Tests


The tester will execute your program like this:

```
./your_program.sh
```

It will then connect to your master and send:

```bash
$ redis-cli WAIT 0 60000
```

The tester will expect to receive `0` immediately (as a RESP integer), since no replicas are connected.

### Notes

No additional notes for this stage.

---

## Stage 69 — WAIT with no commands

| | |
|---|---|
| **Slug** | `tu8` |
| **Difficulty** | Medium |
| **Source** | [replication-17-tu8.md](stage_descriptions/replication-17-tu8.md) |

### What to implement

In this stage, you’ll extend your `WAIT` implementation to handle the case where replicas are connected, but no commands have been sent.

### `WAIT` with connected replicas

In previous stages, we handled the case where no replicas were connected, and the master could safely return `0`.

Now, we’ll consider the case where some replicas are connected. Each replica will have completed the handshake and received the empty RDB file. But since no write commands have been sent yet, the replication offset is still `0`.

In this situation, the master will return the number of connected replicas, since it knows they are all in sync at offset `0`:

```bash
$ redis-cli WAIT 3 500
(integer) 7
$ redis-cli WAIT 7 500
(integer) 7
$ redis-cli WAIT 9 500
(integer) 7
```

In the example above, `7` replicas are connected. No matter how many replicas the client asks for, the master will reply with the number of connected replicas (`7`).

For this stage, you can ignore both arguments (`<numreplicas> <timeout>`) and simply return the number of connected replicas.


### Tests


The tester will execute your program as a master like this:

```
./your_program.sh
```

It will then start **multiple** replicas that connect to your server. Each will complete the handshake and expect to receive an empty RDB file.

It will then connect to your master as a client and send commands like this:

```bash
$ redis-cli WAIT 3 500 # (expecting 7 back)
$ redis-cli WAIT 7 500 # (expecting 7 back)
$ redis-cli WAIT 9 500 # (expecting 7 back)
```

The response to each of these commands should be encoded as a RESP integer (i.e., `:7\r\n`).


### Notes


- Even if `WAIT` is called with a number less than the number of connected replicas, the master should return the count of connected replicas.
- The number of replicas created in this stage will be random, so you can't hardcode `7` as the response, like in the example above.

---

## Stage 70 — WAIT with multiple commands

| | |
|---|---|
| **Slug** | `na2` |
| **Difficulty** | Hard |
| **Source** | [replication-18-na2.md](stage_descriptions/replication-18-na2.md) |

### What to implement

In this stage, you’ll extend your `WAIT` implementation to handle the case where replicas are connected and have received write commands.

### `WAIT` with propagated commands

In previous stages, we handled the cases where:

- No replicas were connected, and the master could safely return `0`.
- Replicas were connected, but hadn't received any write commands.

Now, we’ll handle the case where write commands have been sent to replicas. Since replication offsets are no longer `0`, the master needs to check which replicas have successfully processed the latest write command before replying.

To do this, the master must send `REPLCONF GETACK *` to replicas if there are pending write commands since the last `WAIT`. Each replica will reply with its current offset (`REPLCONF ACK <offset>`).

The `WAIT` command should complete when either:

- The required number of replicas has acknowledged all previous write commands, or
- The timeout expires.

Either way, the master should return the number of replicas that acknowledged all previous write commands as a [RESP integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integers).


### Tests


The tester will execute your program as a master like this:

```
./your_program.sh
```

It will then start **multiple** replicas that connect to your server. Each will complete the handshake and expect to receive an empty RDB file.

Next, the tester will connect to your master as a client and send multiple write commands interleaved with `WAIT` commands:

```bash
$ redis-cli SET foo 123
$ redis-cli WAIT 1 500    # (must wait until either 1 replica has processed previous commands or 500ms have passed)

$ redis-cli SET bar 456
$ redis-cli WAIT 2 500    # (must wait until either 2 replicas have processed previous commands or 500ms have passed)
```


### Notes


- The returned number of replicas might be less than or greater than the expected number of replicas specified in the `WAIT` command.

---

## Stage 71 — The TYPE command

| | |
|---|---|
| **Slug** | `cc3` |
| **Difficulty** | Easy |
| **Source** | [streams-01-cc3.md](stage_descriptions/streams-01-cc3.md) |

### What to implement

In this stage, you'll add support for the `TYPE` command.

### The `TYPE` command

The [TYPE](https://redis.io/commands/type/) command returns the type of value stored at a given key. These types include: `string`, `list`, `set`, `zset`, `hash`, `stream`, and `vectorset`.

Here's an example:

```bash
# Set a key to a string value
$ redis-cli SET some_key "foo"
"OK"

# Check the type of value at the key
$ redis-cli TYPE some_key
"string"
```

The return value is encoded as a [simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings).

If a key doesn't exist, the return value will be `none`.

```bash
$ redis-cli TYPE missing_key
"none"
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `SET` command to your server to create a key with a string value.

```bash
$ redis-cli SET some_key "foo"
```

Next, it will send a `TYPE` command for that key.

```bash
$ redis-cli TYPE some_key
```

Your server should respond with `+string\r\n`, which is `string` encoded as a [simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings).

Next, the tester will send another `TYPE` command with a missing key.

```bash
$ redis-cli TYPE missing_key
```

Your server should respond with `+none\r\n`, which is `none` encoded as a [simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings).


### Notes


- For now, you only need to handle the `string` and `none` types. We'll add support for the `stream` type in later stages.

---

## Stage 72 — Create a stream

| | |
|---|---|
| **Slug** | `cf6` |
| **Difficulty** | Medium |
| **Source** | [streams-02-cf6.md](stage_descriptions/streams-02-cf6.md) |

### What to implement

In this stage, you'll add support for creating [Redis streams](https://redis.io/docs/latest/develop/data-types/streams/) using the `XADD` command.

### Redis Streams & Entries

A [Redis stream](https://redis.io/docs/latest/develop/data-types/streams/) is used to store a sequence of entries in chronological order at a given key. Each entry consists of a unique ID and one or more key-value pairs.

For example, if you were using a Redis stream to store real-time data from a temperature & humidity monitor, the stream might look like this:

```yaml
entries:
  - id: 1526985054069-0 # (ID of the first entry)
    temperature: 36 # (A key-value pair in the first entry)
    humidity: 95 # (Another key-value pair)

  - id: 1526985054079-0 # (ID of the second entry)
    temperature: 37 # (A key-value pair in the second entry)
    humidity: 94 # (Another key-value pair)

  # ... (and so on)
```

We’ll take a closer look at how entry IDs (like `1526985054069-0`) are structured in later stages.

### The `XADD` command

The [`XADD`](https://redis.io/commands/xadd/) command appends an entry to a stream. If the stream doesn't exist, it is created automatically.

The `XADD` command accepts a stream key, an entry ID, and one or more key-value pairs as arguments:

```bash
$ redis-cli XADD stream_key 1526919030474-0 temperature 36 humidity 95
"1526919030474-0"
```

The return value is the ID of the newly added entry as a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).

`XADD` supports other [optional arguments](https://redis.io/docs/latest/commands/xadd/#optional-arguments), but we won't deal with them in this challenge.

`XADD` also supports auto-generated entry IDs, but for this stage, you'll only deal with explicit IDs (like `1526919030474-0`).


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It'll then send an `XADD` command to your server and expect the ID as a response. For example, it might send:

```bash
$ redis-cli XADD stream_key 0-1 foo bar
"0-1"
```

In this case, your server should respond with `$3\r\n0-1\r\n`, which is `0-1` encoded as a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).

Next, the tester will send a `TYPE` command to your server to verify the key's type.

```bash
$ redis-cli TYPE stream_key
"stream"
```

Your server should respond with `+stream\r\n`, which is `stream` encoded as a [simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-strings).


### Notes


- You still need to handle the `string` and `none` return values for the `TYPE` command. `stream` should only be returned for keys that are streams.

---

## Stage 73 — Validating entry IDs

| | |
|---|---|
| **Slug** | `hq8` |
| **Difficulty** | Easy |
| **Source** | [streams-03-hq8.md](stage_descriptions/streams-03-hq8.md) |

### What to implement

In this stage, you'll add support for validating entry IDs to the `XADD` command.

### Entry IDs

Entry IDs are crucial for maintaining the order of entries in Redis streams.

Each ID is made up of two integers separated by a dash: `<millisecondsTime>-<sequenceNumber>`.

For example:

```yaml
entries:
  - id: 1526985054069-0 # (ID of the first entry)
    temperature: 36
    humidity: 95

  - id: 1526985054079-0 # (ID of the second entry)
    temperature: 37
    humidity: 94

  # ... (and so on)
```

These IDs are unique within a stream and are guaranteed to be incremental. This means a new entry's ID will always be greater than the ID of any previous entry.

### Specifying Entry IDs in `XADD`

You can use three different formats to specify the ID for the `XADD` command:

- Explicit (`1526919030474-0`)
- Auto-generate only sequence number (`1526919030474-*`)
- Auto-generate the time and sequence number (`*`)

For this stage, you will only handle explicit IDs (e.g., `1526919030474-0`). You'll add support for the other two cases in the later stages.

Your `XADD` implementation must validate the provided ID based on the following rules:

- The ID must be strictly greater than the last entry's ID.
  - The `millisecondsTime` portion of the new ID must be greater than or equal to the last entry's `millisecondsTime`.
  - If the `millisecondsTime` values are equal, the `sequenceNumber` of the new ID must be greater than the last entry's `sequenceNumber`.
- If the stream is empty, the ID must be greater than `0-0`. The minimum valid ID Redis accepts is `0-1`.

Here's an example of adding an entry with a valid ID followed by an invalid ID:

```bash
$ redis-cli XADD some_key 1-1 foo bar
"1-1"
$ redis-cli XADD some_key 1-1 bar baz
(error) ERR The ID specified in XADD is equal or smaller than the target stream top item
```

The second command fails because `1-1` is not strictly greater than the last ID.

Here's another example:

```bash
$ redis-cli XADD some_key 1-1 foo bar
"1-1"
$ redis-cli XADD some_key 0-2 bar baz
(error) ERR The ID specified in XADD is equal or smaller than the target stream top item
```

The ID `0-2` is invalid because its `millisecondsTime` is less than the last ID's `millisecondsTime`.

Finally, passing `0-0` is always invalid, since IDs must be strictly greater than `0-0`:

```bash
$ redis-cli XADD some_key 0-0 bar baz
(error) ERR The ID specified in XADD must be greater than 0-0
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then create a few entries using `XADD`.

```bash
$ redis-cli XADD stream_key 1-1 foo bar
"1-1"
$ redis-cli XADD stream_key 1-2 bar baz
"1-2"
```

Next, it will send a few `XADD` commands with an invalid ID, such as `1-2` or `0-3`. 

```bash
# The exact time and sequence number as the last entry
$ redis-cli XADD stream_key 1-2 baz foo
(error) ERR The ID specified in XADD is equal or smaller than the target stream top item

# A smaller value for the time and a larger value for the sequence number
$ redis-cli XADD stream_key 0-3 baz foo
(error) ERR The ID specified in XADD is equal or smaller than the target stream top item
```

In each case, your server should respond with `-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n\`, encoded as a
[simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-errors).

After that, the tester will send another `XADD` command with `0-0` as the ID.

```bash
$ redis-cli XADD stream_key 0-0 baz foo
(error) ERR The ID specified in XADD must be greater than 0-0
```

Your server should respond with `-ERR The ID specified in XADD must be greater than 0-0\r\n`, which is the error message above encoded as a
[simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-errors).

### Notes

No additional notes for this stage.

---

## Stage 74 — Partially auto-generated IDs

| | |
|---|---|
| **Slug** | `yh3` |
| **Difficulty** | Medium |
| **Source** | [streams-04-yh3.md](stage_descriptions/streams-04-yh3.md) |

### What to implement

In this stage, you'll extend `XADD` to support auto-generating the sequence number of an entry ID.

### Specifying Entry IDs in `XADD` (Recap)

As a recap, the `XADD` command accepts IDs in three formats:

- Explicit (`1526919030473-0`) (Handled in previous stages)
- Auto-generate only the sequence number (`1526919030474-*`)
- Auto-generate the time part and sequence number (`*`)

For this stage, you'll handle the second case, where only the sequence number is auto-generated.

### Auto-Generating Sequence Numbers

Redis automatically assigns sequence numbers based on the following conditions:

- If the stream is empty for a given time part, the sequence number starts at `0`.
- If there are already entries with the same time part, the new sequence number is the last sequence number plus `1`.
- The only exception is when the time part is `0`. In that case, the default sequence number starts at `1`.

Here's an example of adding an entry with `*` as the sequence number:

```bash
$ redis-cli XADD some_key "1-*" foo bar
"1-0" # The sequence number is 0 if no prior entries exist

$ redis-cli XADD some_key "1-*" bar baz
"1-1" # Adding another entry will increment the sequence number
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `XADD` command with `*` as the sequence number.

```bash
$ redis-cli XADD stream_key 0-* foo bar
```

Your server should respond with `$3\r\n0-1\r\n`, which is `0-1` encoded as a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).

Next, it will send another `XADD` command with `*` as the sequence number, but this time with a random number as the time part.

```bash
$ redis-cli XADD stream_key 5-* foo bar
```

Your server should respond with `$3\r\n5-0\r\n`, which is `5-0` encoded as a bulk string.

After that, the tester will send the same command again.

```bash
$ redis-cli XADD stream_key 5-* bar baz
```

Your server should respond with `$3\r\n5-1\r\n`, which is `5-1` encoded as a bulk string.


### Notes


- The tester will use a random number for the time part (we use `5` in the example above).

---

## Stage 75 — Fully auto-generated IDs

| | |
|---|---|
| **Slug** | `xu6` |
| **Difficulty** | Medium |
| **Source** | [streams-05-xu6.md](stage_descriptions/streams-05-xu6.md) |

### What to implement

In this stage, you'll extend `XADD` to support auto-generating entry IDs.

### Specifying Entry IDs in `XADD` (Recap)

As a recap, the `XADD` command accepts IDs in three formats:

- Explicit (`1526919030473-0`) (Handled in earlier stages)
- Auto-generate only the sequence number (`1526919030474-*`) (Handled in previous stages)
- Auto-generate the time part and sequence number (`*`)

For this stage, you'll handle the third case, where the entire entry ID is auto-generated.

### Auto-Generating Entry IDs

When `*` is used with the `XADD` command, the server automatically generates a unique ID for the new entry:

- It uses the current Unix time in milliseconds for the time part and `0` for the sequence number.
- If an entry with the same timestamp already exists in the stream, the server increments the sequence number by `1`.

Here's an example:

```bash
> XADD stream_key * foo bar
"1526919030474-0"
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then create an entry with `*` as the ID.

```bash
$ redis-cli XADD stream_key * foo bar
```

Your server should respond with a string like `$15\r\n1526919030474-0\r\n`, which is `1526919030474-0` encoded as a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).


### Notes


- The time part of the ID should be the current Unix time in **milliseconds**, not seconds.
- The tester doesn't test the case where a time part already exists in the stream and the sequence number is incremented. This is difficult to test reliably since we'd need to send two commands within the same millisecond.

---

## Stage 76 — Query entries from stream

| | |
|---|---|
| **Slug** | `zx1` |
| **Difficulty** | Medium |
| **Source** | [streams-06-zx1.md](stage_descriptions/streams-06-zx1.md) |

### What to implement

In this stage, you'll add support for querying data from a stream using the `XRANGE` command.

### The `XRANGE` command

The [`XRANGE`](https://redis.io/docs/latest/commands/xrange/) command retrieves a range of entries from a stream.

It takes two arguments: a `start` ID and an `end` ID, and returns all entries within that range. The range is inclusive, meaning entries with IDs equal to the `start` and `end` IDs are included.

Here's an example of how it works:

```bash
$ redis-cli XADD some_key 1526985054069-0 temperature 36 humidity 95
"1526985054069-0" # (ID of the first added entry)
$ redis-cli XADD some_key 1526985054079-0 temperature 37 humidity 94
"1526985054079-0"
$ redis-cli XRANGE some_key 1526985054069 1526985054079
1) 1) 1526985054069-0
   2) 1) temperature
      2) 36
      3) humidity
      4) 95
2) 1) 1526985054079-0
   2) 1) temperature
      2) 37
      3) humidity
      4) 94
```

The command can accept IDs in the format `<millisecondsTime>-<sequenceNumber>`, but the sequence number is optional. If you don't provide a sequence number:

- For the `start` ID, the sequence number defaults to `0`.
- For the `end` ID, the sequence number defaults to the maximum sequence number.

The return value of the command is not exactly what is shown in the example above. This is already formatted by redis-cli.

The actual return value is a [RESP array](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays) of arrays.

Each inner array represents a single entry and contains two elements:

- The entry's ID (as a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings)).
- An array of key-value pairs (as bulk strings) in the order they were added.

The return value of the example above is actually something like this:

```json
[
  [
    "1526985054069-0",
    [
      "temperature",
      "36",
      "humidity",
      "95"
    ]
  ],
  [
    "1526985054079-0",
    [
      "temperature",
      "37",
      "humidity",
      "94"
    ]
  ],
]
```

When encoded as a RESP array, it looks like this:

```text
*2\r\n
*2\r\n
$15\r\n1526985054069-0\r\n
*4\r\n
$11\r\ntemperature\r\n
$2\r\n36\r\n
$8\r\nhumidity\r\n
$2\r\n95\r\n
*2\r\n
$15\r\n1526985054079-0\r\n
*4\r\n
$11\r\ntemperature\r\n
$2\r\n37\r\n
$8\r\nhumidity\r\n
$2\r\n94\r\n
```
*(This response is separated into multiple lines for readability. The actual return value doesn't contain any additional newlines.)*


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then add a few entries to a stream.

```bash
$ redis-cli XADD stream_key 0-1 foo bar
"0-1"
$ redis-cli XADD stream_key 0-2 bar baz
"0-2"
$ redis-cli XADD stream_key 0-3 baz foo
"0-3"
```

Next, it will send an `XRANGE` command to your server.

```bash
$ redis-cli XRANGE stream_key 0-2 0-3
```

Your server should respond with a RESP array containing the range of entries from the IDs provided. 

Based on the example above, the response should look like the following (encoded as a RESP array):

```json
[
  [
    "0-2",
    [
      "bar",
      "baz"
    ]
  ],
  [
    "0-3",
    [
      "baz",
      "foo"
    ]
  ]
]
```

### Notes

No additional notes for this stage.

---

## Stage 77 — Query with -

| | |
|---|---|
| **Slug** | `yp1` |
| **Difficulty** | Easy |
| **Source** | [streams-07-yp1.md](stage_descriptions/streams-07-yp1.md) |

### What to implement

In this stage, you'll extend support for `XRANGE` to allow querying using `-`.

### Using `XRANGE` with `-`

In the `XRANGE` command, the `start` argument can be specified as `-` to retrieve entries from the very beginning of the stream. This provides a simple way to get a range of entries starting from the first one without needing to know its ID.

Here's an example of how that works:

```bash
$ redis-cli XADD some_key 1526985054069-0 temperature 36 humidity 95
"1526985054069-0"

$ redis-cli XADD some_key 1526985054079-0 temperature 37 humidity 94
"1526985054079-0"

$ redis-cli XRANGE some_key - 1526985054079
1) 1) 1526985054069-0
   2) 1) temperature
      2) 36
      3) humidity
      4) 95
2) 1) 1526985054079-0
   2) 1) temperature
      2) 37
      3) humidity
      4) 94
```

In the example above, `XRANGE` retrieves all entries from the beginning of the stream to the entry with ID `1526985054079-0`.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then create a few entries.

```bash
$ redis-cli XADD stream_key 0-1 foo bar
"0-1"
$ redis-cli XADD stream_key 0-2 bar baz
"0-2"
$ redis-cli XADD stream_key 0-3 baz foo
"0-3"
```

Next, it will send an `XRANGE` command using `-` as the `start` ID:

```bash
$ redis-cli XRANGE stream_key - 0-2
1) 1) 0-1
   2) 1) foo
      2) bar
2) 1) 0-2
   2) 1) bar
      2) baz
```

Your server should respond with a RESP array containing the entries from the beginning of the stream up to the entry with the provided `end` ID.

From the example above, the response should look like the following, encoded as a [RESP array](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays):

```json
[
  [
    "0-1",
    [
      "foo",
      "bar"
    ]
  ],
  [
    "0-2",
    [
      "bar",
      "baz"
    ]
  ]
]
```

### Notes

No additional notes for this stage.

---

## Stage 78 — Query with +

| | |
|---|---|
| **Slug** | `fs1` |
| **Difficulty** | Easy |
| **Source** | [streams-08-fs1.md](stage_descriptions/streams-08-fs1.md) |

### What to implement

In this stage, you'll extend support for `XRANGE` to allow querying using `+`.

### Using `XRANGE` with `+`

In the `XRANGE` command, the `end` argument can be specified as `+` to retrieve entries from the given `start` ID to the end of the stream.

Here's an example of how that works:

```bash
$ redis-cli XADD some_key 1526985054069-0 temperature 36 humidity 95
"1526985054069-0"

$ redis-cli XADD some_key 1526985054079-0 temperature 37 humidity 94
"1526985054079-0"

$ redis-cli XRANGE some_key 1526985054069 +
1) 1) 1526985054069-0
   2) 1) temperature
      2) 36
      3) humidity
      4) 95
2) 1) 1526985054079-0
   2) 1) temperature
      2) 37
      3) humidity
      4) 94
```

In the example above, `XRANGE` retrieves all the entries from `some_key` starting from the entry with ID `1526985054069-0` to the very end of the stream.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then create a few entries.

```bash
$ redis-cli XADD stream_key 0-1 foo bar
$ redis-cli XADD stream_key 0-2 bar baz
$ redis-cli XADD stream_key 0-3 baz foo
```

Next, it will send an `XRANGE` command using `+` as the `end` ID:

```bash
$ redis-cli XRANGE stream_key 0-2 +
```

Your server should respond with a RESP array containing the entries from the provided `start` ID to the end of the stream.

From the example above, your response should look like the following, encoded as a [RESP array](https://redis.io/docs/latest/develop/reference/protocol-spec/#arrays):

```json
[
  [
    "0-2",
    [
      "bar",
      "baz"
    ]
  ],
  [
    "0-3",
    [
      "baz",
      "foo"
    ]
  ]
]
```

The raw RESP encoding looks like this:

```text
*2\r\n
*2\r\n
$3\r\n0-2\r\n
*2\r\n
$3\r\nbar\r\n
$3\r\nbaz\r\n
*2\r\n
$3\r\n0-3\r\n
*2\r\n
$3\r\nbaz\r\n
$3\r\nfoo\r\n
```


### Notes

- In the response, the items are shown in separate lines for readability. The tester expects all of these to be in one line.

---

## Stage 79 — Query single stream using XREAD

| | |
|---|---|
| **Slug** | `um0` |
| **Difficulty** | Medium |
| **Source** | [streams-09-um0.md](stage_descriptions/streams-09-um0.md) |

### What to implement

In this stage, you'll add support to querying a stream using the `XREAD` command.

### The `XREAD` Command

The [`XREAD`](https://redis.io/docs/latest/commands/xread/) command is used to read data from one or more streams, starting from a specified entry ID.

Unlike `XRANGE`, which requires both a `start` and `end` ID, `XREAD` takes only a single ID. 

Another difference is that `XREAD` is exclusive. This means that the command retrieves all entries with an ID greater than the specified ID.

The basic syntax for the command is:
```bash
XREAD STREAMS <key> <id>
```

`XREAD` supports other optional arguments, but we won't deal with them at this stage.

Here's an example of its behavior:

```bash
$ redis-cli XADD some_key 1526985054069-0 temperature 36 humidity 95
"1526985054069-0"

$ redis-cli XADD some_key 1526985054079-0 temperature 37 humidity 94
"1526985054079-0"

$ redis-cli XREAD STREAMS some_key 1526985054069-0
1) 1) "some_key"
   2) 1) 1) 1526985054079-0
         2) 1) temperature
            2) 37
            3) humidity
            4) 94
```

The return value is an array of streams, where each stream contains:

- The stream key (as a bulk string).
- An array of entries, where each entry is an array of two parts:
  - The entry's ID (as a bulk string).
  - An array of key-value pairs (as bulk strings).

Here's what the response from the example above would look like:

```json
[
  [
    "some_key",
    [
      [
        "1526985054079-0",
        [
          "temperature",
          "37",
          "humidity",
          "94"
        ]
      ]
    ]
  ]
]
```

When encoded as a RESP array, it looks like this:

```text
*1\r\n
*2\r\n
$8\r\nsome_key\r\n
*1\r\n
*2\r\n
$15\r\n1526985054079-0\r\n
*4\r\n
$11\r\ntemperature\r\n
$2\r\n37\r\n
$8\r\nhumidity\r\n
$2\r\n94\r\n
```
(*The result is shown on separate lines for readability. The actual return value is a single line.*)


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `XADD` command to add an entry to a stream.

```bash
$ redis-cli XADD stream_key 0-1 temperature 96
```

Next, the tester will send an `XREAD` command to your server.

```bash
$ redis-cli XREAD STREAMS stream_key 0-0
```

Your server should respond with a RESP array containing the correct stream entries.

From the example above, your response should look like the following, encoded as a RESP array:

```json
[
  [
    "stream_key",
    [
      [
        "0-1",
        [
          "temperature",
          "96"
        ]
      ]
    ]
  ]
]
```

### Notes

No additional notes for this stage.

---

## Stage 80 — Query multiple streams using XREAD

| | |
|---|---|
| **Slug** | `ru9` |
| **Difficulty** | Medium |
| **Source** | [streams-10-ru9.md](stage_descriptions/streams-10-ru9.md) |

### What to implement

In this stage, you'll add support for querying multiple streams using the `XREAD` command.

### The `XREAD` Command for Multiple Streams

When reading from multiple streams, the `XREAD` command takes the `STREAMS` keyword, followed by a list of stream keys, and then a corresponding list of entry IDs for each stream.

The command format is: 

```bash
XREAD STREAMS <key1> <key2> ... <id1> <id2> ...
```

The server's response remains a RESP array of streams where:

- Each stream is a two-item array: the stream's key and the entries read from it.
- The order of the streams in the response must match the order in which they were specified in the command.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send multiple `XADD` commands to add entries to several streams.

```bash
$ redis-cli XADD stream_key 0-1 temperature 95
$ redis-cli XADD other_stream_key 0-2 humidity 97
```

Next, the tester will send an `XREAD` command to your server with multiple streams.

```bash
$ redis-cli XREAD streams stream_key other_stream_key 0-0 0-1
```

Your server should respond with a RESP array containing the results for all the specified streams.

From the example above, your response should look like the following, encoded as a RESP array:

```json
[
  [
    "stream_key",
    [
      [
        "0-1",
        [
          "temperature",
          "95"
        ]
      ]
    ]
  ],
  [
    "other_stream_key",
    [
      [
        "0-2",
        [
          "humidity",
          "97"
        ]
      ]
    ]
  ]
]
```

### Notes

No additional notes for this stage.

---

## Stage 81 — Blocking reads

| | |
|---|---|
| **Slug** | `bs1` |
| **Difficulty** | Hard |
| **Source** | [streams-11-bs1.md](stage_descriptions/streams-11-bs1.md) |

### What to implement

 In this stage, you’ll extend your `XREAD` implementation to support blocking.

### Understanding Blocking in `XREAD`

By default, the `XREAD` command is synchronous: it returns data immediately if entries are available, or an empty response if not. 

However, with the optional `BLOCK` parameter, clients can wait for new data to arrive.

The syntax looks like this:

```bash
XREAD BLOCK <milliseconds> STREAMS <key> <id>
```

Here's how it works:
- The client sends the command and specifies a timeout in milliseconds.
- If there are already entries with IDs greater than the specified ID, the command returns them immediately.
- If there are no new entries available, the command blocks and waits.
- If a new entry is added to the stream before the timeout expires, the command unblocks and returns the new entry (or entries).
- If the timeout expires and no new data has arrived, the server responds with a null array (`*-1\r\n`).

For example, consider two separate clients. The first client sends a blocking `XREAD` command:

```bash
$ redis-cli XREAD BLOCK 1000 streams some_key 1526985054069-0
```

This client will now block and wait for a new entry to be added.

Meanwhile, a second client can add an entry to the stream:

```bash
$ other-redis-cli XADD some_key 1526985054079-0 temperature 37 humidity 94
"1526985054079-0"
```

If the entry is added within `1000` milliseconds, the first client's `XREAD` command will immediately unblock and respond with the new entry:

```bash
1) 1) "some_key"
   2) 1) 1) 1526985054079-0
      2) 1) temperature
         2) 37
         3) humidity
         4) 94
```

However, if no new entry is added before the timeout expires, the command will fail and return a null array:

```bash
(nil)
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then add an entry to a stream.

```bash
$ redis-cli XADD stream_key 0-1 temperature 96
```

Next, it will send an `XREAD` command to your server with the `BLOCK` option.

```bash
$ redis-cli XREAD BLOCK 1000 streams stream_key 0-1
```

In another instance of the redis-cli, the tester will add an entry within `500` milliseconds.

```bash
$ redis-cli XADD stream_key 0-2 temperature 95
```

Your server should respond to the first client with the following, encoded as a RESP array:

```json
[
  [
    "stream_key",
    [
      [
        "0-2",
        [
          "temperature",
          "95"
        ]
      ]
    ]
  ]
]
```

Next, the tester will send another blocking `XREAD` command, but this time, it will allow the full timeout duration to elapse before checking the response.

```bash
$ redis-cli XREAD block 1000 streams stream_key 0-2
```

In this case, your server should respond with a [null array](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays) (`*-1\r\n`).

### Notes

No additional notes for this stage.

---

## Stage 82 — Blocking reads without timeout

| | |
|---|---|
| **Slug** | `hw1` |
| **Difficulty** | Medium |
| **Source** | [streams-12-hw1.md](stage_descriptions/streams-12-hw1.md) |

### What to implement

In this stage, you'll extend your `XREAD` implementation to support blocking indefinitely.

### Indefinite Blocking in `XREAD`

As a recap, the `XREAD` command supports blocking using the `BLOCK <milliseconds>` parameter. 

If the `milliseconds` value is set to `0`, the `XREAD` command will block indefinitely until a new entry is added to the stream(s).

For example, consider two separate clients. The first client sends a blocking `XREAD` command with `0` as the time passed in:

```bash
$ redis-cli XREAD BLOCK 0 streams some_key 1526985054069-0
```

This client will be blocked indefinitely until a new entry is added.

If the second client adds an entry to the stream at any time:

```bash
$ other-redis-cli XADD some_key 1526985054079-0 temperature 37 humidity 94
"1526985054079-0"
```

The first client's `XREAD` command will immediately unblock and respond with the new entry:

```bash
1) 1) "some_key"
   2) 1) 1) 1526985054079-0
         2) 1) temperature
            2) 37
            3) humidity
            4) 94
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then add an entry to a stream.

```bash
$ redis-cli XADD stream_key 0-1 temperature 96
```

Next, the tester will send an indefinite blocking `XREAD` command to your server:

```bash
$ redis-cli XREAD BLOCK 0 streams stream_key 0-1
```

It will then pause for `1000` milliseconds to confirm the client remains blocked and doesn't time out. After that, it will add another entry:

```bash
$ redis-cli XADD stream_key 0-2 temperature 95
```

Your server should respond with the following, encoded as a RESP array:

```json
[
  [
    "stream_key",
    [
      [
        "0-2",
        [
          "temperature",
          "95"
        ]
      ]
    ]
  ]
]
```

### Notes

No additional notes for this stage.

---

## Stage 83 — Blocking reads using $

| | |
|---|---|
| **Slug** | `xu1` |
| **Difficulty** | Easy |
| **Source** | [streams-13-xu1.md](stage_descriptions/streams-13-xu1.md) |

### What to implement

In this stage, you’ll extend support for `XREAD` to handle `$` as the starting ID in a blocking read.

### Understanding `$` as an Entry ID

When `$` is passed as the ID, `XREAD` will only return new entries added after the command is sent. This is similar to passing in the maximum ID currently available in a stream.

For example, suppose we have two separate clients. The first client sends a blocking `XREAD` command with `1000` as the timeout and `$` as the ID:

```bash
$ redis-cli XREAD BLOCK 1000 streams some_key $
```

Then, the second client adds an entry to the `some_key` stream.

```bash
$ other-redis-cli XADD some_key 1526985054079-0 temperature 37 humidity 94
"1526985054079-0"
```

Similar to the behavior detailed in the earlier stages, if the command is sent within `1000` milliseconds, the first client will be unblocked and get the new entry:

```bash
1) 1) "some_key"
   2) 1) 1) 1526985054079-0
         2) 1) temperature
            2) 37
            3) humidity
            4) 94
```

If not, the response will be a [null array](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays).

```bash
(nil)
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then add an entry to a stream.

```bash
$ redis-cli XADD stream_key 0-1 temperature 96
```

Next, it will send an `XREAD` command to your server with the `BLOCK` command with `0` as the time and `$` as the ID.

```bash
$ redis-cli XREAD BLOCK 0 streams stream_key $
```

In another instance of the redis-cli, the tester will add another entry after `500` milliseconds.

```bash
$ redis-cli XADD stream_key 0-2 temperature 95
```

Your server should respond with the following, as a RESP array:

```json
[
  [
    "stream_key",
    [
      [
        "0-2",
        [
          "temperature",
          "95"
        ]
      ]
    ]
  ]
]
```

After that, the tester will send another `XREAD` command to your server with the `BLOCK` command, but this time, it'll wait for `1000` milliseconds before checking the response of your server.

```bash
$ redis-cli XREAD BLOCK 1000 streams stream_key $
```

Your server should respond with a null array (`*-1\r\n`).

### Notes

No additional notes for this stage.

---

## Stage 84 — The WATCH command

| | |
|---|---|
| **Slug** | `jb7` |
| **Difficulty** | Easy |
| **Source** | [optimistic-locking-01-jb7.md](stage_descriptions/optimistic-locking-01-jb7.md) |

### What to implement

In this stage, you'll add support for the `WATCH` command.

<!-- ### Optimistic Locking in Redis

Say two clients both want to increment a counter that's currently set to `10`. Client A reads `10`, Client B reads `10`. Client A writes `11`. Client B also writes `11`. The second write overwrites the first, so only one increment sticks while the other update gets lost.

This is the "lost update problem", and it shows up whenever multiple clients read-then-write the same key. The classic fix is to lock the key before reading it so nobody else can touch it until you're done (pessimistic locking). That works, but locks are expensive. If most updates don't actually collide, you're paying the cost of locking every time for conflicts that rarely happen.

Optimistic locking flips the approach. Instead of locking upfront, you let every client read and write freely, but you keep track of what they read. Right before a client commits its changes, you check: did any of those values change since the client read them? If so, the commit is rejected, and the client can retry with fresh data. -->

### The `WATCH` Command

Redis uses the [`WATCH`](https://redis.io/docs/latest/commands/watch/) command to implement [optimistic locking](https://redis.io/docs/latest/develop/using-commands/transactions/#optimistic-locking-using-check-and-set). It tells the server to monitor a key for changes. Later, when the client executes a group of commands as a [transaction](https://redis.io/docs/latest/develop/using-commands/transactions/), the server checks whether any watched keys were modified in the meantime. If they were, the whole transaction is discarded.

Here's what that looks like in practice:

```bash
Client A: WATCH counter          # "Tell me if this changes"
Client A: GET counter → 10       # Read current value
Client B: SET counter 20         # Another client sneaks in an update
Client A: MULTI                  # Start a transaction
Client A: SET counter 11         # Queue an update based on stale data
Client A: EXEC                   # Try to execute...
→ (nil)                          # Transaction aborted, counter changed
```

You'll implement this full flow in later stages. For this stage, you only need to parse the `WATCH` command and respond with `+OK\r\n`.

```bash
$ redis-cli WATCH key
OK
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `WATCH` command:

```bash
$ redis-cli WATCH key
```

The tester will verify that:
- The response is `+OK\r\n` (a RESP simple string)
- Your program doesn't crash or produce errors


### Notes

- `WATCH` can accept multiple keys in a single command (e.g., `WATCH key1 key2 key3`), but you only need to handle a single key for now. We'll handle multiple keys in later stages.
- The actual watch tracking and transaction abort behavior will come in later stages.

---

## Stage 85 — WATCH inside transaction

| | |
|---|---|
| **Slug** | `jq9` |
| **Difficulty** | Easy |
| **Source** | [optimistic-locking-02-jq9.md](stage_descriptions/optimistic-locking-02-jq9.md) |

### What to implement

In this stage, you'll implement `MULTI` and update `WATCH` to reject calls inside transactions.

### Redis Transactions

In previous stages, you saw how `WATCH` fits into a larger flow: watch a key, start a transaction with `MULTI`, queue some commands, and execute with `EXEC`. Now you'll start building the transaction side.

[Redis transactions](https://redis.io/docs/latest/develop/using-commands/transactions/) allow clients to execute multiple commands as a single operation. The basic flow is:

1. `MULTI` - Enter transaction mode (commands get queued instead of executing immediately)
2. `SET key value`, `INCR counter`, etc. - Commands are queued
3. `EXEC` - Execute all queued commands

We'll implement `EXEC` in later stages. For this stage, you'll implement the `MULTI` command and make `WATCH` check whether it's being called inside a transaction.

### The `MULTI` Command

The [`MULTI` command](https://redis.io/docs/latest/commands/multi/) is used to start a transaction.

```bash
$ redis-cli MULTI
OK
```

When a client uses `MULTI`, the server:
1. Marks the connection as being inside a transaction
2. Returns `OK` as a simple string

### The `WATCH` Command (Updated)

`WATCH` is meant to be called **before** a transaction starts. Calling it inside a transaction is not allowed because the transaction has already begun, and commands are already being queued.

Now that you have the transaction state, `WATCH` needs to enforce this:

- If the connection is outside a transaction:
    1. Add the key to the connection's collection of watched keys
    2. Return `OK` as a simple string
- If the connection is inside a transaction (after `MULTI`):
    1. Return a RESP error like: `-ERR WATCH inside MULTI is not allowed\r\n`. The exact wording is flexible but should include: `ERR`, `WATCH`, `inside MULTI`, and `not allowed`.

For example:
```bash
$ redis-cli
> WATCH counter
OK
> MULTI
OK
(TX)> WATCH another_key
(error) ERR WATCH inside MULTI is not allowed
```


### Tests


The tester will execute your program like this:
```bash
$ ./your_program.sh
```

It will then spawn a client, begin a transaction using the `MULTI` command, and send a `WATCH` command with a random key:
```bash
$ redis-cli
> MULTI
OK
> WATCH key
(error) ERR WATCH inside MULTI is not allowed
```

The tester will verify that:
- `MULTI` returns `OK` as a simple string
- `WATCH` inside a transaction returns a RESP error containing: `ERR`, `WATCH`, `inside MULTI`, and `not allowed`


### Notes


- You'll need a way to track state per connection (like a map keyed by the connection/socket).
- When `WATCH` is called outside a transaction, you should still track the watched keys even though they're not used yet. This prepares you for later stages.
- The exact error message format is flexible as long as it includes the required keywords.

---

## Stage 86 — Tracking key modifications

| | |
|---|---|
| **Slug** | `mh8` |
| **Difficulty** | Medium |
| **Source** | [optimistic-locking-03-mh8.md](stage_descriptions/optimistic-locking-03-mh8.md) |

### What to implement

In this stage, you'll implement the `EXEC` command and add support for aborting transactions.

### Redis Transactions (Recap)

As a recap, [Redis transactions](https://redis.io/docs/latest/develop/using-commands/transactions/) allow clients to execute multiple commands as a single operation. The basic flow is:

1. `MULTI` - Enter transaction mode (Handled in previous stages)
2. `SET key value`, `INCR counter`, etc. - Commands are queued
3. `EXEC` - Execute all queued commands

For this stage, you'll implement the `EXEC` command and fail transactions when their watched keys have been modified.

### The `EXEC` Command

The [`EXEC` command](https://redis.io/docs/latest/commands/exec/) executes all commands that were queued after `MULTI`:

```bash
$ redis-cli
> SET foo 100
OK
> MULTI
OK
> SET foo 200
QUEUED
> SET bar 300
QUEUED
> EXEC
1) OK
2) OK
```

The response is a RESP array where each element is the result of a queued command, in the order they were queued.

### Failing Transactions With `WATCH`

When a connection has watched keys, `EXEC` needs to check whether any of those keys were modified by another client between the `WATCH` and the `EXEC`. If they were, the transaction is aborted:

```
Client A: WATCH foo             # Server tracks "foo" for Client A
Client A: MULTI
Client A: SET bar 300           # Queued
Client B: SET foo 999           # Modifies Client A's watched key
Client A: EXEC                  # Returns *-1\r\n, queued commands discarded
```

`EXEC` returns a RESP null array (`*-1\r\n`), and the queued commands are discarded without executing.

If none of the watched keys were modified, `EXEC` runs the transaction normally.

<!-- To make this work, your server needs to detect modifications across connections. Whenever it processes a write command, it should check if the modified key is being watched by any other connection. If it is, that connection's transaction should be marked as dirty so that its next `EXEC` knows to abort.

After `EXEC` completes (whether it succeeded or was aborted), clear the connection's watched keys and dirty state. -->


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

The tester will spawn four clients and run two scenarios. 

In the first scenario, it modifies the watched key from a different client:

```bash
# Client 1
> SET foo 100        # → +OK
> SET bar 200        # → +OK
> WATCH foo          # → +OK
> MULTI              # → +OK
> SET bar 300        # → +QUEUED

# Client 2 modifies the watched key
> SET foo 200        # → +OK

# Client 1 tries to execute
> EXEC               # → *-1\r\n (null array, transaction aborted)
> GET bar            # → "200" (unchanged, SET bar 300 was never executed)
```

In the second scenario, it modifies a different key (not the watched one): 

```bash
# Client 3
> SET baz 100        # → +OK
> SET caz 200        # → +OK
> WATCH baz          # → +OK
> MULTI              # → +OK
> SET caz 400        # → +QUEUED

# Client 4 modifies a different key (not the watched one)
> SET caz 300        # → +OK

# Client 3 executes successfully
> EXEC               # → array of responses (baz was not modified)

# Client 4 verifies the transaction's write took effect
> GET caz            # → "400"
```

The tester will verify that:

- `EXEC` returns a RESP null array (`*-1\r\n`) when a watched key was modified by another client
- `EXEC` returns the array of command results when no watched keys were modified
- Queued commands are not executed when the transaction is aborted, and the original values are preserved
- Queued commands are executed when no watched keys are modified, and their writes take effect


### Notes


- It's not enough to compare the value of a watched key at `WATCH` time vs `EXEC` time. If a key was modified and then set back to its original value, the transaction should still abort. Track whether the key was touched, not whether its value changed.
- After `EXEC`, clear the connection's watch state (watched keys and dirty flag) regardless of whether the transaction succeeded or was aborted.

---

## Stage 87 — Watching multiple keys

| | |
|---|---|
| **Slug** | `fp0` |
| **Difficulty** | Medium |
| **Source** | [optimistic-locking-04-fp0.md](stage_descriptions/optimistic-locking-04-fp0.md) |

### What to implement

In this stage, you'll add support for watching multiple keys.

### Watching Multiple Keys

In previous stages, you only needed to handle `WATCH` with a single key. However, the `WATCH` command also accepts multiple keys at once:

```bash
$ redis-cli
> WATCH foo bar baz
OK
```

All the keys are monitored for changes. If any one of them is modified by another client before `EXEC`, the transaction aborts:

```
Client A: WATCH foo bar          # Monitor both keys
Client B: SET bar 300            # Modifies one of the watched keys
Client A: MULTI
Client A: SET foo 400            # Queued
Client A: EXEC                   # Transaction aborted, bar was modified
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

The tester will spawn two clients. Using the first client, it will set two keys, watch both of them, and begin a transaction:

```bash
# Client 1
> SET foo 100        # → OK
> SET bar 200        # → OK
> WATCH foo bar      # → OK
> MULTI              # → OK
> SET bar 300        # → QUEUED
```

Using the second client, it will modify one of the watched keys:

```bash
# Client 2
> SET foo 200        # → OK
```

Back on the first client, the tester will execute the transaction and verify the abort:

```bash
# Client 1
> EXEC               # → *-1\r\n (transaction aborted)

# Client 2
> GET bar            # → "200" (unchanged, transaction had no effect)
```

The tester will verify that:

- `WATCH` with multiple keys returns `+OK\r\n`
- `EXEC` returns a RESP null array (`*-1\r\n`) when any one of the watched keys was modified
- The aborted transaction's queued commands had no effect

### Notes

No additional notes for this stage.

---

## Stage 88 — Watching missing keys

| | |
|---|---|
| **Slug** | `uo9` |
| **Difficulty** | Easy |
| **Source** | [optimistic-locking-05-uo9.md](stage_descriptions/optimistic-locking-05-uo9.md) |

### What to implement

In this stage, you'll add support for watching keys that don't exist yet.

### Watching Non-existent Keys

So far, you've only watched keys that already have values. But `WATCH` also works on keys that don't exist at the time of the call. If a watched key is created by another client before `EXEC`, the transaction should still abort.

```bash
# Client A
> WATCH foo
OK

# Client B
> SET foo 300
OK

# Client A
> MULTI
OK
> SET bar 500
QUEUED
> EXEC
*-1\r\n                # transaction aborted, foo was created after WATCH
```

The key didn't exist when Client A watched it, but Client B created it before `EXEC`. That counts as a modification.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

The tester will then spawn two clients. Using the first client, it will watch a key that doesn't exist:

```bash
# Client 1
> WATCH foo
OK
```

Using the second client, it will create the watched key:

```bash
# Client 2
> SET foo 200
OK
```

Back on the first client, the tester will start a transaction and attempt to execute it:

```bash
# Client 1
> MULTI
OK
> SET foo 300
QUEUED
> EXEC
*-1\r\n                # transaction aborted
> GET foo
"200"                  # unchanged, transaction had no effect
```

The tester will verify that:

- `EXEC` returns a RESP null array (`*-1\r\n`) when a watched key was created by another client
- The aborted transaction's queued commands had no effect


### Notes


- If your implementation already tracks modifications by flagging watched keys on any write command, this stage may already pass without changes.
- The key insight is that "modified" includes going from non-existent to existing. A key doesn't need to have a prior value to be considered touched.

---

## Stage 89 — The UNWATCH command

| | |
|---|---|
| **Slug** | `bn1` |
| **Difficulty** | Easy |
| **Source** | [optimistic-locking-06-bn1.md](stage_descriptions/optimistic-locking-06-bn1.md) |

### What to implement

In this stage, you'll add support for the `UNWATCH` command.

### The `UNWATCH` Command

The [`UNWATCH` command](https://redis.io/docs/latest/commands/unwatch/) clears all watched keys for the current connection. Any keys previously registered with `WATCH` are no longer monitored, so future transactions won't be affected by changes to those keys.

```bash
$ redis-cli
> WATCH foo bar
OK
> UNWATCH
OK
```

The response is always `+OK\r\n`.

After `UNWATCH`, even if the previously watched keys are modified by another client, the next transaction will execute normally.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

The tester will spawn two clients. Using the first client, it will set two keys and watch them:

```bash
# Client 1
> SET foo 100
OK
> SET bar 200
OK
> WATCH foo bar
OK
```

Using the second client, it will modify one of the watched keys:

```bash
# Client 2
> SET foo 200
OK
```

Back on the first client, the tester will unwatch all keys, then start and execute a transaction:

```bash
# Client 1
> UNWATCH
OK
> MULTI
OK
> SET foo 400
QUEUED
> EXEC
1) OK                  # transaction succeeds, keys were unwatched

# Client 2
> GET foo
"400"                  # confirms the transaction's write took effect
```

The tester will verify that:

- `UNWATCH` returns `+OK\r\n`
- `EXEC` succeeds even though a previously watched key was modified, because `UNWATCH` cleared the watch state
- The transaction's queued commands were applied


### Notes


- `UNWATCH` takes no arguments. It always clears all watched keys for the connection.
- `UNWATCH` should work even if no keys were previously watched (it just returns `+OK\r\n`).

---

## Stage 90 — Unwatch on EXEC

| | |
|---|---|
| **Slug** | `fn4` |
| **Difficulty** | Easy |
| **Source** | [optimistic-locking-07-fn4.md](stage_descriptions/optimistic-locking-07-fn4.md) |

### What to implement

In this stage, you'll verify that watched keys are cleared after `EXEC`.

### Clearing Watched Keys on `EXEC`

`WATCH` is meant to protect a single transaction. Once `EXEC` runs (whether the transaction succeeds or aborts), the connection's watch state should be cleared. Without this cleanup, stale watch state could leak into future transactions on the same connection, causing them to fail for reasons unrelated to them.

```bash
$ redis-cli
# Client A
> WATCH foo
OK

# Client B
> SET foo 100
OK

# Client A
> MULTI
OK
> SET bar 200
QUEUED
> EXEC
*-1\r\n                # transaction aborted, foo was modified

# Client A starts a new transaction (no WATCH this time)
> MULTI
OK
> SET bar 200
QUEUED
> EXEC
1) OK                  # succeeds, previous EXEC cleared the watch state
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

The tester will spawn two clients. Using the first client, it will set two keys, watch one of them, and begin a transaction:

```bash
# Client 1
> SET foo 100
OK
> SET bar 200
OK
> WATCH foo
OK
> MULTI
OK
> SET bar 300
QUEUED
```

Using the second client, it will modify the watched key:

```bash
# Client 2
> SET foo 200
OK
```

Back on the first client, the tester will execute the transaction (expecting it to abort), then immediately start and execute a second transaction:

```bash
# Client 1
> EXEC
*-1\r\n                # first transaction aborted

# Client 1 starts a new transaction
> MULTI
OK
> SET bar 300
QUEUED
> EXEC
1) OK                  # second transaction succeeds

# Client 2
> GET bar
"300"                  # confirms the second transaction's write took effect
```

The tester will verify that:

- The first `EXEC` returns a RESP null array (`*-1\r\n`) because the watched key was modified
- The second `EXEC` succeeds because the first `EXEC` cleared the watch state
- The second transaction's queued commands were applied


### Notes


- If you have already cleared the watch state after `EXEC` (as suggested in earlier stages), this stage should pass without additional changes.
- Clearing watched keys on `DISCARD` will be handled in later stages.

---

## Stage 91 — Unwatch on DISCARD

| | |
|---|---|
| **Slug** | `hq1` |
| **Difficulty** | Easy |
| **Source** | [optimistic-locking-08-hq1.md](stage_descriptions/optimistic-locking-08-hq1.md) |

### What to implement

In this stage, you'll implement the `DISCARD` command and verify that it clears watched keys.

### The `DISCARD` Command

The [`DISCARD` command](https://redis.io/docs/latest/commands/discard/) aborts a transaction. It throws away all queued commands and exits transaction mode:

```bash
$ redis-cli
> MULTI
OK
> SET foo 100
QUEUED
> SET bar 200
QUEUED
> DISCARD
OK
```

The response is always `+OK\r\n`.

### Clearing Watch State

Just like `EXEC`, `DISCARD` also clears the connection's watched keys. This means a subsequent transaction on the same connection won't be affected by modifications that happened before the `DISCARD`.

```bash
$ redis-cli
# Client A
> WATCH foo
OK

# Client B
> SET foo 999
OK

# Client A
> MULTI
OK
> SET bar 200
QUEUED
> DISCARD
OK                     # watch state cleared

# Client A starts a new transaction (no WATCH active)
> MULTI
OK
> SET bar 200
QUEUED
> EXEC
1) OK                  # succeeds, DISCARD cleared the watch state
```

Without this cleanup, the second transaction would incorrectly fail because `foo` was modified before the `DISCARD`.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

The tester will spawn two clients. Using the first client, it will set two keys, watch them, and begin a transaction:

```bash
# Client 1
> SET foo 100
OK
> SET bar 200
OK
> WATCH foo bar
OK
> MULTI
OK
> SET bar 300
QUEUED
```

Using the second client, it will modify one of the watched keys:

```bash
# Client 2
> SET foo 400
OK
```

Back on the first client, the tester will discard the transaction, then immediately start and execute a new one:

```bash
# Client 1
> DISCARD
OK

# Client 1 starts a new transaction
> MULTI
OK
> SET bar 300
QUEUED
> EXEC
1) OK                  # succeeds, DISCARD cleared the watch state

# Client 2
> GET bar
"300"                  # confirms the second transaction's write took effect
```

The tester will verify that:

- `DISCARD` returns `+OK\r\n`
- `DISCARD` clears the connection's watch state
- The second transaction succeeds because the watched keys were cleared by `DISCARD`
- The second transaction's queued commands were applied


### Notes


- `DISCARD` should clear both the command queue and the watch state (watched keys and dirty flag).
- If you already handle watch state cleanup in `EXEC`, the `DISCARD` implementation should follow the same pattern.

---

## Stage 92 — Create a sorted set

| | |
|---|---|
| **Slug** | `ct1` |
| **Difficulty** | Easy |
| **Source** | [sorted-sets-01-ct1.md](stage_descriptions/sorted-sets-01-ct1.md) |

### What to implement

In this stage, you'll add support for creating a [sorted set](https://redis.io/docs/latest/develop/data-types/sorted-sets/) using the `ZADD` command.

### Redis Sorted Sets

Sorted sets are one of the data types that Redis supports. A sorted set is a collection of unique elements, where each element is associated with a floating-point score. Unlike regular sets, sorted sets maintain elements in a defined order based on their scores.

This makes them useful for use cases like leaderboards, priority queues, or any scenario where you need fast access to items sorted by a numerical value.

For example, if you were using sorted sets to store user rankings in a game leaderboard, the contents of the sorted set might look like this:

```yaml
racer_scores:
    - member: "Ford"
      score: 6.1
    - member: "Royce"
      score: 8.2
    - member: "Sam-Bodden"
      score: 8.2
    - member: "Prickett"
      score: 14.5
```

Sorted sets are ordered based on increasing scores.


### The `ZADD` Command

The [ZADD](https://redis.io/docs/latest/commands/zadd/) command is used to add a member to a sorted set.

If the sorted set does not exist, it is created and the member is added to it.

Example usage:

```bash
> ZADD racer_scores 8.0 "Sam"
(integer) 1
```

The `ZADD` command takes the key, a score, and the member name as arguments. It returns an integer representing the number of new members added to the sorted set.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `ZADD` command specifying a key, value and score.

```bash
$ redis-cli ZADD zset_key 10.0 zset_member
```

The tester will verify that the response to the command is `:1\r\n`, which is 1 (the number of members added to the sorted set), encoded as a RESP Integer.


### Notes

- In this stage, you'll only need to handle creating a new sorted set with a single member. We will get to adding new members to existing sorted set in the later stages.
- It is recommended to store the score as a 64 bit floating point number for highest precision as the official Redis implementation uses [`double`](https://github.com/redis/redis/blob/bec644aab198049eaa5583631c419b4574b137e1/tests/modules/zset.c#L34).

- We suggest that you implement sorted sets using a data structure where the members are stored in a sorted fashion according to their scores. It'll come in handy in the later stages.
    -  Redis implements sorted sets using a combination of [hash table and skip list](https://github.com/redis/redis/blob/674b829981c0b8ad15a670a32df503e0e4514e96/src/server.h#L1560).

---

## Stage 93 — Add members

| | |
|---|---|
| **Slug** | `hf1` |
| **Difficulty** | Medium |
| **Source** | [sorted-sets-02-hf1.md](stage_descriptions/sorted-sets-02-hf1.md) |

### What to implement

In this stage, you'll add support for adding elements to an existing sorted set.

### Adding elements using `ZADD`

The `ZADD` command can be used to add a new member to an existing sorted set, or update the score of an existing member. It returns the count of new members added to the sorted set as an integer.


Example usage:
```bash
> ZADD zset_key 0.0043 foo
(integer) 1
> ZADD zset_key 8.0 bar
(integer) 1

# No new members were added
> ZADD zset_key 10.0 bar
(integer) 0
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `ZADD` command to create a new sorted set.

```bash
$ redis-cli ZADD zset_key 20.0 zset_member1 (Expecting ":1\r\n")
```

The tester will then send the `ZADD` command a few times to add new members.

```bash
$ redis-cli
> ZADD zset_key 30.1 zset_member2 (Expecting ":1\r\n")
> ZADD zset_key 40.2 zset_member3 (Expecting ":1\r\n")
> ZADD zset_key 50.3 zset_member4 (Expecting ":1\r\n")
```

The tester expects the response to be `:1\r\n` whenever a new member is added.

It will then update the score of an existing member.
```bash
> ZADD zset_key 100.0 zset_member1 (Expecting ":0\r\n")
```

The tester expects the response to be `:0\r\n` whenever an existing member is updated.

### Notes

No additional notes for this stage.

---

## Stage 94 — Retrieve member rank

| | |
|---|---|
| **Slug** | `lg6` |
| **Difficulty** | Medium |
| **Source** | [sorted-sets-03-lg6.md](stage_descriptions/sorted-sets-03-lg6.md) |

### What to implement

In this stage, you'll add support for retrieving the rank of a sorted set member using the `ZRANK` command.

### The `ZRANK` Command

The `ZRANK` command is used to query the rank of a member in a sorted set. It returns an integer, which is 0-based index of the member when the sorted set is ordered by increasing score.
If two members have same score, the members are ordered lexicographically.

Example usage:
```bash
> ZADD zset_key 1.0 member_with_score_1
(integer) 1
> ZADD zset_key 2.0 member_with_score_2
(integer) 1
> ZADD zset_key 2.0 another_member_with_score_2
(integer) 1


> ZRANK zset_key member_with_score_1
(integer) 0
> ZRANK zset_key member_with_score_2
(integer) 2
> ZRANK zset_key another_member_with_score_2
(integer) 1
```

The rank of `another_member_with_score_2` is 1, and `member_with_score_2` is 2. It is because though the both members have same scores, `another_member_with_score_2` preceeds `member_with_score_2` lexicographically.


If the member, or the sorted set does not exist, the command returns null bulk string (`$-1\r\n`).
```bash
# Missing sorted set and member
> ZRANK zset_key missing_member
(nil)
> ZRANK missing_key member
(nil)
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `ZADD` command to create and add new members to it.

```bash
$ redis-cli
> ZADD zset_key 100.0 foo (Expecting ":1\r\n")
> ZADD zset_key 100.0 bar (Expecting ":1\r\n")
> ZADD zset_key 20.0 baz (Expecting ":1\r\n")
> ZADD zset_key 30.1 caz (Expecting ":1\r\n")
> ZADD zset_key 40.2 paz (Expecting ":1\r\n")

# Expected Ranks
# baz -> 0
# caz -> 1
# paz -> 2
# bar -> 3
# foo -> 4
```

The tester will then send multiple `ZRANK` commands specifying the members of the sorted set.
```bash
> ZRANK zset_key caz (Expecting ":1\r\n")
> ZRANK zset_key baz (Expecting ":0\r\n")
> ZRANK zset_key foo (Expecting ":4\r\n")
> ZRANK zset_key bar (Expecting ":3\r\n")
```

The tester will also send `ZRANK` commands where either the member or key doesn't exist.

```bash
> ZRANK zset_key missing_member (Expecting RESP bulk string "$-1\r\n")
> ZRANK missing_key member (Expecting RESP bulk string "$-1\r\n")
```

### Notes

No additional notes for this stage.

---

## Stage 95 — List sorted set members

| | |
|---|---|
| **Slug** | `ic1` |
| **Difficulty** | Easy |
| **Source** | [sorted-sets-04-ic1.md](stage_descriptions/sorted-sets-04-ic1.md) |

### What to implement

In this stage, you'll add support for listing the members of a sorted set using the `ZRANGE` command.

### The `ZRANGE` command
The `ZRANGE` command is used to list the members in a sorted set given a start index and an end index. The index of the first element is 0. The end index is inclusive, which means that the element at the end index will be included in the response.

Example usage:
```bash
> ZADD racer_scores 8.1 "Sam-Bodden"
(integer) 1
> ZADD racer_scores 10.2 "Royce"
(integer) 1
> ZADD racer_scores 6.0 "Ford"
(integer) 1
> ZADD racer_scores 14.1 "Prickett"
(integer) 1

# List members from index 0 to 2
> ZRANGE racer_scores 0 2
1) "Ford"
2) "Sam-Bodden"
3) "Royce"
```

Here are some additional notes on how the `ZRANGE` command behaves with different types of inputs:

- If the sorted set does not exist, an empty array (`*0\r\n`) is returned
- If the start index is greater than or equal to the cardinality of the sorted set, an empty array is returned.
- If the stop index is greater than the cardinality of the sorted set, the stop index is treated as the last element.
- If the start index is greater than the stop index, the result is an empty array.



### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then create a new sorted set with multiple members.

```bash
$ redis-cli
> ZADD zset_key 100.0 foo (Expecting ":1\r\n")
> ZADD zset_key 100.0 bar (Expecting ":1\r\n")
> ZADD zset_key 20.0 baz (Expecting ":1\r\n")
> ZADD zset_key 30.1 caz (Expecting ":1\r\n")
> ZADD zset_key 40.2 paz (Expecting ":1\r\n")
```

After that the tester will send your program a series of `ZRANGE` commands. It will expect the response to be a RESP array, or an empty array in each case, depending on the test case.

As an example, the tester might send your program a command like this.
```bash
> ZRANGE zset_key 2 4
# Expect RESP Encoded Array: ["paz", "bar", "foo"]
```

It will expect the response to be an RESP-encoded array `["paz", "bar", "foo"]`, which would look like this:
```
*3\r\n
$3\r\n
paz\r\n
$3\r\n
bar\r\n
$3\r\n
foo\r\n
```

The tester will issue multiple such commands and verify their responses.


### Notes


- In this stage, you will only implement `ZRANGE` with non-negative indexes. We will get to handling `ZRANGE` with negative indexes in later stages.

---

## Stage 96 — ZRANGE with negative indexes

| | |
|---|---|
| **Slug** | `bj4` |
| **Difficulty** | Easy |
| **Source** | [sorted-sets-05-bj4.md](stage_descriptions/sorted-sets-05-bj4.md) |

### What to implement

In this stage, you'll add support for negative indexes for the `ZRANGE` command.

### `ZRANGE` with negative indexes

The `ZRANGE` command can accept negative indexes too.

Example usage:

```bash
> ZADD racer_scores 8.5 "Sam-Bodden"
(integer) 1
> ZADD racer_scores 10.2 "Royce"
(integer) 1
> ZADD racer_scores 6.1 "Ford"
(integer) 1
> ZADD racer_scores 14.9 "Prickett"
(integer) 1
> ZADD racer_scores 10.2 "Ben"
(integer) 1


# List last 2 elements
> ZRANGE racer_scores -2 -1
1) "Royce"
2) "Prickett"

# List all items except last 2
> ZRANGE racer_scores 0 -3
1) "Ford"
2) "Sam-Bodden"
3) "Ben"
```

An index of -1 refers to the last element, -2 to the second last, and so on. If a absolute value of the negative index is out of range (i.e. >= the cardinality of the sorted set), it is treated as 0 (start of the sorted set).


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `ZADD` command to create a sorted set and add members to it.

```bash
$ redis-cli
> ZADD zset_key 20.0 foo (Expecting ":1\r\n")
> ZADD zset_key 30.1 bar (Expecting ":1\r\n")
> ZADD zset_key 40.2 baz (Expecting ":1\r\n")
> ZADD zset_key 25.0 paz (Expecting ":1\r\n")
> ZADD zset_key 25.0 caz (Expecting ":1\r\n")
```

The tester will then send your program a series of `ZRANGE` commands with one or more negative indexes.

For example, the tester might send you this command:

```bash
> ZRANGE zset_key 2 -1
1) "paz"
2) "bar"
3) "baz"
```

In this case, the tester will verify that the response is the array `["paz", "bar", "baz"]`, which is RESP Encoded as:

```
*3\r\n
$3\r\n
paz\r\n
$3\r\n
bar\r\n
$3\r\n
baz\r\n
```

### Notes

No additional notes for this stage.

---

## Stage 97 — Count sorted set members

| | |
|---|---|
| **Slug** | `kn4` |
| **Difficulty** | Easy |
| **Source** | [sorted-sets-06-kn4.md](stage_descriptions/sorted-sets-06-kn4.md) |

### What to implement

In this stage, you'll add support for counting the number of members in a sorted set using the `ZCARD` command.

### The `ZCARD` Command

The `ZCARD` command is used to query the cardinality (number of elements) of a sorted set. It returns an integer. The response is 0 if the sorted set specified does not exist.

```bash
> ZADD zset_key 1.2 "one"
(integer) 1
> ZADD zset_key 2.2 "two"
(integer) 1
> ZCARD zset_key
(integer) 2

> ZCARD missing_key
(integer) 0
```


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `ZADD` command to create and add members to it.

```bash
$ redis-cli
> ZADD zset_key 20.0 zset_member1 (Expecting ":1\r\n")
> ZADD zset_key 30.1 zset_member2 (Expecting ":1\r\n")
> ZADD zset_key 40.2 zset_member3 (Expecting ":1\r\n")
> ZADD zset_key 50.3 zset_member4 (Expecting ":1\r\n")
```

It will check the number of elements in the sorted set using the `ZCARD` command.
```bash
$ redis-cli ZCARD zset_key (Expecting ":4\r\n")
```

The tester will then update the score of an existing member.
```bash
$ redis-cli ZADD zset_key 100.0 zset_member1 (Expecting ":0\r\n")
```

It will again check the cardinality of the sorted set using the `ZCARD` command.
```bash
$ redis-cli ZCARD zset_key (Expecting ":4\r\n")
```

The tester will also check the cardinality of a non existing sorted set.
```bash
$ redis-cli ZCARD missing_key (Expecting ":0\r\n")
```

### Notes

No additional notes for this stage.

---

## Stage 98 — Retrieve member score

| | |
|---|---|
| **Slug** | `gd7` |
| **Difficulty** | Medium |
| **Source** | [sorted-sets-07-gd7.md](stage_descriptions/sorted-sets-07-gd7.md) |

### What to implement

In this stage, you'll add support for retrieving the score of a sorted set member using the `ZSCORE` command.

### The `ZSCORE` Command

The `ZSCORE` command is used to query the score of a member of a sorted set. If the sorted set and the member both exist, the score of the member is returned as a RESP bulk string.
```bash
> ZADD zset_key 24.34 "one"
(integer) 1
> ZADD zset_key 90.34 "two"
(integer) 1
> ZSCORE zset_key "one"
"24.34"
```

If the member or the sorted set specified in the argument does not exist, RESP null bulk string(`$-1\r\n`) is returned.
```bash
> ZSCORE zset_key "three"
(nil)
> ZSCORE missing_key "member"
(nil)
```



### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `ZADD` command to create a sorted set and add new members to it.

```bash
$ redis-cli
> ZADD zset_key 20.0 zset_member1 (Expecting ":1\r\n")
> ZADD zset_key 30.1 zset_member2 (Expecting ":1\r\n")
> ZADD zset_key 40.2 zset_member3 (Expecting ":1\r\n")
> ZADD zset_key 50.3 zset_member4 (Expecting ":1\r\n")
```

The tester will then send a `ZSCORE` command specifying one of the members. For example, the tester may send your program a command like this:

```bash
> ZSCORE zset_key zset_member2 (Expecting RESP bulk string "30.1")
```

It will expect the response to be "30.1", which is the score of `zset_member2`. The response is encoded as a RESP bulk string:
```
$4\r\n
30.1\r\n
```

The tester will then update the value of the member.
```bash
> ZADD zset_key 100.99 zset_member2 (Expecting ":0\r\n")
```

The tester will then send a `ZSCORE` command specifying the updated member.

```bash
> ZSCORE zset_key zset_member2 (Expecting RESP bulk string "100.99")
```

The tester will also send `ZSCORE` commands where either the member or the key doesn't exist.
```bash
> ZSCORE zset_key zset_member100 (Expecting RESP bulk string "$-1\r\n")
> ZSCORE missing_key member (Expecting RESP bulk string "$-1\r\n")
```

### Notes

No additional notes for this stage.

---

## Stage 99 — Remove a member

| | |
|---|---|
| **Slug** | `sq7` |
| **Difficulty** | Easy |
| **Source** | [sorted-sets-08-sq7.md](stage_descriptions/sorted-sets-08-sq7.md) |

### What to implement

In this stage, you'll add support for removing a member of a sorted set using the `ZREM` command.

## The `ZREM` command

The [`ZREM`](https://redis.io/docs/latest/commands/zrem/) command is used to remove a member from a sorted set given the member's name.

Example Usage:

```bash
> ZADD racer_scores 8.3 "Sam-Bodden"
(integer) 1
> ZADD racer_scores 10.5 "Royce"
(integer) 1

# Remove "Royce" from the sorted set
> ZREM racer_scores "Royce"
(integer) 1

# List the remaining members
> ZRANGE racer_scores 0 -1
1) "Sam-Bodden"

# Remove a non-existing member
> ZREM racer_scores "missing_member"
(integer) 0
```

It returns the number of members removed from the sorted set. If the specified member does not exist, 0 is returned.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then create a new sorted set with multiple members.

```bash
$ redis-cli
> ZADD zset_key 80.5 foo (Expecting ":1\r\n")
> ZADD zset_key 50.3 baz (Expecting ":1\r\n")
> ZADD zset_key 80.5 bar (Expecting ":1\r\n")
```

After that the tester will send your program a `ZREM` command specifying the member to be removed.

As an example, the tester might send your program a command like this.
```bash
> ZREM zset_key "baz"
# Expected: (integer) 1
```

It will then send your program a `ZRANGE` command and check for the remaining members.

```bash
> ZRANGE zset_key 0 -1
# Expected RESP Encoded Array: ["bar", "foo"]
```

The tester will also send a `ZREM` command where the member doesn't exist.
```bash
> ZREM zset_key "missing_member"
# Expected: (integer) 0
```

### Notes

No additional notes for this stage.

---

## Stage 100 — Respond to GEOADD

| | |
|---|---|
| **Slug** | `zt4` |
| **Difficulty** | Easy |
| **Source** | [geospatial-01-zt4.md](stage_descriptions/geospatial-01-zt4.md) |

### What to implement

In this stage, you'll add support for responding to the `GEOADD` command.

### Extension prerequisites

This stage depends on the [**Sorted Sets**](https://redis.io/docs/latest/data-types/sorted-sets/) extension. Before attempting this extension, please make sure you've completed the Sorted Sets extension.

### The `GEOADD` command

The [`GEOADD` command](https://redis.io/docs/latest/commands/geoadd/) adds a location (with longitude, latitude, and name) to a key.

Example usage:

```bash
> GEOADD places -0.0884948 51.506479 "London"
(integer) 1
```

The arguments `GEOADD` accepts are:

1. `key`: The key to store the location in.
2. `longitude`: The longitude of the location.
3. `latitude`: The latitude of the location.
4. `member`: The name of the location.

It returns the count of elements added, encoded as a RESP Integer.

In this stage, you'll only implement the response to the `GEOADD` command. We'll get to validating arguments and storing locations in later stages.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `GEOADD` command:

```bash
$ redis-cli GEOADD places 11.5030378 48.164271 Munich
```

The tester will expect the response to be `:1\r\n`, which is 1 (number of locations added) encoded as a RESP integer.


### Notes


- In this stage, you only need to implement responding to the `GEOADD` command. We'll get to validating arguments and storing locations in later stages.

---

## Stage 101 — Validate coordinates

| | |
|---|---|
| **Slug** | `ck3` |
| **Difficulty** | Easy |
| **Source** | [geospatial-02-ck3.md](stage_descriptions/geospatial-02-ck3.md) |

### What to implement

In this stage, you'll add support for validating the latitude and longitude values provided in a `GEOADD` command.

### Validating latitude and longitude values

The latitude and longitude values used in the `GEOADD` command should be in a certain range as per [EPSG:3857](https://epsg.io/3857).

- Valid longitudes are from -180° to +180°
  - Both these limits are inclusive, so -180° and +180° are both valid.
- Valid latitudes are from -85.05112878° to +85.05112878°
  - Both of these limits are inclusive, so -85.05112878° and +85.05112878° are both valid.
  - The reason these limits are not -/+90° is because of the [Web Mercator projection](https://en.wikipedia.org/wiki/Web_Mercator_projection) that Redis uses.

If either of these values aren't within the appropriate range, `GEOADD` returns an error. Examples:

```bash
# Invalid latitude
> GEOADD places 180 90 test1
(error) ERR invalid longitude,latitude pair 180.000000,90.000000

# Invalid longitude
> GEOADD places 181 0.3 test2
(error) ERR invalid longitude,latitude pair 181.000000,0.300000
```

In this stage, you'll implement validating latitude and longitude values and returning error messages as shown above. The tester is lenient with error message formats, so you don't have to use the exact error message format mentioned above.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send multiple `GEOADD` commands.

If the coordinates are invalid, it will expect an error response. For example:

```bash
# Expecting error
$ redis-cli GEOADD location_key 200 100 foo
(error) ERR invalid longitude,latitude pair 200,100
```

The returned value must:

- Be a [RESP simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-errors) i.e. start with `-` and end with `\r\n`
- The error message must start with `ERR`, like standard Redis error messages
- The error message must contain the word "latitude" if the latitude is invalid
- The error message must contain the word "longitude" if the longitude is invalid

For example, if the latitude is invalid, valid error messages that the tester will accept are:

```bash
-ERR invalid latitude argument\r\n
-ERR invalid latitude value\r\n
-ERR latitude value (200.0) is invalid\r\n
```


### Notes


- You don't need to implement storing locations yet, we'll get to that in later stages.
- The boundary of latitude are clipped at -/+85.05112878° instead of -/+90°. This is because of the [Web Mercator projection](https://en.wikipedia.org/wiki/Web_Mercator_projection) that Redis uses.

---

## Stage 102 — Store a location

| | |
|---|---|
| **Slug** | `tn5` |
| **Difficulty** | Medium |
| **Source** | [geospatial-03-tn5.md](stage_descriptions/geospatial-03-tn5.md) |

### What to implement

In this stage, you'll add support for storing locations in a sorted set.

### Storing locations in sorted set

The locations added using the `GEOADD` command are stored in a sorted set. Redis internally calculates a score for the specified location using a location's latitude and longitude.

For example, the following two commands are equivalent in Redis.

```bash
# Adding a location
$ redis-cli GEOADD places_key 2.2944692 48.8584625 location

# This command is equivalent to the command above
$ redis-cli ZADD places_key 3663832614298053 location
```

In this stage, you'll implement adding locations to a sorted set when a `GEOADD` command is run.

You can hardcode the score to be 0 for now, we'll get to calculating the score in later stages.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send a `GEOADD` command:

```bash
$ redis-cli GEOADD places 2.2944692 48.8584625 Paris
# Expect: (integer) 1
```

The tester will then send a `ZRANGE` command to validate that the location was added to the sorted set:

```bash
$ redis-cli ZRANGE places 0 -1
# Expect RESP Array: ["Paris"]
```


### Notes


- In this stage, you can hardcode the score of the location to be 0. We'll get to calculating the value of score using latitude and longitude in later stages.
- The implementation of the `ZRANGE` command is covered in the sorted sets extension.

---

## Stage 103 — Calculate location score

| | |
|---|---|
| **Slug** | `cr3` |
| **Difficulty** | Hard |
| **Source** | [geospatial-04-cr3.md](stage_descriptions/geospatial-04-cr3.md) |

### What to implement

In this stage, you'll add support for calculating the score of a location.

### Location scores

To store locations in a sorted set, Redis converts latitude and longitude values to a single "score".

We've created a [GitHub repository](https://github.com/codecrafters-io/redis-geocoding-algorithm) that explains how this conversion is done. It includes:

- A description of the algorithm used, along with pseudocode
- Code samples in multiple languages.
- A set of locations & scores to test against

Here's the [repository link](https://github.com/codecrafters-io/redis-geocoding-algorithm).


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send multiple `GEOADD` commands:

```bash
$ redis-cli GEOADD places 2.2944692 48.8584625 Paris
# Expect: (integer) 1
$ redis-cli GEOADD places -0.127758 51.507351 London
# Expect: (integer) 1
```

The tester will validate that scores are calculated correctly by sending multiple `ZSCORE` commands:

```bash
$ redis-cli ZSCORE places Paris
# Expecting bulk string: "3663832614298053"
```

The calculated scores must match the expected values as described in [this repository](https://github.com/codecrafters-io/redis-geocoding-algorithm).

### Notes

No additional notes for this stage.

---

## Stage 104 — Respond to GEOPOS

| | |
|---|---|
| **Slug** | `xg4` |
| **Difficulty** | Easy |
| **Source** | [geospatial-05-xg4.md](stage_descriptions/geospatial-05-xg4.md) |

### What to implement

In this stage, you'll add support for responding to the `GEOPOS` command.

### The `GEOPOS` command

The `GEOPOS` command returns the longitude and latitude of the specified location.

Example usage:

```bash
> GEOADD places -0.0884948 51.506479 "London"
> GEOADD places 11.5030378 48.164271 "Munich"

> GEOPOS places London
1) 1) "-0.08849412202835083"
   2) "51.50647814139934"

> GEOPOS places Munich
1) 1) "11.503036916255951"
   2) "48.16427086232978"
```

It returns an array with one entry for each location requested.

- If a location exists under the key, its entry is an array of two items:
  - Longitude (Encoded as a [RESP Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings))
  - Latitude (Encoded as a [RESP Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings))
- If either the location or key don’t exist, the corresponding entry is a [null array](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays) `(*-1\r\n)`.

To return the latitude and longitude values, Redis decodes the "score" back to latitude and longitude values. We'll cover this process in later stages, for now you can hardcode the returned latitude and longitude values to be 0 (or any number).


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then add multiple locations using the `GEOADD` command.

```bash
$ redis-cli
> GEOADD location_key -0.0884948 51.506479 "London"
# Expect: (integer) 1
> GEOADD location_key 11.5030378 48.164271 "Munich"
# Expect: (integer) 1
```

The tester will then send multiple `GEOPOS` commands:

```bash
> GEOPOS location_key London Munich
# Expecting: [["0", "0"], ["0", "0"]], encoded as "*2\r\n*2\r\n$1\r\n0\r\n$1\r\n0\r\n*2\r\n$1\r\n0\r\n$1\r\n0\r\n"

> GEOPOS location_key missing_location
# Expecting: [nil], encoded as "*1\r\n*-1\r\n"
```

The tester will assert that:

- The response is a RESP array that contains as many elements as the number of locations requested
- For each location requested:
  - If the location exists:
    - The corresponding element is a RESP array with two elements (i.e. longitude and latitude)
    - Both elements are "0" (or any other valid floating point number), encoded as a RESP bulk string
  - If the location doesn't exist:
    - The corresponding element is a [null array](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays) `(*-1\r\n)`.

The tester will also send a `GEOPOS` command using a key that doesn't exist:

```bash
> GEOPOS missing_key London Munich
# Expecting [nil, nil], encoded as "*2\r\n*-1\r\n*-1\r\n"
```

The tester will assert that:

- The response is a RESP array that contains as many elements as the number of locations requested
- Each element of the array is a [null array](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-arrays) `(*-1\r\n)`


### Notes


- In this stage, you can hardcode the returned latitude and longitude values to be 0 (or any other valid floating point number). We'll get to testing actual latitude and longitude values in later stages.

---

## Stage 105 — Decode coordinates

| | |
|---|---|
| **Slug** | `hb5` |
| **Difficulty** | Hard |
| **Source** | [geospatial-06-hb5.md](stage_descriptions/geospatial-06-hb5.md) |

### What to implement

In this stage, you'll add support for decoding the coordinates of a location.

### Decoding latitude and longitude from score

The algorithm to get back the latitude and longitude is essentially the reverse of the one used to compute the score from them.

The [GitHub repository](https://github.com/codecrafters-io/redis-geocoding-algorithm) we referenced earlier explains how this conversion is done. It includes:

- A description of the algorithm used, along with pseudocode
- Code samples in multiple languages.
- A set of locations & scores to test against

Here's the [repository link](https://github.com/codecrafters-io/redis-geocoding-algorithm).


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will add multiple locations using the `ZADD` command. The scores used are valid scores that can be converted back to latitude and longitude values.

```bash
$ redis-cli
> ZADD location_key 3663832614298053 "Foo"
> ZADD location_key 3876464048901851 "Bar"
> ZADD location_key 3468915414364476 "Baz"
> ZADD location_key 3781709020344510 "Caz"
```

The tester will then send multiple `GEOPOS` commands:

```bash
> GEOPOS location_key Foo
# Expecting [["2.294471561908722", "48.85846255040141"]]
```

The tester will validate that the response is a RESP array, which is encoded as:

```
*1\r\n
*2\r\n
$17\r\n
2.294471561908722\r\n
$17\r\n
48.85846255040141\r\n
```


### Notes


- The conversion from latitude/longitude to score and back is lossy, so the tester will be lenient in checking the coordinates provided - it will accept any coordinates that are within 6 decimal places of the original values. For example, for the example shown above, any of the following values will be accepted:
  - `["2.2944715", "48.8584625"]`
  - `["2.294472", "48.858463"]`
  - `["2.294471594", "48.858462987"]`

---

## Stage 106 — Calculate distance

| | |
|---|---|
| **Slug** | `ek6` |
| **Difficulty** | Medium |
| **Source** | [geospatial-07-ek6.md](stage_descriptions/geospatial-07-ek6.md) |

### What to implement

In this stage, you'll add support for calculating the distance between two locations using the `GEODIST` command.

### The `GEODIST` command

The `GEODIST` command returns the distance between two members of a key.

Example usage:

```bash
> GEODIST places Munich Paris
"682477.7582"
```

The distance is returned in meters, encoded as a [RESP bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).

Redis uses the [Haversine's Formula](https://rosettacode.org/wiki/Haversine_formula) to calculate the distance between two points. You can see how this is done in the Redis source code [here](https://github.com/redis/redis/blob/4322cebc1764d433b3fce3b3a108252648bf59e7/src/geohash_helper.c#L228C1-L228C72).


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then add multiple locations using the `GEOADD` command:

```bash
$ redis-cli
> GEOADD places 11.5030378 48.164271 "Munich"
> GEOADD places 2.2944692 48.8584625 "Paris"
```

The tester will then send multiple `GEODIST` commands specifying two locations:

```bash
> GEODIST places Munich Paris
# Expecting "682477.7582"
```

The tester will validate that the response is a RESP bulk string that contains the distance between the two locations, for example:

```bash
$11\r\n682477.7582\r\n
```


### Notes


- When computing distance, set the value of Earth's radius to 6372797.560856 meters. This is the exact value [used by Redis](https://github.com/redis/redis/blob/35aacdf80a0871c933047fc46655b98a73a9374e/src/geohash_helper.c#L52).

- The distance should match the actual distance between the provided locations with a precision of up to 4 decimal places after rounding. For example, if the expected distance is 12345.6789, any of the following returned values will be accepted:
  - `12345.67891`
  - `12345.6789`
  - `12345.67889`

---

## Stage 107 — Search within radius

| | |
|---|---|
| **Slug** | `rm9` |
| **Difficulty** | Easy |
| **Source** | [geospatial-08-rm9.md](stage_descriptions/geospatial-08-rm9.md) |

### What to implement

In this stage, you'll add support for searching locations within a given radius using the `GEOSEARCH` command.

### The GEOSEARCH command

The `GEOSEARCH` command lets you search for locations within a given radius.

It supports several search modes. In our implementation, we'll focus only on the `FROMLONLAT` mode. The `FROMLONLAT` mode searches by directly specifying longitude and latitude.

Example usage:

```bash
> GEOSEARCH places FROMLONLAT 2 48 BYRADIUS 100 m
1) "Paris"
```

The example command above searches for locations in the `places` key that are within 100 meters of the point (longitude: 2, latitude: 48).

The response is a RESP array containing the names of the locations that match the search criteria, each encoded as a [RESP bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).

Note that there are two options we passed to the command:

- `FROMLONLAT <longitude> <latitude>` — This option specifies the center point for the search.
- `BYRADIUS <radius> <unit>` — This option searches within a circular area of the given radius and unit (m, km, mi, etc.).

Redis supports other such options, but in this challenge we'll only use `FROMLONLAT` and `BYRADIUS`.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will add multiple locations using the `GEOADD` command.

```bash
$ redis-cli
> GEOADD places 11.5030378 48.164271 "Munich"
> GEOADD places 2.2944692 48.8584625 "Paris"
> GEOADD places -0.0884948 51.506479 "London"
```

The tester will then send multiple `GEOSEARCH` commands:

```bash
> GEOSEARCH places FROMLONLAT 2 48 BYRADIUS 100000 m
# Expecting ["Paris"]

> GEOSEARCH places FROMLONLAT 2 48 BYRADIUS 500000 m
# Expecting ["Paris, "London"] (Any order)

> GEOSEARCH places FROMLONLAT 11 50 BYRADIUS 300000 m
# Expecting ["Munich"]
```

The tester will validate that the response is a RESP array, for example

```
*2\r\n
$5\r\n
Paris\r\n
$6\r\n
London\r\n
```

Locations can be returned in any order.


### Notes


- The tester will always use the `FROMLONLAT` and `BYRADIUS` options when sending a `GEOSEARCH` command.

---

## Stage 108 — Respond to ACL WHOAMI

| | |
|---|---|
| **Slug** | `jn4` |
| **Difficulty** | Easy |
| **Source** | [auth-01-jn4.md](stage_descriptions/auth-01-jn4.md) |

### What to implement

In this stage, you'll add support for responding to the `ACL WHOAMI` command.

### The `ACL WHOAMI` Command

The [`ACL WHOAMI`](https://redis.io/docs/latest/commands/acl-whoami/) command returns the username associated with the current connection. 

By default, every new connection in Redis is automatically authenticated as the `default` user. 

For example:

```bash
> ACL WHOAMI
"default"
```

The command returns the username of the currently authenticated user, encoded as a [RESP bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).

This default authentication behavior can be changed so that new connections start out unauthenticated. We'll cover that in later stages.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `ACL WHOAMI` command:

```bash
# Expect RESP bulk string: "default"
$ redis-cli ACL WHOAMI
"default"
```

The tester will expect to receive `$7\r\ndefault\r\n` as a response. That's the string `default` encoded as a [bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-strings).


### Notes


- In this stage, you can hardcode the response of the `ACL WHOAMI` command to be `default`. We'll get to enforcing authentication in the later stages.

---

## Stage 109 — Respond to ACL GETUSER

| | |
|---|---|
| **Slug** | `gx8` |
| **Difficulty** | Easy |
| **Source** | [auth-02-gx8.md](stage_descriptions/auth-02-gx8.md) |

### What to implement

In this stage, you'll add support for the `ACL GETUSER` command.

### The `ACL GETUSER` Command

The [`ACL GETUSER`](https://redis.io/docs/latest/commands/acl-getuser/) command retrieves the properties of a specified user. In Redis, the `default` user is present from the start and does not need to be created.

For example:

```bash
> ACL GETUSER default
1) "flags"
2) (empty array)
...
```

The return value is a nested RESP array of property name-value pairs for the user:

```bash
[property_name_1, property_value_1, property_name_2, property_value_2, ...]
```

For this stage, you'll implement just the `flags` property. 

### The `flags` Property

The `flags` property represents a set of attributes that describe how a user behaves or what special permissions they have. Each flag is a short label that defines part of the user’s configuration.

For example, after creating or modifying a user, the flags array might look like this:

```bash
> ACL GETUSER alice
1) "flags"
2) 1) "on"
   2) "allkeys"
   3) "allcommands"
```

For this stage, since the `default` user has no `flags` to report yet, you will hardcode the value to be an empty RESP array (`[]`). 


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `ACL GETUSER` command specifying the `default` user:

```bash
# Expect RESP array: ["flags", []]
$ redis-cli
> ACL GETUSER default
1) "flags"
2) (empty array)
```

The tester will verify that the response is a RESP array with two elements:

1. The first element is the bulk string `flags`.
2. The second element is an empty RESP array.

### Notes

No additional notes for this stage.

---

## Stage 110 — The nopass flag

| | |
|---|---|
| **Slug** | `ql6` |
| **Difficulty** | Easy |
| **Source** | [auth-03-ql6.md](stage_descriptions/auth-03-ql6.md) |

### What to implement

In this stage, you'll add support for responding to the `ACL GETUSER` command with the `nopass` flag set.

### The `nopass` flag

The `nopass` flag is one of the user flags in Redis. It controls the password authentication behaviour:

- If `nopass` is set for a user, authentication succeeds with any password (or no password)
- Setting `nopass` clears any passwords associated with the user

The `default` user has `nopass` set by default, which is why new connections are automatically authenticated.

For example:
```bash
> ACL GETUSER default
1) "flags"
2) 1) "nopass"
...
```

The flags are encoded as a RESP array of bulk strings. 

For this stage, you only need to respond to the `ACL GETUSER` command with the `nopass` flag set. We'll get to enforcing the behavior of the `nopass` flag in later stages.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `ACL GETUSER` command specifying the `default` user.

```bash
# Expect RESP array: ["flags", ["nopass"]]
$ redis-cli
> ACL GETUSER default
1) "flags"
2) 1) "nopass"
```

The tester will verify that the response is a RESP array with two elements:
1. The first element is the bulk string `flags`.
2. The second element is a RESP array containing one element: the bulk string `nopass`.

### Notes

No additional notes for this stage.

---

## Stage 111 — The passwords property

| | |
|---|---|
| **Slug** | `pl7` |
| **Difficulty** | Easy |
| **Source** | [auth-04-pl7.md](stage_descriptions/auth-04-pl7.md) |

### What to implement

In this stage, you'll add support for responding to the `ACL GETUSER` command with the `passwords` property.

### The `passwords` Property

The `passwords` property lists all hash-encoded passwords associated with a user. 

The `default` user does not have any associated passwords unless explicitly configured:

```bash
> ACL GETUSER default
 1) "flags"
 2) 1) "nopass"
 3) "passwords"
 4) (empty array)
```

Your `ACL GETUSER` response must now include the following:

1. The string `passwords`, encoded as a bulk string.
2. A RESP array containing the list of passwords. Since the `default` user has none, you must hardcode this to be an empty array.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `ACL GETUSER` command specifying the `default` user:

```bash
# Expect RESP array: ["flags", ["nopass"], "passwords", []]
$ redis-cli ACL GETUSER default
 1) "flags"
 2) 1) "nopass"
 3) "passwords"
 4) (empty array)
```

The tester will validate that the response is a RESP array with four elements:

1. The first element of the array is the string `flags`, encoded as a RESP bulk string.
2. The second element of the array is a RESP array and contains the `nopass` flag.
3. The third element of the array is the string `passwords`, encoded as a bulk string.
4. The fourth element of the array is an empty array.

### Notes

No additional notes for this stage.

---

## Stage 112 — Setting default user password

| | |
|---|---|
| **Slug** | `uv9` |
| **Difficulty** | Medium |
| **Source** | [auth-05-uv9.md](stage_descriptions/auth-05-uv9.md) |

### What to implement

In this stage, you'll add support for setting the `default` user's password.

### The `ACL SETUSER` Command

The [`ACL SETUSER`](https://redis.io/docs/latest/commands/acl-setuser/) command modifies the properties of an existing user. 

When the command is used with the `>` rule, it adds a password for the specified user:

```bash
> ACL SETUSER default >mypassword
OK
```

The server responds with `OK` encoded as a RESP simple string (`+OK\r\n`).

### Password Storage

Adding a password for a user with `ACL SETUSER` has two effects:
- The password is stored as a SHA-256 hash.
- The `nopass` flag is automatically removed.

For example:

```bash
> ACL SETUSER default >mypassword
OK

> ACL GETUSER default
 1) "flags"
 2) (empty array)
 3) "passwords"
 4) 1) "89e01536ac207279409d4de1e5253e01f4a1769e696db0d6062ca9b8f56767c8"
```

Notice that the `nopass` flag is now gone from the `flags` array. Also, the `mypassword` SHA-256 hash is stored as a bulk string in the `passwords` array. 

Storing only the SHA-256 hash is a security best practice Redis uses to prevent password leaks if the database is compromised.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `ACL GETUSER` command, specifying the `default` user:

```bash
# Expect RESP array: ["flags", ["nopass"], "passwords", []]
$ redis-cli ACL GETUSER default
1) "flags"
2) 1) "nopass"
3) "passwords"
4) (empty array)
```

The tester will verify that:

- The `nopass` flag is present in the flags array.
- The `passwords` array is empty.

Next, the tester will send a `ACL SETUSER` command, specifying the `default` user and a password:

```bash
# Expect: +OK\r\n
> ACL SETUSER default >mypassword
OK
```

Your server must respond with `+OK\r\n`.

Finally, the tester will send a `ACL GETUSER` command, specifying the `default` user:

```bash
# Expect RESP array: ["flags", ["nopass"], "passwords", ["89e01536ac207279409d4de1e5253e01f4a1769e696db0d6062ca9b8f56767c8"]]
> ACL GETUSER default
1) "flags"
2) 1) "nopass"
3) "passwords"
4) 1) "89e01536ac207279409d4de1e5253e01f4a1769e696db0d6062ca9b8f56767c8"
```

The tester will validate the following for your response:

- The `nopass` flag is no longer present.
- The `passwords` array contains the SHA-256 hash of `mypassword` encoded as a bulk string.


### Notes


- Redis uses the SHA-256 hashing algorithm for password storage. You'll need to compute the SHA-256 hash of the provided password and store it.
- The password hash should be stored as a lowercase hexadecimal string.

---

## Stage 113 — The AUTH command

| | |
|---|---|
| **Slug** | `hz3` |
| **Difficulty** | Medium |
| **Source** | [auth-06-hz3.md](stage_descriptions/auth-06-hz3.md) |

### What to implement

In this stage, you'll add support for the `AUTH` command.

### The `AUTH` Command

The [`AUTH`](https://redis.io/docs/latest/commands/auth/) command authenticates the current connection with a specified username and password.

The command format is:

```bash
AUTH <username> <password>
```

For example:

```bash
# Authentication failure
> AUTH default wrongpassword
(error) WRONGPASS invalid username-password pair or user is disabled.

# Authentication success
> AUTH default correctpassword
OK
```

The server responds with `+OK\r\n` if the specified password's hash matches any of the hashes in the user's password list.

If no match is found, the server responds with the [RESP simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-errors): `WRONGPASS invalid username-password pair or user is disabled.`

For this stage, you just need to respond to the `AUTH` command appropriately. You don't need to actually authenticate the connection. We'll get to that in later stages.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send an `ACL SETUSER` command, specifying the `default` user and a password:

```bash
$ redis-cli ACL SETUSER default >mypassword
OK
```

Your server should respond with `OK` encoded as a simple string (`+OK\r\n`).

Next, the tester will send two `AUTH` commands: one with a wrong password and another with a correct password.

```bash
# Expect error starting with: WRONGPASS
> AUTH default wrongpassword
(error) WRONGPASS invalid username-password pair or user is disabled.

# Expect: +OK\r\n
> AUTH default mypassword
OK
```

The tester will verify that:
- Wrong passwords return a [simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-errors) starting with `WRONGPASS`.
- Correct passwords return `+OK\r\n`.


### Notes


- The tester will be lenient in checking error messages. Any authentication failure starting with `WRONGPASS` is valid. For example:
    - `WRONGPASS wrong password`
    - `WRONGPASS invalid authentication`

---

## Stage 114 — Enforce authentication

| | |
|---|---|
| **Slug** | `nm2` |
| **Difficulty** | Medium |
| **Source** | [auth-07-nm2.md](stage_descriptions/auth-07-nm2.md) |

### What to implement

In this stage, you'll add support for enforcing authentication for the `default` user.

### Enforcing `default` User Authentication

When you create a new connection, it is automatically authenticated as the `default` user. This happens because the `nopass` flag is set for the `default` user from the start.

Once you set a password for the `default` user, new connections will no longer be automatically authenticated. However, any connections that were already authenticated will stay logged in.

For example:

```bash
# Client 1
$ redis-cli ACL SETUSER default >password
OK

# This connection remains authenticated as the default user
> ACL WHOAMI
"default"

# Client 2
# This connection is not authenticated
$ redis-cli ACL WHOAMI
(error) NOAUTH Authentication required.
```

When an unauthenticated connection tries to execute a command, return the simple error: `NOAUTH Authentication required.`


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send commands to two different clients:

```bash
# Client 1
$ redis-cli
# Expect: +OK\r\n
> ACL SETUSER default >newpassword
OK

# Expect RESP bulk string: "default"
> ACL WHOAMI
"default"

# Client 2 (new connection)
$ redis-cli
# Expect error starting with: "NOAUTH"
> ACL WHOAMI
(error) NOAUTH Authentication required.
```

The tester will verify that:
- Client 1 can still execute commands and remains authenticated as the `default` user.
- Client 2 receives a `NOAUTH` error when trying to execute commands without authentication.

### Notes

No additional notes for this stage.

---

## Stage 115 — Authenticate using AUTH

| | |
|---|---|
| **Slug** | `ws7` |
| **Difficulty** | Medium |
| **Source** | [auth-08-ws7.md](stage_descriptions/auth-08-ws7.md) |

### What to implement

In this stage, you'll implement enforcing authentication using the `AUTH` command.

### Enforcing Authentication Using `AUTH`

After the `AUTH` command succeeds, the connection becomes authenticated as the specified user. Once authenticated, the connection can execute commands that previously returned `NOAUTH` errors:

For example:

```bash
# Client 1
$ redis-cli
> ACL SETUSER default >newpassword
OK

> ACL WHOAMI
"default"

# Client 2 (new connection)
$ redis-cli
> ACL WHOAMI
(error) NOAUTH Authentication required.

> AUTH default newpassword
OK

# Client 2 is now authenticated as the 'default' user
> ACL WHOAMI
"default"
```

The authentication lasts for the entire connection, so the client does not need to re-authenticate for every command.


### Tests


The tester will execute your program like this:

```bash
$ ./your_program.sh
```

It will then send commands to two different clients:

```bash
# Client 1
$ redis-cli
> ACL SETUSER default >newpassword
OK

> ACL WHOAMI
"default"  # Still authenticated

# Client 2 (new connection)
$ redis-cli
> PING
(error) NOAUTH Authentication required.

> AUTH default newpassword
OK

> PING
PONG
```

The tester will verify that:

1. A new client receives a `NOAUTH` error when attempting to execute commands before authenticating.
2. The `AUTH` command returns `OK` as a simple string on successful authentication.
3. The client can execute commands successfully after authentication.


### Notes


- All commands except `AUTH` should return `NOAUTH` errors when executed by an unauthenticated connection. This rule applies to every command you have implemented, not only to `ACL` commands.

---

# YOUR CUSTOM STAGES (not in CodeCrafters)

These features are in **your Redis clone project** but not part of the CodeCrafters challenge. Build these after the phases above.

---

## Custom Stage A — Sets (SADD, SREM, SMEMBERS, SISMEMBER, SCARD)

Add Set data type to your store. Members are unique strings stored in `map[string]struct{}`.

- [SADD docs](https://redis.io/docs/latest/commands/sadd/)
- [SMEMBERS docs](https://redis.io/docs/latest/commands/smembers/)

---

## Custom Stage B — Hashes (HSET, HGET, HDEL, HGETALL, HEXISTS, HLEN)

Add Hash data type. A hash is a `map[string]string` stored under a key.

- [HSET docs](https://redis.io/docs/latest/commands/hset/)
- [HGETALL docs](https://redis.io/docs/latest/commands/hgetall/)

---

## Custom Stage C — Key Management (DEL, EXISTS, TYPE, KEYS, RENAME)

- [DEL](https://redis.io/docs/latest/commands/del/) — delete keys
- [EXISTS](https://redis.io/docs/latest/commands/exists/) — count existing keys
- [TYPE](https://redis.io/docs/latest/commands/type/) — return type ("string", "list", "set", "hash", "none")
- [KEYS](https://redis.io/docs/latest/commands/keys/) — glob pattern matching
- [RENAME](https://redis.io/docs/latest/commands/rename/) — rename a key

---

## Custom Stage D — TTL Commands (EXPIRE, PEXPIRE, TTL, PTTL, PERSIST)

- [EXPIRE](https://redis.io/docs/latest/commands/expire/) — set TTL in seconds
- [TTL](https://redis.io/docs/latest/commands/ttl/) — get remaining TTL (-1 = no expiry, -2 = doesn't exist)
- [PERSIST](https://redis.io/docs/latest/commands/persist/) — remove expiry

---

## Custom Stage E — MSET, MGET, APPEND, DECR, INCRBY, DECRBY

Batch string commands and numeric operations.

---

## Custom Stage F — Sharded Store (64 shards)

Replace single map+mutex with 64 shards using FNV-1a hash distribution. Each shard gets its own `sync.RWMutex`.

---

## Custom Stage G — Active Expiry (Background Goroutine)

Every 100ms, sample 20 random keys per shard. If >25% expired, resample. This is the [Redis probabilistic expiry algorithm](https://redis.io/docs/latest/commands/expire/#how-redis-expires-keys).

---

## Custom Stage H — Server Commands (DBSIZE, FLUSHDB, INFO, CONFIG)

- DBSIZE — count non-expired keys
- FLUSHDB — delete everything
- INFO — server stats (uptime, clients, commands)

---

## Custom Stage I — Graceful Shutdown + Docker + Makefile

- Listen for SIGINT/SIGTERM, drain connections, close AOF
- Multi-stage Dockerfile
- Makefile with build/run/test targets
