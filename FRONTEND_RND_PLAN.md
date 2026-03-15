# 前端与可视化研发计划

本文档用于承接当前 Dashboard 的下一阶段研发任务，围绕以下目标展开：

1. 在 Header 中分模块展示首页信息
2. 增加订单交易模块，展示当前交易订单信息，区分实盘和模拟交易
3. 增加 Token 扫链数据展示，显示当前 Token 信息和不符合要求的原因
4. 增加后台运行日志查看能力
5. 将 JSON 策略配置迁移到页面化管理，并支持回测

本计划不只覆盖前端页面，还包含实现这些页面所需的后端接口、数据库调整、数据清理、权限控制和验收标准。

---

## 一、当前状态

### 已有能力
- React Dashboard 已具备首页框架、国际化、设置面板和基础指标展示
- 后端已有 `/api/config` 与 `/api/metrics`
- 交易数据、Token 数据、系统配置数据已经部分落库
- 策略仍来自根目录 `strategies.json`
- 回测能力已存在于 Go 后端命令模式

### 现状问题
- 首页信息层级仍不够清晰，缺少模块级导航
- 没有订单视图，无法从前端区分实盘和模拟交易
- Token 扫链结果没有可视化展示，无法定位被过滤的原因
- 后台日志无法从前端查看，只能依赖终端
- 策略配置不在页面中，回测不能从界面直接发起

---

## 二、研发目标

### 总目标
将当前 Dashboard 从“设置页 + 简单概览”升级为“研究、监控、执行、策略管理”一体化运营界面。

### 目标拆分
- 首页具备清晰的信息入口与模块分区
- 交易执行状态可视化
- Token 扫链过程可视化
- 后台日志可视化
- 策略生命周期可视化，并支持在线配置与回测

---

## 三、研发范围

## 模块 A：Header 模块化导航与首页信息分区

### 目标
将首页内容分模块组织在 Header 中，支持快速切换不同业务区域。

### 建议结构
- `Overview`
- `Orders`
- `Token Scanner`
- `Logs`
- `Strategies`
- `Settings`

### 前端任务
- [ ] A1. 重构 Header，增加模块导航 Tab
- [ ] A2. 支持点击切换模块视图
- [ ] A3. Header 中展示全局状态摘要
  - 当前运行模式
  - 实盘/模拟状态
  - 活跃策略数
  - 后端健康状态
  - 当前语言切换入口
- [ ] A4. 支持移动端折叠菜单

### 后端依赖
- [ ] A5. 提供统一首页摘要接口 `GET /api/overview`

### 建议接口字段
```json
{
  "system_mode": "mixed",
  "trade_enabled": true,
  "dex_enabled": false,
  "active_strategy_count": 3,
  "simulated_strategy_count": 2,
  "live_strategy_count": 1,
  "backend_status": "healthy",
  "last_backtest_at": "2026-03-15T09:00:00Z"
}
```

### 验收标准
- Header 可切换 5 个以上业务模块
- 首页不再依赖单一长页面滚动
- 移动端导航可正常使用

---

## 模块 B：订单交易模块

### 目标
展示当前订单与交易信息，明确区分实盘交易和模拟交易。

### 展示内容
- 当前持仓
- 未成交订单
- 最近成交
- 策略来源
- 交易类型：实盘 / 模拟
- 风险状态：正常 / 止损 / 止盈 / 风控暂停

### 前端任务
- [ ] B1. 增加订单交易模块页面
- [ ] B2. 增加筛选条件
  - 全部
  - 实盘
  - 模拟
  - Strategy ID
  - Symbol
- [ ] B3. 增加表格和卡片双视图
- [ ] B4. 展示订单详情抽屉
  - 开仓价格
  - 平仓价格
  - 数量
  - PnL
  - 开平仓时间
  - 策略 ID
  - 风控动作

### 后端任务
- [ ] B5. 提供订单列表接口 `GET /api/orders`
- [ ] B6. 提供成交记录接口 `GET /api/trades`
- [ ] B7. 提供当前持仓接口 `GET /api/positions`

### 数据要求
- 必须区分 `is_simulated`
- 必须包含 `strategy_id`
- 必须包含 `symbol`
- 必须包含 `opened_at` / `closed_at`

### 建议接口示例
```json
{
  "items": [
    {
      "id": 101,
      "symbol": "BNBUSDT",
      "strategy_id": "ema_macd_rsi",
      "mode": "live",
      "side": "LONG",
      "qty": 0.12,
      "entry_price": 612.4,
      "exit_price": 619.1,
      "pnl": 0.804,
      "status": "closed",
      "opened_at": "2026-03-15T08:15:00Z",
      "closed_at": "2026-03-15T08:45:00Z"
    }
  ]
}
```

### 验收标准
- 前端可区分实盘与模拟交易
- 可按策略、模式、交易对筛选
- 可查看当前持仓和最近成交

---

## 模块 C：Token 扫链数据展示

### 目标
可视化当前扫链结果、已通过的 Token、未通过的 Token 及不符合要求原因。

### 核心需求
- 展示当前被扫描到的 Token
- 展示通过筛选的 Token
- 展示被过滤掉的 Token
- 明确“不符合要求”的原因
- 支持按时间、DEX、状态筛选
- 保留最近一周数据，定期清理一周前数据

### 当前实现差距
- 当前数据库中 `Token` 表主要记录通过后保存的代币信息
- 缺少“过滤失败原因”和“扫描状态”的落库字段

### 数据库任务
- [ ] C1. 扩展 `tokens` 表字段
  - `scan_status`：`passed` / `filtered` / `pending`
  - `reject_reason`：字符串或 JSON
  - `updated_at`
- [ ] C2. 如果不想污染现有 `tokens` 表，则新增 `token_scan_results` 表
  - 推荐新增表，避免与最终交易 Token 混淆

### 推荐表结构
```sql
token_scan_results
- id
- token_address
- symbol
- name
- pool_address
- dex
- fee_tier
- liquidity_bnb
- liquidity_usd
- smart_buys
- created_block
- scan_status
- reject_reason
- scanned_at
```

### 后端任务
- [ ] C3. 扫链流程中，无论通过或失败，都记录一条扫描结果
- [ ] C4. 提供扫描结果接口 `GET /api/token-scans`
- [ ] C5. 支持筛选参数
  - `status`
  - `dex`
  - `date_from`
  - `date_to`
  - `keyword`
- [ ] C6. 增加定时清理任务，每周删除 7 天前数据

### 清理策略建议
- 每天凌晨执行清理，不要只在“每周一次”执行
- 删除 `scanned_at < now() - 7 days` 的记录
- 保留交易相关主表，不删除交易数据，只删除扫链明细

### 前端任务
- [ ] C7. 增加 Token Scanner 模块页面
- [ ] C8. 展示通过率统计卡片
- [ ] C9. 增加扫描结果表格
- [ ] C10. 为失败项增加“原因标签”展示
- [ ] C11. 支持快速筛选“失败原因 Top N”

### 验收标准
- 可以从前端查看最近一周 Token 扫链数据
- 可以明确看到某个 Token 为什么被拒绝
- 可以按筛选条件查询
- 清理任务不会影响最新一周数据

---

## 模块 D：后台运行日志查看

### 目标
允许用户在前端查看后端运行日志，用于定位策略执行、扫链和风控问题。

### 日志范围
- 系统日志
- 交易日志
- 扫链日志
- 策略日志
- 错误日志

### 后端实现建议
日志查看不要直接读取 stdout 全量文本。建议做结构化日志输出并按类别持久化或缓存。

### 方案建议

#### 方案 1：日志入库
- 优点：便于过滤、分页、长期保留
- 缺点：写入量大时要控制性能

#### 方案 2：日志写入文件并通过接口 tail
- 优点：实现快
- 缺点：检索、筛选、结构化能力差

#### 推荐方案
- 短期：文件 + tail 接口
- 中期：结构化日志入库或入 Elasticsearch / Loki

### 后端任务
- [ ] D1. 统一日志格式，增加字段
  - `timestamp`
  - `level`
  - `module`
  - `message`
  - `strategy_id`
  - `symbol`
- [ ] D2. 提供日志接口 `GET /api/logs`
- [ ] D3. 支持筛选参数
  - `level`
  - `module`
  - `strategy_id`
  - `keyword`
  - `cursor` / `page`
- [ ] D4. 可选：增加日志流接口
  - `GET /api/logs/stream`
  - 推荐 SSE，前端实现简单

### 前端任务
- [ ] D5. 增加 Logs 模块页面
- [ ] D6. 增加日志等级筛选
- [ ] D7. 增加自动滚动和暂停更新
- [ ] D8. 增加关键字搜索
- [ ] D9. 增加“复制日志片段”功能

### 验收标准
- 前端可以查看最近日志
- 可按模块与级别筛选
- 支持实时刷新或流式查看

---

## 模块 E：策略页面化配置与回测

### 目标
将当前 `strategies.json` 配置迁移到前端页面中管理，并支持从页面发起回测。

### 当前问题
- 策略由根目录 JSON 文件维护
- 用户无法直接从页面配置策略
- 回测只能通过命令行运行

### 推荐方向
将 `strategies.json` 逐步从“主配置源”迁移为“导入导出格式”，真正的数据源改为数据库。

### 数据库任务
- [ ] E1. 新增 `strategies` 表
  - `id`
  - `strategy_id`
  - `type`
  - `params_json`
  - `enabled`
  - `mode`
  - `created_at`
  - `updated_at`
- [ ] E2. 新增 `backtest_runs` 表
  - `id`
  - `strategy_id`
  - `symbol`
  - `interval`
  - `limit`
  - `initial_balance`
  - `result_json`
  - `started_at`
  - `finished_at`
  - `status`

### 后端任务
- [ ] E3. 提供策略管理接口
  - `GET /api/strategies`
  - `POST /api/strategies`
  - `PUT /api/strategies/:id`
  - `DELETE /api/strategies/:id`
- [ ] E4. 提供回测接口
  - `POST /api/backtests`
  - `GET /api/backtests`
  - `GET /api/backtests/:id`
- [ ] E5. 保留 `strategies.json` 导入导出能力
  - 导入到数据库
  - 从数据库导出 JSON
- [ ] E6. 增加参数校验
  - 不同策略类型对应不同参数约束

### 前端任务
- [ ] E7. 增加 Strategies 模块页面
- [ ] E8. 支持策略列表展示
- [ ] E9. 支持策略新建、编辑、复制、删除
- [ ] E10. 支持参数动态表单
- [ ] E11. 支持回测任务发起
- [ ] E12. 支持回测结果查看
  - PnL
  - Win Rate
  - Max Drawdown
  - Trades
  - 回测时间
- [ ] E13. 可选：回测 Equity Curve 图表

### 验收标准
- 不修改文件也能通过页面管理策略
- 可从页面触发回测并查看结果
- 策略配置与回测结果可持久化

---

## 四、建议新增补充项

以下内容不是你原始 5 条中的显式要求，但实际研发时建议一并纳入：

### 补充 1：统一 API 分层
- 当前前端直接在页面内发请求，后续模块变多后维护成本会升高
- 建议增加：
  - `dashboard/src/api/`
  - `dashboard/src/types/`
  - `dashboard/src/features/`

### 补充 2：前端状态管理
- 如果后续有 Orders、Logs、Strategies、Scanner，多模块共享状态会变复杂
- 建议尽早引入轻量状态管理
  - 可用 Context + reducer
  - 或 Zustand

### 补充 3：权限与安全
- 日志、策略配置、订单视图都不应在无鉴权情况下暴露
- 建议把 Dashboard 后续所有新增接口都放到鉴权之后

### 补充 4：回测任务异步化
- 回测时间可能较长，不建议阻塞 HTTP 请求
- 建议异步任务化：
  - 创建任务
  - 轮询状态
  - 返回结果

### 补充 5：策略版本管理
- 页面化配置策略后，必须支持版本追踪
- 建议新增：
  - 版本号
  - 修改人
  - 修改时间
  - 历史版本回滚

### 补充 6：Token 失败原因标准化
- `reject_reason` 不建议只是自由文本
- 建议标准化编码，例如：
  - `LOW_LIQUIDITY`
  - `INSUFFICIENT_SMART_BUYS`
  - `BLACKLISTED_ROUTER`
  - `SUSPICIOUS_CONTRACT`
  - `DUPLICATE_TOKEN`

### 补充 7：日志模块分级
- `module` 建议至少支持：
  - `system`
  - `trading`
  - `dex_sniper`
  - `alpha_engine`
  - `token_scanner`
  - `backtest`

---

## 五、研发阶段计划

## Phase 1：框架与首页重构
目标：先把页面结构搭起来

- [ ] P1.1 Header 模块化导航
- [ ] P1.2 首页模块切换框架
- [ ] P1.3 `GET /api/overview`
- [ ] P1.4 前端模块路由或状态切换机制

交付结果：
- 前端主框架完成
- 首页信息有清晰模块入口

---

## Phase 2：订单与 Token 模块
目标：先补最核心的运行态数据展示

- [ ] P2.1 订单交易模块页面
- [ ] P2.2 `GET /api/orders`
- [ ] P2.3 `GET /api/trades`
- [ ] P2.4 `GET /api/positions`
- [ ] P2.5 Token 扫链结果表结构调整
- [ ] P2.6 `GET /api/token-scans`
- [ ] P2.7 一周前扫链数据清理任务

交付结果：
- 前端能看到当前交易与扫链数据

---

## Phase 3：日志模块
目标：把系统可观测性拉起来

- [ ] P3.1 日志标准化输出
- [ ] P3.2 `GET /api/logs`
- [ ] P3.3 可选 `GET /api/logs/stream`
- [ ] P3.4 前端日志查看页面

交付结果：
- 可以在前端排查后台行为

---

## Phase 4：策略管理与回测
目标：把策略配置和验证流程从文件迁到页面

- [ ] P4.1 `strategies` 表
- [ ] P4.2 `backtest_runs` 表
- [ ] P4.3 策略 CRUD 接口
- [ ] P4.4 回测任务接口
- [ ] P4.5 策略页面
- [ ] P4.6 回测结果页面

交付结果：
- 策略可页面化配置
- 可页面发起回测

---

## 六、优先级建议

### 高优先级
- Header 模块化导航
- 订单交易模块
- Token 扫链结果展示

原因：
- 这三项最直接提升运营可见性
- 与当前“每天验证策略”的阶段最匹配

### 中优先级
- 后台日志查看

原因：
- 对排障价值很高，但前提是日志结构要先统一

### 高价值但相对更重
- 页面化策略配置与回测

原因：
- 价值最大，但涉及后端接口、数据库、异步任务和配置迁移
- 建议在框架稳定后推进

---

## 七、推荐落地顺序

建议按照下面的顺序推进：

1. 先做 Header 模块化导航与 `overview` 接口
2. 再做订单交易模块
3. 然后做 Token 扫链模块与一周清理
4. 再补日志模块
5. 最后做策略页面化配置与回测

这个顺序的原因：
- 先把首页结构稳定住
- 先补“看数据”的核心能力
- 最后再做“改配置 + 跑回测”的重功能

---

## 八、建议新增文档与输出物

为避免后续开发混乱，建议同时补齐以下文档：

- [ ] API 设计文档
- [ ] 数据库变更文档
- [ ] 日志字段规范文档
- [ ] 策略参数校验文档
- [ ] 回测任务状态机说明

---

## 九、完成定义

当以下条件成立时，可视为本轮 Dashboard 升级完成：

- Header 已实现模块化导航
- 可以在前端查看订单、持仓和成交
- 可以在前端查看最近一周的 Token 扫链结果及失败原因
- 可以在前端查看后台日志
- 可以在前端配置策略并发起回测
- 新增模块均有对应后端接口与验收标准
