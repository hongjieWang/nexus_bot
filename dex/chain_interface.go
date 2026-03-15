package dex

import (
	"context"
	"math/big"
)

// IChainClient 定义了多链架构标准接口 (Phase 5.1)
// 无论是 BSC 上的 PancakeSwap, 还是 Solana 上的 Raydium, Base 上的 Uniswap，
// 只需要实现该接口，即可无缝接入 SniperEngine。
type IChainClient interface {
	ChainName() string
	RouterAddress() string
	GetBalance(ctx context.Context, tokenAddress string, walletAddress string) (*big.Int, error)
	BuyToken(ctx context.Context, tokenAddress string, amountIn *big.Int, slippageBps int64) (txHash string, err error)
	SellToken(ctx context.Context, tokenAddress string, amountIn *big.Int, slippageBps int64) (txHash string, err error)
}
