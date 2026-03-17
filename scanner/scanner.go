package scanner

import (
	"bot/config"
	"bot/database"
	"bot/dex"
	"bot/rpc"
	"bot/types"
	"bot/utils"
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

type Scanner struct {
	rpcClient *rpc.Client
	cache     *utils.Cache
	cfg       *config.Config
}

func NewScanner(rpcClient *rpc.Client, cache *utils.Cache, cfg *config.Config) *Scanner {
	return &Scanner{
		rpcClient: rpcClient,
		cache:     cache,
		cfg:       cfg,
	}
}

func (s *Scanner) ProcessNewPool(tokenAddr, poolAddr string, dexVer types.DEXVersion, feeTier uint32, isToken0WBNB bool, blockNum uint64, 
	fetchTokenString func(string, []byte) string,
	isSecureTokenV2 func(string) (bool, string),
	pollLiquidityBNB func(string, types.DEXVersion, bool) (float64, error),
	countSmartBuys func(string, string, uint64) (map[string]string, error),
	hasSmartMoneyBought func(map[string]string) (bool, int),
	isHolderDistributionSafe func(string, uint64) (bool, string),
	getBNBPrice func() float64,
	sendDiscordAlert func(types.TokenInfo),
	v3FeeLabel map[uint32]string,
) {
	if s.cache.Contains(poolAddr) {
		return
	}
	s.cache.Add(poolAddr)

	symbol := fetchTokenString(tokenAddr, dex.SelectorSymbol)
	name := fetchTokenString(tokenAddr, dex.SelectorName)

	feeStr := v3FeeLabel[feeTier]
	if feeStr == "" && feeTier > 0 {
		feeStr = fmt.Sprintf("%dbps", feeTier/100)
	}

	log := slog.With("dex", dexVer, "symbol", symbol, "token", tokenAddr, "pool", poolAddr, "block", blockNum)
	if dexVer == types.DEXv3 {
		log = log.With("fee", feeStr)
	}
	log.Info("🆕 新 Pool 创建，进入筛选流程...")

	// 1. 安全审计
	var pass bool
	var reason string
	for i := 0; i < 3; i++ {
		pass, reason = isSecureTokenV2(tokenAddr)
		if pass || !strings.Contains(reason, "请求失败") {
			break
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if !pass {
		log.Warn("⛔ 筛选淘汰 (安全审计)", "reason", reason)
		return
	}

	// 2. 流动性检查
	liqBNB, err := pollLiquidityBNB(poolAddr, dexVer, isToken0WBNB)
	if err != nil {
		log.Warn("流动性获取失败", "err", err)
		return
	}
	if liqBNB < s.cfg.LiquidityMinBNB {
		log.Info("⏭ 筛选淘汰 (流动性不足)", "liqBNB", fmt.Sprintf("%.2f", liqBNB), "min", s.cfg.LiquidityMinBNB)
		return
	}

	// 3. 聪明钱流入
	hitWallets, err := countSmartBuys(tokenAddr, poolAddr, blockNum)
	if err != nil {
		log.Warn("countSmartBuys 失败", "err", err)
		return
	}
	pass, highQualityHits := hasSmartMoneyBought(hitWallets)
	if !pass {
		log.Info("⏭ 筛选淘汰 (优质聪明钱不足)", "hit", highQualityHits)
		return
	}

	// 4. 持仓分析
	pass, reason = isHolderDistributionSafe(tokenAddr, blockNum)
	if !pass {
		log.Warn("⛔ 筛选淘汰 (持仓分析)", "reason", reason)
		return
	}

	info := types.TokenInfo{
		Address:      tokenAddr,
		Symbol:       symbol,
		Name:         name,
		PoolAddress:  poolAddr,
		LiquidityBNB: liqBNB,
		LiquidityUSD: liqBNB * getBNBPrice(),
		CreatedBlock: blockNum,
		CreatedAt:    time.Now(),
		SmartBuys:    len(hitWallets),
		HitWallets:   hitWallets,
		DEX:          dexVer,
		FeeTier:      feeTier,
	}

	sendDiscordAlert(info)
	database.SaveTokenToDB(info)
	log.Info("✅ 筛选全部通过", "smartBuys", len(hitWallets), "liqBNB", fmt.Sprintf("%.2f", liqBNB))
}

func (s *Scanner) FetchV2LiquidityBNB(ctx context.Context, pairAddr string, isToken0WBNB bool) (float64, error) {
	to := common.HexToAddress(pairAddr)
	result, err := rpc.Call(s.rpcClient, ctx, func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return s.rpcClient.Client.CallContract(c, ethereum.CallMsg{To: &to, Data: dex.SelectorGetReserves}, nil)
	})
	if err != nil {
		return 0, err
	}
	if len(result) < 64 {
		return 0, fmt.Errorf("getReserves 返回 %d bytes", len(result))
	}
	r0 := new(big.Int).SetBytes(result[0:32])
	r1 := new(big.Int).SetBytes(result[32:64])
	bnbR := r1
	if isToken0WBNB {
		bnbR = r0
	}
	f, _ := new(big.Float).Quo(new(big.Float).SetInt(bnbR), big.NewFloat(1e18)).Float64()
	return f, nil
}

func (s *Scanner) FetchV3LiquidityBNB(ctx context.Context, poolAddr string, isToken0WBNB bool) (float64, error) {
	wbnbAddr := common.HexToAddress(dex.WBNB)
	pool := common.HexToAddress(poolAddr)
	data := append([]byte{0x70, 0xa0, 0x82, 0x31}, common.LeftPadBytes(pool.Bytes(), 32)...)

	result, err := rpc.Call(s.rpcClient, ctx, func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return s.rpcClient.Client.CallContract(c, ethereum.CallMsg{To: &wbnbAddr, Data: data}, nil)
	})
	if err != nil {
		return 0, fmt.Errorf("WBNB balanceOf failed: %w", err)
	}
	if len(result) < 32 {
		return 0, fmt.Errorf("balanceOf 返回 %d bytes", len(result))
	}
	f, _ := new(big.Float).Quo(new(big.Float).SetInt(new(big.Int).SetBytes(result)), big.NewFloat(1e18)).Float64()
	return f, nil
}
