package watcher

import (
	"bot/alpha"
	"bot/dex"
	"bot/types"
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func (w *Watcher) StartSmartMoneyWatcher(wssURL string, processNewPool func(string, string, types.DEXVersion, uint32, bool, uint64)) {
	go func() {
		for {
			err := w.runSmartMoneySubscription(wssURL, processNewPool)
			slog.Warn("SmartMoney WSS 中断，5秒后重连", "err", err)
			time.Sleep(5 * time.Second)
		}
	}()
}

func (w *Watcher) runSmartMoneySubscription(wssURL string, processNewPool func(string, string, types.DEXVersion, uint32, bool, uint64)) error {
	ctx := context.Background()
	wsClient, err := ethclient.DialContext(ctx, wssURL)
	if err != nil { return err }
	defer wsClient.Close()

	// 监听所有 Transfer 事件，后续在代码中过滤 From 为聪明钱的
	query := ethereum.FilterQuery{
		Topics: [][]common.Hash{{dex.TopicTransfer}},
	}
	logCh := make(chan gethtypes.Log, 256)
	sub, err := wsClient.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil { return err }
	defer sub.Unsubscribe()

	slog.Info("✅ SmartMoney 实时监听已启动")

	for {
		select {
		case err := <-sub.Err(): return err
		case lg := <-logCh:
			if len(lg.Topics) < 3 { continue }
			fromAddr := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
			
			// 检查是否来自聪明钱 (此处需要 scoringEngine 提供判定接口)
			// 为了简化，目前先从数据库或内存 map 匹配
			if isSmart, _ := w.scoring.HasSmartMoneyBought(map[string]string{fromAddr: "check"}); isSmart {
				tokenAddr := strings.ToLower(lg.Address.Hex())
				w.handleSmartMoneyBuy(tokenAddr, fromAddr, lg.BlockNumber, processNewPool)
			}
		}
	}
}

func (w *Watcher) handleSmartMoneyBuy(tokenAddr, fromAddr string, block uint64, processNewPool func(string, string, types.DEXVersion, uint32, bool, uint64)) {
	if w.cache.Contains(tokenAddr + "_smart") { return }
	w.cache.Add(tokenAddr + "_smart")

	slog.Info("🔍 发现聪明钱买入存量 Token", "token", tokenAddr, "wallet", fromAddr)
	
	// 尝试寻找对应的池子并触发 ProcessNewPool
	// 这里通常需要调用 alpha.FindV2Pool 等逻辑，为了保持 watcher 纯粹，建议通过回调
	poolAddr, isT0, err := alpha.FindV2Pool(w.rpcClient, tokenAddr)
	if err == nil && poolAddr != "" {
		processNewPool(tokenAddr, poolAddr, types.DEXv2, 0, isT0, block)
	}
}
