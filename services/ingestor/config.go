package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	BinanceWSURL      string
	BinanceRESTURL    string
	TradeStream       string
	Symbols           []string
	LogLevel          string
	BatchSize         int
	FlushInterval     time.Duration
	StartWithBackfill bool
	BackfillType      BackfillType
	BackfillMinutes   int
	KeepUnreadyTrade  bool
	StartupTradeMode  StartupTradeMode
	RESTWeightPerMin  int
	RESTMaxRetries    int
	RESTBaseBackoff   time.Duration
	RESTMaxBackoff    time.Duration
	RESTBanCooldown   time.Duration
}

func LoadConfig() (Config, error) {
	_ = godotenv.Load()

	backfillType, err := parseBackfillType(getenv("BACKFILL_TYPE", "approx"))
	if err != nil {
		return Config{}, err
	}
	startupTradeMode, err := parseStartupTradeMode(getenv("STARTUP_PRE_READY_TRADE_MODE", "approx"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		DatabaseURL:    getenv("DATABASE_URL", "postgresql://xno:xno_pass@timescaledb:5432/xno_market?sslmode=disable"),
		BinanceWSURL:   getenv("BINANCE_WS_URL", "wss://fstream.binance.com"),
		BinanceRESTURL: getenv("BINANCE_REST_URL", "https://fapi.binance.com"),
		TradeStream:    "aggTrade",
		Symbols: parseSymbols(getenv("SYMBOLS",
			"BTCUSDT,ETHUSDT,SOLUSDT,BNBUSDT,XRPUSDT,DOGEUSDT,ADAUSDT,LINKUSDT,AVAXUSDT,TRXUSDT,TONUSDT,DOTUSDT,MATICUSDT,LTCUSDT,NEARUSDT,APTUSDT,ATOMUSDT,FILUSDT,OPUSDT,ARBUSDT")),
		LogLevel:          getenv("LOG_LEVEL", "info"),
		BatchSize:         atoiWithDefault(getenv("BATCH_SIZE", "100"), 100),
		FlushInterval:     time.Duration(atoiWithDefault(getenv("FLUSH_INTERVAL_MS", "50"), 50)) * time.Millisecond,
		StartWithBackfill: strings.EqualFold(getenv("START_WITH_BACKFILL", "false"), "true"),
		BackfillType:      backfillType,
		BackfillMinutes:   atoiWithDefault(getenv("BACKFILL_MINUTES", "15"), 15),
		KeepUnreadyTrade:  strings.EqualFold(getenv("KEEP_UNREADY_TRADE", "true"), "true"),
		StartupTradeMode:  startupTradeMode,
		RESTWeightPerMin:  atoiWithDefault(getenv("REST_WEIGHT_PER_MIN", "2400"), 2400),
		RESTMaxRetries:    atoiWithDefault(getenv("REST_MAX_RETRIES", "6"), 6),
		RESTBaseBackoff:   time.Duration(atoiWithDefault(getenv("REST_BASE_BACKOFF_MS", "1000"), 1000)) * time.Millisecond,
		RESTMaxBackoff:    time.Duration(atoiWithDefault(getenv("REST_MAX_BACKOFF_MS", "30000"), 30000)) * time.Millisecond,
		RESTBanCooldown:   time.Duration(atoiWithDefault(getenv("REST_BAN_COOLDOWN_SEC", "120"), 120)) * time.Second,
	}

	if cfg.RESTWeightPerMin <= 0 {
		cfg.RESTWeightPerMin = 2400
	}
	if cfg.RESTMaxRetries < 0 {
		cfg.RESTMaxRetries = 0
	}
	if cfg.StartWithBackfill && cfg.BackfillType == BackfillTypeApprox && cfg.KeepUnreadyTrade && cfg.StartupTradeMode != StartupTradeModeApprox {
		return Config{}, fmt.Errorf("invalid startup config: STARTUP_PRE_READY_TRADE_MODE must be 'approx' when START_WITH_BACKFILL=true and BACKFILL_TYPE=approx and KEEP_UNREADY_TRADE=true")
	}

	return cfg, nil
}

func parseBackfillType(s string) (BackfillType, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case string(BackfillTypeApprox):
		return BackfillTypeApprox, nil
	case string(BackfillTypeNull):
		return BackfillTypeNull, nil
	default:
		return "", fmt.Errorf("invalid BACKFILL_TYPE: %q (allowed: approx|null)", s)
	}
}

func parseStartupTradeMode(s string) (StartupTradeMode, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case string(StartupTradeModeApprox):
		return StartupTradeModeApprox, nil
	case string(StartupTradeModeNull):
		return StartupTradeModeNull, nil
	default:
		return "", fmt.Errorf("invalid STARTUP_PRE_READY_TRADE_MODE: %q (allowed: approx|null)", s)
	}
}

func parseSymbols(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		n := strings.ToUpper(strings.TrimSpace(p))
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

func getenv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func atoiWithDefault(v string, fallback int) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
