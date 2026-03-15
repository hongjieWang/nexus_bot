package database

import (
	"bot/types"
	"log/slog"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Token 模型：记录扫描到的 DEX 代币信息
type Token struct {
	ID           uint      `gorm:"primaryKey"`
	Address      string    `gorm:"type:varchar(64);uniqueIndex"`
	Symbol       string    `gorm:"type:varchar(20)"`
	Name         string    `gorm:"type:varchar(100)"`
	PoolAddress  string    `gorm:"type:varchar(64)"`
	LiquidityBNB float64   `gorm:"type:double"`
	LiquidityUSD float64   `gorm:"type:double"`
	CreatedBlock uint64    `gorm:"type:bigint"`
	SmartBuys    int       `gorm:"type:int"`
	DEX          string    `gorm:"type:varchar(20)"`
	FeeTier      uint32    `gorm:"type:int"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// TradeHistory 模型：记录所有的 CEX/DEX 开平仓记录
type TradeHistory struct {
	ID          uint      `gorm:"primaryKey"`
	Symbol      string    `gorm:"type:varchar(20);index"`
	StrategyID  string    `gorm:"type:varchar(50);index"`
	Side        string    `gorm:"type:varchar(10)"` // "BUY" / "SELL" 或 "LONG" / "SHORT"
	Qty         float64   `gorm:"type:double"`
	EntryPrice  float64   `gorm:"type:double"`
	ExitPrice   float64   `gorm:"type:double"`
	PnL         float64   `gorm:"type:double"`
	Balance     float64   `gorm:"type:double"`
	IsSimulated bool      `gorm:"type:boolean"`
	OpenedAt    time.Time `gorm:"index"`
	ClosedAt    time.Time
}

// SmartWallet 模型：聪明钱数据库，支持历史战绩与防女巫 (Phase 3)
type SmartWallet struct {
	Address        string    `gorm:"type:varchar(64);primaryKey"`
	Label          string    `gorm:"type:varchar(100)"`
	Score          float64   `gorm:"type:double;default:50.0"`   // 动态权重评分
	WinRate        float64   `gorm:"type:double;default:0.0"`    // 胜率
	TotalTrades    int       `gorm:"type:int;default:0"`         // 总交易笔数
	WinTrades      int       `gorm:"type:int;default:0"`         // ← 新增
	ROI            float64   `gorm:"type:double;default:0.0"`    // 平均投资回报率
	AvgEntryBlocks float64   `gorm:"type:double;default:999"`    // ← 新增：平均入场块数
	LastActiveAt   time.Time `gorm:"index"`                      // ← 新增：最近一次交易时间
	IsMEV          bool      `gorm:"type:boolean;default:false"` // 是否被标记为 MEV/套利机器人
	ClusterID      string    `gorm:"type:varchar(64)"`           // 实体聚类 ID (防女巫)
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// SmartEntity 模型：用于聚合被判定为同一个矩阵号/女巫的多个钱包战绩
type SmartEntity struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)"` // 对应 SmartWallet 的 ClusterID
	WalletCount int       `gorm:"type:int;default:0"`          // 实体包含的钱包数量
	TotalTrades int       `gorm:"type:int;default:0"`          // 实体总交易笔数
	WinRate     float64   `gorm:"type:double;default:0.0"`     // 实体总胜率
	ROI         float64   `gorm:"type:double;default:0.0"`     // 实体平均回报率
	Score       float64   `gorm:"type:double;default:50.0"`    // 实体最终权重得分
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// SmartWalletTrade 模型：记录聪明钱的每一笔操作轨迹 (用于后续回测与打分)
type SmartWalletTrade struct {
	ID           uint      `gorm:"primaryKey"`
	Wallet       string    `gorm:"type:varchar(64);index"`
	TokenAddress string    `gorm:"type:varchar(64);index"`
	Action       string    `gorm:"type:varchar(10)"` // "BUY" / "SELL"
	AmountUSD    float64   `gorm:"type:double"`      // 交易金额 (USD)
	BlockNum     uint64    `gorm:"type:bigint"`
	Price        float64   `gorm:"type:double;default:0"` // ← 新增：成交价格（BNB 计）
	PoolAddress  string    `gorm:"type:varchar(64)"`      // ← 新增：来源 pool
	Timestamp    time.Time `gorm:"autoCreateTime"`
}

// SystemConfig 模型：用于在数据库中安全存储用户前端配置的密钥 (Phase 5)
type SystemConfig struct {
	ID    string `gorm:"primaryKey;type:varchar(50)"`
	Value string `gorm:"type:text"` // 存储加密后的敏感信息
}

func InitDB() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		slog.Warn("MYSQL_DSN 未设置，数据库记录功能已禁用")
		return
	}

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		slog.Error("连接 MySQL 失败", "err", err)
		return
	}

	// 自动迁移模式，确保表结构与结构体一致
	err = DB.AutoMigrate(&Token{}, &TradeHistory{}, &SmartWallet{}, &SmartEntity{}, &SmartWalletTrade{}, &SystemConfig{})
	if err != nil {
		slog.Error("MySQL 表结构迁移失败", "err", err)
		return
	}

	slog.Info("✅ MySQL 数据库连接成功，ORM (GORM) 初始化完成，表结构已就绪")
}

// syncSmartWalletsToDB 同步最新获取的聪明钱列表到数据库，支持 MEV 过滤
func SyncSmartWalletsToDB(wallets map[string]string) {
	if DB == nil {
		return
	}

	count := 0
	for addr, label := range wallets {
		wallet := SmartWallet{
			Address: addr,
			Label:   label,
			Score:   50.0,
		}
		// 如果不存在则插入，存在则更新 Label
		if err := DB.Where(SmartWallet{Address: addr}).FirstOrCreate(&wallet).Error; err == nil {
			if wallet.Label != label {
				DB.Model(&wallet).Update("Label", label)
			}
			count++
		}
	}
	slog.Info("🧠 聪明钱数据库同步完毕", "upserted", count)
}

// recordSmartWalletTrade 记录聪明钱交互轨迹
func RecordSmartWalletTrade(walletAddr, tokenAddr, action string, amountUSD float64, blockNum uint64) {
	if DB == nil {
		return
	}
	trade := SmartWalletTrade{
		Wallet:       walletAddr,
		TokenAddress: tokenAddr,
		Action:       action,
		AmountUSD:    amountUSD,
		BlockNum:     blockNum,
	}
	DB.Create(&trade)
}

// RecordSmartWalletTradeWithPrice 记录带价格的聪明钱交互轨迹
func RecordSmartWalletTradeWithPrice(walletAddr, tokenAddr, action string, blockNum uint64, price float64, poolAddr string) {
	if DB == nil {
		return
	}
	// 防止重复写入同一笔卖出
	var cnt int64
	DB.Model(&SmartWalletTrade{}).
		Where("wallet = ? AND token_address = ? AND action = ? AND block_num = ?",
			walletAddr, tokenAddr, action, blockNum).
		Count(&cnt)
	if cnt > 0 {
		return
	}
	trade := SmartWalletTrade{
		Wallet:       walletAddr,
		TokenAddress: tokenAddr,
		Action:       action,
		BlockNum:     blockNum,
		Price:        price,
		PoolAddress:  poolAddr,
	}
	DB.Create(&trade)
}

// getFilteredSmartWallets 获取高分且非 MEV 的聪明钱地址列表
func GetFilteredSmartWallets() map[string]string {
	if DB == nil {
		return nil
	}

	var wallets []SmartWallet
	// 过滤掉被标记为 MEV 的机器人，并可以设定分数阈值 (例如 Score > 20)
	if err := DB.Where("is_mev = ? AND score > ?", false, 20.0).Find(&wallets).Error; err != nil {
		return nil
	}

	filtered := make(map[string]string)
	for _, w := range wallets {
		filtered[w.Address] = w.Label
	}
	return filtered
}

// logTradeToDB 记录交易到 MySQL (兼容之前 trading.go 的调用)
func LogTradeToDB(symbol, strategyID, side string, qty, entryPrice, exitPrice, pnl, balance float64, openedAt, closedAt time.Time, isSimulated bool) {
	if DB == nil {
		return
	}

	trade := TradeHistory{
		Symbol:      symbol,
		StrategyID:  strategyID,
		Side:        side,
		Qty:         qty,
		EntryPrice:  entryPrice,
		ExitPrice:   exitPrice,
		PnL:         pnl,
		Balance:     balance,
		IsSimulated: isSimulated,
		OpenedAt:    openedAt,
		ClosedAt:    closedAt,
	}

	if err := DB.Create(&trade).Error; err != nil {
		slog.Error("记录交易到 MySQL 失败", "err", err)
	}
}

// saveTokenToDB 将新发现的代币信息持久化到数据库
func SaveTokenToDB(info types.TokenInfo) {
	if DB == nil {
		return
	}

	token := Token{
		Address:      info.Address,
		Symbol:       info.Symbol,
		Name:         info.Name,
		PoolAddress:  info.PoolAddress,
		LiquidityBNB: info.LiquidityBNB,
		LiquidityUSD: info.LiquidityUSD,
		CreatedBlock: info.CreatedBlock,
		SmartBuys:    info.SmartBuys,
		DEX:          string(info.DEX),
		FeeTier:      info.FeeTier,
	}

	// 使用 Clauses 进行 Insert ... ON DUPLICATE KEY UPDATE 以防止重复
	// 但这需要额外引入包，所以简单的可以用 FirstOrCreate 替代
	var existing Token
	result := DB.Where("address = ?", info.Address).First(&existing)
	if result.Error == gorm.ErrRecordNotFound {
		if err := DB.Create(&token).Error; err != nil {
			slog.Error("保存 Token 到 MySQL 失败", "err", err)
		}
	} else if result.Error != nil {
		slog.Error("查询 Token 失败", "err", result.Error)
	}
}
