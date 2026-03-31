package compute

import "math"

// Pattern holds a detected candlestick pattern name and its directional bias.
type Pattern struct {
	Name      string // e.g. "hammer", "bearish_engulfing"
	Sentiment int    // +1 bullish, -1 bearish, 0 neutral
}

// DetectPatterns analyses the last 1–3 bars (bars ordered oldest-first) and
// returns every matched candlestick pattern. At least 1 bar is required; 2-bar
// patterns are skipped when only 1 bar is available.
func DetectPatterns(bars []Bar) []Pattern {
	if len(bars) == 0 {
		return nil
	}
	cur := bars[len(bars)-1]
	body := math.Abs(cur.Close - cur.Open)
	totalRange := cur.High - cur.Low
	if totalRange <= 0 {
		return nil
	}
	upper := cur.High - math.Max(cur.Open, cur.Close)
	lower := math.Min(cur.Open, cur.Close) - cur.Low

	var out []Pattern

	// ── Single-bar patterns ──────────────────────────────────────────────────

	// Doji: body is ≤ 5 % of total range.
	if body/totalRange <= 0.05 && body > 0 {
		out = append(out, Pattern{"doji", 0})
	}

	// Hammer: small body in upper third of range, long lower shadow (≥ 2× body),
	// tiny upper shadow (≤ 0.5× body). Signals potential bullish reversal.
	if body > 0 && lower >= 2*body && upper <= 0.5*body {
		out = append(out, Pattern{"hammer", 1})
	}

	// Shooting Star: inverse of hammer — long upper shadow, small body near low.
	if body > 0 && upper >= 2*body && lower <= 0.5*body {
		out = append(out, Pattern{"shooting_star", -1})
	}

	// Pin Bar: dominant wick ≥ 67 % of total range, body ≤ 15 % of range.
	// Avoid double-counting bars already classified as hammer / shooting star.
	dominantWick := math.Max(upper, lower)
	isHammer := body > 0 && lower >= 2*body && upper <= 0.5*body
	isStar := body > 0 && upper >= 2*body && lower <= 0.5*body
	if !isHammer && !isStar && dominantWick/totalRange >= 0.67 && body/totalRange <= 0.15 {
		sentiment := 0
		if lower > upper {
			sentiment = 1 // rejection of low prices → bullish bias
		} else {
			sentiment = -1 // rejection of high prices → bearish bias
		}
		out = append(out, Pattern{"pin_bar", sentiment})
	}

	if len(bars) < 2 {
		return out
	}

	// ── Two-bar patterns ─────────────────────────────────────────────────────

	prev := bars[len(bars)-2]
	curBullish := cur.Close > cur.Open
	prevBullish := prev.Close > prev.Open
	prevBodyHigh := math.Max(prev.Open, prev.Close)
	prevBodyLow := math.Min(prev.Open, prev.Close)

	// Bullish Engulfing: previous bar bearish, current bar bullish and its body
	// completely engulfs the previous body.
	if !prevBullish && curBullish &&
		cur.Open <= prevBodyLow && cur.Close >= prevBodyHigh {
		out = append(out, Pattern{"bullish_engulfing", 1})
	}

	// Bearish Engulfing: previous bar bullish, current bar bearish and engulfing.
	if prevBullish && !curBullish &&
		cur.Open >= prevBodyHigh && cur.Close <= prevBodyLow {
		out = append(out, Pattern{"bearish_engulfing", -1})
	}

	// Inside Bar: current high and low are entirely within the previous bar's range.
	// Indicates consolidation / compression before a potential breakout.
	if cur.High < prev.High && cur.Low > prev.Low {
		out = append(out, Pattern{"inside_bar", 0})
	}

	return out
}

// PatternSentiment aggregates the sentiment of a pattern list into +1, 0, or -1.
func PatternSentiment(patterns []Pattern) int {
	total := 0
	for _, p := range patterns {
		total += p.Sentiment
	}
	if total > 0 {
		return 1
	} else if total < 0 {
		return -1
	}
	return 0
}
