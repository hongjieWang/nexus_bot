package main

import (
	"bot/strategy"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// StrategyConfig 定义了策略配置文件的结构
type StrategyConfig struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

// LoadStrategies 从 JSON 配置文件中加载并初始化策略实例
func LoadStrategies(filepath string) ([]strategy.BaseStrategy, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("读取策略配置文件失败: %w", err)
	}

	var configs []StrategyConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("解析策略配置文件失败: %w", err)
	}

	var strats []strategy.BaseStrategy
	for _, cfg := range configs {
		var s strategy.BaseStrategy

		switch cfg.Type {
		case "ema_macd_rsi":
			s = strategy.NewEmaMacdRsiStrategy(cfg.ID)

		case "momentum_breakout":
			lookback := getInt(cfg.Params, "lookback", 20)
			breakoutThresholdBps := getFloat(cfg.Params, "breakoutThresholdBps", 50.0)
			volumeSurgeMult := getFloat(cfg.Params, "volumeSurgeMult", 2.0)
			trailingStopBps := getFloat(cfg.Params, "trailingStopBps", 30.0)
			size := getFloat(cfg.Params, "size", 0)
			s = strategy.NewMomentumBreakoutStrategy(cfg.ID, lookback, breakoutThresholdBps, volumeSurgeMult, trailingStopBps, size)

		case "simple_mm":
			spreadBps := getFloat(cfg.Params, "spreadBps", 20.0)
			size := getFloat(cfg.Params, "size", 0.05)
			s = strategy.NewSimpleMMStrategy(cfg.ID, spreadBps, size)

		case "grid_mm":
			gridSpacingBps := getFloat(cfg.Params, "gridSpacingBps", 20.0)
			numLevels := getInt(cfg.Params, "numLevels", 3)
			sizePerLevel := getFloat(cfg.Params, "sizePerLevel", 0.05)
			maxPosition := getFloat(cfg.Params, "maxPosition", 0.15)
			s = strategy.NewGridMMStrategy(cfg.ID, gridSpacingBps, numLevels, sizePerLevel, maxPosition)

		default:
			slog.Warn("未知的策略类型，将跳过加载", "type", cfg.Type, "id", cfg.ID)
			continue
		}

		strats = append(strats, s)
		slog.Info("✅ 成功加载策略配置", "id", cfg.ID, "type", cfg.Type)
	}

	return strats, nil
}

// 辅助函数：安全地从 map 中获取 float64
func getFloat(m map[string]interface{}, key string, def float64) float64 {
	if val, ok := m[key]; ok {
		if f, isFloat := val.(float64); isFloat {
			return f
		}
	}
	return def
}

// 辅助函数：安全地从 map 中获取 int
func getInt(m map[string]interface{}, key string, def int) int {
	if val, ok := m[key]; ok {
		if f, isFloat := val.(float64); isFloat { // JSON number 解析出来默认是 float64
			return int(f)
		}
	}
	return def
}
