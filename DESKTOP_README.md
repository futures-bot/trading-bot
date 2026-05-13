# XRP Trading Bot - Desktop Application

A native desktop application for the XRP Trading Bot, built with Wails (Go + React).

## 🚀 Features

- **Real-time Trading Dashboard** - Monitor live price, EMA indicators, and signals
- **Position Management** - View current position with entry, stop-loss, and take-profit
- **Performance Metrics** - Track win rate, total P&L, and trade statistics
- **Trade History** - Review past trades with detailed information
- **Multi-Mode Support** - Paper trading, Testnet, and Live trading
- **Native Performance** - Lightweight desktop app (~15MB)
- **Cross-Platform** - Windows, macOS, and Linux support

## 📋 Prerequisites

### 1. Install Wails CLI
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 2. Install Node.js
Node.js v16 or higher is required for the frontend.
```bash
node --version  # Should be v16+
npm --version
```

### 3. Platform-Specific Requirements

**Linux:**
```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev
```

**macOS:**
- Xcode Command Line Tools
```bash
xcode-select --install
```

**Windows:**
- No additional requirements

## 🛠️ Development

### Run in Development Mode (with hot reload)
```bash
# From project root
wails dev
```

This will:
1. Start the Go backend
2. Start the React dev server
3. Open the desktop window
4. Enable hot reload for both Go and React code

### Project Structure
```
futures-test/
├── cmd/
│   └── desktop/          # Desktop app entry point
│       └── main.go
├── desktop/              # Wails app logic
│   ├── app.go           # Exposed Go methods
│   └── events.go        # Real-time events
├── frontend/            # React UI
│   ├── src/
│   │   ├── components/  # UI components
│   │   ├── types/       # TypeScript types
│   │   └── App.tsx      # Main app
│   └── package.json
├── internal/            # Your existing trading logic
└── wails.json           # Wails configuration
```

## 🏗️ Building for Production

### Build for Current Platform
```bash
wails build
```

Output: `build/bin/trading-bot-desktop` (or `.exe` on Windows)

### Build for Specific Platform
```bash
# macOS
wails build -platform darwin/universal

# Windows
wails build -platform windows/amd64

# Linux
wails build -platform linux/amd64
```

### Clean Build
```bash
wails build -clean
```

## 📱 Using the Desktop App

### 1. Start the Application
```bash
./build/bin/trading-bot-desktop
```

Or double-click the executable in your file manager.

### 2. Control Panel
- **Trading Mode**: Choose Paper, Testnet, or Live
- **Symbol**: Select trading pair (default: XRPUSDT)
- **Start/Stop**: Control the bot

### 3. Dashboard Features

**Price Display:**
- Current price of XRP
- EMA9 and EMA21 values
- Bullish/Bearish indicator
- EMA gap percentage

**Position Card:**
- Current position details (LONG/SHORT)
- Entry price and quantity
- Stop-loss and take-profit levels
- Manual close button

**Performance Metrics:**
- Total P&L
- Total trades count
- Win rate percentage
- Wins vs Losses

**Trade History:**
- Recent trades table
- Entry/exit prices
- P&L per trade
- Exit reasons

### 4. Real-Time Updates

The app automatically updates with:
- Price ticks every second
- Position changes
- Trade completions
- Balance updates
- Trading signals

## 🔧 Configuration

The desktop app uses the same `config.yaml` as the HTTP API version:

```yaml
symbol: XRPUSDT
leverage: 10
session_budget: 100.0
ema_fast: 9
ema_slow: 21
take_profit_pct: 0.5
stop_loss_pct: 0.2
# ... etc
```

For testnet/live trading, create a `.env` file:
```bash
BINANCE_API_KEY=your_api_key
BINANCE_SECRET_KEY=your_secret_key
```

## 🎨 Customization

### Change App Name
Edit `wails.json`:
```json
{
  "name": "Your Bot Name",
  "outputfilename": "your-bot-name"
}
```

### Change Window Size
Edit `cmd/desktop/main.go`:
```go
Width:  1280,
Height: 720,
```

### Change Theme Colors
Edit `frontend/tailwind.config.js`:
```js
colors: {
  background: '#0f172a',
  card: '#1e293b',
  primary: '#3b82f6',
  // ...
}
```

## 🐛 Troubleshooting

### Wails CLI Not Found
```bash
# Make sure $GOPATH/bin is in your PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

### Frontend Build Fails
```bash
# Manually install frontend dependencies
cd frontend
npm install
cd ..
wails dev
```

### App Won't Start
1. Check if `config.yaml` exists
2. Check if `trading_bot.db` is accessible
3. Check logs for error messages

### Hot Reload Not Working
```bash
# Restart Wails dev mode
wails dev -s
```

## 📦 Distribution

### Create Release Package

**macOS:**
```bash
wails build -clean
# Creates .app bundle in build/bin/
# Can be distributed as-is or packaged in .dmg
```

**Windows:**
```bash
wails build -clean
# Creates .exe in build/bin/
# Distribute with any DLLs if needed
```

**Linux:**
```bash
wails build -clean
# Creates binary in build/bin/
# Package as .deb or .rpm if desired
```

## 🔄 Differences from HTTP API

| Feature | Desktop App | HTTP API |
|---------|-------------|----------|
| **Communication** | Direct Go calls | HTTP REST |
| **Interface** | Native window | Browser/curl |
| **Real-time** | WebSocket-like events | Polling required |
| **Overhead** | Minimal | HTTP overhead |
| **Use Case** | Daily trading | Automation/scripts |

## 🚀 Next Steps

1. **Install Wails**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
2. **Install Frontend Deps**: `cd frontend && npm install`
3. **Run Dev Mode**: `wails dev`
4. **Start Trading**: Configure and click "Start Bot"
5. **Build Production**: `wails build` when ready

## 📚 Resources

- [Wails Documentation](https://wails.io/)
- [React Documentation](https://react.dev/)
- [TailwindCSS Documentation](https://tailwindcss.com/)

## 🤝 Support

For issues or questions:
1. Check the logs in the app console
2. Review `trading_bot.db` and `trades.jsonl`
3. Check Wails documentation for platform-specific issues

---

**Happy Trading! 🚀📈**