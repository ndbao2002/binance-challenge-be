from __future__ import annotations

import datetime as _dt
import os
from typing import TypeAlias

import numpy as np
import pandas as pd
from sqlalchemy import create_engine, text

DatetimeType: TypeAlias = _dt.datetime | pd.Timestamp | np.datetime64 | str


SNAPSHOT_COLUMNS = [
    "symbol",
    "trade_id",
    "timestamp",
    "price",
    "qty",
    "amount",
    "bp1",
    "bp2",
    "bp3",
    "bp4",
    "bp5",
    "bp6",
    "bq1",
    "bq2",
    "bq3",
    "bq4",
    "bq5",
    "bq6",
    "ap1",
    "ap2",
    "ap3",
    "ap4",
    "ap5",
    "ap6",
    "aq1",
    "aq2",
    "aq3",
    "aq4",
    "aq5",
    "aq6",
]


def _normalize_time(value: DatetimeType) -> pd.Timestamp:
    ts = pd.to_datetime(value, utc=True)
    if pd.isna(ts):
        raise ValueError(f"invalid datetime value: {value!r}")
    return pd.Timestamp(ts)


def _database_url(explicit: str | None) -> str:
    url = explicit or os.getenv("DATABASE_URL")
    if not url:
        raise ValueError("DATABASE_URL is required (pass database_url or set env)")
    return url


def load_data(
    symbol: str,
    start_time: DatetimeType,
    end_time: DatetimeType,
    database_url: str | None = None,
) -> pd.DataFrame:
    """Load historical normalized snapshots for one symbol in a time range.

    Args:
        symbol: Futures symbol, e.g. "BTCUSDT".
        start_time: Inclusive range start.
        end_time: Inclusive range end.
        database_url: Optional Postgres SQLAlchemy URL.

    Returns:
        pandas.DataFrame with challenge fields and timestamp as datetime64[ns, UTC].
    """
    symbol_norm = symbol.strip().upper()
    if not symbol_norm:
        raise ValueError("symbol must be non-empty")

    start_ts = _normalize_time(start_time)
    end_ts = _normalize_time(end_time)
    if start_ts > end_ts:
        raise ValueError("start_time must be <= end_time")

    query = text(
        """
        SELECT
            symbol,
            trade_id,
            event_time AS timestamp,
            price,
            qty,
            amount,
            bp1, bp2, bp3, bp4, bp5, bp6,
            bq1, bq2, bq3, bq4, bq5, bq6,
            ap1, ap2, ap3, ap4, ap5, ap6,
            aq1, aq2, aq3, aq4, aq5, aq6
        FROM market_snapshots
        WHERE symbol = :symbol
          AND event_time >= :start_time
          AND event_time <= :end_time
        ORDER BY event_time ASC, trade_id ASC
        """
    )

    engine = create_engine(_database_url(database_url), future=True)
    with engine.connect() as conn:
        df = pd.read_sql_query(
            query,
            conn,
            params={
                "symbol": symbol_norm,
                "start_time": start_ts.to_pydatetime(),
                "end_time": end_ts.to_pydatetime(),
            },
            parse_dates=["timestamp"],
        )

    if df.empty:
        return pd.DataFrame(columns=SNAPSHOT_COLUMNS)

    # Keep numeric columns in float64 for downstream quant workflows.
    numeric_cols = [c for c in SNAPSHOT_COLUMNS if c not in {"symbol", "trade_id", "timestamp"}]
    for col in numeric_cols:
        df[col] = pd.to_numeric(df[col], errors="coerce")

    df["trade_id"] = pd.to_numeric(df["trade_id"], errors="coerce").astype("Int64")
    df["timestamp"] = pd.to_datetime(df["timestamp"], utc=True)
    return df[SNAPSHOT_COLUMNS]


def load_latest(
    symbol: str,
    limit: int = 200,
    database_url: str | None = None,
) -> pd.DataFrame:
    """Load most recent snapshots for one symbol (newest first then sorted ascending)."""
    symbol_norm = symbol.strip().upper()
    if not symbol_norm:
        raise ValueError("symbol must be non-empty")
    if limit <= 0:
        raise ValueError("limit must be > 0")

    query = text(
        """
        SELECT
            symbol,
            trade_id,
            event_time AS timestamp,
            price,
            qty,
            amount,
            bp1, bp2, bp3, bp4, bp5, bp6,
            bq1, bq2, bq3, bq4, bq5, bq6,
            ap1, ap2, ap3, ap4, ap5, ap6,
            aq1, aq2, aq3, aq4, aq5, aq6
        FROM market_snapshots
        WHERE symbol = :symbol
        ORDER BY event_time DESC, trade_id DESC
        LIMIT :limit
        """
    )

    engine = create_engine(_database_url(database_url), future=True)
    with engine.connect() as conn:
        df = pd.read_sql_query(
            query,
            conn,
            params={"symbol": symbol_norm, "limit": int(limit)},
            parse_dates=["timestamp"],
        )

    if df.empty:
        return pd.DataFrame(columns=SNAPSHOT_COLUMNS)

    df = df.sort_values(["timestamp", "trade_id"], ascending=[True, True], kind="mergesort")
    df["timestamp"] = pd.to_datetime(df["timestamp"], utc=True)
    return df[SNAPSHOT_COLUMNS]
