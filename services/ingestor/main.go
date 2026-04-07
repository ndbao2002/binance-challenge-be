package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type depthSeqState struct {
	SnapshotID int64
	LastU      int64
	Synced     bool
}

type symbolWorker struct {
	symbol      string
	cfg         Config
	logger      *logrus.Logger
	syncMgr     *SyncManager
	orderBook   *OrderBook
	depthIn     chan DepthUpdate
	tradeIn     chan TradeEvent
	snapshotOut chan<- Snapshot

	seq           depthSeqState
	ready         bool
	pendingDepth  []DepthUpdate
	pendingTrades []TradeEvent
}

func applyDepthWithBinanceRules(ob *OrderBook, st *depthSeqState, d DepthUpdate) (needResync bool) {
	if !st.Synced {
		if d.UpdateID < st.SnapshotID {
			return false
		}
		if d.FirstID <= st.SnapshotID && st.SnapshotID <= d.UpdateID {
			ob.ApplyDelta(d)
			st.Synced = true
			st.LastU = d.UpdateID
		}
		if d.FirstID > st.SnapshotID {
			return true
		}
		return false
	}

	if d.UpdateID <= st.LastU {
		return false
	}

	if d.PrevID != st.LastU {
		return true
	}

	ob.ApplyDelta(d)
	st.LastU = d.UpdateID
	return false
}

func buildStartupTradeSnapshot(ob *OrderBook, trade TradeEvent, mode StartupTradeMode) Snapshot {
	snap := BuildSnapshot(ob, trade)
	if mode == StartupTradeModeNull {
		snap.DepthValid = false
	}
	return snap
}

func newSymbolWorker(symbol string, cfg Config, syncMgr *SyncManager, logger *logrus.Logger, snapshotOut chan<- Snapshot) *symbolWorker {
	return &symbolWorker{
		symbol:      symbol,
		cfg:         cfg,
		logger:      logger,
		syncMgr:     syncMgr,
		orderBook:   NewOrderBook(),
		depthIn:     make(chan DepthUpdate, 2048),
		tradeIn:     make(chan TradeEvent, 2048),
		snapshotOut: snapshotOut,
	}
}

func (w *symbolWorker) emitSnapshot(ctx context.Context, snap Snapshot) bool {
	select {
	case w.snapshotOut <- snap:
		return true
	case <-ctx.Done():
		return false
	}
}

func (w *symbolWorker) seed(ctx context.Context) {
	if err := w.syncMgr.SeedOrderBook(ctx, w.symbol, w.orderBook); err != nil {
		w.logger.WithError(err).WithField("symbol", w.symbol).Warn("seed order book failed")
	}
	w.seq = depthSeqState{SnapshotID: w.orderBook.LastUpdateID()}
}

func (w *symbolWorker) replayPendingDepth(ctx context.Context) {
	if len(w.pendingDepth) == 0 {
		return
	}

	sort.Slice(w.pendingDepth, func(i, j int) bool {
		return w.pendingDepth[i].UpdateID < w.pendingDepth[j].UpdateID
	})

	for _, d := range w.pendingDepth {
		if applyDepthWithBinanceRules(w.orderBook, &w.seq, d) {
			if err := w.syncMgr.SeedOrderBook(ctx, w.symbol, w.orderBook); err != nil {
				w.logger.WithError(err).WithField("symbol", w.symbol).Warn("sequence break resync failed")
				continue
			}
			w.seq = depthSeqState{SnapshotID: w.orderBook.LastUpdateID()}
		}
	}

	w.pendingDepth = w.pendingDepth[:0]
}

func (w *symbolWorker) flushPendingTrades(ctx context.Context) bool {
	if len(w.pendingTrades) == 0 {
		return true
	}

	if !w.cfg.KeepUnreadyTrade {
		w.pendingTrades = w.pendingTrades[:0]
		return true
	}

	for _, t := range w.pendingTrades {
		snap := buildStartupTradeSnapshot(w.orderBook, t, w.cfg.StartupTradeMode)
		if !w.emitSnapshot(ctx, snap) {
			return false
		}
	}

	w.pendingTrades = w.pendingTrades[:0]
	return true
}

func (w *symbolWorker) tryBecomeReady(ctx context.Context) bool {
	w.replayPendingDepth(ctx)
	if !w.seq.Synced {
		return true
	}
	if !w.flushPendingTrades(ctx) {
		return false
	}
	w.ready = true
	w.logger.WithField("symbol", w.symbol).Info("symbol is ready")
	return true
}

func (w *symbolWorker) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	w.seed(ctx)

	resyncTicker := time.NewTicker(30 * time.Second)
	defer resyncTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case d := <-w.depthIn:
			if !w.ready {
				w.pendingDepth = append(w.pendingDepth, d)
				if !w.tryBecomeReady(ctx) {
					return
				}
				continue
			}

			if applyDepthWithBinanceRules(w.orderBook, &w.seq, d) {
				if err := w.syncMgr.SeedOrderBook(ctx, w.symbol, w.orderBook); err != nil {
					w.logger.WithError(err).WithField("symbol", w.symbol).Warn("sequence break resync failed")
					continue
				}
				w.seq = depthSeqState{SnapshotID: w.orderBook.LastUpdateID()}
				w.ready = false
			}
		case t := <-w.tradeIn:
			if !w.ready || !w.seq.Synced {
				if w.cfg.KeepUnreadyTrade {
					w.pendingTrades = append(w.pendingTrades, t)
				}
				continue
			}

			snap := BuildSnapshot(w.orderBook, t)
			if !w.emitSnapshot(ctx, snap) {
				return
			}
		case <-resyncTicker.C:
			if !w.ready || w.orderBook.LevelCount() >= 100 {
				continue
			}
			if err := w.syncMgr.SeedOrderBook(ctx, w.symbol, w.orderBook); err != nil {
				w.logger.WithError(err).WithField("symbol", w.symbol).Warn("resync failed")
				continue
			}
			w.seq = depthSeqState{SnapshotID: w.orderBook.LastUpdateID()}
			w.ready = false
		}
	}
}

func dispatchDepth(ctx context.Context, in <-chan DepthUpdate, workers map[string]*symbolWorker) {
	for {
		select {
		case <-ctx.Done():
			return
		case d := <-in:
			w, ok := workers[d.Symbol]
			if !ok {
				continue
			}
			select {
			case w.depthIn <- d:
			case <-ctx.Done():
				return
			}
		}
	}
}

func dispatchTrade(ctx context.Context, in <-chan TradeEvent, workers map[string]*symbolWorker) {
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-in:
			w, ok := workers[t.Symbol]
			if !ok {
				continue
			}
			select {
			case w.tradeIn <- t:
			case <-ctx.Done():
				return
			}
		}
	}
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		logrus.WithError(err).Fatal("invalid configuration")
	}
	logger := logrus.New()
	lvl, err := logrus.ParseLevel(cfg.LogLevel)
	if err == nil {
		logger.SetLevel(lvl)
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.WithError(err).Fatal("open database")
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.WithError(err).Fatal("ping database")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	restLimiter := NewRESTLimiter(cfg.RESTWeightPerMin, time.Minute)
	restRetry := RESTRetryConfig{
		MaxRetries:  cfg.RESTMaxRetries,
		BaseBackoff: cfg.RESTBaseBackoff,
		MaxBackoff:  cfg.RESTMaxBackoff,
		BanCooldown: cfg.RESTBanCooldown,
	}

	tradeCh := make(chan TradeEvent, 2000)
	depthCh := make(chan DepthUpdate, 4000)
	snapshotCh := make(chan Snapshot, 4000)

	orderBooks := make(map[string]*OrderBook, len(cfg.Symbols))
	for _, symbol := range cfg.Symbols {
		orderBooks[symbol] = NewOrderBook()
	}

	writer := NewDBWriter(db, cfg.BatchSize, cfg.FlushInterval)
	go func() {
		if err := writer.Run(ctx, snapshotCh); err != nil {
			logger.WithError(err).Error("writer stopped")
			cancel()
		}
	}()

	if cfg.StartWithBackfill {
		backfill := NewBackfillManager(cfg.BinanceRESTURL, writer, restLimiter, restRetry, logger)
		if err := backfill.Run(ctx, cfg.Symbols, cfg.BackfillMinutes, cfg.BackfillType); err != nil {
			logger.WithError(err).Fatal("backfill failed")
		}
	}

	depthWS := NewWebSocketManager(cfg.BinanceWSURL, cfg.Symbols, cfg.TradeStream, true, false, logger)
	tradeWS := NewWebSocketManager(cfg.BinanceWSURL, cfg.Symbols, cfg.TradeStream, false, true, logger)

	workers := make(map[string]*symbolWorker, len(cfg.Symbols))
	var workersWG sync.WaitGroup
	for _, symbol := range cfg.Symbols {
		syncMgr := NewSyncManager(cfg.BinanceRESTURL, restLimiter, restRetry, logger)
		w := newSymbolWorker(symbol, cfg, syncMgr, logger, snapshotCh)
		workers[symbol] = w
		workersWG.Add(1)
		go w.run(ctx, &workersWG)
	}

	go depthWS.Run(ctx, tradeCh, depthCh)
	go tradeWS.Run(ctx, tradeCh, depthCh)
	go dispatchDepth(ctx, depthCh, workers)
	go dispatchTrade(ctx, tradeCh, workers)

	logger.WithFields(logrus.Fields{"symbols": len(cfg.Symbols), "trade_stream": cfg.TradeStream, "batch_size": cfg.BatchSize, "flush_interval": cfg.FlushInterval.String()}).Info("ingestor started")
	<-ctx.Done()
	workersWG.Wait()
	logger.Info("shutdown signal received")

	_ = os.Stdout.Sync()
}
