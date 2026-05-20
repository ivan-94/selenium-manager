package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
