package alpha

import (
	"bot/config"
	"bot/database"
	"bot/metrics"
	"log/slog"
	"strconv"
	"time"
)

// detectClusters 基于 Union-Find 和多维权重的女巫/矩阵号聚类算法
func (e *AlphaEngine) detectClusters() {
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
	threshold := 10
	if v, err := strconv.Atoi(thresholdStr); err == nil && v > 0 {
		threshold = v
	}

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

				timeWeight := 5.0 * (1.0 - float64(diff)/float64(timeWin))
				if timeWeight < 1.0 {
					timeWeight = 1.0
				}

				amt1, amt2 := tokenInteracts[i].AmountUSD, tokenInteracts[j].AmountUSD
				amountWeight := 0.0
				if amt1 > 0 && amt2 > 0 {
					ratio := amt1 / amt2
					if ratio > 1.0 {
						ratio = 1.0 / ratio
					}
					if ratio > 0.9 {
						amountWeight = 5.0
					}
				}

				fundingWeight := 0.0
				if TraceFundingSource(e.rpcClient, w1, w2) {
					fundingWeight = 20.0
				}

				coOccurrence[w1][w2] += (timeWeight + amountWeight + fundingWeight)
			}
		}
	}

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
		metrics.SybilFilteredWallets.Set(float64(totalNodes))
		metrics.AlphaClustersTotal.Set(float64(clusterIndex - 1))
		slog.Info("🔗 [Alpha Engine] 加权实体聚类完成", "nodes", totalNodes, "clusters", clusterIndex-1)
	}
}

// aggregateEntities 将同一个 ClusterID 的多个钱包战绩聚合到 SmartEntity 表
func (e *AlphaEngine) aggregateEntities() {
	type Result struct {
		ClusterID   string
		WalletCount int
		TotalTrades int
		AvgWinRate  float64
		AvgROI      float64
		AvgScore    float64
	}

	var results []Result
	database.DB.Model(&database.SmartWallet{}).
		Select("cluster_id, count(address) as wallet_count, sum(total_trades) as total_trades, avg(win_rate) as avg_win_rate, avg(roi) as avg_roi, avg(score) as avg_score").
		Where("cluster_id != '' AND is_mev = ?", false).
		Group("cluster_id").
		Scan(&results)

	for _, r := range results {
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
		database.DB.Save(&entity)
	}

	if len(results) > 0 {
		slog.Info("🧬 [Alpha Engine] 实体 (Entity) 聚合完成", "total_entities", len(results))
	}
}
