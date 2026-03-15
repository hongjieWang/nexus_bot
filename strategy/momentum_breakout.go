package strategy

import "math"

// MomentumBreakoutStrategy — enter on volume + price breakout above/below N-period range.
type MomentumBreakoutStrategy struct {
	strategyID           string
	lookback             int
	breakbackThresholdBps float64
	volumeSurgeMult      float64
	trailingStopBps      float64
	size                 float64

	highs   []float64
	lows    []float64
	volumes []float64

	// 关键修复：维护历史最高/最低价以实现真正的追踪止损 (High #8)
	highWaterMark float64
	lowWaterMark  float64
}

func NewMomentumBreakoutStrategy(
	id string,
	lookback int,
	breakoutThresholdBps float64,
	volumeSurgeMult float64,
	trailingStopBps float64,
	size float64,
) *MomentumBreakoutStrategy {
	if id == "" {
		id = "momentum_breakout"
	}
	return &MomentumBreakoutStrategy{
		strategyID:           id,
		lookback:             lookback,
		breakbackThresholdBps: breakoutThresholdBps,
		volumeSurgeMult:      volumeSurgeMult,
		trailingStopBps:      trailingStopBps,
		size:                 size,
		highs:                make([]float64, 0, lookback),
		lows:                 make([]float64, 0, lookback),
		volumes:              make([]float64, 0, lookback),
		highWaterMark:        0,
		lowWaterMark:         0,
	}
}

func (s *MomentumBreakoutStrategy) ID() string {
	return s.strategyID
}

func (s *MomentumBreakoutStrategy) OnTick(snapshot MarketSnapshot, ctx *StrategyContext) []StrategyDecision {
	mid := snapshot.MidPrice
	if mid <= 0 {
		return nil
	}

	// Use ask as proxy high, bid as proxy low
	high := snapshot.Ask
	if high <= 0 {
		high = mid
	}
	low := snapshot.Bid
	if low <= 0 {
		low = mid
	}
	vol := snapshot.Volume24h
	if vol == 0 {
		vol = snapshot.OpenInterest
	}

	if len(s.highs) < s.lookback {
		s.highs = append(s.highs, high)
		s.lows = append(s.lows, low)
		s.volumes = append(s.volumes, vol)
		return nil
	}

	// Compute range from PREVIOUS period before adding current tick
	periodHigh := s.highs[0]
	periodLow := s.lows[0]
	sumVol := 0.0
	for i := 0; i < len(s.highs); i++ {
		if s.highs[i] > periodHigh {
			periodHigh = s.highs[i]
		}
		if s.lows[i] < periodLow {
			periodLow = s.lows[i]
		}
		sumVol += s.volumes[i]
	}

	avgVol := sumVol / float64(len(s.volumes))
	if avgVol <= 0 {
		avgVol = 1
	}

	// Update history with current tick (implement rolling window)
	s.highs = append(s.highs[1:], high)
	s.lows = append(s.lows[1:], low)
	s.volumes = append(s.volumes[1:], vol)

	// Check for volume surge
	volSurge := vol > avgVol*s.volumeSurgeMult

	// Check for breakout
	upsideBps := 0.0
	if periodHigh > 0 {
		upsideBps = (mid - periodHigh) / periodHigh * 10000.0
	}
	downsideBps := 0.0
	if periodLow > 0 {
		downsideBps = (periodLow - mid) / periodLow * 10000.0
	}

	if ctx == nil {
		ctx = &StrategyContext{}
	}
	var orders []StrategyDecision

	// Trailing stop for existing position
	if ctx.PositionQty != 0 {
		if ctx.PositionQty > 0 { // Long position
			if s.highWaterMark == 0 || mid > s.highWaterMark {
				s.highWaterMark = mid
			}
			stopPrice := s.highWaterMark * (1.0 - s.trailingStopBps/10000.0)
			if snapshot.Bid <= stopPrice {
				s.highWaterMark = 0 // Reset
				orders = append(orders, StrategyDecision{
					Action:     "place_order",
					Instrument: snapshot.Instrument,
					Side:       "sell",
					Size:       math.Abs(ctx.PositionQty),
					LimitPrice: math.Round(snapshot.Bid*100) / 100,
					OrderType:  "Ioc",
					Meta:       map[string]interface{}{"signal": "trailing_stop_long", "stop_price": math.Round(stopPrice*100) / 100},
				})
			}
		} else { // Short position
			if s.lowWaterMark == 0 || mid < s.lowWaterMark {
				s.lowWaterMark = mid
			}
			stopPrice := s.lowWaterMark * (1.0 + s.trailingStopBps/10000.0)
			if snapshot.Ask >= stopPrice {
				s.lowWaterMark = 0 // Reset
				orders = append(orders, StrategyDecision{
					Action:     "place_order",
					Instrument: snapshot.Instrument,
					Side:       "buy",
					Size:       math.Abs(ctx.PositionQty),
					LimitPrice: math.Round(snapshot.Ask*100) / 100,
					OrderType:  "Ioc",
					Meta:       map[string]interface{}{"signal": "trailing_stop_short", "stop_price": math.Round(stopPrice*100) / 100},
				})
			}
		}
		return orders
	}

	// No position: reset water marks
	s.highWaterMark = 0
	s.lowWaterMark = 0

	// Breakout entry (no position)
	if upsideBps > s.breakbackThresholdBps && volSurge {
		orders = append(orders, StrategyDecision{
			Action:     "place_order",
			Instrument: snapshot.Instrument,
			Side:       "buy",
			Size:       s.size,
			LimitPrice: math.Round(snapshot.Ask*100) / 100,
			OrderType:  "Ioc",
			Meta:       map[string]interface{}{"signal": "breakout_long", "breakout_bps": upsideBps, "volume_surge": true},
		})
	} else if downsideBps > s.breakbackThresholdBps && volSurge {
		orders = append(orders, StrategyDecision{
			Action:     "place_order",
			Instrument: snapshot.Instrument,
			Side:       "sell",
			Size:       s.size,
			LimitPrice: math.Round(snapshot.Bid*100) / 100,
			OrderType:  "Ioc",
			Meta:       map[string]interface{}{"signal": "breakout_short", "breakout_bps": downsideBps, "volume_surge": true},
		})
	}

	return orders
}
