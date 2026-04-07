package main

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func nullableDepth(v float64, valid bool) sql.NullFloat64 {
	if !valid {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: v, Valid: true}
}

type DBWriter struct {
	db            *sql.DB
	batchSize     int
	flushInterval time.Duration
}

func NewDBWriter(db *sql.DB, batchSize int, flushInterval time.Duration) *DBWriter {
	return &DBWriter{db: db, batchSize: batchSize, flushInterval: flushInterval}
}

func (w *DBWriter) Run(ctx context.Context, in <-chan Snapshot) error {
	batch := make([]Snapshot, 0, w.batchSize)
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				_ = w.WriteBatch(context.Background(), batch)
			}
			return nil
		case s, ok := <-in:
			if !ok {
				if len(batch) > 0 {
					return w.WriteBatch(context.Background(), batch)
				}
				return nil
			}
			batch = append(batch, s)
			if len(batch) >= w.batchSize {
				if err := w.WriteBatch(ctx, batch); err != nil {
					return err
				}
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) == 0 {
				continue
			}
			if err := w.WriteBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
}

func (w *DBWriter) WriteBatch(ctx context.Context, snapshots []Snapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range snapshots {
		var tradeID int64
		err = tx.QueryRowContext(ctx,
			"INSERT INTO processed_trades(symbol, trade_id, first_event_time) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING RETURNING trade_id",
			s.Symbol, s.TradeID, s.EventTime,
		).Scan(&tradeID)

		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO market_snapshots (
				symbol, trade_id, event_time, price, qty, amount,
				bp1, bp2, bp3, bp4, bp5, bp6,
				bq1, bq2, bq3, bq4, bq5, bq6,
				ap1, ap2, ap3, ap4, ap5, ap6,
				aq1, aq2, aq3, aq4, aq5, aq6
			) VALUES (
				$1,$2,$3,$4,$5,$6,
				$7,$8,$9,$10,$11,$12,
				$13,$14,$15,$16,$17,$18,
				$19,$20,$21,$22,$23,$24,
				$25,$26,$27,$28,$29,$30
			)
		`,
			s.Symbol, s.TradeID, s.EventTime, s.Price, s.Qty, s.Amount,
			nullableDepth(s.BP[0], s.DepthValid), nullableDepth(s.BP[1], s.DepthValid), nullableDepth(s.BP[2], s.DepthValid), nullableDepth(s.BP[3], s.DepthValid), nullableDepth(s.BP[4], s.DepthValid), nullableDepth(s.BP[5], s.DepthValid),
			nullableDepth(s.BQ[0], s.DepthValid), nullableDepth(s.BQ[1], s.DepthValid), nullableDepth(s.BQ[2], s.DepthValid), nullableDepth(s.BQ[3], s.DepthValid), nullableDepth(s.BQ[4], s.DepthValid), nullableDepth(s.BQ[5], s.DepthValid),
			nullableDepth(s.AP[0], s.DepthValid), nullableDepth(s.AP[1], s.DepthValid), nullableDepth(s.AP[2], s.DepthValid), nullableDepth(s.AP[3], s.DepthValid), nullableDepth(s.AP[4], s.DepthValid), nullableDepth(s.AP[5], s.DepthValid),
			nullableDepth(s.AQ[0], s.DepthValid), nullableDepth(s.AQ[1], s.DepthValid), nullableDepth(s.AQ[2], s.DepthValid), nullableDepth(s.AQ[3], s.DepthValid), nullableDepth(s.AQ[4], s.DepthValid), nullableDepth(s.AQ[5], s.DepthValid),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
