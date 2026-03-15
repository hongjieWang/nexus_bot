package dex

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const ERC20ABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"},{"constant":false,"inputs":[{"name":"_spender","type":"address"},{"name":"_value","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"},{"constant":true,"inputs":[{"name":"_owner","type":"address"},{"name":"_spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"},{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"}]`

type ERC20Client struct {
	Wallet *Wallet
	ABI    abi.ABI
}

func NewERC20Client(wallet *Wallet) (*ERC20Client, error) {
	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse erc20 abi: %w", err)
	}
	return &ERC20Client{Wallet: wallet, ABI: parsedABI}, nil
}

func (e *ERC20Client) BalanceOf(ctx context.Context, tokenAddress string, owner common.Address) (*big.Int, error) {
	to := common.HexToAddress(tokenAddress)
	data, err := e.ABI.Pack("balanceOf", owner)
	if err != nil {
		return nil, err
	}
	msg := ethereum.CallMsg{To: &to, Data: data}
	result, err := e.Wallet.Client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}

	out, err := e.ABI.Unpack("balanceOf", result)
	if err != nil || len(out) == 0 {
		return nil, fmt.Errorf("failed to unpack balanceOf: %v", err)
	}
	return out[0].(*big.Int), nil
}

func (e *ERC20Client) Decimals(ctx context.Context, tokenAddress string) (uint8, error) {
	to := common.HexToAddress(tokenAddress)
	data, err := e.ABI.Pack("decimals")
	if err != nil {
		return 0, err
	}
	msg := ethereum.CallMsg{To: &to, Data: data}
	result, err := e.Wallet.Client.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, err
	}

	out, err := e.ABI.Unpack("decimals", result)
	if err != nil || len(out) == 0 {
		return 0, fmt.Errorf("failed to unpack decimals: %v", err)
	}
	return out[0].(uint8), nil
}

func (e *ERC20Client) Approve(ctx context.Context, tokenAddress string, spender common.Address, amount *big.Int) (*types.Transaction, error) {
	to := common.HexToAddress(tokenAddress)
	data, err := e.ABI.Pack("approve", spender, amount)
	if err != nil {
		return nil, err
	}

	auth, err := e.Wallet.TransactOpts(ctx)
	if err != nil {
		return nil, err
	}

	msg := ethereum.CallMsg{
		From:     e.Wallet.Address,
		To:       &to,
		GasPrice: auth.GasPrice,
		Data:     data,
	}
	gasLimit, err := e.Wallet.Client.EstimateGas(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("approve gas estimation failed: %w", err)
	}
	auth.GasLimit = gasLimit * 120 / 100 // 20% buffer

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    auth.Nonce.Uint64(),
		To:       &to,
		Value:    big.NewInt(0),
		Gas:      auth.GasLimit,
		GasPrice: auth.GasPrice,
		Data:     data,
	})

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx: %w", err)
	}

	err = e.Wallet.Client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send tx: %w", err)
	}

	return signedTx, nil
}
