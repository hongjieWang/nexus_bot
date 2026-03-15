import { useEffect, useState } from 'react'
import axios from 'axios'
import { 
  Activity, 
  Terminal, 
  Target, 
  ShieldCheck, 
  Key, 
  Lock, 
  Save, 
  CheckCircle2, 
  AlertTriangle,
  Cpu,
  Fingerprint
} from 'lucide-react'
import './App.css'

const API_BASE = 'http://localhost:18080/api'

interface Metrics {
  total_trades: number
  win_rate: number
  active_tokens: number
}

function App() {
  const [metrics, setMetrics] = useState<Metrics | null>(null)
  
  // Config state
  const [dexEnabled, setDexEnabled] = useState(false)
  const [tradeEnabled, setTradeEnabled] = useState(false)
  const [execKey, setExecKey] = useState('')
  const [binanceKey, setBinanceKey] = useState('')
  const [binanceSecret, setBinanceSecret] = useState('')
  
  const [hasPrivateKey, setHasPrivateKey] = useState(false)
  const [status, setStatus] = useState<{type: 'success'|'error', msg: string} | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetchMetrics()
    fetchConfig()
    
    const interval = setInterval(fetchMetrics, 10000)
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
      setDexEnabled(res.data.dex_sniper_enabled)
      setTradeEnabled(res.data.trade_enabled)
      setHasPrivateKey(res.data.has_private_key)
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
      }
      
      const res = await axios.post(`${API_BASE}/config`, payload)
      setStatus({ type: 'success', msg: res.data.msg || 'Secure configuration updated and encrypted.' })
      
      setExecKey('')
      setBinanceKey('')
      setBinanceSecret('')
      
      fetchConfig()
      setTimeout(() => setStatus(null), 5000)
    } catch (err: any) {
      setStatus({ type: 'error', msg: err.response?.data?.msg || 'System encryption failure' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="dashboard-layout">
      
      <header className="header">
        <div className="logo-area">
          <Terminal size={36} className="logo-icon" />
          <h1 className="header-title">NEXUS // TRADING SUITE</h1>
        </div>
        <div className="sys-status">
          <div className="status-dot"></div>
          SYS.ONLINE & SECURE
        </div>
      </header>

      {/* Metrics Row */}
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-header">
            <Activity size={20} />
            <span className="metric-title">TOTAL EXECUTIONS</span>
          </div>
          <div className="metric-value">
            {metrics?.total_trades || 0}
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-header">
            <Target size={20} />
            <span className="metric-title">ALPHA WIN RATE</span>
          </div>
          <div className="metric-value highlight">
            {metrics ? metrics.win_rate.toFixed(1) : 0}<span style={{fontSize: '1.5rem', color: 'var(--text-muted)'}}>%</span>
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-header">
            <Cpu size={20} />
            <span className="metric-title">ACTIVE TOKENS</span>
          </div>
          <div className="metric-value">
            {metrics?.active_tokens || 0}
          </div>
        </div>
      </div>

      {/* Config Section */}
      <div className="config-section">
        
        <div className="panel">
          <h2 className="panel-title">
            <ShieldCheck size={24} />
            SECURITY VAULT & PARAMETERS
          </h2>
          
          <form onSubmit={handleSaveConfig} className="form-grid">
            
            <div className="toggle-group">
              <label className="cyber-toggle">
                <div className="toggle-label">
                  <span className="toggle-title">DEX Sniper Engine</span>
                  <span className="toggle-desc">Auto-buy & panic-sell on Web3 liquidity pools</span>
                </div>
                <input 
                  type="checkbox" 
                  checked={dexEnabled} 
                  onChange={(e) => setDexEnabled(e.target.checked)} 
                />
                <div className="switch"></div>
              </label>

              <div style={{height: '1px', background: 'rgba(255,255,255,0.05)', margin: '10px 0'}}></div>

              <label className="cyber-toggle">
                <div className="toggle-label">
                  <span className="toggle-title">CEX Quant Engine</span>
                  <span className="toggle-desc">Algorithmic trading on Binance Futures</span>
                </div>
                <input 
                  type="checkbox" 
                  checked={tradeEnabled} 
                  onChange={(e) => setTradeEnabled(e.target.checked)} 
                />
                <div className="switch"></div>
              </label>
            </div>

            <div className="input-group">
              <label>EVM PRIVATE KEY (DEX EXECUTION)</label>
              <div className="input-wrapper">
                <Key className="input-icon" />
                <input 
                  className="cyber-input"
                  type="password" 
                  placeholder={hasPrivateKey ? "********** SECURELY CONFIGURED **********" : "Enter 0x-less private key"}
                  value={execKey}
                  onChange={(e) => setExecKey(e.target.value)}
                  autoComplete="off"
                />
              </div>
            </div>

            <div className="input-group">
              <label>BINANCE API KEY</label>
              <div className="input-wrapper">
                <Fingerprint className="input-icon" />
                <input 
                  className="cyber-input"
                  type="password" 
                  placeholder="Enter API Key"
                  value={binanceKey}
                  onChange={(e) => setBinanceKey(e.target.value)}
                  autoComplete="off"
                />
              </div>
            </div>

            <div className="input-group">
              <label>BINANCE API SECRET</label>
              <div className="input-wrapper">
                <Lock className="input-icon" />
                <input 
                  className="cyber-input"
                  type="password" 
                  placeholder="Enter API Secret"
                  value={binanceSecret}
                  onChange={(e) => setBinanceSecret(e.target.value)}
                  autoComplete="off"
                />
              </div>
            </div>

            <button type="submit" className="cyber-button" disabled={saving}>
              {saving ? (
                <>ENCRYPTING...</>
              ) : (
                <><Save size={20} /> COMMIT TO SECURE ENCLAVE</>
              )}
            </button>

            {status && (
              <div className={`status-alert ${status.type}`}>
                {status.type === 'success' ? <CheckCircle2 size={20}/> : <AlertTriangle size={20}/>}
                {status.msg}
              </div>
            )}
          </form>
        </div>

        <div className="info-panel">
          <div className="info-card">
            <h3><Lock size={18} /> AES-CFB ENCRYPTION</h3>
            <p>All sensitive keys are symmetrically encrypted using military-grade AES-CFB protocol before being persisted to the MySQL database.</p>
            <p>Raw private keys are never stored in plaintext and never leave the internal enclave.</p>
            <div className="secure-badge">
              <ShieldCheck size={14} /> ZERO-KNOWLEDGE READY
            </div>
          </div>

          <div className="info-card">
            <h3><Activity size={18} /> LIVE TELEMETRY</h3>
            <p>Engine telemetry is piped directly from the Go runtime over REST APIs.</p>
            <p>The Alpha Engine continuously calculates win rates and filters MEV bots asynchronously.</p>
          </div>
        </div>

      </div>
    </div>
  )
}

export default App