package dex

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// SellTokenForBNB 自动卖出目标代币换回 BNB (支持带税代币)
func (p *PancakeV2Client) SellTokenForBNB(ctx context.Context, tokenAddress string, tokenAmount *big.Int, slippageBps int64) (*types.Transaction, error) {
	path := []common.Address{
		common.HexToAddress(tokenAddress),
		common.HexToAddress(WBNBAddress),
	}

	// 1. 预估可获得的 BNB 数量
	amountsOut, err := p.GetAmountsOut(ctx, tokenAmount, path)
	if err != nil {
		return nil, fmt.Errorf("GetAmountsOut failed (might be illiquid): %w", err)
	}
	expectedOut := amountsOut[len(amountsOut)-1]

	// 2. 根据滑点计算 minOut
	multiplier := big.NewInt(10000 - slippageBps)
	amountOutMin := new(big.Int).Mul(expectedOut, multiplier)
	amountOutMin.Div(amountOutMin, big.NewInt(10000))

	// 3. 构建交易
	auth, err := p.Wallet.TransactOpts(ctx)
	if err != nil {
		return nil, err
	}
	to := p.Wallet.Address
	deadline := big.NewInt(time.Now().Add(5 * time.Minute).Unix())

	data, err := p.ABI.Pack(
		"swapExactTokensForETHSupportingFeeOnTransferTokens",
		tokenAmount,
		amountOutMin,
		path,
		to,
		deadline,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pack data: %w", err)
	}

	// 4. 估算 Gas
	msg := ethereum.CallMsg{
		From:     p.Wallet.Address,
		To:       &p.Router,
		GasPrice: auth.GasPrice,
		Data:     data,
	}
	gasLimit, err := p.Wallet.Client.EstimateGas(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("gas estimation failed (might be honeypot or insufficient allowance): %w", err)
	}
	auth.GasLimit = gasLimit * 120 / 100 // 增加 20% 缓冲防 OutOfGas

	// 5. 签名发送
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    auth.Nonce.Uint64(),
		To:       &p.Router,
		Value:    big.NewInt(0), // 卖出代币不需要发送 BNB
		Gas:      auth.GasLimit,
		GasPrice: auth.GasPrice,
		Data:     data,
	})

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx: %w", err)
	}

	err = p.Wallet.Client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send tx: %w", err)
	}

	return signedTx, nil
}
