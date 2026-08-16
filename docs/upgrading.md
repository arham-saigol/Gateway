# Upgrading & Safe Rollbacks

Arham Gateway uses a backup-first, zero-runtime-dependency upgrade strategy. All database migrations are embedded inside the binary and applied automatically and idempotently on startup.

---

## 1. Upgrading Arham Gateway

### Step 1: Backup Database and Master Key
```bash
sudo cp /var/lib/arham-gateway/gateway.db /var/lib/arham-gateway/gateway.db.bak-$(date +%Y%m%d)
sudo cp /etc/arham-gateway/master.key /etc/arham-gateway/master.key.bak
```

### Step 2: Replace Gateway Binary
```bash
sudo cp gateway-new /usr/local/bin/gateway
sudo chmod +x /usr/local/bin/gateway
```

### Step 3: Restart Service
```bash
sudo gateway restart
```

### Step 4: Verify Health
```bash
gateway status
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

---

## 2. Rollback Procedure

If an upgraded version encounters an issue:

1. Stop the service:
   ```bash
   sudo gateway stop
   ```
2. Restore previous binary:
   ```bash
   sudo cp /usr/local/bin/gateway.bak /usr/local/bin/gateway
   ```
3. Restore database snapshot if migrations modified table layouts incompatibly:
   ```bash
   sudo cp /var/lib/arham-gateway/gateway.db.bak-* /var/lib/arham-gateway/gateway.db
   ```
4. Restart service:
   ```bash
   sudo gateway start
   ```
