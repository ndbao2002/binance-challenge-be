package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type WebSocketManager struct {
	baseURL     string
	symbols     []string
	tradeStream string
	withDepth   bool
	withTrade   bool
	logger      *logrus.Logger
}

func NewWebSocketManager(baseURL string, symbols []string, tradeStream string, withDepth bool, withTrade bool, logger *logrus.Logger) *WebSocketManager {
	stream := strings.TrimSpace(tradeStream)
	if stream == "" {
		stream = "aggTrade"
	}
	return &WebSocketManager{baseURL: strings.TrimSuffix(baseURL, "/"), symbols: symbols, tradeStream: stream, withDepth: withDepth, withTrade: withTrade, logger: logger}
}

func (w *WebSocketManager) Run(ctx context.Context, tradeCh chan<- TradeEvent, depthCh chan<- DepthUpdate) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := w.connect()
		if err != nil {
			attempt++
			wait := backoff(attempt)
			w.logger.WithError(err).Warnf("ws connect failed, retry in %s", wait)
			if !sleepOrDone(ctx, wait) {
				return
			}
			continue
		}

		attempt = 0
		w.logger.Info("websocket connected")
		_ = conn.SetReadDeadline(time.Now().Add(12 * time.Minute))
		conn.SetPingHandler(func(appData string) error {
			_ = conn.SetReadDeadline(time.Now().Add(12 * time.Minute))
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
		})
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(12 * time.Minute))
			return nil
		})

		resetTimer := time.NewTimer(23*time.Hour + 30*time.Minute)
		errCh := make(chan error, 1)
		go func() { errCh <- w.readLoop(ctx, conn, tradeCh, depthCh) }()

		select {
		case <-ctx.Done():
			_ = conn.Close()
			resetTimer.Stop()
			return
		case <-resetTimer.C:
			w.logger.Info("planned reconnect at 23.5h")
			_ = conn.Close()
		case err := <-errCh:
			if err != nil {
				w.logger.WithError(err).Warn("websocket read loop exited")
			}
			_ = conn.Close()
		}
	}
}

func (w *WebSocketManager) connect() (*websocket.Conn, error) {
	streamNames := make([]string, 0, len(w.symbols)*2)
	for _, s := range w.symbols {
		sym := strings.ToLower(s)
		if w.withDepth {
			streamNames = append(streamNames, sym+"@depth@100ms")
		}
		if w.withTrade {
			streamNames = append(streamNames, sym+"@"+w.tradeStream)
		}
	}
	if len(streamNames) == 0 {
		return nil, fmt.Errorf("no streams configured")
	}

	u, err := url.Parse(w.baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/stream"
	q := u.Query()
	q.Set("streams", strings.Join(streamNames, "/"))
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", u.String(), err)
	}
	return conn, nil
}

func (w *WebSocketManager) readLoop(ctx context.Context, conn *websocket.Conn, tradeCh chan<- TradeEvent, depthCh chan<- DepthUpdate) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(data, &envelope); err != nil {
			continue
		}

		streamRaw, ok1 := envelope["stream"]
		payload, ok2 := envelope["data"]
		if !ok1 || !ok2 {
			continue
		}

		var streamName string
		if err := json.Unmarshal(streamRaw, &streamName); err != nil {
			continue
		}

		if w.withTrade && strings.Contains(strings.ToLower(streamName), "@"+strings.ToLower(w.tradeStream)) {
			evt, err := parseTrade(payload)
			if err == nil {
				select {
				case tradeCh <- evt:
				case <-ctx.Done():
					return nil
				}
			}
			continue
		}

		if w.withDepth && strings.Contains(streamName, "@depth") {
			evt, err := parseDepth(payload)
			if err == nil {
				select {
				case depthCh <- evt:
				case <-ctx.Done():
					return nil
				}
			}
		}
	}
}

func parseTrade(data []byte) (TradeEvent, error) {
	var p aggTradePayload
	if err := json.Unmarshal(data, &p); err != nil {
		return TradeEvent{}, err
	}
	price, err := strconv.ParseFloat(p.Price, 64)
	if err != nil {
		return TradeEvent{}, err
	}
	qty, err := strconv.ParseFloat(p.Qty, 64)
	if err != nil {
		return TradeEvent{}, err
	}
	return TradeEvent{
		Symbol:    strings.ToUpper(p.Symbol),
		TradeID:   p.AggTrade,
		EventTime: time.UnixMilli(p.EventTime),
		Price:     price,
		Qty:       qty,
	}, nil
}

func parseDepth(data []byte) (DepthUpdate, error) {
	var p depthPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return DepthUpdate{}, err
	}

	bids := make([]DepthLevel, 0, len(p.Bids))
	for _, b := range p.Bids {
		if len(b) < 2 {
			continue
		}
		price, err1 := strconv.ParseFloat(b[0], 64)
		qty, err2 := strconv.ParseFloat(b[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		bids = append(bids, DepthLevel{Price: price, Qty: qty})
	}

	asks := make([]DepthLevel, 0, len(p.Asks))
	for _, a := range p.Asks {
		if len(a) < 2 {
			continue
		}
		price, err1 := strconv.ParseFloat(a[0], 64)
		qty, err2 := strconv.ParseFloat(a[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		asks = append(asks, DepthLevel{Price: price, Qty: qty})
	}

	return DepthUpdate{
		Symbol:    strings.ToUpper(p.Symbol),
		FirstID:   p.FirstID,
		UpdateID:  p.UpdateID,
		PrevID:    p.PrevID,
		EventTime: time.UnixMilli(p.EventTime),
		Bids:      bids,
		Asks:      asks,
	}, nil
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	v := time.Second << (attempt - 1)
	if v > 30*time.Second {
		return 30 * time.Second
	}
	return v
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
