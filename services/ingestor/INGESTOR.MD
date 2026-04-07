# Ingestor Runtime Flow (Current)

## Status
Implementation is complete for the current challenge scope and validated in runtime tests.

## Goal
Ingest top futures symbols from Binance USD-M futures and persist trade-aligned market snapshots with top-6 book context in an idempotent way.

## Architecture

```
services/ingestor/
  main.go
  config.go
  types.go
  websocket.go
  orderbook.go
  sync.go
  snapshot.go
  writer.go
  backfill.go
  rest_limit.go
```

## End-to-End Data Flow
1. Open 2 websocket managers:
   - depth streams: <symbol>@depth@100ms
   - trade streams: <symbol>@aggTrade
2. Parse combined stream payloads and push events into shared depth/trade channels.
3. Dispatch events to a dedicated worker per symbol.
4. Each symbol worker maintains its own in-memory order book and depth sequence state.
5. On valid trade, build snapshot with top-6 bid/ask levels and send to writer.
6. Writer performs idempotent write via processed_trades gate, then inserts into market_snapshots.

## Symbol Worker Model
- One goroutine per symbol.
- Internal buffers:
  - pendingDepth: depth events received before symbol is ready
  - pendingTrades: trades received before symbol is ready (optional, config-driven)
- Readiness is per-symbol, not global.

## Binance Depth Sequencing Rules
Depth payload fields are mapped as:
- U -> FirstID
- u -> UpdateID
- pu -> PrevID

Applied rules:
1. After seed snapshot (lastUpdateId), ignore depth updates where u < lastUpdateId.
2. First accepted event must satisfy U <= lastUpdateId <= u.
3. After sync, require pu == previous_u.
4. If continuity breaks, reseed that symbol from REST and re-enter startup sync.
5. Depth quantities are absolute; qty == 0 removes that price level.

## Startup Behavior
Per symbol startup steps:
1. Seed order book from REST depth snapshot (/fapi/v1/depth?limit=1000).
2. Sort and replay buffered depth updates through Binance sequence checks.
3. If configured, flush buffered trades into snapshots using startup trade policy.
4. Mark symbol as ready only when depth is synced and startup queues are processed.

Startup pre-ready trade policy:
- KEEP_UNREADY_TRADE=false: drop pre-ready trades.
- KEEP_UNREADY_TRADE=true and STARTUP_PRE_READY_TRADE_MODE=approx:
  - emit with current book top-6 at flush time.
- KEEP_UNREADY_TRADE=true and STARTUP_PRE_READY_TRADE_MODE=null:
  - emit with null depth columns.

## Runtime Resync Safeguard
Each symbol worker runs a periodic health check (30s):
- If symbol is ready but total depth levels are too low (<100), force reseed.

## Persistence and Idempotency
- Writer batches by size and interval.
- For each snapshot in a DB transaction:
  1. Insert (symbol, trade_id) into processed_trades with ON CONFLICT DO NOTHING RETURNING trade_id.
  2. If no row returned, treat as duplicate and skip snapshot insert.
  3. Otherwise insert full row into market_snapshots.
- Depth columns are nullable and controlled by snapshot.DepthValid.

## Backfill
Backfill source:
- REST /fapi/v1/aggTrades

Backfill controls:
- START_WITH_BACKFILL (true|false)
- BACKFILL_TYPE (approx|null)
- BACKFILL_MINUTES (>0)

Backfill behavior:
- Requests are paginated and constrained to Binance one-hour max window per request.
- approx mode:
  - fetch one current depth snapshot per symbol and use it as static context for all backfill trades.
  - if depth fetch/parse fails, fallback to null depth for that symbol.
- null mode:
  - write trade snapshots with null depth columns.
- Backfill writes through the same DB writer/idempotency path as live flow.

Config consistency rule:
- If START_WITH_BACKFILL=true and BACKFILL_TYPE=approx and KEEP_UNREADY_TRADE=true,
  STARTUP_PRE_READY_TRADE_MODE must be approx.

## REST Rate Limit and Retry
- Shared REST limiter with configurable weight budget per minute.
- Depth request weight is derived from requested limit.
- Retry covers 429, 418, and 5xx.
- Retry delay is chosen from:
  1. Retry-After response header
  2. Binance retryAfter fields in error body
  3. ban cooldown (for 418)
  4. exponential backoff
- Used weight headers (X-MBX-USED-WEIGHT-*) are observed to align internal limiter state.

## Websocket Reliability
- Reconnect with exponential backoff on connection/read failures.
- Planned reconnect at 23.5h to avoid 24h stream hard cutoff issues.
- Trade stream matching is case-normalized to prevent silent drops due to stream name casing.

## Scope Notes
- Trade stream is intentionally aggTrade for this challenge scope.
- Symbols are normalized uppercase internally and lowercase for stream subscription names.
- Historical order book reconstruction is not exact from REST; approx depth in backfill/startup is best-effort context.
