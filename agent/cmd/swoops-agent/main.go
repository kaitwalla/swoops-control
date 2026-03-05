package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/tmux"
	"github.com/kaitwalla/swoops-control/pkg/version"
	"github.com/kaitwalla/swoops-control/pkg/worktree"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: swoops-agent <command>")
		fmt.Println("Commands:")
		fmt.Println("  run               Start the agent daemon")
		fmt.Println("  version           Show version information")
		fmt.Println("  service-install   Install and start agent service (systemd/launchd)")
		fmt.Println("  service-uninstall Stop and uninstall agent service")
		fmt.Println("  service-status    Show agent service status")
		fmt.Println("  mcp-serve         Start MCP server mode for a session")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		versionInfo := version.Get()
		fmt.Println(versionInfo.String())
		os.Exit(0)
	case "run":
		if err := runCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "mcp-serve":
		if err := mcpServeCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "service-install":
		if err := serviceInstallCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "service-uninstall":
		if err := serviceUninstallCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "service-status":
		if err := serviceStatusCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	serverAddr := fs.String("server", "127.0.0.1:9090", "control-plane gRPC address")
	hostID := fs.String("host-id", "", "registered host ID from control plane")
	authToken := fs.String("auth-token", "", "agent authentication token (or set SWOOPS_AGENT_TOKEN)")
	hostName := fs.String("host-name", "", "logical host name override (defaults to OS hostname)")
	tlsCert := fs.String("tls-cert", "", "path to client TLS certificate for mTLS")
	tlsKey := fs.String("tls-key", "", "path to client TLS private key for mTLS")
	serverCA := fs.String("server-ca", "", "path to server CA certificate (for TLS verification)")
	insecure := fs.Bool("insecure", true, "use insecure connection (no TLS)")
	httpURL := fs.String("http-url", "", "control plane HTTP URL (for downloading CA certificate)")
	downloadCA := fs.Bool("download-ca", false, "automatically download CA certificate from server")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *hostID == "" {
		return errors.New("--host-id is required")
	}

	// Handle CA certificate download if requested
	if *downloadCA && !*insecure {
		if *serverCA == "" {
			// Default CA path
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home directory: %w", err)
			}
			caDir := filepath.Join(homeDir, ".config", "swoops", "certs")
			if err := os.MkdirAll(caDir, 0755); err != nil {
				return fmt.Errorf("create certs directory: %w", err)
			}
			*serverCA = filepath.Join(caDir, "server-ca.pem")
		}

		// Determine HTTP URL if not provided
		httpURLStr := *httpURL
		if httpURLStr == "" {
			// Extract host:port from gRPC address and default to HTTP on port 8080
			host := strings.Split(*serverAddr, ":")[0]
			httpURLStr = fmt.Sprintf("http://%s:8080", host)
		}

		log.Printf("Downloading CA certificate from %s/api/v1/ca-cert", httpURLStr)
		if err := downloadCACertificate(httpURLStr, *serverCA); err != nil {
			return fmt.Errorf("download CA certificate: %w", err)
		}
		log.Printf("CA certificate downloaded to %s", *serverCA)
	}

	// Get auth token from flag or environment
	token := *authToken
	if token == "" {
		token = os.Getenv("SWOOPS_AGENT_TOKEN")
	}
	if token == "" {
		return errors.New("--auth-token is required (or set SWOOPS_AGENT_TOKEN)")
	}

	name := *hostName
	if name == "" {
		osName, err := os.Hostname()
		if err == nil && osName != "" {
			name = osName
		}
	}
	if name == "" {
		name = *hostID
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	versionInfo := version.Get()
	log.Printf("swoops-agent starting: %s host_id=%s server=%s insecure=%v", versionInfo.String(), *hostID, *serverAddr, *insecure)

	// Create a channel to send update info to the runtime
	updateInfoChan := make(chan *version.UpdateInfo, 1)

	// Check for updates periodically in background
	go func() {
		checkForUpdate := func() {
			checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			updateInfo, err := version.CheckForUpdates(checkCtx, "kaitwalla", "swoops-control")
			if err != nil {
				log.Printf("version check: failed to check for updates: %v", err)
				return
			}
			if updateInfo.UpdateAvailable {
				log.Printf("⚠️  Update available: v%s → v%s", updateInfo.CurrentVersion, updateInfo.LatestVersion)
				log.Printf("   Download: %s", updateInfo.UpdateURL)
			}
			// Send update info to channel (non-blocking)
			select {
			case updateInfoChan <- updateInfo:
			default:
			}
		}

		// Check on startup
		checkForUpdate()

		// Check every 24 hours
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkForUpdate()
			}
		}
	}()

	tlsConfig := &tlsClientConfig{
		insecure:  *insecure,
		tlsCert:   *tlsCert,
		tlsKey:    *tlsKey,
		serverCA:  *serverCA,
	}

	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		err := connectAndRun(ctx, *serverAddr, *hostID, token, name, tlsConfig, updateInfoChan)
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}

		log.Printf("agent connection closed: %v (retry in %s)", err, backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

type tlsClientConfig struct {
	insecure  bool
	tlsCert   string
	tlsKey    string
	serverCA  string
}

func connectAndRun(ctx context.Context, serverAddr, hostID, authToken, hostName string, tlsConfig *tlsClientConfig, updateInfoChan <-chan *version.UpdateInfo) error {
	// Configure gRPC credentials based on TLS settings
	var creds credentials.TransportCredentials
	if tlsConfig.insecure {
		creds = insecure.NewCredentials()
	} else {
		config := &tls.Config{
			MinVersion: tls.VersionTLS13,
		}

		// Load client certificate if provided (for mTLS)
		if tlsConfig.tlsCert != "" && tlsConfig.tlsKey != "" {
			cert, err := tls.LoadX509KeyPair(tlsConfig.tlsCert, tlsConfig.tlsKey)
			if err != nil {
				return fmt.Errorf("load client certificate: %w", err)
			}
			config.Certificates = []tls.Certificate{cert}
			log.Printf("loaded client certificate for mTLS: %s", tlsConfig.tlsCert)
		}

		// Load server CA certificate if provided
		if tlsConfig.serverCA != "" {
			caCert, err := os.ReadFile(tlsConfig.serverCA)
			if err != nil {
				return fmt.Errorf("read server CA certificate: %w", err)
			}

			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				return fmt.Errorf("failed to parse server CA certificate")
			}

			config.RootCAs = certPool
			log.Printf("loaded server CA certificate: %s", tlsConfig.serverCA)
		}

		creds = credentials.NewTLS(config)
	}

	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("dial control plane: %w", err)
	}
	defer conn.Close()

	client := agentrpc.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	outbound := make(chan *agentrpc.AgentEnvelope, 256)
	sendErr := make(chan error, 1)
	go func() {
		for msg := range outbound {
			if err := stream.Send(msg); err != nil {
				sendErr <- err
				return
			}
		}
		sendErr <- nil
	}()

	rt := newAgentRuntime(outbound)

	select {
	case outbound <- &agentrpc.AgentEnvelope{
		Hello: &agentrpc.AgentHello{
			HostID:       hostID,
			AuthToken:    authToken,
			HostName:     hostName,
			AgentVersion: version.Get().Version,
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
		},
	}:
	case err := <-sendErr:
		return fmt.Errorf("send hello: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}

	recvDone := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					recvDone <- nil
					return
				}
				recvDone <- err
				return
			}
			if err := rt.handleControlMessage(msg); err != nil {
				recvDone <- err
				return
			}
		}
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			rt.close()
			_ = stream.CloseSend()
			return ctx.Err()
		case err := <-recvDone:
			rt.close()
			return err
		case err := <-sendErr:
			rt.close()
			return err
		case updateInfo := <-updateInfoChan:
			rt.setUpdateInfo(updateInfo)
		case <-ticker.C:
			hb := &agentrpc.Heartbeat{
				SentUnix:        time.Now().Unix(),
				RunningSessions: int32(rt.runningSessions()),
			}
			if updateInfo := rt.getUpdateInfo(); updateInfo != nil {
				hb.UpdateAvailable = updateInfo.UpdateAvailable
				hb.CurrentVersion = updateInfo.CurrentVersion
				hb.LatestVersion = updateInfo.LatestVersion
				hb.UpdateURL = updateInfo.UpdateURL
			}
			select {
			case outbound <- &agentrpc.AgentEnvelope{
				Heartbeat: hb,
			}:
			case err := <-sendErr:
				rt.close()
				return fmt.Errorf("send heartbeat: %w", err)
			case <-ctx.Done():
				rt.close()
				_ = stream.CloseSend()
				return ctx.Err()
			}
		}
	}
}

type sessionRuntime struct {
	sessionID    string
	baseRepoPath string
	worktreePath string
	tmuxSession  string
	stopPoll     chan struct{}
	lastOutput   string
}

type agentRuntime struct {
	tmux *tmux.Runner
	wt   *worktree.Manager

	outbound chan<- *agentrpc.AgentEnvelope

	mu              sync.Mutex
	sessions        map[string]*sessionRuntime
	updateInfo      *version.UpdateInfo
}

func newAgentRuntime(outbound chan<- *agentrpc.AgentEnvelope) *agentRuntime {
	return &agentRuntime{
		tmux:     &tmux.Runner{},
		wt:       &worktree.Manager{},
		outbound: outbound,
		sessions: make(map[string]*sessionRuntime),
	}
}

func (a *agentRuntime) close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, s := range a.sessions {
		close(s.stopPoll)
	}
	a.sessions = map[string]*sessionRuntime{}
}

func (a *agentRuntime) runningSessions() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sessions)
}

func (a *agentRuntime) setUpdateInfo(info *version.UpdateInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.updateInfo = info
}

func (a *agentRuntime) getUpdateInfo() *version.UpdateInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.updateInfo
}

func (a *agentRuntime) handleControlMessage(msg *agentrpc.ControlEnvelope) error {
	if msg == nil || msg.Command == nil {
		return nil
	}
	cmd := msg.Command
	var err error
	switch cmd.Command {
	case agentrpc.CommandLaunch:
		err = a.handleLaunch(cmd)
	case agentrpc.CommandStop:
		err = a.handleStop(cmd)
	case agentrpc.CommandInput:
		err = a.handleInput(cmd)
	case agentrpc.CommandUpdateAgent:
		err = a.handleUpdateAgent(cmd)
	default:
		return a.sendCommandResult(cmd, true, "")
	}
	if err != nil {
		if sendErr := a.sendCommandResult(cmd, false, err.Error()); sendErr != nil {
			return fmt.Errorf("%v (and failed to send command result: %v)", err, sendErr)
		}
		return nil
	}
	return a.sendCommandResult(cmd, true, "")
}

func (a *agentRuntime) handleLaunch(cmd *agentrpc.SessionCommand) error {
	args := cmd.Args
	baseRepoPath := args["base_repo_path"]
	worktreePath := args["worktree_path"]
	branchName := args["branch_name"]
	agentType := args["agent_type"]
	prompt := args["prompt"]
	modelOverride := args["model_override"]
	tmuxSession := args["tmux_session"]
	if tmuxSession == "" {
		tmuxSession = tmuxName(cmd.SessionID)
	}
	if baseRepoPath == "" || worktreePath == "" || branchName == "" || prompt == "" || cmd.SessionID == "" {
		return fmt.Errorf("launch command missing required args")
	}

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return a.sendErrorOutput(cmd.SessionID, fmt.Errorf("mkdir worktree root: %w", err))
	}

	if err := a.wt.Create(baseRepoPath, worktreePath, branchName); err != nil {
		return a.sendErrorOutput(cmd.SessionID, fmt.Errorf("create worktree: %w", err))
	}

	// Generate MCP config for AI agent tools
	serverAddr := args["server_addr"]
	apiKey := args["api_key"]
	if serverAddr != "" && apiKey != "" {
		if err := a.generateMCPConfig(cmd.SessionID, agentType, worktreePath, serverAddr, apiKey); err != nil {
			log.Printf("warn: failed to generate MCP config for session %s: %v", cmd.SessionID, err)
			// Non-fatal: continue session launch
		}
	}

	if err := a.tmux.CreateSession(tmuxSession, worktreePath); err != nil {
		_ = a.wt.Remove(baseRepoPath, worktreePath)
		return a.sendErrorOutput(cmd.SessionID, fmt.Errorf("create tmux session: %w", err))
	}

	agentCmd := buildAgentCommand(agentType, prompt, modelOverride)
	if err := a.tmux.SendKeys(tmuxSession, agentCmd); err != nil {
		_ = a.tmux.KillSession(tmuxSession)
		_ = a.wt.Remove(baseRepoPath, worktreePath)
		return a.sendErrorOutput(cmd.SessionID, fmt.Errorf("launch agent command: %w", err))
	}

	sr := &sessionRuntime{
		sessionID:    cmd.SessionID,
		baseRepoPath: baseRepoPath,
		worktreePath: worktreePath,
		tmuxSession:  tmuxSession,
		stopPoll:     make(chan struct{}),
	}
	a.mu.Lock()
	// If relaunching same session id, stop prior poller.
	if prev, ok := a.sessions[cmd.SessionID]; ok {
		close(prev.stopPoll)
	}
	a.sessions[cmd.SessionID] = sr
	a.mu.Unlock()
	go a.pollOutput(sr)

	return a.sendOutput(cmd.SessionID, "session launched via swoops-agent")
}

func (a *agentRuntime) handleStop(cmd *agentrpc.SessionCommand) error {
	a.mu.Lock()
	sr, ok := a.sessions[cmd.SessionID]
	if ok {
		delete(a.sessions, cmd.SessionID)
		close(sr.stopPoll)
	}
	a.mu.Unlock()

	tmuxSession := tmuxName(cmd.SessionID)
	if ok && sr.tmuxSession != "" {
		tmuxSession = sr.tmuxSession
	}
	if err := a.tmux.KillSession(tmuxSession); err != nil {
		// best-effort; session may already be gone
		log.Printf("agent stop: kill tmux %s: %v", tmuxSession, err)
	}
	if ok && sr.baseRepoPath != "" && sr.worktreePath != "" {
		if err := a.wt.Remove(sr.baseRepoPath, sr.worktreePath); err != nil {
			log.Printf("agent stop: remove worktree %s: %v", sr.worktreePath, err)
		}
	}
	return a.sendOutput(cmd.SessionID, "session stopped via swoops-agent")
}

func (a *agentRuntime) handleInput(cmd *agentrpc.SessionCommand) error {
	input := cmd.Args["input"]
	if input == "" {
		return nil
	}

	a.mu.Lock()
	sr, ok := a.sessions[cmd.SessionID]
	a.mu.Unlock()

	tmuxSession := tmuxName(cmd.SessionID)
	if ok && sr.tmuxSession != "" {
		tmuxSession = sr.tmuxSession
	}
	if err := a.tmux.SendKeys(tmuxSession, input); err != nil {
		return a.sendErrorOutput(cmd.SessionID, fmt.Errorf("send input: %w", err))
	}
	return nil
}

func (a *agentRuntime) handleUpdateAgent(cmd *agentrpc.SessionCommand) error {
	log.Printf("Received update command")

	// Get update info from runtime
	updateInfo := a.getUpdateInfo()
	if updateInfo == nil || !updateInfo.UpdateAvailable {
		return fmt.Errorf("no update available")
	}

	log.Printf("Starting agent update from v%s to v%s", updateInfo.CurrentVersion, updateInfo.LatestVersion)

	// Download new binary
	tmpFile, err := os.CreateTemp("", "swoops-agent-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up if we fail

	log.Printf("Downloading update from %s", updateInfo.UpdateURL)
	resp, err := http.Get(updateInfo.UpdateURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write update: %w", err)
	}
	tmpFile.Close()

	// Make the new binary executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}

	// Get path to current executable
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get current executable path: %w", err)
	}

	// Backup current binary
	backupPath := currentPath + ".backup"
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(tmpPath, currentPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	log.Printf("✅ Agent updated successfully to v%s - restart required", updateInfo.LatestVersion)
	log.Printf("   The agent will need to be restarted for the update to take effect")

	return nil
}

func (a *agentRuntime) pollOutput(sr *sessionRuntime) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sr.stopPoll:
			return
		case <-ticker.C:
			out, err := a.tmux.CapturePane(sr.tmuxSession, 500)
			if err != nil {
				continue
			}
			if out == sr.lastOutput {
				continue
			}
			sr.lastOutput = out
			_ = a.sendOutput(sr.sessionID, out)
		}
	}
}

func (a *agentRuntime) sendOutput(sessionID, content string) error {
	select {
	case a.outbound <- &agentrpc.AgentEnvelope{
		Output: &agentrpc.SessionOutput{
			SessionID: sessionID,
			Content:   content,
		},
	}:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out sending output envelope")
	}
}

func (a *agentRuntime) sendErrorOutput(sessionID string, err error) error {
	sendErr := a.sendOutput(sessionID, "Error: "+err.Error())
	if sendErr != nil {
		return fmt.Errorf("%v (and failed to report error output: %v)", err, sendErr)
	}
	return err
}

func (a *agentRuntime) sendCommandResult(cmd *agentrpc.SessionCommand, ok bool, message string) error {
	select {
	case a.outbound <- &agentrpc.AgentEnvelope{
		CommandResult: &agentrpc.CommandResult{
			CommandID: cmd.CommandID,
			SessionID: cmd.SessionID,
			Ok:        ok,
			Message:   message,
		},
	}:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out sending command result")
	}
}

func buildAgentCommand(agentType, prompt, modelOverride string) string {
	switch agentType {
	case "claude":
		cmd := "claude"
		if modelOverride != "" {
			cmd += " --model " + shellQuote(modelOverride)
		}
		cmd += " --print " + shellQuote(prompt)
		return cmd
	default:
		cmd := "codex"
		if modelOverride != "" {
			cmd += " --model " + shellQuote(modelOverride)
		}
		cmd += " " + shellQuote(prompt)
		return cmd
	}
}

func tmuxName(sessionID string) string {
	if len(sessionID) > 12 {
		sessionID = sessionID[:12]
	}
	return "swoop-" + sessionID
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (a *agentRuntime) generateMCPConfig(sessionID, agentType, worktreePath, serverAddr, apiKey string) error {
	var configPath string
	var configContent string

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"swoops-orchestrator": map[string]interface{}{
				"command": "swoops-agent",
				"args": []string{
					"mcp-serve",
					"--session-id", sessionID,
					"--server", serverAddr,
				},
				"env": map[string]string{
					"SWOOPS_API_KEY": apiKey,
				},
			},
		},
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal MCP config: %w", err)
	}
	configContent = string(configJSON)

	switch agentType {
	case "claude":
		configPath = filepath.Join(worktreePath, ".mcp.json")
	case "codex":
		codexDir := filepath.Join(worktreePath, ".codex")
		if err := os.MkdirAll(codexDir, 0o755); err != nil {
			return fmt.Errorf("create .codex directory: %w", err)
		}
		configPath = filepath.Join(codexDir, "mcp.json")
	default:
		return fmt.Errorf("unsupported agent type: %s", agentType)
	}

	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}

	log.Printf("generated MCP config for session %s at %s", sessionID, configPath)
	return nil
}

// downloadCACertificate downloads the CA certificate from the control plane HTTP API
func downloadCACertificate(baseURL, destPath string) error {
	// Make HTTP request to download CA cert
	url := fmt.Sprintf("%s/api/v1/ca-cert", baseURL)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	// Read the certificate data
	certData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Write to destination file
	if err := os.WriteFile(destPath, certData, 0644); err != nil {
		return fmt.Errorf("write certificate file: %w", err)
	}

	return nil
}
