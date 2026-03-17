package watcher

import (
	"bot/alpha"
	"bot/config"
	"bot/rpc"
	"bot/types"
	"bot/utils"
)

type Watcher struct {
	rpcClient *rpc.Client
	cfg       *config.Config
	cache     *utils.Cache
	scoring   *alpha.ScoringEngine
}

func NewWatcher(rpcClient *rpc.Client, cfg *config.Config, cache *utils.Cache, scoring *alpha.ScoringEngine) *Watcher {
	return &Watcher{
		rpcClient: rpcClient,
		cfg:       cfg,
		cache:     cache,
		scoring:   scoring,
	}
}

func (w *Watcher) StartAll(wssURL string, processNewPool func(string, string, types.DEXVersion, uint32, bool, uint64)) {
	w.StartSmartMoneyWatcher(wssURL, processNewPool)
	w.StartMemeRushWatcher(wssURL)
	w.StartVolumeAnomalyWatcher()
}
