# Go SDK (`binance_sdk`)

This package provides native Go access to the backend market data API.

## Features

- `Latest(...)`: fetch most recent normalized snapshots
- `Stream(...)`: subscribe to live snapshots from `/sse`
- `Stats(...)`: fetch backend runtime stats

## Example

```go
ctx := context.Background()
client := binance_sdk.NewClient("http://localhost:8080", "")

latest, err := client.Latest(ctx, "BTCUSDT", 100)
if err != nil {
    panic(err)
}
fmt.Println("latest rows:", len(latest))

events, errs := client.Stream(ctx, "BTCUSDT")
for i := 0; i < 5; i++ {
    select {
    case e := <-events:
        fmt.Println(e.Symbol, e.TradeID, e.Price)
    case err := <-errs:
        panic(err)
    }
}
```
