# Automatic Certificate Rotation

Swoops now supports automatic certificate rotation without requiring server restarts. This is critical for production deployments where uptime is important and certificates need to be renewed periodically.

## Overview

The certificate rotation system monitors certificate files and automatically reloads them when they change. This works for both:

1. **HTTP/HTTPS Server** - Web interface and API
2. **gRPC Server** - Agent connections

## How It Works

### Let's Encrypt (Autocert)

For Let's Encrypt certificates via `autocert`, certificate rotation is **built-in and automatic**:

```yaml
server:
  autocert_enabled: true
  autocert_domain: swoops.example.com
  autocert_email: admin@example.com
  autocert_cache_dir: /var/cache/swoops/autocert
```

- Certificates are automatically obtained and renewed
- No manual intervention required
- Renewals happen ~30 days before expiration
- **No server restart needed**

### Manual TLS Certificates

For manually provided certificates, Swoops now includes automatic rotation:

```yaml
server:
  tls_enabled: true
  tls_cert: /etc/ssl/certs/swoops.crt
  tls_key: /etc/ssl/private/swoops.key
```

**How it works:**
1. Certificate files are monitored every 5 minutes
2. When modification time changes, certificates are automatically reloaded
3. New connections use the updated certificate
4. Existing connections continue with the old certificate until they close
5. **No server restart needed**

### gRPC TLS Certificates

gRPC certificates are also monitored and rotated automatically:

```yaml
grpc:
  tls_cert: /etc/ssl/certs/grpc.crt
  tls_key: /etc/ssl/private/grpc.key
  client_ca: /etc/ssl/certs/ca.crt  # Optional for mTLS
  require_mtls: true
```

**Features:**
- Server certificate rotation (cert + key)
- CA certificate rotation (for client verification in mTLS)
- Checks every 5 minutes for file changes
- **No server restart needed**

## Certificate Renewal Workflow

### Example: Certbot with Manual Certificates

If you're using Certbot to manage Let's Encrypt certificates manually:

```bash
# Renew certificates
sudo certbot renew

# Copy to Swoops location
sudo cp /etc/letsencrypt/live/swoops.example.com/fullchain.pem /etc/ssl/certs/swoops.crt
sudo cp /etc/letsencrypt/live/swoops.example.com/privkey.pem /etc/ssl/private/swoops.key

# Update modification time to trigger reload
touch /etc/ssl/certs/swoops.crt

# Swoops will automatically detect and reload within 5 minutes
# No restart required!
```

### Automated Renewal with Certbot Hook

Create a renewal hook at `/etc/letsencrypt/renewal-hooks/deploy/swoops-deploy.sh`:

```bash
#!/bin/bash
# This script runs after successful certificate renewal

DOMAIN="swoops.example.com"
CERT_PATH="/etc/ssl/certs/swoops.crt"
KEY_PATH="/etc/ssl/private/swoops.key"

# Copy new certificates
cp "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" "$CERT_PATH"
cp "/etc/letsencrypt/live/$DOMAIN/privkey.pem" "$KEY_PATH"

# Set proper permissions
chmod 644 "$CERT_PATH"
chmod 600 "$KEY_PATH"

# Swoops will automatically reload within 5 minutes
echo "Certificates updated for Swoops. Automatic reload will occur within 5 minutes."
```

Make it executable:
```bash
chmod +x /etc/letsencrypt/renewal-hooks/deploy/swoops-deploy.sh
```

## Monitoring Certificate Rotation

### Check Logs

Certificate rotation events are logged:

```json
{
  "time": "2026-03-04T10:15:00Z",
  "level": "INFO",
  "msg": "Certificate files changed, reloading...",
  "cert_file": "/etc/ssl/certs/swoops.crt",
  "key_file": "/etc/ssl/private/swoops.key"
}
```

```json
{
  "time": "2026-03-04T10:15:00Z",
  "level": "INFO",
  "msg": "Certificate reloaded successfully"
}
```

### Verify Certificate in Use

Check the certificate currently being served:

```bash
# For HTTPS
echo | openssl s_client -connect swoops.example.com:443 -servername swoops.example.com 2>/dev/null | \
  openssl x509 -noout -dates

# For gRPC
echo | openssl s_client -connect swoops.example.com:9090 2>/dev/null | \
  openssl x509 -noout -dates
```

## Configuration Options

### Check Interval

The default check interval is 5 minutes. This is hardcoded in `server/internal/certrotate/certrotate.go`:

```go
checkInterval: 5 * time.Minute
```

To modify, edit the file and rebuild:
```go
checkInterval: 1 * time.Minute  // Check every minute
```

## Troubleshooting

### Certificates Not Reloading

**Check file permissions:**
```bash
ls -la /etc/ssl/certs/swoops.crt
ls -la /etc/ssl/private/swoops.key
```

The swoops process must have read access to both files.

**Check modification time:**
```bash
stat /etc/ssl/certs/swoops.crt
```

The modification time must change for rotation to trigger. If you copy over the same certificate, use `touch`:
```bash
touch /etc/ssl/certs/swoops.crt
```

**Check logs:**
```bash
journalctl -u swoopsd -f | grep -i certificate
```

### Error Loading Certificate

If you see this error:
```json
{
  "level": "ERROR",
  "msg": "Failed to reload certificate",
  "error": "tls: failed to find any PEM data in certificate input"
}
```

**Causes:**
- File is empty or corrupt
- File permissions prevent reading
- File path is incorrect

**Solution:**
```bash
# Verify certificate is valid PEM format
openssl x509 -in /etc/ssl/certs/swoops.crt -text -noout

# Verify key matches certificate
openssl rsa -in /etc/ssl/private/swoops.key -check
```

### Existing Connections

**Important:** Certificate rotation only affects **new** connections. Existing connections will continue using the old certificate until they close.

For HTTP/HTTPS:
- Most browsers close idle connections after a few seconds
- New page loads will use the new certificate

For gRPC:
- Long-lived agent connections may need to reconnect
- Agents automatically reconnect on connection errors

## Best Practices

1. **Use Let's Encrypt with autocert** - Simplest approach, fully automatic
2. **Test certificate rotation** before relying on it in production
3. **Monitor logs** for reload success/failure
4. **Set up alerts** for certificate expiration (even with auto-renewal)
5. **Keep backup certificates** in case of renewal failures
6. **Use proper file permissions** (cert: 644, key: 600)
7. **Coordinate with load balancers** if using multiple servers

## Security Considerations

- **File permissions:** Private keys should be readable only by the swoops user
- **Atomic updates:** Use `mv` instead of `cp` when possible to ensure atomic file updates
- **Validation:** Always test new certificates before deploying:
  ```bash
  openssl verify -CAfile ca.crt swoops.crt
  ```
- **Monitoring:** Set up alerts for certificate expiration as a safety net

## Comparison with Other Solutions

| Approach | Restart Required | Complexity | Automatic Renewal |
|----------|------------------|------------|-------------------|
| **Autocert (Let's Encrypt)** | No | Low | Yes |
| **Manual + Rotation (Swoops)** | No | Medium | With hooks |
| **Manual without Rotation** | Yes | Low | With hooks + restart |
| **Reverse Proxy (nginx/Caddy)** | No* | Medium | With proxy config |

\* The reverse proxy may need reload, but Swoops doesn't

## Future Enhancements

Potential improvements for future versions:

- [ ] Configurable check interval via config file
- [ ] Webhook notifications on certificate reload
- [ ] Certificate expiration warnings in logs
- [ ] API endpoint to view current certificate info
- [ ] Manual reload trigger via API
- [ ] Support for certificate bundles
- [ ] Integration with HashiCorp Vault
- [ ] Support for ACME protocols beyond Let's Encrypt

## Related Files

- `server/internal/certrotate/certrotate.go` - Certificate rotation implementation
- `server/cmd/swoopsd/main.go` - Integration into main server (lines 107-121, 143-176)
- `WAF_README.md` - Web Application Firewall documentation
