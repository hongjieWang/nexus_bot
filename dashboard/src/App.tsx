import { useEffect, useState } from 'react'
import axios from 'axios'
import {
  Activity,
  AlertTriangle,
  ArrowUpRight,
  Bot,
  CandlestickChart,
  CheckCircle2,
  Cpu,
  Fingerprint,
  Globe,
  KeyRound,
  Layers3,
  Lock,
  Radar,
  Save,
  ShieldCheck,
  Sparkles,
  Target,
  TimerReset,
  Wallet,
  Waves,
} from 'lucide-react'
import './App.css'
import { interpolate, localeOptions, useI18n } from './i18n'

const API_BASE = import.meta.env.VITE_API_BASE || '/api'

interface Metrics {
  total_trades?: number
  win_rate?: number
  active_tokens?: number
}

type StrategyStatus = 'Validated' | 'Watch' | 'Tuning'

interface StrategySeed {
  id: string
  status: StrategyStatus
  confidence: string
  winRate: string
  drawdown: string
  expectancy: string
}

interface FeedSeed {
  time: string
  tone: 'good' | 'warn' | 'neutral'
  titleKey: string
  detailKey: string
}

const strategySeeds: StrategySeed[] = [
  {
    id: 'ema_macd_rsi',
    status: 'Validated',
    confidence: 'A-',
    winRate: '58.3%',
    drawdown: '8.2%',
    expectancy: '+1.36R',
  },
  {
    id: 'momentum_breakout',
    status: 'Watch',
    confidence: 'B+',
    winRate: '51.8%',
    drawdown: '11.4%',
    expectancy: '+0.82R',
  },
  {
    id: 'grid_mm',
    status: 'Tuning',
    confidence: 'B',
    winRate: '63.1%',
    drawdown: '13.7%',
    expectancy: '+0.54R',
  },
  {
    id: 'simple_mm',
    status: 'Watch',
    confidence: 'B-',
    winRate: '49.6%',
    drawdown: '9.1%',
    expectancy: '+0.28R',
  },
]

const feedSeeds: FeedSeed[] = [
  {
    time: '09:10',
    tone: 'good',
    titleKey: 'strategyLab.feed.item1Title',
    detailKey: 'strategyLab.feed.item1Detail',
  },
  {
    time: '09:14',
    tone: 'warn',
    titleKey: 'strategyLab.feed.item2Title',
    detailKey: 'strategyLab.feed.item2Detail',
  },
  {
    time: '09:19',
    tone: 'neutral',
    titleKey: 'strategyLab.feed.item3Title',
    detailKey: 'strategyLab.feed.item3Detail',
  },
  {
    time: '09:22',
    tone: 'neutral',
    titleKey: 'strategyLab.feed.item4Title',
    detailKey: 'strategyLab.feed.item4Detail',
  },
]

function formatNumber(locale: string, value: number, options?: Intl.NumberFormatOptions) {
  return new Intl.NumberFormat(locale, options).format(value)
}

function App() {
  const { locale, setLocale, t } = useI18n()
  const [metrics, setMetrics] = useState<Metrics | null>(null)
  const [dexEnabled, setDexEnabled] = useState(false)
  const [tradeEnabled, setTradeEnabled] = useState(false)
  const [execKey, setExecKey] = useState('')
  const [binanceKey, setBinanceKey] = useState('')
  const [binanceSecret, setBinanceSecret] = useState('')
  const [tradeUsdt, setTradeUsdt] = useState('50')
  const [hasPrivateKey, setHasPrivateKey] = useState(false)
  const [status, setStatus] = useState<{ type: 'success' | 'error'; msg: string } | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetchMetrics()
    fetchConfig()

    const interval = setInterval(() => {
      fetchMetrics()
      fetchConfig()
    }, 10000)

    return () => clearInterval(interval)
  }, [])

  const fetchMetrics = async () => {
    try {
      const res = await axios.get(`${API_BASE}/metrics`)
      setMetrics(res.data)
    } catch (err) {
      console.error('Failed to fetch metrics', err)
    }
  }

  const fetchConfig = async () => {
    try {
      const res = await axios.get(`${API_BASE}/config`)
      setDexEnabled(Boolean(res.data?.dex_sniper_enabled))
      setTradeEnabled(Boolean(res.data?.trade_enabled))
      setHasPrivateKey(Boolean(res.data?.has_private_key))
      setTradeUsdt(String(res.data?.trade_usdt || '50'))
    } catch (err) {
      console.error('Failed to fetch config', err)
    }
  }

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setStatus(null)

    try {
      const payload = {
        DEX_SNIPER_ENABLED: dexEnabled ? 'true' : 'false',
        TRADE_ENABLED: tradeEnabled ? 'true' : 'false',
        EXECUTION_PRIVATE_KEY: execKey,
        BINANCE_API_KEY: binanceKey,
        BINANCE_API_SECRET: binanceSecret,
        TRADE_USDT: tradeUsdt,
      }

      await axios.post(`${API_BASE}/config`, payload)
      setStatus({ type: 'success', msg: t('common.saveSuccess') })

      setExecKey('')
      setBinanceKey('')
      setBinanceSecret('')

      fetchConfig()
      setTimeout(() => setStatus(null), 5000)
    } catch (err) {
      console.error('Failed to save config', err)
      setStatus({ type: 'error', msg: t('common.saveError') })
    } finally {
      setSaving(false)
    }
  }

  const safeMetrics = {
    total_trades: Number(metrics?.total_trades ?? 0),
    win_rate: Number(metrics?.win_rate ?? 0),
    active_tokens: Number(metrics?.active_tokens ?? 0),
  }

  const validatedCount = strategySeeds.filter((strategy) => strategy.status === 'Validated').length
  const watchCount = strategySeeds.filter((strategy) => strategy.status === 'Watch').length
  const readinessScore = [tradeEnabled, dexEnabled, hasPrivateKey].filter(Boolean).length
  const readinessLabel =
    readinessScore === 3
      ? t('hero.readinessArmed')
      : readinessScore === 2
        ? t('hero.readinessNearlyReady')
        : t('hero.readinessObservation')
  const readinessTone = readinessScore >= 2 ? 'good' : 'warn'

  const strategySnapshots = strategySeeds.map((strategy) => ({
    ...strategy,
    label: t(`strategyLab.strategies.${strategy.id}.label`),
    market: t(`strategyLab.strategies.${strategy.id}.market`),
    cadence: t(`strategyLab.strategies.${strategy.id}.cadence`),
    thesis: t(`strategyLab.strategies.${strategy.id}.thesis`),
    statusLabel: t(`strategyLab.statuses.${strategy.status}`),
  }))

  const verificationFeed = feedSeeds.map((item) => ({
    ...item,
    title: t(item.titleKey),
    detail: t(item.detailKey),
  }))

  const overviewCards = [
    {
      label: t('overview.totalExecutionsLabel'),
      value: formatNumber(locale, safeMetrics.total_trades),
      meta: t('overview.totalExecutionsMeta'),
      accent: 'cyan',
      icon: Activity,
    },
    {
      label: t('overview.alphaWinRateLabel'),
      value: `${formatNumber(locale, safeMetrics.win_rate, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`,
      meta: t('overview.alphaWinRateMeta'),
      accent: 'amber',
      icon: Target,
    },
    {
      label: t('overview.activeTokensLabel'),
      value: formatNumber(locale, safeMetrics.active_tokens),
      meta: t('overview.activeTokensMeta'),
      accent: 'teal',
      icon: Waves,
    },
    {
      label: t('overview.validatedStrategiesLabel'),
      value: interpolate(t('common.validatedStrategiesValue'), {
        validated: validatedCount,
        total: strategySeeds.length,
      }),
      meta: interpolate(t('common.watchCountMeta'), { count: watchCount }),
      accent: 'rose',
      icon: CandlestickChart,
    },
  ] as const

  const readinessItems = [
    {
      label: t('commandDeck.cexEngineLabel'),
      value: tradeEnabled ? t('commandDeck.enabled') : t('commandDeck.standby'),
      tone: tradeEnabled ? 'good' : 'neutral',
    },
    {
      label: t('commandDeck.dexSniperLabel'),
      value: dexEnabled ? t('commandDeck.enabled') : t('commandDeck.standby'),
      tone: dexEnabled ? 'good' : 'neutral',
    },
    {
      label: t('commandDeck.vaultKeyLabel'),
      value: hasPrivateKey ? t('commandDeck.present') : t('commandDeck.missing'),
      tone: hasPrivateKey ? 'good' : 'warn',
    },
    {
      label: t('commandDeck.perTradeBudgetLabel'),
      value: interpolate(t('common.perTradeBudgetValue'), { value: tradeUsdt || '0' }),
      tone: 'neutral',
    },
  ] as const

  return (
    <div className="app-shell">
      <div className="dashboard-layout">
        <section className="hero-grid">
          <div className="hero-panel masthead">
            <div className="masthead-toolbar">
              <div className="eyebrow">
                <Sparkles size={14} />
                {t('hero.eyebrow')}
              </div>

              <label className="locale-switcher">
                <span className="locale-switcher-label">
                  <Globe size={14} />
                  {t('language.label')}
                </span>
                <select value={locale} onChange={(e) => setLocale(e.target.value as typeof locale)}>
                  {localeOptions.map((option) => (
                    <option key={option.code} value={option.code}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <div className="masthead-topline">
              <div>
                <p className="hero-kicker">{t('hero.kicker')}</p>
                <h1 className="hero-title">{t('hero.title')}</h1>
              </div>

              <div className={`hero-status ${readinessTone}`}>
                <span className="hero-status-dot" />
                {readinessLabel}
              </div>
            </div>

            <p className="hero-copy">{t('hero.copy')}</p>

            <div className="hero-strip">
              <div className="hero-strip-item">
                <span className="strip-label">{t('hero.verificationCadenceLabel')}</span>
                <strong>{t('hero.verificationCadenceValue')}</strong>
              </div>
              <div className="hero-strip-item">
                <span className="strip-label">{t('hero.primaryMarketLabel')}</span>
                <strong>BNBUSDT</strong>
              </div>
              <div className="hero-strip-item">
                <span className="strip-label">{t('hero.runtimeModeLabel')}</span>
                <strong>{tradeEnabled || dexEnabled ? t('hero.runtimeModeArmed') : t('hero.runtimeModeObservation')}</strong>
              </div>
            </div>
          </div>

          <aside className="hero-panel command-panel">
            <div className="panel-caption">{t('commandDeck.title')}</div>
            <div className="command-score">
              <div>
                <span className="command-score-label">{t('commandDeck.readinessScoreLabel')}</span>
                <strong>{interpolate(t('common.readinessScoreValue'), { value: readinessScore })}</strong>
              </div>
              <ArrowUpRight size={18} />
            </div>

            <div className="readiness-list">
              {readinessItems.map((item) => (
                <div key={item.label} className="readiness-item">
                  <span>{item.label}</span>
                  <strong className={`tone-${item.tone}`}>{item.value}</strong>
                </div>
              ))}
            </div>

            <div className="command-note">
              <Radar size={16} />
              {t('commandDeck.note')}
            </div>
          </aside>
        </section>

        <section className="section-block">
          <div className="section-heading">
            <div>
              <p className="section-kicker">{t('overview.kicker')}</p>
              <h2>{t('overview.title')}</h2>
            </div>
            <span className="section-tag">{t('common.liveMetrics')}</span>
          </div>

          <div className="overview-grid">
            {overviewCards.map((card) => {
              const Icon = card.icon

              return (
                <article key={card.label} className={`overview-card accent-${card.accent}`}>
                  <div className="overview-card-top">
                    <span>{card.label}</span>
                    <Icon size={18} />
                  </div>
                  <div className="overview-value">{card.value}</div>
                  <p>{card.meta}</p>
                </article>
              )
            })}
          </div>

          <div className="signal-grid">
            <article className="insight-panel">
              <div className="panel-caption">{t('posture.title')}</div>
              <div className="signal-stat-row">
                <div>
                  <span className="signal-label">{t('posture.validated')}</span>
                  <strong>{formatNumber(locale, validatedCount)}</strong>
                </div>
                <div>
                  <span className="signal-label">{t('posture.watchlist')}</span>
                  <strong>{formatNumber(locale, watchCount)}</strong>
                </div>
                <div>
                  <span className="signal-label">{t('posture.tuning')}</span>
                  <strong>{formatNumber(locale, strategySeeds.length - validatedCount - watchCount)}</strong>
                </div>
              </div>
              <div className="signal-bars">
                <div className="signal-bar-row">
                  <span>{t('posture.validatedConfidence')}</span>
                  <div className="signal-bar">
                    <span style={{ width: '74%' }} />
                  </div>
                </div>
                <div className="signal-bar-row">
                  <span>{t('posture.executionReadiness')}</span>
                  <div className="signal-bar">
                    <span style={{ width: `${(readinessScore / 3) * 100}%` }} />
                  </div>
                </div>
                <div className="signal-bar-row">
                  <span>{t('posture.telemetryCoverage')}</span>
                  <div className="signal-bar">
                    <span style={{ width: '58%' }} />
                  </div>
                </div>
              </div>
            </article>

            <article className="insight-panel">
              <div className="panel-caption">{t('posture.notesTitle')}</div>
              <div className="checklist">
                <div className="checklist-item">
                  <ShieldCheck size={16} />
                  <span>{t('posture.note1')}</span>
                </div>
                <div className="checklist-item">
                  <Bot size={16} />
                  <span>{t('posture.note2')}</span>
                </div>
                <div className="checklist-item">
                  <Layers3 size={16} />
                  <span>{t('posture.note3')}</span>
                </div>
              </div>
            </article>
          </div>
        </section>

        <section className="section-block">
          <div className="section-heading">
            <div>
              <p className="section-kicker">{t('strategyLab.kicker')}</p>
              <h2>{t('strategyLab.title')}</h2>
            </div>
            <span className="section-tag">{t('strategyLab.seededTag')}</span>
          </div>

          <div className="lab-layout">
            <div className="strategy-board">
              {strategySnapshots.map((strategy) => (
                <article key={strategy.id} className="strategy-card">
                  <div className="strategy-card-top">
                    <div>
                      <p className="strategy-market">{strategy.market}</p>
                      <h3>{strategy.label}</h3>
                    </div>
                    <span className={`strategy-status status-${strategy.status.toLowerCase()}`}>{strategy.statusLabel}</span>
                  </div>

                  <p className="strategy-thesis">{strategy.thesis}</p>

                  <div className="strategy-metrics">
                    <div>
                      <span>{t('strategyLab.confidence')}</span>
                      <strong>{strategy.confidence}</strong>
                    </div>
                    <div>
                      <span>{t('strategyLab.winRate')}</span>
                      <strong>{strategy.winRate}</strong>
                    </div>
                    <div>
                      <span>{t('strategyLab.drawdown')}</span>
                      <strong>{strategy.drawdown}</strong>
                    </div>
                    <div>
                      <span>{t('strategyLab.expectancy')}</span>
                      <strong>{strategy.expectancy}</strong>
                    </div>
                  </div>

                  <div className="strategy-footer">
                    <span>{strategy.cadence}</span>
                    <span className="strategy-link">
                      {t('common.reviewNext')}
                      <ArrowUpRight size={14} />
                    </span>
                  </div>
                </article>
              ))}
            </div>

            <aside className="activity-panel">
              <div className="panel-caption">{t('strategyLab.validationFeedTitle')}</div>
              <div className="feed-list">
                {verificationFeed.map((item) => (
                  <div key={`${item.time}-${item.titleKey}`} className="feed-item">
                    <div className={`feed-dot tone-${item.tone}`} />
                    <div>
                      <div className="feed-item-top">
                        <strong>{item.title}</strong>
                        <span>{item.time}</span>
                      </div>
                      <p>{item.detail}</p>
                    </div>
                  </div>
                ))}
              </div>

              <div className="activity-divider" />

              <div className="panel-caption">{t('strategyLab.apiTargetsTitle')}</div>
              <div className="target-list">
                <div className="target-item">
                  <Cpu size={16} />
                  <span>{t('strategyLab.apiTargetOverview')}</span>
                </div>
                <div className="target-item">
                  <TimerReset size={16} />
                  <span>{t('strategyLab.apiTargetValidations')}</span>
                </div>
                <div className="target-item">
                  <Wallet size={16} />
                  <span>{t('strategyLab.apiTargetExecution')}</span>
                </div>
              </div>
            </aside>
          </div>
        </section>

        <section className="section-block settings-block">
          <div className="section-heading">
            <div>
              <p className="section-kicker">{t('settings.kicker')}</p>
              <h2>{t('settings.title')}</h2>
            </div>
            <span className="section-tag">{t('settings.liveConfig')}</span>
          </div>

          <div className="settings-layout">
            <div className="vault-panel">
              <form onSubmit={handleSaveConfig} className="vault-form">
                <div className="toggle-cluster">
                  <label className="toggle-row">
                    <div>
                      <span className="toggle-title">{t('settings.dexSniperTitle')}</span>
                      <span className="toggle-desc">{t('settings.dexSniperDesc')}</span>
                    </div>
                    <div className="toggle-control">
                      <input type="checkbox" checked={dexEnabled} onChange={(e) => setDexEnabled(e.target.checked)} />
                      <span className="switch" />
                    </div>
                  </label>

                  <label className="toggle-row">
                    <div>
                      <span className="toggle-title">{t('settings.cexEngineTitle')}</span>
                      <span className="toggle-desc">{t('settings.cexEngineDesc')}</span>
                    </div>
                    <div className="toggle-control">
                      <input type="checkbox" checked={tradeEnabled} onChange={(e) => setTradeEnabled(e.target.checked)} />
                      <span className="switch" />
                    </div>
                  </label>
                </div>

                <div className="vault-grid">
                  <label className="input-group">
                    <span>{t('settings.evmPrivateKey')}</span>
                    <div className="input-shell">
                      <KeyRound className="input-icon" size={16} />
                      <input
                        className="vault-input"
                        type="password"
                        placeholder={hasPrivateKey ? t('settings.evmPlaceholderPresent') : t('settings.evmPlaceholderMissing')}
                        value={execKey}
                        onChange={(e) => setExecKey(e.target.value)}
                        autoComplete="off"
                      />
                    </div>
                  </label>

                  <label className="input-group">
                    <span>{t('settings.tradeBudget')}</span>
                    <div className="input-shell">
                      <Wallet className="input-icon" size={16} />
                      <input
                        className="vault-input"
                        type="text"
                        placeholder={t('settings.tradeBudgetPlaceholder')}
                        value={tradeUsdt}
                        onChange={(e) => setTradeUsdt(e.target.value)}
                        autoComplete="off"
                      />
                    </div>
                  </label>

                  <label className="input-group">
                    <span>{t('settings.binanceApiKey')}</span>
                    <div className="input-shell">
                      <Fingerprint className="input-icon" size={16} />
                      <input
                        className="vault-input"
                        type="password"
                        placeholder={t('settings.binanceApiKeyPlaceholder')}
                        value={binanceKey}
                        onChange={(e) => setBinanceKey(e.target.value)}
                        autoComplete="off"
                      />
                    </div>
                  </label>

                  <label className="input-group">
                    <span>{t('settings.binanceApiSecret')}</span>
                    <div className="input-shell">
                      <Lock className="input-icon" size={16} />
                      <input
                        className="vault-input"
                        type="password"
                        placeholder={t('settings.binanceApiSecretPlaceholder')}
                        value={binanceSecret}
                        onChange={(e) => setBinanceSecret(e.target.value)}
                        autoComplete="off"
                      />
                    </div>
                  </label>
                </div>

                <button type="submit" className="save-button" disabled={saving}>
                  {saving ? (
                    t('settings.saveSaving')
                  ) : (
                    <>
                      <Save size={18} />
                      {t('settings.saveIdle')}
                    </>
                  )}
                </button>

                {status && (
                  <div className={`status-alert ${status.type}`}>
                    {status.type === 'success' ? <CheckCircle2 size={18} /> : <AlertTriangle size={18} />}
                    <span>{status.msg}</span>
                  </div>
                )}
              </form>
            </div>

            <aside className="operations-panel">
              <div className="ops-card">
                <div className="panel-caption">{t('settings.operationalPostureTitle')}</div>
                <div className="ops-state-list">
                  <div className="ops-state">
                    <ShieldCheck size={16} />
                    <div>
                      <strong>{t('settings.vaultIsolationTitle')}</strong>
                      <span>{hasPrivateKey ? t('settings.vaultIsolationConfigured') : t('settings.vaultIsolationMissing')}</span>
                    </div>
                  </div>
                  <div className="ops-state">
                    <Bot size={16} />
                    <div>
                      <strong>{t('settings.strategyExecutionTitle')}</strong>
                      <span>{tradeEnabled ? t('settings.strategyExecutionEnabled') : t('settings.strategyExecutionStandby')}</span>
                    </div>
                  </div>
                  <div className="ops-state">
                    <Waves size={16} />
                    <div>
                      <strong>{t('settings.onchainPostureTitle')}</strong>
                      <span>{dexEnabled ? t('settings.onchainPostureEnabled') : t('settings.onchainPostureStandby')}</span>
                    </div>
                  </div>
                </div>
              </div>

              <div className="ops-card">
                <div className="panel-caption">{t('settings.nextPhaseTitle')}</div>
                <div className="phase-list">
                  <div className="phase-item">
                    <span>01</span>
                    <p>{t('settings.phase1')}</p>
                  </div>
                  <div className="phase-item">
                    <span>02</span>
                    <p>{t('settings.phase2')}</p>
                  </div>
                  <div className="phase-item">
                    <span>03</span>
                    <p>{t('settings.phase3')}</p>
                  </div>
                </div>
              </div>
            </aside>
          </div>
        </section>
      </div>
    </div>
  )
}

export default App
