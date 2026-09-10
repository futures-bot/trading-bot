# Production Runbook: GCP VM Instance Deployment

This document contains step-by-step instructions to provision a Google Cloud Platform (GCP) Compute Engine virtual machine, configure Go dependencies, secure environmental secrets, register the trading engine as a persistent `systemd` daemon, and implement standard system-level log rotation policies.

---

## 1. Cloud Resource Provisioning

Run the following command from your local machine to deploy a cost-efficient, high-availability `e2-micro` virtual machine instance inside GCP:

```bash
# Provision the GCP Compute Engine VM instance
gcloud compute instances create trading-bot-prod \
  --zone=us-east1-c \
  --machine-type=e2-micro \
  --network-tier=STANDARD \
  --image-family=debian-11 \
  --image-project=debian-cloud \
  --boot-disk-size=10GB \
  --boot-disk-type=pd-standard \
  --metadata=enable-oslogin=TRUE
```

### Configure Secure Firewall Policies
Since the trading bot is completely CLI-driven and doesn't run external web/WebSocket ports, **no custom inbound port rules are required**. Secure your instance by keeping all inbound ports closed except for SSH (`22`), which is managed automatically by Google OS Login.

---

## 2. Remote Machine Setup & Compilation

### A. Access the Instance
Establish a secure SSH session to the newly created instance:

```bash
gcloud compute ssh trading-bot-prod --zone=us-east1-c
```

### B. Install Go Runtime Env
Run the following setup script on your VM instance to install Go:

```bash
# Download the stable Go binary archive
curl -OL https://go.dev/dl/go1.22.5.linux-amd64.tar.gz

# Extract archive to /usr/local
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz

# Configure system paths for your user profile
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile

# Verify installation success
go version
```

### C. Clone and Build Code
Synchronize workspace files and compile the optimized execution binary:

```bash
# Clone the repository
git clone https://github.com/bercho001-cpu/trading-bot.git ~/trading-bot-repo
cd ~/trading-bot-repo

# Fetch and verify Go dependencies
go mod download

# Compile an optimized production binary with debug symbol stripping (-s -w)
go build -ldflags="-s -w" -o trading-bot main.go
```

---

## 3. Inject Secure Configuration

Create a production-specific `.env` file containing database, NATS, and exchange keys on the VM. **Do not commit this file to source control.**

```bash
# Write environment file on the VM
cat > ~/trading-bot-repo/.env << 'EOF'
DATABASE_URL="host=your-rds-or-supabase-postgres port=5432 user=postgres password=secure_password dbname=postgres sslmode=require"
NATS_URL="nats://localhost:4222"
NATS_CREDS_FILE=""
BINANCE_API_KEY="your_binance_testnet_api_key"
BINANCE_SECRET_KEY="your_binance_testnet_secret_key"
TELEGRAM_BOT_TOKEN="your_telegram_bot_token"
TELEGRAM_CHAT_ID="your_telegram_chat_id"
EOF
```

---

## 4. Run as a Monitored systemd Daemon

To guarantee high availability, continuous 24/7 background execution, and automated process recovery upon crashes or system reboots, we configure `systemd` to supervise the execution.

### A. Generate System Unit Configuration
Create the unit definition file:

```bash
sudo cat > /etc/systemd/system/trading-bot.service << 'EOF'
[Unit]
Description=Enterprise Futures Trading System (EFTS) Daemon
After=network.target network-online.target nats-server.service postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/trading-bot-repo
Environment="PATH=/usr/local/go/bin:/usr/bin:/usr/local/bin"
ExecStart=/root/trading-bot-repo/trading-bot run
Restart=always
RestartSec=10
LimitNOFILE=65535

# Process logs are piped cleanly to standard syslog paths and handled by systemd-journald
StandardOutput=append:/var/log/trading-bot.log
StandardError=append:/var/log/trading-bot.log

[Install]
WantedBy=multi-user.target
EOF
```

*Note: Adjust `User` and paths (`/root/...` or `/home/your-user/...`) to reflect your OS Login setup.*

### B. Register and Launch Daemon
Reload configurations and start the continuous staging background task:

```bash
# Reload daemon configurations
sudo systemctl daemon-reload

# Enable service to auto-start on system boots
sudo systemctl enable trading-bot

# Launch the trading daemon
sudo systemctl start trading-bot
```

---

## 5. Implement Production Log Rotation Policies

To prevent the daemon's log output (`/var/log/trading-bot.log`) from consuming disk volume over time, establish a rigid log rotation rule using standard linux utilities.

### A. Configure `logrotate`
Create a dedicated log rotation definition file:

```bash
sudo cat > /etc/logrotate.d/trading-bot << 'EOF'
/var/log/trading-bot.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    create 0640 root adm
}
EOF
```

### B. Explanation of Log Rotation Rules
*   **`daily`**: Rotates the file once every 24 hours.
*   **`rotate 14`**: Retains a maximum of 14 compressed historical log files, giving you a full 2-week history.
*   **`compress` & `delaycompress`**: Compresses old logs using `gzip` to minimize disk footprints, delaying compression by one rotation cycle to prevent issues with active file handles.
*   **`copytruncate`**: Truncates the active log file in place after making a backup copy. This allows the active Go binary to continue writing logs to the same file descriptor without needing a process restart.

---

## 6. Maintenance & Telemetry Auditing

Operators can run administrative CLI queries directly on the VM:

```bash
cd ~/trading-bot-repo

# Check active service status and uptime metrics
sudo systemctl status trading-bot

# Tail active execution logs
sudo journalctl -u trading-bot -f -n 100

# Perform terminal audits of executing trades and sessions
./trading-bot stats
./trading-bot sessions
./trading-bot trades
```
