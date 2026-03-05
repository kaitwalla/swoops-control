package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.store.ListHosts()
	if err != nil {
		writeInternalError(w, err)
		return
	}

	sessions, err := s.store.ListSessions("", "")
	if err != nil {
		writeInternalError(w, err)
		return
	}

	onlineHosts := 0
	for _, h := range hosts {
		if h.Status == "online" {
			onlineHosts++
		}
	}

	statusCounts := make(map[string]int)
	for _, sess := range sessions {
		statusCounts[string(sess.Status)]++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_hosts":      len(hosts),
		"online_hosts":     onlineHosts,
		"total_sessions":   len(sessions),
		"sessions_by_status": statusCounts,
	})
}

// handleGetCACert serves the CA certificate for agents to download
// This endpoint is unauthenticated to allow agents to bootstrap TLS connections
func (s *Server) handleGetCACert(w http.ResponseWriter, r *http.Request) {
	// If gRPC is running in insecure mode, there's no CA cert to serve
	if s.config.GRPC.Insecure {
		writeError(w, http.StatusNotFound, "TLS is not enabled on this server")
		return
	}

	// Determine which CA cert to serve
	// For mTLS, we serve the client CA (which agents need to verify the server)
	// For regular TLS, we serve the CA that signed the server cert
	var certPath string

	// First check if there's a client CA (mTLS setup)
	if s.config.GRPC.ClientCA != "" {
		certPath = s.config.GRPC.ClientCA
	} else {
		// Try to find the CA cert by looking in the same directory as the server cert
		if s.config.GRPC.TLSCert != "" {
			certDir := filepath.Dir(s.config.GRPC.TLSCert)
			// Common CA cert names
			possibleNames := []string{"ca-cert.pem", "server-ca.pem", "root_ca.crt", "ca.pem"}
			for _, name := range possibleNames {
				path := filepath.Join(certDir, name)
				if _, err := os.Stat(path); err == nil {
					certPath = path
					break
				}
			}
		}
	}

	if certPath == "" {
		writeError(w, http.StatusNotFound, "CA certificate not found on server")
		return
	}

	// Read the certificate file
	certData, err := os.ReadFile(certPath)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	// Serve as PEM file
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=\"ca-cert.pem\"")
	w.WriteHeader(http.StatusOK)
	w.Write(certData)
}

// handleGetServerInfo returns server configuration information needed for agent setup
func (s *Server) handleGetServerInfo(w http.ResponseWriter, r *http.Request) {
	// Determine the gRPC server address
	grpcHost := s.config.GRPC.Host
	if grpcHost == "0.0.0.0" || grpcHost == "" {
		// Get the hostname from the request or use external URL
		if s.config.Server.ExternalURL != "" {
			// Extract hostname from external URL
			url := s.config.Server.ExternalURL
			url = strings.TrimPrefix(url, "http://")
			url = strings.TrimPrefix(url, "https://")
			if idx := strings.Index(url, ":"); idx != -1 {
				grpcHost = url[:idx]
			} else if idx := strings.Index(url, "/"); idx != -1 {
				grpcHost = url[:idx]
			} else {
				grpcHost = url
			}
		} else {
			grpcHost = r.Host
			if idx := strings.Index(grpcHost, ":"); idx != -1 {
				grpcHost = grpcHost[:idx]
			}
		}
	}

	grpcAddr := fmt.Sprintf("%s:%d", grpcHost, s.config.GRPC.Port)

	// Determine the HTTP URL for downloading CA cert
	httpURL := s.config.Server.ExternalURL
	if httpURL == "" {
		scheme := "http"
		if s.config.Server.TLSEnabled || s.config.Server.AutocertEnabled {
			scheme = "https"
		}
		httpURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	// Generate setup command
	setupCmd := fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/kaitwalla/swoops-control/main/setup.sh | bash -s -- --server %s", grpcAddr)

	if !s.config.GRPC.Insecure {
		setupCmd += fmt.Sprintf(" --download-ca --http-url %s", httpURL)
	}

	// If host_id and auth_token are provided as query params, include them in the setup command
	hostID := r.URL.Query().Get("host_id")
	authToken := r.URL.Query().Get("auth_token")
	if hostID != "" && authToken != "" {
		setupCmd += fmt.Sprintf(" --host-id %s --auth-token %s", hostID, authToken)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"grpc_address":  grpcAddr,
		"grpc_secure":   !s.config.GRPC.Insecure,
		"http_url":      httpURL,
		"setup_command": setupCmd,
	})
}
