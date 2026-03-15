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

// detectClusters 基于 Union-Find 和时间窗口的女巫/矩阵号聚类算法
func detectClusters() {
	// 查找最近 7 天的 Token 交互记录
	type TokenInteraction struct {
		TokenAddress string
		Wallet       string
		Timestamp    time.Time
	}

	var interactions []TokenInteraction
	database.DB.Table("smart_wallet_trades").
		Select("token_address, wallet, timestamp").
		Where("timestamp > ?", time.Now().Add(-7*24*time.Hour)).
		Order("token_address, timestamp ASC").
		Scan(&interactions)

	// 构建 Token -> []TokenInteraction 的映射
	tokenToInteractions := make(map[string][]TokenInteraction)
	for _, it := range interactions {
		tokenToInteractions[it.TokenAddress] = append(tokenToInteractions[it.TokenAddress], it)
	}

	// 记录两两钱包在极短时间内的同车次数 (加强版防误判：例如 15 分钟内共同买入)
	coOccurrence := make(map[string]map[string]int)
	walletSet := make(map[string]bool)

	for _, tokenInteracts := range tokenToInteractions {
		n := len(tokenInteracts)
		if n < 2 {
			continue
		}
		// O(N^2) 扫描单 Token 内的距离，因为已经按时间排序，可以通过滑动窗口优化，这里暂用简单的双重循环
		for i := 0; i < n; i++ {
			walletSet[tokenInteracts[i].Wallet] = true
			for j := i + 1; j < n; j++ {
				// 如果两笔交易相差超过 15 分钟，说明并非同一批脚本发出的狙击，跳过
				if tokenInteracts[j].Timestamp.Sub(tokenInteracts[i].Timestamp) > 15*time.Minute {
					break // 后面的时间差更大
				}
				
				w1, w2 := tokenInteracts[i].Wallet, tokenInteracts[j].Wallet
				if w1 == w2 {
					continue
				}
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

	// === Union-Find (并查集) 实现 ===
	parent := make(map[string]string)
	
	// 初始化：每个节点各自为独立集合
	for w := range walletSet {
		parent[w] = w
	}

	// 查找根节点 (带路径压缩)
	var find func(string) string
	find = func(i string) string {
		if parent[i] == i {
			return i
		}
		parent[i] = find(parent[i])
		return parent[i]
	}

	// 合并两个集合
	union := func(i, j string) {
		rootI := find(i)
		rootJ := find(j)
		if rootI != rootJ {
			// 简单的将一个根指向另一个，不需要 rank
			parent[rootI] = rootJ 
		}
	}

	// 根据同车次数阈值进行连边 (Union)
	// 阈值：如果在 15 分钟内共同狙击过 >= 3 个相同的土狗，判定为女巫/矩阵号
	for w1, peers := range coOccurrence {
		for w2, count := range peers {
			if count >= 3 {
				union(w1, w2)
			}
		}
	}

	// 提取聚类结果
	clusters := make(map[string][]string)
	for w := range walletSet {
		r := find(w)
		clusters[r] = append(clusters[r], w)
	}

	// 写入数据库
	clusterIndex := 1
	totalNodes := 0

	for root, members := range clusters {
		// 只有包含 2 个及以上成员的才算有效 Cluster
		if len(members) > 1 {
			cID := "CLUSTER-" + time.Now().Format("060102") + "-" + string(rune('A'+clusterIndex-1))
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
		slog.Info("🔗 [Alpha Engine] 实体聚类(防女巫)完成", "identified_nodes", totalNodes, "total_clusters", clusterIndex-1)
	}
}
