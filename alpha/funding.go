package alpha

import (
	"bot/rpc"
	"context"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// 已知 CEX 热钱包
var knownCEXHotWallets = map[string]string{
	"0x28c6c06298d514db089934071355e5743bf21d60": "BINANCE",
	"0x3f5ce5fbfe3e9af3971dd833d26ba9b5c9ae9a9":  "BINANCE",
	"0xdfd5293d8e347dfe59e90efd55b2956a1343963d": "BINANCE",
	"0x56eddb7aa87536c09ccc2793473599fd21a8b17f": "BINANCE",
	"0x21a31ee1afc51d94c2efccaa2092ad1028285549": "BINANCE",
	"0xbe0eb53f46cd790cd13851d5eff43d12404d33e8": "BINANCE",
	"0xf977814e90da44bfa03b6295a0616a897441acec": "BINANCE",
	"0x98ec059dc3adfbdd63429454aeb0c990fba4a128": "OKX",
	"0x236f9f97e0e62388479bf9e5ba4889e46b0273c3": "OKX",
	"0xa7efae728d2936e78bda97dc267687568dd593f3": "OKX",
	"0xf89d7b9c864f589bbf53a82105107622b35eaa40": "BYBIT",
	"0x2b5634c42055806a59e9107ed44d43c426e58258": "BYBIT",
	"0xd6216fc19db775df9774a6e33526131da7d19a2c": "KUCOIN",
	"0x55fe002aeff02f77364de339a1292923a15844b8": "KUCOIN",
	"0x0d0707963952f2fba59dd06f2b425ace40b492fe": "GATE",
	"0x7793cd85c11a924478d358d49b05b37e91b5810f": "GATE",
	"0x1ab4973a48dc892cd9971ece8e01dcc7688f8f23": "BITGET",
	"0x75e89d5979e4f6fba9f97c104c2f0afb3f1dcb88": "MEXC",
	"0xaab27b150451726ec7738aa1d0a94505c8729bd1": "HTX",
	"0x6748f50f686bfbca6fe8ad62b22228b87f31ff2b": "HTX",
}

var fundingCache sync.Map

func TraceFundingSource(rpcClient *rpc.Client, w1, w2 string) bool {
	src1 := GetWalletFundingSource(rpcClient, w1)
	if src1 == "" { return false }
	src2 := GetWalletFundingSource(rpcClient, w2)
	if src2 == "" { return false }
	return src1 == src2
}

func GetWalletFundingSource(rpcClient *rpc.Client, wallet string) string {
	if src, ok := fundingCache.Load(wallet); ok {
		return src.(string)
	}

	wallet = strings.ToLower(wallet)
	if cex, ok := knownCEXHotWallets[wallet]; ok {
		fundingCache.Store(wallet, cex)
		return cex
	}

	latest, err := rpc.Call(rpcClient, context.Background(), func(ctx context.Context) (uint64, error) {
		return rpcClient.Client.BlockNumber(ctx)
	})
	if err != nil { return "" }

	fromBlock := new(big.Int).Sub(big.NewInt(int64(latest)), big.NewInt(30*24*60*60/3)) // ≈30天
	query := ethereum.FilterQuery{
		Topics:    [][]common.Hash{{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))}, nil, {common.HexToHash(wallet)}},
		FromBlock: fromBlock,
		ToBlock:   new(big.Int).SetUint64(latest),
	}

	logs, err := rpc.Call(rpcClient, context.Background(), func(ctx context.Context) ([]gethtypes.Log, error) {
		return rpcClient.Client.FilterLogs(ctx, query)
	})
	if err != nil { return "" }

	for _, lg := range logs {
		if len(lg.Topics) < 2 { continue }
		from := strings.ToLower(common.BytesToAddress(lg.Topics[1].Bytes()).Hex())
		if cex, ok := knownCEXHotWallets[from]; ok {
			fundingCache.Store(wallet, cex)
			return cex
		}
	}

	return ""
}
