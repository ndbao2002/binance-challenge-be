package main

import "time"

type BackfillType string
type StartupTradeMode string

const (
	BackfillTypeApprox BackfillType = "approx"
	BackfillTypeNull   BackfillType = "null"

	StartupTradeModeApprox StartupTradeMode = "approx"
	StartupTradeModeNull   StartupTradeMode = "null"
)

type DepthLevel struct {
	Price float64
	Qty   float64
}

type TradeEvent struct {
	Symbol    string
	TradeID   int64
	EventTime time.Time
	Price     float64
	Qty       float64
}

type DepthUpdate struct {
	Symbol    string
	FirstID   int64
	UpdateID  int64
	PrevID    int64
	EventTime time.Time
	Bids      []DepthLevel
	Asks      []DepthLevel
}

type Snapshot struct {
	Symbol     string
	TradeID    int64
	EventTime  time.Time
	Price      float64
	Qty        float64
	Amount     float64
	DepthValid bool

	BP [6]float64
	BQ [6]float64
	AP [6]float64
	AQ [6]float64
}

type combinedStreamMessage struct {
	Stream string `json:"stream"`
	Data   []byte `json:"data"`
}

type aggTradePayload struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	AggTrade  int64  `json:"a"`
	Price     string `json:"p"`
	Qty       string `json:"q"`
}

type depthPayload struct {
	EventType string     `json:"e"`
	EventTime int64      `json:"E"`
	Symbol    string     `json:"s"`
	FirstID   int64      `json:"U"`
	UpdateID  int64      `json:"u"`
	PrevID    int64      `json:"pu"`
	Bids      [][]string `json:"b"`
	Asks      [][]string `json:"a"`
}

type depthSnapshotResponse struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}
