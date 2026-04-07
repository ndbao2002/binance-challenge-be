export type ConnectionState = "connecting" | "open" | "closed" | "error";

export type Snapshot = {
  symbol: string;
  trade_id: number;
  event_time: string;
  price: number;
  qty: number;
  amount: number;
  bp1?: number;
  ap1?: number;
};

export type BackendStats = {
  active_clients: number;
  total_pushed: number;
  total_dropped: number;
  last_poll_rows: number;
  last_poll_at: string;
  last_cursor: {
    event_time: string;
    symbol: string;
    trade_id: number;
  };
  poll_errors: number;
};
