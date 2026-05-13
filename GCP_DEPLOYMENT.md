# GCP Deployment Guide

This guide explains how to deploy the trading bot to a Google Cloud Platform (GCP) Virtual Machine (VM) and run it as a service with automatic restarts and log rotation.

## 1. Prerequisites

- A GCP project with billing enabled.
- The `gcloud` command-line tool installed and authenticated.

## 2. VM Setup

Follow the instructions in `DEPLOYMENT.md` (steps 1 and 2) to create a GCP VM and set up the firewall rules.

## 3. Application Setup

Follow the instructions in `DEPLOYMENT.md` (steps 3, 4, and 5) to SSH into the VM, install Go, clone the repository, build the application, and configure the environment variables in an `.env` file.

## 4. Run as a Service

To run the bot continuously, even after a reboot, you can set it up as a `systemd` service.

Follow the instructions in `DEPLOYMENT.md` (Step 6) to create a `systemd` service file for the trading bot. This will ensure that the bot restarts automatically if it crashes or if the VM is rebooted.

## 5. Log Rotation

To prevent the log files from growing indefinitely, you can use `logrotate`.

1.  **Create a `logrotate` configuration file for the trading bot:**

    ```bash
    sudo nano /etc/logrotate.d/trading-bot
    ```

2.  **Add the following content to the file:**

    ```
    /var/log/trading-bot.log {
        daily
        rotate 7
        compress
        delaycompress
        missingok
        notifempty
        create 0640 root adm
    }
    ```

3.  **Modify the `systemd` service to redirect the output to a file.**

    Open the service file:

    ```bash
    sudo nano /etc/systemd/system/trading-bot.service
    ```

    And modify the `[Service]` section to look like this:

    ```
    [Service]
    Type=simple
    User=your_user
    WorkingDirectory=/home/your_user/trading-bot-repo
    Environment="PATH=/usr/local/go/bin:/usr/bin"
    ExecStart=/home/your_user/trading-bot-repo/trading-bot run
    Restart=always
    RestartSec=10
    StandardOutput=append:/var/log/trading-bot.log
    StandardError=append:/var/log/trading-bot.log
    ```

    Replace `your_user` with your username.

4.  **Reload the `systemd` daemon and restart the service:**

    ```bash
    sudo systemctl daemon-reload
    sudo systemctl restart trading-bot
    ```

Now the bot will run as a service, restart automatically, and its logs will be rotated daily.