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
	"github.com/ivan-94/selenium-manager/internal/browserlab/catalog"
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

func TestSearchChromeJSONReportsNormalizedVersions(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport:       appSupport,
		ListenAddr:       "127.0.0.1:49321",
		Version:          "test-version",
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "arm64",
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"search", "chrome", "119", "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(search chrome --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}

	var payload api.BrowserSearchResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("search JSON was invalid: %v\n%s", err, stdout.String())
	}
	if len(payload.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(payload.Results))
	}
	result := payload.Results[0]
	if result.BrowserVersion != "119.0" {
		t.Fatalf("BrowserVersion = %q, want 119.0", result.BrowserVersion)
	}
	if result.ImageTag != "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404" {
		t.Fatalf("ImageTag = %q, want exact Selenium image tag", result.ImageTag)
	}
}

func TestSearchChromeHumanShowsBrowserVersionFirstAndTagDetails(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport:       appSupport,
		ListenAddr:       "127.0.0.1:49321",
		Version:          "test-version",
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "arm64",
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"search", "chrome", "119", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(search chrome) exit = %d, want 0; output %s", exitCode, stdout.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("stdout = %q, want result and details", stdout.String())
	}
	if !strings.HasPrefix(lines[0], "Chrome 119.0") {
		t.Fatalf("first line = %q, want browser version first", lines[0])
	}
	if !strings.Contains(stdout.String(), "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404") {
		t.Fatalf("stdout = %q, want exact Selenium image tag in details", stdout.String())
	}
	if !strings.Contains(stdout.String(), "linux/amd64") {
		t.Fatalf("stdout = %q, want platform details", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Apple Silicon") {
		t.Fatalf("stdout = %q, want Apple Silicon warning", stdout.String())
	}
}

func readChromeTagsFixture(t *testing.T) catalog.DockerHubTagsPage {
	t.Helper()
	fixture, err := os.ReadFile("../catalog/testdata/dockerhub_standalone_chrome_tags.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var page catalog.DockerHubTagsPage
	if err := json.Unmarshal(fixture, &page); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return page
}
