package binance_sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (c *Client) Stream(ctx context.Context, symbol string) (<-chan Snapshot, <-chan error) {
	out := make(chan Snapshot, 256)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)

		sseURL, err := c.endpoint("/sse", map[string]string{"symbol": symbol})
		if err != nil {
			errCh <- err
			return
		}

		backoff := time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
			if reqErr != nil {
				errCh <- reqErr
				return
			}
			req.Header.Set("Accept", "text/event-stream")
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}

			resp, doErr := c.httpClient.Do(req)
			if doErr != nil {
				if !sleepOrDone(ctx, backoff) {
					return
				}
				if backoff < 15*time.Second {
					backoff *= 2
				}
				continue
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				errCh <- fmt.Errorf("sse status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
				if !sleepOrDone(ctx, backoff) {
					return
				}
				if backoff < 15*time.Second {
					backoff *= 2
				}
				continue
			}

			backoff = time.Second
			scanner := bufio.NewScanner(resp.Body)
			var eventName string
			var dataLines []string

			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					if eventName == "snapshot" && len(dataLines) > 0 {
						payload := strings.Join(dataLines, "\n")
						var s Snapshot
						if err := json.Unmarshal([]byte(payload), &s); err == nil {
							select {
							case out <- s:
							case <-ctx.Done():
								resp.Body.Close()
								return
							}
						}
					}
					eventName = ""
					dataLines = dataLines[:0]
					continue
				}

				if strings.HasPrefix(line, ":") {
					continue
				}
				if strings.HasPrefix(line, "event:") {
					eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
					continue
				}
				if strings.HasPrefix(line, "data:") {
					dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				}
			}
			resp.Body.Close()

			if scanErr := scanner.Err(); scanErr != nil && ctx.Err() == nil {
				errCh <- scanErr
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	return out, errCh
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
