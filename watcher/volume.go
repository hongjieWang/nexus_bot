package watcher

import (
	"time"
)

func (w *Watcher) StartVolumeAnomalyWatcher() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			w.checkVolumeAnomalies()
		}
	}()
}

func (w *Watcher) checkVolumeAnomalies() {
	// 扫描数据库中活跃 Token 的成交量
}
