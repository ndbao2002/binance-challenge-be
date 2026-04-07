import { useEffect, useMemo, useRef, useState } from "react";
import type { ConnectionState, Snapshot } from "./types";

const MAX_ROWS = 250;

function getSSEUrl(symbol: string, token: string): string {
  const base = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080";
  const params = new URLSearchParams();
  if (symbol !== "ALL") {
    params.set("symbol", symbol);
  }
  if (token.trim() !== "") {
    params.set("token", token.trim());
  }
  const query = params.toString();
  return query ? `${base}/sse?${query}` : `${base}/sse`;
}

export function useSSESnapshots(token: string) {
  const [selectedSymbol, setSelectedSymbol] = useState<string>("ALL");
  const [rows, setRows] = useState<Snapshot[]>([]);
  const [connection, setConnection] = useState<ConnectionState>("connecting");
  const sourceRef = useRef<EventSource | null>(null);

  const sseUrl = useMemo(() => getSSEUrl(selectedSymbol, token), [selectedSymbol, token]);

  useEffect(() => {
    setRows([]);
    setConnection("connecting");

    const source = new EventSource(sseUrl);
    sourceRef.current = source;

    source.onopen = () => {
      setConnection("open");
    };

    source.onerror = () => {
      setConnection(source.readyState === EventSource.CLOSED ? "closed" : "error");
    };

    source.addEventListener("snapshot", (event) => {
      const message = event as MessageEvent;
      try {
        const parsed = JSON.parse(message.data) as Snapshot;
        setRows((current) => [parsed, ...current].slice(0, MAX_ROWS));
      } catch {
        // Ignore malformed events and continue stream processing.
      }
    });

    return () => {
      source.close();
      sourceRef.current = null;
    };
  }, [sseUrl]);

  return {
    selectedSymbol,
    setSelectedSymbol,
    rows,
    connection,
  };
}
