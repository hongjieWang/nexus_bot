package main

import (
	"bot/config"
	"bot/database"
	"log/slog"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

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

// traceFundingSource 资金溯源模拟函数 (连接 CEX 提币热钱包 / 混币器)
func traceFundingSource(w1, w2 string) bool {
	// 在生产环境中，此处应通过 RPC 查询或索引器获取钱包的第一笔入账交易 (Inflow)
	// 如果两个钱包的第一笔大额资金均来自同一个 CEX 提币地址或同一个中转钱包，则判定为同源。
	
	// 这里模拟一个简单的逻辑：如果是已知的关联钱包对，返回 true
	// 实际开发中可对接本地的 funding_sources 缓存表
	return false 
}

// startAlphaEngine 启动 Phase 3: 聪明钱 Alpha 挖掘与防女巫/MEV 引擎
func startAlphaEngine() {
	if database.DB == nil {
		slog.Warn("数据库未配置，Alpha Engine (Phase 3) 无法启动")
		return
	}

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
			// 基准分可调
			baseScoreStr := config.GetConfig("ALPHA_BASE_SCORE")
			baseScore := 50.0
			if v, err := strconv.ParseFloat(baseScoreStr, 64); err == nil {
				baseScore = v
			}

			newScore := baseScore 
			// 交易频率权重：过低没参考价值，适中最好，过高疑似机器人
			if w.TotalTrades > 3 && w.TotalTrades <= 30 {
				newScore += float64(w.TotalTrades) * 1.5
			} else if w.TotalTrades > 30 {
				newScore -= 5.0 
			}

			// 胜率与 ROI 权重 (假设已通过回测引擎更新)
			newScore += w.WinRate * 40.0 // 胜率贡献最高 40 分
			newScore += (w.ROI / 100.0) * 10.0 // ROI 贡献

			// 平滑处理得分，设置上下限
			if newScore > 100.0 {
				newScore = 100.0
			}
			if newScore < 0.0 {
				newScore = 0.0
			}
			w.Score = newScore
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
