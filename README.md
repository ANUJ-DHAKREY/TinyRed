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
- **Optimistic Locking**: `WATCH` / `UNWATCH`, aborting a transaction if a watched key changed before `EXEC`
- **Pub/Sub**: `SUBSCRIBE`, `PUBLISH`, `UNSUBSCRIBE`, restricted command set while in subscriber mode
- **Sorted Sets**: `ZADD`, `ZCARD`, `ZRANGE`, `ZRANK`, `ZSCORE`, `ZREM`, backed by a skip list for ordered access
- **Geospatial**: `GEOADD`, `GEOPOS`, `GEODIST`, `GEOSEARCH` (`FROMLONLAT`/`BYRADIUS`), built on top of sorted sets by encoding longitude/latitude into a single interleaved (Morton code) score; distance uses the Haversine formula
- **AOF Persistence**: append-only file logging of write commands, manifest-based file tracking, replay on startup
- **RDB Persistence**: loading an existing `dump.rdb` file on startup

## Supported Commands

`PING` · `ECHO` · `SET` · `GET` · `CONFIG` · `KEYS` · `RPUSH` · `LPUSH` · `LPOP` · `LLEN` · `LRANGE` · `BLPOP` · `INCR` · `MULTI` · `EXEC` · `DISCARD` · `WATCH` · `UNWATCH` · `SUBSCRIBE` · `PUBLISH` · `UNSUBSCRIBE` · `ZADD` · `ZCARD` · `ZRANGE` · `ZRANK` · `ZSCORE` · `ZREM` · `GEOADD` · `GEOPOS` · `GEODIST` · `GEOSEARCH`

## Planned

- Streams (`XADD`, `XRANGE`, `XREAD`, ...)
- Replication (`REPLCONF`, `PSYNC`, `WAIT`)
- ACL / `AUTH`
- Bitmaps
- Custom extensions: hashes, sets, key management, active expiry, sharding, graceful shutdown, extra string ops, extra server commands

## Project Layout

- `resp/` — RESP protocol parsing and response encoding
- `store/` — in-memory key-value store
- `server/` — TCP server, command dispatch, connection handling
- `aof/` — append-only file persistence
- `rdb/` — RDB file loading
