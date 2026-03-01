package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/swoopsh/swoops/pkg/version"
)

const (
	githubOwner = "anthropics"
	githubRepo  = "swoops-control"
)

// handleVersion returns current version info and checks for updates
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	info := version.Get()

	// Check for updates in background (don't block response)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	updateInfo, err := version.CheckForUpdates(ctx, githubOwner, githubRepo)
	if err != nil {
		log.Printf("failed to check for updates: %v", err)
		// Still return current version info even if update check fails
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version":     info.Version,
			"git_commit":  info.GitCommit,
			"build_time":  info.BuildTime,
			"update_check_failed": true,
		})
		return
	}

	response := map[string]interface{}{
		"version":          info.Version,
		"git_commit":       info.GitCommit,
		"build_time":       info.BuildTime,
		"latest_version":   updateInfo.LatestVersion,
		"update_available": updateInfo.UpdateAvailable,
	}

	if updateInfo.UpdateAvailable {
		response["update_url"] = updateInfo.UpdateURL
		response["release_notes"] = updateInfo.ReleaseNotes
	}

	writeJSON(w, http.StatusOK, response)
}
