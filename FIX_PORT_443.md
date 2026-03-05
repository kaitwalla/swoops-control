# Fixing Port 443 Permission Denied Error

## The Problem

When using autocert (Let's Encrypt), swoopsd needs to bind to port 443 (HTTPS) and port 80 (HTTP for ACME challenges). On Linux, binding to ports below 1024 requires elevated privileges.

Error:
```
HTTPS server error: listen tcp 0.0.0.0:443: bind: permission denied
```

## Solutions

### Solution 1: Grant CAP_NET_BIND_SERVICE Capability (Recommended)

This allows the binary to bind to privileged ports without running as root:

```bash
# Grant the capability to the swoopsd binary
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd

# Verify it was set
getcap /usr/local/bin/swoopsd
# Should output: /usr/local/bin/swoopsd = cap_net_bind_service+ep
```

**Important**: You need to re-run this command **after every update** since capabilities are tied to the binary file itself.

#### Automate with systemd

Add this to your systemd service file to automatically set capabilities on service start:

```ini
[Service]
# Grant capability before starting
ExecStartPre=/usr/sbin/setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
ExecStart=/usr/local/bin/swoopsd --config /etc/swoops/config.yaml
```

Full example: `/etc/systemd/system/swoopsd.service`
```ini
[Unit]
Description=Swoops Control Plane
After=network.target

[Service]
Type=simple
User=swoops
Group=swoops
WorkingDirectory=/var/lib/swoops

# Automatically grant capability on start
ExecStartPre=/usr/sbin/setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
ExecStart=/usr/local/bin/swoopsd --config /etc/swoops/config.yaml

# Restart on failure
Restart=on-failure
RestartSec=5s

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=swoopsd

[Install]
WantedBy=multi-user.target
```

Then reload and restart:
```bash
sudo systemctl daemon-reload
sudo systemctl restart swoopsd
```

### Solution 2: Use a Reverse Proxy (nginx/Caddy)

Run swoopsd on a high port (e.g., 8443) and use a reverse proxy on port 443:

**Config:**
```yaml
server:
  host: 0.0.0.0
  port: 8443
  tls_enabled: true
  tls_cert: /etc/swoops/certs/server.crt
  tls_key: /etc/swoops/certs/server.key
```

**nginx config:**
```nginx
server {
    listen 443 ssl http2;
    server_name ctrl.kait.dev;

    ssl_certificate /etc/letsencrypt/live/ctrl.kait.dev/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/ctrl.kait.dev/privkey.pem;

    location / {
        proxy_pass https://localhost:8443;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Caddy config (simpler):**
```
ctrl.kait.dev {
    reverse_proxy localhost:8443 {
        transport http {
            tls_insecure_skip_verify
        }
    }
}
```

**Pros:**
- No special capabilities needed
- Proxy can handle multiple services
- Can use proxy's own TLS termination

**Cons:**
- More complex setup
- Can't use autocert in swoopsd (use proxy's cert management instead)

### Solution 3: Run on High Ports

Disable autocert and run on high ports without TLS:

```yaml
server:
  host: 0.0.0.0
  port: 8080  # No special privileges needed
  autocert_enabled: false
```

Then use a reverse proxy (nginx/Caddy) for TLS termination.

### Solution 4: Run as Root (NOT Recommended)

Modify the systemd service to run as root:

```ini
[Service]
User=root
Group=root
ExecStart=/usr/local/bin/swoopsd --config /etc/swoops/config.yaml
```

**Warning**: This is a security risk. Use Solution 1 instead.

## Recommended Approach

For your setup with autocert, use **Solution 1** with the automated systemd approach:

1. Update your systemd service file to include `ExecStartPre`
2. This will automatically grant the capability after each update
3. No manual intervention needed after updates

## Verifying the Fix

After applying the fix, check the logs:

```bash
sudo journalctl -u swoopsd -f
```

You should see:
```
INFO: Swoops control plane starting on https://0.0.0.0:443 (automatic HTTPS via Let's Encrypt)
INFO: HTTP redirect server starting on :80 (for ACME challenges and HTTPS redirect)
```

And NO "permission denied" errors.

## Troubleshooting

### Capability not persisting after update

**Cause**: Capabilities are bound to the binary's inode. When you replace the binary (during update), the capability is lost.

**Solution**: Use the `ExecStartPre` approach in systemd to automatically re-grant the capability.

### "Operation not permitted" when setting capability

**Cause**: You need root access to grant capabilities.

**Solution**: Use `sudo`:
```bash
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/swoopsd
```

### SELinux blocking capability

If you're on RHEL/CentOS/Fedora with SELinux:

```bash
# Check if SELinux is blocking
sudo ausearch -m avc -ts recent

# Allow the capability
sudo setsebool -P httpd_can_network_connect 1
```

### Filesystem doesn't support capabilities

Some filesystems (like NFS, some tmpfs) don't support extended attributes needed for capabilities.

**Solution**: Move the binary to a filesystem that supports capabilities (usually the root filesystem):
```bash
sudo mv /usr/local/bin/swoopsd /opt/swoops/bin/swoopsd
```

## Summary

For autocert with Let's Encrypt:
1. Use Solution 1 (CAP_NET_BIND_SERVICE)
2. Add `ExecStartPre` to systemd service
3. Capabilities will be automatically granted after updates
4. No manual intervention needed
