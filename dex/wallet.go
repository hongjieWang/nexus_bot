package dex

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Wallet manages the private key and signing transactions
type Wallet struct {
	Client     *ethclient.Client
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
	ChainID    *big.Int
}

// NewWallet initializes a wallet from a hex private key
func NewWallet(client *ethclient.Client, hexKey string, chainID int64) (*Wallet, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")
	privateKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &Wallet{
		Client:     client,
		PrivateKey: privateKey,
		Address:    address,
		ChainID:    big.NewInt(chainID),
	}, nil
}

// TransactOpts generates the standard transaction options
func (w *Wallet) TransactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	nonce, err := w.Client.PendingNonceAt(ctx, w.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := w.Client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(w.PrivateKey, w.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)     // in wei
	auth.GasLimit = uint64(300000) // Default Gas Limit
	auth.GasPrice = gasPrice

	return auth, nil
}
