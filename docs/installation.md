# Installation Guide

This guide covers installing and setting up **Arham Gateway** on Ubuntu (e.g. GCP e2-small VM or standalone Linux server).

## System Requirements

- **Operating System**: Ubuntu 22.04 LTS or 24.04 LTS (x86_64 or arm64)
- **Compute / Memory**: Minimum 1 vCPU, 1GB RAM (GCP `e2-small` or `e2-micro` is sufficient)
- **Disk**: 10GB persistent disk
- **Dependencies**: None required at runtime (single static binary, pure Go SQLite embedded, no Docker/Node/Postgres required).

## Target Filesystem Layout

```text
/usr/local/bin/gateway               # Gateway executable
/etc/arham-gateway/config.toml       # Service configuration
/etc/arham-gateway/master.key        # 32-byte AES master key (mode 0600)
/var/lib/arham-gateway/gateway.db    # SQLite database in WAL mode
/etc/systemd/system/gateway.service  # Systemd service unit
```

## Step 1: Install Binary

Download or copy the `gateway` binary to `/usr/local/bin/gateway`:

```bash
sudo cp gateway /usr/local/bin/gateway
sudo chmod +x /usr/local/bin/gateway
```

## Step 2: Run Setup

Run `gateway setup` with root/sudo privileges:

```bash
sudo gateway setup
```

The interactive setup wizard will:
1. Create `/etc/arham-gateway` and `/var/lib/arham-gateway` with restricted permissions.
2. Generate a 32-byte master encryption key `/etc/arham-gateway/master.key` (mode `0600`).
3. Create the SQLite database `/var/lib/arham-gateway/gateway.db` in WAL mode and apply schema migrations.
4. Securely prompt twice for the initial dashboard administrator password (without terminal echo).
5. Install and enable the hardened systemd unit `/etc/systemd/system/gateway.service`.
6. Check for Cloudflared and provide installation instructions if missing.

## Step 3: Start Service

```bash
sudo gateway start
```

Verify service and local listener health:

```bash
gateway status
```

Expected output:
```text
Arham Gateway Local Listener (127.0.0.1:8080): healthy
Systemd Service Status:
● gateway.service - Arham Gateway OpenAI-Compatible Model Gateway
     Loaded: loaded (/etc/systemd/system/gateway.service; enabled)
     Active: active (running)
```

## Step 4: Uninstalling

To uninstall Arham Gateway:

```bash
# Stop and disable systemd service
sudo gateway stop
sudo systemctl disable gateway
sudo rm /etc/systemd/system/gateway.service
sudo systemctl daemon-reload

# Remove binary and data
sudo rm /usr/local/bin/gateway
sudo rm -rf /etc/arham-gateway
sudo rm -rf /var/lib/arham-gateway
```
