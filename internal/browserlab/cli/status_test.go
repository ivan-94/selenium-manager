package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivan-94/selenium-manager/internal/browserlab/api"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
)

func TestStatusJSONReportsDaemonShape(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport: appSupport,
		ListenAddr: "127.0.0.1:49321",
		Version:    "test-version",
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"status", "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(status --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}

	var payload api.StatusResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("status JSON was invalid: %v\n%s", err, stdout.String())
	}
	if payload.State != "running" {
		t.Fatalf("state = %q, want running", payload.State)
	}
	if payload.Service != "browserlabd" {
		t.Fatalf("service = %q, want browserlabd", payload.Service)
	}
	if !payload.API.LocalhostOnly {
		t.Fatal("api.localhostOnly = false, want true")
	}
}

func TestStatusHumanReportsStoppedWhenDaemonIsUnreachable(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())

	var stdout bytes.Buffer
	exitCode := Run([]string{"status", "--base-url", "http://127.0.0.1:1"}, &stdout, &bytes.Buffer{})
	if exitCode != 2 {
		t.Fatalf("Run(status) exit = %d, want 2", exitCode)
	}
	if got := stdout.String(); got != "BrowserLab daemon: stopped\n" {
		t.Fatalf("stdout = %q, want stopped line", got)
	}
}

func TestDaemonInstallWritesUserLaunchAgentPlist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BROWSERLAB_HOME", t.TempDir())

	var stdout bytes.Buffer
	exitCode := RunWithOptions(
		[]string{"daemon", "install", "--daemon-path", "/tmp/browserlabd"},
		&stdout,
		&bytes.Buffer{},
		Options{},
	)
	if exitCode != 0 {
		t.Fatalf("Run(daemon install) exit = %d, want 0; stdout %s", exitCode, stdout.String())
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.browserlab.daemon.plist")
	plist, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("LaunchAgent plist was not written: %v", err)
	}
	if !strings.Contains(string(plist), "<string>/tmp/browserlabd</string>") {
		t.Fatalf("plist does not reference daemon path:\n%s", string(plist))
	}
	if strings.Contains(stdout.String(), "sudo") {
		t.Fatalf("install output mentioned sudo: %s", stdout.String())
	}
}

func TestDaemonStartReportsLifecycleCommandFailures(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())

	var stderr bytes.Buffer
	exitCode := RunWithOptions(
		[]string{"daemon", "start", "--daemon-path", "/tmp/browserlabd"},
		&bytes.Buffer{},
		&stderr,
		Options{
			LifecycleRunner: cliFailingRunner{
				output: "Bootstrap failed: 5: Input/output error",
				err:    errors.New("exit status 5"),
			},
			UID: func() int { return 501 },
		},
	)
	if exitCode != 1 {
		t.Fatalf("Run(daemon start) exit = %d, want 1", exitCode)
	}
	if got := stderr.String(); !strings.Contains(got, "daemon start failed") || !strings.Contains(got, "Bootstrap failed") {
		t.Fatalf("stderr = %q, want lifecycle failure with launchctl output", got)
	}
}

type cliFailingRunner struct {
	output string
	err    error
}

func (r cliFailingRunner) Run(_ context.Context, _ string, _ ...string) (string, error) {
	return r.output, r.err
}
