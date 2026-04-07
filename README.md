# XNO Order Flow Firehose

Real-time market data pipeline for Binance USD-M futures.

## 1. Solution Summary and Architecture

This solution ingests the top 20 futures pairs from Binance, keeps a synchronized local L2 order book per symbol, and creates normalized snapshots when each trade arrives.

Each snapshot contains:

- trade fields: timestamp, price, qty, amount
- book fields: top-6 bid/ask prices and quantities (bp1..bp6, bq1..bq6, ap1..ap6, aq1..aq6)

High-level flow:

1. `services/ingestor` consumes `@depth@100ms` and `@aggTrade` streams.
2. Per-symbol workers enforce Binance sequence rules (`U/u/pu`) and keep local book state.
3. On trade, worker merges trade + current top-6 depth and sends snapshot to writer.
4. Writer persists idempotently via `processed_trades` gate, then `market_snapshots`.
5. `services/go-backend` (Fiber v3) reads new rows and broadcasts via `GET /sse`.
6. `services/react-frontend` (Vite + React + TypeScript) renders live dashboard, auth, and stats.
7. SDKs expose data access:
     - Python package `binance_sdk` with `load_data()`
     - Go SDK for latest data and SSE streaming

## 2. Run Locally

### Prerequisites

- Docker + Docker Compose
- (Optional, local SDK testing) Go 1.25.x, Python 3.10+, uv

### Environment file

Create `.env` from the provided example before starting services:

```bash
cp .env.example .env
```

### Start full stack

```bash
docker compose up --build -d
```

### Stop

```bash
docker compose down -v
```

### Verify services

```bash
curl -s http://localhost:8080/health
curl -s "http://localhost:8080/latest?symbol=BTCUSDT&limit=5"
curl -N "http://localhost:8080/sse?symbol=BTCUSDT"
```

Frontend URL: `http://localhost:5173`

### Backfill behavior on startup

The ingestor supports 15-minute startup backfill via env vars:

- `START_WITH_BACKFILL=true`
- `BACKFILL_MINUTES=15`
- `BACKFILL_TYPE=approx|null`
- `KEEP_UNREADY_TRADE=true|false`
- `STARTUP_PRE_READY_TRADE_MODE=approx|null`

`START_WITH_BACKFILL` and `KEEP_UNREADY_TRADE` are similar in intent (both try to avoid an empty startup window), but they apply to different data windows:

- `START_WITH_BACKFILL` fetches historical trades (last 15 minutes) from REST at startup.
- `KEEP_UNREADY_TRADE` keeps only live trades that arrive before a symbol becomes ready.

While backfill logic is implemented, it is not enabled by default and is generally not recommended when depth fidelity matters. The reason is that backfill cannot preserve the original historical depth state. When enabled, the ingestor fetches historical trades for the last 15 minutes and emits snapshots using the configured `BACKFILL_TYPE`:
- `approx`: emits snapshots using a current/top-of-book approximation, not the true historical book state.
- `null`: emits snapshots with null depth columns, so historical depth state is unavailable.
The same limitation applies to startup live trades that arrive before a symbol is marked ready.

## 3. Language/Framework Choices and Why

- Ingestion engine: Go
    - Chosen for concurrency model (goroutines/channels), low latency, and stable websocket processing.
- Backend API: Go + Fiber v3
    - Required by challenge and appropriate for fast SSE endpoint delivery.
- Frontend UI: Vite + React + TypeScript
    - Required by challenge and suitable for responsive real-time dashboards.
- Database: PostgreSQL + TimescaleDB
    - Time-series friendly storage with hypertables and operational simplicity for high write volume.
- SDK interface: Python package (`binance_sdk`) + Go SDK access path
    - Matches requirement for native Python `load_data()` while supporting Go-side latest/stream usage.

## 4. Third-Party Libraries and Trade-offs

- TimescaleDB vs pure PostgreSQL (design trade-off)
    - Why TimescaleDB: automatic time-series chunking/compression/retention and simpler ops for sustained high-write snapshots.
    - Cost accepted: idempotency uses `processed_trades` + `market_snapshots`, so writes are effectively doubled (one dedup gate insert plus one snapshot insert).
    - Storage trade-off: extra dedup table overhead on recent data is accepted because compressed historical chunks reduce long-term footprint significantly.
    - Alternative not chosen: pure PostgreSQL + manual partitions would reduce write/storage overhead but increases partition lifecycle and maintenance complexity.

- `github.com/gorilla/websocket` (ingestor)
    - Pros: mature websocket client, reliable behavior.
    - Trade-off: lower-level API, more manual reconnect/control logic.
- `github.com/lib/pq` (ingestor/backend)
    - Pros: stable PostgreSQL driver.
    - Trade-off: does not include richer helpers like pgx pool ergonomics.
- `github.com/gofiber/fiber/v3` (backend)
    - Pros: high performance, clean SSE implementation.
    - Trade-off: v3 ecosystem still newer than v2 in some references/examples.
- `github.com/joho/godotenv` (ingestor)
    - Pros: easy local env loading.
    - Trade-off: extra dependency; container envs can work without it.
- Python: `pandas`, `numpy`, `sqlalchemy`, `psycopg2-binary` (SDK)
    - Pros: familiar data analysis workflow and straightforward DB access.
    - Trade-off: higher memory overhead than lighter alternatives for very large pulls.
- Frontend: React + Vite + TypeScript
    - Pros: fast dev/build loop and type-safe UI code.
    - Trade-off: requires TS build tooling for even simple UI changes.

## 5. Evidence and Related Docs

- Evidence files are in `evidence/`:
    - `logs.txt`
    - `analysis.ipynb`
    - dashboard screenshots (`UI_ALL.png`, `UI_BTCUSDT.png`, `DB.png`)
- Ingestor runtime notes: `services/ingestor/INGESTOR.MD`
- Go SDK notes: `sdk/go/binance_sdk/README.md`
