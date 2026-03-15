# NEXUS // Web3 Trading Suite

> A production-grade, multi-chain, automated crypto trading system featuring a Go-based execution engine and a React-powered Cyberpunk Dashboard.

![Nexus Dashboard](https://via.placeholder.com/1200x600/0f1219/00f0ff?text=NEXUS+TRADING+SUITE)

## 🚀 Overview

NEXUS is a highly modular, enterprise-grade algorithmic trading suite designed for both Decentralized Exchanges (DEX) and Centralized Exchanges (CEX). It transcends simple scripting by incorporating a resilient database architecture, multi-chain capabilities, and advanced heuristic engines for Alpha generation and risk management.

### Key Capabilities

*   **🔫 DEX Sniper Engine**: Automatically snipes new liquidity pools (currently on BSC/PancakeSwap) with dynamic Gas estimation, MEV sandwich protection, and auto-Approve mechanisms.
*   **🧠 Alpha Engine (Smart Money Tracking)**: Continuously scrapes, scores, and tracks "Smart Money" wallets. Features a built-in heuristic filter to identify and blacklist MEV/Arbitrage bots and a clustering algorithm to detect Sybil/Matrix entity networks.

### 🧠 Alpha Scoring Model

The Alpha Engine uses a multi-dimensional weighted scoring model to evaluate wallet quality:

<style>
.formula-box { background: #0f1219; border-radius: 8px; padding: 14px 18px; margin: 12px 0; border-left: 3px solid #00f0ff; font-family: monospace; font-size: 13px; line-height: 1.8; color: #fff; }
.factor-row { display: flex; gap: 10px; margin-bottom: 8px; align-items: flex-start; }
.f-tag { min-width: 60px; padding: 3px 8px; border-radius: 4px; font-size: 11px; font-weight: 500; text-align: center; margin-top: 2px; }
.f-name { font-weight: 500; font-size: 13px; color: #00f0ff; }
.f-desc { font-size: 12px; color: #94a3b8; line-height: 1.5; }
.weight-bar-wrap { height: 6px; background: #1e293b; border-radius: 3px; margin-top: 5px; width: 100%; }
.weight-bar { height: 6px; border-radius: 3px; }
.tag-a { background: #26215C; color: #CECBF6; }
.tag-b { background: #04342C; color: #9FE1CB; }
.tag-c { background: #412402; color: #FAC775; }
.tag-d { background: #4A1B0C; color: #F5C4B3; }
.tag-e { background: #042C53; color: #85B7EB; }
</style>

<div class="formula-box">
Score = 100 × [<br>
  &nbsp;&nbsp;W_wr &times; f_winrate(WinRate)      <span style="color:#475569">// Win Rate Component</span><br>
  + W_roi &times; f_roi(ROI)              <span style="color:#475569">// ROI Component</span><br>
  + W_rec &times; f_recency(LastActive)   <span style="color:#475569">// Recency Component</span><br>
  + W_con &times; f_consistency(Trades)   <span style="color:#475569">// Consistency Component</span><br>
  + W_ear &times; f_early(AvgBlockDelta)  <span style="color:#475569">// Early Entry Component</span><br>
]
</div>

<div class="factor-row">
  <div class="f-tag tag-a">W=0.35</div>
  <div>
    <div class="f-name">f_winrate — Win Rate (Highest Weight)</div>
    <div class="f-desc">Calculated by resolving BUY/SELL pairs. A price increase of 20%+ after BUY is considered a win.<br>f(wr) = sigmoid(wr, center=0.55, steepness=10). A 55% win rate yields 0.5 points.</div>
    <div class="weight-bar-wrap"><div class="weight-bar" style="width:70%;background:#7F77DD"></div></div>
  </div>
</div>

<div class="factor-row">
  <div class="f-tag tag-b">W=0.30</div>
  <div>
    <div class="f-name">f_roi — Average ROI</div>
    <div class="f-desc">Mean ROI across all closed trades.<br>f(roi) = tanh(roi / 0.5). Squashes outliers to prevent single high-ROI "lucky" trades from skewing the total score.</div>
    <div class="weight-bar-wrap"><div class="weight-bar" style="width:60%;background:#1D9E75"></div></div>
  </div>
</div>

<div class="factor-row">
  <div class="f-tag tag-c">W=0.20</div>
  <div>
    <div class="f-name">f_recency — Recency Component</div>
    <div class="f-desc">Prioritizes wallets that have been active recently.<br>f(days) = e^(−days/14). Activity from 7 days ago scores 0.61; 30 days ago scores only 0.11.</div>
    <div class="weight-bar-wrap"><div class="weight-bar" style="width:40%;background:#BA7517"></div></div>
  </div>
</div>

<div class="factor-row">
  <div class="f-tag tag-d">W=0.10</div>
  <div>
    <div class="f-name">f_consistency — Frequency Consistency</div>
    <div class="f-desc">Penalizes noise (too few trades) and MEV bots (too many trades).<br>f(n) = 1 − |log(n/15)| / 4. Target range is 8-30 trades for a full score.</div>
    <div class="weight-bar-wrap"><div class="weight-bar" style="width:20%;background:#D85A30"></div></div>
  </div>
</div>

<div class="factor-row">
  <div class="f-tag tag-e">W=0.05</div>
  <div>
    <div class="f-name">f_early — Early Entry Reward</div>
    <div class="f-desc">Avg blocks from pool creation to entry. (BSC ≈ 3s/block).<br>f(blocks) = e^(−blocks/100). Entry within 50 blocks (2.5 mins) scores 0.61+.</div>
    <div class="weight-bar-wrap"><div class="weight-bar" style="width:10%;background:#378ADD"></div></div>
  </div>
</div>

> **Note**: To ensure accurate scoring, the Alpha Engine uses a background worker (`trackSmartWalletSells`) to scan for exit prices by resolving subsequent `Transfer` events from smart wallets back to liquidity pools or routers.

*   **📉 CEX Quant Engine**: Connects to Binance Futures for algorithmic trading (e.g., Grid Market Making, Momentum Breakout, EMA/MACD/RSI strategies) with real-time position tracking and global Take Profit/Stop Loss management.
*   **🛡️ Rug-Pull Panic Sell**: Real-time WSS listener that monitors LP `Sync` events. If liquidity is suddenly removed (Rug Pull), it overrides slippage limits (up to 50%) to execute an emergency Panic Sell in the same block.
*   **📊 Cyberpunk React Dashboard**: A sleek, futuristic Web UI for monitoring total trades, win rates, and active tracked tokens.
*   **🔐 Zero-Knowledge Security Vault**: Raw API keys and execution private keys are **never** stored in plaintext. They are symmetrically encrypted using military-grade `AES-CFB` before being persisted to the MySQL database.
*   **🧪 Backtester**: A built-in offline backtesting engine capable of simulating Limit, Stop, and Market orders against historical K-line data to calculate Max Drawdown, Win Rate, and PnL before risking real capital.

## 🏗️ Architecture

*   **Backend Engine**: Go (Golang)
*   **Smart Contract Interaction**: `go-ethereum` (Geth) ABI bindings
*   **Database ORM**: `gorm.io/gorm` (MySQL)
*   **Frontend UI**: React + TypeScript + Vite + Tailwind CSS (Lucide Icons)
*   **Encryption**: AES-CFB (Advanced Encryption Standard - Cipher Feedback Mode)

## 📦 Installation & Setup

### Prerequisites

*   Go 1.21+
*   Node.js 18+ (for Dashboard)
*   MySQL Server (Local or Cloud)

### 1. Clone & Configure

Clone the repository and set up your master environment variables:

```bash
cp .env.example .env
```

Edit the `.env` file:
```ini
# MySQL Database Connection (Required)
MYSQL_DSN="user:password@tcp(127.0.0.1:3306)/nexus_db?charset=utf8mb4&parseTime=True&loc=Local"

# Discord Webhook for Alerts (Optional)
DISCORD_WEBHOOK="https://discord.com/api/webhooks/..."

# RPC Nodes (Required)
BSC_HTTP_RPC="https://bsc-dataseed.binance.org/"
BSC_WSS_RPC="wss://bsc-ws-node.nodedapp.com"

# The Master Key used to encrypt/decrypt secrets in the database (Must be >= 32 characters)
MASTER_KEY="your_super_secret_32_byte_master_key"
```

### 2. Start the Go Execution Engine

The Go backend handles all blockchain interactions, the API server, and the database migrations.

```bash
go mod tidy
go build -o nexus-bot
./nexus-bot
```

*The backend API server will start on `http://localhost:18080`.*

### 3. Start the React Dashboard

In a new terminal window, spin up the frontend UI:

```bash
cd dashboard
npm install
npm run dev
```

Visit `http://localhost:5173` in your browser. From the dashboard, you can securely enter your Binance API Keys and your EVM Private Key.
All strategies are loaded via `strategies.json`. During live trading, use the `ACTIVE_STRATEGY` environment variable to specify a unique strategy ID.

### strategies.json Format

```json
[
  {
    "id": "my_ema",
    "type": "ema_macd_rsi",
    "params": {}
  },
  {
    "id": "my_grid",
    "type": "grid_mm",
    "params": {
      "gridSpacingBps": 20.0,
      "numLevels": 3,
      "sizePerLevel": 0.05,
      "maxPosition": 0.15
    }
  }
]
```

### Strategy Overview

#### `ema_macd_rsi` — Trend Following Strategy

Entry based on triple moving average (EMA 9/21/55), RSI(14), and MACD multi-layer filtering:

- **Long Conditions**: EMA9 > EMA21 > EMA55, Price > EMA21, RSI between 45–70, MACD Line > Signal Line.
- **Short Conditions**: EMA9 < EMA21 < EMA55, Price < EMA21, RSI between 30–55, MACD Line < Signal Line.
- **Take Profit/Stop Loss**: TP at ATR(14) × 4, SL at ATR(14) × 2.
- **Minimum K-lines**: 60 candles (ensures full warmup for EMA55 + MACD Signal).

#### `momentum_breakout` — Momentum Breakout Strategy

Enters when price breaks out of the N-period High/Low range with increased volume:

- **Parameters**: `lookback` (lookback period, default 20), `breakoutThresholdBps` (breakout magnitude, default 50 bps), `volumeSurgeMult` (volume surge multiplier, default 2×), `trailingStopBps` (trailing stop, default 30 bps).
- **Trailing Stop**: Maintains high/low watermarks; the stop line only moves in a favorable direction and resets automatically after closing.

#### `simple_mm` — Simple Market Making Strategy

Symmetric Bid/Ask quoting with inventory skew protection:

- **Parameters**: `spreadBps` (spread, default 20 bps), `size` (order size).
- **Inventory Skew**: Shifts the quote center upward when long positions accumulate to reduce further buying incentive; maximum position is `size × 5`, stopping one-sided quotes when exceeded.

#### `grid_mm` — Grid Market Making Strategy

Places N-levels of limit orders above and below the mid-price:

- **Parameters**: `gridSpacingBps` (interval), `numLevels` (levels), `sizePerLevel` (size per level), `maxPosition` (max position).
- **ReduceOnly Mode**: Stops placing new grid orders and only places closing orders when the engine passes `ctx.ReduceOnly=true`.

---

## 🧪 Backtesting

Validate strategy parameters without starting the live trading engine:

```bash
./nexus-bot backtest
```

The backtester reads all strategies from `strategies.json`, running 1500 candles (15m interval) per strategy with an initial balance of 50 USDT.

**Backtest Result Example:**

```
strategy=my_ema pnl=$12.34 win_rate=58.33% trades=24 max_drawdown=8.21% ($4.11)
```

**Backtest Engine Logic:**

- Market orders are filled at the **next candle's Open price** to eliminate look-ahead bias.
- Each fill deducts a **0.04% Taker fee**.
- Limit orders are matched based on the candle's High/Low range; SL/TP orders also participate in matching.

Ensure you have a `strategies.json` file in your root directory:

```json
[
    {
        "id": "grid_mm_wide",
        "type": "grid_mm",
        "params": {
            "gridSpacingBps": 20.0,
            "numLevels": 3,
            "sizePerLevel": 0.05,
            "maxPosition": 0.15
        }
    }
]
```

Run the backtester:
```bash
./nexus-bot backtest
```

## 🔒 Security Disclaimer

While NEXUS utilizes AES-CFB encryption for database storage, you are solely responsible for securing your host machine, the `MASTER_KEY` inside your `.env`, and your actual funds. **Never commit your `.env` or `strategies.json` files.**

This software is for educational and research purposes. Use at your own risk in production environments.
