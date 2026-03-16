package main

import (
	"math"
	"testing"
	"time"

	"bot/strategy"
)

func TestApplyFillTracksPositionLifecycle(t *testing.T) {
	engine := &TradingEngine{
		runID:            "test-run",
		accountID:        "simulated:test",
		simulatedBalance: 50,
		strat:            strategy.NewSimpleMMStrategy("test_mm", 20, 0.05),
	}

	engine.applyFill("BUY", 1, 100, "open")
	if engine.positionQty != 1 {
		t.Fatalf("expected long position, got %v", engine.positionQty)
	}
	if engine.entryPrice != 100 {
		t.Fatalf("expected entry price 100, got %v", engine.entryPrice)
	}
	if engine.positionOpenedAt.IsZero() {
		t.Fatal("expected non-zero opened_at after opening a position")
	}

	openedAt := engine.positionOpenedAt
	engine.applyFill("SELL", 0.4, 101, "reduce")
	if !engine.positionOpenedAt.Equal(openedAt) {
		t.Fatal("expected opened_at to remain unchanged after partial close")
	}

	engine.applyFill("SELL", 0.6, 102, "close")
	if engine.positionQty != 0 {
		t.Fatalf("expected flat position after full close, got %v", engine.positionQty)
	}
	if !engine.positionOpenedAt.IsZero() {
		t.Fatal("expected opened_at to reset after full close")
	}
	if math.Abs(engine.simulatedBalance-51.6) > 1e-9 {
		t.Fatalf("expected balance 51.6, got %.10f", engine.simulatedBalance)
	}
}

func TestApplyFillReverseResetsPositionClock(t *testing.T) {
	engine := &TradingEngine{
		runID:            "test-run",
		accountID:        "simulated:test",
		simulatedBalance: 50,
		strat:            strategy.NewSimpleMMStrategy("test_mm", 20, 0.05),
	}

	engine.applyFill("BUY", 1, 100, "open")
	engine.positionOpenedAt = time.Now().Add(-time.Minute)
	previousOpenedAt := engine.positionOpenedAt

	engine.applyFill("SELL", 2, 98, "reverse")
	if engine.positionQty != -1 {
		t.Fatalf("expected short position after reverse, got %v", engine.positionQty)
	}
	if engine.entryPrice != 98 {
		t.Fatalf("expected reversed entry price 98, got %v", engine.entryPrice)
	}
	if !engine.positionOpenedAt.After(previousOpenedAt) {
		t.Fatal("expected reverse to reset opened_at for the new position")
	}
}
