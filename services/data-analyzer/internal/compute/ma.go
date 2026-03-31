package compute

import "math"

// SMA returns the simple moving average of the last `period` values.
// Returns (0, false) when there is insufficient data.
func SMA(values []float64, period int) (float64, bool) {
	if period <= 0 || len(values) < period {
		return 0, false
	}
	slice := values[len(values)-period:]
	var sum float64
	for _, v := range slice {
		sum += v
	}
	return sum / float64(period), true
}

// EMA returns the exponential moving average using a Wilder-style multiplier
// k = 2/(period+1), seeded from the first `period`-bar SMA.
// Returns (0, false) when there is insufficient data.
func EMA(values []float64, period int) (float64, bool) {
	if period <= 0 || len(values) < period {
		return 0, false
	}
	k := 2.0 / float64(period+1)
	var seed float64
	for i := 0; i < period; i++ {
		seed += values[i]
	}
	ema := seed / float64(period)
	for i := period; i < len(values); i++ {
		ema = values[i]*k + ema*(1-k)
	}
	return ema, true
}

// RSI returns the Wilder-smoothed Relative Strength Index for the given period.
// Requires at least period+1 data points.
// Returns (0, false) when there is insufficient data.
func RSI(closes []float64, period int) (float64, bool) {
	if period <= 0 || len(closes) < period+1 {
		return 0, false
	}
	var gains, losses float64
	for i := 1; i <= period; i++ {
		d := closes[i] - closes[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	for i := period + 1; i < len(closes); i++ {
		d := closes[i] - closes[i-1]
		if d > 0 {
			avgGain = (avgGain*float64(period-1) + d) / float64(period)
			avgLoss = avgLoss * float64(period-1) / float64(period)
		} else {
			avgGain = avgGain * float64(period-1) / float64(period)
			avgLoss = (avgLoss*float64(period-1) - d) / float64(period)
		}
	}
	if avgLoss == 0 {
		return 100, true
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs), true
}

// EMASeries returns the EMA at each bar using the same seeding rule as EMA().
// Indices 0..period-2 are NaN; index period-1 is the SMA seed, then EMA continues.
func EMASeries(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || n < period {
		return out
	}
	k := 2.0 / float64(period+1)
	var seed float64
	for i := 0; i < period; i++ {
		seed += values[i]
	}
	ema := seed / float64(period)
	out[period-1] = ema
	for i := period; i < n; i++ {
		ema = values[i]*k + ema*(1-k)
		out[i] = ema
	}
	return out
}

// RSISeries returns Wilder RSI per bar. Indices 0..period-1 are NaN; RSI starts at index `period`.
func RSISeries(closes []float64, period int) []float64 {
	n := len(closes)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || n < period+1 {
		return out
	}
	var gains, losses float64
	for i := 1; i <= period; i++ {
		d := closes[i] - closes[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	out[period] = rsiFromAvgs(avgGain, avgLoss)
	for i := period + 1; i < n; i++ {
		d := closes[i] - closes[i-1]
		if d > 0 {
			avgGain = (avgGain*float64(period-1) + d) / float64(period)
			avgLoss = avgLoss * float64(period-1) / float64(period)
		} else {
			avgGain = avgGain * float64(period-1) / float64(period)
			avgLoss = (avgLoss*float64(period-1) - d) / float64(period)
		}
		out[i] = rsiFromAvgs(avgGain, avgLoss)
	}
	return out
}

func rsiFromAvgs(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

// RollingStdPop returns the population standard deviation of the last `period` closes ending at end index `end` (inclusive).
func RollingStdPop(closes []float64, period, end int) (float64, bool) {
	if period <= 0 || end < period-1 || end >= len(closes) {
		return 0, false
	}
	start := end - period + 1
	var sum float64
	for i := start; i <= end; i++ {
		sum += closes[i]
	}
	mean := sum / float64(period)
	var sq float64
	for i := start; i <= end; i++ {
		d := closes[i] - mean
		sq += d * d
	}
	return math.Sqrt(sq / float64(period)), true
}
