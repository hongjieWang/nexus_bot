package strategy

import "math"

type marketRegime struct {
	Direction       string
	IsTrending      bool
	IsVolatile      bool
	ShouldStandDown bool
	ATRBps          float64
}

func detectMarketRegime(snapshot MarketSnapshot, ctx *StrategyContext, atrLimitBps, trendATRMult float64) marketRegime {
	if ctx == nil || ctx.Meta == nil {
		return marketRegime{}
	}

	closes, okCloses := ctx.Meta["closes"].([]float64)
	highs, okHighs := ctx.Meta["highs"].([]float64)
	lows, okLows := ctx.Meta["lows"].([]float64)
	if !okCloses || !okHighs || !okLows || len(closes) < 55 || len(highs) < 15 || len(lows) < 15 {
		return marketRegime{}
	}

	price := snapshot.MidPrice
	if price <= 0 {
		price = closes[len(closes)-1]
	}
	if price <= 0 {
		return marketRegime{}
	}

	ema21 := seriesEMA(closes, 21)
	ema55 := seriesEMA(closes, 55)
	atr := seriesATR14(highs, lows, closes)
	if len(ema21) == 0 || len(ema55) == 0 || len(atr) == 0 {
		return marketRegime{}
	}

	e21 := lastSeriesValue(ema21)
	e55 := lastSeriesValue(ema55)
	atrVal := lastSeriesValue(atr)
	if atrVal <= 0 {
		return marketRegime{}
	}

	regime := marketRegime{
		ATRBps: atrVal / price * 10000.0,
	}

	emaGap := math.Abs(e21 - e55)
	priceGap := math.Abs(price - e21)
	if e21 > e55 && price > e21 && emaGap >= atrVal*0.35 && priceGap >= atrVal*trendATRMult {
		regime.Direction = "up"
		regime.IsTrending = true
	} else if e21 < e55 && price < e21 && emaGap >= atrVal*0.35 && priceGap >= atrVal*trendATRMult {
		regime.Direction = "down"
		regime.IsTrending = true
	}

	regime.IsVolatile = regime.ATRBps >= atrLimitBps
	regime.ShouldStandDown = regime.IsTrending || regime.IsVolatile
	return regime
}

func seriesEMA(data []float64, period int) []float64 {
	if len(data) < period {
		return nil
	}
	result := make([]float64, len(data))
	k := 2.0 / float64(period+1)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	result[period-1] = sum / float64(period)
	for i := period; i < len(data); i++ {
		result[i] = data[i]*k + result[i-1]*(1-k)
	}
	return result
}

func seriesATR14(highs, lows, closes []float64) []float64 {
	period := 14
	n := len(closes)
	if n < period+1 || len(highs) < n || len(lows) < n {
		return nil
	}
	trueRanges := make([]float64, n)
	for i := 1; i < n; i++ {
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		tr := hl
		if hpc > tr {
			tr = hpc
		}
		if lpc > tr {
			tr = lpc
		}
		trueRanges[i] = tr
	}
	result := make([]float64, n)
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trueRanges[i]
	}
	result[period] = sum / float64(period)
	for i := period + 1; i < n; i++ {
		result[i] = (result[i-1]*float64(period-1) + trueRanges[i]) / float64(period)
	}
	return result
}

func lastSeriesValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}
