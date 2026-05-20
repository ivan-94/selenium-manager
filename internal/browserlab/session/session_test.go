package session

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ivan-94/selenium-manager/internal/browserlab/grid"
	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

func TestBuildCapabilitiesTargetsInstalledBrowserVersion(t *testing.T) {
	record := registry.BrowserRecord{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
		Platform: "linux/amd64",
		Enabled:  true,
	}

	capabilities := BuildCapabilities(record)

	want := map[string]any{
		"browserName":            "chrome",
		"browserVersion":         "119.0",
		"platformName":           "linux",
		"se:name":                "BrowserLab manual chrome 119.0",
		"browserlab:sessionType": "manual",
	}
	if !reflect.DeepEqual(capabilities, want) {
		t.Fatalf("BuildCapabilities() = %#v, want %#v", capabilities, want)
	}
}

func TestManagerCreateHeldSessionDefaultsURLAndDoesNotQuit(t *testing.T) {
	store := registry.NewFileStore(t.TempDir())
	_, err := store.Save(registry.BrowserRecord{
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
	webdriver := &recordingWebDriver{
		sessionID: "session-123",
		capabilities: map[string]any{
			"se:vnc":             "ws://172.17.0.2:4444/session/session-123/se/vnc",
			"se:vncLocalAddress": "ws://172.17.0.2:7900",
		},
		currentURL: "about:blank",
		title:      "",
	}
	manager := Manager{
		Store:     store,
		Grid:      grid.Manager{Root: t.TempDir(), Runner: &runningGridRunner{}},
		WebDriver: webdriver,
		Now:       func() time.Time { return time.Date(2026, 5, 20, 10, 30, 0, 0, time.UTC) },
	}

	result, err := manager.CreateHeldSession(context.Background(), CreateRequest{
		BrowserName:    "chrome",
		BrowserVersion: "119.0",
	})
	if err != nil {
		t.Fatalf("CreateHeldSession() error = %v", err)
	}

	if webdriver.newSessionEndpoint != "http://127.0.0.1:4444/wd/hub" {
		t.Fatalf("new session endpoint = %q, want grid WebDriver endpoint", webdriver.newSessionEndpoint)
	}
	if webdriver.navigatedURL != "about:blank" {
		t.Fatalf("navigated URL = %q, want about:blank", webdriver.navigatedURL)
	}
	if webdriver.quitCalled {
		t.Fatal("held manual session was quit by manager")
	}
	if result.SessionID != "session-123" || result.BrowserVersion != "119.0" {
		t.Fatalf("result = %+v, want held chrome 119.0 session", result)
	}
	if result.CurrentURL != "about:blank" {
		t.Fatalf("current URL = %q, want about:blank", result.CurrentURL)
	}
	if result.GridURL != "http://127.0.0.1:4444" || result.WebDriverEndpoint != "http://127.0.0.1:4444/wd/hub" {
		t.Fatalf("grid fields = %q / %q", result.GridURL, result.WebDriverEndpoint)
	}
	if result.NoVNC.URL != "http://127.0.0.1:4444/ui/#/sessions/session-123" {
		t.Fatalf("noVNC URL = %q, want grid session URL", result.NoVNC.URL)
	}
	if result.NoVNC.VNCWebSocketURL != "ws://127.0.0.1:4444/session/session-123/se/vnc" {
		t.Fatalf("VNC websocket URL = %q, want grid-routed websocket", result.NoVNC.VNCWebSocketURL)
	}
	if result.StartedAt != "2026-05-20T10:30:00Z" {
		t.Fatalf("startedAt = %q", result.StartedAt)
	}
}

func TestResolveNoVNCUsesGridURLForRoutedVNCWebSocket(t *testing.T) {
	resolution := ResolveNoVNC("http://127.0.0.1:4444", "session-123", map[string]any{
		"se:vnc":             "ws://172.17.0.2:4444/session/session-123/se/vnc",
		"se:vncLocalAddress": "ws://172.17.0.2:7900",
	})

	if resolution.URL != "http://127.0.0.1:4444/ui/#/sessions/session-123" {
		t.Fatalf("URL = %q", resolution.URL)
	}
	if resolution.VNCWebSocketURL != "ws://127.0.0.1:4444/session/session-123/se/vnc" {
		t.Fatalf("VNCWebSocketURL = %q", resolution.VNCWebSocketURL)
	}
	if resolution.VNCLocalAddress != "ws://172.17.0.2:7900" {
		t.Fatalf("VNCLocalAddress = %q", resolution.VNCLocalAddress)
	}
}

type recordingWebDriver struct {
	sessionID          string
	capabilities       map[string]any
	currentURL         string
	title              string
	newSessionEndpoint string
	navigatedURL       string
	quitCalled         bool
}

func (driver *recordingWebDriver) NewSession(_ context.Context, request NewSessionRequest) (NewSessionResult, error) {
	driver.newSessionEndpoint = request.WebDriverEndpoint
	return NewSessionResult{SessionID: driver.sessionID, Capabilities: driver.capabilities}, nil
}

func (driver *recordingWebDriver) Navigate(_ context.Context, _ string, _ string, url string) error {
	driver.navigatedURL = url
	return nil
}

func (driver *recordingWebDriver) CurrentURL(_ context.Context, _ string, _ string) (string, error) {
	return driver.currentURL, nil
}

func (driver *recordingWebDriver) Title(_ context.Context, _ string, _ string) (string, error) {
	return driver.title, nil
}

func (driver *recordingWebDriver) Quit(_ context.Context, _ string, _ string) error {
	driver.quitCalled = true
	return nil
}

type runningGridRunner struct{}

func (runningGridRunner) Start(_ context.Context, request grid.StartRequest) (grid.RuntimeState, error) {
	return grid.RuntimeState{State: "running", ContainerName: request.ContainerName, ContainerID: "grid-123"}, nil
}

func (runningGridRunner) Status(_ context.Context, containerName string) (grid.RuntimeState, error) {
	return grid.RuntimeState{State: "running", ContainerName: containerName, ContainerID: "grid-123"}, nil
}

func (runningGridRunner) Stop(_ context.Context, containerName string) (grid.RuntimeState, error) {
	return grid.RuntimeState{State: "stopped", ContainerName: containerName}, nil
}
