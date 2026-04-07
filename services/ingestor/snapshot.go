package main

func BuildSnapshot(ob *OrderBook, trade TradeEvent) Snapshot {
	bp, bq, ap, aq := ob.GetTopSix()
	return Snapshot{
		Symbol:     trade.Symbol,
		TradeID:    trade.TradeID,
		EventTime:  trade.EventTime,
		Price:      trade.Price,
		Qty:        trade.Qty,
		Amount:     trade.Price * trade.Qty,
		DepthValid: true,
		BP:         bp,
		BQ:         bq,
		AP:         ap,
		AQ:         aq,
	}
}
