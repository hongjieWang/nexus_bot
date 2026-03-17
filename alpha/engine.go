package alpha

import (
	"bot/database"
	"bot/rpc"
	"log/slog"
	"time"
)

type AlphaEngine struct {
	rpcClient *rpc.Client
}

func NewAlphaEngine(rpcClient *rpc.Client) *AlphaEngine {
	return &AlphaEngine{rpcClient: rpcClient}
}

func (e *AlphaEngine) StartAll() {
	if database.DB == nil {
		slog.Warn("数据库未配置，Alpha Engine 无法启动")
		return
	}

	slog.Info("🔮 Alpha Engine 后台分析任务已启动")

	go func() {
		time.Sleep(1 * time.Minute)
		e.evaluateSmartWallets()

		ticker := time.NewTicker(4 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			e.evaluateSmartWallets()
		}
	}()

	go func() {
		time.Sleep(2 * time.Minute)
		e.trackSmartWalletSells()

		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			e.trackSmartWalletSells()
		}
	}()
}
