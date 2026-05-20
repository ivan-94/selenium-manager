package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
	"github.com/ivan-94/selenium-manager/internal/browserlab/native"
)

const DefaultListenAddr = "127.0.0.1:49321"

type ServerOptions struct {
	AppSupport     config.AppSupport
	ListenAddr     string
	Version        string
	NativeRuntimes []native.RuntimeStatus
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

func NewHandler(options ServerOptions) http.Handler {
	if options.ListenAddr == "" {
		options.ListenAddr = DefaultListenAddr
	}
	if options.Version == "" {
		options.Version = "dev"
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
