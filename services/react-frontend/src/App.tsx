import { useEffect, useMemo, useState, type ChangeEvent } from "react";
import { useSSESnapshots } from "./useSSESnapshots";
import type { BackendStats, Snapshot } from "./types";

const topSymbols = [
  "ALL",
  "BTCUSDT",
  "ETHUSDT",
  "ETHUSDC",
  "BTCUSDC",
  "SOLUSDT",
  "XAUUSDT",
  "XAGUSDT",
  "REDUSDT",
  "TRUUSDT",
  "XRPUSDT",
  "CLUSDT",
  "ZECUSDT",
  "DOGEUSDT",
  "SIRENUSDT",
  "TAOUSDT",
  "1000PEPEUSDT",
  "BULLAUSDT",
  "PLAYUSDT",
  "BNBUSDT",
  "BZUSDT"
];

function spread(snapshot: Snapshot): number | null {
  if (snapshot.ap1 == null || snapshot.bp1 == null) {
    return null;
  }
  return snapshot.ap1 - snapshot.bp1;
}

function formatNumber(value: number | null | undefined, digits = 4): string {
  if (value == null || Number.isNaN(value)) {
    return "-";
  }
  return value.toLocaleString(undefined, { maximumFractionDigits: digits });
}

function buildPath(values: number[], width: number, height: number): string {
  if (values.length < 2) {
    return "";
  }

  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;

  return values
    .map((v, i) => {
      const x = (i / (values.length - 1)) * width;
      const y = height - ((v - min) / span) * height;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");
}

function TrendCard({
  title,
  values,
  stroke,
  latest,
  precision,
}: {
  title: string;
  values: number[];
  stroke: string;
  latest: number | null;
  precision: number;
}) {
  const width = 540;
  const height = 180;
  const path = buildPath(values, width, height);

  return (
    <article className="card trend-card">
      <div className="trend-header">
        <h3>{title}</h3>
        <p>{formatNumber(latest, precision)}</p>
      </div>
      <div className="trend-canvas">
        {values.length < 2 ? (
          <div className="trend-empty">Waiting for more live ticks...</div>
        ) : (
          <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label={title}>
            <defs>
              <linearGradient id={`fill-${title.replace(/\s+/g, "-").toLowerCase()}`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={stroke} stopOpacity="0.4" />
                <stop offset="100%" stopColor={stroke} stopOpacity="0" />
              </linearGradient>
            </defs>
            <polyline fill="none" stroke={stroke} strokeWidth="2.5" points={path} />
          </svg>
        )}
      </div>
    </article>
  );
}

export function App() {
  const [tokenInput, setTokenInput] = useState<string>(() => localStorage.getItem("xno.dashboard.token") ?? "");
  const [token, setToken] = useState<string>(() => localStorage.getItem("xno.dashboard.token") ?? "");
  const [backendStats, setBackendStats] = useState<BackendStats | null>(null);
  const { connection, selectedSymbol, setSelectedSymbol, rows } = useSSESnapshots(token);

  const stats = useMemo(() => {
    if (rows.length === 0) {
      return { count: 0, avgSpread: null as number | null, latest: null as Snapshot | null };
    }

    let sum = 0;
    let c = 0;
    for (const row of rows) {
      const s = spread(row);
      if (s != null) {
        sum += s;
        c += 1;
      }
    }

    return {
      count: rows.length,
      avgSpread: c > 0 ? sum / c : null,
      latest: rows[0] ?? null,
    };
  }, [rows]);

  const chartRows = useMemo(() => rows.slice(0, 90).reverse(), [rows]);
  const priceSeries = useMemo(() => chartRows.map((row) => row.price), [chartRows]);
  const spreadSeries = useMemo(
    () => chartRows.map((row) => spread(row)).filter((v): v is number => v != null),
    [chartRows]
  );
  const latestSpread = useMemo(() => {
    if (!stats.latest) {
      return null;
    }
    return spread(stats.latest);
  }, [stats.latest]);

  const onSymbolChange = (e: ChangeEvent<HTMLSelectElement>) => {
    setSelectedSymbol(e.target.value);
  };

  useEffect(() => {
    localStorage.setItem("xno.dashboard.token", token);
  }, [token]);

  useEffect(() => {
    const base = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080";
    const controller = new AbortController();

    async function fetchStats() {
      const params = new URLSearchParams();
      if (token.trim() !== "") {
        params.set("token", token.trim());
      }
      const query = params.toString();
      const url = query ? `${base}/stats?${query}` : `${base}/stats`;
      try {
        const res = await fetch(url, { signal: controller.signal });
        if (!res.ok) {
          setBackendStats(null);
          return;
        }
        const parsed = (await res.json()) as BackendStats;
        setBackendStats(parsed);
      } catch {
        setBackendStats(null);
      }
    }

    void fetchStats();
    const timer = window.setInterval(() => {
      void fetchStats();
    }, 2000);

    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [token]);

  const onTokenInput = (e: ChangeEvent<HTMLInputElement>) => {
    setTokenInput(e.target.value);
  };

  const onApplyToken = () => {
    setToken(tokenInput.trim());
  };

  return (
    <div className="page">
      <div className="bg-grid" />
      <header className="hero">
        <p className="eyebrow">XNO Firehose</p>
        <h1>Live Market Snapshots</h1>
        <p className="subtext">Real-time SSE stream from normalized trade + top-6 orderbook snapshots.</p>
      </header>

      <section className="controls card">
        <label>
          Symbol
          <select value={selectedSymbol} onChange={onSymbolChange}>
            {topSymbols.map((symbol) => (
              <option key={symbol} value={symbol}>
                {symbol}
              </option>
            ))}
          </select>
        </label>
        <label>
          Dashboard Token
          <div className="token-row">
            <input value={tokenInput} onChange={onTokenInput} placeholder="optional bearer token" />
            <button type="button" onClick={onApplyToken}>
              Apply
            </button>
          </div>
        </label>
        <div className={`status ${connection}`}>Connection: {connection}</div>
      </section>

      <section className="stats-grid">
        <article className="card stat">
          <h3>Snapshots Buffered</h3>
          <p>{stats.count}</p>
        </article>
        <article className="card stat">
          <h3>Average Spread</h3>
          <p>{formatNumber(stats.avgSpread, 6)}</p>
        </article>
        <article className="card stat">
          <h3>Latest Symbol</h3>
          <p>{stats.latest?.symbol ?? "-"}</p>
        </article>
        <article className="card stat">
          <h3>Latest Price</h3>
          <p>{formatNumber(stats.latest?.price, 2)}</p>
        </article>
        <article className="card stat">
          <h3>SSE Clients</h3>
          <p>{backendStats?.active_clients ?? "-"}</p>
        </article>
        <article className="card stat">
          <h3>Total Pushed</h3>
          <p>{backendStats?.total_pushed ?? "-"}</p>
        </article>
        <article className="card stat">
          <h3>Total Dropped</h3>
          <p>{backendStats?.total_dropped ?? "-"}</p>
        </article>
        <article className="card stat">
          <h3>Poll Errors</h3>
          <p>{backendStats?.poll_errors ?? "-"}</p>
        </article>
      </section>

      <section className="trend-grid">
        <TrendCard
          title="Price Trend"
          values={priceSeries}
          stroke="#f4c95d"
          latest={stats.latest?.price ?? null}
          precision={3}
        />
        <TrendCard
          title="Spread Trend"
          values={spreadSeries}
          stroke="#73d2de"
          latest={latestSpread}
          precision={6}
        />
      </section>

      <section className="card table-wrap">
        <table>
          <thead>
            <tr>
              <th>Time (UTC)</th>
              <th>Symbol</th>
              <th>Price</th>
              <th>Qty</th>
              <th>Amount</th>
              <th>Spread</th>
              <th>Trade ID</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={`${row.symbol}-${row.trade_id}-${row.event_time}`}>
                <td>{new Date(row.event_time).toISOString()}</td>
                <td>{row.symbol}</td>
                <td>{formatNumber(row.price, 3)}</td>
                <td>{formatNumber(row.qty, 4)}</td>
                <td>{formatNumber(row.amount, 2)}</td>
                <td>{formatNumber(spread(row), 6)}</td>
                <td>{row.trade_id}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}
