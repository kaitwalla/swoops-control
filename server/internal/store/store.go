package store

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/swoopsh/swoops/pkg/models"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	db *sql.DB
}

// ErrNotFound is returned when a mutation targets a nonexistent row.
var ErrNotFound = fmt.Errorf("not found")

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	migrations := []string{
		"migrations/001_init.sql",
		"migrations/002_add_agent_auth.sql",
		"migrations/003_mcp_bridge.sql",
	}

	for _, migration := range migrations {
		data, err := migrationsFS.ReadFile(migration)
		if err != nil {
			return fmt.Errorf("read %s: %w", migration, err)
		}
		if _, err := s.db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", migration, err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// ---- Host CRUD ----

func (s *Store) CreateHost(h *models.Host) error {
	labelsJSON, _ := json.Marshal(h.Labels)
	pluginsJSON, _ := json.Marshal(h.InstalledPlugins)
	toolsJSON, _ := json.Marshal(h.InstalledTools)

	// Generate auth token if not provided
	if h.AgentAuthToken == "" {
		token, err := models.GenerateAuthToken()
		if err != nil {
			return fmt.Errorf("generate auth token: %w", err)
		}
		h.AgentAuthToken = token
	}

	_, err := s.db.Exec(`
		INSERT INTO hosts (id, name, hostname, ssh_port, ssh_user, ssh_key_path, os, arch, status, agent_version, agent_auth_token, labels_json, max_sessions, base_repo_path, worktree_root, installed_plugins_json, installed_tools_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, h.Name, h.Hostname, h.SSHPort, h.SSHUser, h.SSHKeyPath, h.OS, h.Arch,
		h.Status, h.AgentVersion, h.AgentAuthToken, string(labelsJSON), h.MaxSessions,
		h.BaseRepoPath, h.WorktreeRoot, string(pluginsJSON), string(toolsJSON),
		h.CreatedAt, h.UpdatedAt,
	)
	return err
}

func (s *Store) GetHost(id string) (*models.Host, error) {
	row := s.db.QueryRow(`SELECT id, name, hostname, ssh_port, ssh_user, ssh_key_path, os, arch, status, agent_version, agent_auth_token, labels_json, max_sessions, base_repo_path, worktree_root, installed_plugins_json, installed_tools_json, last_heartbeat, created_at, updated_at FROM hosts WHERE id = ?`, id)
	return scanHost(row)
}

func (s *Store) ListHosts() ([]*models.Host, error) {
	rows, err := s.db.Query(`SELECT id, name, hostname, ssh_port, ssh_user, ssh_key_path, os, arch, status, agent_version, agent_auth_token, labels_json, max_sessions, base_repo_path, worktree_root, installed_plugins_json, installed_tools_json, last_heartbeat, created_at, updated_at FROM hosts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []*models.Host
	for rows.Next() {
		h, err := scanHostRows(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *Store) UpdateHost(h *models.Host) error {
	labelsJSON, _ := json.Marshal(h.Labels)
	pluginsJSON, _ := json.Marshal(h.InstalledPlugins)
	toolsJSON, _ := json.Marshal(h.InstalledTools)
	h.UpdatedAt = time.Now()

	res, err := s.db.Exec(`
		UPDATE hosts SET name=?, hostname=?, ssh_port=?, ssh_user=?, ssh_key_path=?, os=?, arch=?, status=?, agent_version=?, labels_json=?, max_sessions=?, base_repo_path=?, worktree_root=?, installed_plugins_json=?, installed_tools_json=?, last_heartbeat=?, updated_at=?
		WHERE id=?`,
		h.Name, h.Hostname, h.SSHPort, h.SSHUser, h.SSHKeyPath, h.OS, h.Arch,
		h.Status, h.AgentVersion, string(labelsJSON), h.MaxSessions,
		h.BaseRepoPath, h.WorktreeRoot, string(pluginsJSON), string(toolsJSON),
		h.LastHeartbeat, h.UpdatedAt, h.ID,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) DeleteHost(id string) error {
	res, err := s.db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) UpsertHostHeartbeat(id, agentVersion, osName, arch string, at time.Time) error {
	now := time.Now()
	res, err := s.db.Exec(`
		UPDATE hosts
		SET status=?, agent_version=CASE WHEN ? <> '' THEN ? ELSE agent_version END,
		    os=CASE WHEN ? <> '' THEN ? ELSE os END,
		    arch=CASE WHEN ? <> '' THEN ? ELSE arch END,
		    last_heartbeat=?, updated_at=?
		WHERE id=?`,
		models.HostStatusOnline,
		agentVersion, agentVersion,
		osName, osName,
		arch, arch,
		at, now, id,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) TouchHostHeartbeat(id string, at time.Time) error {
	now := time.Now()
	res, err := s.db.Exec(`
		UPDATE hosts
		SET status=?, last_heartbeat=?, updated_at=?
		WHERE id=?`,
		models.HostStatusOnline, at, now, id,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) UpdateHostStatus(id string, status models.HostStatus) error {
	now := time.Now()
	res, err := s.db.Exec(`
		UPDATE hosts
		SET status=?, updated_at=?
		WHERE id=?`,
		status, now, id,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

// ---- Session CRUD ----

func (s *Store) CreateSession(sess *models.Session) error {
	envJSON, _ := json.Marshal(sess.EnvVars)
	mcpJSON, _ := json.Marshal(sess.MCPServers)
	pluginsJSON, _ := json.Marshal(sess.Plugins)
	toolsJSON, _ := json.Marshal(sess.AllowedTools)
	flagsJSON, _ := json.Marshal(sess.ExtraFlags)

	_, err := s.db.Exec(`
		INSERT INTO sessions (id, name, host_id, template_id, agent_type, status, prompt, branch_name, worktree_path, tmux_session, agent_pid, model_override, env_vars_json, mcp_servers_json, plugins_json, allowed_tools_json, extra_flags_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.Name, sess.HostID, sess.TemplateID, sess.AgentType,
		sess.Status, sess.Prompt, sess.BranchName, sess.WorktreePath,
		sess.TmuxSessionName, sess.AgentPID, sess.ModelOverride,
		string(envJSON), string(mcpJSON), string(pluginsJSON),
		string(toolsJSON), string(flagsJSON),
		sess.CreatedAt, sess.UpdatedAt,
	)
	return err
}

func (s *Store) GetSession(id string) (*models.Session, error) {
	row := s.db.QueryRow(`SELECT id, name, host_id, template_id, agent_type, status, prompt, branch_name, worktree_path, tmux_session, agent_pid, model_override, env_vars_json, mcp_servers_json, plugins_json, allowed_tools_json, extra_flags_json, last_output, started_at, stopped_at, created_at, updated_at FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (s *Store) ListSessions(hostID, status string) ([]*models.Session, error) {
	query := `SELECT id, name, host_id, template_id, agent_type, status, prompt, branch_name, worktree_path, tmux_session, agent_pid, model_override, env_vars_json, mcp_servers_json, plugins_json, allowed_tools_json, extra_flags_json, last_output, started_at, stopped_at, created_at, updated_at FROM sessions WHERE 1=1`
	var args []interface{}

	if hostID != "" {
		query += ` AND host_id = ?`
		args = append(args, hostID)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		sess, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

func (s *Store) UpdateSessionStatus(id string, status models.SessionStatus) error {
	now := time.Now()
	res, err := s.db.Exec(`UPDATE sessions SET status=?, updated_at=? WHERE id=?`, status, now, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

// UpdateSession updates all mutable fields of a session.
func (s *Store) UpdateSession(sess *models.Session) error {
	envJSON, _ := json.Marshal(sess.EnvVars)
	mcpJSON, _ := json.Marshal(sess.MCPServers)
	pluginsJSON, _ := json.Marshal(sess.Plugins)
	toolsJSON, _ := json.Marshal(sess.AllowedTools)
	flagsJSON, _ := json.Marshal(sess.ExtraFlags)
	sess.UpdatedAt = time.Now()

	res, err := s.db.Exec(`
		UPDATE sessions SET name=?, status=?, worktree_path=?, tmux_session=?, agent_pid=?,
		model_override=?, env_vars_json=?, mcp_servers_json=?, plugins_json=?,
		allowed_tools_json=?, extra_flags_json=?, last_output=?,
		started_at=?, stopped_at=?, updated_at=?
		WHERE id=?`,
		sess.Name, sess.Status, sess.WorktreePath, sess.TmuxSessionName, sess.AgentPID,
		sess.ModelOverride, string(envJSON), string(mcpJSON), string(pluginsJSON),
		string(toolsJSON), string(flagsJSON), sess.LastOutput,
		sess.StartedAt, sess.StoppedAt, sess.UpdatedAt, sess.ID,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

// UpdateSessionOutput updates only the last_output field.
func (s *Store) UpdateSessionOutput(id, output string) error {
	now := time.Now()
	res, err := s.db.Exec(`UPDATE sessions SET last_output=?, updated_at=? WHERE id=?`, output, now, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) DeleteSession(id string) error {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func checkRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Scan helpers ----

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanHost(row scannable) (*models.Host, error) {
	var h models.Host
	var labelsJSON, pluginsJSON, toolsJSON string
	var lastHeartbeat sql.NullTime

	err := row.Scan(
		&h.ID, &h.Name, &h.Hostname, &h.SSHPort, &h.SSHUser, &h.SSHKeyPath,
		&h.OS, &h.Arch, &h.Status, &h.AgentVersion, &h.AgentAuthToken, &labelsJSON,
		&h.MaxSessions, &h.BaseRepoPath, &h.WorktreeRoot,
		&pluginsJSON, &toolsJSON, &lastHeartbeat,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(labelsJSON), &h.Labels)
	json.Unmarshal([]byte(pluginsJSON), &h.InstalledPlugins)
	json.Unmarshal([]byte(toolsJSON), &h.InstalledTools)
	if lastHeartbeat.Valid {
		h.LastHeartbeat = &lastHeartbeat.Time
	}

	if h.Labels == nil {
		h.Labels = make(map[string]string)
	}

	return &h, nil
}

func scanHostRows(rows *sql.Rows) (*models.Host, error) {
	return scanHost(rows)
}

func scanSession(row scannable) (*models.Session, error) {
	var sess models.Session
	var envJSON, mcpJSON, pluginsJSON, toolsJSON, flagsJSON string
	var startedAt, stoppedAt sql.NullTime

	err := row.Scan(
		&sess.ID, &sess.Name, &sess.HostID, &sess.TemplateID, &sess.AgentType,
		&sess.Status, &sess.Prompt, &sess.BranchName, &sess.WorktreePath,
		&sess.TmuxSessionName, &sess.AgentPID, &sess.ModelOverride,
		&envJSON, &mcpJSON, &pluginsJSON, &toolsJSON, &flagsJSON,
		&sess.LastOutput, &startedAt, &stoppedAt,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(envJSON), &sess.EnvVars)
	json.Unmarshal([]byte(mcpJSON), &sess.MCPServers)
	json.Unmarshal([]byte(pluginsJSON), &sess.Plugins)
	json.Unmarshal([]byte(toolsJSON), &sess.AllowedTools)
	json.Unmarshal([]byte(flagsJSON), &sess.ExtraFlags)

	if startedAt.Valid {
		sess.StartedAt = &startedAt.Time
	}
	if stoppedAt.Valid {
		sess.StoppedAt = &stoppedAt.Time
	}
	if sess.EnvVars == nil {
		sess.EnvVars = make(map[string]string)
	}

	return &sess, nil
}

func scanSessionRows(rows *sql.Rows) (*models.Session, error) {
	return scanSession(rows)
}
