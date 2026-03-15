package main

import (
	"bot/database"
	"log/slog"
	"time"
)

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
		// 启发式特征：如果在极短时间内产生巨大交易频率（例如超过 50 笔/天），高度疑似 MEV/高频套利
		if w.TotalTrades > 50 {
			var recentTrades int64
			database.DB.Model(&database.SmartWalletTrade{}).
				Where("wallet = ? AND timestamp > ?", w.Address, time.Now().Add(-24*time.Hour)).
				Count(&recentTrades)

			if recentTrades > 50 {
				w.IsMEV = true
				w.Score = 0 // 直接淘汰
				slog.Info("🤖 [Alpha Engine] 标记 MEV/套利机器人", "wallet", w.Address, "24h_trades", recentTrades)
			}
		}

		// 3. 动态权重打分与战绩回测 (Phase 3.1)
		// 现阶段为骨架：后续可接入完整的 RPC 历史查询查其卖出价格计算真实 ROI 和胜率
		// 目前采用基础交互积分制：交易活跃度适中（非 MEV）给予加分，不活跃降分
		if !w.IsMEV {
			newScore := 50.0 // 基准分
			if w.TotalTrades > 5 && w.TotalTrades <= 50 {
				newScore += float64(w.TotalTrades) * 2.0 // 合理的早期狙击手，适度加分
			} else if w.TotalTrades > 50 {
				newScore -= 10.0 // 过于频繁可能质量下降
			}

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
	// 简单的同车分析：如果两个钱包总是买同样的代币，标记为相同实体 (Cluster)
	detectClusters()

	slog.Info("✅ [Alpha Engine] 聪明钱深度分析完毕", "total_wallets", len(wallets))
}

// detectClusters 简单的女巫/矩阵号聚类算法
func detectClusters() {
	// 查找最热门的 Token (被最多聪明钱买过的)
	type TokenInteraction struct {
		TokenAddress string
		Wallet       string
	}

	var interactions []TokenInteraction
	// 近期 7 天的记录
	database.DB.Table("smart_wallet_trades").
		Select("token_address, wallet").
		Where("timestamp > ?", time.Now().Add(-7*24*time.Hour)).
		Group("token_address, wallet").
		Scan(&interactions)

	// 构建 Token -> []Wallets 的反向映射
	tokenToWallets := make(map[string][]string)
	for _, it := range interactions {
		tokenToWallets[it.TokenAddress] = append(tokenToWallets[it.TokenAddress], it.Wallet)
	}

	// 查找高度重合的钱包组合 (极其简化的聚类示例)
	// 现实中需要基于并查集 (Union-Find) 或者 K-Means 处理，这里做个基础框架。
	// 记录两两钱包同车次数
	coOccurrence := make(map[string]map[string]int)

	for _, wallets := range tokenToWallets {
		if len(wallets) < 2 {
			continue
		}
		for i := 0; i < len(wallets); i++ {
			for j := i + 1; j < len(wallets); j++ {
				w1, w2 := wallets[i], wallets[j]
				if w1 > w2 {
					w1, w2 = w2, w1 // 保证顺序一致
				}
				if coOccurrence[w1] == nil {
					coOccurrence[w1] = make(map[string]int)
				}
				coOccurrence[w1][w2]++
			}
		}
	}

	// 如果同车次数 >= 3，则认为是同一个矩阵/女巫号
	clusterMap := make(map[string]string)
	clusterIndex := 1

	for w1, peers := range coOccurrence {
		for w2, count := range peers {
			if count >= 3 {
				// 找到女巫，分配同一个 ClusterID
				cID := clusterMap[w1]
				if cID == "" {
					cID = clusterMap[w2]
				}
				if cID == "" {
					cID = "CLUSTER-" + time.Now().Format("060102") + "-" + string(rune('A'+clusterIndex))
					clusterIndex++
				}
				clusterMap[w1] = cID
				clusterMap[w2] = cID
			}
		}
	}

	// 更新数据库
	for walletAddr, clusterID := range clusterMap {
		database.DB.Model(&database.SmartWallet{}).Where("address = ?", walletAddr).Update("cluster_id", clusterID)
	}

	if len(clusterMap) > 0 {
		slog.Info("🔗 [Alpha Engine] 实体聚类(防女巫)完成", "identified_nodes", len(clusterMap), "total_clusters", clusterIndex-1)
	}
}
