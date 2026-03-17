# BSC Sniper 项目重构方案（2026 优化版）

**作者**：Grok（xAI）  
**版本**：v2.0  
**日期**：2026-03-17  
**目标**：将当前 ~1800 行 `main.go` + `alpha_engine.go` 重构为**模块化、可测试、可扩展**的顶级 BSC 聪明钱狙击系统，并为后续 **AI（Grok/Claude） + X（Twitter） + 币安广场** 信号集成打好基础。

---

## 一、拆分后的最终项目目录结构

```bash
bsc-sniper/
├── go.mod
├── go.sum
├── main.go                          # 仅启动顺序 + 优雅关机（<150行）
├── config/
│   └── config.go                    # 环境变量统一管理 + 默认值
├── types/
│   └── types.go                     # TokenInfo、DEXVersion 等
├── database/
│   ├── db.go
│   ├── models.go
│   └── repo.go                      # 数据库操作
├── rpc/
│   ├── client.go                    # 统一 RPC + 限速器 + 重试
│   └── limiter.go
├── dex/
│   ├── constants.go                 # 所有地址、Topic、Selector
│   ├── v2.go
│   └── v3.go
├── scanner/
│   ├── new_pool.go                  # processNewPool + 三关筛选
│   ├── security.go                  # GoPlus 安全审计
│   └── holder.go                    # 持仓分布分析
├── alpha/
│   ├── engine.go                    # Alpha Engine 主入口
│   ├── scoring.go                   # ROI / WinRate 打分
│   ├── clustering.go                # 女巫聚类 + 实体聚合
│   ├── sell_tracker.go
│   └── funding.go                   # CEX 热钱包溯源
├── watcher/
│   ├── smart_money.go               # 存量 Token 聪明钱监听
│   ├── meme_rush.go                 # Four.Meme Bonding Curve
│   └── volume.go                    # 量价异常监控
├── notifier/
│   └── discord.go                   # 所有 Discord 推送
├── sniper/
│   ├── engine.go                    # 自动狙击
│   └── trading.go
├── social/                          # ← 新增（AI + X + Binance Square）
│   ├── client.go
│   ├── analyzer.go                  # AI 提示词 + Grok 调用
│   └── score.go                     # 社交信号权重
├── backtest/
│   └── backtest.go                  # 完全独立的回测模式
├── metrics/
│   └── metrics.go                   # Prometheus 全埋点
├── utils/
│   ├── cache.go                     # LRU Seen Cache
│   ├── price.go
│   └── token.go                     # fetchSymbol / fetchName
├── .env.example
├── strategies.json
├── Dockerfile
├── docker-compose.yml
└── README.md

二、详细任务清单（分 4 个阶段，建议 7 天完成）
Phase 0：准备工作（1 小时）

go mod init bsc-sniper
 创建 .env.example 并补充新增变量（ALPHA_*、SOCIAL_*、GROK_API_KEY 等）
git checkout -b feature/refactor-modules
 备份当前 main.go 和 alpha_engine.go

Phase 1：核心基础设施（P0，4-6 小时）

 创建 rpc/（client.go + limiter.go）
 创建 utils/cache.go（LRU 替换全局 seen）
 创建 metrics/metrics.go（扩展到 15+ 个指标）
 创建 config/config.go（struct + godotenv）
 重写 main.go（注入依赖，删除所有全局变量）
 测试：go run main.go（确认 RPC 限速与 seen 缓存）

Phase 2：DEX + Scanner 模块（P0，5-7 小时）

dex/constants.go（集中所有硬编码）
dex/v2.go + dex/v3.go
scanner/new_pool.go、security.go、holder.go
 迁移 pollLiquidityBNB、fetchV2/V3LiquidityBNB
 测试：新池创建 + Discord 预警完整流程

Phase 3：Alpha Engine + Watcher 全拆分（P1，6-8 小时）

alpha/ 五个文件完整迁移
watcher/ 三个文件（StartAll() 统一启动）
notifier/discord.go（统一所有 Alert）
 测试：聪明钱监听、Meme Rush、SELL 轨迹、女巫聚类全部正常

Phase 4：社交 AI 集成准备 + 收尾（P2，4-6 小时）

 创建 social/ 骨架（analyzer.go 含 Grok 调用模板）
 在 scanner/new_pool.go 加入社交 + AI 最终评分
 回测逻辑完全移入 backtest/
 添加优雅关机（errgroup + context）
 编写 main_test.go（覆盖关键函数）
 更新 README.md


三、推荐新增依赖
Bashgo get github.com/hashicorp/golang-lru/v2
go get golang.org/x/sync/errgroup
go get github.com/asaskevich/EventBus
go get github.com/prometheus/client_golang/prometheus
go get github.com/joho/godotenv

四、下一步立即行动（建议今天开始）
优先级顺序：

今天：完成 Phase 1（rpc + config + utils）—— 最快看到效果
明天：Phase 2（dex + scanner）
后天：Phase 3（alpha + watcher）
周末：Phase 4（social AI 集成）


文件生成说明：

直接复制上方全部内容保存为 BSC-Sniper-重构方案.md 即可
可放入项目根目录作为开发路线图

需要我继续生成具体文件代码？
回复以下任意选项，我立即输出：

「输出 rpc/client.go 完整代码」
「输出精简版 main.go」
「输出 social/analyzer.go AI 模板」
「输出其他模块」