package api

import (
	"encoding/json"
	"net/http"
)

// handleWAFStats returns WAF statistics
func (s *Server) handleWAFStats(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "WAF is not enabled",
		})
		return
	}

	stats := s.waf.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

// handleGetWAFConfig returns the current WAF configuration
func (s *Server) handleGetWAFConfig(w http.ResponseWriter, r *http.Request) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	writeJSON(w, http.StatusOK, s.config.WAF)
}

// handleUpdateWAFConfig updates WAF configuration (requires server restart to take effect)
func (s *Server) handleUpdateWAFConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled            *bool    `json:"enabled"`
		RateLimitEnabled   *bool    `json:"rate_limit_enabled"`
		RequestsPerMinute  *int     `json:"requests_per_minute"`
		BurstSize          *int     `json:"burst_size"`
		FilterEnabled      *bool    `json:"filter_enabled"`
		MaxRequestBodySize *int64   `json:"max_request_body_size"`
		BlockSuspiciousUA  *bool    `json:"block_suspicious_ua"`
		LogBlockedRequests *bool    `json:"log_blocked_requests"`
		BlockedIPs         []string `json:"blocked_ips"`
		AllowedIPs         []string `json:"allowed_ips"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate numeric fields
	if req.RequestsPerMinute != nil && *req.RequestsPerMinute <= 0 {
		writeError(w, http.StatusBadRequest, "requests_per_minute must be positive")
		return
	}
	if req.BurstSize != nil && *req.BurstSize <= 0 {
		writeError(w, http.StatusBadRequest, "burst_size must be positive")
		return
	}
	if req.MaxRequestBodySize != nil && *req.MaxRequestBodySize <= 0 {
		writeError(w, http.StatusBadRequest, "max_request_body_size must be positive")
		return
	}

	// Update config values with synchronization
	s.configMu.Lock()
	defer s.configMu.Unlock()

	if req.Enabled != nil {
		s.config.WAF.Enabled = *req.Enabled
	}
	if req.RateLimitEnabled != nil {
		s.config.WAF.RateLimitEnabled = *req.RateLimitEnabled
	}
	if req.RequestsPerMinute != nil {
		s.config.WAF.RequestsPerMinute = *req.RequestsPerMinute
	}
	if req.BurstSize != nil {
		s.config.WAF.BurstSize = *req.BurstSize
	}
	if req.FilterEnabled != nil {
		s.config.WAF.FilterEnabled = *req.FilterEnabled
	}
	if req.MaxRequestBodySize != nil {
		s.config.WAF.MaxRequestBodySize = *req.MaxRequestBodySize
	}
	if req.BlockSuspiciousUA != nil {
		s.config.WAF.BlockSuspiciousUA = *req.BlockSuspiciousUA
	}
	if req.LogBlockedRequests != nil {
		s.config.WAF.LogBlockedRequests = *req.LogBlockedRequests
	}
	if req.BlockedIPs != nil {
		// Make defensive copy to avoid shared memory races
		s.config.WAF.BlockedIPs = append([]string(nil), req.BlockedIPs...)
	}
	if req.AllowedIPs != nil {
		// Make defensive copy to avoid shared memory races
		s.config.WAF.AllowedIPs = append([]string(nil), req.AllowedIPs...)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "WAF configuration updated - restart server for changes to take effect",
		"config":  s.config.WAF,
	})
}

// handleBlockIP adds an IP to the blocklist (takes effect immediately)
func (s *Server) handleBlockIP(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		writeError(w, http.StatusBadRequest, "WAF is not enabled")
		return
	}

	var req struct {
		IP string `json:"ip"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.IP == "" {
		writeError(w, http.StatusBadRequest, "IP address is required")
		return
	}

	s.waf.BlockIP(req.IP)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "IP blocked successfully",
		"ip":      req.IP,
	})
}

// handleUnblockIP removes an IP from the blocklist (takes effect immediately)
func (s *Server) handleUnblockIP(w http.ResponseWriter, r *http.Request) {
	if s.waf == nil {
		writeError(w, http.StatusBadRequest, "WAF is not enabled")
		return
	}

	var req struct {
		IP string `json:"ip"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.IP == "" {
		writeError(w, http.StatusBadRequest, "IP address is required")
		return
	}

	s.waf.UnblockIP(req.IP)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "IP unblocked successfully",
		"ip":      req.IP,
	})
}
