package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ivan-94/selenium-manager/internal/browserlab/catalog"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
	"github.com/ivan-94/selenium-manager/internal/browserlab/install"
	"github.com/ivan-94/selenium-manager/internal/browserlab/native"
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
		NativeRuntimes: []native.RuntimeStatus{{
			ID:               "safari",
			DisplayName:      "Safari",
			Kind:             "native",
			Installable:      false,
			Status:           "ready",
			BrowserAvailable: true,
			BrowserVersion:   "17.5",
			DriverAvailable:  true,
			DriverVersion:    "Included with Safari 17.5",
			Message:          "Current macOS Safari is available as a native, non-installable runtime.",
			SetupGuidance:    []string{"Use the current macOS Safari for native checks."},
			OutOfScope:       native.SafariOldVersionsOutOfScope,
		}},
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
	if len(status.NativeRuntimes) != 1 {
		t.Fatalf("native runtimes = %d, want 1", len(status.NativeRuntimes))
	}
	safari := status.NativeRuntimes[0]
	if safari.ID != "safari" || safari.Kind != "native" || safari.Installable {
		t.Fatalf("safari runtime = %+v, want native non-installable safari", safari)
	}
	if safari.OutOfScope != native.SafariOldVersionsOutOfScope {
		t.Fatalf("safari outOfScope = %q, want old Safari out-of-scope copy", safari.OutOfScope)
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

func TestBrowserInstallEndpointPullsExactSeleniumImageAndListsInstalledBrowser(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	puller := &recordingPuller{}

	server := httptest.NewServer(NewHandler(ServerOptions{
		AppSupport:       appSupport,
		ListenAddr:       "127.0.0.1:49321",
		Version:          "test-version",
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "arm64",
		ImagePuller:      puller,
	}))
	t.Cleanup(server.Close)

	body := bytes.NewBufferString(`{
		"browserName": "chrome",
		"browserVersion": "119.0",
		"imageTag": "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
		"repository": "selenium/standalone-chrome",
		"tag": "119.0-chromedriver-119.0-grid-4.43.0-20260404",
		"platforms": ["linux/amd64"]
	}`)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/browsers/install", body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+appSupport.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/browsers/install error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /v1/browsers/install status = %d, want 200", resp.StatusCode)
	}

	var installed BrowserInstallResponse
	if err := json.NewDecoder(resp.Body).Decode(&installed); err != nil {
		t.Fatalf("decode install response: %v", err)
	}
	if installed.Record.ImageTag != "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404" {
		t.Fatalf("installed image = %q, want exact Selenium tag", installed.Record.ImageTag)
	}
	if installed.Record.Platform != "linux/amd64" {
		t.Fatalf("installed platform = %q, want linux/amd64", installed.Record.Platform)
	}
	if installed.Record.Source != "selenium-dockerhub" || !installed.Record.Enabled {
		t.Fatalf("installed record = %+v, want Selenium source and enabled state", installed.Record)
	}
	if len(puller.requests) != 1 || puller.requests[0].ImageTag != installed.Record.ImageTag {
		t.Fatalf("pull requests = %+v, want exact image pull", puller.requests)
	}
	if len(installed.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want Apple Silicon emulation warning", installed.Warnings)
	}

	listReq, err := http.NewRequest(http.MethodGet, server.URL+"/v1/browsers/installed", nil)
	if err != nil {
		t.Fatalf("NewRequest(list) error = %v", err)
	}
	listReq.Header.Set("Authorization", "Bearer "+appSupport.Token)
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatalf("GET /v1/browsers/installed error = %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/browsers/installed status = %d, want 200", listResp.StatusCode)
	}
	var list BrowserListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(list.Browsers) != 1 || list.Browsers[0].ImageTag != installed.Record.ImageTag {
		t.Fatalf("list = %+v, want installed browser", list.Browsers)
	}
}

func TestBrowserInstallEndpointSkipsPullWhenAlreadyInstalled(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	puller := &recordingPuller{}
	server := httptest.NewServer(NewHandler(ServerOptions{
		AppSupport:       appSupport,
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "amd64",
		ImagePuller:      puller,
	}))
	t.Cleanup(server.Close)

	for attempt := 0; attempt < 2; attempt++ {
		body := bytes.NewBufferString(`{"browserName":"chrome","browserVersion":"119.0","imageTag":"selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404","platforms":["linux/amd64"]}`)
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/browsers/install", body)
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+appSupport.Token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST attempt %d error = %v", attempt+1, err)
		}
		var payload BrowserInstallResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			t.Fatalf("decode attempt %d: %v", attempt+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want 200", attempt+1, resp.StatusCode)
		}
		if attempt == 1 && !payload.AlreadyInstalled {
			t.Fatal("second install did not report alreadyInstalled")
		}
	}
	if len(puller.requests) != 1 {
		t.Fatalf("pull count = %d, want one pull for repeated install", len(puller.requests))
	}
}

func TestBrowserInstallEndpointMapsPullFailure(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	server := httptest.NewServer(NewHandler(ServerOptions{
		AppSupport:       appSupport,
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "amd64",
		ImagePuller:      &recordingPuller{err: errors.New("manifest unknown")},
	}))
	t.Cleanup(server.Close)

	body := bytes.NewBufferString(`{"browserName":"chrome","browserVersion":"119.0","imageTag":"selenium/standalone-chrome:missing","platforms":["linux/amd64"]}`)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/browsers/install", body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+appSupport.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/browsers/install error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	var problem StatusProblem
	if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != "pull_failed" || problem.Message == "" {
		t.Fatalf("problem = %+v, want pull_failed", problem)
	}
}

func TestBrowserInstallEndpointReportsPlatformMismatch(t *testing.T) {
	t.Setenv("BROWSERLAB_HOME", t.TempDir())
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		t.Fatalf("EnsureAppSupport() error = %v", err)
	}
	puller := &recordingPuller{}
	server := httptest.NewServer(NewHandler(ServerOptions{
		AppSupport:       appSupport,
		CatalogSource:    catalog.StaticTagSource{Page: readChromeTagsFixture(t)},
		HostArchitecture: "arm64",
		ImagePuller:      puller,
	}))
	t.Cleanup(server.Close)

	body := bytes.NewBufferString(`{"browserName":"chrome","browserVersion":"119.0","imageTag":"selenium/standalone-chrome:ppc","platforms":["linux/ppc64le"]}`)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/browsers/install", body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+appSupport.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/browsers/install error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	var problem StatusProblem
	if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != "platform_mismatch" || !strings.Contains(problem.Message, "linux/arm64") {
		t.Fatalf("problem = %+v, want platform mismatch guidance for arm64", problem)
	}
	if len(puller.requests) != 0 {
		t.Fatalf("pull requests = %+v, want no pull on platform mismatch", puller.requests)
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

type recordingPuller struct {
	requests []install.PullRequest
	err      error
}

func (puller *recordingPuller) PullImage(_ context.Context, request install.PullRequest, report func(install.ProgressEvent)) error {
	puller.requests = append(puller.requests, request)
	if report != nil {
		report(install.ProgressEvent{Stage: "pulling", Message: "pulling " + request.ImageTag})
	}
	return puller.err
}
