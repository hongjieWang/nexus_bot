package strategy

import "testing"

func TestSimpleMMReducesInventoryDuringTrendStandDown(t *testing.T) {
	strat := NewSimpleMMStrategy("simple_mm_test", 20, 0.05)
	ctx := &StrategyContext{
		PositionQty: -0.10,
		Meta:        trendMeta(60, 100, 2),
	}
	snapshot := MarketSnapshot{
		Instrument: "BNBUSDT",
		MidPrice:   218,
	}

	orders := strat.OnTick(snapshot, ctx)
	if len(orders) != 1 {
		t.Fatalf("expected one reduce-only order, got %d", len(orders))
	}
	if orders[0].Side != "buy" {
		t.Fatalf("expected buy order to cover short inventory, got %s", orders[0].Side)
	}
	if orders[0].Meta["signal"] != "mm_reduce_only" {
		t.Fatalf("expected mm_reduce_only signal, got %v", orders[0].Meta["signal"])
	}
}

func TestGridMMSkipsQuotingDuringTrendWhenFlat(t *testing.T) {
	strat := NewGridMMStrategy("grid_mm_test", 20, 3, 0.05, 0.15)
	ctx := &StrategyContext{
		Meta: trendMeta(60, 100, 2),
	}
	snapshot := MarketSnapshot{
		Instrument: "BNBUSDT",
		MidPrice:   218,
	}

	orders := strat.OnTick(snapshot, ctx)
	if len(orders) != 0 {
		t.Fatalf("expected no grid orders during trend stand-down, got %d", len(orders))
	}
}

func TestGridMMStillQuotesInRange(t *testing.T) {
	strat := NewGridMMStrategy("grid_mm_test", 20, 2, 0.05, 0.15)
	ctx := &StrategyContext{
		Meta: rangeMeta(60, 100),
	}
	snapshot := MarketSnapshot{
		Instrument: "BNBUSDT",
		MidPrice:   100,
	}

	orders := strat.OnTick(snapshot, ctx)
	if len(orders) == 0 {
		t.Fatal("expected grid orders in range regime")
	}
}

func trendMeta(length int, start, step float64) map[string]interface{} {
	closes := make([]float64, length)
	highs := make([]float64, length)
	lows := make([]float64, length)
	for i := 0; i < length; i++ {
		price := start + float64(i)*step
		closes[i] = price
		highs[i] = price + 1
		lows[i] = price - 1
	}
	return map[string]interface{}{
		"closes": closes,
		"highs":  highs,
		"lows":   lows,
	}
}

func rangeMeta(length int, price float64) map[string]interface{} {
	closes := make([]float64, length)
	highs := make([]float64, length)
	lows := make([]float64, length)
	for i := 0; i < length; i++ {
		closes[i] = price
		highs[i] = price + 0.2
		lows[i] = price - 0.2
	}
	return map[string]interface{}{
		"closes": closes,
		"highs":  highs,
		"lows":   lows,
	}
}
