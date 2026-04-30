package main

import (
	"math"
)

// EMA calculates the Exponential Moving Average
func EMA(data []float64, period int) []float64 {
	if len(data) == 0 {
		return nil
	}
	ema := make([]float64, len(data))
	multiplier := 2.0 / float64(period+1)

	ema[0] = data[0]
	for i := 1; i < len(data); i++ {
		ema[i] = (data[i]-ema[i-1])*multiplier + ema[i-1]
	}
	return ema
}

// RSI calculates the Relative Strength Index
func RSI(data []float64, period int) float64 {
	if len(data) <= period {
		return 50.0
	}
	var gains, losses float64
	for i := 1; i <= period; i++ {
		diff := data[i] - data[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	for i := period + 1; i < len(data); i++ {
		diff := data[i] - data[i-1]
		if diff > 0 {
			avgGain = (avgGain*float64(period-1) + diff) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) - diff) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// MACD calculates Moving Average Convergence Divergence
func MACD(data []float64) (macd, signal, hist float64) {
	if len(data) < 26 {
		return 0, 0, 0
	}
	ema12 := EMA(data, 12)
	ema26 := EMA(data, 26)

	macdLine := make([]float64, len(data))
	for i := range data {
		macdLine[i] = ema12[i] - ema26[i]
	}

	signalLine := EMA(macdLine, 9)
	
	lastIdx := len(data) - 1
	return macdLine[lastIdx], signalLine[lastIdx], macdLine[lastIdx] - signalLine[lastIdx]
}

// BollingerBands calculates the BB
func BollingerBands(data []float64, period int, stdDevMult float64) (upper, lower float64) {
	if len(data) < period {
		return 0, 0
	}
	lastData := data[len(data)-period:]
	var sum float64
	for _, v := range lastData {
		sum += v
	}
	sma := sum / float64(period)

	var variance float64
	for _, v := range lastData {
		variance += math.Pow(v-sma, 2)
	}
	stdDev := math.Sqrt(variance / float64(period))

	return sma + stdDevMult*stdDev, sma - stdDevMult*stdDev
}

// ATR calculates Average True Range
func ATR(prices []PricePoint, period int) float64 {
	if len(prices) < period+1 {
		return 0
	}
	tr := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		h_l := prices[i].High - prices[i].Low
		h_pc := math.Abs(prices[i].High - prices[i-1].Close)
		l_pc := math.Abs(prices[i].Low - prices[i-1].Close)
		tr[i-1] = math.Max(h_l, math.Max(h_pc, l_pc))
	}

	var sum float64
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	atr := sum / float64(period)

	for i := period; i < len(tr); i++ {
		atr = (atr*float64(period-1) + tr[i]) / float64(period)
	}
	return atr
}
