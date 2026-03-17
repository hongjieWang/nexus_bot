package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	MasterKey             string
	RPCURL                string
	WSSURL                string
	RPCRPS                int
	MySQLDSN              string
	BinanceAPIKey         string
	BinanceAPISecret      string
	ExecutionPrivateKey   string
	DexSniperEnabled      bool
	TradeEnabled          bool
	TradeUSDT             float64
	AlphaMEVThreshold     int
	AlphaClusterWindow    int
	AlphaClusterThreshold int
	DiscordWebhook        string
	LiquidityMinBNB       float64
	SmartBuyMin           int
	TransferScanBlocks    int64
	SeenTTL               int // minutes
	LiqPollTimeout        int // seconds
	LiqPollInterval       int // seconds
}

var (
	AppConfig *Config
	once      sync.Once
)

// LoadConfig 加载并返回单例配置对象
func LoadConfig() *Config {
	once.Do(func() {
		// 尝试加载 .env 文件，如果不存在也不报错（可通过系统环境变量）
		_ = godotenv.Load()

		AppConfig = &Config{
			MasterKey:             getEnv("MASTER_KEY", "default_master_key_must_be_32_bt"),
			RPCURL:                getEnv("RPC_URL", "https://bsc-dataseed.binance.org/"),
			WSSURL:                getEnv("WSS_URL", "wss://bsc-ws-node.com"),
			RPCRPS:                getEnvInt("RPC_RPS", 50),
			MySQLDSN:              getEnv("MYSQL_DSN", ""),
			BinanceAPIKey:         getEnv("BINANCE_API_KEY", ""),
			BinanceAPISecret:      getEnv("BINANCE_API_SECRET", ""),
			ExecutionPrivateKey:   getEnv("EXECUTION_PRIVATE_KEY", ""),
			DexSniperEnabled:      getEnvBool("DEX_SNIPER_ENABLED", false),
			TradeEnabled:          getEnvBool("TRADE_ENABLED", false),
			TradeUSDT:             getEnvFloat("TRADE_USDT", 10.0),
			AlphaMEVThreshold:     getEnvInt("ALPHA_MEV_THRESHOLD", 0),
			AlphaClusterWindow:    getEnvInt("ALPHA_CLUSTER_TIME_WINDOW_MINS", 60),
			AlphaClusterThreshold: getEnvInt("ALPHA_CLUSTER_THRESHOLD", 3),
			DiscordWebhook:        getEnv("DISCORD_WEBHOOK", ""),
			LiquidityMinBNB:       getEnvFloat("LIQUIDITY_MIN_BNB", 3.0),
			SmartBuyMin:           getEnvInt("SMART_BUY_MIN", 2),
			TransferScanBlocks:    int64(getEnvInt("TRANSFER_SCAN_BLOCKS", 200)),
			SeenTTL:               getEnvInt("SEEN_TTL_MINS", 30),
			LiqPollTimeout:        getEnvInt("LIQ_POLL_TIMEOUT_SEC", 30),
			LiqPollInterval:       getEnvInt("LIQ_POLL_INTERVAL_SEC", 2),
		}

		log.Println("Configuration loaded successfully")
	})
	return AppConfig
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	valueStr := strings.ToLower(os.Getenv(key))
	if valueStr == "" {
		return defaultValue
	}
	return valueStr == "true" || valueStr == "1"
}

func getEnvFloat(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue
	}
	return value
}
