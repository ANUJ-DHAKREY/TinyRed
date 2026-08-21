# TinyRed Testing Guide

This guide gives you a practical way to test Redis stages like an infrastructure project, not just a small library.

## 1) General Testing Concepts

### Testing pyramid for backend/infra services

1. Unit tests
- Fast, isolated, no network.
- Validate pure logic: parsing, command dispatch, data structures.

2. Integration tests
- Real sockets/processes.
- Validate protocol behavior end-to-end.

3. Contract/protocol tests
- Validate exact wire format and edge cases.
- For Redis clone projects this is critical (RESP encoding/decoding).

4. System/e2e tests
- Full server process with realistic scenarios: concurrency, expiry timing, persistence, replication.

### Why this matters for Redis-like servers
- Your users are clients and other processes, not Go functions.
- Correctness is on the wire (TCP + RESP), not just in-memory behavior.
- Timing and ordering issues (timeouts, concurrency, expiry) are common failure points.

## 2) How Testing Works in Go

### Core tools
- `go test ./...`: run all tests.
- `go test -run TestName`: run specific tests.
- `go test -v`: verbose output.
- `go test -race`: detect data races (very important for concurrent servers).
- `go test -cover ./...`: coverage report.

### Common patterns
- Table-driven tests for many command variants.
- Subtests (`t.Run`) for stage-specific scenarios.
- Helper functions (`t.Helper`) to avoid duplicate setup.
- Deterministic timeouts using `SetReadDeadline`.

### Good Go testing habits
- Keep each test focused on one behavior.
- Avoid flaky sleeps; prefer deadlines and retries.
- Start a fresh server per test when state isolation matters.
- Fail with clear expected vs actual wire responses.

## 3) How Infra Projects Usually Test

### Black-box first
Infrastructure teams usually test through public interfaces:
- Start process
- Send network requests
- Assert protocol output

This mirrors production use and catches integration bugs early.

### Golden protocol assertions
For protocol servers, assert exact RESP bytes:
- `+PONG\r\n`
- `$-1\r\n`
- Arrays, bulk strings, and errors exactly as specified.

### Deterministic environment
- Use random free port (`127.0.0.1:0` during test setup).
- Use strict timeouts.
- Clean process lifecycle in test cleanup.

### Concurrency + race checks
- Add `go test -race` in your default workflow.
- Add parallel client tests once features are stable.

## 4) What Was Added In This Repo

Added file: [base_stage_behavior_test.go](base_stage_behavior_test.go)

It includes:
- Stage-wise integration tests for base stages 1 to 7
- A reusable test harness that:
  - starts TinyRed as a subprocess (`go run . --port <port>`)
  - dials TCP clients
  - sends RESP commands
  - validates responses

### Stage gating
By default, tests assume up to stage 7.

Control with:
- `TINYRED_BASE_STAGE=<n>`

Examples:
- `TINYRED_BASE_STAGE=3 go test -v -run BaseStage ./...`
- `TINYRED_BASE_STAGE=7 go test -v -run BaseStage ./...`

This lets you run only the stages you have implemented while keeping future tests ready.

## 5) Recommended Command Workflow

From project directory:

```bash
go test -v -run BaseStage
```

When working stage by stage:

```bash
TINYRED_BASE_STAGE=4 go test -v -run BaseStage
```

Before commit:

```bash
go test -race ./...
```

## 6) How To Extend For Next Stages

Use the same pattern already in [base_stage_behavior_test.go](base_stage_behavior_test.go):

1. Add a new `Test...` function for the stage.
2. Start server via `startTinyRed`.
3. Send RESP with `writeRESPArray`.
4. Assert exact wire response (`readLine` / `readBulkString`).
5. Add stage gate with `requireStage(t, stageNumber)`.

Examples for future:
- Lists: verify `RPUSH`, `LRANGE` array responses and index edge cases.
- Transactions: verify `MULTI/EXEC` queue behavior and error entries.
- Pub/Sub: validate subscriber mode and asynchronous message delivery.
- Persistence: restart process and assert restored state.

## 7) Suggested Next Improvement

As your code grows, split tests by feature:
- `base_stage_behavior_test.go`
- `lists_stages_test.go`
- `transactions_stages_test.go`
- `pubsub_stages_test.go`

This keeps stage debugging fast and readable.

## 8) Redis Client Compatibility Checklist

Use this checklist to decide whether TinyRed is ready for `redis-cli`, common Redis libraries, and stricter client behavior.

### 8.1 Must-have for `redis-cli`

- RESP2 parsing for arrays, bulk strings, simple strings, integers, nulls, and errors.
- Correct CRLF handling on every request and response.
- Case-insensitive command names like `PING`, `ping`, and `PiNg`.
- Correct null responses:
  - missing strings as `$-1\r\n`
  - empty arrays as `*0\r\n`
- Correct error replies with the `-ERR ...\r\n` format.
- Sequential request handling over a single TCP connection.
- Multiple commands on the same connection.

### 8.2 Must-have for common client libraries

- Stable TCP server startup and clean connection acceptance.
- No unsolicited data before the client sends a command.
- Proper read/write buffering so pipelined commands are handled in order.
- Reasonable timeout and blocking behavior for commands like `BLPOP` later on.
- Pub/Sub style connection state later on, where commands change how the connection behaves.
- Deterministic response formatting so library parsers do not fail on minor wire differences.
- Graceful handling of empty payloads, missing keys, and expired keys.

### 8.3 Advanced compatibility items

- RESP3 awareness if you later want broader client support.
- `AUTH`, `HELLO`, `SELECT`, and other connection/session commands when you extend beyond the base stages.
- Transaction and watch semantics that match Redis expectations closely.
- Pub/Sub semantics for subscribed connections.
- Replication-related behavior, including sync and ACK flows.
- Persistence restart behavior that restores data consistently after a process restart.
- Concurrency safety under `go test -race` and multi-client stress tests.

### Practical rule

If `redis-cli` works but a client library does not, the problem is usually one of these:
- wrong RESP shape
- wrong error/null formatting
- unexpected data before the first command
- connection state not matching the command that was sent
- missing feature support rather than bad serialization
