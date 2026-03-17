package alpha

import (
	"bot/config"
	"bot/database"
	"log/slog"
	"math"
	"strconv"
	"time"
)

// evaluateSmartWallets 对数据库中的聪明钱进行战绩回测、打分和过滤
func (e *AlphaEngine) evaluateSmartWallets() {
	slog.Info("🔍 [Alpha Engine] 开始执行聪明钱深度分析...")

	var wallets []database.SmartWallet
	if err := database.DB.Find(&wallets).Error; err != nil {
		slog.Error("Alpha Engine 获取钱包列表失败", "err", err)
		return
	}

	for i := range wallets {
		w := &wallets[i]

		var trades []database.SmartWalletTrade
		database.DB.Where("wallet = ?", w.Address).Find(&trades)

		w.TotalTrades = len(trades)

		thresholdMEV := 50
		if v, err := strconv.Atoi(config.GetConfig("ALPHA_MEV_THRESHOLD")); err == nil && v > 0 {
			thresholdMEV = v
		}

		if w.TotalTrades > thresholdMEV {
			var recentTrades int64
			database.DB.Model(&database.SmartWalletTrade{}).
				Where("wallet = ? AND timestamp > ?", w.Address, time.Now().Add(-24*time.Hour)).
				Count(&recentTrades)

			if recentTrades > int64(thresholdMEV) {
				w.IsMEV = true
				w.Score = 0
				slog.Info("🤖 [Alpha Engine] 标记 MEV/套利机器人", "wallet", w.Address, "24h_trades", recentTrades)
			}
		}

		if !w.IsMEV {
			winRate, avgROI, avgEntryBlocks := resolveWalletROI(w.Address)

			w.WinRate = winRate
			w.ROI = avgROI
			w.AvgEntryBlocks = avgEntryBlocks

			var lastTrades []database.SmartWalletTrade
			if err := database.DB.Where("wallet = ?", w.Address).
				Order("timestamp DESC").Limit(1).Find(&lastTrades).Error; err == nil && len(lastTrades) > 0 {
				w.LastActiveAt = &lastTrades[0].Timestamp
			}

			if w.LastActiveAt == nil || w.LastActiveAt.IsZero() {
				if !w.CreatedAt.IsZero() {
					w.LastActiveAt = &w.CreatedAt
				} else {
					now := time.Now()
					w.LastActiveAt = &now
				}
			}

			var closedCount int64
			database.DB.Model(&database.SmartWalletTrade{}).
				Where("wallet = ? AND action = 'SELL'", w.Address).
				Count(&closedCount)

			if closedCount >= 3 {
				w.Score = calcScore(w)
			} else {
				w.Score = 30.0 + float64(w.TotalTrades)*0.5
				w.Score = math.Min(w.Score, 49.0)
			}
		}

		database.DB.Save(w)
	}

	e.detectClusters()
	e.aggregateEntities()

	slog.Info("✅ [Alpha Engine] 聪明钱深度分析完毕", "total_wallets", len(wallets))
}

func resolveWalletROI(walletAddr string) (winRate float64, avgROI float64, avgEntryBlocks float64) {
	type TradePair struct {
		TokenAddress string
		BuyPrice     float64
		BuyBlock     uint64
		SellPrice    float64
		HasSell      bool
	}

	var buys []database.SmartWalletTrade
	database.DB.Where("wallet = ? AND action = 'BUY'", walletAddr).
		Order("timestamp ASC").Find(&buys)

	if len(buys) == 0 {
		return 0, 0, 999
	}

	pairs := make([]TradePair, 0, len(buys))
	totalEntryBlocks := 0.0

	for _, b := range buys {
		pair := TradePair{
			TokenAddress: b.TokenAddress,
			BuyPrice:     b.Price,
			BuyBlock:     b.BlockNum,
		}

		var sell database.SmartWalletTrade
		err := database.DB.Where(
			"wallet = ? AND token_address = ? AND action = 'SELL' AND block_num > ?",
			walletAddr, b.TokenAddress, b.BlockNum,
		).Order("block_num ASC").First(&sell).Error

		if err == nil && sell.Price > 0 {
			pair.SellPrice = sell.Price
			pair.HasSell = true
		}

		var token database.Token
		if err := database.DB.Where("address = ?", b.TokenAddress).First(&token).Error; err == nil {
			delta := float64(b.BlockNum) - float64(token.CreatedBlock)
			if delta < 0 {
				delta = 0
			}
			totalEntryBlocks += delta
		}

		pairs = append(pairs, pair)
	}

	avgEntryBlocks = totalEntryBlocks / float64(len(pairs))

	closedPairs := 0
	wins := 0
	totalROI := 0.0

	for _, p := range pairs {
		if !p.HasSell || p.BuyPrice <= 0 {
			continue
		}
		closedPairs++
		roi := (p.SellPrice - p.BuyPrice) / p.BuyPrice
		totalROI += roi
		if roi > 0.20 {
			wins++
		}
	}

	if closedPairs > 0 {
		winRate = float64(wins) / float64(closedPairs)
		avgROI = totalROI / float64(closedPairs)
	}
	return
}

func calcScore(w *database.SmartWallet) float64 {
	const (
		wWinRate     = 0.35
		wROI         = 0.30
		wRecency     = 0.20
		wConsistency = 0.10
		wEarly       = 0.05
	)

	fWinRate := sigmoid(w.WinRate, 0.55, 10.0)

	fROI := math.Tanh(w.ROI / 0.5)
	if fROI < 0 {
		fROI = 0
	}

	var lastActive time.Time
	if w.LastActiveAt != nil {
		lastActive = *w.LastActiveAt
	} else {
		lastActive = time.Now()
	}
	daysSinceActive := time.Since(lastActive).Hours() / 24.0
	fRecency := math.Exp(-daysSinceActive / 14.0)

	fConsistency := 0.0
	if w.TotalTrades > 0 {
		logDelta := math.Abs(math.Log(float64(w.TotalTrades) / 15.0))
		fConsistency = math.Max(0, 1.0-logDelta/4.0)
	}

	fEarly := math.Exp(-w.AvgEntryBlocks / 100.0)

	raw := wWinRate*fWinRate + wROI*fROI + wRecency*fRecency +
		wConsistency*fConsistency + wEarly*fEarly

	score := raw * 100.0
	return math.Min(100.0, math.Max(0.0, score))
}

func sigmoid(x, center, steepness float64) float64 {
	return 1.0 / (1.0 + math.Exp(-steepness*(x-center)))
}
