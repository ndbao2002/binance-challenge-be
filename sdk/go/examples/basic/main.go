package main

import (
	"context"
	"fmt"
	"os"
	"time"

	binance_sdk "binance_sdk_go"
)

func main() {
	baseURL := getenv("API_BASE_URL", "http://localhost:8080")
	token := getenv("DASHBOARD_TOKEN", "")
	symbol := getenv("SYMBOL", "BTCUSDT")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := binance_sdk.NewClient(baseURL, token)

	latest, err := client.Latest(ctx, symbol, 20)
	if err != nil {
		panic(err)
	}
	fmt.Printf("latest rows for %s: %d\n", symbol, len(latest))
	if len(latest) > 0 {
		fmt.Printf("most recent: trade_id=%d price=%f\n", latest[0].TradeID, latest[0].Price)
	}

	events, errs := client.Stream(ctx, symbol)
	for i := 0; i < 3; i++ {
		select {
		case e, ok := <-events:
			if !ok {
				fmt.Println("stream ended")
				return
			}
			fmt.Printf("stream event: %s trade_id=%d price=%f\n", e.Symbol, e.TradeID, e.Price)
		case err := <-errs:
			if err != nil {
				panic(err)
			}
		}
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
