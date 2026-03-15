package main

import (
	"bot/api"
	"bot/config"
	"bot/database"
	"bot/types"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ================== 环境变量 ==================
func mustEnv(key string) string {
	v := config.GetConfig(key)
	if v == "" {
		slog.Error("环境变量未设置", "key", key)
		os.Exit(1)
	}
	return v
}

// ================== 常量 ==================
const (
	// ── 合约地址 ──────────────────────────────────────────
	pancakeV2Factory = "0xcA143Ce32Fe78f1f7019d7d551a6402fC5350c73"
	pancakeV3Factory = "0x0BFbCF9fa4f9C56B0F40a671Ad40E0805A091865"
	wbnb             = "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"

	// ── 外部接口 ──────────────────────────────────────────
	goPlusBase     = "https://api.gopluslabs.io/api/v1/token_security/56/"
	bnbPriceURL    = "https://api.binance.com/api/v3/ticker/price?symbol=BNBUSDT"
	leaderboardURL = "https://web3.binance.com/bapi/defi/v1/public/wallet-direct/market/leaderboard/query"

	// ── 刷新间隔 ──────────────────────────────────────────
	leaderboardPeriod  = "7d"
	leaderboardPages   = 4
	leaderboardRefresh = 1 * time.Hour

	// ── 扫描参数 ──────────────────────────────────────────
	transferScanBlocks = int64(200)
	liqPollInterval    = 2 * time.Second
	liqPollTimeout     = 30 * time.Second
	smartBuyMin        = 2
	liquidityMinBNB    = 3.0

	// ── 运行参数 ──────────────────────────────────────────
	httpTimeout        = 10 * time.Second
	seenTTL            = 30 * time.Minute
	cleanupEvery       = 5 * time.Minute
	reconnectDelay     = 5 * time.Second
	maxConcurrentPairs = 8

	// ── RPC 限速 ──────────────────────────────────────────
	// Chainstack 免费套餐约 25 RPS，留 20% 余量取 20 RPS
	// 可通过环境变量 RPC_RPS 覆盖
	defaultRPCRPS = 20
	// 令牌桶突发上限：设为 RPS 的 1/4，防止瞬间打完整桶
	rpcBurstFactor = 4
	// 429 退避：首次等待 500ms，每次翻倍，最多 8 次
	rpcRetryBase = 500 * time.Millisecond
	rpcRetryMax  = 8
)

// ── ABI 选择器 ────────────────────────────────────────────
var (
	selectorGetReserves = []byte{0x09, 0x02, 0xf1, 0xac} // getReserves()  V2
	selectorSlot0       = []byte{0x38, 0x50, 0xc7, 0xbd} // slot0()        V3
	selectorLiquidity   = []byte{0x1a, 0x68, 0x65, 0x02} // liquidity()    V3
	selectorSymbol      = []byte{0x95, 0xd8, 0x9b, 0x41} // symbol()
	selectorName        = []byte{0x06, 0xfd, 0xde, 0x03} // name()
)

// ── 事件 Topic ────────────────────────────────────────────
var (
	topicV2PairCreated = crypto.Keccak256Hash([]byte("PairCreated(address,address,address,uint256)"))
	topicV3PoolCreated = crypto.Keccak256Hash([]byte("PoolCreated(address,address,uint24,int24,address)"))
	topicTransfer      = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
)

// ── DEX Router 白名单 ─────────────────────────────────────
var dexRouters = map[string]struct{}{
	"0x10ed43c718714eb63d5aa57b78b54704e256024e": {}, // PancakeSwap V2
	"0x13f4ea83d0bd40e75c8222255bc855a974568dd4": {}, // PancakeSwap V3
	"0x1b81d678ffb9c0263b24a97847620c99d213eb14": {}, // PancakeSwap V3 (new)
	"0x6352a56caadc4f1e25cd6c75970fa768a3304e64": {}, // OpenOcean
	"0x1111111254eeb25477b68fb85ed929f73a960582": {}, // 1inch v5
	"0x6131b5fae19ea4f9d964eac0408e4408b66337b5": {}, // KyberSwap
}

// V3 手续费档位标签
var v3FeeLabel = map[uint32]string{
	100:   "0.01%",
	500:   "0.05%",
	2500:  "0.25%",
	10000: "1%",
}

var zeroAddress = strings.ToLower("0x0000000000000000000000000000000000000000")

// ================== 聪明钱 ==================
var (
	smartWallets   = make(map[string]string)
	smartWalletsMu sync.RWMutex
)

// ================== 数据结构 ==================
type leaderboardResp struct {
	Code string `json:"code"`
	Data struct {
		Data []struct {
			Address      string `json:"address"`
			AddressLabel string `json:"addressLabel"`
		} `json:"data"`
		Pages int `json:"pages"`
	} `json:"data"`
}

// ================== 全局变量 ==================
var (
	seen   = make(map[string]time.Time)
	seenMu sync.Mutex

	httpClient     *http.Client
	rpcHTTP        *ethclient.Client
	discordWebhook string

	bnbPriceMu   sync.RWMutex
	cachedBNBUSD float64 = 600.0

	pairSemaphore chan struct{}
	rpcLimiter    chan struct{}
)

// ================== RPC 限速器 ==================

func initRPCLimiter(rps int) {
	burst := rps / rpcBurstFactor
	if burst < 1 {
		burst = 1
	}
	rpcLimiter = make(chan struct{}, burst)
	interval := time.Second / time.Duration(rps)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			select {
			case rpcLimiter <- struct{}{}:
			default:
			}
		}
	}()
	for i := 0; i < burst; i++ {
		select {
		case rpcLimiter <- struct{}{}:
		default:
		}
	}
	slog.Info("RPC 限速器已启动", "rps", rps, "burst", burst)
}

func acquireRPC(ctx context.Context) error {
	select {
	case <-rpcLimiter:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// rpcCallWithRetry 遇到 429 自动退避重试，优先使用节点返回的等待时间
func rpcCallWithRetry[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	wait := rpcRetryBase
	for attempt := 0; attempt <= rpcRetryMax; attempt++ {
		if err := acquireRPC(ctx); err != nil {
			return zero, err
		}
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		errStr := err.Error()
		is429 := strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "-32005") ||
			strings.Contains(errStr, "exceeded") ||
			strings.Contains(errStr, "rate limit")

		if !is429 || attempt == rpcRetryMax {
			return zero, err
		}

		suggestedWait := parseTryAgainIn(errStr)
		if suggestedWait > 0 {
			wait = suggestedWait + 50*time.Millisecond
			slog.Warn("RPC 429 限速，退避重试", "attempt", attempt+1, "wait", wait, "source", "node_suggested")
		} else {
			slog.Warn("RPC 429 限速，退避重试", "attempt", attempt+1, "wait", wait, "source", "exponential_backoff")
		}

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return zero, ctx.Err()
		}
		if suggestedWait == 0 {
			wait *= 2
		}
	}
	return zero, fmt.Errorf("RPC 重试 %d 次后仍失败", rpcRetryMax)
}

// parseTryAgainIn 兼容转义/非转义两种 JSON 引号格式
func parseTryAgainIn(errStr string) time.Duration {
	for _, key := range []string{`"try_again_in":"`, `\"try_again_in\":\"`} {
		idx := strings.Index(errStr, key)
		if idx < 0 {
			continue
		}
		start := idx + len(key)
		rest := errStr[start:]
		endMark := `"`
		if key[0] == '\\' {
			endMark = `\"`
		}
		end := strings.Index(rest, endMark)
		if end < 0 {
			continue
		}
		d, err := time.ParseDuration(rest[:end])
		if err == nil && d > 0 {
			return d
		}
	}
	return 0
}

// ================== 入口 ==================
func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// 全局最先初始化数据库
	database.InitDB()

	// Phase 4.2 检查是否通过命令行参数启动回测模式
	if len(os.Args) > 1 && os.Args[1] == "backtest" {
		slog.Info("🛠️ 进入离线回测模式 (Backtest Mode)")
		strats, err := LoadStrategies("strategies.json")
		if err != nil || len(strats) == 0 {
			slog.Error("加载策略失败，无法回测", "err", err)
			os.Exit(1)
		}

		for _, s := range strats {
			// 对每个加载的策略运行 1500 根 15m K线的回测，初始资金 50U
			RunBacktest(s, "BNBUSDT", "15m", 1500, 50.0)
		}
		os.Exit(0)
	}

	discordWebhook = mustEnv("DISCORD_WEBHOOK")
	bscHTTP := mustEnv("BSC_HTTP_RPC")
	bscWSS := mustEnv("BSC_WSS_RPC")

	pairSemaphore = make(chan struct{}, maxConcurrentPairs)
	httpClient = &http.Client{Timeout: httpTimeout}

	rps := defaultRPCRPS
	if v := os.Getenv("RPC_RPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rps = n
		}
	}
	initRPCLimiter(rps)

	var err error
	rpcHTTP, err = ethclient.Dial(bscHTTP)
	if err != nil {
		slog.Error("HTTP RPC 连接失败", "err", err)
		os.Exit(1)
	}
	defer rpcHTTP.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	blockNum, err := rpcHTTP.BlockNumber(ctx)
	cancel()
	if err != nil {
		slog.Error("HTTP RPC 不可用", "err", err)
		os.Exit(1)
	}
	slog.Info("HTTP RPC 已连接", "block", blockNum)

	if err := pingDiscord(); err != nil {
		slog.Error("Discord Webhook 不可用", "err", err)
		os.Exit(1)
	}
	slog.Info("Discord Webhook 已就绪")

	// BNB 价格
	updateBNBPrice()
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for range t.C {
			updateBNBPrice()
		}
	}()

	// 聪明钱排行榜
	if err := refreshLeaderboard(); err != nil {
		slog.Warn("排行榜初始化失败（将继续运行）", "err", err)
	}
	go func() {
		t := time.NewTicker(leaderboardRefresh)
		defer t.Stop()
		for range t.C {
			if err := refreshLeaderboard(); err != nil {
				slog.Warn("排行榜刷新失败", "err", err)
			}
		}
	}()

	go cleanupSeen()

	// 初始化 DEX Sniper 引擎
	initSniperEngine(rpcHTTP, bscWSS)

	// 启动 Phase 3: 聪明钱分析引擎
	startAlphaEngine()

	// 启动 Phase 5: React Dashboard API 服务器
	go api.StartAPIServer()

	// 启动交易引擎 (支持模拟测试或真实下单)
	go startTradingEngine()

	slog.Info("WSS 实时订阅启动",
		"V2Factory", pancakeV2Factory,
		"V3Factory", pancakeV3Factory,
	)

	// V2 订阅独立 goroutine，V3 运行在主循环
	go func() {
		for {
			if err := runV2Subscription(bscWSS); err != nil {
				slog.Warn("V2 WSS 中断，即将重连", "err", err, "delay", reconnectDelay)
			}
			time.Sleep(reconnectDelay)
		}
	}()

	for {
		if err := runV3Subscription(bscWSS); err != nil {
			slog.Warn("V3 WSS 中断，即将重连", "err", err, "delay", reconnectDelay)
		}
		time.Sleep(reconnectDelay)
	}
}

// ================== V2 WSS 订阅 ==================
func runV2Subscription(bscWSS string) error {
	ctx := context.Background()
	rpcWSS, err := ethclient.DialContext(ctx, bscWSS)
	if err != nil {
		return fmt.Errorf("V2 WSS 连接失败: %w", err)
	}
	defer rpcWSS.Close()

	query := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(pancakeV2Factory)},
		Topics:    [][]common.Hash{{topicV2PairCreated}},
	}
	logCh := make(chan gethtypes.Log, 64)
	sub, err := rpcWSS.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil {
		return fmt.Errorf("V2 SubscribeFilterLogs 失败: %w", err)
	}
	defer sub.Unsubscribe()
	slog.Info("✅ V2 PairCreated 订阅成功")

	for {
		select {
		case err := <-sub.Err():
			return fmt.Errorf("V2 订阅错误: %w", err)
		case lg := <-logCh:
			pairSemaphore <- struct{}{}
			go func(l gethtypes.Log) {
				defer func() { <-pairSemaphore }()
				handleV2PairCreated(l)
			}(lg)
		}
	}
}

// ================== V3 WSS 订阅 ==================
func runV3Subscription(bscWSS string) error {
	ctx := context.Background()
	rpcWSS, err := ethclient.DialContext(ctx, bscWSS)
	if err != nil {
		return fmt.Errorf("V3 WSS 连接失败: %w", err)
	}
	defer rpcWSS.Close()

	query := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(pancakeV3Factory)},
		Topics:    [][]common.Hash{{topicV3PoolCreated}},
	}
	logCh := make(chan gethtypes.Log, 64)
	sub, err := rpcWSS.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil {
		return fmt.Errorf("V3 SubscribeFilterLogs 失败: %w", err)
	}
	defer sub.Unsubscribe()
	slog.Info("✅ V3 PoolCreated 订阅成功")

	for {
		select {
		case err := <-sub.Err():
			return fmt.Errorf("V3 订阅错误: %w", err)
		case lg := <-logCh:
			pairSemaphore <- struct{}{}
			go func(l gethtypes.Log) {
				defer func() { <-pairSemaphore }()
				handleV3PoolCreated(l)
			}(lg)
		}
	}
}

// ================== V2 事件处理 ==================
// PairCreated(address indexed token0, address indexed token1, address pair, uint256)
// Topics[1]=token0  Topics[2]=token1  Data[0:32]=pairAddr
func handleV2PairCreated(lg gethtypes.Log) {
	if len(lg.Topics) < 3 || len(lg.Data) < 32 {
		return
	}
	wbnbNorm := strings.ToLower(wbnb)
	token0 := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
	token1 := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())

	var tokenAddr string
	var isToken0WBNB bool
	switch {
	case token0 == wbnbNorm:
		tokenAddr, isToken0WBNB = token1, true
	case token1 == wbnbNorm:
		tokenAddr, isToken0WBNB = token0, false
	default:
		return
	}
	poolAddr := strings.ToLower(common.BytesToAddress(lg.Data[:32]).Hex())
	processNewPool(tokenAddr, poolAddr, types.DEXv2, 0, isToken0WBNB, lg.BlockNumber)
}

// ================== V3 事件处理 ==================
// PoolCreated(address indexed token0, address indexed token1, uint24 indexed fee, int24 tickSpacing, address pool)
// Topics[1]=token0  Topics[2]=token1  Topics[3]=fee
// Data[0:32]=tickSpacing  Data[32:64]=poolAddr
func handleV3PoolCreated(lg gethtypes.Log) {
	if len(lg.Topics) < 4 || len(lg.Data) < 64 {
		return
	}
	wbnbNorm := strings.ToLower(wbnb)
	token0 := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
	token1 := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())

	feeBig := new(big.Int).SetBytes(lg.Topics[3].Bytes())
	feeTier := uint32(feeBig.Uint64())

	var tokenAddr string
	var isToken0WBNB bool
	switch {
	case token0 == wbnbNorm:
		tokenAddr, isToken0WBNB = token1, true
	case token1 == wbnbNorm:
		tokenAddr, isToken0WBNB = token0, false
	default:
		return
	}
	poolAddr := strings.ToLower(common.BytesToAddress(lg.Data[32:64]).Hex())
	processNewPool(tokenAddr, poolAddr, types.DEXv3, feeTier, isToken0WBNB, lg.BlockNumber)
}

// ================== 公共处理逻辑 ==================
func processNewPool(tokenAddr, poolAddr string, dex types.DEXVersion, feeTier uint32, isToken0WBNB bool, blockNum uint64) {
	seenMu.Lock()
	if last, exists := seen[tokenAddr]; exists && time.Since(last) < seenTTL {
		seenMu.Unlock()
		return
	}
	seen[tokenAddr] = time.Now()
	seenMu.Unlock()

	symbol := fetchTokenString(tokenAddr, selectorSymbol)
	name := fetchTokenString(tokenAddr, selectorName)

	feeStr := v3FeeLabel[feeTier]
	if feeStr == "" && feeTier > 0 {
		feeStr = fmt.Sprintf("%dbps", feeTier/100)
	}

	log := slog.With("dex", dex, "symbol", symbol, "token", tokenAddr, "pool", poolAddr, "block", blockNum)
	if dex == types.DEXv3 {
		log = log.With("fee", feeStr)
	}
	log.Info("🆕 新 Pool 创建")

	liqBNB, err := pollLiquidityBNB(poolAddr, dex, isToken0WBNB)
	if err != nil {
		log.Warn("流动性获取失败", "err", err)
		return
	}
	if liqBNB < liquidityMinBNB {
		log.Info("⏭ 流动性不足，跳过", "liqBNB", fmt.Sprintf("%.2f", liqBNB), "min", liquidityMinBNB)
		return
	}

	hitWallets, err := countSmartBuys(tokenAddr, poolAddr, blockNum)
	if err != nil {
		log.Warn("countSmartBuys 失败", "err", err)
		return
	}
	if len(hitWallets) < smartBuyMin {
		log.Info("⏭ 聪明钱不足，跳过", "hit", len(hitWallets), "min", smartBuyMin)
		return
	}

	pass, reason := isQualityGoPlus(tokenAddr)
	if !pass {
		log.Warn("⛔ GoPlus 未通过", "reason", reason)
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
		DEX:          dex,
		FeeTier:      feeTier,
	}

	sendDiscordAlert(info)
	database.SaveTokenToDB(info)
	log.Info("✅ 预警已发送并入库", "smartBuys", len(hitWallets), "liqBNB", fmt.Sprintf("%.2f", liqBNB))

	// 自动狙击 (如果满足安全与配置条件，由引擎内部进行实盘执行)
	go SniperBuyAndTrack(info)
}

// ================== 流动性轮询 ==================
func pollLiquidityBNB(poolAddr string, dex types.DEXVersion, isToken0WBNB bool) (float64, error) {
	deadline := time.Now().Add(liqPollTimeout)
	for time.Now().Before(deadline) {
		var liq float64
		var err error
		if dex == types.DEXv2 {
			liq, err = fetchV2LiquidityBNB(poolAddr, isToken0WBNB)
		} else {
			liq, err = fetchV3LiquidityBNB(poolAddr, isToken0WBNB)
		}
		if err == nil && liq > 0 {
			return liq, nil
		}
		time.Sleep(liqPollInterval)
	}
	if dex == types.DEXv2 {
		return fetchV2LiquidityBNB(poolAddr, isToken0WBNB)
	}
	return fetchV3LiquidityBNB(poolAddr, isToken0WBNB)
}

// ================== V2 流动性：getReserves() ==================
func fetchV2LiquidityBNB(pairAddr string, isToken0WBNB bool) (float64, error) {
	to := common.HexToAddress(pairAddr)
	result, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return rpcHTTP.CallContract(c, ethereum.CallMsg{To: &to, Data: selectorGetReserves}, nil)
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

// ================== V3 流动性：slot0() + liquidity() ==================
// virtualBNB = L × 2^96 / sqrtPriceX96  (token0==WBNB)
// virtualBNB = L × sqrtPriceX96 / 2^96  (token1==WBNB)
func fetchV3LiquidityBNB(poolAddr string, isToken0WBNB bool) (float64, error) {
	to := common.HexToAddress(poolAddr)

	slot0Result, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return rpcHTTP.CallContract(c, ethereum.CallMsg{To: &to, Data: selectorSlot0}, nil)
	})
	if err != nil {
		return 0, fmt.Errorf("slot0 call: %w", err)
	}
	if len(slot0Result) < 32 {
		return 0, fmt.Errorf("slot0 返回 %d bytes", len(slot0Result))
	}
	sqrtPriceX96 := new(big.Int).SetBytes(slot0Result[0:32])
	if sqrtPriceX96.Sign() == 0 {
		return 0, fmt.Errorf("pool 尚未初始化（sqrtPriceX96=0）")
	}

	liqResult, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return rpcHTTP.CallContract(c, ethereum.CallMsg{To: &to, Data: selectorLiquidity}, nil)
	})
	if err != nil {
		return 0, fmt.Errorf("liquidity call: %w", err)
	}
	if len(liqResult) < 32 {
		return 0, fmt.Errorf("liquidity 返回 %d bytes", len(liqResult))
	}
	liquidity := new(big.Int).SetBytes(liqResult[0:32])
	if liquidity.Sign() == 0 {
		return 0, fmt.Errorf("当前档位流动性为 0")
	}

	q96 := new(big.Int).Lsh(big.NewInt(1), 96)
	var virtualBNBWei *big.Int
	if isToken0WBNB {
		num := new(big.Int).Mul(liquidity, q96)
		virtualBNBWei = new(big.Int).Div(num, sqrtPriceX96)
	} else {
		num := new(big.Int).Mul(liquidity, sqrtPriceX96)
		virtualBNBWei = new(big.Int).Div(num, q96)
	}

	bnb, _ := new(big.Float).Quo(new(big.Float).SetInt(virtualBNBWei), big.NewFloat(1e18)).Float64()
	return bnb, nil
}

// ================== 聪明钱扫描 ==================
func countSmartBuys(tokenAddr, poolAddr string, createdBlock uint64) (map[string]string, error) {
	latest, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) (uint64, error) {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return rpcHTTP.BlockNumber(c)
	})
	if err != nil {
		return nil, fmt.Errorf("获取最新块: %w", err)
	}

	toBlock := createdBlock + uint64(transferScanBlocks)
	if toBlock > latest {
		toBlock = latest
	}

	smartWalletsMu.RLock()
	var toTopics []common.Hash
	for addr := range smartWallets {
		toTopics = append(toTopics, common.HexToHash(addr))
	}
	smartWalletsMu.RUnlock()

	if len(toTopics) == 0 {
		return nil, fmt.Errorf("聪明钱列表为空")
	}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(createdBlock),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Addresses: []common.Address{common.HexToAddress(tokenAddr)},
		Topics:    [][]common.Hash{{topicTransfer}, nil, toTopics},
	}

	logs, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) ([]gethtypes.Log, error) {
		c, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		return rpcHTTP.FilterLogs(c, query)
	})
	if err != nil {
		return nil, fmt.Errorf("FilterLogs(#%d~#%d): %w", createdBlock, toBlock, err)
	}

	tokenNorm := strings.ToLower(tokenAddr)
	hitWallets := make(map[string]string)

	for _, lg := range logs {
		if len(lg.Topics) < 3 {
			continue
		}
		fromAddr := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
		toAddr := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())

		if fromAddr == zeroAddress || fromAddr == tokenNorm {
			continue
		}

		_, isRouter := dexRouters[fromAddr]
		source := "unknown"
		if isRouter {
			source = "router"
		} else if fromAddr == strings.ToLower(poolAddr) {
			source = "pool"
		}

		smartWalletsMu.RLock()
		label, ok := smartWallets[toAddr]
		smartWalletsMu.RUnlock()

		if ok {
			if _, already := hitWallets[toAddr]; !already {
				hitWallets[toAddr] = label

				// 从 Log Data 中解析转账金额 (uint256)
				amount := new(big.Int).SetBytes(lg.Data)
				// 暂时记为 0 USD，后续可根据 Token 价格计算
				amountUSD := 0.0 
				_ = amount // 避免未使用变量警告

				// 记录交互轨迹到数据库
				database.RecordSmartWalletTrade(toAddr, tokenAddr, "BUY", amountUSD, lg.BlockNumber)

				slog.Info("🧠 聪明钱命中",
					"token", tokenAddr, "wallet", toAddr,
					"label", label, "source", source, "block", lg.BlockNumber,
				)
			}
		}
	}

	slog.Info("📊 聪明钱扫描完成",
		"token", tokenAddr,
		"range", fmt.Sprintf("#%d~#%d", createdBlock, toBlock),
		"hit", len(hitWallets),
	)
	return hitWallets, nil
}

// ================== GoPlus 安全检查 ==================
func isQualityGoPlus(addr string) (bool, string) {
	resp, err := httpClient.Get(goPlusBase + addr)
	if err != nil {
		return false, fmt.Sprintf("GoPlus 请求失败: %v", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false, "GoPlus JSON 解析失败"
	}
	result, ok := data["result"].(map[string]interface{})
	if !ok {
		return false, "GoPlus result 字段异常"
	}
	r, ok := result[strings.ToLower(addr)].(map[string]interface{})
	if !ok {
		r, ok = result[addr].(map[string]interface{})
		if !ok {
			return false, "GoPlus 未找到 token 数据"
		}
	}

	if toInt(r["is_honeypot"]) == 1 {
		return false, "蜜罐合约"
	}
	if toInt(r["is_open_source"]) != 1 {
		return false, "合约未开源"
	}
	if toInt(r["owner_change_balance"]) == 1 {
		return false, "owner 可修改余额"
	}
	if buyTax := toFloat(r["buy_tax"]); buyTax > 0.1 {
		return false, fmt.Sprintf("买税过高: %.0f%%", buyTax*100)
	}
	if sellTax := toFloat(r["sell_tax"]); sellTax > 0.1 {
		return false, fmt.Sprintf("卖税过高: %.0f%%", sellTax*100)
	}
	return true, ""
}

// ================== RPC 辅助 ==================
func fetchTokenString(tokenAddr string, selector []byte) string {
	to := common.HexToAddress(tokenAddr)
	result, err := rpcCallWithRetry(context.Background(), func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return rpcHTTP.CallContract(c, ethereum.CallMsg{To: &to, Data: selector}, nil)
	})
	if err != nil || len(result) < 64 {
		return "UNKNOWN"
	}
	strLen := new(big.Int).SetBytes(result[32:64]).Int64()
	if strLen <= 0 || int64(len(result)) < 64+strLen {
		return "UNKNOWN"
	}
	return strings.TrimRight(string(result[64:64+strLen]), "\x00")
}

// ================== 排行榜 ==================
func refreshLeaderboard() error {
	newWallets := make(map[string]string)
	totalPages, err := fetchLeaderboardPage(1, newWallets)
	if err != nil {
		return err
	}
	maxPages := totalPages
	if maxPages > leaderboardPages {
		maxPages = leaderboardPages
	}
	for page := 2; page <= maxPages; page++ {
		if _, err := fetchLeaderboardPage(page, newWallets); err != nil {
			slog.Warn("排行榜分页失败", "page", page, "err", err)
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Phase 3: 同步抓取的聪明钱到数据库
	database.SyncSmartWalletsToDB(newWallets)

	// 从数据库中加载过滤掉 MEV 且分数合格的聪明钱
	filteredWallets := database.GetFilteredSmartWallets()
	if filteredWallets == nil || len(filteredWallets) == 0 {
		filteredWallets = newWallets // 降级处理
	}

	smartWalletsMu.Lock()
	smartWallets = filteredWallets
	smartWalletsMu.Unlock()

	slog.Info("排行榜刷新并过滤完成", "raw_count", len(newWallets), "filtered_count", len(filteredWallets), "pages", maxPages)
	return nil
}

func fetchLeaderboardPage(page int, dest map[string]string) (int, error) {
	url := fmt.Sprintf("%s?tag=ALL&pageNo=%d&pageSize=25&sortBy=0&orderBy=0&period=%s&chainId=56",
		leaderboardURL, page, leaderboardPeriod)
	resp, err := httpClient.Get(url)
	if err != nil {
		return 0, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	var result leaderboardResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if result.Code != "000000" {
		return 0, fmt.Errorf("API 返回错误码: %s", result.Code)
	}
	for _, item := range result.Data.Data {
		addr := strings.ToLower(item.Address)
		label := item.AddressLabel
		if label == "" {
			label = addr[:8] + "..."
		}
		dest[addr] = label
	}
	return result.Data.Pages, nil
}

// ================== BNB 价格 ==================
func updateBNBPrice() {
	resp, err := httpClient.Get(bnbPriceURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var data struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}
	if p, err := strconv.ParseFloat(data.Price, 64); err == nil && p > 0 {
		bnbPriceMu.Lock()
		cachedBNBUSD = p
		bnbPriceMu.Unlock()
		slog.Info("BNB 价格更新", "price", fmt.Sprintf("$%.2f", p))
	}
}

func getBNBPrice() float64 {
	bnbPriceMu.RLock()
	defer bnbPriceMu.RUnlock()
	return cachedBNBUSD
}

// ================== Discord 推送 ==================
type (
	discordEmbed struct {
		Title     string              `json:"title"`
		Color     int                 `json:"color"`
		Fields    []discordEmbedField `json:"fields"`
		Footer    discordFooter       `json:"footer"`
		Timestamp string              `json:"timestamp"`
	}
	discordEmbedField struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Inline bool   `json:"inline"`
	}
	discordFooter struct {
		Text string `json:"text"`
	}
	discordPayload struct {
		Username  string         `json:"username"`
		AvatarURL string         `json:"avatar_url"`
		Embeds    []discordEmbed `json:"embeds"`
	}
)

func pingDiscord() error {
	body, _ := json.Marshal(discordPayload{Username: "BSC Scanner"})
	resp, err := httpClient.Post(discordWebhook+"?wait=true", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 && resp.StatusCode != 400 {
		return fmt.Errorf("Discord 返回 %d", resp.StatusCode)
	}
	return nil
}

func sendDiscordAlert(t types.TokenInfo) {
	ageMin := int64(time.Since(t.CreatedAt).Minutes())

	walletLines := make([]string, 0, len(t.HitWallets))
	for addr, label := range t.HitWallets {
		short := addr[:6] + "..." + addr[len(addr)-4:]
		walletLines = append(walletLines,
			fmt.Sprintf("[%s](https://bscscan.com/address/%s) `%s`", label, addr, short),
		)
	}
	walletDetail := strings.Join(walletLines, "\n")
	if walletDetail == "" {
		walletDetail = "—"
	}

	dexLabel := fmt.Sprintf("PancakeSwap %s", t.DEX)
	if t.DEX == types.DEXv3 && t.FeeTier > 0 {
		if label, ok := v3FeeLabel[t.FeeTier]; ok {
			dexLabel += fmt.Sprintf(" (%s)", label)
		}
	}

	embed := discordEmbed{
		Title: fmt.Sprintf("🔥 聪明钱链上狙击预警 [%s]", dexLabel),
		Color: 0xFF4500,
		Fields: []discordEmbedField{
			{Name: "🪙 币种", Value: fmt.Sprintf("`%s` (%s)", t.Symbol, t.Name), Inline: true},
			{Name: "⏱ 币龄", Value: fmt.Sprintf("%d 分钟", ageMin), Inline: true},
			{Name: "🧠 聪明钱", Value: fmt.Sprintf("**%d 个地址**", t.SmartBuys), Inline: true},
			{Name: "💧 流动性", Value: fmt.Sprintf("%.2f BNB ($%.0f)", t.LiquidityBNB, t.LiquidityUSD), Inline: true},
			{Name: "📡 DEX", Value: dexLabel, Inline: true},
			{Name: "🏦 创建块", Value: fmt.Sprintf("#%d", t.CreatedBlock), Inline: true},
			{Name: "📋 Token", Value: fmt.Sprintf("`%s`", t.Address), Inline: false},
			{Name: "🔗 Pool", Value: fmt.Sprintf("`%s`", t.PoolAddress), Inline: false},
			{Name: "🧠 命中钱包", Value: walletDetail, Inline: false},
			{
				Name: "🔍 链接",
				Value: fmt.Sprintf(
					"[DexScreener](https://dexscreener.com/bsc/%s)  ·  "+
						"[GoPlus](https://gopluslabs.io/token-security/56/%s)  ·  "+
						"[BscScan](https://bscscan.com/token/%s)",
					t.Address, t.Address, t.Address,
				),
				Inline: false,
			},
		},
		Footer:    discordFooter{Text: fmt.Sprintf("BSC | %s | WSS 实时订阅", dexLabel)},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(discordPayload{
		Username:  "BSC Chain Scanner",
		AvatarURL: "https://assets.pancakeswap.finance/web/favicon/favicon-32x32.png",
		Embeds:    []discordEmbed{embed},
	})
	if err != nil {
		slog.Error("Discord payload 序列化失败", "err", err)
		return
	}
	resp, err := httpClient.Post(discordWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("Discord 发送失败", "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		slog.Error("Discord 返回异常状态码", "status", resp.StatusCode)
	}
}

// ================== 工具函数 ==================
func cleanupSeen() {
	ticker := time.NewTicker(cleanupEvery)
	defer ticker.Stop()
	for range ticker.C {
		seenMu.Lock()
		for addr, t := range seen {
			if time.Since(t) > seenTTL {
				delete(seen, addr)
			}
		}
		n := len(seen)
		seenMu.Unlock()
		slog.Info("seen 清理完成", "remaining", n)
	}
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	}
	return -1
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}
