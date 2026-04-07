package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type RESTRetryConfig struct {
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	BanCooldown time.Duration
}

type RESTLimiter struct {
	mu       sync.Mutex
	capacity int
	window   time.Duration
	used     int
	resetAt  time.Time
}

func NewRESTLimiter(capacity int, window time.Duration) *RESTLimiter {
	if capacity <= 0 {
		capacity = 1800
	}
	if window <= 0 {
		window = time.Minute
	}
	now := time.Now()
	return &RESTLimiter{capacity: capacity, window: window, resetAt: now.Add(window)}
}

func (l *RESTLimiter) Wait(ctx context.Context, weight int) error {
	if l == nil {
		return nil
	}
	if weight <= 0 {
		weight = 1
	}

	for {
		l.mu.Lock()
		now := time.Now()
		if now.After(l.resetAt) {
			l.used = 0
			l.resetAt = now.Add(l.window)
		}

		if l.used+weight <= l.capacity {
			l.used += weight
			l.mu.Unlock()
			return nil
		}

		wait := time.Until(l.resetAt)
		l.mu.Unlock()

		if wait <= 0 {
			continue
		}

		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (l *RESTLimiter) ObserveUsed(used int) {
	if l == nil || used < 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.After(l.resetAt) {
		l.used = 0
		l.resetAt = now.Add(l.window)
	}
	if used > l.used {
		l.used = used
	}
}

func doBinanceJSON(
	ctx context.Context,
	client *http.Client,
	limiter *RESTLimiter,
	retryCfg RESTRetryConfig,
	weight int,
	op string,
	logger *logrus.Logger,
	requestFactory func() (*http.Request, error),
	out any,
) error {
	maxRetries := retryCfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := limiter.Wait(ctx, weight); err != nil {
			return err
		}

		req, err := requestFactory()
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("%s request failed after retries: %w", op, err)
			}
			wait := exponentialBackoff(attempt, retryCfg)
			if logger != nil {
				logger.WithFields(logrus.Fields{"op": op, "attempt": attempt + 1, "wait": wait.String()}).WithError(err).Warn("rest request failed; retrying")
			}
			if !sleepOrDone(ctx, wait) {
				return ctx.Err()
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			if usedWeight, ok := extractUsedWeight(resp.Header); ok {
				limiter.ObserveUsed(usedWeight)
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(out)
			resp.Body.Close()
			if decodeErr != nil {
				return fmt.Errorf("%s decode failed: %w", op, decodeErr)
			}
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		if usedWeight, ok := extractUsedWeight(resp.Header); ok {
			limiter.ObserveUsed(usedWeight)
		}
		resp.Body.Close()

		status := resp.StatusCode
		retryable := status == http.StatusTooManyRequests || status == http.StatusTeapot || status >= http.StatusInternalServerError
		if !retryable || attempt == maxRetries {
			return fmt.Errorf("%s status %d: %s", op, status, strings.TrimSpace(string(body)))
		}

		wait := retryAfterDelay(resp.Header.Get("Retry-After"), body, status, attempt, retryCfg)
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"op":      op,
				"status":  status,
				"attempt": attempt + 1,
				"wait":    wait.String(),
			}).Warn("rest rate/server limit encountered; retrying")
		}
		if !sleepOrDone(ctx, wait) {
			return ctx.Err()
		}
	}

	return fmt.Errorf("%s exhausted retries", op)
}

func retryAfterDelay(retryAfter string, body []byte, statusCode int, attempt int, cfg RESTRetryConfig) time.Duration {
	if retryAfter != "" {
		if sec, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
		if t, err := http.ParseTime(retryAfter); err == nil {
			d := time.Until(t)
			if d > 0 {
				return d
			}
		}
	}

	if d, ok := retryAfterFromBinanceBody(body); ok {
		return d
	}

	if statusCode == http.StatusTeapot && cfg.BanCooldown > 0 {
		return cfg.BanCooldown
	}

	return exponentialBackoff(attempt, cfg)
}

func retryAfterFromBinanceBody(body []byte) (time.Duration, bool) {
	if len(body) == 0 {
		return 0, false
	}

	type binanceErrData struct {
		RetryAfter float64 `json:"retryAfter"`
	}
	type binanceErr struct {
		RetryAfter float64        `json:"retryAfter"`
		Data       binanceErrData `json:"data"`
	}
	type binanceErrEnvelope struct {
		Error      binanceErr `json:"error"`
		RetryAfter float64    `json:"retryAfter"`
	}

	var env binanceErrEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return 0, false
	}

	raw := firstPositive(env.Error.Data.RetryAfter, env.Error.RetryAfter, env.RetryAfter)
	if raw <= 0 {
		return 0, false
	}

	now := time.Now()

	if raw > 1e12 {
		ts := time.UnixMilli(int64(raw))
		d := time.Until(ts)
		if d > 0 {
			return d, true
		}
		return 0, false
	}

	if raw > 1e9 {
		ts := time.Unix(int64(raw), 0)
		d := ts.Sub(now)
		if d > 0 {
			return d, true
		}
		return 0, false
	}

	if raw <= math.MaxFloat64/float64(time.Second) {
		d := time.Duration(raw * float64(time.Second))
		if d > 0 {
			return d, true
		}
	}

	return 0, false
}

func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func exponentialBackoff(attempt int, cfg RESTRetryConfig) time.Duration {
	base := cfg.BaseBackoff
	if base <= 0 {
		base = time.Second
	}
	max := cfg.MaxBackoff
	if max <= 0 {
		max = 30 * time.Second
	}

	wait := base
	for i := 0; i < attempt; i++ {
		if wait >= max {
			return max
		}
		wait *= 2
	}
	if wait > max {
		return max
	}
	return wait
}

func depthRequestWeight(limit int) int {
	switch {
	case limit <= 50:
		return 2
	case limit <= 100:
		return 5
	case limit <= 500:
		return 10
	default:
		return 20
	}
}

func extractUsedWeight(h http.Header) (int, bool) {
	maxUsed := -1
	for k, vals := range h {
		if !strings.HasPrefix(strings.ToUpper(k), "X-MBX-USED-WEIGHT-") {
			continue
		}
		if len(vals) == 0 {
			continue
		}
		v := strings.TrimSpace(vals[0])
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if n > maxUsed {
			maxUsed = n
		}
	}
	if maxUsed < 0 {
		return 0, false
	}
	return maxUsed, true
}
