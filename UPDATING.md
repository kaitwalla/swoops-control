# Updating Swoops

This guide explains how to update your Swoops installation to the latest version.

## Method 1: In-Place Update (Recommended)

Swoops includes a built-in update mechanism that downloads and installs the latest release from GitHub.

### From the Running Server

```bash
# Run the update command
sudo /usr/local/bin/swoopsd --update

# Restart the service
sudo systemctl restart swoopsd
```

### What Happens During Update

1. **Download**: Fetches the latest release from GitHub
2. **Install**: Replaces the binary at the current location
3. **Capability Grant** (Linux only): Automatically grants `CAP_NET_BIND_SERVICE` for port 443/80
4. **Exit**: Returns and waits for you to restart

### Automatic Capability Grant

On Linux, the update process automatically runs:
```bash
setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
```

This allows Swoops to bind to ports 443 and 80 without running as root (required for autocert/Let's Encrypt).

If this fails (e.g., not running as root), you'll see:
```
Warning: Failed to grant CAP_NET_BIND_SERVICE capability
If using autocert, run manually: sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
```

Run the command manually with sudo if needed.

## Method 2: Systemd with Auto-Capability

Use the provided systemd service file that automatically grants capabilities on every restart:

**Install the service file:**
```bash
sudo cp deploy/swoopsd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable swoopsd
```

**Key feature in the service file:**
```ini
ExecStartPre=/usr/sbin/setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
```

This means:
- Capabilities are automatically granted on every service start
- No manual intervention needed after updates
- Updates can be done without worrying about capabilities

**Update process with this setup:**
```bash
# Update the binary
sudo /usr/local/bin/swoopsd --update

# Restart (capability is automatically granted)
sudo systemctl restart swoopsd
```

## Method 3: Manual Download

If the built-in update isn't working, download manually:

```bash
# Determine your architecture
uname -m  # x86_64 = amd64, aarch64 = arm64

# Download for Linux amd64
curl -L -o /tmp/swoopsd \
  https://github.com/kaitwalla/swoops-control/releases/latest/download/swoopsd-linux-amd64

# Or for Linux arm64
curl -L -o /tmp/swoopsd \
  https://github.com/kaitwalla/swoops-control/releases/latest/download/swoopsd-linux-arm64

# Make executable
chmod +x /tmp/swoopsd

# Install (requires sudo for system locations)
sudo mv /tmp/swoopsd /usr/local/bin/swoopsd

# Grant capability (if using autocert)
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd

# Restart service
sudo systemctl restart swoopsd
```

## Troubleshooting

### "Permission denied" when updating

**Cause**: The binary is in a system location requiring root access.

**Solution**: Run with sudo:
```bash
sudo /usr/local/bin/swoopsd --update
```

### Update downloads but capability grant fails

**Cause**: The update process couldn't grant `CAP_NET_BIND_SERVICE`.

**Solution**: Grant it manually:
```bash
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
sudo systemctl restart swoopsd
```

Or use the systemd service file with `ExecStartPre` (Method 2).

### Port 443 permission denied after update

**Cause**: The capability was lost during the update.

**Symptoms**:
```
HTTPS server error: listen tcp 0.0.0.0:443: bind: permission denied
```

**Solution 1** - Grant capability manually:
```bash
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
sudo systemctl restart swoopsd
```

**Solution 2** - Use the systemd service with auto-capability (recommended):
```bash
sudo cp deploy/swoopsd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl restart swoopsd
```

### "No such file or directory" for setcap

**Cause**: `setcap` utility not installed.

**Solution**: Install libcap:
```bash
# Debian/Ubuntu
sudo apt install libcap2-bin

# RHEL/CentOS/Rocky
sudo yum install libcap

# Alpine
sudo apk add libcap
```

### Update downloads wrong architecture

**Cause**: Architecture detection failed.

**Solution**: Set GOARCH explicitly:
```bash
# For x86_64/amd64
sudo GOARCH=amd64 /usr/local/bin/swoopsd --update

# For ARM64/aarch64
sudo GOARCH=arm64 /usr/local/bin/swoopsd --update
```

### Binary is replaced but service doesn't restart

The update process does NOT automatically restart the service. You must restart manually:

```bash
sudo systemctl restart swoopsd
```

Or add a post-update script:
```bash
#!/bin/bash
# /usr/local/bin/update-swoops.sh
/usr/local/bin/swoopsd --update && systemctl restart swoopsd
```

## Checking Update Status

### View current version

```bash
/usr/local/bin/swoopsd --version
```

### View service logs

```bash
# Follow logs in real-time
sudo journalctl -u swoopsd -f

# View recent logs
sudo journalctl -u swoopsd -n 50

# Check for update messages
sudo journalctl -u swoopsd | grep -i update
```

### Check for available updates

The server automatically checks for updates on startup and logs if a newer version is available:

```
⚠️  Update available: v1.1.0 → v1.2.0
   Download: https://github.com/kaitwalla/swoops-control/releases/tag/v1.2.0
```

## Update Best Practices

1. **Test in staging first** - Always test updates in a non-production environment
2. **Backup database** - Back up `/var/lib/swoops/swoops.db` before updating
3. **Review changelog** - Check release notes for breaking changes
4. **Use systemd auto-capability** - Eliminates manual capability grants
5. **Monitor logs** - Watch logs during and after update for issues
6. **Schedule maintenance window** - Brief downtime during restart

## Automated Updates

### Option 1: Systemd Timer

Create `/etc/systemd/system/swoops-update.service`:
```ini
[Unit]
Description=Update Swoops to latest version
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/swoopsd --update
ExecStartPost=/usr/bin/systemctl restart swoopsd
```

Create `/etc/systemd/system/swoops-update.timer`:
```ini
[Unit]
Description=Update Swoops weekly

[Timer]
OnCalendar=weekly
Persistent=true

[Install]
WantedBy=timers.target
```

Enable:
```bash
sudo systemctl enable --now swoops-update.timer
```

### Option 2: Cron Job

Create `/etc/cron.weekly/update-swoops`:
```bash
#!/bin/bash
/usr/local/bin/swoopsd --update && systemctl restart swoopsd
```

Make executable:
```bash
sudo chmod +x /etc/cron.weekly/update-swoops
```

## Rollback

If an update causes issues:

```bash
# Download previous version
VERSION=v1.1.0  # Replace with desired version
curl -L -o /tmp/swoopsd \
  https://github.com/kaitwalla/swoops-control/releases/download/${VERSION}/swoopsd-linux-amd64

# Install
sudo mv /tmp/swoopsd /usr/local/bin/swoopsd
sudo chmod +x /usr/local/bin/swoopsd

# Grant capability
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd

# Restart
sudo systemctl restart swoopsd
```

## Related Documentation

- [FIX_PORT_443.md](./FIX_PORT_443.md) - Fixing port 443 permission issues
- [CERTIFICATE_ROTATION.md](./CERTIFICATE_ROTATION.md) - Certificate management
- [WAF_README.md](./WAF_README.md) - Web Application Firewall
