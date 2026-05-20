package cli

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ivan-94/selenium-manager/internal/browserlab/api"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
	"github.com/ivan-94/selenium-manager/internal/browserlab/native"
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

func TestStatusHumanReportsNativeSafariRuntime(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport: appSupport,
		ListenAddr: "127.0.0.1:49321",
		Version:    "test-version",
		NativeRuntimes: []native.RuntimeStatus{{
			ID:               "safari",
			DisplayName:      "Safari",
			Kind:             "native",
			Installable:      false,
			Status:           "setup_required",
			BrowserAvailable: true,
			BrowserVersion:   "17.5",
			DriverAvailable:  false,
			Message:          "Safari is installed, but SafariDriver is not available to BrowserLab.",
			SetupGuidance:    []string{"Enable Safari WebDriver support with: safaridriver --enable"},
			OutOfScope:       native.SafariOldVersionsOutOfScope,
		}},
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"status", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(status) exit = %d, want 0; output %s", exitCode, stdout.String())
	}

	got := stdout.String()
	for _, want := range []string{
		"Native runtimes:",
		"Safari: setup_required (native, non-installable)",
		"Safari version: 17.5",
		"Setup: Enable Safari WebDriver support with: safaridriver --enable",
		native.SafariOldVersionsOutOfScope,
	} {
		if !bytes.Contains([]byte(got), []byte(want)) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
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
