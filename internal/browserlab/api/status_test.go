package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ivan-94/selenium-manager/internal/browserlab/catalog"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
)

func TestStatusEndpointRequiresLocalTokenAndReturnsDaemonStatus(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	server := httptest.NewServer(NewHandler(ServerOptions{
		AppSupport: appSupport,
		ListenAddr: "127.0.0.1:49321",
		Version:    "test-version",
	}))
	t.Cleanup(server.Close)

	unauthorized, err := http.Get(server.URL + "/v1/status")
	if err != nil {
		t.Fatalf("GET /v1/status without token error = %v", err)
	}
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /v1/status without token status = %d, want 401", unauthorized.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/status", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+appSupport.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/status error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/status status = %d, want 200", resp.StatusCode)
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status.State != "running" {
		t.Fatalf("state = %q, want running", status.State)
	}
	if status.API.Bind != "127.0.0.1:49321" {
		t.Fatalf("api bind = %q, want 127.0.0.1:49321", status.API.Bind)
	}
	if !status.API.LocalhostOnly {
		t.Fatal("api localhostOnly = false, want true")
	}
	if status.Paths.ConfigDir != appSupport.Paths.ConfigDir {
		t.Fatalf("config dir = %q, want %q", status.Paths.ConfigDir, appSupport.Paths.ConfigDir)
	}
}

func TestHealthzIsBootstrapSafe(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	server := httptest.NewServer(NewHandler(ServerOptions{
		AppSupport: appSupport,
		ListenAddr: DefaultListenAddr,
		Version:    "test-version",
	}))
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", resp.StatusCode)
	}
}

func TestBrowserSearchEndpointReturnsNormalizedChromeResults(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}

	source := catalog.StaticTagSource{Page: readChromeTagsFixture(t)}
	server := httptest.NewServer(NewHandler(ServerOptions{
		AppSupport:       appSupport,
		ListenAddr:       "127.0.0.1:49321",
		Version:          "test-version",
		CatalogSource:    source,
		HostArchitecture: "arm64",
	}))
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/browsers/search?browser=chrome&q=119", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+appSupport.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/browsers/search error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/browsers/search status = %d, want 200", resp.StatusCode)
	}

	var search BrowserSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&search); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if search.Browser != "chrome" {
		t.Fatalf("Browser = %q, want chrome", search.Browser)
	}
	if len(search.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(search.Results))
	}
	result := search.Results[0]
	if result.BrowserVersion != "119.0" {
		t.Fatalf("BrowserVersion = %q, want 119.0", result.BrowserVersion)
	}
	if result.ImageTag != "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404" {
		t.Fatalf("ImageTag = %q, want full tag", result.ImageTag)
	}
	if len(result.Platforms) != 1 || result.Platforms[0] != "linux/amd64" {
		t.Fatalf("Platforms = %#v, want linux/amd64", result.Platforms)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want Apple Silicon warning", result.Warnings)
	}
}

func TestDefaultListenAddrIsLocalhostOnly(t *testing.T) {
	if !IsLocalListenAddr(DefaultListenAddr) {
		t.Fatalf("DefaultListenAddr %q must be localhost-only", DefaultListenAddr)
	}
	for _, addr := range []string{"0.0.0.0:49321", ":49321", "[::]:49321"} {
		if IsLocalListenAddr(addr) {
			t.Fatalf("IsLocalListenAddr(%q) = true, want false", addr)
		}
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
