package sshexec

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client manages SSH connections to a remote host.
type Client struct {
	Hostname        string
	Port            int
	User            string
	KeyPath         string
	KnownHostsPath string // path to known_hosts file; empty = use ~/.ssh/known_hosts

	mu     sync.Mutex
	client *ssh.Client
}

// NewClient creates a new SSH client configuration.
func NewClient(hostname string, port int, user, keyPath string) *Client {
	return &Client{
		Hostname: hostname,
		Port:     port,
		User:     user,
		KeyPath:  keyPath,
	}
}

func (c *Client) hostKeyCallback() (ssh.HostKeyCallback, error) {
	khPath := c.KnownHostsPath
	if khPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		khPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	// If the known_hosts file doesn't exist, create it so knownhosts.New doesn't fail
	if _, err := os.Stat(khPath); os.IsNotExist(err) {
		dir := filepath.Dir(khPath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create ssh dir: %w", err)
		}
		f, err := os.Create(khPath)
		if err != nil {
			return nil, fmt.Errorf("create known_hosts: %w", err)
		}
		f.Close()
	}

	cb, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("parse known_hosts %s: %w", khPath, err)
	}

	// Wrap callback to trust on first use (TOFU) but reject key mismatches
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		if err == nil {
			return nil
		}

		// Check if this is a "host not in known_hosts" vs "key mismatch"
		var keyErr *knownhosts.KeyError
		if isKeyNotFound(err, &keyErr) {
			// TOFU: host not seen before, add it
			line := knownhosts.Line([]string{hostname}, key)
			f, ferr := os.OpenFile(khPath, os.O_APPEND|os.O_WRONLY, 0644)
			if ferr != nil {
				return fmt.Errorf("host key unknown and could not update known_hosts: %w", err)
			}
			defer f.Close()
			if _, ferr := fmt.Fprintln(f, line); ferr != nil {
				return fmt.Errorf("host key unknown and could not write known_hosts: %w", err)
			}
			return nil
		}

		// Key mismatch — potential MITM, reject
		return err
	}, nil
}

// isKeyNotFound checks if a knownhosts error is a "key not found" (TOFU) scenario
// vs a key mismatch (which should be rejected).
func isKeyNotFound(err error, keyErr **knownhosts.KeyError) bool {
	ke, ok := err.(*knownhosts.KeyError)
	if !ok {
		return false
	}
	*keyErr = ke
	// If Want is empty, the host is not in known_hosts at all (TOFU).
	// If Want has entries, this is a key MISMATCH — reject it.
	return len(ke.Want) == 0
}

func (c *Client) connect() (*ssh.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		_, _, err := c.client.SendRequest("keepalive@swoops", true, nil)
		if err == nil {
			return c.client, nil
		}
		c.client.Close()
		c.client = nil
	}

	keyBytes, err := os.ReadFile(c.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("read SSH key %s: %w", c.KeyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse SSH key: %w", err)
	}

	hostKeyCB, err := c.hostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("host key verification setup: %w", err)
	}

	config := &ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCB,
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", c.Hostname, c.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", addr, err)
	}

	c.client = client
	return client, nil
}

// Exec runs a command on the remote host and returns its combined output.
func (c *Client) Exec(command string) ([]byte, error) {
	client, err := c.connect()
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("SSH new session: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(command)
	if err != nil {
		return out, fmt.Errorf("SSH exec %q: %w", command, err)
	}
	return out, nil
}

// ExecFunc returns a function compatible with tmux.Runner and worktree.Manager.
func (c *Client) ExecFunc() func(name string, args ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		cmd := name
		for _, a := range args {
			cmd += " " + shellQuote(a)
		}
		return c.Exec(cmd)
	}
}

// TestConnection verifies SSH connectivity.
func (c *Client) TestConnection() error {
	out, err := c.Exec("echo ok")
	if err != nil {
		return err
	}
	if len(out) == 0 {
		return fmt.Errorf("empty response from host")
	}
	return nil
}

// Close closes the SSH connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		return err
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + shellEscape(s) + "'"
}

func shellEscape(s string) string {
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += `'\''`
		} else {
			result += string(c)
		}
	}
	return result
}
