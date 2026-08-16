# Backup & Disaster Recovery Guide

Backing up Arham Gateway requires preserving two essential components:
1. **The SQLite Database**: `/var/lib/arham-gateway/gateway.db`
2. **The Master Encryption Key**: `/etc/arham-gateway/master.key`

> **CRITICAL SECURITY INVARIANT**:
> The database and master key must be stored securely. Neither artifact alone discloses provider API keys, because provider secrets in SQLite are encrypted with AES-256-GCM using the separate master key.

---

## 1. Creating a Backup

### Step 1: Hot-Backup SQLite Database
SQLite in WAL mode supports online online hot-backups without stopping the gateway service using standard file copy or SQLite VACUUM INTO:

```bash
# Method A: Safe online copy
sudo sqlite3 /var/lib/arham-gateway/gateway.db ".backup /backup/gateway-$(date +%Y%m%d).db"

# Method B: Direct copy when service is temporarily stopped
sudo gateway stop
sudo cp /var/lib/arham-gateway/gateway.db /backup/gateway-$(date +%Y%m%d).db
sudo gateway start
```

### Step 2: Backup Master Encryption Key
```bash
sudo cp /etc/arham-gateway/master.key /backup/master-$(date +%Y%m%d).key
```

### Step 3: Archive with Encrypted Storage
```bash
sudo tar -czf /secure-backups/arham-gateway-backup-$(date +%Y%m%d).tar.gz -C /backup gateway-*.db master-*.key
```

---

## 2. Restoring from Backup

To restore Arham Gateway to a fresh VM or recovered disk:

1. **Install Gateway Binary**:
   ```bash
   sudo cp gateway /usr/local/bin/gateway
   sudo chmod +x /usr/local/bin/gateway
   ```

2. **Restore Master Key & Config**:
   ```bash
   sudo mkdir -p /etc/arham-gateway
   sudo cp master.key /etc/arham-gateway/master.key
   sudo chmod 0600 /etc/arham-gateway/master.key
   ```

3. **Restore Database**:
   ```bash
   sudo mkdir -p /var/lib/arham-gateway
   sudo cp gateway.db /var/lib/arham-gateway/gateway.db
   sudo chmod 0600 /var/lib/arham-gateway/gateway.db
   ```

4. **Verify and Start Service**:
   ```bash
   sudo gateway status
   sudo gateway start
   ```
