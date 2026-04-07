package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	_ "github.com/lib/pq"
)

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

type cursor struct {
	EventTime time.Time
	Symbol    string
	TradeID   int64
}

type Hub struct {
	mu      sync.RWMutex
	clients map[uint64]chan Snapshot
	nextID  uint64
	pushed  uint64
	dropped uint64
}

type RuntimeStats struct {
	mu         sync.RWMutex
	lastCursor cursor
	lastPollAt time.Time
	lastRows   int
	pollErrors uint64
}

type StatsResponse struct {
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

func NewHub() *Hub {
	return &Hub{clients: make(map[uint64]chan Snapshot)}
}

func (h *Hub) Subscribe() (uint64, <-chan Snapshot, func()) {
	id := atomic.AddUint64(&h.nextID, 1)
	ch := make(chan Snapshot, 1024)

	h.mu.Lock()
	h.clients[id] = ch
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		if c, ok := h.clients[id]; ok {
			delete(h.clients, id)
			close(c)
		}
		h.mu.Unlock()
	}

	return id, ch, cancel
}

func (h *Hub) Publish(s Snapshot) {
	atomic.AddUint64(&h.pushed, 1)
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, ch := range h.clients {
		select {
		case ch <- s:
		default:
			atomic.AddUint64(&h.dropped, 1)
		}
	}
}

func (h *Hub) ActiveClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) TotalPushed() uint64 {
	return atomic.LoadUint64(&h.pushed)
}

func (h *Hub) TotalDropped() uint64 {
	return atomic.LoadUint64(&h.dropped)
}

func (r *RuntimeStats) setPoll(cur cursor, rowCount int) {
	r.mu.Lock()
	r.lastCursor = cur
	r.lastRows = rowCount
	r.lastPollAt = time.Now().UTC()
	r.mu.Unlock()
}

func (r *RuntimeStats) incPollError() {
	atomic.AddUint64(&r.pollErrors, 1)
}

func (r *RuntimeStats) pollErrorsCount() uint64 {
	return atomic.LoadUint64(&r.pollErrors)
}

func (r *RuntimeStats) snapshot() (cursor, time.Time, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastCursor, r.lastPollAt, r.lastRows
}

func pollSnapshots(ctx context.Context, db *sql.DB, hub *Hub, pollInterval time.Duration, stats *RuntimeStats) {
	cur := cursor{EventTime: time.Unix(0, 0).UTC()}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		rows, err := db.QueryContext(ctx, `
			SELECT
				symbol, trade_id, event_time, price, qty, amount,
				bp1, bp2, bp3, bp4, bp5, bp6,
				bq1, bq2, bq3, bq4, bq5, bq6,
				ap1, ap2, ap3, ap4, ap5, ap6,
				aq1, aq2, aq3, aq4, aq5, aq6
			FROM market_snapshots
			WHERE
				(event_time > $1)
				OR (event_time = $1 AND symbol > $2)
				OR (event_time = $1 AND symbol = $2 AND trade_id > $3)
			ORDER BY event_time ASC, symbol ASC, trade_id ASC
			LIMIT 3000
		`, cur.EventTime, cur.Symbol, cur.TradeID)
		if err != nil {
			log.Printf("poll query error: %v", err)
			stats.incPollError()
			continue
		}

		rowsCount := 0

		for rows.Next() {
			var s Snapshot
			err = rows.Scan(
				&s.Symbol, &s.TradeID, &s.EventTime, &s.Price, &s.Qty, &s.Amount,
				&s.BP1, &s.BP2, &s.BP3, &s.BP4, &s.BP5, &s.BP6,
				&s.BQ1, &s.BQ2, &s.BQ3, &s.BQ4, &s.BQ5, &s.BQ6,
				&s.AP1, &s.AP2, &s.AP3, &s.AP4, &s.AP5, &s.AP6,
				&s.AQ1, &s.AQ2, &s.AQ3, &s.AQ4, &s.AQ5, &s.AQ6,
			)
			if err != nil {
				log.Printf("poll scan error: %v", err)
				continue
			}

			cur = cursor{EventTime: s.EventTime, Symbol: s.Symbol, TradeID: s.TradeID}
			hub.Publish(s)
			rowsCount++
		}
		if err := rows.Err(); err != nil {
			log.Printf("poll rows error: %v", err)
			stats.incPollError()
		}
		_ = rows.Close()
		stats.setPoll(cur, rowsCount)
	}
}

func buildStats(hub *Hub, stats *RuntimeStats) StatsResponse {
	cur, lastPollAt, lastRows := stats.snapshot()
	resp := StatsResponse{
		ActiveClients: hub.ActiveClients(),
		TotalPushed:   hub.TotalPushed(),
		TotalDropped:  hub.TotalDropped(),
		LastPollRows:  lastRows,
		PollErrors:    stats.pollErrorsCount(),
	}
	if !lastPollAt.IsZero() {
		resp.LastPollAt = lastPollAt.Format(time.RFC3339)
	}
	resp.LastCursor.EventTime = cur.EventTime.Format(time.RFC3339Nano)
	resp.LastCursor.Symbol = cur.Symbol
	resp.LastCursor.TradeID = cur.TradeID
	return resp
}

func parseLimit(s string, fallback int, max int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || v <= 0 {
		return fallback
	}
	if v > max {
		return max
	}
	return v
}

func loadLatestSnapshots(ctx context.Context, db *sql.DB, symbol string, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 200
	}

	baseSelect := `
		SELECT
			symbol, trade_id, event_time, price, qty, amount,
			bp1, bp2, bp3, bp4, bp5, bp6,
			bq1, bq2, bq3, bq4, bq5, bq6,
			ap1, ap2, ap3, ap4, ap5, ap6,
			aq1, aq2, aq3, aq4, aq5, aq6
		FROM market_snapshots
	`

	var rows *sql.Rows
	var err error

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		rows, err = db.QueryContext(ctx, baseSelect+` ORDER BY event_time DESC, symbol DESC, trade_id DESC LIMIT $1`, limit)
	} else {
		rows, err = db.QueryContext(ctx, baseSelect+` WHERE symbol = $1 ORDER BY event_time DESC, trade_id DESC LIMIT $2`, symbol, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Snapshot, 0, limit)
	for rows.Next() {
		var s Snapshot
		if scanErr := rows.Scan(
			&s.Symbol, &s.TradeID, &s.EventTime, &s.Price, &s.Qty, &s.Amount,
			&s.BP1, &s.BP2, &s.BP3, &s.BP4, &s.BP5, &s.BP6,
			&s.BQ1, &s.BQ2, &s.BQ3, &s.BQ4, &s.BQ5, &s.BQ6,
			&s.AP1, &s.AP2, &s.AP3, &s.AP4, &s.AP5, &s.AP6,
			&s.AQ1, &s.AQ2, &s.AQ3, &s.AQ4, &s.AQ5, &s.AQ6,
		); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, s)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func tokenFromRequest(c fiber.Ctx) string {
	auth := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(c.Query("token"))
}

func authMiddleware(requiredToken string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if requiredToken == "" {
			return c.Next()
		}
		if tokenFromRequest(c) != requiredToken {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Next()
	}
}

func mustEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func main() {
	dbURL := mustEnv("DATABASE_URL", "postgresql://xno:xno_pass@timescaledb:5432/xno_market?sslmode=disable")
	port := mustEnv("PORT", "8080")
	dashboardToken := mustEnv("DASHBOARD_TOKEN", "")
	pollMs, _ := strconv.Atoi(mustEnv("SSE_POLL_INTERVAL_MS", "200"))
	if pollMs < 50 {
		pollMs = 50
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	hub := NewHub()
	runtimeStats := &RuntimeStats{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pollSnapshots(ctx, db, hub, time.Duration(pollMs)*time.Millisecond, runtimeStats)

	app := fiber.New()
	app.Use(cors.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})

	secured := app.Group("/", authMiddleware(dashboardToken))

	secured.Get("/stats", func(c fiber.Ctx) error {
		return c.JSON(buildStats(hub, runtimeStats))
	})

	secured.Get("/latest", func(c fiber.Ctx) error {
		symbol := c.Query("symbol")
		limit := parseLimit(c.Query("limit"), 200, 2000)

		rows, err := loadLatestSnapshots(c.Context(), db, symbol, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(rows)
	})

	secured.Get("/sse", func(c fiber.Ctx) error {
		symbolFilter := c.Query("symbol")

		_, events, unsubscribe := hub.Subscribe()
		reqCtx := c.RequestCtx()
		doneCh := reqCtx.Done()

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		reqCtx.SetBodyStreamWriter(func(w *bufio.Writer) {
			defer unsubscribe()
			keepAlive := time.NewTicker(15 * time.Second)
			defer keepAlive.Stop()

			for {
				select {
				case <-doneCh:
					return
				case <-keepAlive.C:
					if _, err := w.WriteString(": keepalive\n\n"); err != nil {
						return
					}
					if err := w.Flush(); err != nil {
						return
					}
				case s, ok := <-events:
					if !ok {
						return
					}
					if symbolFilter != "" && symbolFilter != s.Symbol {
						continue
					}

					payload, err := json.Marshal(s)
					if err != nil {
						continue
					}
					if _, err := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload); err != nil {
						return
					}
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		})

		return nil
	})

	addr := ":" + port
	log.Printf("go-backend listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("fiber listen: %v", err)
	}
}
