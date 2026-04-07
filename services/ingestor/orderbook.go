package main

import (
	"sort"
	"sync"
)

type OrderBook struct {
	mu           sync.RWMutex
	bids         map[float64]float64
	asks         map[float64]float64
	lastUpdateID int64
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		bids: make(map[float64]float64),
		asks: make(map[float64]float64),
	}
}

func (ob *OrderBook) SetSnapshotLevels(lastUpdateID int64, bids, asks []DepthLevel) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.bids = make(map[float64]float64, len(bids))
	ob.asks = make(map[float64]float64, len(asks))
	for _, lv := range bids {
		if lv.Qty > 0 {
			ob.bids[lv.Price] = lv.Qty
		}
	}
	for _, lv := range asks {
		if lv.Qty > 0 {
			ob.asks[lv.Price] = lv.Qty
		}
	}
	ob.lastUpdateID = lastUpdateID
}

func (ob *OrderBook) ApplyDelta(update DepthUpdate) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if update.UpdateID <= ob.lastUpdateID {
		return
	}

	for _, lv := range update.Bids {
		if lv.Qty == 0 {
			delete(ob.bids, lv.Price)
			continue
		}
		ob.bids[lv.Price] = lv.Qty
	}
	for _, lv := range update.Asks {
		if lv.Qty == 0 {
			delete(ob.asks, lv.Price)
			continue
		}
		ob.asks[lv.Price] = lv.Qty
	}
	ob.lastUpdateID = update.UpdateID
}

func (ob *OrderBook) LastUpdateID() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.lastUpdateID
}

func (ob *OrderBook) LevelCount() int {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return len(ob.bids) + len(ob.asks)
}

func (ob *OrderBook) TopLevels(n int) (bids, asks []DepthLevel) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bidPrices := make([]float64, 0, len(ob.bids))
	for p := range ob.bids {
		bidPrices = append(bidPrices, p)
	}
	sort.Slice(bidPrices, func(i, j int) bool { return bidPrices[i] > bidPrices[j] })

	askPrices := make([]float64, 0, len(ob.asks))
	for p := range ob.asks {
		askPrices = append(askPrices, p)
	}
	sort.Slice(askPrices, func(i, j int) bool { return askPrices[i] < askPrices[j] })

	if len(bidPrices) > n {
		bidPrices = bidPrices[:n]
	}
	if len(askPrices) > n {
		askPrices = askPrices[:n]
	}

	bids = make([]DepthLevel, 0, len(bidPrices))
	for _, p := range bidPrices {
		bids = append(bids, DepthLevel{Price: p, Qty: ob.bids[p]})
	}
	asks = make([]DepthLevel, 0, len(askPrices))
	for _, p := range askPrices {
		asks = append(asks, DepthLevel{Price: p, Qty: ob.asks[p]})
	}
	return bids, asks
}

func (ob *OrderBook) GetTopSix() (bp, bq, ap, aq [6]float64) {
	bids, asks := ob.TopLevels(6)
	for i := 0; i < len(bids) && i < 6; i++ {
		bp[i] = bids[i].Price
		bq[i] = bids[i].Qty
	}
	for i := 0; i < len(asks) && i < 6; i++ {
		ap[i] = asks[i].Price
		aq[i] = asks[i].Qty
	}
	return bp, bq, ap, aq
}
