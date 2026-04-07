package main

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type SyncManager struct {
	baseURL  string
	client   *http.Client
	limiter  *RESTLimiter
	retryCfg RESTRetryConfig
	logger   *logrus.Logger
}

func NewSyncManager(baseURL string, limiter *RESTLimiter, retryCfg RESTRetryConfig, logger *logrus.Logger) *SyncManager {
	return &SyncManager{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		client:   &http.Client{Timeout: 10 * time.Second},
		limiter:  limiter,
		retryCfg: retryCfg,
		logger:   logger,
	}
}

func (s *SyncManager) SeedOrderBook(ctx context.Context, symbol string, ob *OrderBook) error {
	snapshot, err := s.fetchDepthSnapshot(ctx, symbol, 1000)
	if err != nil {
		return err
	}
	bids, asks, err := parseDepthSnapshot(snapshot)
	if err != nil {
		return err
	}
	ob.SetSnapshotLevels(snapshot.LastUpdateID, bids, asks)
	return nil
}

func (s *SyncManager) fetchDepthSnapshot(ctx context.Context, symbol string, limit int) (depthSnapshotResponse, error) {
	var out depthSnapshotResponse
	err := doBinanceJSON(
		ctx,
		s.client,
		s.limiter,
		s.retryCfg,
		depthRequestWeight(limit),
		"depth_snapshot",
		s.logger,
		func() (*http.Request, error) {
			u, err := url.Parse(s.baseURL + "/fapi/v1/depth")
			if err != nil {
				return nil, err
			}
			q := u.Query()
			q.Set("symbol", strings.ToUpper(symbol))
			q.Set("limit", strconv.Itoa(limit))
			u.RawQuery = q.Encode()
			return http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		},
		&out,
	)
	if err != nil {
		return depthSnapshotResponse{}, err
	}
	return out, nil
}

func parseDepthSnapshot(snapshot depthSnapshotResponse) ([]DepthLevel, []DepthLevel, error) {
	bids := make([]DepthLevel, 0, len(snapshot.Bids))
	for _, b := range snapshot.Bids {
		if len(b) < 2 {
			continue
		}
		p, err1 := strconv.ParseFloat(b[0], 64)
		q, err2 := strconv.ParseFloat(b[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		bids = append(bids, DepthLevel{Price: p, Qty: q})
	}

	asks := make([]DepthLevel, 0, len(snapshot.Asks))
	for _, a := range snapshot.Asks {
		if len(a) < 2 {
			continue
		}
		p, err1 := strconv.ParseFloat(a[0], 64)
		q, err2 := strconv.ParseFloat(a[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		asks = append(asks, DepthLevel{Price: p, Qty: q})
	}

	return bids, asks, nil
}
