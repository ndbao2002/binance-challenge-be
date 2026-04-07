package main

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type BackfillManager struct {
	logger  *logrus.Logger
	baseURL string
	client  *http.Client
	writer  *DBWriter
	syncMgr *SyncManager
	limiter *RESTLimiter
	retry   RESTRetryConfig
}

type aggTradeREST struct {
	AggTradeID int64  `json:"a"`
	Price      string `json:"p"`
	Qty        string `json:"q"`
	TradeTime  int64  `json:"T"`
}

func NewBackfillManager(baseURL string, writer *DBWriter, limiter *RESTLimiter, retryCfg RESTRetryConfig, logger *logrus.Logger) *BackfillManager {
	return &BackfillManager{
		logger:  logger,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
		writer:  writer,
		syncMgr: NewSyncManager(baseURL, limiter, retryCfg, logger),
		limiter: limiter,
		retry:   retryCfg,
	}
}

func (b *BackfillManager) Run(ctx context.Context, symbols []string, minutes int, mode BackfillType) error {
	if minutes <= 0 {
		return nil
	}

	end := time.Now().UTC()
	start := end.Add(-time.Duration(minutes) * time.Minute)
	b.logger.WithFields(logrus.Fields{"symbols": len(symbols), "minutes": minutes, "mode": mode}).Info("backfill started")

	for _, symbol := range symbols {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		trades, err := b.fetchAggTradesWindow(ctx, symbol, start, end)
		if err != nil {
			b.logger.WithError(err).WithField("symbol", symbol).Warn("backfill trade fetch failed")
			continue
		}
		if len(trades) == 0 {
			continue
		}

		var bp, bq, ap, aq [6]float64
		depthValid := mode == BackfillTypeApprox
		if mode == BackfillTypeApprox {
			snapshot, err := b.syncMgr.fetchDepthSnapshot(ctx, symbol, 1000)
			if err != nil {
				b.logger.WithError(err).WithField("symbol", symbol).Warn("backfill depth snapshot failed; falling back to null mode for symbol")
				depthValid = false
			} else {
				bids, asks, err := parseDepthSnapshot(snapshot)
				if err != nil {
					b.logger.WithError(err).WithField("symbol", symbol).Warn("backfill depth parse failed; falling back to null mode for symbol")
					depthValid = false
				} else {
					for i := 0; i < len(bids) && i < 6; i++ {
						bp[i] = bids[i].Price
						bq[i] = bids[i].Qty
					}
					for i := 0; i < len(asks) && i < 6; i++ {
						ap[i] = asks[i].Price
						aq[i] = asks[i].Qty
					}
				}
			}
		}

		snapshots := make([]Snapshot, 0, len(trades))
		for _, tr := range trades {
			price, err1 := strconv.ParseFloat(tr.Price, 64)
			qty, err2 := strconv.ParseFloat(tr.Qty, 64)
			if err1 != nil || err2 != nil {
				continue
			}
			snapshots = append(snapshots, Snapshot{
				Symbol:     strings.ToUpper(symbol),
				TradeID:    tr.AggTradeID,
				EventTime:  time.UnixMilli(tr.TradeTime),
				Price:      price,
				Qty:        qty,
				Amount:     price * qty,
				DepthValid: depthValid,
				BP:         bp,
				BQ:         bq,
				AP:         ap,
				AQ:         aq,
			})
		}

		for i := 0; i < len(snapshots); i += b.writer.batchSize {
			j := i + b.writer.batchSize
			if j > len(snapshots) {
				j = len(snapshots)
			}
			if err := b.writer.WriteBatch(ctx, snapshots[i:j]); err != nil {
				b.logger.WithError(err).WithField("symbol", symbol).Warn("backfill write batch failed")
				break
			}
		}

		b.logger.WithFields(logrus.Fields{"symbol": symbol, "rows": len(snapshots), "mode": mode}).Info("backfill completed for symbol")
	}

	b.logger.Info("backfill finished")
	return nil
}

func (b *BackfillManager) fetchAggTradesWindow(ctx context.Context, symbol string, start, end time.Time) ([]aggTradeREST, error) {
	rows := make([]aggTradeREST, 0, 4096)
	cursor := start.UnixMilli()
	endMs := end.UnixMilli()
	const maxWindowMs = int64(time.Hour/time.Millisecond) - 1

	for cursor <= endMs {
		requestEnd := endMs
		if cursor+maxWindowMs < requestEnd {
			requestEnd = cursor + maxWindowMs
		}

		var chunk []aggTradeREST
		err := doBinanceJSON(
			ctx,
			b.client,
			b.limiter,
			b.retry,
			20,
			"agg_trades_backfill",
			b.logger,
			func() (*http.Request, error) {
				u, err := url.Parse(b.baseURL + "/fapi/v1/aggTrades")
				if err != nil {
					return nil, err
				}
				q := u.Query()
				q.Set("symbol", strings.ToUpper(symbol))
				q.Set("startTime", strconv.FormatInt(cursor, 10))
				q.Set("endTime", strconv.FormatInt(requestEnd, 10))
				q.Set("limit", "1000")
				u.RawQuery = q.Encode()
				return http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
			},
			&chunk,
		)
		if err != nil {
			return nil, err
		}

		if len(chunk) == 0 {
			break
		}

		rows = append(rows, chunk...)
		last := chunk[len(chunk)-1]
		next := last.TradeTime + 1
		if next <= cursor {
			break
		}
		if next > requestEnd {
			next = requestEnd + 1
		}
		cursor = next
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TradeTime == rows[j].TradeTime {
			return rows[i].AggTradeID < rows[j].AggTradeID
		}
		return rows[i].TradeTime < rows[j].TradeTime
	})

	return rows, nil
}
