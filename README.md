# NEXUS // Web3 Trading Suite

> A production-grade, multi-chain, automated crypto trading system featuring a Go-based execution engine and a React-powered Cyberpunk Dashboard.

![Nexus Dashboard](https://via.placeholder.com/1200x600/0f1219/00f0ff?text=NEXUS+TRADING+SUITE)

## 🚀 Overview

NEXUS is a highly modular, enterprise-grade algorithmic trading suite designed for both Decentralized Exchanges (DEX) and Centralized Exchanges (CEX). It transcends simple scripting by incorporating a resilient database architecture, multi-chain capabilities, and advanced heuristic engines for Alpha generation and risk management.

### Key Capabilities

*   **🔫 DEX Sniper Engine**: Automatically snipes new liquidity pools (currently on BSC/PancakeSwap) with dynamic Gas estimation, MEV sandwich protection, and auto-Approve mechanisms.
*   **🧠 Alpha Engine (Smart Money Tracking)**: Continuously scrapes, scores, and tracks "Smart Money" wallets. Features a built-in heuristic filter to identify and blacklist MEV/Arbitrage bots and a clustering algorithm to detect Sybil/Matrix entity networks.
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

## 🧪 Backtesting

You can run offline backtests on historical data to validate strategy parameters without starting the live trading engine.

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
