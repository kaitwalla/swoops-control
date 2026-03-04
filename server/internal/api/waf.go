package api

import (
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kaitwalla/swoops-control/server/internal/metrics"
)

// WAFConfig holds configuration for the WAF middleware
type WAFConfig struct {
	// Rate limiting
	RateLimitEnabled     bool
	RequestsPerMinute    int
	BurstSize            int

	// Request filtering
	FilterEnabled        bool
	MaxRequestBodySize   int64 // in bytes

	// IP control
	BlockedIPs           []string
	AllowedIPs           []string // if set, only these IPs are allowed

	// Bot protection
	BlockSuspiciousUA    bool

	// Logging
	LogBlockedRequests   bool
}

// DefaultWAFConfig returns sensible defaults
func DefaultWAFConfig() WAFConfig {
	return WAFConfig{
		RateLimitEnabled:   true,
		RequestsPerMinute:  60,
		BurstSize:          10,
		FilterEnabled:      true,
		MaxRequestBodySize: 10 * 1024 * 1024, // 10MB
		BlockSuspiciousUA:  true,
		LogBlockedRequests: true,
	}
}

// WAFMiddleware provides Web Application Firewall capabilities
type WAFMiddleware struct {
	config        WAFConfig
	logger        *slog.Logger

	// Rate limiting
	rateLimiter   *ipRateLimiter

	// IP blocking
	blockedIPs    map[string]bool
	allowedIPs    map[string]bool
	ipMutex       sync.RWMutex

	// Attack pattern detection
	patterns      *attackPatterns
}

// NewWAFMiddleware creates a new WAF middleware
func NewWAFMiddleware(config WAFConfig, logger *slog.Logger) *WAFMiddleware {
	waf := &WAFMiddleware{
		config:     config,
		logger:     logger,
		blockedIPs: make(map[string]bool),
		allowedIPs: make(map[string]bool),
		patterns:   newAttackPatterns(),
	}

	// Initialize rate limiter
	if config.RateLimitEnabled {
		waf.rateLimiter = newIPRateLimiter(config.RequestsPerMinute, config.BurstSize)
	}

	// Load blocked IPs
	for _, ip := range config.BlockedIPs {
		waf.blockedIPs[ip] = true
	}

	// Load allowed IPs
	for _, ip := range config.AllowedIPs {
		waf.allowedIPs[ip] = true
	}

	return waf
}

// Middleware returns the http middleware handler
func (waf *WAFMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)

		// Check IP allowlist (if configured)
		if len(waf.allowedIPs) > 0 && !waf.isIPAllowed(clientIP) {
			waf.blockRequest(w, r, "ip_not_allowed", clientIP)
			return
		}

		// Check IP blocklist
		if waf.isIPBlocked(clientIP) {
			waf.blockRequest(w, r, "ip_blocked", clientIP)
			return
		}

		// Rate limiting
		if waf.config.RateLimitEnabled {
			if !waf.rateLimiter.Allow(clientIP) {
				waf.blockRequest(w, r, "rate_limit_exceeded", clientIP)
				return
			}
		}

		// Check request size
		if r.ContentLength > waf.config.MaxRequestBodySize {
			waf.blockRequest(w, r, "request_too_large", clientIP)
			return
		}

		// Check for suspicious patterns
		if waf.config.FilterEnabled {
			if waf.patterns.detectAttack(r) {
				waf.blockRequest(w, r, "malicious_pattern_detected", clientIP)
				return
			}
		}

		// Check suspicious user agents
		if waf.config.BlockSuspiciousUA {
			if waf.isSuspiciousUserAgent(r.UserAgent()) {
				waf.blockRequest(w, r, "suspicious_user_agent", clientIP)
				return
			}
		}

		// Request passed all checks
		next.ServeHTTP(w, r)
	})
}

// blockRequest blocks a request and logs it
func (waf *WAFMiddleware) blockRequest(w http.ResponseWriter, r *http.Request, reason string, clientIP string) {
	// Emit metrics
	metrics.WAFRequestsBlocked.WithLabelValues(reason).Inc()

	if reason == "rate_limit_exceeded" {
		metrics.WAFRateLimitHits.Inc()
	}

	if waf.config.LogBlockedRequests {
		waf.logger.Warn("WAF blocked request",
			"reason", reason,
			"client_ip", clientIP,
			"method", r.Method,
			"path", r.URL.Path,
			"user_agent", r.UserAgent(),
			"referer", r.Referer(),
		)
	}

	http.Error(w, "Forbidden", http.StatusForbidden)
}

// isIPBlocked checks if an IP is blocked
func (waf *WAFMiddleware) isIPBlocked(ip string) bool {
	waf.ipMutex.RLock()
	defer waf.ipMutex.RUnlock()
	return waf.blockedIPs[ip]
}

// isIPAllowed checks if an IP is in the allowlist
func (waf *WAFMiddleware) isIPAllowed(ip string) bool {
	waf.ipMutex.RLock()
	defer waf.ipMutex.RUnlock()
	return waf.allowedIPs[ip]
}

// BlockIP adds an IP to the blocklist
func (waf *WAFMiddleware) BlockIP(ip string) {
	waf.ipMutex.Lock()
	defer waf.ipMutex.Unlock()
	waf.blockedIPs[ip] = true
	waf.logger.Info("IP blocked", "ip", ip)
}

// UnblockIP removes an IP from the blocklist
func (waf *WAFMiddleware) UnblockIP(ip string) {
	waf.ipMutex.Lock()
	defer waf.ipMutex.Unlock()
	delete(waf.blockedIPs, ip)
	waf.logger.Info("IP unblocked", "ip", ip)
}

// isSuspiciousUserAgent checks for common scanner/bot user agents
func (waf *WAFMiddleware) isSuspiciousUserAgent(ua string) bool {
	if ua == "" {
		return false // Allow empty UA for API clients
	}

	ua = strings.ToLower(ua)

	suspiciousPatterns := []string{
		"masscan",
		"nmap",
		"nikto",
		"sqlmap",
		"acunetix",
		"nessus",
		"openvas",
		"metasploit",
		"burpsuite",
		"havij",
		"dirbuster",
		"gobuster",
		"nuclei",
		"zgrab",
		"shodan",
		"censys",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(ua, pattern) {
			return true
		}
	}

	return false
}

// getClientIP extracts the real client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (if behind proxy)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ipRateLimiter implements per-IP rate limiting
type ipRateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int
	burst    int
	cleanup  time.Duration
}

type visitor struct {
	limiter  *rateLimiter
	lastSeen time.Time
}

type rateLimiter struct {
	tokens    int
	maxTokens int
	refillRate int
	lastRefill time.Time
	mu        sync.Mutex
}

func newIPRateLimiter(requestsPerMinute, burst int) *ipRateLimiter {
	rl := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     requestsPerMinute,
		burst:    burst,
		cleanup:  5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanupVisitors()

	return rl
}

func (rl *ipRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{
			limiter: &rateLimiter{
				tokens:    rl.burst,
				maxTokens: rl.burst,
				refillRate: rl.rate,
				lastRefill: time.Now(),
			},
			lastSeen: time.Now(),
		}
		rl.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	rl.mu.Unlock()

	return v.limiter.Allow()
}

func (r *rateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	tokensToAdd := int(elapsed.Minutes() * float64(r.refillRate))

	if tokensToAdd > 0 {
		r.tokens += tokensToAdd
		if r.tokens > r.maxTokens {
			r.tokens = r.maxTokens
		}
		r.lastRefill = now
	}

	// Check if we have tokens
	if r.tokens > 0 {
		r.tokens--
		return true
	}

	return false
}

func (rl *ipRateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.cleanup {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// attackPatterns holds patterns for detecting common attacks
type attackPatterns struct {
	sqlInjection   []*regexp.Regexp
	xss            []*regexp.Regexp
	pathTraversal  []*regexp.Regexp
	commandInjection []*regexp.Regexp
}

func newAttackPatterns() *attackPatterns {
	return &attackPatterns{
		sqlInjection: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\bunion\b.*\bselect\b|\bselect\b.*\bfrom\b)`),
			regexp.MustCompile(`(?i)(\bor\b\s+1\s*=\s*1|\band\b\s+1\s*=\s*1)`),
			regexp.MustCompile(`(?i)(;.*drop\s+table|;.*insert\s+into|;.*update\s+)`),
			regexp.MustCompile(`(?i)(exec\s*\(|execute\s*\(|0x[0-9a-f]+)`),
		},
		xss: []*regexp.Regexp{
			regexp.MustCompile(`(?i)<script[^>]*>.*</script>`),
			regexp.MustCompile(`(?i)javascript:`),
			regexp.MustCompile(`(?i)on(load|error|click|mouseover)\s*=`),
			regexp.MustCompile(`(?i)<iframe[^>]*>`),
		},
		pathTraversal: []*regexp.Regexp{
			regexp.MustCompile(`\.\.\/`),
			regexp.MustCompile(`\.\.\\`),
			regexp.MustCompile(`%2e%2e[\/\\]`),
			regexp.MustCompile(`(?i)(\/etc\/passwd|\/etc\/shadow|c:\\windows\\system32)`),
		},
		commandInjection: []*regexp.Regexp{
			regexp.MustCompile(`[;&|]\s*(cat|ls|wget|curl|nc|bash|sh|cmd|powershell)\s`),
			regexp.MustCompile(`\$\([^)]*\)`),
			regexp.MustCompile("`[^`]*`"),
		},
	}
}

func (ap *attackPatterns) detectAttack(r *http.Request) bool {
	// Check URL path
	if ap.checkString(r.URL.Path) {
		return true
	}

	// Check query parameters
	for _, values := range r.URL.Query() {
		for _, value := range values {
			if ap.checkString(value) {
				return true
			}
		}
	}

	// Check common headers
	headersToCheck := []string{"Referer", "User-Agent", "Cookie"}
	for _, header := range headersToCheck {
		if ap.checkString(r.Header.Get(header)) {
			return true
		}
	}

	return false
}

func (ap *attackPatterns) checkString(s string) bool {
	// Check all pattern types
	allPatterns := [][]*regexp.Regexp{
		ap.sqlInjection,
		ap.xss,
		ap.pathTraversal,
		ap.commandInjection,
	}

	for _, patterns := range allPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(s) {
				return true
			}
		}
	}

	return false
}

// GetStats returns WAF statistics
func (waf *WAFMiddleware) GetStats() map[string]interface{} {
	waf.ipMutex.RLock()
	blockedCount := len(waf.blockedIPs)
	allowedCount := len(waf.allowedIPs)
	waf.ipMutex.RUnlock()

	var visitorCount int
	if waf.rateLimiter != nil {
		waf.rateLimiter.mu.RLock()
		visitorCount = len(waf.rateLimiter.visitors)
		waf.rateLimiter.mu.RUnlock()
	}

	return map[string]interface{}{
		"blocked_ips":     blockedCount,
		"allowed_ips":     allowedCount,
		"active_visitors": visitorCount,
		"config": map[string]interface{}{
			"rate_limit_enabled":   waf.config.RateLimitEnabled,
			"requests_per_minute":  waf.config.RequestsPerMinute,
			"filter_enabled":       waf.config.FilterEnabled,
			"max_request_size":     waf.config.MaxRequestBodySize,
			"block_suspicious_ua":  waf.config.BlockSuspiciousUA,
		},
	}
}
