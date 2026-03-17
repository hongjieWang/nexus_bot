package main

import (
	"bot/alpha"
	"bot/api"
	"bot/config"
	"bot/database"
	"bot/dex"
	"bot/notifier"
	"bot/rpc"
	"bot/scanner"
	"bot/types"
	"bot/utils"
	"bot/watcher"
	"context"
	"encoding/json"
	_ "fmt"
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
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	appCache      *utils.Cache
	rpcClient     *rpc.Client
	pairSemaphore chan struct{}

	scoringEngine *alpha.ScoringEngine
	alphaEngine   *alpha.AlphaEngine
	appScanner    *scanner.Scanner
	appWatcher    *watcher.Watcher
	appConfig     *config.Config

	bnbPriceMu   sync.RWMutex
	cachedBNBUSD float64 = 600.0
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	appConfig = config.LoadConfig()
	appCache = utils.NewCache(10000)
	pairSemaphore = make(chan struct{}, 8)

	var err error
	rpcClient, err = rpc.NewClient(appConfig.RPCURL, appConfig.RPCRPS)
	if err != nil {
		slog.Error("RPC 连接失败", "err", err)
		os.Exit(1)
	}
	defer rpcClient.Close()

	database.InitDB(appConfig.MySQLDSN)

	// 初始化各个引擎
	scoringEngine = alpha.NewScoringEngine(rpcClient)
	alphaEngine = alpha.NewAlphaEngine(rpcClient)
	appScanner = scanner.NewScanner(rpcClient, appCache, appConfig)
	appWatcher = watcher.NewWatcher(rpcClient, appConfig, appCache, scoringEngine)

	updateBNBPrice()
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			updateBNBPrice()
		}
	}()

	alphaEngine.StartAll()
	appWatcher.StartAll(appConfig.WSSURL, processNewPool)

	go api.StartAPIServer()
	go startTradingEngine()

	slog.Info("🚀 BSC Sniper 2.0 模块化版本已启动")

	go runSubscription(appConfig.WSSURL, dex.PancakeV2Factory, dex.TopicV2PairCreated, handleV2PairCreated)
	runSubscription(appConfig.WSSURL, dex.PancakeV3Factory, dex.TopicV3PoolCreated, handleV3PoolCreated)
}

func runSubscription(wssURL, factory string, topic common.Hash, handler func(gethtypes.Log)) {
	for {
		ctx := context.Background()
		wsClient, err := ethclient.DialContext(ctx, wssURL)
		if err != nil {
			slog.Warn("WSS 连接失败，5秒后重连", "err", err)
			time.Sleep(5 * time.Second)
			continue
		}

		query := ethereum.FilterQuery{
			Addresses: []common.Address{common.HexToAddress(factory)},
			Topics:    [][]common.Hash{{topic}},
		}
		logCh := make(chan gethtypes.Log, 128)
		sub, err := wsClient.SubscribeFilterLogs(ctx, query, logCh)
		if err != nil {
			wsClient.Close()
			slog.Warn("WSS 订阅失败", "err", err)
			time.Sleep(5 * time.Second)
			continue
		}

		exit := false
		for !exit {
			select {
			case err := <-sub.Err():
				slog.Warn("WSS 订阅中断", "err", err)
				exit = true
			case lg := <-logCh:
				pairSemaphore <- struct{}{}
				go func(l gethtypes.Log) { defer func() { <-pairSemaphore }(); handler(l) }(lg)
			}
		}
		sub.Unsubscribe()
		wsClient.Close()
		time.Sleep(5 * time.Second)
	}
}

func handleV2PairCreated(lg gethtypes.Log) {
	if len(lg.Topics) < 3 || len(lg.Data) < 32 {
		return
	}
	wbnbNorm := strings.ToLower(dex.WBNB)
	t0 := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
	t1 := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())
	var tokenAddr string
	var isT0WBNB bool
	if t0 == wbnbNorm {
		tokenAddr, isT0WBNB = t1, true
	} else if t1 == wbnbNorm {
		tokenAddr, isT0WBNB = t0, false
	} else {
		return
	}
	poolAddr := strings.ToLower(common.BytesToAddress(lg.Data[:32]).Hex())
	processNewPool(tokenAddr, poolAddr, types.DEXv2, 0, isT0WBNB, lg.BlockNumber)
}

func handleV3PoolCreated(lg gethtypes.Log) {
	if len(lg.Topics) < 4 || len(lg.Data) < 64 {
		return
	}
	wbnbNorm := strings.ToLower(dex.WBNB)
	t0 := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
	t1 := strings.ToLower(common.BytesToAddress([]byte(lg.Topics[2].Hex())).Hex()) // 修复了之前的错误 Hex() 调用
	fee := uint32(new(big.Int).SetBytes(lg.Topics[3].Bytes()).Uint64())
	var tokenAddr string
	var isT0WBNB bool
	if t0 == wbnbNorm {
		tokenAddr, isT0WBNB = t1, true
	} else if t1 == wbnbNorm {
		tokenAddr, isT0WBNB = t0, false
	} else {
		return
	}
	poolAddr := strings.ToLower(common.BytesToAddress(lg.Data[32:64]).Hex())
	processNewPool(tokenAddr, poolAddr, types.DEXv3, fee, isT0WBNB, lg.BlockNumber)
}

func processNewPool(tokenAddr, poolAddr string, dexVer types.DEXVersion, fee uint32, isT0WBNB bool, block uint64) {
	appScanner.ProcessNewPool(tokenAddr, poolAddr, dexVer, fee, isT0WBNB, block,
		fetchTokenString, alpha.IsSecureTokenV2, pollLiquidityBNB,
		func(t, p string, b uint64) (map[string]string, error) {
			return scoringEngine.CountSmartBuys(t, p, b, appConfig)
		},
		scoringEngine.HasSmartMoneyBought,
		func(t string, b uint64) (bool, string) {
			return alpha.IsHolderDistributionSafe(rpcClient, t, b)
		},
		getBNBPrice, sendDiscordAlert, v3FeeLabel,
	)
}

func pollLiquidityBNB(addr string, ver types.DEXVersion, isT0 bool) (float64, error) {
	ctx := context.Background()
	if ver == types.DEXv2 {
		return appScanner.FetchV2LiquidityBNB(ctx, addr, isT0)
	}
	return appScanner.FetchV3LiquidityBNB(ctx, addr, isT0)
}

func getBNBPrice() float64 { bnbPriceMu.RLock(); defer bnbPriceMu.RUnlock(); return cachedBNBUSD }

func updateBNBPrice() {
	resp, err := http.Get("https://api.binance.com/api/v3/ticker/price?symbol=BNBUSDT")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var data struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
		p, _ := strconv.ParseFloat(data.Price, 64)
		bnbPriceMu.Lock()
		cachedBNBUSD = p
		bnbPriceMu.Unlock()
	}
}

func sendDiscordAlert(t types.TokenInfo) {
	notifier.SendDiscordAlert(appConfig, t, v3FeeLabel, getBNBPrice())
}

var v3FeeLabel = map[uint32]string{100: "0.01%", 500: "0.05%", 2500: "0.25%", 10000: "1%"}

func fetchTokenString(addr string, sel []byte) string {
	to := common.HexToAddress(addr)
	res, err := rpc.Call(rpcClient, context.Background(), func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return rpcClient.Client.CallContract(c, ethereum.CallMsg{To: &to, Data: sel}, nil)
	})
	if err != nil || len(res) < 64 {
		return "UNKNOWN"
	}
	l := new(big.Int).SetBytes(res[32:64]).Int64()
	if l <= 0 || l > 100 {
		return "UNKNOWN"
	}
	return string(res[64 : 64+l])
}

func refreshLeaderboard() error {
	// 临时占位，后续可接入 alpha.RefreshLeaderboard
	return nil
}
