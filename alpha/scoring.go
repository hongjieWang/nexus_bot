package alpha

import (
	"bot/config"
	"bot/dex"
	"bot/rpc"
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

type ScoringEngine struct {
	rpcClient    *rpc.Client
	smartWallets map[string]string
	mu           sync.RWMutex
}

func NewScoringEngine(rpcClient *rpc.Client) *ScoringEngine {
	return &ScoringEngine{
		rpcClient:    rpcClient,
		smartWallets: make(map[string]string),
	}
}

func (e *ScoringEngine) SetSmartWallets(wallets map[string]string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.smartWallets = wallets
}

func (e *ScoringEngine) CountSmartBuys(tokenAddr, poolAddr string, createdBlock uint64, cfg *config.Config) (map[string]string, error) {
	latest, err := rpc.Call(e.rpcClient, context.Background(), func(ctx context.Context) (uint64, error) {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return e.rpcClient.Client.BlockNumber(c)
	})
	if err != nil {
		return nil, fmt.Errorf("获取最新块: %w", err)
	}

	toBlock := createdBlock + uint64(cfg.TransferScanBlocks)
	if toBlock > latest {
		toBlock = latest
	}

	e.mu.RLock()
	fromTopics := make([]common.Hash, 0, len(e.smartWallets))
	for addr := range e.smartWallets {
		fromTopics = append(fromTopics, common.HexToHash(addr))
	}
	e.mu.RUnlock()

	if len(fromTopics) == 0 {
		return nil, nil
	}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(createdBlock),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Addresses: []common.Address{common.HexToAddress(tokenAddr)},
		Topics:    [][]common.Hash{{dex.TopicTransfer}, fromTopics},
	}

	logs, err := rpc.Call(e.rpcClient, context.Background(), func(ctx context.Context) ([]gethtypes.Log, error) {
		c, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		return e.rpcClient.Client.FilterLogs(c, query)
	})
	if err != nil {
		return nil, fmt.Errorf("FilterLogs(#%d~#%d): %w", createdBlock, toBlock, err)
	}

	hits := make(map[string]string)
	e.mu.RLock()
	for _, lg := range logs {
		if len(lg.Topics) < 2 {
			continue
		}
		fromAddr := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
		if label, ok := e.smartWallets[fromAddr]; ok {
			hits[fromAddr] = label
		}
	}
	e.mu.RUnlock()

	return hits, nil
}

func (e *ScoringEngine) HasSmartMoneyBought(hitWallets map[string]string) (bool, int) {
	if len(hitWallets) == 0 {
		return false, 0
	}

	highQualityHits := 0
	for _, label := range hitWallets {
		// 简单的打分逻辑：包含特定关键词的视为高质量
		if strings.Contains(strings.ToUpper(label), "TOP") || strings.Contains(strings.ToUpper(label), "SMART") {
			highQualityHits++
		}
	}

	// 只要有聪明钱买入，目前就先通过，具体打分可在此扩展
	return len(hitWallets) >= 1, len(hitWallets)
}
