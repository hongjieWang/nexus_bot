import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

export type Locale = 'zh-CN' | 'en' | 'ja' | 'ko'

type TranslationTree = {
  [key: string]: string | TranslationTree
}

type I18nContextValue = {
  locale: Locale
  setLocale: (locale: Locale) => void
  t: (key: string) => string
}

const LOCALE_STORAGE_KEY = 'nexus-dashboard-locale'

export const localeOptions: Array<{ code: Locale; label: string }> = [
  { code: 'zh-CN', label: '简体中文' },
  { code: 'en', label: 'English' },
  { code: 'ja', label: '日本語' },
  { code: 'ko', label: '한국어' },
]

const translations: Record<Locale, TranslationTree> = {
  'zh-CN': {
    language: {
      label: '语言',
    },
    common: {
      notAvailable: '暂无',
      liveMetrics: '实时指标',
      reviewNext: '继续查看',
      perTradeBudgetValue: '{value} USDT',
      readinessScoreValue: '{value}/3',
      validatedStrategiesValue: '{validated}/{total}',
      watchCountMeta: '另有 {count} 个策略处于观察中',
      saveSuccess: '配置已更新并加密保存。',
      saveError: '配置保存失败。',
    },
    hero: {
      eyebrow: '每日验证控制台',
      kicker: 'Nexus 策略控制台',
      title: '在一个界面里完成研究审阅、部署准备和密钥控制。',
      copy:
        '后端已经在按天验证策略。这一版前端把首页改成运营台，用于在开启真实资金前审查信号质量、部署姿态和系统准备度。',
      verificationCadenceLabel: '验证频率',
      verificationCadenceValue: '每日',
      primaryMarketLabel: '主交易市场',
      runtimeModeLabel: '当前运行模式',
      runtimeModeArmed: '可执行工作区',
      runtimeModeObservation: '观察工作区',
      readinessArmed: '已就绪',
      readinessNearlyReady: '接近就绪',
      readinessObservation: '观察模式',
    },
    commandDeck: {
      title: '指挥面板',
      readinessScoreLabel: '准备度评分',
      note:
        '策略实验室中的快照当前仍是前端种子数据，下一步可以直接替换为后端的验证接口结果。',
      cexEngineLabel: 'CEX 引擎',
      dexSniperLabel: 'DEX 狙击',
      vaultKeyLabel: '私钥状态',
      perTradeBudgetLabel: '单笔预算',
      enabled: '已启用',
      standby: '待机',
      present: '已配置',
      missing: '缺失',
    },
    navigation: {
      kicker: '模块导航',
      title: '在首页头部切换业务模块。',
      description: 'Phase 1 先把 Header 和模块切换框架搭好，Orders、Scanner、Logs 会在后续阶段接入真实数据。',
      activeModuleLabel: '当前模块',
      summaryExecutions: '执行记录',
      summaryStrategies: '策略数量',
      summaryLocale: '当前语言',
      tabs: {
        overview: {
          label: '总览',
          description: '系统状态与部署准备',
        },
        orders: {
          label: '订单',
          description: '实盘与模拟交易面板',
        },
        scanner: {
          label: '扫链',
          description: 'Token 发现与过滤结果',
        },
        logs: {
          label: '日志',
          description: '后台运行与排障查看',
        },
        strategies: {
          label: '策略',
          description: '策略验证与回测入口',
        },
        settings: {
          label: '设置',
          description: '密钥仓与引擎控制',
        },
      },
      placeholders: {
        orders: {
          title: '订单交易模块骨架',
          description: '这个模块会承接当前订单、持仓、成交和风控动作，并明确区分实盘与模拟交易。',
          point1: '前端将增加订单列表、持仓视图和成交明细抽屉。',
          point2: '后端将接入 `/api/orders`、`/api/trades`、`/api/positions`。',
          point3: '后续会补充按策略、模式、交易对筛选能力。',
        },
        scanner: {
          title: 'Token 扫链模块骨架',
          description: '这个模块会展示最近一周的 Token 扫描结果，并明确显示未通过的原因。',
          point1: '前端将增加筛选、状态标签和失败原因分组。',
          point2: '后端需要补充扫链结果表及 `/api/token-scans`。',
          point3: '会加入最近 7 天数据保留和定期清理机制。',
        },
        logs: {
          title: '运行日志模块骨架',
          description: '这个模块会让前端可查看后台日志，用于定位策略执行、扫链和风控问题。',
          point1: '前端将支持实时刷新、级别筛选和关键字搜索。',
          point2: '后端需要统一日志格式并提供 `/api/logs` 接口。',
          point3: '后续可扩展为 SSE 流式日志查看。',
        },
      },
    },
    overview: {
      kicker: '总览',
      title: '系统脉搏与部署姿态',
      totalExecutionsLabel: '总执行次数',
      totalExecutionsMeta: '全部已记录成交',
      alphaWinRateLabel: 'Alpha 胜率',
      alphaWinRateMeta: '已完成平仓结果',
      activeTokensLabel: '活跃代币',
      activeTokensMeta: '当前链上追踪标的',
      validatedStrategiesLabel: '已验证策略',
      validatedStrategiesMeta: '另有 {count} 个策略处于观察中',
    },
    posture: {
      title: '验证姿态',
      validated: '已验证',
      watchlist: '观察列表',
      tuning: '调参中',
      validatedConfidence: '验证置信度',
      executionReadiness: '执行准备度',
      telemetryCoverage: '遥测覆盖率',
      notesTitle: '今日运行说明',
      note1: '密钥与安全控制已经从策略审阅区域中独立出来。',
      note2: '回测优先的工作流现在是首页主视图的核心。',
      note3: '当前前端结构已为接入验证记录、成交、仓位和告警预留位置。',
    },
    strategyLab: {
      kicker: '策略实验室',
      title: '每日验证看板',
      seededTag: '在后端接口就绪前使用种子数据',
      confidence: '置信度',
      winRate: '胜率',
      drawdown: '回撤',
      expectancy: '期望值',
      validationFeedTitle: '验证动态',
      apiTargetsTitle: '下一步 API 目标',
      apiTargetOverview: '`GET /api/overview` 用于更丰富的总览指标',
      apiTargetValidations: '`GET /api/validations` 用于每日策略验证记录',
      apiTargetExecution: '`GET /api/execution-feed` 用于成交、告警和仓位动态',
      statuses: {
        Validated: '已验证',
        Watch: '观察中',
        Tuning: '调参中',
      },
      strategies: {
        ema_macd_rsi: {
          label: 'EMA / MACD / RSI',
          market: 'BNBUSDT · 15m',
          cadence: '每日验证',
          thesis: '趋势过滤器在方向性行情下仍最稳定，是当前最接近实盘部署的策略。',
        },
        momentum_breakout: {
          label: '动量突破',
          market: 'BNBUSDT · 15m',
          cadence: '每日验证',
          thesis: '上行捕捉能力仍强，但震荡区间会明显拉大回撤。',
        },
        grid_mm: {
          label: '网格做市',
          market: 'BNBUSDT · 区间',
          cadence: '盘中验证',
          thesis: '成交质量不错，但 reduce-only 切换阶段还需要更强的运营确定性。',
        },
        simple_mm: {
          label: '简化做市',
          market: 'BNBUSDT · 微观结构',
          cadence: '每日验证',
          thesis: '适合作为基准策略，用来对比库存控制和更复杂做市逻辑。',
        },
      },
      feed: {
        item1Title: '每日验证批次完成',
        item1Detail: '4 套策略已经在最近 1500 根 K 线窗口上完成回放。',
        item2Title: '突破模型加入观察列表',
        item2Detail: '动量模型仍保持盈利，但回撤较前一日扩大。',
        item3Title: 'DEX 遥测已写入',
        item3Detail: '链上追踪代币数量已经进入 dashboard 指标。',
        item4Title: '下一步前后端联调目标',
        item4Detail: '用 Go 后端的验证记录接口替换当前种子策略快照。',
      },
    },
    settings: {
      kicker: '设置',
      title: '密钥仓与引擎控制',
      liveConfig: '实时配置',
      dexSniperTitle: 'DEX 狙击引擎',
      dexSniperDesc: '在支持的池子上自动买入并在风险事件下快速卖出',
      cexEngineTitle: 'CEX 量化引擎',
      cexEngineDesc: '允许将已验证策略发送到 Binance Futures 执行',
      evmPrivateKey: 'EVM 私钥',
      evmPlaceholderPresent: '私钥已配置',
      evmPlaceholderMissing: '输入不带 0x 的私钥',
      tradeBudget: '单笔交易预算',
      tradeBudgetPlaceholder: '50',
      binanceApiKey: 'Binance API Key',
      binanceApiKeyPlaceholder: '输入 API Key',
      binanceApiSecret: 'Binance API Secret',
      binanceApiSecretPlaceholder: '输入 API Secret',
      saveIdle: '提交到安全密钥仓',
      saveSaving: '正在提交安全更新...',
      operationalPostureTitle: '运行姿态',
      vaultIsolationTitle: '密钥隔离',
      vaultIsolationConfigured: '执行私钥已存在，界面中仅显示掩码状态。',
      vaultIsolationMissing: '执行私钥尚未配置。',
      strategyExecutionTitle: '策略执行',
      strategyExecutionEnabled: '实盘执行开关已启用。',
      strategyExecutionStandby: '当前仍以观察模式运行。',
      onchainPostureTitle: '链上姿态',
      onchainPostureEnabled: 'Sniper 监控已启用。',
      onchainPostureStandby: 'Sniper 当前仍关闭。',
      nextPhaseTitle: '下一阶段界面',
      phase1: '增加带筛选和下钻能力的验证记录历史。',
      phase2: '从后端接入成交、仓位和风险告警动态。',
      phase3: '增加策略对比视图和参数扫描汇总。',
    },
  },
  en: {
    language: {
      label: 'Language',
    },
    common: {
      notAvailable: 'N/A',
      liveMetrics: 'Live metrics',
      reviewNext: 'Review next',
      perTradeBudgetValue: '{value} USDT',
      readinessScoreValue: '{value}/3',
      validatedStrategiesValue: '{validated}/{total}',
      watchCountMeta: '{count} more under observation',
      saveSuccess: 'Configuration updated and stored securely.',
      saveError: 'Failed to save configuration.',
    },
    hero: {
      eyebrow: 'Daily Verification Console',
      kicker: 'Nexus strategy desk',
      title: 'Operate research, readiness, and vault controls from one surface.',
      copy:
        'The backend is already validating strategies every day. This frontend revision turns the landing surface into an operations desk for reviewing signal quality, deployment posture, and system readiness before capital is switched on.',
      verificationCadenceLabel: 'Verification cadence',
      verificationCadenceValue: 'Daily',
      primaryMarketLabel: 'Primary market',
      runtimeModeLabel: 'Runtime mode',
      runtimeModeArmed: 'Armed workspace',
      runtimeModeObservation: 'Observation workspace',
      readinessArmed: 'Armed',
      readinessNearlyReady: 'Nearly Ready',
      readinessObservation: 'Observation Mode',
    },
    commandDeck: {
      title: 'Command Deck',
      readinessScoreLabel: 'Readiness score',
      note:
        'Strategy snapshots in the lab section are seeded UI data for this pass and are ready to be replaced by backend validation endpoints next.',
      cexEngineLabel: 'CEX engine',
      dexSniperLabel: 'DEX sniper',
      vaultKeyLabel: 'Vault key',
      perTradeBudgetLabel: 'Per-trade budget',
      enabled: 'Enabled',
      standby: 'Standby',
      present: 'Present',
      missing: 'Missing',
    },
    navigation: {
      kicker: 'Module Navigation',
      title: 'Switch dashboard surfaces from the header.',
      description:
        'Phase 1 builds the header navigation and module shell first. Orders, scanner, and logs will attach to real data in the next stages.',
      activeModuleLabel: 'Active module',
      summaryExecutions: 'Executions',
      summaryStrategies: 'Strategies',
      summaryLocale: 'Locale',
      tabs: {
        overview: {
          label: 'Overview',
          description: 'System pulse and deployment posture',
        },
        orders: {
          label: 'Orders',
          description: 'Live and simulated trading panel',
        },
        scanner: {
          label: 'Scanner',
          description: 'Token discovery and filter outcomes',
        },
        logs: {
          label: 'Logs',
          description: 'Backend runtime and troubleshooting',
        },
        strategies: {
          label: 'Strategies',
          description: 'Validation and backtest entrypoint',
        },
        settings: {
          label: 'Settings',
          description: 'Vault and engine controls',
        },
      },
      placeholders: {
        orders: {
          title: 'Orders module scaffold',
          description:
            'This module will carry current orders, positions, fills, and risk actions, with a clear split between live and simulated trading.',
          point1: 'Frontend will add order lists, position views, and fill detail drawers.',
          point2: 'Backend will expose `/api/orders`, `/api/trades`, and `/api/positions`.',
          point3: 'Filters for strategy, mode, and symbol will follow.',
        },
        scanner: {
          title: 'Token scanner scaffold',
          description:
            'This module will show the last seven days of token scan results and make rejection reasons visible.',
          point1: 'Frontend will add filters, status tags, and failure-reason grouping.',
          point2: 'Backend needs a scan result table and `/api/token-scans`.',
          point3: 'A rolling seven-day retention job will be added.',
        },
        logs: {
          title: 'Runtime logs scaffold',
          description:
            'This module will expose backend logs in the frontend for strategy execution, scanner, and risk troubleshooting.',
          point1: 'Frontend will support refresh, level filters, and keyword search.',
          point2: 'Backend needs standardized log fields and `/api/logs`.',
          point3: 'SSE-based streaming can be added later.',
        },
      },
    },
    overview: {
      kicker: 'Overview',
      title: 'System pulse and deployment posture',
      totalExecutionsLabel: 'Total Executions',
      totalExecutionsMeta: 'All recorded fills',
      alphaWinRateLabel: 'Alpha Win Rate',
      alphaWinRateMeta: 'Realized close outcomes',
      activeTokensLabel: 'Active Tokens',
      activeTokensMeta: 'Tracked on-chain candidates',
      validatedStrategiesLabel: 'Validated Strategies',
      validatedStrategiesMeta: '{count} more under observation',
    },
    posture: {
      title: 'Verification posture',
      validated: 'Validated',
      watchlist: 'Watchlist',
      tuning: 'Tuning',
      validatedConfidence: 'Validated confidence',
      executionReadiness: 'Execution readiness',
      telemetryCoverage: 'Telemetry coverage',
      notesTitle: "Today's operating notes",
      note1: 'Vault controls are separated from the strategy review surface.',
      note2: 'The backtest-first workflow is now the center of the home screen.',
      note3: 'The current frontend structure is ready for validation runs, fills, positions, and alerts next.',
    },
    strategyLab: {
      kicker: 'Strategy Lab',
      title: 'Daily validation board',
      seededTag: 'Seeded until backend endpoints arrive',
      confidence: 'Confidence',
      winRate: 'Win rate',
      drawdown: 'Drawdown',
      expectancy: 'Expectancy',
      validationFeedTitle: 'Validation feed',
      apiTargetsTitle: 'Immediate API targets',
      apiTargetOverview: '`GET /api/overview` for richer top-line stats',
      apiTargetValidations: '`GET /api/validations` for daily strategy runs',
      apiTargetExecution: '`GET /api/execution-feed` for fills, alerts, and positions',
      statuses: {
        Validated: 'Validated',
        Watch: 'Watch',
        Tuning: 'Tuning',
      },
      strategies: {
        ema_macd_rsi: {
          label: 'EMA / MACD / RSI',
          market: 'BNBUSDT · 15m',
          cadence: 'Daily verification',
          thesis: 'Trend filters remain the cleanest on directional sessions and are still the closest to deployment.',
        },
        momentum_breakout: {
          label: 'Momentum Breakout',
          market: 'BNBUSDT · 15m',
          cadence: 'Daily verification',
          thesis: 'Upside capture remains strong, but volatility clusters still widen drawdown during chop.',
        },
        grid_mm: {
          label: 'Grid Market Making',
          market: 'BNBUSDT · Range',
          cadence: 'Intraday validation',
          thesis: 'Fill quality is attractive, but reduce-only transitions still need stronger operational confidence.',
        },
        simple_mm: {
          label: 'Simple Market Making',
          market: 'BNBUSDT · Microstructure',
          cadence: 'Daily verification',
          thesis: 'Useful as a benchmark strategy for inventory control and future market-making variants.',
        },
      },
      feed: {
        item1Title: 'Daily validation batch finished',
        item1Detail: 'Four strategies replayed on the latest 1,500-candle window.',
        item2Title: 'Breakout profile moved to watchlist',
        item2Detail: 'The momentum model stayed profitable, but drawdown expanded versus the prior day.',
        item3Title: 'DEX telemetry ingested',
        item3Detail: 'Tracked token counts have been rolled into dashboard metrics.',
        item4Title: 'Next integration target',
        item4Detail: 'Replace seeded strategy snapshots with validation-run endpoints from the Go backend.',
      },
    },
    settings: {
      kicker: 'Settings',
      title: 'Vault and engine controls',
      liveConfig: 'Live config',
      dexSniperTitle: 'DEX Sniper Engine',
      dexSniperDesc: 'Auto-buy on supported pools and exit quickly on risk events',
      cexEngineTitle: 'CEX Quant Engine',
      cexEngineDesc: 'Allow validated strategies to reach Binance Futures execution',
      evmPrivateKey: 'EVM private key',
      evmPlaceholderPresent: 'Vault key already configured',
      evmPlaceholderMissing: 'Enter 0x-less private key',
      tradeBudget: 'Per-trade budget',
      tradeBudgetPlaceholder: '50',
      binanceApiKey: 'Binance API key',
      binanceApiKeyPlaceholder: 'Enter API key',
      binanceApiSecret: 'Binance API secret',
      binanceApiSecretPlaceholder: 'Enter API secret',
      saveIdle: 'Commit to secure enclave',
      saveSaving: 'Committing secure update...',
      operationalPostureTitle: 'Operational posture',
      vaultIsolationTitle: 'Vault isolation',
      vaultIsolationConfigured: 'Execution key detected and masked in the UI.',
      vaultIsolationMissing: 'Execution key is still missing.',
      strategyExecutionTitle: 'Strategy execution',
      strategyExecutionEnabled: 'The live execution switch is enabled.',
      strategyExecutionStandby: 'The system is still running in observation mode.',
      onchainPostureTitle: 'On-chain posture',
      onchainPostureEnabled: 'Sniper monitoring is armed.',
      onchainPostureStandby: 'Sniper monitoring remains disabled.',
      nextPhaseTitle: 'Next UI phase',
      phase1: 'Add validation history with filtering and drill-down.',
      phase2: 'Expose fills, positions, and risk alerts from backend APIs.',
      phase3: 'Introduce strategy compare views and parameter sweep summaries.',
    },
  },
  ja: {
    language: {
      label: '言語',
    },
    common: {
      notAvailable: 'なし',
      liveMetrics: 'ライブ指標',
      reviewNext: '次を確認',
      perTradeBudgetValue: '{value} USDT',
      readinessScoreValue: '{value}/3',
      validatedStrategiesValue: '{validated}/{total}',
      watchCountMeta: 'さらに {count} 件を監視中',
      saveSuccess: '設定を更新し、安全に保存しました。',
      saveError: '設定の保存に失敗しました。',
    },
    hero: {
      eyebrow: '日次検証コンソール',
      kicker: 'Nexus 戦略デスク',
      title: 'リサーチ確認、稼働準備、キー管理を 1 つの画面で扱います。',
      copy:
        'バックエンドはすでに日次で戦略を検証しています。このフロントエンド改修では、実資金を有効にする前にシグナル品質、稼働姿勢、システム準備状況を確認する運用画面に切り替えます。',
      verificationCadenceLabel: '検証頻度',
      verificationCadenceValue: '毎日',
      primaryMarketLabel: '主要市場',
      runtimeModeLabel: '実行モード',
      runtimeModeArmed: '実行可能ワークスペース',
      runtimeModeObservation: '観測ワークスペース',
      readinessArmed: '準備完了',
      readinessNearlyReady: 'ほぼ準備完了',
      readinessObservation: '観測モード',
    },
    commandDeck: {
      title: 'コマンドデッキ',
      readinessScoreLabel: '準備スコア',
      note:
        'ラボセクションの戦略スナップショットは現時点では UI の種データです。次の段階でバックエンドの検証 API に置き換えられます。',
      cexEngineLabel: 'CEX エンジン',
      dexSniperLabel: 'DEX スナイパー',
      vaultKeyLabel: '鍵の状態',
      perTradeBudgetLabel: '1 回あたり予算',
      enabled: '有効',
      standby: '待機',
      present: '設定済み',
      missing: '未設定',
    },
    navigation: {
      kicker: 'モジュールナビゲーション',
      title: 'ヘッダーから業務モジュールを切り替えます。',
      description:
        'Phase 1 ではヘッダーとモジュール切替の土台を先に整えます。Orders、Scanner、Logs は次の段階で実データに接続します。',
      activeModuleLabel: '現在のモジュール',
      summaryExecutions: '実行記録',
      summaryStrategies: '戦略数',
      summaryLocale: '言語',
      tabs: {
        overview: {
          label: '概要',
          description: 'システム状態と稼働準備',
        },
        orders: {
          label: '注文',
          description: '本番とシミュレーションの取引画面',
        },
        scanner: {
          label: 'スキャン',
          description: 'Token 発見とフィルタ結果',
        },
        logs: {
          label: 'ログ',
          description: 'バックエンド実行状況と調査',
        },
        strategies: {
          label: '戦略',
          description: '検証とバックテストの入口',
        },
        settings: {
          label: '設定',
          description: 'ボールトとエンジン制御',
        },
      },
      placeholders: {
        orders: {
          title: '注文モジュールの土台',
          description:
            'このモジュールでは現在の注文、ポジション、約定、リスク動作を表示し、本番取引とシミュレーション取引を明確に分けます。',
          point1: 'フロントエンドでは注文一覧、ポジション表示、約定詳細ドロワーを追加します。',
          point2: 'バックエンドでは `/api/orders`、`/api/trades`、`/api/positions` を提供します。',
          point3: '戦略、モード、銘柄での絞り込みを後続で追加します。',
        },
        scanner: {
          title: 'Token スキャナの土台',
          description:
            'このモジュールでは直近 7 日間の Token スキャン結果を表示し、拒否理由も確認できるようにします。',
          point1: 'フロントエンドではフィルタ、状態タグ、失敗理由グループを追加します。',
          point2: 'バックエンドではスキャン結果テーブルと `/api/token-scans` が必要です。',
          point3: '7 日保持の定期クリーンアップも追加します。',
        },
        logs: {
          title: '実行ログの土台',
          description:
            'このモジュールでは戦略実行、スキャン、リスク対応の確認のためにバックエンドログをフロントエンドで閲覧可能にします。',
          point1: 'フロントエンドでは更新、レベル絞り込み、キーワード検索を追加します。',
          point2: 'バックエンドでは標準化ログ項目と `/api/logs` が必要です。',
          point3: '後続で SSE ストリーミングにも拡張できます。',
        },
      },
    },
    overview: {
      kicker: '概要',
      title: 'システム状態と稼働姿勢',
      totalExecutionsLabel: '総実行回数',
      totalExecutionsMeta: '記録済みの全約定',
      alphaWinRateLabel: 'Alpha 勝率',
      alphaWinRateMeta: '確定済みクローズ結果',
      activeTokensLabel: 'アクティブトークン',
      activeTokensMeta: '追跡中のオンチェーン候補',
      validatedStrategiesLabel: '検証済み戦略',
      validatedStrategiesMeta: 'さらに {count} 件を監視中',
    },
    posture: {
      title: '検証姿勢',
      validated: '検証済み',
      watchlist: '監視中',
      tuning: '調整中',
      validatedConfidence: '検証信頼度',
      executionReadiness: '実行準備度',
      telemetryCoverage: 'テレメトリ網羅率',
      notesTitle: '本日の運用メモ',
      note1: 'キー管理は戦略レビュー面から分離されています。',
      note2: 'バックテスト優先のワークフローがホーム画面の中心になりました。',
      note3: '現在のフロントエンド構成は、検証履歴、約定、ポジション、アラートの追加に対応できます。',
    },
    strategyLab: {
      kicker: '戦略ラボ',
      title: '日次検証ボード',
      seededTag: 'バックエンド API が来るまで種データを使用',
      confidence: '信頼度',
      winRate: '勝率',
      drawdown: 'ドローダウン',
      expectancy: '期待値',
      validationFeedTitle: '検証フィード',
      apiTargetsTitle: '次の API 目標',
      apiTargetOverview: '`GET /api/overview` でより豊富な概要指標を取得',
      apiTargetValidations: '`GET /api/validations` で日次戦略検証を取得',
      apiTargetExecution: '`GET /api/execution-feed` で約定、アラート、ポジションを取得',
      statuses: {
        Validated: '検証済み',
        Watch: '監視中',
        Tuning: '調整中',
      },
      strategies: {
        ema_macd_rsi: {
          label: 'EMA / MACD / RSI',
          market: 'BNBUSDT · 15m',
          cadence: '日次検証',
          thesis: '方向性のある相場ではトレンドフィルタが最も安定しており、現在最も本番に近い戦略です。',
        },
        momentum_breakout: {
          label: 'モメンタム・ブレイクアウト',
          market: 'BNBUSDT · 15m',
          cadence: '日次検証',
          thesis: '上昇局面の捕捉力は高いままですが、もみ合い相場ではドローダウンが広がります。',
        },
        grid_mm: {
          label: 'グリッド・マーケットメイク',
          market: 'BNBUSDT · レンジ',
          cadence: '日中検証',
          thesis: '約定品質は良好ですが、reduce-only への遷移には運用面での確証がまだ必要です。',
        },
        simple_mm: {
          label: 'シンプル・マーケットメイク',
          market: 'BNBUSDT · マイクロ構造',
          cadence: '日次検証',
          thesis: '在庫制御や将来のマーケットメイク戦略を比較する基準として有用です。',
        },
      },
      feed: {
        item1Title: '日次検証バッチ完了',
        item1Detail: '4 つの戦略を最新 1500 本のローソク足で再生しました。',
        item2Title: 'ブレイクアウト戦略を監視に移動',
        item2Detail: 'モメンタム戦略は依然として利益を維持していますが、前日よりドローダウンが拡大しました。',
        item3Title: 'DEX テレメトリを取り込み',
        item3Detail: '追跡トークン数がダッシュボード指標に反映されました。',
        item4Title: '次の統合対象',
        item4Detail: 'Go バックエンドの検証 API で種データの戦略スナップショットを置き換えます。',
      },
    },
    settings: {
      kicker: '設定',
      title: 'ボールトとエンジン制御',
      liveConfig: 'ライブ設定',
      dexSniperTitle: 'DEX スナイパーエンジン',
      dexSniperDesc: '対応プールで自動購入し、リスク時には即座に離脱します',
      cexEngineTitle: 'CEX クオンツエンジン',
      cexEngineDesc: '検証済み戦略を Binance Futures 実行に送ります',
      evmPrivateKey: 'EVM 秘密鍵',
      evmPlaceholderPresent: '秘密鍵はすでに設定済みです',
      evmPlaceholderMissing: '0x なしの秘密鍵を入力',
      tradeBudget: '1 回あたり予算',
      tradeBudgetPlaceholder: '50',
      binanceApiKey: 'Binance API キー',
      binanceApiKeyPlaceholder: 'API キーを入力',
      binanceApiSecret: 'Binance API シークレット',
      binanceApiSecretPlaceholder: 'API シークレットを入力',
      saveIdle: '安全ボールトへ保存',
      saveSaving: '安全な更新を保存中...',
      operationalPostureTitle: '運用姿勢',
      vaultIsolationTitle: '鍵の分離',
      vaultIsolationConfigured: '実行鍵が検出され、UI ではマスク表示されています。',
      vaultIsolationMissing: '実行鍵はまだ未設定です。',
      strategyExecutionTitle: '戦略実行',
      strategyExecutionEnabled: 'ライブ実行スイッチは有効です。',
      strategyExecutionStandby: 'システムはまだ観測モードで稼働しています。',
      onchainPostureTitle: 'オンチェーン姿勢',
      onchainPostureEnabled: 'スナイパー監視は有効です。',
      onchainPostureStandby: 'スナイパー監視は無効のままです。',
      nextPhaseTitle: '次の UI フェーズ',
      phase1: '絞り込みとドリルダウン付きの検証履歴を追加。',
      phase2: 'バックエンド API から約定、ポジション、リスクアラートを公開。',
      phase3: '戦略比較画面とパラメータスイープ要約を追加。',
    },
  },
  ko: {
    language: {
      label: '언어',
    },
    common: {
      notAvailable: '없음',
      liveMetrics: '실시간 지표',
      reviewNext: '다음 검토',
      perTradeBudgetValue: '{value} USDT',
      readinessScoreValue: '{value}/3',
      validatedStrategiesValue: '{validated}/{total}',
      watchCountMeta: '추가로 {count}개 전략 관찰 중',
      saveSuccess: '설정이 업데이트되었고 안전하게 저장되었습니다.',
      saveError: '설정 저장에 실패했습니다.',
    },
    hero: {
      eyebrow: '일일 검증 콘솔',
      kicker: 'Nexus 전략 데스크',
      title: '연구 검토, 실행 준비, 키 관리를 하나의 화면에서 처리합니다.',
      copy:
        '백엔드는 이미 매일 전략을 검증하고 있습니다. 이번 프런트엔드 개편은 실제 자금을 켜기 전에 신호 품질, 운영 자세, 시스템 준비 상태를 검토하는 운영 화면으로 바꿉니다.',
      verificationCadenceLabel: '검증 주기',
      verificationCadenceValue: '매일',
      primaryMarketLabel: '주요 시장',
      runtimeModeLabel: '실행 모드',
      runtimeModeArmed: '실행 가능한 워크스페이스',
      runtimeModeObservation: '관찰 워크스페이스',
      readinessArmed: '준비 완료',
      readinessNearlyReady: '거의 준비됨',
      readinessObservation: '관찰 모드',
    },
    commandDeck: {
      title: '커맨드 덱',
      readinessScoreLabel: '준비 점수',
      note:
        '랩 섹션의 전략 스냅샷은 현재 UI 시드 데이터입니다. 다음 단계에서 백엔드 검증 API 결과로 교체할 수 있습니다.',
      cexEngineLabel: 'CEX 엔진',
      dexSniperLabel: 'DEX 스나이퍼',
      vaultKeyLabel: '키 상태',
      perTradeBudgetLabel: '거래당 예산',
      enabled: '활성',
      standby: '대기',
      present: '설정됨',
      missing: '없음',
    },
    navigation: {
      kicker: '모듈 내비게이션',
      title: '헤더에서 업무 모듈을 전환합니다.',
      description:
        'Phase 1 에서는 헤더와 모듈 전환 프레임만 먼저 구축합니다. Orders, Scanner, Logs 는 다음 단계에서 실제 데이터와 연결합니다.',
      activeModuleLabel: '현재 모듈',
      summaryExecutions: '실행 기록',
      summaryStrategies: '전략 수',
      summaryLocale: '언어',
      tabs: {
        overview: {
          label: '개요',
          description: '시스템 상태와 실행 준비',
        },
        orders: {
          label: '주문',
          description: '실거래와 모의거래 패널',
        },
        scanner: {
          label: '스캐너',
          description: 'Token 탐지와 필터 결과',
        },
        logs: {
          label: '로그',
          description: '백엔드 실행 상태와 장애 분석',
        },
        strategies: {
          label: '전략',
          description: '검증과 백테스트 진입점',
        },
        settings: {
          label: '설정',
          description: '볼트와 엔진 제어',
        },
      },
      placeholders: {
        orders: {
          title: '주문 모듈 골격',
          description:
            '이 모듈은 현재 주문, 포지션, 체결, 리스크 동작을 담고 실거래와 모의거래를 명확히 구분합니다.',
          point1: '프런트엔드는 주문 목록, 포지션 뷰, 체결 상세 드로어를 추가합니다.',
          point2: '백엔드는 `/api/orders`, `/api/trades`, `/api/positions` 를 제공합니다.',
          point3: '전략, 모드, 심볼 필터는 다음 단계에서 추가합니다.',
        },
        scanner: {
          title: 'Token 스캐너 골격',
          description:
            '이 모듈은 최근 7일의 Token 스캔 결과를 보여주고 탈락 사유를 명확히 드러냅니다.',
          point1: '프런트엔드는 필터, 상태 태그, 실패 사유 그룹을 추가합니다.',
          point2: '백엔드는 스캔 결과 테이블과 `/api/token-scans` 가 필요합니다.',
          point3: '7일 보관과 정기 정리 작업을 추가합니다.',
        },
        logs: {
          title: '실행 로그 골격',
          description:
            '이 모듈은 전략 실행, 스캐너, 리스크 대응 문제를 추적하기 위해 백엔드 로그를 프런트엔드에서 볼 수 있게 합니다.',
          point1: '프런트엔드는 새로고침, 레벨 필터, 키워드 검색을 지원합니다.',
          point2: '백엔드는 표준 로그 필드와 `/api/logs` 가 필요합니다.',
          point3: '추후 SSE 기반 스트리밍으로 확장할 수 있습니다.',
        },
      },
    },
    overview: {
      kicker: '개요',
      title: '시스템 상태와 실행 준비',
      totalExecutionsLabel: '총 실행 횟수',
      totalExecutionsMeta: '기록된 전체 체결',
      alphaWinRateLabel: 'Alpha 승률',
      alphaWinRateMeta: '청산 완료 결과',
      activeTokensLabel: '활성 토큰',
      activeTokensMeta: '추적 중인 온체인 후보',
      validatedStrategiesLabel: '검증 완료 전략',
      validatedStrategiesMeta: '추가로 {count}개 전략 관찰 중',
    },
    posture: {
      title: '검증 상태',
      validated: '검증 완료',
      watchlist: '관찰 목록',
      tuning: '튜닝 중',
      validatedConfidence: '검증 신뢰도',
      executionReadiness: '실행 준비도',
      telemetryCoverage: '텔레메트리 커버리지',
      notesTitle: '오늘의 운영 메모',
      note1: '볼트 제어는 전략 검토 화면과 분리되어 있습니다.',
      note2: '백테스트 우선 워크플로가 이제 홈 화면의 중심입니다.',
      note3: '현재 프런트엔드 구조는 검증 기록, 체결, 포지션, 알림을 다음 단계에서 수용할 수 있습니다.',
    },
    strategyLab: {
      kicker: '전략 랩',
      title: '일일 검증 보드',
      seededTag: '백엔드 엔드포인트 준비 전까지 시드 데이터 사용',
      confidence: '신뢰도',
      winRate: '승률',
      drawdown: '드로다운',
      expectancy: '기대값',
      validationFeedTitle: '검증 피드',
      apiTargetsTitle: '다음 API 목표',
      apiTargetOverview: '`GET /api/overview` 로 더 풍부한 상단 지표 제공',
      apiTargetValidations: '`GET /api/validations` 로 일일 전략 검증 기록 제공',
      apiTargetExecution: '`GET /api/execution-feed` 로 체결, 알림, 포지션 제공',
      statuses: {
        Validated: '검증 완료',
        Watch: '관찰 중',
        Tuning: '튜닝 중',
      },
      strategies: {
        ema_macd_rsi: {
          label: 'EMA / MACD / RSI',
          market: 'BNBUSDT · 15m',
          cadence: '일일 검증',
          thesis: '추세 필터는 방향성 장세에서 가장 안정적이며 현재 가장 실전 배치에 가깝습니다.',
        },
        momentum_breakout: {
          label: '모멘텀 돌파',
          market: 'BNBUSDT · 15m',
          cadence: '일일 검증',
          thesis: '상승 구간 포착력은 여전히 강하지만 횡보 구간에서는 드로다운이 커집니다.',
        },
        grid_mm: {
          label: '그리드 마켓메이킹',
          market: 'BNBUSDT · 박스권',
          cadence: '장중 검증',
          thesis: '체결 품질은 좋지만 reduce-only 전환 구간은 아직 운영 확신이 더 필요합니다.',
        },
        simple_mm: {
          label: '단순 마켓메이킹',
          market: 'BNBUSDT · 마이크로구조',
          cadence: '일일 검증',
          thesis: '재고 제어와 향후 마켓메이킹 변형 전략을 비교하는 기준 전략으로 유용합니다.',
        },
      },
      feed: {
        item1Title: '일일 검증 배치 완료',
        item1Detail: '4개 전략이 최신 1500개 캔들 구간에서 재생되었습니다.',
        item2Title: '돌파 전략을 관찰 목록으로 이동',
        item2Detail: '모멘텀 모델은 여전히 수익성이 있지만 전일 대비 드로다운이 확대되었습니다.',
        item3Title: 'DEX 텔레메트리 반영',
        item3Detail: '추적 토큰 수가 대시보드 지표에 반영되었습니다.',
        item4Title: '다음 통합 목표',
        item4Detail: 'Go 백엔드의 검증 엔드포인트로 시드 전략 스냅샷을 교체합니다.',
      },
    },
    settings: {
      kicker: '설정',
      title: '볼트와 엔진 제어',
      liveConfig: '실시간 설정',
      dexSniperTitle: 'DEX 스나이퍼 엔진',
      dexSniperDesc: '지원 풀에서 자동 매수하고 리스크 이벤트 시 빠르게 이탈합니다',
      cexEngineTitle: 'CEX 퀀트 엔진',
      cexEngineDesc: '검증된 전략을 Binance Futures 실행으로 전달합니다',
      evmPrivateKey: 'EVM 개인키',
      evmPlaceholderPresent: '볼트 키가 이미 설정되었습니다',
      evmPlaceholderMissing: '0x 없는 개인키 입력',
      tradeBudget: '거래당 예산',
      tradeBudgetPlaceholder: '50',
      binanceApiKey: 'Binance API 키',
      binanceApiKeyPlaceholder: 'API 키 입력',
      binanceApiSecret: 'Binance API 시크릿',
      binanceApiSecretPlaceholder: 'API 시크릿 입력',
      saveIdle: '보안 볼트에 저장',
      saveSaving: '보안 업데이트 저장 중...',
      operationalPostureTitle: '운영 자세',
      vaultIsolationTitle: '키 격리',
      vaultIsolationConfigured: '실행 키가 감지되었고 UI 에서는 마스킹됩니다.',
      vaultIsolationMissing: '실행 키가 아직 없습니다.',
      strategyExecutionTitle: '전략 실행',
      strategyExecutionEnabled: '실거래 실행 스위치가 활성화되었습니다.',
      strategyExecutionStandby: '시스템은 아직 관찰 모드로 동작 중입니다.',
      onchainPostureTitle: '온체인 자세',
      onchainPostureEnabled: '스나이퍼 모니터링이 활성화되었습니다.',
      onchainPostureStandby: '스나이퍼 모니터링은 비활성 상태입니다.',
      nextPhaseTitle: '다음 UI 단계',
      phase1: '필터와 드릴다운이 있는 검증 이력 추가.',
      phase2: '백엔드 API 에서 체결, 포지션, 리스크 알림 노출.',
      phase3: '전략 비교 화면과 파라미터 스윕 요약 추가.',
    },
  },
}

const I18nContext = createContext<I18nContextValue | null>(null)

function resolveInitialLocale(): Locale {
  if (typeof window === 'undefined') {
    return 'en'
  }

  const savedLocale = window.localStorage.getItem(LOCALE_STORAGE_KEY)
  if (savedLocale && isLocale(savedLocale)) {
    return savedLocale
  }

  const browserLocale = window.navigator.language
  if (browserLocale.startsWith('zh')) {
    return 'zh-CN'
  }
  if (browserLocale.startsWith('ja')) {
    return 'ja'
  }
  if (browserLocale.startsWith('ko')) {
    return 'ko'
  }

  return 'en'
}

function isLocale(value: string): value is Locale {
  return value === 'zh-CN' || value === 'en' || value === 'ja' || value === 'ko'
}

function resolveTranslation(locale: Locale, key: string): string {
  const segments = key.split('.')
  let current: string | TranslationTree = translations[locale]

  for (const segment of segments) {
    if (typeof current === 'string') {
      return key
    }

    current = current[segment]
    if (current === undefined) {
      return key
    }
  }

  return typeof current === 'string' ? current : key
}

export function interpolate(template: string, values: Record<string, string | number>) {
  return Object.entries(values).reduce((result, [key, value]) => {
    return result.replaceAll(`{${key}}`, String(value))
  }, template)
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(resolveInitialLocale)

  useEffect(() => {
    document.documentElement.lang = locale
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale)
  }, [locale])

  const value: I18nContextValue = {
    locale,
    setLocale: setLocaleState,
    t: (key: string) => resolveTranslation(locale, key),
  }

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const context = useContext(I18nContext)
  if (!context) {
    throw new Error('useI18n must be used within I18nProvider')
  }

  return context
}
