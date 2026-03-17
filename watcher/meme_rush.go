package watcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

func (w *Watcher) StartMemeRushWatcher(wssURL string) {
	go func() {
		for {
			err := w.runMemeRushSubscription(wssURL)
			slog.Warn("MemeRush WSS 中断，5秒后重连", "err", err)
			time.Sleep(5 * time.Second)
		}
	}()
}

func (w *Watcher) runMemeRushSubscription(wssURL string) error {
	ctx := context.Background()
	wsClient, err := ethclient.DialContext(ctx, wssURL)
	if err != nil {
		return err
	}
	defer wsClient.Close()

	// 模拟 Four.Meme 或其他 Pump 平台的 Factory 监听
	slog.Info("✅ MemeRush 实时监听已启动")
	select {}
}
