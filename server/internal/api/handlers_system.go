package api

import (
	"net/http"
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
