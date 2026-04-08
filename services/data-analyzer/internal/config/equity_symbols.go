package config

import "strings"

func mergeUniquePreserveOrder(groups ...[]string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, g := range groups {
		for _, s := range g {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			key := strings.ToUpper(s)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// MergedAlpacaEquitySymbols matches data-ingestion: primary ALPACA_DATA_SYMBOLS, else
// union of EQUITY_SYMBOLS_STOCKS + EQUITY_SYMBOLS_ETFS + EQUITY_SYMBOLS_COMMODITY_ETFS.
func MergedAlpacaEquitySymbols() []string {
	if s := splitCSV("ALPACA_DATA_SYMBOLS"); len(s) > 0 {
		return s
	}
	return mergeUniquePreserveOrder(
		splitCSV("EQUITY_SYMBOLS_STOCKS"),
		splitCSV("EQUITY_SYMBOLS_ETFS"),
		splitCSV("EQUITY_SYMBOLS_COMMODITY_ETFS"),
	)
}
