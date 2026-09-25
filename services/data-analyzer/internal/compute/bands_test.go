package compute

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

func TestClassifyRSI(t *testing.T) {
	cases := map[float64]string{
		0: RSIOversold, 29.99: RSIOversold,
		30: RSINormal, 50.1: RSINormal, 70: RSINormal,
		70.01: RSIOverbought, 100: RSIOverbought,
	}
	for v, want := range cases {
		if got := ClassifyRSI(v, BotRSIThresholds); got != want {
			t.Errorf("ClassifyRSI(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestClassifyADX(t *testing.T) {
	cases := map[float64]string{0: ADXNotStrong, 25: ADXNotStrong, 25.01: ADXStrong, 60: ADXStrong}
	for v, want := range cases {
		if got := ClassifyADX(v); got != want {
			t.Errorf("ClassifyADX(%v) = %q, want %q", v, got, want)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

func read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func number(t *testing.T, src, pattern string) float64 {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("pattern %q not found; the bot's threshold moved or was renamed", pattern)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// The bands are a port, so they must track the bot's source.
func TestBandsMatchTheBot(t *testing.T) {
	cfg := read(t, "services/analyst-bot/config.py")
	if v := number(t, cfg, `bot_rsi_oversold:\s*float\s*=\s*([0-9.]+)`); v != BotRSIThresholds.Oversold {
		t.Errorf("bot_rsi_oversold default %v, Go %v", v, BotRSIThresholds.Oversold)
	}
	if v := number(t, cfg, `bot_rsi_overbought:\s*float\s*=\s*([0-9.]+)`); v != BotRSIThresholds.Overbought {
		t.Errorf("bot_rsi_overbought default %v, Go %v", v, BotRSIThresholds.Overbought)
	}
	env := read(t, ".env.example")
	if v := number(t, env, `(?m)^BOT_RSI_OVERSOLD=([0-9.]+)`); v != BotRSIThresholds.Oversold {
		t.Errorf(".env.example BOT_RSI_OVERSOLD %v, Go %v", v, BotRSIThresholds.Oversold)
	}
	if v := number(t, env, `(?m)^BOT_RSI_OVERBOUGHT=([0-9.]+)`); v != BotRSIThresholds.Overbought {
		t.Errorf(".env.example BOT_RSI_OVERBOUGHT %v, Go %v", v, BotRSIThresholds.Overbought)
	}
	sq := read(t, "services/analyst-bot/actions/rules/bb_squeeze.py")
	if v := number(t, sq, `adx_val\s*>\s*([0-9.]+)`); v != ADXStrongTrend {
		t.Errorf("bb_squeeze.py ADX threshold %v, Go %v", v, ADXStrongTrend)
	}
}
