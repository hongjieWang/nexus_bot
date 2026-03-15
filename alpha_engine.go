package main

import (
	"bot/config"
	"bot/database"
	"context"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"log/slog"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var alphaRPCClient *ethclient.Client

// 已知 CEX 热钱包（2026最新精选，可每周更新）
var knownCEXHotWallets = map[string]string{
	"0x28c6c06298d514db089934071355e5743bf21d60": "BINANCE",
	"0x3f5CE5FBFe3E9af3971dD833D26bA9b5C9aE9A9":  "BINANCE",
	"0xc9f5296eb3ac266c94568d790b6e91eba7d76a11": "CEXIO",
	"0xad6ec9801f04f45e7f6d907ec6b72246b66ff4f3": "CEXIO",
	// ...（继续补充 50+ 个，来源：bscscan labeled + slowmist）
}

// 线程安全缓存（开发用，生产换 Redis）
var fundingCache sync.Map // wallet → "BINANCE"
var (
	alphaClustersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "alpha_clusters_total",
		Help: "Current number of smart wallet clusters identified",
	})
	sybilFilteredWallets = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sybil_filtered_wallets",
		Help: "Current number of wallets filtered/flagged as sybil matrix",
	})
)

// traceFundingSource 使用 RPC 溯源两个钱包的资金来源是否为同一个 CEX
func traceFundingSource(w1, w2 string) bool {
	if alphaRPCClient == nil {
		return false
	}

	src1 := getWalletFundingSource(w1)
	if src1 == "" {
		return false
	}

	src2 := getWalletFundingSource(w2)
	if src2 == "" {
		return false
	}

	return src1 == src2
}

// resolveWalletROI 扫描某钱包对某代币的卖出记录，计算 ROI
// 依赖：SmartWalletTrade 中的 BUY 记录已存在 Price 字段
func resolveWalletROI(walletAddr string) (winRate float64, avgROI float64, avgEntryBlocks float64) {
	type TradePair struct {
		TokenAddress string
		BuyPrice     float64
		BuyBlock     uint64
		SellPrice    float64
		HasSell      bool
	}

	// 查出所有 BUY 记录
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

		// 查找对应的 SELL（同 wallet 同 token，时间在 BUY 之后）
		var sell database.SmartWalletTrade
		err := database.DB.Where(
			"wallet = ? AND token_address = ? AND action = 'SELL' AND block_num > ?",
			walletAddr, b.TokenAddress, b.BlockNum,
		).Order("block_num ASC").First(&sell).Error

		if err == nil && sell.Price > 0 {
			pair.SellPrice = sell.Price
			pair.HasSell = true
		}

		// 记录入场时间：需要代币池创建块号，从 tokens 表查
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

	// 只用有 SELL 记录的配对计算胜率和 ROI
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
		if roi > 0.20 { // 至少盈利 20% 才算赢
			wins++
		}
	}

	if closedPairs > 0 {
		winRate = float64(wins) / float64(closedPairs)
		avgROI = totalROI / float64(closedPairs)
	}
	return
}

// calcScore 多维加权评分，返回 0-100
func calcScore(w *database.SmartWallet) float64 {
	const (
		wWinRate     = 0.35
		wROI         = 0.30
		wRecency     = 0.20
		wConsistency = 0.10
		wEarly       = 0.05
	)

	// ── 1. 胜率分量：sigmoid 以 55% 为中心 ──
	fWinRate := sigmoid(w.WinRate, 0.55, 10.0)

	// ── 2. ROI 分量：tanh 压缩，防单笔极端值拉偏 ──
	fROI := math.Tanh(w.ROI / 0.5)
	if fROI < 0 {
		fROI = 0 // ROI 为负时该分量归零
	}

	// ── 3. 近期活跃分量：指数衰减，半衰期约 14 天 ──
	daysSinceActive := time.Since(w.LastActiveAt).Hours() / 24.0
	fRecency := math.Exp(-daysSinceActive / 14.0)

	// ── 4. 交易频率一致性：峰值在 15 笔，两侧对数衰减 ──
	fConsistency := 0.0
	if w.TotalTrades > 0 {
		logDelta := math.Abs(math.Log(float64(w.TotalTrades) / 15.0))
		fConsistency = math.Max(0, 1.0-logDelta/4.0)
	}

	// ── 5. 早入场奖励：指数衰减，100 块为特征尺度 ──
	fEarly := math.Exp(-w.AvgEntryBlocks / 100.0)

	raw := wWinRate*fWinRate + wROI*fROI + wRecency*fRecency +
		wConsistency*fConsistency + wEarly*fEarly

	score := raw * 100.0
	return math.Min(100.0, math.Max(0.0, score))
}

// sigmoid 广义 Logistic 函数
func sigmoid(x, center, steepness float64) float64 {
	return 1.0 / (1.0 + math.Exp(-steepness*(x-center)))
}

// getWalletFundingSource 是 traceFundingSource 的辅助函数，带缓存
func getWalletFundingSource(wallet string) string {
	if src, ok := fundingCache.Load(wallet); ok {
		return src.(string)
	}

	if cex, ok := knownCEXHotWallets[wallet]; ok {
		fundingCache.Store(wallet, cex)
		return cex
	}

	// RPC 溯源逻辑
	latest, err := alphaRPCClient.BlockNumber(context.Background())
	if err != nil {
		slog.Error("获取最新区块号失败 for funding source", "err", err)
		return ""
	}

	fromBlock := new(big.Int).Sub(big.NewInt(int64(latest)), big.NewInt(30*24*60*60/3)) // ≈30天

	// 查询入账历史
	query := ethereum.FilterQuery{
		Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))}, nil, {common.HexToHash(wallet)}},
		FromBlock: fromBlock,
		ToBlock:   new(big.Int).SetUint64(latest),
	}

	logs, err := alphaRPCClient.FilterLogs(context.Background(), query)
	if err != nil {
		slog.Error("FilterLogs 失败 for funding source", "err", err)
		return ""
	}

	for _, log := range logs {
		if len(log.Topics) == 3 {
			fromAddr := common.BytesToAddress(log.Topics[1].Bytes()).Hex()
			if cex, ok := knownCEXHotWallets[fromAddr]; ok {
				fundingCache.Store(wallet, cex)
				return cex
			}
		}
	}

	fundingCache.Store(wallet, "") // 存空，避免重复查询
	return ""
}

// startAlphaEngine 启动 Phase 3: 聪明钱 Alpha 挖掘与防女巫/MEV 引擎
func startAlphaEngine(rpcClient *ethclient.Client) {
	if database.DB == nil {
		slog.Warn("数据库未配置，Alpha Engine (Phase 3) 无法启动")
		return
	}
	alphaRPCClient = rpcClient // 保存 RPC 客户端实例

	slog.Info("🔮 Alpha Engine 启动，开启智能打分与实体聚类扫描...")

	go func() {
		// 初次启动延迟 1 分钟后运行
		time.Sleep(1 * time.Minute)
		evaluateSmartWallets()

		// 之后每 4 小时做一次深度分析
		ticker := time.NewTicker(4 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			evaluateSmartWallets()
		}
	}()

	// ← 新增：SELL 轨迹扫描 goroutine
	go func() {
		// 延迟 2 分钟，等 rpcHTTP 和数据库都稳定后再启动
		time.Sleep(2 * time.Minute)
		trackSmartWalletSells()

		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			trackSmartWalletSells()
		}
	}()
}

// evaluateSmartWallets 对数据库中的聪明钱进行战绩回测、打分和过滤
func evaluateSmartWallets() {
	slog.Info("🔍 [Alpha Engine] 开始执行聪明钱深度分析...")

	var wallets []database.SmartWallet
	if err := database.DB.Find(&wallets).Error; err != nil {
		slog.Error("Alpha Engine 获取钱包列表失败", "err", err)
		return
	}

	for i := range wallets {
		w := &wallets[i]

		// 1. 获取该钱包的交互记录
		var trades []database.SmartWalletTrade
		database.DB.Where("wallet = ?", w.Address).Find(&trades)

		w.TotalTrades = len(trades)

		// 2. MEV / 套利机器人过滤 (Phase 3.3)
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
				w.Score = 0 // 直接淘汰
				slog.Info("🤖 [Alpha Engine] 标记 MEV/套利机器人", "wallet", w.Address, "24h_trades", recentTrades)
			}
		}

		// 3. 动态权重打分与战绩回测 (Phase 3.1)
		if !w.IsMEV {
			// 计算真实战绩
			winRate, avgROI, avgEntryBlocks := resolveWalletROI(w.Address)

			// 写回统计字段
			w.WinRate = winRate
			w.ROI = avgROI
			w.AvgEntryBlocks = avgEntryBlocks

			// 更新最后活跃时间
			var lastTrade database.SmartWalletTrade
			if err := database.DB.Where("wallet = ?", w.Address).
				Order("timestamp DESC").First(&lastTrade).Error; err == nil {
				w.LastActiveAt = lastTrade.Timestamp
			}

			// 关键修复：防止零值时间导致 MySQL 报错 (0000-00-00)
			if w.LastActiveAt.IsZero() {
				if !w.CreatedAt.IsZero() {
					w.LastActiveAt = w.CreatedAt
				} else {
					w.LastActiveAt = time.Now()
				}
			}

			// 样本量保护：少于 3 笔已结交易时降级使用旧公式，避免噪声得高分
			var closedCount int64 // 修复: 直接使用 int64 (中危 #15)
			database.DB.Model(&database.SmartWalletTrade{}).
				Where("wallet = ? AND action = 'SELL'", w.Address).
				Count(&closedCount)

			if closedCount >= 3 {
				w.Score = calcScore(w)
			} else {
				// 数据不足时给保守基准分（低于过滤阈值 20 分的钱包不会被追踪）
				w.Score = 30.0 + float64(w.TotalTrades)*0.5
				w.Score = math.Min(w.Score, 49.0) // 未验证的钱包上限 49 分
			}
		}

		// 保存回数据库
		database.DB.Save(w)
	}

	// 4. 实体聚类与防女巫 (Phase 3.2)
	detectClusters()

	// 5. 聚合实体战绩 (实体 PnL / WinRate 汇总)
	aggregateEntities()

	slog.Info("✅ [Alpha Engine] 聪明钱深度分析完毕", "total_wallets", len(wallets))
}

// detectClusters 基于 Union-Find 和多维权重的女巫/矩阵号聚类算法
func detectClusters() {
	// 查找最近 7 天的 Token 交互记录
	type TokenInteraction struct {
		TokenAddress string
		Wallet       string
		Timestamp    time.Time
		AmountUSD    float64
	}

	var interactions []TokenInteraction
	database.DB.Table("smart_wallet_trades").
		Select("token_address, wallet, timestamp, amount_usd").
		Where("timestamp > ?", time.Now().Add(-7*24*time.Hour)).
		Order("token_address, timestamp ASC").
		Scan(&interactions)

	// 构建 Token -> []TokenInteraction 的映射
	tokenToInteractions := make(map[string][]TokenInteraction)
	for _, it := range interactions {
		tokenToInteractions[it.TokenAddress] = append(tokenToInteractions[it.TokenAddress], it)
	}

	timeWinStr := config.GetConfig("ALPHA_CLUSTER_TIME_WINDOW_MINS")
	timeWinMins := 15
	if v, err := strconv.Atoi(timeWinStr); err == nil && v > 0 {
		timeWinMins = v
	}
	timeWin := time.Duration(timeWinMins) * time.Minute

	thresholdStr := config.GetConfig("ALPHA_CLUSTER_THRESHOLD")
	threshold := 10 // 提高了阈值，因为现在是加权分
	if v, err := strconv.Atoi(thresholdStr); err == nil && v > 0 {
		threshold = v
	}

	// 记录两两钱包的累积关联分数
	coOccurrence := make(map[string]map[string]float64)
	walletSet := make(map[string]bool)

	for _, tokenInteracts := range tokenToInteractions {
		n := len(tokenInteracts)
		if n < 2 {
			continue
		}
		for i := 0; i < n; i++ {
			walletSet[tokenInteracts[i].Wallet] = true
			for j := i + 1; j < n; j++ {
				diff := tokenInteracts[j].Timestamp.Sub(tokenInteracts[i].Timestamp)
				if diff > timeWin {
					break
				}

				w1, w2 := tokenInteracts[i].Wallet, tokenInteracts[j].Wallet
				if w1 == w2 {
					continue
				}
				if w1 > w2 {
					w1, w2 = w2, w1
				}
				if coOccurrence[w1] == nil {
					coOccurrence[w1] = make(map[string]float64)
				}

				// --- 多维权重计算 ---
				// 1. 基础同车分 (按时间差衰减)
				// 越是同时买入，分数越高。1秒内买入给 5分，15分钟边缘给 1分
				timeWeight := 5.0 * (1.0 - float64(diff)/float64(timeWin))
				if timeWeight < 1.0 {
					timeWeight = 1.0
				}

				// 2. 金额相似度权重
				// 如果两个钱包买入金额非常接近 (例如相差 < 10%)，极大增加关联概率
				amt1, amt2 := tokenInteracts[i].AmountUSD, tokenInteracts[j].AmountUSD
				amountWeight := 0.0
				if amt1 > 0 && amt2 > 0 {
					ratio := amt1 / amt2
					if ratio > 1.0 {
						ratio = 1.0 / ratio
					}
					if ratio > 0.9 { // 金额极其相近
						amountWeight = 5.0
					}
				}

				// 3. 资金溯源权重 (CEX/Mixer)
				fundingWeight := 0.0
				if traceFundingSource(w1, w2) {
					fundingWeight = 20.0
				}

				coOccurrence[w1][w2] += (timeWeight + amountWeight + fundingWeight)
			}
		}
	}

	// === Union-Find (并查集) 实现 ===
	parent := make(map[string]string)
	for w := range walletSet {
		parent[w] = w
	}

	var find func(string) string
	find = func(i string) string {
		if parent[i] == i {
			return i
		}
		parent[i] = find(parent[i])
		return parent[i]
	}

	union := func(i, j string) {
		rootI := find(i)
		rootJ := find(j)
		if rootI != rootJ {
			parent[rootI] = rootJ
		}
	}

	// 根据累积加权分数进行连边
	for w1, peers := range coOccurrence {
		for w2, totalWeight := range peers {
			if totalWeight >= float64(threshold) {
				union(w1, w2)
			}
		}
	}

	clusters := make(map[string][]string)
	for w := range walletSet {
		r := find(w)
		clusters[r] = append(clusters[r], w)
	}

	clusterIndex := 1
	totalNodes := 0
	// 先清除旧的 ClusterID
	database.DB.Model(&database.SmartWallet{}).Where("1=1").Update("cluster_id", "")

	for _, members := range clusters {
		if len(members) > 1 {
			cID := "CLUSTER-" + time.Now().Format("060102") + "-" + strconv.Itoa(clusterIndex)
			clusterIndex++

			for _, w := range members {
				database.DB.Model(&database.SmartWallet{}).
					Where("address = ?", w).
					Update("cluster_id", cID)
				totalNodes++
			}
		}
	}

	if totalNodes > 0 {
		sybilFilteredWallets.Set(float64(totalNodes))
		alphaClustersTotal.Set(float64(clusterIndex - 1))
		slog.Info("🔗 [Alpha Engine] 加权实体聚类完成", "nodes", totalNodes, "clusters", clusterIndex-1)
	}
}

// aggregateEntities 将同一个 ClusterID 的多个钱包战绩聚合到 SmartEntity 表
func aggregateEntities() {
	type Result struct {
		ClusterID   string
		WalletCount int
		TotalTrades int
		AvgWinRate  float64
		AvgROI      float64
		AvgScore    float64
	}

	var results []Result
	// GORM Group By query: 聚合非 MEV 钱包的数据
	database.DB.Model(&database.SmartWallet{}).
		Select("cluster_id, count(address) as wallet_count, sum(total_trades) as total_trades, avg(win_rate) as avg_win_rate, avg(roi) as avg_roi, avg(score) as avg_score").
		Where("cluster_id != '' AND is_mev = ?", false).
		Group("cluster_id").
		Scan(&results)

	for _, r := range results {
		// 聚合得分：除了平均分外，如果钱包数量多，说明实体规模大，给予额外加权 (Sybil Power)
		finalScore := r.AvgScore
		if r.WalletCount > 5 {
			finalScore += 5.0
		}

		entity := database.SmartEntity{
			ID:          r.ClusterID,
			WalletCount: r.WalletCount,
			TotalTrades: r.TotalTrades,
			WinRate:     r.AvgWinRate,
			ROI:         r.AvgROI,
			Score:       finalScore,
		}
		// Upsert (存在就更新，不存在就插入)
		database.DB.Save(&entity)
	}

	if len(results) > 0 {
		slog.Info("🧬 [Alpha Engine] 实体 (Entity) 聚合完成", "total_entities", len(results))
	}
}

// ================== SELL 轨迹扫描 ==================

// trackSmartWalletSells 扫描近 7 天有 BUY 记录的聪明钱，补录其卖出行为
// 直接使用 main.go 中的全局变量：rpcHTTP、dexRouters、topicTransfer
func trackSmartWalletSells() {
	slog.Info("🔍 [Alpha Engine] 开始扫描聪明钱 SELL 轨迹...")

	type BuyRecord struct {
		Wallet       string
		TokenAddress string
		PoolAddress  string
		MinBlock     uint64
	}

	var records []BuyRecord
	database.DB.Table("smart_wallet_trades").
		Select("wallet, token_address, pool_address, min(block_num) as min_block").
		Where("action = 'BUY' AND timestamp > ?", time.Now().Add(-7*24*time.Hour)).
		Group("wallet, token_address").
		Scan(&records)

	if len(records) == 0 {
		return
	}

	latest, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) (uint64, error) {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return rpcHTTP.BlockNumber(c)
	})
	if err != nil {
		slog.Warn("[Alpha Engine] 获取最新块失败，跳过 SELL 扫描", "err", err)
		return
	}

	found := 0
	for _, r := range records {
		// 已有 SELL 记录则跳过
		var cnt int64
		database.DB.Model(&database.SmartWalletTrade{}).
			Where("wallet = ? AND token_address = ? AND action = 'SELL'", r.Wallet, r.TokenAddress).
			Count(&cnt)
		if cnt > 0 {
			continue
		}

		toBlock := r.MinBlock + 2000
		if toBlock > latest {
			toBlock = latest
		}
		if toBlock <= r.MinBlock {
			continue
		}

		// 扫描从 wallet 发出的 Transfer（即卖出动作）
		query := ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(r.MinBlock + 1),
			ToBlock:   new(big.Int).SetUint64(toBlock),
			Addresses: []common.Address{common.HexToAddress(r.TokenAddress)},
			Topics: [][]common.Hash{
				{topicTransfer},
				{common.HexToHash(r.Wallet)}, // from = 聪明钱地址
			},
		}

		logs, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) ([]gethtypes.Log, error) {
			c, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			return rpcHTTP.FilterLogs(c, query)
		})
		if err != nil {
			continue
		}

		for _, lg := range logs {
			if len(lg.Topics) < 3 {
				continue
			}
			toAddr := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())

			// 卖出的接收方必须是 router 或 pool，否则是普通转账不算卖出
			_, isRouter := dexRouters[toAddr]
			isPool := r.PoolAddress != "" && toAddr == strings.ToLower(r.PoolAddress)
			if !isRouter && !isPool {
				continue
			}

			// 在卖出块估算代币价格
			price := estimatePriceAtBlock(r.TokenAddress, r.PoolAddress, lg.BlockNumber)

			database.RecordSmartWalletTradeWithPrice(
				r.Wallet, r.TokenAddress, "SELL", lg.BlockNumber, price, r.PoolAddress,
			)
			found++
			break // 同一 token 只记录最早的一笔卖出
		}
	}

	slog.Info("✅ [Alpha Engine] SELL 轨迹扫描完毕", "new_sells_found", found, "pairs_scanned", len(records))
}

// estimatePriceAtBlock 查询某 V2 Pool 在指定块的 getReserves，
// 返回每个 token 对应的 BNB 单价（BNB/token）
// 注意：仅支持 V2；V3 目前返回 0（降级，不影响统计正确性）
func estimatePriceAtBlock(tokenAddr, poolAddr string, blockNum uint64) float64 {
	if poolAddr == "" {
		return 0
	}

	to := common.HexToAddress(poolAddr)
	blockBig := new(big.Int).SetUint64(blockNum)

	result, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return rpcHTTP.CallContract(c,
			ethereum.CallMsg{To: &to, Data: selectorGetReserves},
			blockBig,
		)
	})
	if err != nil || len(result) < 64 {
		return 0
	}

	r0 := new(big.Int).SetBytes(result[0:32])
	r1 := new(big.Int).SetBytes(result[32:64])
	if r0.Sign() == 0 || r1.Sign() == 0 {
		return 0
	}

	wbnbNorm := strings.ToLower(wbnb)
	tokenNorm := strings.ToLower(tokenAddr)

	// 判断哪边是 WBNB：通过比对地址
	// r0 对应 token0（字典序较小的地址），r1 对应 token1
	var bnbReserve, tokenReserve *big.Int
	if tokenNorm < wbnbNorm {
		// token 是 token0，WBNB 是 token1
		tokenReserve, bnbReserve = r0, r1
	} else {
		// WBNB 是 token0，token 是 token1
		bnbReserve, tokenReserve = r0, r1
	}

	if tokenReserve.Sign() == 0 {
		return 0
	}

	// price = bnbReserve / tokenReserve（单位均为 wei，比值即为价格）
	price, _ := new(big.Float).Quo(
		new(big.Float).SetInt(bnbReserve),
		new(big.Float).SetInt(tokenReserve),
	).Float64()

	return price
}
