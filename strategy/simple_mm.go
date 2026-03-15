package strategy

import "math"

// SimpleMMStrategy — 对称做市策略，含库存偏斜与最大持仓保护。
// 当净持仓偏多时，上移报价中心（抬高 ask 减少卖出意愿，下移 bid 减少买入意愿），反之亦然。
type SimpleMMStrategy struct {
	strategyID  string
	spreadBps   float64
	size        float64
	maxPosition float64 // 最大净持仓（绝对值），超限后停止单边报价
	skewFactor  float64 // 库存偏斜力度：每持有 1 单位多仓时中间价向上偏移的 bps
}

func NewSimpleMMStrategy(id string, spreadBps float64, size float64) *SimpleMMStrategy {
	if id == "" {
		id = "simple_mm"
	}
	return &SimpleMMStrategy{
		strategyID:  id,
		spreadBps:   spreadBps,
		size:        size,
		maxPosition: size * 5,        // 默认最大持仓为单次下单量的 5 倍
		skewFactor:  spreadBps * 0.3, // 偏斜幅度为半价差的 30%
	}
}

func (s *SimpleMMStrategy) ID() string {
	return s.strategyID
}

func (s *SimpleMMStrategy) OnTick(snapshot MarketSnapshot, ctx *StrategyContext) []StrategyDecision {
	if snapshot.MidPrice <= 0 {
		return nil
	}

	posQty := 0.0
	if ctx != nil {
		posQty = ctx.PositionQty
	}

	// 库存偏斜：净多仓时中间价上移（降低继续买入的吸引力），净空仓时下移
	// skewBps = posQty / size × skewFactor（以持仓数量归一化）
	skewBps := 0.0
	if s.size > 0 {
		skewBps = (posQty / s.size) * s.skewFactor
	}
	skewedMid := snapshot.MidPrice * (1.0 + skewBps/10000.0)

	halfSpread := snapshot.MidPrice * (s.spreadBps / 10000.0) / 2.0
	bid := math.Round((skewedMid-halfSpread)*100) / 100
	ask := math.Round((skewedMid+halfSpread)*100) / 100

	var orders []StrategyDecision

	// 最大持仓保护：超限后停止加仓方向的报价
	if posQty <= s.maxPosition { // 可以继续买入
		orders = append(orders, StrategyDecision{
			Action:     "place_order",
			Instrument: snapshot.Instrument,
			Side:       "buy",
			Size:       s.size,
			LimitPrice: bid,
			OrderType:  "Gtc",
			Meta:       map[string]interface{}{"skew_bps": skewBps},
		})
	}

	if posQty >= -s.maxPosition { // 可以继续卖出
		orders = append(orders, StrategyDecision{
			Action:     "place_order",
			Instrument: snapshot.Instrument,
			Side:       "sell",
			Size:       s.size,
			LimitPrice: ask,
			OrderType:  "Gtc",
			Meta:       map[string]interface{}{"skew_bps": skewBps},
		})
	}

	return orders
}
