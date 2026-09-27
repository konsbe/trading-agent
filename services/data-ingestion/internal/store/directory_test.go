package store

import "testing"

// Finnhub /stock/symbol types as stored in universe_symbols on 2026-09-27.
func TestDirectoryAssetType(t *testing.T) {
	for typ, want := range map[string]string{
		"Common Stock": "equity", "ADR": "equity", "REIT": "equity", "NY Reg Shrs": "equity", "MLP": "equity",
		"ETP": "etf", "Closed-End Fund": "etf",
		"Equity WRT": "other", "Unit": "other", "Right": "other", "PUBLIC": "other", "": "other",
	} {
		if got := DirectoryAssetType(typ); got != want {
			t.Errorf("DirectoryAssetType(%q) = %q, want %q", typ, got, want)
		}
	}
}
