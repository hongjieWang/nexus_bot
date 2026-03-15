package api

import (
	"bot/config"
	"bot/database"
	"encoding/json"
	"log/slog"
	"net/http"
)

// StartAPIServer 启动给 React Dashboard 调用的后端 API
func StartAPIServer() {
	mux := http.NewServeMux()

	// CORS 处理中间件
	cors := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				return
			}
			next(w, r)
		}
	}

	// 核心配置接口：加密存取 API Keys 和 私钥
	mux.HandleFunc("/api/config", cors(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// 为了安全，GET 时不在接口中明文返回 PrivateKey 等高度敏感信息，只返回其是否已配置的状态
			hasKey := config.GetConfig("EXECUTION_PRIVATE_KEY") != ""

			json.NewEncoder(w).Encode(map[string]interface{}{
				"dex_sniper_enabled": config.GetConfig("DEX_SNIPER_ENABLED") == "true",
				"trade_enabled":      config.GetConfig("TRADE_ENABLED") == "true",
				"has_private_key":    hasKey,
				"trade_usdt":         config.GetConfig("TRADE_USDT"),
			})
			return
		}

		if r.Method == "POST" {
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
				for k, v := range payload {
					// 只有非空值才更新
					if v != "" {
						config.SetConfig(k, v)
					}
				}
				json.NewEncoder(w).Encode(map[string]string{"status": "success", "msg": "配置已加密并安全入库"})
			}
		}
	}))

	// 大盘与交易可视化数据接口
	mux.HandleFunc("/api/metrics", cors(func(w http.ResponseWriter, r *http.Request) {
		var totalTrades int64
		var winTrades int64
		var activeTokens int64

		if database.DB != nil {
			database.DB.Model(&database.TradeHistory{}).Count(&totalTrades)
			database.DB.Model(&database.TradeHistory{}).Where("pn_l > 0").Count(&winTrades)
			database.DB.Model(&database.Token{}).Count(&activeTokens)
		}

		winRate := 0.0
		if totalTrades > 0 {
			winRate = float64(winTrades) / float64(totalTrades) * 100
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_trades":  totalTrades,
			"win_rate":      winRate,
			"active_tokens": activeTokens,
		})
	}))

	slog.Info("🌐 React Dashboard API Server 启动于 :18080")
	if err := http.ListenAndServe(":18080", mux); err != nil {
		slog.Error("API Server 启动失败", "err", err)
	}
}
