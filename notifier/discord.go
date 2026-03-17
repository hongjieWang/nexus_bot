package notifier

import (
	"bot/config"
	"bot/types"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type discordPayload struct {
	Username  string         `json:"username"`
	AvatarURL string         `json:"avatar_url"`
	Embeds    []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title     string              `json:"title"`
	Color     int                 `json:"color"`
	Fields    []discordEmbedField `json:"fields"`
	Footer    discordFooter       `json:"footer"`
	Timestamp string              `json:"timestamp"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordFooter struct {
	Text string `json:"text"`
}

func SendDiscordAlert(cfg *config.Config, t types.TokenInfo, v3FeeLabel map[uint32]string, bnbPrice float64) {
	if cfg.DiscordWebhook == "" {
		return
	}

	ageMin := int(time.Since(t.CreatedAt).Minutes())
	var walletLines []string
	for addr, label := range t.HitWallets {
		walletLines = append(walletLines, fmt.Sprintf("• `%s` (%s)", addr, label))
	}
	walletDetail := strings.Join(walletLines, "\n")
	if walletDetail == "" {
		walletDetail = "—"
	}

	dexLabel := fmt.Sprintf("PancakeSwap %s", t.DEX)
	if t.DEX == types.DEXv3 && t.FeeTier > 0 {
		if label, ok := v3FeeLabel[t.FeeTier]; ok {
			dexLabel += fmt.Sprintf(" (%s)", label)
		}
	}

	embed := discordEmbed{
		Title: fmt.Sprintf("🔥 聪明钱链上狙击预警 [%s]", dexLabel),
		Color: 0xFF4500,
		Fields: []discordEmbedField{
			{Name: "🪙 币种", Value: fmt.Sprintf("`%s` (%s)", t.Symbol, t.Name), Inline: true},
			{Name: "⏱ 币龄", Value: fmt.Sprintf("%d 分钟", ageMin), Inline: true},
			{Name: "🧠 聪明钱", Value: fmt.Sprintf("**%d 个地址**", t.SmartBuys), Inline: true},
			{Name: "💧 流动性", Value: fmt.Sprintf("%.2f BNB ($%.0f)", t.LiquidityBNB, t.LiquidityUSD), Inline: true},
			{Name: "📡 DEX", Value: dexLabel, Inline: true},
			{Name: "🏦 创建块", Value: fmt.Sprintf("#%d", t.CreatedBlock), Inline: true},
			{Name: "📋 Token", Value: fmt.Sprintf("`%s`", t.Address), Inline: false},
			{Name: "🔗 Pool", Value: fmt.Sprintf("`%s`", t.PoolAddress), Inline: false},
			{Name: "🧠 命中钱包", Value: walletDetail, Inline: false},
			{
				Name: "🔍 链接",
				Value: fmt.Sprintf(
					"[DexScreener](https://dexscreener.com/bsc/%s)  ·  "+
						"[GoPlus](https://gopluslabs.io/token-security/56/%s)  ·  "+
						"[BscScan](https://bscscan.com/token/%s)",
					t.Address, t.Address, t.Address,
				),
				Inline: false,
			},
		},
		Footer:    discordFooter{Text: fmt.Sprintf("BSC | %s | WSS 实时订阅", dexLabel)},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(discordPayload{
		Username:  "BSC Chain Scanner",
		AvatarURL: "https://assets.pancakeswap.finance/web/favicon/favicon-32x32.png",
		Embeds:    []discordEmbed{embed},
	})
	if err != nil {
		slog.Error("Discord payload 序列化失败", "err", err)
		return
	}
	
	resp, err := http.Post(cfg.DiscordWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("Discord 发送失败", "err", err)
		return
	}
	defer resp.Body.Close()
}

func SendTradingAlert(webhookURL string, action string, posQty, pnl, fillPrice float64) {
	if webhookURL == "" {
		return
	}
	var title string
	if action == "open" {
		title = fmt.Sprintf("🔵 开仓/加仓 BNB (数量: %.3f)", posQty)
	} else {
		title = fmt.Sprintf("⚪ 模拟平仓/减仓 BNB (盈利: $%.2f)", pnl)
	}

	embed := discordEmbed{
		Title: title, Color: 0x00C851,
		Fields: []discordEmbedField{
			{Name: "净持仓", Value: fmt.Sprintf("%.3f", posQty), Inline: true},
			{Name: "成交价", Value: fmt.Sprintf("$%.4f", fillPrice), Inline: true},
		},
	}
	body, _ := json.Marshal(discordPayload{Username: "Grid Bot", Embeds: []discordEmbed{embed}})
	http.Post(webhookURL, "application/json", bytes.NewReader(body))
}
