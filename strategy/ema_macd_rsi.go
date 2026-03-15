package strategy

import "math"

type EmaMacdRsiStrategy struct {
	strategyID string
}

func NewEmaMacdRsiStrategy(id string) *EmaMacdRsiStrategy {
	if id == "" {
		id = "ema_macd_rsi"
	}
	return &EmaMacdRsiStrategy{strategyID: id}
}

func (s *EmaMacdRsiStrategy) ID() string {
	return s.strategyID
}

func (s *EmaMacdRsiStrategy) OnTick(snapshot MarketSnapshot, ctx *StrategyContext) []StrategyDecision {
	if ctx == nil || ctx.Meta == nil {
		return nil
	}

	closes, ok1 := ctx.Meta["closes"].([]float64)
	highs, ok2 := ctx.Meta["highs"].([]float64)
	lows, ok3 := ctx.Meta["lows"].([]float64)

	if !ok1 || !ok2 || !ok3 || len(closes) < 55 {
		return nil
	}

	price := snapshot.MidPrice
	if price <= 0 {
		price = closes[len(closes)-1]
	}

	ema9 := ema(closes, 9)
	ema21 := ema(closes, 21)
	ema55 := ema(closes, 55)
	rsi := rsi14(closes)
	macdLine, signalLine := macd(closes)
	atr := atr14(highs, lows, closes)

	e9 := last(ema9)
	e21 := last(ema21)
	e55 := last(ema55)
	r := last(rsi)
	ml := last(macdLine)
	ms := last(signalLine)
	atrVal := last(atr)

	// Save latest indicators to meta so the engine can log them if needed
	ctx.Meta["ema21"] = e21
	ctx.Meta["rsi"] = r
	ctx.Meta["atr"] = atrVal

	longCond := e9 > e21 && e21 > e55 && price > e21 && r >= 45 && r <= 70 && ml > ms
	shortCond := e9 < e21 && e21 < e55 && price < e21 && r >= 30 && r <= 55 && ml < ms

	var orders []StrategyDecision

	if ctx.PositionQty == 0 {
		var side string
		var sl float64
		var tp float64
		if longCond {
			side = "buy"
			sl = price - atrVal*2
			tp = price + atrVal*4
		} else if shortCond {
			side = "sell"
			sl = price + atrVal*2
			tp = price - atrVal*4
		}

		if side != "" {
			orders = append(orders, StrategyDecision{
				Action:     "place_order",
				Instrument: snapshot.Instrument,
				Side:       side,
				Size:       0, // Engine calculates size
				LimitPrice: price,
				OrderType:  "Market",
				Meta: map[string]interface{}{
					"signal": "entry",
					"sl":     sl,
					"tp":     tp,
				},
			})
		}
	}

	return orders
}

func ema(data []float64, period int) []float64 {
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

func rsi14(data []float64) []float64 {
	period := 14
	if len(data) < period+1 {
		return nil
	}
	result := make([]float64, len(data))
	var avgGain, avgLoss float64
	for i := 1; i <= period; i++ {
		diff := data[i] - data[i-1]
		if diff > 0 {
			avgGain += diff
		} else {
			avgLoss += math.Abs(diff)
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)
	if avgLoss == 0 {
		result[period] = 100
	} else {
		result[period] = 100 - 100/(1+(avgGain/avgLoss))
	}

	for i := period+1; i < len(data); i++ {
		diff := data[i] - data[i-1]
		gain, loss := 0.0, 0.0
		if diff > 0 {
			gain = diff
		} else {
			loss = math.Abs(diff)
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		if avgLoss == 0 {
			result[i] = 100
		} else {
			result[i] = 100 - 100/(1+(avgGain/avgLoss))
		}
	}
	return result
}

func macd(data []float64) (macdLine, signalLine []float64) {
	ema12 := ema(data, 12)
	ema26 := ema(data, 26)
	if ema12 == nil || ema26 == nil {
		return nil, nil
	}
	ml := make([]float64, len(data))
	for i := range data {
		ml[i] = ema12[i] - ema26[i]
	}
	sl := ema(ml[25:], 9)
	fullSL := make([]float64, len(data))
	copy(fullSL[25+8:], sl[8:])
	return ml, fullSL
}

func atr14(highs, lows, closes []float64) []float64 {
	period := 14
	n := len(closes)
	if n < period+1 {
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
	for i := period+1; i < n; i++ {
		result[i] = (result[i-1]*float64(period-1) + trueRanges[i]) / float64(period)
	}
	return result
}

func last(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}
