package momentum

import (
	"testing"
	"time"
)

var testSession = time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

// freshFiling stamps a v2 input with a share count filed a month before the
// session, well inside the default age limit, so tests about other gates are
// not accidentally testing staleness.
func freshFiling(in GateInput) GateInput {
	in.SessionDate = testSession
	in.MarketCapPITFiled = testSession.AddDate(0, -1, 0)
	return in
}

func v2Default() GateConfig {
	cfg := DefaultGateConfig()
	cfg.Version = GateV2
	return cfg
}

func evalFiled(t *testing.T, cfg GateConfig, filed time.Time) GateResult {
	t.Helper()
	pit := 1e9 // inside the market band: only the filing age can fail it
	return EvaluateGates(passingFeatures(t, 5.0), GateInput{
		MarketCapPIT: &pit, MarketCapPITFiled: filed, SessionDate: testSession,
	}, cfg)
}

func TestDefaultGateConfig_PITMaxFilingAgeIs15Months(t *testing.T) {
	if got := DefaultGateConfig().PITMaxFilingAgeMonths; got != 15 {
		t.Fatalf("PITMaxFilingAgeMonths = %d, want 15", got)
	}
}

func TestGateV2_FreshFilingPasses(t *testing.T) {
	res := evalFiled(t, v2Default(), testSession.AddDate(0, -3, 0))
	if !res.Passed {
		t.Fatalf("v2 rejected a 3-month-old filing: %v", res.Failures)
	}
}

// Filed exactly N months before the session is inside the limit; one day
// earlier is outside it.
func TestGateV2_FilingAgeBoundary(t *testing.T) {
	boundary := time.Date(2025, 6, 25, 0, 0, 0, 0, time.UTC) // 2026-09-25 - 15 months
	if res := evalFiled(t, v2Default(), boundary); !res.Passed {
		t.Errorf("filing exactly 15 months old was rejected: %v", res.Failures)
	}
	// Time of day and zone must not move the boundary.
	late := time.Date(2025, 6, 25, 23, 59, 0, 0, time.FixedZone("EDT", -4*3600))
	if res := evalFiled(t, v2Default(), late); !res.Passed {
		t.Errorf("boundary filing with a time-of-day was rejected: %v", res.Failures)
	}
	res := evalFiled(t, v2Default(), boundary.AddDate(0, 0, -1))
	if res.Passed || !hasFailure(res, GateMarketCapPITUnavailable) {
		t.Errorf("filing one day past 15 months: passed=%v failures=%v, want %s",
			res.Passed, res.Failures, GateMarketCapPITUnavailable)
	}
}

// The JAGX case: an in-band cap computed from a years-old share count must be
// rejected exactly like a symbol with no filing, not judged against the band.
func TestGateV2_StaleFilingIsUnavailable(t *testing.T) {
	res := evalFiled(t, v2Default(), time.Date(2018, 5, 15, 0, 0, 0, 0, time.UTC))
	if res.Passed {
		t.Fatal("v2 passed on a share count filed in 2018 — the stale count decided the market band")
	}
	if !hasFailure(res, GateMarketCapPITUnavailable) {
		t.Errorf("failures = %v, want %s", res.Failures, GateMarketCapPITUnavailable)
	}
	for _, f := range []string{GateMarketCapTooLow, GateMarketCapTooHigh, GateMarketCapUnavailable} {
		if hasFailure(res, f) {
			t.Errorf("stale filing also recorded %s; it must read as unavailable only: %v", f, res.Failures)
		}
	}
}

func TestGateV2_ZeroLimitDisablesAgeCheck(t *testing.T) {
	cfg := v2Default()
	cfg.PITMaxFilingAgeMonths = 0
	if res := evalFiled(t, cfg, time.Date(2018, 5, 15, 0, 0, 0, 0, time.UTC)); !res.Passed {
		t.Errorf("limit 0 still rejected a stale filing: %v", res.Failures)
	}
	// With no limit, missing dates are irrelevant too (the backtest's
	// reproduction mode).
	pit := 1e9
	if res := EvaluateGates(passingFeatures(t, 5.0), GateInput{MarketCapPIT: &pit}, cfg); !res.Passed {
		t.Errorf("limit 0 rejected an undated PIT cap: %v", res.Failures)
	}
}

// With a limit set, an undated PIT cap cannot be shown to be fresh and fails
// closed, so a caller that forgets to pass the dates cannot bypass the limit.
func TestGateV2_MissingDatesFailClosedWhenLimitSet(t *testing.T) {
	pit := 1e9
	for name, in := range map[string]GateInput{
		"no filed date":   {MarketCapPIT: &pit, SessionDate: testSession},
		"no session date": {MarketCapPIT: &pit, MarketCapPITFiled: testSession.AddDate(0, -1, 0)},
	} {
		res := EvaluateGates(passingFeatures(t, 5.0), in, v2Default())
		if res.Passed || !hasFailure(res, GateMarketCapPITUnavailable) {
			t.Errorf("%s: passed=%v failures=%v, want %s", name, res.Passed, res.Failures, GateMarketCapPITUnavailable)
		}
	}
}

func TestGateV1_UnaffectedByFilingAge(t *testing.T) {
	cfg := DefaultGateConfig()
	cfg.Version = GateV1
	today := 1e9
	pit := 1e9
	res := EvaluateGates(passingFeatures(t, 5.0), GateInput{
		MarketCap: &today, MarketCapPIT: &pit,
		MarketCapPITFiled: time.Date(2018, 5, 15, 0, 0, 0, 0, time.UTC), SessionDate: testSession,
	}, cfg)
	if !res.Passed {
		t.Errorf("v1 was affected by the PIT filing age: %v", res.Failures)
	}
	if hasFailure(res, GateMarketCapPITUnavailable) {
		t.Errorf("v1 recorded %s: %v", GateMarketCapPITUnavailable, res.Failures)
	}
}
