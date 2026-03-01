package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	systemdUnitName = "swoops-agent.service"
	launchdLabel    = "sh.swoops.agent"
)

type serviceConfig struct {
	BinaryPath string
	ServerAddr string
	HostID     string
	HostName   string
}

func serviceInstallCommand(args []string) error {
	fs := flag.NewFlagSet("service-install", flag.ContinueOnError)
	server := fs.String("server", "127.0.0.1:9090", "control-plane gRPC address")
	hostID := fs.String("host-id", "", "registered host ID from control plane")
	hostName := fs.String("host-name", "", "logical host name override (defaults to OS hostname)")
	binary := fs.String("binary", "", "path to swoops-agent binary (defaults to current executable)")
	scope := fs.String("scope", "user", "service scope for linux: user or system")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *hostID == "" {
		return errors.New("--host-id is required")
	}

	bin := *binary
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable path: %w", err)
		}
		bin = exe
	}
	cfg := serviceConfig{
		BinaryPath: bin,
		ServerAddr: *server,
		HostID:     *hostID,
		HostName:   *hostName,
	}

	switch runtime.GOOS {
	case "linux":
		return installSystemd(cfg, *scope)
	case "darwin":
		return installLaunchd(cfg)
	default:
		return fmt.Errorf("service install not supported on %s", runtime.GOOS)
	}
}

func serviceUninstallCommand(args []string) error {
	fs := flag.NewFlagSet("service-uninstall", flag.ContinueOnError)
	scope := fs.String("scope", "user", "service scope for linux: user or system")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "linux":
		return uninstallSystemd(*scope)
	case "darwin":
		return uninstallLaunchd()
	default:
		return fmt.Errorf("service uninstall not supported on %s", runtime.GOOS)
	}
}

func serviceStatusCommand(args []string) error {
	fs := flag.NewFlagSet("service-status", flag.ContinueOnError)
	scope := fs.String("scope", "user", "service scope for linux: user or system")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "linux":
		return statusSystemd(*scope)
	case "darwin":
		return statusLaunchd()
	default:
		return fmt.Errorf("service status not supported on %s", runtime.GOOS)
	}
}

func installSystemd(cfg serviceConfig, scope string) error {
	system := false
	switch scope {
	case "user":
	case "system":
		system = true
	default:
		return fmt.Errorf("invalid --scope %q, must be user or system", scope)
	}

	unit, err := renderSystemdUnit(cfg)
	if err != nil {
		return err
	}

	unitPath, err := systemdUnitPath(system)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("create systemd directory: %w", err)
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	cmds := systemdReloadEnableCommands(system)
	for _, c := range cmds {
		if out, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(c.Args, " "), err, strings.TrimSpace(string(out)))
		}
	}

	fmt.Printf("Installed and started systemd service at %s\n", unitPath)
	return nil
}

func uninstallSystemd(scope string) error {
	system := false
	switch scope {
	case "user":
	case "system":
		system = true
	default:
		return fmt.Errorf("invalid --scope %q, must be user or system", scope)
	}

	stopDisable := systemdStopDisableCommands(system)
	for _, c := range stopDisable {
		_ = c.Run() // best effort in uninstall
	}

	unitPath, err := systemdUnitPath(system)
	if err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove unit file: %w", err)
	}

	for _, c := range systemdReloadCommands(system) {
		_ = c.Run()
	}

	fmt.Printf("Uninstalled systemd service from %s\n", unitPath)
	return nil
}

func statusSystemd(scope string) error {
	system := false
	switch scope {
	case "user":
	case "system":
		system = true
	default:
		return fmt.Errorf("invalid --scope %q, must be user or system", scope)
	}

	cmd := systemdStatusCommand(system)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s\n", strings.TrimSpace(string(out)))
		return err
	}
	fmt.Printf("%s\n", strings.TrimSpace(string(out)))
	return nil
}

func installLaunchd(cfg serviceConfig) error {
	plist, err := renderLaunchdPlist(cfg)
	if err != nil {
		return err
	}

	path, err := launchdPlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	unload := exec.Command("launchctl", "bootout", launchdTarget(), path)
	_ = unload.Run() // ignore: may not exist yet
	load := exec.Command("launchctl", "bootstrap", launchdTarget(), path)
	if out, err := load.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, strings.TrimSpace(string(out)))
	}
	enable := exec.Command("launchctl", "enable", launchdServiceName())
	_ = enable.Run()
	kick := exec.Command("launchctl", "kickstart", "-k", launchdServiceName())
	_ = kick.Run()

	fmt.Printf("Installed and started launchd agent at %s\n", path)
	return nil
}

func uninstallLaunchd() error {
	path, err := launchdPlistPath()
	if err != nil {
		return err
	}

	_ = exec.Command("launchctl", "bootout", launchdTarget(), path).Run()
	_ = exec.Command("launchctl", "disable", launchdServiceName()).Run()

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove plist: %w", err)
	}

	fmt.Printf("Uninstalled launchd agent from %s\n", path)
	return nil
}

func statusLaunchd() error {
	cmd := exec.Command("launchctl", "print", launchdServiceName())
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to list for older launchctl behavior.
		listOut, listErr := exec.Command("launchctl", "list", launchdLabel).CombinedOutput()
		if listErr != nil {
			fmt.Printf("%s\n", strings.TrimSpace(string(out)))
			fmt.Printf("%s\n", strings.TrimSpace(string(listOut)))
			return err
		}
		fmt.Printf("%s\n", strings.TrimSpace(string(listOut)))
		return nil
	}
	fmt.Printf("%s\n", strings.TrimSpace(string(out)))
	return nil
}

func renderSystemdUnit(cfg serviceConfig) (string, error) {
	if cfg.BinaryPath == "" || cfg.ServerAddr == "" || cfg.HostID == "" {
		return "", errors.New("binary path, server addr, and host id are required")
	}

	var args []string
	args = append(args, shellQuote(cfg.BinaryPath), "run", "--server", shellQuote(cfg.ServerAddr), "--host-id", shellQuote(cfg.HostID))
	if cfg.HostName != "" {
		args = append(args, "--host-name", shellQuote(cfg.HostName))
	}
	execLine := strings.Join(args, " ")

	var b bytes.Buffer
	b.WriteString("[Unit]\n")
	b.WriteString("Description=Swoops Agent\n")
	b.WriteString("After=network-online.target\n")
	b.WriteString("Wants=network-online.target\n\n")
	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	b.WriteString("ExecStart=" + execLine + "\n")
	b.WriteString("Restart=always\n")
	b.WriteString("RestartSec=2\n")
	b.WriteString("KillMode=process\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=default.target\n")
	return b.String(), nil
}

func renderLaunchdPlist(cfg serviceConfig) (string, error) {
	if cfg.BinaryPath == "" || cfg.ServerAddr == "" || cfg.HostID == "" {
		return "", errors.New("binary path, server addr, and host id are required")
	}
	args := []string{
		cfg.BinaryPath,
		"run",
		"--server", cfg.ServerAddr,
		"--host-id", cfg.HostID,
	}
	if cfg.HostName != "" {
		args = append(args, "--host-name", cfg.HostName)
	}

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString(`<dict>` + "\n")
	b.WriteString(`  <key>Label</key><string>` + xmlEscape(launchdLabel) + `</string>` + "\n")
	b.WriteString(`  <key>ProgramArguments</key>` + "\n")
	b.WriteString(`  <array>` + "\n")
	for _, a := range args {
		b.WriteString(`    <string>` + xmlEscape(a) + `</string>` + "\n")
	}
	b.WriteString(`  </array>` + "\n")
	b.WriteString(`  <key>RunAtLoad</key><true/>` + "\n")
	b.WriteString(`  <key>KeepAlive</key><true/>` + "\n")
	b.WriteString(`  <key>StandardOutPath</key><string>` + xmlEscape(filepath.Join(os.TempDir(), "swoops-agent.log")) + `</string>` + "\n")
	b.WriteString(`  <key>StandardErrorPath</key><string>` + xmlEscape(filepath.Join(os.TempDir(), "swoops-agent.err.log")) + `</string>` + "\n")
	b.WriteString(`</dict>` + "\n")
	b.WriteString(`</plist>` + "\n")
	return b.String(), nil
}

func systemdUnitPath(system bool) (string, error) {
	if system {
		return "/etc/systemd/system/" + systemdUnitName, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName), nil
}

func systemdReloadEnableCommands(system bool) []*exec.Cmd {
	if system {
		return []*exec.Cmd{
			exec.Command("systemctl", "daemon-reload"),
			exec.Command("systemctl", "enable", "--now", systemdUnitName),
		}
	}
	return []*exec.Cmd{
		exec.Command("systemctl", "--user", "daemon-reload"),
		exec.Command("systemctl", "--user", "enable", "--now", systemdUnitName),
	}
}

func systemdStopDisableCommands(system bool) []*exec.Cmd {
	if system {
		return []*exec.Cmd{
			exec.Command("systemctl", "disable", "--now", systemdUnitName),
		}
	}
	return []*exec.Cmd{
		exec.Command("systemctl", "--user", "disable", "--now", systemdUnitName),
	}
}

func systemdReloadCommands(system bool) []*exec.Cmd {
	if system {
		return []*exec.Cmd{exec.Command("systemctl", "daemon-reload")}
	}
	return []*exec.Cmd{exec.Command("systemctl", "--user", "daemon-reload")}
}

func systemdStatusCommand(system bool) *exec.Cmd {
	if system {
		return exec.Command("systemctl", "status", systemdUnitName, "--no-pager")
	}
	return exec.Command("systemctl", "--user", "status", systemdUnitName, "--no-pager")
}

func launchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func launchdTarget() string {
	u, err := user.Current()
	if err != nil {
		return "gui/" + fmt.Sprintf("%d", os.Getuid())
	}
	return "gui/" + u.Uid
}

func launchdServiceName() string {
	return launchdTarget() + "/" + launchdLabel
}

func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}
