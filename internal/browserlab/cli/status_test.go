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
	"github.com/ivan-94/selenium-manager/internal/browserlab/grid"
	"github.com/ivan-94/selenium-manager/internal/browserlab/install"
	"github.com/ivan-94/selenium-manager/internal/browserlab/native"
	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
	browserlabsession "github.com/ivan-94/selenium-manager/internal/browserlab/session"
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

func TestInstallChromeByVersionJSONReportsStructuredResult(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	puller := &cliRecordingPuller{}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport:       appSupport,
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "amd64",
		ImagePuller:      puller,
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"install", "chrome", "119.0", "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(install chrome --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	var payload api.BrowserInstallResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("install JSON was invalid: %v\n%s", err, stdout.String())
	}
	if payload.Record.ImageTag != "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404" {
		t.Fatalf("imageTag = %q, want exact Selenium tag", payload.Record.ImageTag)
	}
	if payload.Record.Source != "selenium-dockerhub" || !payload.Record.Enabled {
		t.Fatalf("record = %+v, want source and enabled state", payload.Record)
	}
	if len(payload.Progress) == 0 {
		t.Fatal("progress was empty")
	}
	if len(puller.requests) != 1 || puller.requests[0].ImageTag != payload.Record.ImageTag {
		t.Fatalf("pull requests = %+v, want exact pull", puller.requests)
	}
}

func TestInstallChromeByImageTagJSON(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport:       appSupport,
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "amd64",
		ImagePuller:      &cliRecordingPuller{},
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"install", "chrome", "--image-tag", imageTag, "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(install chrome --image-tag --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	var payload api.BrowserInstallResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("install JSON was invalid: %v\n%s", err, stdout.String())
	}
	if payload.Record.Version != "119.0" {
		t.Fatalf("version = %q, want parsed image version", payload.Record.Version)
	}
	if payload.Record.ImageTag != imageTag {
		t.Fatalf("imageTag = %q, want %q", payload.Record.ImageTag, imageTag)
	}
}

func TestBrowsersJSONListsInstalledRegistryEntries(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport:       appSupport,
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "amd64",
		ImagePuller:      &cliRecordingPuller{},
	}))
	t.Cleanup(server.Close)

	var installOut bytes.Buffer
	if code := Run([]string{"install", "chrome", "119.0", "--json", "--base-url", server.URL}, &installOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("install exit = %d, want 0; output %s", code, installOut.String())
	}

	var stdout bytes.Buffer
	exitCode := Run([]string{"browsers", "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(browsers --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	var payload api.BrowserListResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("browsers JSON was invalid: %v\n%s", err, stdout.String())
	}
	if len(payload.Browsers) != 1 || payload.Browsers[0].Family != "chrome" || !payload.Browsers[0].Enabled {
		t.Fatalf("browsers = %+v, want installed chrome", payload.Browsers)
	}
}

func TestBrowsersDisableJSONUpdatesInstalledRegistryEntry(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	store := registry.NewFileStore(appSupport.Paths.Root)
	_, err = store.Save(registry.BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: imageTag,
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport: appSupport,
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"browsers", "disable", "--image-tag", imageTag, "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(browsers disable --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	var payload api.BrowserDisableResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("disable JSON was invalid: %v\n%s", err, stdout.String())
	}
	if payload.Record.Enabled {
		t.Fatalf("record Enabled = true, want false")
	}
}

func TestBrowsersUninstallJSONRequiresConfirmationBeforeDeletingImage(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	store := registry.NewFileStore(appSupport.Paths.Root)
	_, err = store.Save(registry.BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: imageTag,
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	remover := &cliRecordingImageRemover{}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport:   appSupport,
		ImageRemover: remover,
	}))
	t.Cleanup(server.Close)

	var unconfirmedOut bytes.Buffer
	unconfirmedExit := Run([]string{"browsers", "uninstall", "--image-tag", imageTag, "--delete-image", "--json", "--base-url", server.URL}, &unconfirmedOut, &bytes.Buffer{})
	if unconfirmedExit != 2 {
		t.Fatalf("unconfirmed uninstall exit = %d, want 2; output %s", unconfirmedExit, unconfirmedOut.String())
	}
	if !strings.Contains(unconfirmedOut.String(), "image_delete_confirmation_required") {
		t.Fatalf("unconfirmed output = %q, want confirmation problem", unconfirmedOut.String())
	}
	if len(remover.removed) != 0 {
		t.Fatalf("removed images = %#v, want none before confirmation", remover.removed)
	}

	var confirmedOut bytes.Buffer
	confirmedExit := Run([]string{"browsers", "uninstall", "--image-tag", imageTag, "--delete-image", "--confirm-delete-image", "--json", "--base-url", server.URL}, &confirmedOut, &bytes.Buffer{})
	if confirmedExit != 0 {
		t.Fatalf("confirmed uninstall exit = %d, want 0; output %s", confirmedExit, confirmedOut.String())
	}
	var payload api.BrowserUninstallResponse
	if err := json.Unmarshal(confirmedOut.Bytes(), &payload); err != nil {
		t.Fatalf("uninstall JSON was invalid: %v\n%s", err, confirmedOut.String())
	}
	if !payload.ImageDeleted {
		t.Fatal("ImageDeleted = false, want true")
	}
	if len(remover.removed) != 1 || remover.removed[0] != imageTag {
		t.Fatalf("removed images = %#v, want confirmed image deletion", remover.removed)
	}
}

func TestGridStartJSONReportsStableWebDriverEndpoint(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	store := registry.NewFileStore(appSupport.Paths.Root)
	_, err = store.Save(registry.BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport: appSupport,
		GridRunner: &cliGridRunner{},
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"grid", "start", "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(grid start --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	var payload api.GridStatusResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("grid JSON was invalid: %v\n%s", err, stdout.String())
	}
	if payload.Status.State != "running" {
		t.Fatalf("grid state = %q, want running", payload.Status.State)
	}
	if payload.Status.WebDriverEndpoint != "http://127.0.0.1:4444/wd/hub" {
		t.Fatalf("webdriver endpoint = %q, want stable localhost endpoint", payload.Status.WebDriverEndpoint)
	}
	if len(payload.Status.Browsers) != 1 || payload.Status.Browsers[0].BrowserName != "chrome" || payload.Status.Browsers[0].BrowserVersion != "119.0" {
		t.Fatalf("browsers = %+v, want chrome 119.0 capability target", payload.Status.Browsers)
	}
}

func TestGridConfigHumanPrintsReadOnlyConfig(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	store := registry.NewFileStore(appSupport.Paths.Root)
	_, err = store.Save(registry.BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport: appSupport,
		GridRunner: &cliGridRunner{},
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"grid", "config", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(grid config) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	if got := stdout.String(); !strings.Contains(got, "[docker]") || !strings.Contains(got, `"browserVersion\":\"119.0\"`) {
		t.Fatalf("stdout = %q, want generated read-only Dynamic Grid config", got)
	}
}

func TestSessionOpenJSONCreatesHeldManualSessionAndDefaultsURL(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	store := registry.NewFileStore(appSupport.Paths.Root)
	imageTag := "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"
	_, err = store.Save(registry.BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: imageTag,
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	webdriver := &cliWebDriver{
		sessionID: "session-123",
		capabilities: map[string]any{
			"se:vnc": "ws://172.17.0.2:4444/session/session-123/se/vnc",
		},
		currentURL: "about:blank",
	}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport:      appSupport,
		GridRunner:      &cliGridRunner{running: true},
		WebDriverClient: webdriver,
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"session", "open", "chrome", "119.0", "--json", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(session open --json) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	var payload api.ManualSessionResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("session JSON was invalid: %v\n%s", err, stdout.String())
	}
	if payload.SessionID != "session-123" || payload.RequestedURL != "about:blank" || payload.CurrentURL != "about:blank" {
		t.Fatalf("payload = %+v, want held about:blank session", payload)
	}
	if payload.NoVNC.URL != "http://127.0.0.1:4444/ui/#/sessions/session-123" {
		t.Fatalf("noVNC URL = %q", payload.NoVNC.URL)
	}
	if webdriver.navigatedURL != "about:blank" {
		t.Fatalf("navigated URL = %q, want about:blank", webdriver.navigatedURL)
	}
	if webdriver.quitCalled {
		t.Fatal("CLI-created manual session was quit")
	}
}

func TestSessionOpenHumanPrintsSessionDetails(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	store := registry.NewFileStore(appSupport.Paths.Root)
	_, err = store.Save(registry.BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	server := httptest.NewServer(api.NewHandler(api.ServerOptions{
		AppSupport: appSupport,
		GridRunner: &cliGridRunner{running: true},
		WebDriverClient: &cliWebDriver{
			sessionID:  "session-456",
			currentURL: "https://example.test/",
			title:      "Example",
		},
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	exitCode := Run([]string{"session", "open", "chrome", "119.0", "https://example.test/", "--base-url", server.URL}, &stdout, &bytes.Buffer{})
	if exitCode != 0 {
		t.Fatalf("Run(session open) exit = %d, want 0; output %s", exitCode, stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Manual session: session-456",
		"Browser: Chrome 119.0",
		"Requested URL: https://example.test/",
		"Current URL: https://example.test/",
		"Title: Example",
		"Grid: http://127.0.0.1:4444",
		"WebDriver: http://127.0.0.1:4444/wd/hub",
		"noVNC: http://127.0.0.1:4444/ui/#/sessions/session-456",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

type cliRecordingPuller struct {
	requests []install.PullRequest
}

type cliRecordingImageRemover struct {
	removed []string
}

func (remover *cliRecordingImageRemover) RemoveImage(_ context.Context, imageTag string) error {
	remover.removed = append(remover.removed, imageTag)
	return nil
}

type cliGridRunner struct {
	running bool
}

type cliWebDriver struct {
	sessionID          string
	capabilities       map[string]any
	currentURL         string
	title              string
	newSessionEndpoint string
	navigatedURL       string
	quitCalled         bool
}

func (driver *cliWebDriver) NewSession(_ context.Context, request browserlabsession.NewSessionRequest) (browserlabsession.NewSessionResult, error) {
	driver.newSessionEndpoint = request.WebDriverEndpoint
	return browserlabsession.NewSessionResult{SessionID: driver.sessionID, Capabilities: driver.capabilities}, nil
}

func (driver *cliWebDriver) Navigate(_ context.Context, _ string, _ string, url string) error {
	driver.navigatedURL = url
	return nil
}

func (driver *cliWebDriver) CurrentURL(_ context.Context, _ string, _ string) (string, error) {
	return driver.currentURL, nil
}

func (driver *cliWebDriver) Title(_ context.Context, _ string, _ string) (string, error) {
	return driver.title, nil
}

func (driver *cliWebDriver) Quit(_ context.Context, _ string, _ string) error {
	driver.quitCalled = true
	return nil
}

func (runner *cliGridRunner) Start(_ context.Context, request grid.StartRequest) (grid.RuntimeState, error) {
	runner.running = true
	return grid.RuntimeState{State: "running", ContainerName: request.ContainerName, ContainerID: "grid-123"}, nil
}

func (runner *cliGridRunner) Status(_ context.Context, containerName string) (grid.RuntimeState, error) {
	if runner.running {
		return grid.RuntimeState{State: "running", ContainerName: containerName, ContainerID: "grid-123"}, nil
	}
	return grid.RuntimeState{State: "stopped", ContainerName: containerName}, nil
}

func (runner *cliGridRunner) Stop(_ context.Context, containerName string) (grid.RuntimeState, error) {
	runner.running = false
	return grid.RuntimeState{State: "stopped", ContainerName: containerName}, nil
}

func (puller *cliRecordingPuller) PullImage(_ context.Context, request install.PullRequest, report func(install.ProgressEvent)) error {
	puller.requests = append(puller.requests, request)
	if report != nil {
		report(install.ProgressEvent{Stage: "pulling", Message: request.ImageTag})
	}
	return nil
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
