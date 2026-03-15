package strategy

import "math"

// GridMMStrategy — Grid market maker: fixed-interval grid levels above and below mid.
type GridMMStrategy struct {
	strategyID     string
	gridSpacingBps float64
	numLevels      int
	sizePerLevel   float64
	maxPosition    float64
}

func NewGridMMStrategy(id string, gridSpacingBps float64, numLevels int, sizePerLevel, maxPosition float64) *GridMMStrategy {
	if id == "" {
		id = "grid_mm"
	}
	return &GridMMStrategy{
		strategyID:     id,
		gridSpacingBps: gridSpacingBps,
		numLevels:      numLevels,
		sizePerLevel:   sizePerLevel,
		maxPosition:    maxPosition,
	}
}

func (s *GridMMStrategy) ID() string {
	return s.strategyID
}

func (s *GridMMStrategy) OnTick(snapshot MarketSnapshot, ctx *StrategyContext) []StrategyDecision {
	mid := snapshot.MidPrice
	if mid <= 0 {
		return nil
	}

	if ctx == nil {
		ctx = &StrategyContext{}
	}

	var orders []StrategyDecision

	// Reduce only — close position, don't open new grid
	if ctx.ReduceOnly && ctx.PositionQty != 0 {
		closeSide := "buy"
		closePrice := snapshot.Ask
		if ctx.PositionQty > 0 {
			closeSide = "sell"
			closePrice = snapshot.Bid
		}

		orders = append(orders, StrategyDecision{
			Action:     "place_order",
			Instrument: snapshot.Instrument,
			Side:       closeSide,
			Size:       math.Abs(ctx.PositionQty),
			LimitPrice: math.Round(closePrice*100) / 100,
			OrderType:  "Gtc",
			Meta:       map[string]interface{}{"signal": "reduce_only_close"},
		})
		return orders
	}

	spacing := mid * s.gridSpacingBps / 10000.0

	for i := 1; i <= s.numLevels; i++ {
		fi := float64(i)
		// Bid levels below mid
		bidPrice := mid - spacing*fi
		// Ask levels above mid
		askPrice := mid + spacing*fi

		// Respect max position
		if ctx.PositionQty+s.sizePerLevel <= s.maxPosition {
			orders = append(orders, StrategyDecision{
				Action:     "place_order",
				Instrument: snapshot.Instrument,
				Side:       "buy",
				Size:       s.sizePerLevel,
				LimitPrice: math.Round(bidPrice*100) / 100,
				OrderType:  "Gtc",
				Meta:       map[string]interface{}{"signal": "grid_bid", "level": i},
			})
		}

		if ctx.PositionQty-s.sizePerLevel >= -s.maxPosition {
			orders = append(orders, StrategyDecision{
				Action:     "place_order",
				Instrument: snapshot.Instrument,
				Side:       "sell",
				Size:       s.sizePerLevel,
				LimitPrice: math.Round(askPrice*100) / 100,
				OrderType:  "Gtc",
				Meta:       map[string]interface{}{"signal": "grid_ask", "level": i},
			})
		}
	}

	return orders
}
