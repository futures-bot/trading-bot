#!/bin/bash

echo "==================================="
echo "XRP Trading Bot - Desktop Setup"
echo "==================================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi
echo "✅ Go is installed: $(go version)"

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Error: Node.js is not installed"
    echo "Please install Node.js from https://nodejs.org/"
    exit 1
fi
echo "✅ Node.js is installed: $(node --version)"

# Install Wails CLI
echo ""
echo "Installing Wails CLI..."
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Check if Wails was installed
if ! command -v wails &> /dev/null; then
    echo "❌ Error: Wails CLI installation failed"
    echo "Make sure \$GOPATH/bin is in your PATH"
    echo "Add this to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\$PATH:\$(go env GOPATH)/bin"
    exit 1
fi
echo "✅ Wails CLI installed: $(wails version)"

# Install frontend dependencies
echo ""
echo "Installing frontend dependencies..."
cd frontend
npm install
cd ..
echo "✅ Frontend dependencies installed"

# Check for platform-specific requirements
echo ""
echo "Checking platform requirements..."

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "🐧 Linux detected"
    echo "Please ensure you have these packages installed:"
    echo "  sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    echo "🍎 macOS detected"
    echo "Please ensure Xcode Command Line Tools are installed:"
    echo "  xcode-select --install"
elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    echo "🪟 Windows detected"
    echo "No additional requirements needed"
fi

echo ""
echo "==================================="
echo "✅ Setup Complete!"
echo "==================================="
echo ""
echo "Next steps:"
echo "  1. Run in development mode:"
echo "     wails dev"
echo ""
echo "  2. Build for production:"
echo "     wails build"
echo ""
echo "  3. Run the app:"
echo "     ./build/bin/trading-bot-desktop"
echo ""
echo "See DESKTOP_README.md for more information"