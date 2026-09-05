# TinyRed

TinyRed is a multithreaded, from-scratch implementation of Redis in Go. It speaks the real RESP protocol, so any RESP-compatible client — including `redis-cli` — can connect to it and use it exactly like a real Redis server.

## Quick Start

```bash
go build -o tinyred .
./tinyred --port 6379
```

In another terminal:

```bash
redis-cli -p 6379
```

## Implemented

- **Base**: TCP server, concurrent clients, `PING`, `ECHO`, `SET`/`GET` with expiry, `CONFIG GET`, `KEYS`
- **Lists**: `RPUSH`, `LPUSH`, `LRANGE`, `LLEN`, `LPOP`, `BLPOP`
- **Transactions**: `MULTI`, `EXEC`, `DISCARD`, `INCR`, command queueing, per-client isolated transaction state
- **AOF Persistence**: append-only file logging of write commands, manifest-based file tracking, replay on startup
- **RDB Persistence**: loading an existing `dump.rdb` file on startup

## Supported Commands

`PING` · `ECHO` · `SET` · `GET` · `CONFIG` · `KEYS` · `RPUSH` · `LPUSH` · `LPOP` · `LLEN` · `LRANGE` · `BLPOP` · `INCR` · `MULTI` · `EXEC` · `DISCARD`

## Planned

- Pub/Sub (`SUBSCRIBE`, `PUBLISH`, `UNSUBSCRIBE`)
- Sorted Sets (`ZADD`, `ZRANGE`, `ZSCORE`, ...)
- Geospatial commands (`GEOADD`, `GEOSEARCH`, ...)
- Streams (`XADD`, `XRANGE`, `XREAD`, ...)
- Optimistic locking (`WATCH` / `UNWATCH`)
- Replication (`REPLCONF`, `PSYNC`, `WAIT`)
- ACL / `AUTH`
- Custom extensions: hashes, sets, key management, active expiry, sharding, graceful shutdown, extra string ops, extra server commands

## Project Layout

- `resp/` — RESP protocol parsing and response encoding
- `store/` — in-memory key-value store
- `server/` — TCP server, command dispatch, connection handling
- `aof/` — append-only file persistence
- `rdb/` — RDB file loading
