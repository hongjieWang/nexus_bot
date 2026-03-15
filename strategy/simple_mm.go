package strategy

import "math"

// SimpleMMStrategy is a simple market-making strategy — symmetric bid/ask quoting around mid.
type SimpleMMStrategy struct {
	strategyID string
	spreadBps  float64
	size       float64
}

func NewSimpleMMStrategy(id string, spreadBps float64, size float64) *SimpleMMStrategy {
	if id == "" {
		id = "simple_mm"
	}
	return &SimpleMMStrategy{
		strategyID: id,
		spreadBps:  spreadBps,
		size:       size,
	}
}

func (s *SimpleMMStrategy) ID() string {
	return s.strategyID
}

func (s *SimpleMMStrategy) OnTick(snapshot MarketSnapshot, ctx *StrategyContext) []StrategyDecision {
	if snapshot.MidPrice <= 0 {
		return nil
	}

	halfSpread := snapshot.MidPrice * (s.spreadBps / 10000.0) / 2.0
	bid := math.Round((snapshot.MidPrice-halfSpread)*100) / 100
	ask := math.Round((snapshot.MidPrice+halfSpread)*100) / 100

	return []StrategyDecision{
		{
			Action:     "place_order",
			Instrument: snapshot.Instrument,
			Side:       "buy",
			Size:       s.size,
			LimitPrice: bid,
			OrderType:  "Gtc",
		},
		{
			Action:     "place_order",
			Instrument: snapshot.Instrument,
			Side:       "sell",
			Size:       s.size,
			LimitPrice: ask,
			OrderType:  "Gtc",
		},
	}
}
