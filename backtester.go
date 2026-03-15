package main

import (
	"bot/strategy"
	"fmt"
	"log/slog"
	"math"
)

// BacktestResult 存放回测计算的核心指标
type BacktestResult struct {
	StrategyID     string
	TotalTrades    int
	WinningTrades  int
	LosingTrades   int
	WinRate        float64
	FinalBalance   float64
	TotalPnL       float64
	MaxDrawdown    float64
	MaxDrawdownPct float64
}

// RunBacktest 执行单策略的离线回测 (Phase 4.2)
func RunBacktest(strat strategy.BaseStrategy, symbol, interval string, limit int, initialBalance float64) {
	slog.Info("⏳ 开始回测...", "strategy", strat.ID(), "symbol", symbol, "interval", interval, "limit", limit)

	// 1. 获取历史数据
	klines, err := fetchKlines(symbol, interval, limit)
	if err != nil {
		slog.Error("回测获取 K 线失败", "err", err)
		return
	}

	if len(klines) < 100 {
		slog.Error("数据量过少，无法进行回测", "count", len(klines))
		return
	}

	// 2. 初始化模拟引擎状态
	balance := initialBalance
	peakBalance := initialBalance
	maxDrawdown := 0.0

	positionQty := 0.0
	entryPrice := 0.0

	totalTrades := 0
	winningTrades := 0
	losingTrades := 0

	// 用于回测的 active limit orders
	activeOrders := make(map[int64]*ActiveOrder)
	var orderSeq int64 = 0

	// 3. 逐根 K 线回放 (需要保留足够的前置数据供指标计算，比如预留 50 根)
	startIdx := 50
	for i := startIdx; i < len(klines); i++ {
		// 当前的可见历史 K 线切片
		history := klines[:i+1]
		currentKline := klines[i]

		// 模拟挂单撮合 (假设在当前 K 线的 High/Low 之间都能撮合成交)
		// 如果有挂单，先判断是否触发
		for id, o := range activeOrders {
			filled := false
			fillPrice := o.Price

			if o.Type == "LIMIT" {
				if o.Side == "BUY" && currentKline.Low <= o.Price {
					filled = true
				} else if o.Side == "SELL" && currentKline.High >= o.Price {
					filled = true
				}
			} else if o.Type == "STOP_MARKET" {
				if o.Side == "BUY" && currentKline.High >= o.StopPrice {
					filled, fillPrice = true, o.StopPrice
				} else if o.Side == "SELL" && currentKline.Low <= o.StopPrice {
					filled, fillPrice = true, o.StopPrice
				}
			} else if o.Type == "TAKE_PROFIT_MARKET" {
				if o.Side == "BUY" && currentKline.Low <= o.StopPrice {
					filled, fillPrice = true, o.StopPrice
				} else if o.Side == "SELL" && currentKline.High >= o.StopPrice {
					filled, fillPrice = true, o.StopPrice
				}
			}

			if filled {
				// 成交逻辑
				qty := o.Qty
				if o.Side == "SELL" {
					qty = -qty
				}

				// 计算平仓 PnL
				if (positionQty > 0 && qty < 0) || (positionQty < 0 && qty > 0) {
					closeQty := math.Min(math.Abs(positionQty), math.Abs(qty))
					var pnl float64
					if positionQty > 0 {
						pnl = (fillPrice - entryPrice) / entryPrice * (closeQty * entryPrice)
					} else {
						pnl = (entryPrice - fillPrice) / entryPrice * (closeQty * entryPrice)
					}

					balance += pnl
					totalTrades++
					if pnl > 0 {
						winningTrades++
					} else {
						losingTrades++
					}

					// 更新资金曲线峰值和回撤
					if balance > peakBalance {
						peakBalance = balance
					}
					drawdown := peakBalance - balance
					if drawdown > maxDrawdown {
						maxDrawdown = drawdown
					}
				}

				// 更新持仓
				newQty := positionQty + qty
				if math.Abs(newQty) > 0.0001 {
					if math.Signbit(positionQty) == math.Signbit(newQty) && positionQty != 0 {
						entryPrice = (entryPrice*math.Abs(positionQty) + fillPrice*math.Abs(qty)) / math.Abs(newQty)
					} else if positionQty == 0 {
						entryPrice = fillPrice
					} else {
						entryPrice = fillPrice
					}
				} else {
					entryPrice = 0
					newQty = 0
				}
				positionQty = newQty

				delete(activeOrders, id)
			}
		}

		// 构建上下文并调用策略
		ctx := &strategy.StrategyContext{
			PositionQty: positionQty,
			Meta: map[string]interface{}{
				"closes": closes(history),
				"highs":  highs(history),
				"lows":   lows(history),
			},
		}

		snapshot := strategy.MarketSnapshot{
			Instrument: symbol,
			MidPrice:   currentKline.Close, // Tick 驱动以收盘价为准
		}

		orders := strat.OnTick(snapshot, ctx)

		// 简单的订单处理
		hasLimit := false
		for _, o := range orders {
			if o.OrderType == "Gtc" {
				hasLimit = true
			}
		}

		if hasLimit {
			// 清理旧限价单
			activeOrders = make(map[int64]*ActiveOrder)
		}

		for _, o := range orders {
			if o.Action == "place_order" {
				qty := o.Size
				if qty <= 0 {
					qty = roundQty(50.0 * 3.0 / currentKline.Close) // 模拟默认开仓 50U 3倍
				}
				if qty <= 0 {
					continue
				}

				if o.OrderType == "Market" {
					// 假设立即以收盘价成交
					// (与上面的成交逻辑重复，这里为简化回测直接套用)
					fillPrice := currentKline.Close
					sideQty := qty
					if o.Side == "sell" || o.Side == "SELL" {
						sideQty = -qty
					}

					if (positionQty > 0 && sideQty < 0) || (positionQty < 0 && sideQty > 0) {
						closeQty := math.Min(math.Abs(positionQty), math.Abs(sideQty))
						var pnl float64
						if positionQty > 0 {
							pnl = (fillPrice - entryPrice) / entryPrice * (closeQty * entryPrice)
						} else {
							pnl = (entryPrice - fillPrice) / entryPrice * (closeQty * entryPrice)
						}

						balance += pnl
						totalTrades++
						if pnl > 0 {
							winningTrades++
						} else {
							losingTrades++
						}

						if balance > peakBalance {
							peakBalance = balance
						}
						drawdown := peakBalance - balance
						if drawdown > maxDrawdown {
							maxDrawdown = drawdown
						}
					}

					newQty := positionQty + sideQty
					if math.Abs(newQty) > 0.0001 {
						if math.Signbit(positionQty) == math.Signbit(newQty) && positionQty != 0 {
							entryPrice = (entryPrice*math.Abs(positionQty) + fillPrice*math.Abs(qty)) / math.Abs(newQty)
						} else {
							entryPrice = fillPrice
						}
					} else {
						entryPrice = 0
						newQty = 0
					}
					positionQty = newQty

					// 止损止盈
					sl, _ := o.Meta["sl"].(float64)
					tp, _ := o.Meta["tp"].(float64)
					if sl > 0 {
						orderSeq++
						closeSide := "SELL"
						if o.Side == "sell" || o.Side == "SELL" {
							closeSide = "BUY"
						}
						activeOrders[orderSeq] = &ActiveOrder{ID: orderSeq, Side: closeSide, Type: "STOP_MARKET", StopPrice: sl, Qty: qty}
					}
					if tp > 0 {
						orderSeq++
						closeSide := "SELL"
						if o.Side == "sell" || o.Side == "SELL" {
							closeSide = "BUY"
						}
						activeOrders[orderSeq] = &ActiveOrder{ID: orderSeq, Side: closeSide, Type: "TAKE_PROFIT_MARKET", StopPrice: tp, Qty: qty}
					}

				} else if o.OrderType == "Gtc" {
					orderSeq++
					side := "BUY"
					if o.Side == "sell" || o.Side == "SELL" {
						side = "SELL"
					}
					activeOrders[orderSeq] = &ActiveOrder{ID: orderSeq, Side: side, Type: "LIMIT", Price: o.LimitPrice, Qty: qty}
				}
			}
		}
	}

	// 强制平仓收尾
	if positionQty != 0 {
		fillPrice := klines[len(klines)-1].Close
		var pnl float64
		if positionQty > 0 {
			pnl = (fillPrice - entryPrice) / entryPrice * (positionQty * entryPrice)
		} else {
			pnl = (entryPrice - fillPrice) / entryPrice * (math.Abs(positionQty) * entryPrice)
		}
		balance += pnl
		totalTrades++
		if pnl > 0 {
			winningTrades++
		} else {
			losingTrades++
		}
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100.0
	}

	maxDrawdownPct := 0.0
	if peakBalance > 0 {
		maxDrawdownPct = maxDrawdown / peakBalance * 100.0
	}

	res := BacktestResult{
		StrategyID:     strat.ID(),
		TotalTrades:    totalTrades,
		WinningTrades:  winningTrades,
		LosingTrades:   losingTrades,
		WinRate:        winRate,
		FinalBalance:   balance,
		TotalPnL:       balance - initialBalance,
		MaxDrawdown:    maxDrawdown,
		MaxDrawdownPct: maxDrawdownPct,
	}

	slog.Info("📈 回测结果",
		"strategy", res.StrategyID,
		"pnl", fmt.Sprintf("$%.2f", res.TotalPnL),
		"win_rate", fmt.Sprintf("%.2f%%", res.WinRate),
		"trades", res.TotalTrades,
		"max_drawdown", fmt.Sprintf("%.2f%% ($%.2f)", res.MaxDrawdownPct, res.MaxDrawdown),
	)
}
