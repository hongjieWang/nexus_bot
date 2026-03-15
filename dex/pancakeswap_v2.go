package dex

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const PancakeV2RouterABI = `[{"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"}],"name":"getAmountsOut","outputs":[{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactETHForTokensSupportingFeeOnTransferTokens","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForETHSupportingFeeOnTransferTokens","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

const (
	PancakeV2RouterAddress = "0x10ED43C718714eb63d5aA57B78B54704E256024E"
	WBNBAddress            = "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"
)

type PancakeV2Client struct {
	Wallet *Wallet
	Router common.Address
	ABI    abi.ABI
}

func NewPancakeV2Client(wallet *Wallet) (*PancakeV2Client, error) {
	parsedABI, err := abi.JSON(strings.NewReader(PancakeV2RouterABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse router abi: %w", err)
	}

	return &PancakeV2Client{
		Wallet: wallet,
		Router: common.HexToAddress(PancakeV2RouterAddress),
		ABI:    parsedABI,
	}, nil
}

// GetAmountsOut 预估交易获得的代币数量
func (p *PancakeV2Client) GetAmountsOut(ctx context.Context, amountIn *big.Int, path []common.Address) ([]*big.Int, error) {
	data, err := p.ABI.Pack("getAmountsOut", amountIn, path)
	if err != nil {
		return nil, err
	}

	msg := ethereum.CallMsg{To: &p.Router, Data: data}
	result, err := p.Wallet.Client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}

	out, err := p.ABI.Unpack("getAmountsOut", result)
	if err != nil || len(out) == 0 {
		return nil, fmt.Errorf("failed to unpack getAmountsOut: %v", err)
	}

	amounts := out[0].([]*big.Int)
	return amounts, nil
}

// BuyTokenWithBNB 自动用 BNB 买入目标代币 (支持带税代币)
func (p *PancakeV2Client) BuyTokenWithBNB(ctx context.Context, tokenAddress string, bnbAmount *big.Int, slippageBps int64) (*types.Transaction, error) {
	path := []common.Address{
		common.HexToAddress(WBNBAddress),
		common.HexToAddress(tokenAddress),
	}

	// 1. 预估可获得的代币数量
	amountsOut, err := p.GetAmountsOut(ctx, bnbAmount, path)
	if err != nil {
		return nil, fmt.Errorf("GetAmountsOut failed (might be illiquid): %w", err)
	}
	expectedOut := amountsOut[len(amountsOut)-1]

	// 2. 根据滑点计算 minOut
	// amountOutMin = expectedOut * (10000 - slippageBps) / 10000
	multiplier := big.NewInt(10000 - slippageBps)
	amountOutMin := new(big.Int).Mul(expectedOut, multiplier)
	amountOutMin.Div(amountOutMin, big.NewInt(10000))

	// 3. 构建交易
	auth, err := p.Wallet.TransactOpts(ctx)
	if err != nil {
		return nil, err
	}
	auth.Value = bnbAmount
	to := p.Wallet.Address
	deadline := big.NewInt(time.Now().Add(5 * time.Minute).Unix())

	data, err := p.ABI.Pack(
		"swapExactETHForTokensSupportingFeeOnTransferTokens",
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
		Value:    auth.Value,
		Data:     data,
	}
	gasLimit, err := p.Wallet.Client.EstimateGas(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("gas estimation failed (might be honeypot or insufficient BNB): %w", err)
	}
	auth.GasLimit = gasLimit * 120 / 100 // 增加 20% 缓冲防 OutOfGas

	// 5. 签名发送
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    auth.Nonce.Uint64(),
		To:       &p.Router,
		Value:    auth.Value,
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
