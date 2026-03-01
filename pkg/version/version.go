package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Build information - set via -ldflags at build time
var (
	Version   = "dev"      // e.g., "1.2.3" or "dev"
	GitCommit = "unknown"  // short git SHA
	BuildTime = "unknown"  // RFC3339 timestamp
)

// Info represents version and build information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
}

// Get returns the current version info
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
	}
}

// String returns a human-readable version string
func (i Info) String() string {
	if i.Version == "dev" {
		return fmt.Sprintf("swoops dev (%s, built %s)", i.GitCommit, i.BuildTime)
	}
	return fmt.Sprintf("swoops v%s (%s)", i.Version, i.GitCommit)
}

// CheckUpdate queries GitHub releases API for the latest version
type UpdateInfo struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	UpdateURL      string `json:"update_url"`
	UpdateAvailable bool  `json:"update_available"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
}

// CheckForUpdates checks GitHub releases for a newer version
func CheckForUpdates(ctx context.Context, owner, repo string) (*UpdateInfo, error) {
	info := Get()

	// Don't check for updates on dev builds
	if info.Version == "dev" {
		return &UpdateInfo{
			CurrentVersion:  info.Version,
			LatestVersion:   info.Version,
			UpdateAvailable: false,
		}, nil
	}

	latest, err := fetchLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}

	updateAvailable := compareVersions(info.Version, latest.TagName)

	return &UpdateInfo{
		CurrentVersion:  info.Version,
		LatestVersion:   latest.TagName,
		UpdateURL:       latest.HTMLURL,
		UpdateAvailable: updateAvailable,
		ReleaseNotes:    latest.Body,
	}, nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

func fetchLatestRelease(ctx context.Context, owner, repo string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("swoops/%s", Version))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// compareVersions returns true if latest > current
// Assumes semantic versioning (v1.2.3)
func compareVersions(current, latest string) bool {
	// Strip 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Simple string comparison works for semantic versions in most cases
	// For production, consider using a proper semver library
	return latest > current
}
