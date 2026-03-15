package main

import (
	"bot/dex"
	"context"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SniperPosition 记录我们在 DEX 上的持仓状态
type SniperPosition struct {
	TokenAddress   string
	Symbol         string
	PoolAddress    string
	BuyBNBAmount   *big.Int
	TokenBalance   *big.Int
	EntryBNBValue  float64 // 买入时花费的 BNB 数量 (float64 方便计算)
	OpenedAt       time.Time
	TakeProfitMult float64 // 止盈倍数，比如 2.0 表示翻倍出本
	StopLossMult   float64 // 止损倍数，比如 0.8 表示跌 20% 止损
}

type SniperEngine struct {
	mu          sync.Mutex
	rpcHTTP     *ethclient.Client
	rpcWSS      *ethclient.Client
	wallet      *dex.Wallet
	pancakeV2   *dex.PancakeV2Client
	erc20Client *dex.ERC20Client
	positions   map[string]*SniperPosition
	enabled     bool // 是否开启实盘买入

	buyBnbAmount   *big.Int
	slippageBps    int64
	takeProfitMult float64
	stopLossMult   float64
}

var globalSniper *SniperEngine

func initSniperEngine(rpcHTTP *ethclient.Client, wssURL string) {
	enabled := strings.ToLower(GetConfig("DEX_SNIPER_ENABLED")) == "true"
	if !enabled {
		slog.Warn("🚫 DEX_SNIPER_ENABLED 未开启，仅记录信号不进行真实链上狙击")
	}

	hexKey := GetConfig("EXECUTION_PRIVATE_KEY")
	if enabled && hexKey == "" {
		slog.Error("EXECUTION_PRIVATE_KEY 未设置，无法启动实盘 Sniper")
		enabled = false
	}

	var wallet *dex.Wallet
	var pcv2 *dex.PancakeV2Client
	var erc20 *dex.ERC20Client
	var err error

	if enabled {
		// BSC ChainID = 56
		wallet, err = dex.NewWallet(rpcHTTP, hexKey, 56)
		if err != nil {
			slog.Error("初始化 Wallet 失败", "err", err)
			enabled = false
		} else {
			pcv2, err = dex.NewPancakeV2Client(wallet)
			if err != nil {
				slog.Error("初始化 PancakeV2Client 失败", "err", err)
				enabled = false
			}
			erc20, err = dex.NewERC20Client(wallet)
			if err != nil {
				slog.Error("初始化 ERC20Client 失败", "err", err)
				enabled = false
			}
		}
	}

	rpcWSS, err := ethclient.Dial(wssURL)
	if err != nil {
		slog.Error("Sniper WSS 客户端连接失败", "err", err)
	}

	// 读取交易配置
	buyAmountFloat := parseEnvFloat("DEX_BUY_BNB", 0.01)
	buyBnbAmount := new(big.Float).Mul(big.NewFloat(buyAmountFloat), big.NewFloat(1e18))
	buyBnbAmountInt, _ := buyBnbAmount.Int(nil)

	globalSniper = &SniperEngine{
		rpcHTTP:        rpcHTTP,
		rpcWSS:         rpcWSS,
		wallet:         wallet,
		pancakeV2:      pcv2,
		erc20Client:    erc20,
		positions:      make(map[string]*SniperPosition),
		enabled:        enabled,
		buyBnbAmount:   buyBnbAmountInt,
		slippageBps:    int64(parseEnvInt("DEX_SLIPPAGE_BPS", 1000)), // 默认 10% 滑点
		takeProfitMult: parseEnvFloat("DEX_TAKE_PROFIT_MULT", 2.0),   // 默认翻倍止盈
		stopLossMult:   parseEnvFloat("DEX_STOP_LOSS_MULT", 0.8),     // 默认跌 20% 止损
	}

	if enabled {
		slog.Info("🔫 DEX Sniper 引擎已启动", "buyBNB", buyAmountFloat, "tp", globalSniper.takeProfitMult, "sl", globalSniper.stopLossMult)
		go globalSniper.monitorPositions()
		go globalSniper.monitorRugPulls()
	}
}

// SniperBuyAndTrack 自动买入并加入监控 (Phase 1.1)
func SniperBuyAndTrack(info TokenInfo) {
	if globalSniper == nil || !globalSniper.enabled {
		return
	}
	if info.DEX != DEXv2 {
		slog.Info("⏭ 当前 Sniper 仅支持 V2，跳过", "dex", info.DEX)
		return
	}

	globalSniper.mu.Lock()
	if _, exists := globalSniper.positions[info.Address]; exists {
		globalSniper.mu.Unlock()
		return
	}
	globalSniper.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slog.Info("🔥 发起 Sniper 买入", "symbol", info.Symbol, "token", info.Address)

	// 1. 发起买入
	tx, err := globalSniper.pancakeV2.BuyTokenWithBNB(ctx, info.Address, globalSniper.buyBnbAmount, globalSniper.slippageBps)
	if err != nil {
		slog.Error("❌ Sniper 买入失败", "token", info.Address, "err", err)
		return
	}
	slog.Info("✅ Sniper 买入交易已广播", "hash", tx.Hash().Hex())

	// 2. 等待交易确认
	receipt, err := bindWaitMined(ctx, globalSniper.rpcHTTP, tx.Hash())
	if err != nil || receipt.Status != 1 {
		slog.Error("❌ Sniper 买入上链失败或回滚", "hash", tx.Hash().Hex(), "err", err)
		return
	}

	// 3. 获取余额
	balance, err := globalSniper.erc20Client.BalanceOf(ctx, info.Address, globalSniper.wallet.Address)
	if err != nil || balance.Sign() <= 0 {
		slog.Error("❌ 获取代币余额失败 (可能未成功买入)", "err", err)
		return
	}

	// 4. 发起授权 (Approve) 以便后续卖出
	slog.Info("🔓 开始自动授权 Router 卖出", "token", info.Address)
	maxInt := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	appTx, err := globalSniper.erc20Client.Approve(ctx, info.Address, globalSniper.pancakeV2.Router, maxInt)
	if err != nil {
		slog.Error("⚠️ 授权 Router 失败，后续可能无法自动卖出", "err", err)
	} else {
		slog.Info("✅ 授权交易已广播", "hash", appTx.Hash().Hex())
		// 我们不需要死等授权成功，可以后台慢慢等
	}

	// 5. 加入持仓追踪
	buyValF, _ := new(big.Float).Quo(new(big.Float).SetInt(globalSniper.buyBnbAmount), big.NewFloat(1e18)).Float64()

	pos := &SniperPosition{
		TokenAddress:   info.Address,
		Symbol:         info.Symbol,
		PoolAddress:    info.PoolAddress,
		BuyBNBAmount:   globalSniper.buyBnbAmount,
		TokenBalance:   balance,
		EntryBNBValue:  buyValF,
		OpenedAt:       time.Now(),
		TakeProfitMult: globalSniper.takeProfitMult,
		StopLossMult:   globalSniper.stopLossMult,
	}

	globalSniper.mu.Lock()
	globalSniper.positions[info.Address] = pos
	globalSniper.mu.Unlock()

	slog.Info("🎯 Sniper 持仓已建立并开始监控", "symbol", info.Symbol, "balance", balance.String())

	// TODO: 也可以记录到数据库的 trades_history 中
}

// monitorPositions 定时轮询价格，执行止盈止损 (Phase 1.2)
func (s *SniperEngine) monitorPositions() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		// 复制一份，避免长时间锁
		activeTokens := make([]*SniperPosition, 0, len(s.positions))
		for _, pos := range s.positions {
			activeTokens = append(activeTokens, pos)
		}
		s.mu.Unlock()

		for _, pos := range activeTokens {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			path := []common.Address{
				common.HexToAddress(pos.TokenAddress),
				common.HexToAddress(dex.WBNBAddress),
			}
			amounts, err := s.pancakeV2.GetAmountsOut(ctx, pos.TokenBalance, path)
			cancel()

			if err != nil {
				// 获取价格失败，可能池子被撤或流动性枯竭
				continue
			}

			currentBnbValInt := amounts[len(amounts)-1]
			currentBnbVal, _ := new(big.Float).Quo(new(big.Float).SetInt(currentBnbValInt), big.NewFloat(1e18)).Float64()

			hitTP := currentBnbVal >= pos.EntryBNBValue*pos.TakeProfitMult
			hitSL := currentBnbVal <= pos.EntryBNBValue*pos.StopLossMult

			if hitTP || hitSL {
				reason := "TP"
				if hitSL {
					reason = "SL"
				}
				slog.Info("⚠️ 触发自动平仓", "symbol", pos.Symbol, "reason", reason, "currentBNB", currentBnbVal, "entryBNB", pos.EntryBNBValue)
				s.executeSell(pos.TokenAddress, pos.TokenBalance, "Auto-"+reason)
			}
		}
	}
}

// monitorRugPulls 监听撤池子等恶性事件进行 Panic Sell (Phase 1.3)
func (s *SniperEngine) monitorRugPulls() {
	if s.rpcWSS == nil {
		return
	}

	for {
		s.mu.Lock()
		if len(s.positions) == 0 {
			s.mu.Unlock()
			time.Sleep(5 * time.Second)
			continue
		}

		var poolAddresses []common.Address
		for _, pos := range s.positions {
			poolAddresses = append(poolAddresses, common.HexToAddress(pos.PoolAddress))
		}
		s.mu.Unlock()

		// 监听 LP 代币发送到 Zero Address (撤池子) 或者 Sync 事件突然归零
		query := ethereum.FilterQuery{
			Addresses: poolAddresses,
			// 我们只监听 Sync 事件 (Topic 0 = Sync) 来快速判断流动性变化
			// Sync(uint112 reserve0, uint112 reserve1)
			Topics: [][]common.Hash{{crypto.Keccak256Hash([]byte("Sync(uint112,uint112)"))}},
		}

		ctx := context.Background()
		logCh := make(chan gethtypes.Log, 64)
		sub, err := s.rpcWSS.SubscribeFilterLogs(ctx, query, logCh)
		if err != nil {
			slog.Warn("PanicSell WSS 订阅失败，重试", "err", err)
			time.Sleep(5 * time.Second)
			continue
		}

		func() {
			defer sub.Unsubscribe()
			for {
				select {
				case err := <-sub.Err():
					slog.Warn("PanicSell WSS 断开", "err", err)
					return
				case lg := <-logCh:
					// 判断 Sync 的 reserve 是否骤降
					if len(lg.Data) >= 64 {
						r0 := new(big.Int).SetBytes(lg.Data[0:32])
						r1 := new(big.Int).SetBytes(lg.Data[32:64])

						// 如果任意一边储备量极低，疑似撤池子，触发 Panic Sell
						if r0.Cmp(big.NewInt(1000)) < 0 || r1.Cmp(big.NewInt(1000)) < 0 {
							poolHex := strings.ToLower(lg.Address.Hex())
							s.mu.Lock()
							var targetToken *SniperPosition
							for _, pos := range s.positions {
								if strings.ToLower(pos.PoolAddress) == poolHex {
									targetToken = pos
									break
								}
							}
							s.mu.Unlock()

							if targetToken != nil {
								slog.Warn("🚨 检测到疑似撤池子(Liquidity Removed)，执行 Panic Sell!", "symbol", targetToken.Symbol)
								s.executeSell(targetToken.TokenAddress, targetToken.TokenBalance, "Panic-Rug")
							}
						}
					}
				}
			}
		}()
	}
}

func (s *SniperEngine) executeSell(tokenAddress string, amount *big.Int, reason string) {
	s.mu.Lock()
	pos, exists := s.positions[tokenAddress]
	if !exists {
		s.mu.Unlock()
		return
	}
	// 立刻从 map 移除，防止重复卖出
	delete(s.positions, tokenAddress)
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 对于 Panic Sell，接受极大的滑点 (50% = 5000 bps)
	slip := s.slippageBps
	if strings.Contains(reason, "Panic") {
		slip = 5000
	}

	tx, err := s.pancakeV2.SellTokenForBNB(ctx, tokenAddress, amount, slip)
	if err != nil {
		slog.Error("❌ 卖出失败", "token", tokenAddress, "reason", reason, "err", err)
		return
	}
	slog.Info("✅ 卖出交易已广播", "token", tokenAddress, "reason", reason, "hash", tx.Hash().Hex())

	receipt, err := bindWaitMined(ctx, s.rpcHTTP, tx.Hash())
	if err != nil || receipt.Status != 1 {
		slog.Error("❌ 卖出上链失败或回滚", "hash", tx.Hash().Hex(), "err", err)
		return
	}

	slog.Info("💰 卖出成功已确认", "symbol", pos.Symbol, "reason", reason)
	// TODO: 记录到数据库
}

// bindWaitMined 辅助等待交易确认
func bindWaitMined(ctx context.Context, b *ethclient.Client, txHash common.Hash) (*gethtypes.Receipt, error) {
	queryTicker := time.NewTicker(time.Second)
	defer queryTicker.Stop()

	for {
		receipt, err := b.TransactionReceipt(ctx, txHash)
		if receipt != nil {
			return receipt, nil
		}
		if err != nil {
			slog.Debug("Receipt not found yet", "hash", txHash.Hex())
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-queryTicker.C:
		}
	}
}
