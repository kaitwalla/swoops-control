package api

import (
	"net/http"
	"os"
	"path/filepath"
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
