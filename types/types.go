package types

import (
	"time"
)

type DEXVersion string

const (
	DEXv2 DEXVersion = "V2"
	DEXv3 DEXVersion = "V3"
)

type TokenInfo struct {
	Address      string
	Symbol       string
	Name         string
	PoolAddress  string
	LiquidityBNB float64
	LiquidityUSD float64
	CreatedBlock uint64
	CreatedAt    time.Time
	SmartBuys    int
	HitWallets   map[string]string
	DEX          DEXVersion
	FeeTier      uint32
}
