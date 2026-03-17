package alpha

import (
	"bot/dex"
	"bot/rpc"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

const goPlusBase = "https://api.gopluslabs.io/api/v1/token_security/56?contract_addresses="

func IsSecureTokenV2(addr string) (bool, string) {
	resp, err := http.Get(goPlusBase + addr)
	if err != nil {
		return false, fmt.Sprintf("GoPlus 请求失败: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  map[string]struct {
			IsHoneypot   string `json:"is_honeypot"`
			BuyTax       string `json:"buy_tax"`
			SellTax      string `json:"sell_tax"`
			CannotSell   string `json:"cannot_sell_all"`
			OwnerBalance string `json:"owner_balance"`
			HiddenOwner  string `json:"hidden_owner"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "解析 GoPlus 失败"
	}

	data, ok := result.Result[strings.ToLower(addr)]
	if !ok {
		return false, "GoPlus 未返回该代币 data"
	}

	if data.IsHoneypot == "1" {
		return false, "确认是蜜罐 (Honeypot)"
	}
	if data.CannotSell == "1" {
		return false, "无法全部卖出"
	}
	if data.HiddenOwner == "1" {
		return false, "存在隐藏权限 (Hidden Owner)"
	}

	return true, "安全审计通过"
}

func IsHolderDistributionSafe(rpcClient *rpc.Client, tokenAddr string, createdBlock uint64) (bool, string) {
	latest, err := rpcClient.Client.BlockNumber(context.Background())
	if err != nil {
		return false, "无法获取最新区块"
	}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(createdBlock),
		ToBlock:   new(big.Int).SetUint64(latest),
		Addresses: []common.Address{common.HexToAddress(tokenAddr)},
		Topics:    [][]common.Hash{{dex.TopicTransfer}},
	}
	
	logs, err := rpc.Call(rpcClient, context.Background(), func(ctx context.Context) ([]gethtypes.Log, error) {
		return rpcClient.Client.FilterLogs(ctx, query)
	})
	if err != nil {
		return false, "FilterLogs 失败"
	}

	balances := make(map[string]*big.Int)
	totalSupply := big.NewInt(0)

	for _, lg := range logs {
		if len(lg.Topics) < 3 {
			continue
		}
		from := common.BytesToAddress(lg.Topics[1].Bytes())
		to := common.BytesToAddress(lg.Topics[2].Bytes())
		amount := new(big.Int).SetBytes(lg.Data)

		if from.Hex() == strings.ToLower(dex.ZeroAddress.Hex()) {
			totalSupply.Add(totalSupply, amount)
		}

		if _, ok := balances[from.Hex()]; !ok {
			balances[from.Hex()] = big.NewInt(0)
		}
		if _, ok := balances[to.Hex()]; !ok {
			balances[to.Hex()] = big.NewInt(0)
		}

		balances[from.Hex()].Sub(balances[from.Hex()], amount)
		balances[to.Hex()].Add(balances[to.Hex()], amount)
	}

	if totalSupply.Sign() == 0 {
		return true, "新币无供应量数据"
	}

	type holder struct {
		addr    string
		balance *big.Int
	}
	var sortedHolders []holder
	for addr, bal := range balances {
		if bal.Sign() > 0 && addr != strings.ToLower(dex.ZeroAddress.Hex()) {
			sortedHolders = append(sortedHolders, holder{addr, bal})
		}
	}

	sort.Slice(sortedHolders, func(i, j int) bool {
		return sortedHolders[i].balance.Cmp(sortedHolders[j].balance) > 0
	})

	top10Sum := big.NewInt(0)
	for i := 0; i < 10 && i < len(sortedHolders); i++ {
		top10Sum.Add(top10Sum, sortedHolders[i].balance)
	}

	ratio, _ := new(big.Float).Quo(new(big.Float).SetInt(top10Sum), new(big.Float).SetInt(totalSupply)).Float64()
	if ratio > 0.8 {
		return false, fmt.Sprintf("前10持仓过高: %.1f%%", ratio*100)
	}

	return true, fmt.Sprintf("前10持仓合理: %.1f%%", ratio*100)
}

func FindV2Pool(rpcClient *rpc.Client, tokenAddr string) (string, bool, error) {
	factoryAddr := common.HexToAddress(dex.PancakeV2Factory)
	token := common.HexToAddress(tokenAddr)
	wbnbAddr := common.HexToAddress(dex.WBNB)

	getPairSel := []byte{0xe6, 0xa4, 0x39, 0x05}
	input := make([]byte, 4+64)
	copy(input[:4], getPairSel)
	copy(input[4+12:36], token.Bytes())
	copy(input[36+12:68], wbnbAddr.Bytes())

	result, err := rpc.Call(rpcClient, context.Background(), func(ctx context.Context) ([]byte, error) {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return rpcClient.Client.CallContract(c, ethereum.CallMsg{To: &factoryAddr, Data: input}, nil)
	})
	if err != nil || len(result) < 32 {
		return "", false, fmt.Errorf("getPair 失败: %w", err)
	}

	pool := common.BytesToAddress(result[len(result)-20:])
	if pool == (common.Address{}) {
		return "", false, fmt.Errorf("池子不存在")
	}

	poolAddr := strings.ToLower(pool.Hex())
	isToken0WBNB := strings.ToLower(dex.WBNB) < strings.ToLower(tokenAddr)
	return poolAddr, isToken0WBNB, nil
}
