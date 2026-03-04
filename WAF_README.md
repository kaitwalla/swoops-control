# Web Application Firewall (WAF) for Swoops

The Swoops control plane now includes a built-in Web Application Firewall to protect against malicious requests and scanning attempts.

## Features

### 1. **Rate Limiting**
- Per-IP rate limiting to prevent abuse
- Configurable requests per minute and burst size
- Automatic cleanup of inactive rate limit entries

### 2. **Malicious Pattern Detection**
Detects and blocks common attack patterns:
- **SQL Injection**: `' OR 1=1`, `UNION SELECT`, `DROP TABLE`, etc.
- **Cross-Site Scripting (XSS)**: `<script>`, `javascript:`, `onerror=`, etc.
- **Path Traversal**: `../`, `..\\`, `/etc/passwd`, etc.
- **Command Injection**: Shell commands, backticks, `$()`

### 3. **Suspicious User Agent Blocking**
Blocks known scanner and bot user agents:
- Vulnerability scanners: Nikto, Nessus, OpenVAS, Acunetix, SQLMap
- Port scanners: Nmap, Masscan, ZGrab
- Web crawlers: Shodan, Censys
- Penetration testing tools: Metasploit, Burp Suite, DirBuster, Gobuster

### 4. **IP Blocking/Allowlisting**
- Block specific IP addresses
- Allowlist mode: only permit specific IPs
- Dynamic IP blocking via API (no restart required)

### 5. **Request Size Limits**
- Configurable maximum request body size
- Prevents DoS attacks via large payloads

### 6. **Prometheus Metrics**
- `swoops_waf_requests_blocked_total` - Total blocked requests by reason
- `swoops_waf_rate_limit_hits_total` - Rate limit violations
- `swoops_waf_malicious_patterns_total` - Malicious patterns detected

## Configuration

Add the `waf` section to your `config.yaml`:

```yaml
waf:
  # Enable WAF protection
  enabled: true

  # Rate limiting settings
  rate_limit_enabled: true
  requests_per_minute: 60  # Max 60 requests per IP per minute
  burst_size: 10           # Allow burst of 10 requests

  # Request filtering
  filter_enabled: true
  max_request_body_size: 10485760  # 10MB in bytes

  # Block suspicious user agents (scanners, bots)
  block_suspicious_ua: true

  # Log all blocked requests for security monitoring
  log_blocked_requests: true

  # Block specific IPs
  blocked_ips:
    - "192.168.1.100"
    - "10.0.0.50"

  # Allow only specific IPs (if set, only these IPs can access)
  # Leave empty to allow all IPs (subject to other WAF rules)
  allowed_ips: []
```

## API Endpoints

All WAF management endpoints require authentication.

### Get WAF Statistics
```bash
GET /api/v1/waf/stats
```

Returns:
```json
{
  "blocked_ips": 2,
  "allowed_ips": 0,
  "active_visitors": 5,
  "config": {
    "rate_limit_enabled": true,
    "requests_per_minute": 60,
    "filter_enabled": true,
    "max_request_size": 10485760,
    "block_suspicious_ua": true
  }
}
```

### Get WAF Configuration
```bash
GET /api/v1/waf/config
```

### Update WAF Configuration
```bash
PUT /api/v1/waf/config
Content-Type: application/json

{
  "enabled": true,
  "rate_limit_enabled": true,
  "requests_per_minute": 100,
  "burst_size": 20
}
```

**Note**: Most configuration changes require a server restart. Only IP blocking/unblocking takes effect immediately.

### Block an IP Address (Immediate Effect)
```bash
POST /api/v1/waf/block-ip
Content-Type: application/json

{
  "ip": "192.168.1.100"
}
```

### Unblock an IP Address (Immediate Effect)
```bash
POST /api/v1/waf/unblock-ip
Content-Type: application/json

{
  "ip": "192.168.1.100"
}
```

## Monitoring

### View Blocked Requests in Logs
When `log_blocked_requests: true`, blocked requests appear in structured logs:

```json
{
  "time": "2026-03-04T09:58:29.241356-08:00",
  "level": "WARN",
  "msg": "WAF blocked request",
  "reason": "suspicious_user_agent",
  "client_ip": "192.168.1.50",
  "method": "GET",
  "path": "/api/v1/health",
  "user_agent": "Nikto/2.1.6",
  "referer": ""
}
```

### Prometheus Metrics
Access metrics at `/metrics`:

```bash
curl http://localhost:8080/metrics | grep waf
```

Example output:
```
swoops_waf_requests_blocked_total{reason="malicious_pattern_detected"} 15
swoops_waf_requests_blocked_total{reason="rate_limit_exceeded"} 42
swoops_waf_requests_blocked_total{reason="suspicious_user_agent"} 8
swoops_waf_rate_limit_hits_total 42
```

## Testing

See [example-waf-config.yaml](./example-waf-config.yaml) for a complete configuration example.

### Test WAF Protection
```bash
# Normal request - should succeed
curl http://localhost:8080/api/v1/health

# SQL injection - should be blocked
curl "http://localhost:8080/api/v1/health?id=1' OR 1=1--"

# XSS attempt - should be blocked
curl "http://localhost:8080/api/v1/health?name=<script>alert('xss')</script>"

# Scanner user agent - should be blocked
curl -A "Nikto/2.1.6" http://localhost:8080/api/v1/health

# Rate limit test - send 100 requests rapidly
for i in {1..100}; do curl -s http://localhost:8080/api/v1/health; done
```

## Block Reasons

The WAF can block requests for the following reasons:

| Reason | Description |
|--------|-------------|
| `ip_not_allowed` | IP not in allowlist (when allowlist is configured) |
| `ip_blocked` | IP is in the blocklist |
| `rate_limit_exceeded` | Too many requests from this IP |
| `request_too_large` | Request body exceeds size limit |
| `malicious_pattern_detected` | SQL injection, XSS, path traversal, or command injection detected |
| `suspicious_user_agent` | Known scanner or bot user agent |

## Best Practices

1. **Enable logging in production** to track attack attempts
2. **Monitor Prometheus metrics** to identify patterns
3. **Tune rate limits** based on your legitimate traffic patterns
4. **Use allowlists carefully** - they can lock you out if misconfigured
5. **Review blocked requests regularly** to adjust rules and reduce false positives
6. **Combine with reverse proxy** (nginx/Caddy) for additional protection
7. **Enable TLS** to prevent credential interception

## Performance Impact

The WAF middleware is designed to be lightweight:
- Pattern matching uses compiled regex for speed
- Rate limiting uses in-memory maps with periodic cleanup
- Most checks complete in microseconds
- Minimal memory overhead per active IP

For high-traffic deployments, consider:
- Increasing `requests_per_minute` appropriately
- Using a dedicated WAF appliance or CDN for edge protection
- Monitoring the `/metrics` endpoint for performance metrics

## Limitations

- **Not a replacement for a full WAF solution** (like ModSecurity or cloud WAFs)
- Pattern matching may have false positives - review logs regularly
- IP-based blocking can be bypassed with proxies/VPNs
- Rate limiting is per-instance (not shared across multiple servers)
- Configuration changes (except IP blocking) require restart

## Future Enhancements

Potential improvements for future versions:
- Geoblocking by country
- Advanced pattern learning
- Integration with threat intelligence feeds
- Distributed rate limiting (Redis-backed)
- CAPTCHA challenges for suspicious requests
- Automatic IP blocking based on behavior
