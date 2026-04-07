package binance_sdk

import "time"

type Snapshot struct {
	Symbol    string    `json:"symbol"`
	TradeID   int64     `json:"trade_id"`
	EventTime time.Time `json:"event_time"`
	Price     float64   `json:"price"`
	Qty       float64   `json:"qty"`
	Amount    float64   `json:"amount"`
	BP1       *float64  `json:"bp1,omitempty"`
	BP2       *float64  `json:"bp2,omitempty"`
	BP3       *float64  `json:"bp3,omitempty"`
	BP4       *float64  `json:"bp4,omitempty"`
	BP5       *float64  `json:"bp5,omitempty"`
	BP6       *float64  `json:"bp6,omitempty"`
	BQ1       *float64  `json:"bq1,omitempty"`
	BQ2       *float64  `json:"bq2,omitempty"`
	BQ3       *float64  `json:"bq3,omitempty"`
	BQ4       *float64  `json:"bq4,omitempty"`
	BQ5       *float64  `json:"bq5,omitempty"`
	BQ6       *float64  `json:"bq6,omitempty"`
	AP1       *float64  `json:"ap1,omitempty"`
	AP2       *float64  `json:"ap2,omitempty"`
	AP3       *float64  `json:"ap3,omitempty"`
	AP4       *float64  `json:"ap4,omitempty"`
	AP5       *float64  `json:"ap5,omitempty"`
	AP6       *float64  `json:"ap6,omitempty"`
	AQ1       *float64  `json:"aq1,omitempty"`
	AQ2       *float64  `json:"aq2,omitempty"`
	AQ3       *float64  `json:"aq3,omitempty"`
	AQ4       *float64  `json:"aq4,omitempty"`
	AQ5       *float64  `json:"aq5,omitempty"`
	AQ6       *float64  `json:"aq6,omitempty"`
}

type Stats struct {
	ActiveClients int    `json:"active_clients"`
	TotalPushed   uint64 `json:"total_pushed"`
	TotalDropped  uint64 `json:"total_dropped"`
	LastPollRows  int    `json:"last_poll_rows"`
	LastPollAt    string `json:"last_poll_at"`
	LastCursor    struct {
		EventTime string `json:"event_time"`
		Symbol    string `json:"symbol"`
		TradeID   int64  `json:"trade_id"`
	} `json:"last_cursor"`
	PollErrors uint64 `json:"poll_errors"`
}
