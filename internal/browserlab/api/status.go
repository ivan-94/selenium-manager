package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/ivan-94/selenium-manager/internal/browserlab/catalog"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
	"github.com/ivan-94/selenium-manager/internal/browserlab/grid"
	"github.com/ivan-94/selenium-manager/internal/browserlab/install"
	"github.com/ivan-94/selenium-manager/internal/browserlab/native"
	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

const DefaultListenAddr = "127.0.0.1:49321"

type ServerOptions struct {
	AppSupport       config.AppSupport
	ListenAddr       string
	Version          string
	NativeRuntimes   []native.RuntimeStatus
	CatalogSource    catalog.TagSource
	ImagePuller      install.ImagePuller
	GridRunner       grid.RuntimeRunner
	HostArchitecture string
}

type StatusResponse struct {
	State          string                 `json:"state"`
	Service        string                 `json:"service"`
	Version        string                 `json:"version"`
	CheckedAt      string                 `json:"checkedAt"`
	API            APIStatus              `json:"api"`
	Paths          PathStatus             `json:"paths"`
	Checks         []StatusCheck          `json:"checks"`
	NativeRuntimes []native.RuntimeStatus `json:"nativeRuntimes"`
	Error          *StatusProblem         `json:"error,omitempty"`
}

type APIStatus struct {
	Bind          string `json:"bind"`
	LocalhostOnly bool   `json:"localhostOnly"`
}

type PathStatus struct {
	ConfigDir    string `json:"configDir"`
	LogsDir      string `json:"logsDir"`
	ArtifactsDir string `json:"artifactsDir"`
}

type StatusCheck struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Message string `json:"message"`
}

type StatusProblem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type BrowserSearchResponse struct {
	Browser string                        `json:"browser"`
	Query   string                        `json:"query"`
	Results []catalog.ChromeVersionResult `json:"results"`
}

type BrowserInstallResponse = install.Result

type BrowserListResponse struct {
	Browsers []registry.BrowserRecord `json:"browsers"`
}

type GridStatusResponse struct {
	Status grid.Status `json:"status"`
}

type GridConfigResponse struct {
	Config grid.GeneratedConfig `json:"config"`
}

func NewHandler(options ServerOptions) http.Handler {
	if options.ListenAddr == "" {
		options.ListenAddr = DefaultListenAddr
	}
	if options.Version == "" {
		options.Version = "dev"
	}
	if options.CatalogSource == nil {
		source := catalog.NewDockerHubTagSource()
		options.CatalogSource = source
	}
	if options.HostArchitecture == "" {
		options.HostArchitecture = runtime.GOARCH
	}
	registryStore := registry.NewFileStore(options.AppSupport.Paths.Root)
	installer := install.Installer{
		Store:            registryStore,
		CatalogSource:    options.CatalogSource,
		ImagePuller:      options.ImagePuller,
		HostArchitecture: options.HostArchitecture,
	}
	gridManager := grid.Manager{
		Root:   options.AppSupport.Paths.Root,
		Runner: options.GridRunner,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"state":   "running",
			"service": "browserlabd",
		})
	})
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		writeJSON(w, http.StatusOK, NewStatusResponse(options))
	})
	mux.HandleFunc("GET /v1/browsers/search", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		browser := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("browser")))
		if browser == "" {
			browser = "chrome"
		}
		if browser != "chrome" {
			writeJSON(w, http.StatusBadRequest, StatusProblem{
				Code:    "unsupported_browser",
				Message: "only official Selenium Chrome search is supported in this slice",
			})
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		results, err := catalog.SearchChromeVersions(r.Context(), options.CatalogSource, catalog.SearchOptions{
			Query:            query,
			HostArchitecture: options.HostArchitecture,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, StatusProblem{
				Code:    "catalog_search_failed",
				Message: err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, BrowserSearchResponse{
			Browser: browser,
			Query:   query,
			Results: results,
		})
	})
	mux.HandleFunc("POST /v1/browsers/install", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		var request install.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, StatusProblem{
				Code:    "invalid_json",
				Message: err.Error(),
			})
			return
		}
		result, err := installer.Install(r.Context(), request)
		if err != nil {
			writeInstallError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /v1/browsers/installed", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		browsers, err := registryStore.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, StatusProblem{
				Code:    "registry_read_failed",
				Message: err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, BrowserListResponse{Browsers: browsers})
	})
	mux.HandleFunc("GET /v1/grid/status", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		browsers, err := registryStore.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, StatusProblem{
				Code:    "registry_read_failed",
				Message: err.Error(),
			})
			return
		}
		status, err := gridManager.Status(r.Context(), browsers)
		if err != nil {
			writeGridError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, GridStatusResponse{Status: status})
	})
	mux.HandleFunc("POST /v1/grid/start", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		browsers, err := registryStore.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, StatusProblem{
				Code:    "registry_read_failed",
				Message: err.Error(),
			})
			return
		}
		status, err := gridManager.Start(r.Context(), browsers)
		if err != nil {
			writeGridError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, GridStatusResponse{Status: status})
	})
	mux.HandleFunc("POST /v1/grid/stop", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		browsers, err := registryStore.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, StatusProblem{
				Code:    "registry_read_failed",
				Message: err.Error(),
			})
			return
		}
		status, err := gridManager.Stop(r.Context(), browsers)
		if err != nil {
			writeGridError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, GridStatusResponse{Status: status})
	})
	mux.HandleFunc("GET /v1/grid/config", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, options.AppSupport.Token) {
			writeJSON(w, http.StatusUnauthorized, StatusProblem{
				Code:    "unauthorized",
				Message: "missing or invalid bearer token",
			})
			return
		}
		browsers, err := registryStore.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, StatusProblem{
				Code:    "registry_read_failed",
				Message: err.Error(),
			})
			return
		}
		generated, err := grid.GenerateConfig(browsers, grid.ConfigOptions{})
		if err != nil {
			writeGridError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, GridConfigResponse{Config: generated})
	})
	return mux
}

func NewStatusResponse(options ServerOptions) StatusResponse {
	if options.ListenAddr == "" {
		options.ListenAddr = DefaultListenAddr
	}
	if options.Version == "" {
		options.Version = "dev"
	}
	nativeRuntimes := options.NativeRuntimes
	if nativeRuntimes == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		nativeRuntimes = native.DetectAll(ctx)
	}

	return StatusResponse{
		State:     "running",
		Service:   "browserlabd",
		Version:   options.Version,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		API: APIStatus{
			Bind:          options.ListenAddr,
			LocalhostOnly: IsLocalListenAddr(options.ListenAddr),
		},
		Paths: PathStatus{
			ConfigDir:    options.AppSupport.Paths.ConfigDir,
			LogsDir:      options.AppSupport.Paths.LogsDir,
			ArtifactsDir: options.AppSupport.Paths.ArtifactsDir,
		},
		Checks: []StatusCheck{{
			Name:    "daemon",
			State:   "ok",
			Message: "daemon is reachable",
		}},
		NativeRuntimes: nativeRuntimes,
	}
}

func IsLocalListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || strings.EqualFold(host, "localhost")
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	return r.Header.Get("Authorization") == "Bearer "+token
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeInstallError(w http.ResponseWriter, err error) {
	var problem install.Problem
	if errors.As(err, &problem) {
		status := http.StatusBadRequest
		if problem.Code == "pull_failed" || problem.Code == "catalog_search_failed" {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, StatusProblem{Code: problem.Code, Message: problem.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, StatusProblem{
		Code:    "install_failed",
		Message: err.Error(),
	})
}

func writeGridError(w http.ResponseWriter, err error) {
	var problem grid.Problem
	if errors.As(err, &problem) {
		status := http.StatusBadRequest
		if problem.Code == "grid_start_failed" || problem.Code == "grid_stop_failed" {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, StatusProblem{Code: problem.Code, Message: problem.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, StatusProblem{
		Code:    "grid_failed",
		Message: err.Error(),
	})
}
