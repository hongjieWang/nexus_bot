package main

import (
	"bot/config"
	"bot/database"
	"bot/strategy"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	futuresBase     = "https://fapi.binance.com"
	klineEndpoint   = "https://fapi.binance.com/fapi/v1/klines"
	tradingSymbol   = "BNBUSDT"
	defaultUSDT     = 50.0
	defaultLev      = 3
	defaultInterval = "15m"
	defaultMaxLoss  = 100.0
)

type Kline struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

type ActiveOrder struct {
	ID        int64
	BinanceID string
	Side      string // "BUY", "SELL"
	Type      string // "LIMIT", "MARKET", "STOP_MARKET", "TAKE_PROFIT_MARKET"
	Price     float64
	Qty       float64
	StopPrice float64
}

type RiskState struct {
	mu              sync.Mutex
	DailyLoss       float64
	DailyStart      time.Time
	ConsecutiveLoss int
	TotalTrades     int
	TotalPnL        float64
}

type TradingEngine struct {
	apiKey    string
	apiSecret string
	enabled   bool

	tradeUSDT        float64
	leverage         int
	interval         string
	maxDayLoss       float64
	simulatedBalance float64

	// ==== OMS (订单与持仓追踪) ====
	positionQty  float64 // >0=LONG, <0=SHORT
	entryPrice   float64
	activeOrders map[int64]*ActiveOrder
	orderSeq     int64
	globalSL     float64
	globalTP     float64
	// ===========================

	risk   RiskState
	klines []Kline
	strat  strategy.BaseStrategy
}

func startTradingEngine() {
	apiKey := config.GetConfig("BINANCE_API_KEY")
	apiSecret := config.GetConfig("BINANCE_API_SECRET")
	enabled := strings.ToLower(config.GetConfig("TRADE_ENABLED")) == "true"
	if apiKey == "" || apiSecret == "" {
		slog.Warn("🚫 API密钥未设置，降级为模拟模式")
		enabled = false
	}

	simBalance := parseEnvFloat("SIMULATED_BALANCE", 50.0)

	// 读取策略配置 (Phase 4.1)
	strats, err := LoadStrategies("strategies.json")
	if err != nil || len(strats) == 0 {
		slog.Warn("⚠️ 无法从 strategies.json 加载策略或配置为空，将使用默认内置策略", "err", err)
		strats = []strategy.BaseStrategy{
			strategy.NewEmaMacdRsiStrategy("ema_macd_rsi"),
			strategy.NewMomentumBreakoutStrategy("momentum_breakout", 20, 50.0, 2.0, 30.0, 0),
			strategy.NewSimpleMMStrategy("simple_mm", 20.0, 0.05),
			strategy.NewGridMMStrategy("grid_mm", 20.0, 3, 0.05, 0.15),
		}
	}

	// 实盘模式：通过 ACTIVE_STRATEGY 环境变量指定唯一运行的策略，防止多策略并行对冲/互撤挂单
	// 模拟模式下：所有策略并行运行用于比较验证
	if enabled {
		activeID := envOrDefault("ACTIVE_STRATEGY", "")
		if activeID == "" {
			// 未指定时取第一个策略并警告
			slog.Warn("⚠️ 实盘模式未设置 ACTIVE_STRATEGY，默认仅运行第一个策略以防对冲风险",
				"using", strats[0].ID(),
				"hint", "请在环境变量中设置 ACTIVE_STRATEGY=<策略ID>",
			)
			strats = strats[:1]
		} else {
			// 按 ID 过滤出指定策略
			var selected []strategy.BaseStrategy
			for _, s := range strats {
				if s.ID() == activeID {
					selected = append(selected, s)
					break
				}
			}
			if len(selected) == 0 {
				slog.Error("❌ ACTIVE_STRATEGY 指定的策略 ID 不存在，拒绝启动实盘", "id", activeID)
				return
			}
			strats = selected
			slog.Info("🎯 实盘策略已锁定", "strategy", activeID)
		}
	}

	slog.Info("🏁 启动策略引擎", "策略数量", len(strats), "实盘", enabled)

	var wg sync.WaitGroup
	for _, s := range strats {
		engine := &TradingEngine{
			apiKey:           apiKey,
			apiSecret:        apiSecret,
			enabled:          enabled,
			tradeUSDT:        parseEnvFloat("TRADE_USDT", defaultUSDT),
			leverage:         parseEnvInt("TRADE_LEVERAGE", defaultLev),
			interval:         envOrDefault("TRADE_INTERVAL", defaultInterval),
			maxDayLoss:       parseEnvFloat("MAX_DAILY_LOSS", defaultMaxLoss),
			simulatedBalance: simBalance,
			activeOrders:     make(map[int64]*ActiveOrder),
			strat:            s,
		}
		if engine.enabled {
			engine.setLeverage()
		}
		wg.Add(1)
		go func(e *TradingEngine) {
			defer wg.Done()
			e.syncPosition()
			e.run()
		}(engine)
	}
	wg.Wait()
}

func (e *TradingEngine) run() {
	waitForNextCandle(e.interval)
	ticker := newCandleTicker(e.interval)
	defer ticker.Stop()

	e.onCandle()
	for range ticker.C {
		e.onCandle()
	}
}

func (e *TradingEngine) onCandle() {
	if e.enabled {
		e.syncPosition()
	}

	klines, err := fetchKlines(tradingSymbol, e.interval, 200)
	if err != nil {
		return
	}
	e.klines = klines
	lastKline := klines[len(klines)-1]
	price := lastKline.Close

	if !e.enabled {
		// 模拟最新 K 线的 High / Low 击穿挂单
		e.simulateFills(lastKline.High, lastKline.Low)
	}

	ctx := &strategy.StrategyContext{
		PositionQty: e.positionQty,
		Meta: map[string]interface{}{
			"closes": closes(klines),
			"highs":  highs(klines),
			"lows":   lows(klines),
		},
	}

	snapshot := strategy.MarketSnapshot{
		Instrument: tradingSymbol,
		MidPrice:   price,
		// 修复：补充 Bid/Ask（用最新 K 线近似），供 grid_mm ReduceOnly 和 MomentumBreakout 使用
		// 实盘中 Bid/Ask 略偏于真实价格，此处用 close 作保守近似
		Bid:       price,
		Ask:       price,
		Volume24h: lastKline.Volume,
	}

	orders := e.strat.OnTick(snapshot, ctx)

	var atrVal float64
	if v, ok := ctx.Meta["atr"].(float64); ok {
		atrVal = v
	}

	slog.Info("📊 状态更新", "strategy", e.strat.ID(), "price", price, "posQty", e.positionQty, "openOrders", len(e.activeOrders))

	// 处理退出条件 (适用于未设置限价止盈止损单的单边策略)
	if e.positionQty != 0 && len(orders) == 0 {
		e.checkGlobalExitConditions(price, atrVal)
	}

	// 执行新的订单决策
	if len(orders) > 0 {
		e.executeDecisions(orders, price)
	}
}

func (e *TradingEngine) simulateFills(high, low float64) {
	for id, o := range e.activeOrders {
		filled := false
		fillPrice := o.Price

		if o.Type == "LIMIT" {
			if o.Side == "BUY" && low <= o.Price {
				filled = true
			} else if o.Side == "SELL" && high >= o.Price {
				filled = true
			}
		} else if o.Type == "STOP_MARKET" {
			if o.Side == "BUY" && high >= o.StopPrice {
				filled, fillPrice = true, o.StopPrice
			} else if o.Side == "SELL" && low <= o.StopPrice {
				filled, fillPrice = true, o.StopPrice
			}
		} else if o.Type == "TAKE_PROFIT_MARKET" {
			if o.Side == "BUY" && low <= o.StopPrice {
				filled, fillPrice = true, o.StopPrice
			} else if o.Side == "SELL" && high >= o.StopPrice {
				filled, fillPrice = true, o.StopPrice
			}
		}

		if filled {
			e.applyFill(o.Side, o.Qty, fillPrice)
			delete(e.activeOrders, id)
			slog.Info("🤝 [模拟] 挂单成交", "strategy", e.strat.ID(), "type", o.Type, "side", o.Side, "price", fillPrice, "qty", o.Qty)
		}
	}
}

func (e *TradingEngine) applyFill(side string, qty, execPrice float64) {
	if side == "SELL" {
		qty = -qty
	}
	oldQty := e.positionQty
	newQty := oldQty + qty

	// 如果有平仓逻辑，计算 PnL
	var pnl float64
	if (oldQty > 0 && qty < 0) || (oldQty < 0 && qty > 0) {
		closeQty := math.Min(math.Abs(oldQty), math.Abs(qty))
		if oldQty > 0 { // Was LONG, now closing
			pnl = (execPrice - e.entryPrice) * closeQty
		} else { // Was SHORT, now closing
			pnl = (e.entryPrice - execPrice) * closeQty
		}

		e.simulatedBalance += pnl
		database.LogTradeToDB(tradingSymbol, e.strat.ID(), side, closeQty, e.entryPrice, execPrice, pnl, e.simulatedBalance, time.Now(), time.Now(), true)
		e.recordResult(pnl)
		sendTradingAlert("close_fill", e.positionQty, e.entryPrice, closeQty, execPrice, pnl)
	}

	// 重新计算持仓均价
	if math.Abs(newQty) > 0.0001 {
		if math.Signbit(oldQty) == math.Signbit(newQty) && oldQty != 0 {
			// 加仓
			e.entryPrice = (e.entryPrice*math.Abs(oldQty) + execPrice*math.Abs(qty)) / math.Abs(newQty)
		} else if oldQty == 0 {
			// 新开仓
			e.entryPrice = execPrice
			sendTradingAlert("open", newQty, e.entryPrice, 0, 0, 0)
		} else {
			// 仓位反转
			e.entryPrice = execPrice
		}
	} else {
		e.entryPrice = 0
		newQty = 0
	}
	e.positionQty = newQty
}

func (e *TradingEngine) executeDecisions(decisions []strategy.StrategyDecision, currentPrice float64) {
	if !e.riskCheck() {
		return
	}

	// 网格策略每次返回最新的全量网格挂单，需要先撤销之前的所有未成交挂单
	// 但对于单边策略，如果是单纯发市价单，可能不需要全撤。
	// 这里做统一处理：只要策略发出了 place_order 的决定，并且有限价单，就全撤旧单
	hasLimit := false
	for _, d := range decisions {
		if d.OrderType == "Gtc" {
			hasLimit = true
		}
	}

	if hasLimit {
		if e.enabled {
			e.cancelAllOrders()
		} else {
			e.activeOrders = make(map[int64]*ActiveOrder)
		}
	}

	for _, d := range decisions {
		if d.Action != "place_order" {
			continue
		}

		qty := d.Size
		if qty <= 0 {
			qty = roundQty(e.tradeUSDT * float64(e.leverage) / currentPrice)
		}
		if qty <= 0 {
			continue
		}

		side := strings.ToUpper(d.Side)
		if d.OrderType == "Market" {
			// 市价单立刻执行
			if e.enabled {
				e.placeOrder(side, "MARKET", qty, 0, 0)
			} else {
				e.applyFill(side, qty, currentPrice)
			}

			// 处理策略附带的止盈止损
			sl, _ := d.Meta["sl"].(float64)
			tp, _ := d.Meta["tp"].(float64)
			if sl > 0 {
				e.globalSL = sl
				if e.enabled {
					closeSide := "SELL"
					if side == "SELL" {
						closeSide = "BUY"
					}
					e.placeOrder(closeSide, "STOP_MARKET", qty, 0, sl)
				}
			}
			if tp > 0 {
				e.globalTP = tp
				if e.enabled {
					closeSide := "SELL"
					if side == "SELL" {
						closeSide = "BUY"
					}
					e.placeOrder(closeSide, "TAKE_PROFIT_MARKET", qty, 0, tp)
				}
			}

			// 单边策略一次只下一单，防止并发多单
			break

		} else if d.OrderType == "Gtc" {
			// 限价单放入 OMS 追踪
			if e.enabled {
				e.placeOrder(side, "LIMIT", qty, d.LimitPrice, 0)
			} else {
				e.orderSeq++
				e.activeOrders[e.orderSeq] = &ActiveOrder{
					ID: e.orderSeq, Side: side, Type: "LIMIT", Price: d.LimitPrice, Qty: qty,
				}
			}
		}
	}
}

func (e *TradingEngine) checkGlobalExitConditions(price, atr float64) {
	hit, reason := false, ""
	if e.positionQty > 0 {
		if e.globalSL > 0 && price <= e.globalSL {
			hit, reason = true, "止损"
		} else if e.globalTP > 0 && price >= e.globalTP {
			hit, reason = true, "止盈"
		}
	} else if e.positionQty < 0 {
		if e.globalSL > 0 && price >= e.globalSL {
			hit, reason = true, "止损"
		} else if e.globalTP > 0 && price <= e.globalTP {
			hit, reason = true, "止盈"
		}
	}

	if !hit {
		return
	}
	slog.Info("📌 触发全局平仓条件", "strategy", e.strat.ID(), "reason", reason)

	qty := math.Abs(e.positionQty)
	closeSide := "SELL"
	if e.positionQty < 0 {
		closeSide = "BUY"
	}

	if e.enabled {
		e.cancelAllOrders()
		e.placeOrder(closeSide, "MARKET", qty, 0, 0)
	} else {
		// 关键修复：模拟模式下平仓后也需清除所有本地活跃挂单 (Severe #5)
		e.activeOrders = make(map[int64]*ActiveOrder)
		e.applyFill(closeSide, qty, price)
	}
	e.globalSL, e.globalTP = 0, 0
}

func (e *TradingEngine) riskCheck() bool {
	e.risk.mu.Lock()
	defer e.risk.mu.Unlock()
	if time.Since(e.risk.DailyStart) > 24*time.Hour {
		e.risk.DailyLoss = 0
		e.risk.DailyStart = time.Now()
		e.risk.ConsecutiveLoss = 0
	}
	if e.risk.DailyLoss >= e.maxDayLoss {
		return false
	}
	if e.risk.ConsecutiveLoss >= 3 {
		return false
	}
	return true
}

func (e *TradingEngine) recordResult(pnl float64) {
	e.risk.mu.Lock()
	defer e.risk.mu.Unlock()
	e.risk.TotalPnL += pnl
	e.risk.TotalTrades++
	if pnl < 0 {
		e.risk.DailyLoss += math.Abs(pnl)
		e.risk.ConsecutiveLoss++
	} else {
		e.risk.ConsecutiveLoss = 0
	}
}

// ================== 币安合约 API (One-Way Mode / BOTH PositionSide) ==================

func (e *TradingEngine) placeOrder(side, orderType string, qty, price, stopPrice float64) error {
	params := url.Values{}
	params.Set("symbol", tradingSymbol)
	params.Set("side", side)
	params.Set("positionSide", "BOTH") // 为了适配网格与多方向，统一使用单向模式 (One-Way)
	params.Set("type", orderType)
	params.Set("quantity", formatQty(qty))

	if orderType == "LIMIT" {
		params.Set("timeInForce", "GTC")
		params.Set("price", fmt.Sprintf("%.4f", price))
	} else if orderType == "STOP_MARKET" || orderType == "TAKE_PROFIT_MARKET" {
		params.Set("stopPrice", fmt.Sprintf("%.4f", stopPrice))
		params.Set("closePosition", "true")
	}

	params.Set("timestamp", nowMS())
	params.Set("signature", e.sign(params.Encode()))
	_, err := e.futuresRequest("POST", "/fapi/v1/order", params)
	return err
}

func (e *TradingEngine) cancelAllOrders() error {
	params := url.Values{}
	params.Set("symbol", tradingSymbol)
	params.Set("timestamp", nowMS())
	params.Set("signature", e.sign(params.Encode()))
	_, err := e.futuresRequest("DELETE", "/fapi/v1/allOpenOrders", params)
	return err
}

func (e *TradingEngine) setLeverage() error {
	params := url.Values{}
	params.Set("symbol", tradingSymbol)
	params.Set("leverage", strconv.Itoa(e.leverage))
	params.Set("timestamp", nowMS())
	params.Set("signature", e.sign(params.Encode()))
	_, err := e.futuresRequest("POST", "/fapi/v1/leverage", params)
	return err
}

func (e *TradingEngine) syncPosition() {
	params := url.Values{}
	params.Set("symbol", tradingSymbol)
	params.Set("timestamp", nowMS())
	params.Set("signature", e.sign(params.Encode()))

	resp, err := e.futuresRequest("GET", "/fapi/v2/positionRisk", params)
	if err != nil {
		return
	}

	var positions []struct {
		PositionAmt string `json:"positionAmt"`
		EntryPrice  string `json:"entryPrice"`
	}
	if err := json.Unmarshal(resp, &positions); err != nil {
		return
	}

	netQty := 0.0
	totalNotional := 0.0 // 加权计算均价：∑(qty × entryPrice)

	for _, p := range positions {
		qty, _ := strconv.ParseFloat(p.PositionAmt, 64)
		if math.Abs(qty) > 0.001 {
			entry, _ := strconv.ParseFloat(p.EntryPrice, 64)
			netQty += qty
			totalNotional += math.Abs(qty) * entry
		}
	}

	if math.Abs(netQty) > 0.001 {
		e.positionQty = netQty
		// 加权均价：总名义价值 / 总持仓量（绝对值）
		e.entryPrice = totalNotional / math.Abs(netQty)
	} else {
		e.positionQty = 0
		e.entryPrice = 0
	}
}

func (e *TradingEngine) futuresRequest(method, path string, params url.Values) ([]byte, error) {
	var reqURL string
	var body io.Reader
	if method == "GET" || method == "DELETE" {
		reqURL = futuresBase + path + "?" + params.Encode()
	} else {
		reqURL = futuresBase + path
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequestWithContext(context.Background(), method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", e.apiKey)
	if method != "GET" && method != "DELETE" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API err: %s", string(data))
	}
	return data, nil
}

func (e *TradingEngine) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(e.apiSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func fetchKlines(symbol, interval string, limit int) ([]Kline, error) {
	u := fmt.Sprintf("%s?symbol=%s&interval=%s&limit=%d", klineEndpoint, symbol, interval, limit)
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var klines []Kline
	for _, r := range raw {
		klines = append(klines, Kline{
			OpenTime:  int64(r[0].(float64)),
			Open:      parseF(r[1]),
			High:      parseF(r[2]),
			Low:       parseF(r[3]),
			Close:     parseF(r[4]),
			Volume:    parseF(r[5]), // 修复：r[5] 为成交量，之前未赋值导致 MomentumBreakout 的 volSurge 永远为 false
			CloseTime: int64(parseF(r[6])),
		})
	}
	return klines, nil
}

func sendTradingAlert(action string, posQty, entry, fillQty, fillPrice, pnl float64) {
	if discordWebhook == "" {
		return
	}
	var title string
	if action == "open" {
		title = fmt.Sprintf("🔵 开仓/加仓 BNB (数量: %.3f)", posQty)
	} else {
		title = fmt.Sprintf("⚪ 模拟平仓/减仓 BNB (盈利: $%.2f)", pnl)
	}

	embed := discordEmbed{
		Title: title, Color: 0x00C851,
		Fields: []discordEmbedField{
			{Name: "净持仓", Value: fmt.Sprintf("%.3f", posQty), Inline: true},
			{Name: "成交价", Value: fmt.Sprintf("$%.4f", fillPrice), Inline: true},
		},
	}
	body, _ := json.Marshal(discordPayload{Username: "Grid Bot", Embeds: []discordEmbed{embed}})
	httpClient.Post(discordWebhook, "application/json", strings.NewReader(string(body)))
}

func closes(klines []Kline) []float64 {
	r := make([]float64, len(klines))
	for i, k := range klines {
		r[i] = k.Close
	}
	return r
}
func highs(klines []Kline) []float64 {
	r := make([]float64, len(klines))
	for i, k := range klines {
		r[i] = k.High
	}
	return r
}
func lows(klines []Kline) []float64 {
	r := make([]float64, len(klines))
	for i, k := range klines {
		r[i] = k.Low
	}
	return r
}
func parseF(v interface{}) float64 {
	if s, ok := v.(string); ok {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}
func roundQty(qty float64) float64 { return math.Floor(qty*1000) / 1000 }
func formatQty(qty float64) string { return strconv.FormatFloat(qty, 'f', 3, 64) }
func nowMS() string                { return strconv.FormatInt(time.Now().UnixMilli(), 10) }
func parseEnvFloat(key string, def float64) float64 {
	if v := config.GetConfig(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
func parseEnvInt(key string, def int) int {
	if v := config.GetConfig(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
func envOrDefault(key, def string) string {
	if v := config.GetConfig(key); v != "" {
		return v
	}
	return def
}
func waitForNextCandle(interval string) {
	d := intervalDuration(interval)
	next := time.Now().Truncate(d).Add(d)
	if wait := time.Until(next) + 2*time.Second; wait > 0 {
		time.Sleep(wait)
	}
}
func newCandleTicker(interval string) *time.Ticker { return time.NewTicker(intervalDuration(interval)) }
func intervalDuration(interval string) time.Duration {
	mapping := map[string]time.Duration{"1m": time.Minute, "3m": 3 * time.Minute, "5m": 5 * time.Minute, "15m": 15 * time.Minute, "30m": 30 * time.Minute, "1h": time.Hour, "4h": 4 * time.Hour, "1d": 24 * time.Hour}
	if d, ok := mapping[interval]; ok {
		return d
	}
	return 15 * time.Minute
}
