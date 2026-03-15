import os
import re

def read_file(path):
    with open(path, 'r') as f:
        return f.read()

def write_file(path, content):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        f.write(content)

def add_import(code, pkg):
    if f'"{pkg}"' in code: return code
    return re.sub(r'import \(', f'import (\n\t"{pkg}"', code, count=1)

main_go = read_file('main.go')
api_go = read_file('api.go')
config_go = read_file('config_manager.go')
db_go = read_file('db.go')
trading_go = read_file('trading.go')
dex_sniper_go = read_file('dex_sniper.go')
alpha_engine_go = read_file('alpha_engine.go')

# 1. Create types package
types_content = """package types

import (
	"time"
	"bot/dex"
)

type DEXVersion string

const (
	DEXv2 DEXVersion = "V2"
	DEXv3 DEXVersion = "V3"
)

type TokenInfo struct {
	Address      string
	Symbol       string
	Name         string
	PoolAddress  string
	LiquidityBNB float64
	LiquidityUSD float64
	CreatedBlock uint64
	CreatedAt    time.Time
	SmartBuys    int
	HitWallets   map[string]string
	DEX          DEXVersion
	FeeTier      uint32
}
"""
write_file('types/types.go', types_content)

# Remove TokenInfo from main.go
main_go = re.sub(r'// ── DEX 版本 ──────────────────────────────────────────────.*?type DEXVersion string.*?const \(\n\tDEXv2 DEXVersion = "V2"\n\tDEXv3 DEXVersion = "V3"\n\)', '', main_go, flags=re.DOTALL)
main_go = re.sub(r'type TokenInfo struct \{.*?\n\}', '', main_go, flags=re.DOTALL)

# 2. Refactor database
db_go = db_go.replace('package main', 'package database')
db_go = add_import(db_go, 'bot/types')
db_go = db_go.replace('TokenInfo', 'types.TokenInfo')
db_go = db_go.replace('var db *gorm.DB', 'var DB *gorm.DB')
db_go = db_go.replace('db.', 'DB.')
db_go = db_go.replace('db = gorm.Open', 'DB, err = gorm.Open')
db_go = db_go.replace('func initDB(', 'func InitDB(')
db_go = db_go.replace('func logTradeToDB(', 'func LogTradeToDB(')
db_go = db_go.replace('func saveTokenToDB(', 'func SaveTokenToDB(')
db_go = db_go.replace('func syncSmartWalletsToDB(', 'func SyncSmartWalletsToDB(')
db_go = db_go.replace('func recordSmartWalletTrade(', 'func RecordSmartWalletTrade(')
db_go = db_go.replace('func getFilteredSmartWallets(', 'func GetFilteredSmartWallets(')
db_go = db_go.replace('if db == nil', 'if DB == nil')
db_go = db_go.replace('if db != nil', 'if DB != nil')

write_file('database/db.go', db_go)
os.remove('db.go')

# 3. Refactor config
config_go = config_go.replace('package main', 'package config')
config_go = add_import(config_go, 'bot/database')
config_go = config_go.replace('db != nil', 'database.DB != nil')
config_go = config_go.replace('db == nil', 'database.DB == nil')
config_go = config_go.replace('db.Session', 'database.DB.Session')
config_go = config_go.replace('db.Logger', 'database.DB.Logger')
config_go = config_go.replace('db.Save', 'database.DB.Save')
config_go = config_go.replace('SystemConfig', 'database.SystemConfig')
config_go = config_go.replace('db.First', 'database.DB.First')

write_file('config/config_manager.go', config_go)
os.remove('config_manager.go')

# 4. Refactor api
api_go = api_go.replace('package main', 'package api')
api_go = add_import(api_go, 'bot/config')
api_go = add_import(api_go, 'bot/database')
api_go = api_go.replace('GetConfig(', 'config.GetConfig(')
api_go = api_go.replace('SetConfig(', 'config.SetConfig(')
api_go = api_go.replace('if db != nil', 'if database.DB != nil')
api_go = api_go.replace('db.Model', 'database.DB.Model')
api_go = api_go.replace('TradeHistory{}', 'database.TradeHistory{}')
api_go = api_go.replace('Token{}', 'database.Token{}')

write_file('api/api.go', api_go)
os.remove('api.go')

# 5. Update main package files
def update_main_pkg_file(code):
    code = add_import(code, 'bot/config')
    code = add_import(code, 'bot/database')
    code = add_import(code, 'bot/types')
    
    code = code.replace('initDB()', 'database.InitDB()')
    code = code.replace('logTradeToDB(', 'database.LogTradeToDB(')
    code = code.replace('saveTokenToDB(', 'database.SaveTokenToDB(')
    code = code.replace('syncSmartWalletsToDB(', 'database.SyncSmartWalletsToDB(')
    code = code.replace('recordSmartWalletTrade(', 'database.RecordSmartWalletTrade(')
    code = code.replace('getFilteredSmartWallets(', 'database.GetFilteredSmartWallets(')
    code = code.replace('GetConfig(', 'config.GetConfig(')
    code = code.replace('SetConfig(', 'config.SetConfig(')
    code = code.replace('TokenInfo', 'types.TokenInfo')
    code = code.replace('DEXVersion', 'types.DEXVersion')
    code = code.replace('DEXv2', 'types.DEXv2')
    code = code.replace('DEXv3', 'types.DEXv3')
    
    code = code.replace('db == nil', 'database.DB == nil')
    code = code.replace('db.Find', 'database.DB.Find')
    code = code.replace('db.Where', 'database.DB.Where')
    code = code.replace('db.Model', 'database.DB.Model')
    code = code.replace('db.Save', 'database.DB.Save')
    code = code.replace('db.Table', 'database.DB.Table')
    code = code.replace('SmartWalletTrade', 'database.SmartWalletTrade')
    code = code.replace('SmartWallet', 'database.SmartWallet')
    
    return code

main_go = update_main_pkg_file(main_go)
trading_go = update_main_pkg_file(trading_go)
dex_sniper_go = update_main_pkg_file(dex_sniper_go)
alpha_engine_go = update_main_pkg_file(alpha_engine_go)

main_go = add_import(main_go, 'bot/api')
main_go = main_go.replace('StartAPIServer()', 'api.StartAPIServer()')

write_file('main.go', main_go)
write_file('trading.go', trading_go)
write_file('dex_sniper.go', dex_sniper_go)
write_file('alpha_engine.go', alpha_engine_go)
