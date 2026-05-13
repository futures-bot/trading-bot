# Quick Start Guide - Desktop App

Get your XRP Trading Bot desktop app running in 5 minutes!

## 🚀 Quick Setup

### Step 1: Run Installation Script
```bash
./install-desktop.sh
```

This will:
- ✅ Check Go and Node.js installation
- ✅ Install Wails CLI
- ✅ Install frontend dependencies
- ✅ Verify platform requirements

### Step 2: Start Development Mode
```bash
wails dev
```

The desktop window will open automatically with hot reload enabled!

### Step 3: Start Trading
1. In the app window, select "Paper" mode
2. Enter symbol "XRPUSDT"
3. Click "Start Bot"
4. Watch real-time trading in action!

## 📊 What You'll See

### Control Panel (Top)
- Mode selector: Paper / Testnet / Live
- Symbol input: XRPUSDT
- Start/Stop button
- Current config display

### Market Data
- Real-time XRP price
- EMA9 and EMA21 indicators
- Bullish/Bearish trend
- EMA gap percentage

### Current Position (Left)
- LONG or SHORT position
- Entry price and quantity
- Stop-loss and take-profit levels
- Manual close button

### Performance (Right)
- Total P&L
- Win rate percentage
- Total trades count
- Wins vs Losses breakdown

### Trade History (Bottom)
- All completed trades
- Entry/exit prices
- P&L per trade
- Exit reasons (TP/SL/Manual)

## 🎮 Controls

| Action | How To |
|--------|--------|
| Start Trading | Click "Start Bot" |
| Stop Trading | Click "Stop Bot" |
| Close Position | Click "Close Position" in position card |
| Refresh History | Click "Refresh" in trade history |
| Change Mode | Select from dropdown (only when stopped) |

## 🔧 Configuration

### Change Trading Parameters
Edit `config.yaml`:
```yaml
leverage: 10              # Change leverage
ema_fast: 9              # Fast EMA period
ema_slow: 21             # Slow EMA period
take_profit_pct: 0.5     # Take profit %
stop_loss_pct: 0.2       # Stop loss %
```

Restart the app for changes to take effect.

### Enable Testnet Trading
1. Create `.env` file:
```bash
BINANCE_API_KEY=your_testnet_api_key
BINANCE_SECRET_KEY=your_testnet_secret_key
```

2. Select "Testnet" mode in the app
3. Click "Start Bot"

## 📱 Building Production App

### Build for Your Platform
```bash
wails build
```

Output: `build/bin/trading-bot-desktop`

### Distribute the App
**Just share the executable!**
- No dependencies needed
- Single file (~15MB)
- Works on any machine with same OS

### Cross-Platform Builds
```bash
# macOS
wails build -platform darwin/universal

# Windows
wails build -platform windows/amd64

# Linux
wails build -platform linux/amd64
```

## 🐛 Troubleshooting

### App Won't Start
```bash
# Check if config.yaml exists
ls -la config.yaml

# Run with verbose logging
wails dev -v 2
```

### Frontend Not Loading
```bash
# Reinstall frontend dependencies
cd frontend
rm -rf node_modules package-lock.json
npm install
cd ..
wails dev
```

### Wails Command Not Found
```bash
# Add Go bin to PATH
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
```

## 📈 Trading Tips

### Paper Trading (Recommended for Testing)
- ✅ No real money risk
- ✅ Test strategies safely
- ✅ Default balance: $1000 USDT
- ✅ Perfect for learning

### Testnet Trading (Advanced)
- ⚠️ Requires Binance testnet account
- ⚠️ Uses testnet API keys
- ⚠️ Simulates real trading
- ⚠️ No real money involved

### Live Trading (Production)
- ❌ **USE AT YOUR OWN RISK**
- ❌ Real money is involved
- ❌ Requires production API keys
- ❌ Test thoroughly first!

## 🎯 Next Steps

1. ✅ Run `./install-desktop.sh`
2. ✅ Start with `wails dev`
3. ✅ Test paper trading
4. ✅ Review DESKTOP_README.md for details
5. ✅ Build production app when ready

## 💡 Pro Tips

- **Monitor Performance**: Check win rate and adjust strategy
- **Use Paper Mode**: Test changes before going live
- **Set Proper Stops**: Always use stop-loss protection
- **Review History**: Learn from past trades
- **Stay Updated**: Monitor the price display for trends

## 🔗 Resources

- **Main README**: `DESKTOP_README.md`
- **Configuration**: `config.yaml`
- **Trading Rules**: `.clinerules/MASTER_RULES.md`
- **Wails Docs**: https://wails.io

---

**Ready to trade? Run `./install-desktop.sh` now!** 🚀