CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS public.processed_trades (
    symbol TEXT NOT NULL,
    trade_id BIGINT NOT NULL,
    first_event_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (symbol, trade_id)
);

CREATE TABLE IF NOT EXISTS public.market_snapshots (
    symbol TEXT NOT NULL,
    trade_id BIGINT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    qty DOUBLE PRECISION NOT NULL,
    amount DOUBLE PRECISION NOT NULL,

    bp1 DOUBLE PRECISION,
    bp2 DOUBLE PRECISION,
    bp3 DOUBLE PRECISION,
    bp4 DOUBLE PRECISION,
    bp5 DOUBLE PRECISION,
    bp6 DOUBLE PRECISION,

    bq1 DOUBLE PRECISION,
    bq2 DOUBLE PRECISION,
    bq3 DOUBLE PRECISION,
    bq4 DOUBLE PRECISION,
    bq5 DOUBLE PRECISION,
    bq6 DOUBLE PRECISION,

    ap1 DOUBLE PRECISION,
    ap2 DOUBLE PRECISION,
    ap3 DOUBLE PRECISION,
    ap4 DOUBLE PRECISION,
    ap5 DOUBLE PRECISION,
    ap6 DOUBLE PRECISION,

    aq1 DOUBLE PRECISION,
    aq2 DOUBLE PRECISION,
    aq3 DOUBLE PRECISION,
    aq4 DOUBLE PRECISION,
    aq5 DOUBLE PRECISION,
    aq6 DOUBLE PRECISION,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (symbol, trade_id, event_time)
);

SELECT create_hypertable('public.market_snapshots', 'event_time', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_market_snapshots_symbol_event_time
    ON public.market_snapshots (symbol, event_time DESC);
