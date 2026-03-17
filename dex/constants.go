package dex

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Factory 地址
const (
	PancakeV2Factory = "0xcA143Ce32Fe78f1f7019d7d551a6402fC5350c73"
	PancakeV3Factory = "0x0BFbCF9fa4f9C56B0F40a671Ad40E0805A091865"
	WBNB             = "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"
)

// 事件 Topic
var (
	TopicV2PairCreated = crypto.Keccak256Hash([]byte("PairCreated(address,address,address,uint256)"))
	TopicV3PoolCreated = crypto.Keccak256Hash([]byte("PoolCreated(address,address,uint24,int24,address)"))
	TopicTransfer      = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
)

// ABI 选择器
var (
	SelectorGetReserves = []byte{0x09, 0x02, 0xf1, 0xac}
	SelectorSlot0       = []byte{0x38, 0x50, 0xc7, 0xbd}
	SelectorLiquidity   = []byte{0x1a, 0x68, 0x65, 0x02}
	SelectorSymbol      = []byte{0x95, 0xd8, 0x9b, 0x41}
	SelectorName        = []byte{0x06, 0xfd, 0xde, 0x03}
)

// DEX Router 白名单
var DexRouters = map[string]struct{}{
	strings.ToLower("0x10ed43c718714eb63d5aa57b78b54704e256024e"): {}, // PancakeSwap V2
	strings.ToLower("0x13f4ea83d0bd40e75c8222255bc855a974568dd4"): {}, // PancakeSwap V3
	strings.ToLower("0x1b81d678ffb9c0263b24a97847620c99d213eb14"): {}, // PancakeSwap V3 (new)
	strings.ToLower("0x6352a56caadc4f1e25cd6c75970fa768a3304e64"): {}, // OpenOcean
	strings.ToLower("0x1111111254eeb25477b68fb85ed929f73a960582"): {}, // 1inch v5
	strings.ToLower("0x6131b5fae19ea4f9d964eac0408e4408b66337b5"): {}, // KyberSwap
}

var ZeroAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")
