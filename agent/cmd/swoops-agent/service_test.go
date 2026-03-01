package main

import (
	"strings"
	"testing"
)

func TestRenderSystemdUnit(t *testing.T) {
	cfg := serviceConfig{
		BinaryPath: "/opt/swoops/swoops-agent",
		ServerAddr: "10.0.0.1:9090",
		HostID:     "host-123",
		HostName:   "prod-a",
	}
	unit, err := renderSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("renderSystemdUnit: %v", err)
	}
	if !strings.Contains(unit, "ExecStart='/opt/swoops/swoops-agent' run --server '10.0.0.1:9090' --host-id 'host-123' --host-name 'prod-a'") {
		t.Fatalf("unexpected ExecStart line:\n%s", unit)
	}
}

func TestRenderLaunchdPlist(t *testing.T) {
	cfg := serviceConfig{
		BinaryPath: "/opt/swoops/swoops-agent",
		ServerAddr: "10.0.0.1:9090",
		HostID:     "host-123",
	}
	plist, err := renderLaunchdPlist(cfg)
	if err != nil {
		t.Fatalf("renderLaunchdPlist: %v", err)
	}
	if !strings.Contains(plist, "<string>/opt/swoops/swoops-agent</string>") {
		t.Fatalf("plist missing binary path:\n%s", plist)
	}
	if !strings.Contains(plist, "<string>--host-id</string>") || !strings.Contains(plist, "<string>host-123</string>") {
		t.Fatalf("plist missing host args:\n%s", plist)
	}
}

func TestXMLEscape(t *testing.T) {
	in := `<tag attr="x&y">'z'</tag>`
	got := xmlEscape(in)
	want := "&lt;tag attr=&quot;x&amp;y&quot;&gt;&apos;z&apos;&lt;/tag&gt;"
	if got != want {
		t.Fatalf("xmlEscape=%q want %q", got, want)
	}
}
