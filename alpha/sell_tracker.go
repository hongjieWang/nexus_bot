package alpha

import (
	"bot/database"
	"bot/dex"
	"context"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// trackSmartWalletSells 扫描近 7 天有 BUY 记录的聪明钱，补录其卖出行为
func (e *AlphaEngine) trackSmartWalletSells() {
	slog.Info("🔍 [Alpha Engine] 开始扫描聪明钱 SELL 轨迹...")

	type BuyRecord struct {
		Wallet       string
		TokenAddress string
		PoolAddress  string
		MinBlock     uint64
	}

	var records []BuyRecord
	database.DB.Table("smart_wallet_trades").
		Select("wallet, token_address, pool_address, min(block_num) as min_block").
		Where("action = 'BUY' AND timestamp > ?", time.Now().Add(-7*24*time.Hour)).
		Group("wallet, token_address, pool_address").
		Scan(&records)

	if len(records) == 0 {
		return
	}

	latest, err := e.rpcClient.Client.BlockNumber(context.Background())
	if err != nil {
		slog.Warn("[Alpha Engine] 获取最新块失败，跳过 SELL 扫描", "err", err)
		return
	}

	found := 0
	for _, r := range records {
		var cnt int64
		database.DB.Model(&database.SmartWalletTrade{}).
			Where("wallet = ? AND token_address = ? AND action = 'SELL'", r.Wallet, r.TokenAddress).
			Count(&cnt)
		if cnt > 0 {
			continue
		}

		toBlock := r.MinBlock + 2000
		if toBlock > latest {
			toBlock = latest
		}
		if toBlock <= r.MinBlock {
			continue
		}

		query := ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(r.MinBlock + 1),
			ToBlock:   new(big.Int).SetUint64(toBlock),
			Addresses: []common.Address{common.HexToAddress(r.TokenAddress)},
			Topics: [][]common.Hash{
				{dex.TopicTransfer},
				{common.HexToHash(r.Wallet)}, // from = 聪明钱地址
			},
		}

		logs, err := e.rpcClient.Client.FilterLogs(context.Background(), query)
		if err != nil {
			continue
		}

		for _, lg := range logs {
			if len(lg.Topics) < 3 {
				continue
			}
			toAddr := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())

			_, isRouter := dex.DexRouters[toAddr]
			isPool := r.PoolAddress != "" && toAddr == strings.ToLower(r.PoolAddress)
			if !isRouter && !isPool {
				continue
			}

			price := estimatePriceAtBlock(e, r.TokenAddress, r.PoolAddress, lg.BlockNumber)

			database.RecordSmartWalletTradeWithPrice(
				r.Wallet, r.TokenAddress, "SELL", lg.BlockNumber, price, r.PoolAddress,
			)
			found++
			break
		}
	}

	slog.Info("✅ [Alpha Engine] SELL 轨迹扫描完毕", "new_sells_found", found, "pairs_scanned", len(records))
}

func estimatePriceAtBlock(e *AlphaEngine, tokenAddr, poolAddr string, blockNum uint64) float64 {
	if poolAddr == "" {
		return 0
	}

	to := common.HexToAddress(poolAddr)
	blockBig := new(big.Int).SetUint64(blockNum)

	result, err := e.rpcClient.Client.CallContract(context.Background(),
		ethereum.CallMsg{To: &to, Data: dex.SelectorGetReserves},
		blockBig,
	)
	if err != nil || len(result) < 64 {
		return 0
	}

	r0 := new(big.Int).SetBytes(result[0:32])
	r1 := new(big.Int).SetBytes(result[32:64])
	if r0.Sign() == 0 || r1.Sign() == 0 {
		return 0
	}

	wbnbNorm := strings.ToLower(dex.WBNB)
	tokenNorm := strings.ToLower(tokenAddr)

	var bnbReserve, tokenReserve *big.Int
	if tokenNorm < wbnbNorm {
		tokenReserve, bnbReserve = r0, r1
	} else {
		bnbReserve, tokenReserve = r0, r1
	}

	if tokenReserve.Sign() == 0 {
		return 0
	}

	price, _ := new(big.Float).Quo(
		new(big.Float).SetInt(bnbReserve),
		new(big.Float).SetInt(tokenReserve),
	).Float64()

	return price
}
