import os
import re

def read_file(path):
    with open(path, 'r') as f:
        return f.read()

def write_file(path, content):
    d = os.path.dirname(path)
    if d:
        os.makedirs(d, exist_ok=True)
    with open(path, 'w') as f:
        f.write(content)

def add_import(code, pkg):
    if f'"{pkg}"' in code: return code
    return re.sub(r'import \(', f'import (\n\t"{pkg}"', code, count=1)

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
    code = code.replace('db != nil', 'database.DB != nil')
    code = code.replace('db.Find', 'database.DB.Find')
    code = code.replace('db.Where', 'database.DB.Where')
    code = code.replace('db.Model', 'database.DB.Model')
    code = code.replace('db.Save', 'database.DB.Save')
    code = code.replace('db.Table', 'database.DB.Table')
    code = code.replace('SmartWalletTrade', 'database.SmartWalletTrade')
    code = code.replace('SmartWallet', 'database.SmartWallet')
    
    return code

for file in ['main.go', 'trading.go', 'dex_sniper.go', 'alpha_engine.go', 'backtester.go']:
    content = read_file(file)
    # only modify main_go for tokenInfo removal, but wait, we already did some removals manually?
    if file == 'main.go':
        content = re.sub(r'// ── DEX 版本 ──────────────────────────────────────────────.*?type DEXVersion string.*?const \(\n\tDEXv2 DEXVersion = "V2"\n\tDEXv3 DEXVersion = "V3"\n\)', '', content, flags=re.DOTALL)
        content = re.sub(r'type TokenInfo struct \{.*?\n\}', '', content, flags=re.DOTALL)
        content = add_import(content, 'bot/api')
        content = content.replace('StartAPIServer()', 'api.StartAPIServer()')

    content = update_main_pkg_file(content)
    write_file(file, content)

