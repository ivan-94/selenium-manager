package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ivan-94/selenium-manager/internal/browserlab/api"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
	"github.com/ivan-94/selenium-manager/internal/browserlab/grid"
	"github.com/ivan-94/selenium-manager/internal/browserlab/install"
	"github.com/ivan-94/selenium-manager/internal/browserlab/lifecycle"
	"github.com/ivan-94/selenium-manager/internal/browserlab/native"
)

const defaultBaseURL = "http://" + api.DefaultListenAddr

type Options struct {
	LifecycleRunner lifecycle.Runner
	UID             func() int
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithOptions(args, stdout, stderr, Options{})
}

func RunWithOptions(args []string, stdout io.Writer, stderr io.Writer, options Options) int {
	if len(args) == 0 {
		writeUsage(stderr)
		return 64
	}

	switch args[0] {
	case "daemon":
		return runDaemon(args[1:], stdout, stderr, options)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "install":
		return runInstall(args[1:], stdout, stderr)
	case "browsers":
		return runBrowsers(args[1:], stdout, stderr)
	case "grid":
		return runGrid(args[1:], stdout, stderr)
	case "session":
		return runSession(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		writeUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 64
	}
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: browserlab status [--json] [--base-url URL]")
	fmt.Fprintln(w, "       browserlab search chrome [query] [--json] [--base-url URL]")
	fmt.Fprintln(w, "       browserlab install chrome [version] [--image-tag TAG] [--json] [--base-url URL]")
	fmt.Fprintln(w, "       browserlab browsers [--json] [--base-url URL]")
	fmt.Fprintln(w, "       browserlab grid <start|stop|status|config> [--json] [--base-url URL]")
	fmt.Fprintln(w, "       browserlab session open chrome VERSION [URL] [--json] [--base-url URL]")
	fmt.Fprintln(w, "       browserlab daemon <install|start|stop|restart|status|logs> [--daemon-path PATH]")
}

func runInstall(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, err := parseInstallArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprintln(stderr, "usage: browserlab install chrome [version] [--image-tag TAG] [--json] [--base-url URL]")
		return 64
	}

	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		writeProblem(stdout, parsed.jsonOutput, "config_error", err.Error())
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := installBrowser(ctx, parsed.baseURL, appSupport.Token, install.Request{
		BrowserName:    parsed.browser,
		BrowserVersion: parsed.version,
		ImageTag:       parsed.imageTag,
	})
	if err != nil {
		writeProblem(stdout, parsed.jsonOutput, "install_failed", err.Error())
		return 2
	}

	if parsed.jsonOutput {
		_ = json.NewEncoder(stdout).Encode(result)
		return 0
	}
	writeInstallHuman(stdout, result)
	return 0
}

func runBrowsers(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("browsers", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	baseURL := flags.String("base-url", defaultBaseURL, "daemon base URL")
	if err := flags.Parse(args); err != nil {
		return 64
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))
		return 64
	}

	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		writeProblem(stdout, *jsonOutput, "config_error", err.Error())
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	list, err := fetchInstalledBrowsers(ctx, *baseURL, appSupport.Token)
	if err != nil {
		writeProblem(stdout, *jsonOutput, "daemon_unreachable", err.Error())
		return 2
	}
	if *jsonOutput {
		_ = json.NewEncoder(stdout).Encode(list)
		return 0
	}
	writeBrowsersHuman(stdout, list)
	return 0
}

func runGrid(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: browserlab grid <start|stop|status|config> [--json] [--base-url URL]")
		return 64
	}
	command := args[0]
	flags := flag.NewFlagSet("grid "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	baseURL := flags.String("base-url", defaultBaseURL, "daemon base URL")
	if err := flags.Parse(args[1:]); err != nil {
		return 64
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))
		return 64
	}

	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		writeProblem(stdout, *jsonOutput, "config_error", err.Error())
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch command {
	case "start", "stop":
		status, err := sendGridCommand(ctx, *baseURL, appSupport.Token, command)
		if err != nil {
			writeProblem(stdout, *jsonOutput, "grid_failed", err.Error())
			return 2
		}
		if *jsonOutput {
			_ = json.NewEncoder(stdout).Encode(status)
			return 0
		}
		writeGridStatusHuman(stdout, status.Status)
		return 0
	case "status":
		status, err := fetchGridStatus(ctx, *baseURL, appSupport.Token)
		if err != nil {
			writeProblem(stdout, *jsonOutput, "grid_failed", err.Error())
			return 2
		}
		if *jsonOutput {
			_ = json.NewEncoder(stdout).Encode(status)
			return 0
		}
		writeGridStatusHuman(stdout, status.Status)
		return 0
	case "config":
		config, err := fetchGridConfig(ctx, *baseURL, appSupport.Token)
		if err != nil {
			writeProblem(stdout, *jsonOutput, "grid_failed", err.Error())
			return 2
		}
		if *jsonOutput {
			_ = json.NewEncoder(stdout).Encode(config)
			return 0
		}
		fmt.Fprint(stdout, config.Config.TOML)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown grid command: %s\n", command)
		return 64
	}
}

func runSession(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: browserlab session open chrome VERSION [URL] [--json] [--base-url URL]")
		return 64
	}
	command := args[0]
	switch command {
	case "open":
		return runSessionOpen(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown session command: %s\n", command)
		return 64
	}
}

func runSessionOpen(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, err := parseSessionOpenArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprintln(stderr, "usage: browserlab session open chrome VERSION [URL] [--json] [--base-url URL]")
		return 64
	}

	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		writeProblem(stdout, parsed.jsonOutput, "config_error", err.Error())
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := createManualSession(ctx, parsed.baseURL, appSupport.Token, api.ManualSessionRequest{
		BrowserName:    parsed.browser,
		BrowserVersion: parsed.version,
		URL:            parsed.url,
	})
	if err != nil {
		writeProblem(stdout, parsed.jsonOutput, "session_failed", err.Error())
		return 2
	}
	if parsed.jsonOutput {
		_ = json.NewEncoder(stdout).Encode(result)
		return 0
	}
	writeManualSessionHuman(stdout, result)
	return 0
}

func runSearch(args []string, stdout io.Writer, stderr io.Writer) int {
	parsed, err := parseSearchArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprintln(stderr, "usage: browserlab search chrome [query] [--json] [--base-url URL]")
		return 64
	}

	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		writeSearchError(stdout, parsed.jsonOutput, "config_error", err.Error())
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	search, err := fetchBrowserSearch(ctx, parsed.baseURL, appSupport.Token, parsed.browser, parsed.query)
	if err != nil {
		writeSearchError(stdout, parsed.jsonOutput, "daemon_unreachable", err.Error())
		return 2
	}

	if parsed.jsonOutput {
		_ = json.NewEncoder(stdout).Encode(search)
		return 0
	}
	writeSearchHuman(stdout, search)
	return 0
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "write machine-readable JSON")
	baseURL := flags.String("base-url", defaultBaseURL, "daemon base URL")
	if err := flags.Parse(args); err != nil {
		return 64
	}

	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		writeStopped(stdout, *jsonOutput, "config_error", err.Error())
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := fetchStatus(ctx, *baseURL, appSupport.Token)
	if err != nil {
		writeStopped(stdout, *jsonOutput, "daemon_unreachable", err.Error())
		return 2
	}

	if *jsonOutput {
		_ = json.NewEncoder(stdout).Encode(status)
		return 0
	}
	fmt.Fprintf(stdout, "BrowserLab daemon: %s\n", status.State)
	fmt.Fprintf(stdout, "API: %s\n", status.API.Bind)
	fmt.Fprintf(stdout, "Config: %s\n", status.Paths.ConfigDir)
	writeNativeRuntimes(stdout, status.NativeRuntimes)
	return 0
}

func writeNativeRuntimes(stdout io.Writer, runtimes []native.RuntimeStatus) {
	if len(runtimes) == 0 {
		return
	}
	fmt.Fprintln(stdout, "Native runtimes:")
	for _, runtime := range runtimes {
		installable := "non-installable"
		if runtime.Installable {
			installable = "installable"
		}
		fmt.Fprintf(stdout, "- %s: %s (%s, %s)\n", runtime.DisplayName, runtime.Status, runtime.Kind, installable)
		if runtime.BrowserVersion != "" {
			fmt.Fprintf(stdout, "  Safari version: %s\n", runtime.BrowserVersion)
		}
		if runtime.DriverVersion != "" {
			fmt.Fprintf(stdout, "  SafariDriver: %s\n", runtime.DriverVersion)
		}
		for _, guidance := range runtime.SetupGuidance {
			fmt.Fprintf(stdout, "  Setup: %s\n", guidance)
		}
		if runtime.OutOfScope != "" {
			fmt.Fprintf(stdout, "  Scope: %s\n", runtime.OutOfScope)
		}
	}
}

func runDaemon(args []string, stdout io.Writer, stderr io.Writer, options Options) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: browserlab daemon <install|start|stop|restart|status|logs> [--daemon-path PATH]")
		return 64
	}

	command := args[0]
	flags := flag.NewFlagSet("daemon "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	daemonPathFlag := flags.String("daemon-path", "", "path to browserlabd executable")
	if err := flags.Parse(args[1:]); err != nil {
		return 64
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))
		return 64
	}

	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		fmt.Fprintf(stderr, "daemon %s failed: %v\n", command, err)
		return 1
	}

	daemonPath, err := lifecycle.ResolveDaemonPath(*daemonPathFlag)
	if err != nil && command != "stop" && command != "status" && command != "logs" {
		fmt.Fprintf(stderr, "daemon %s failed: %v\n", command, err)
		return 1
	}

	manager := lifecycle.Manager{
		AppSupport: appSupport,
		DaemonPath: daemonPath,
		Runner:     options.LifecycleRunner,
		UID:        options.UID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result lifecycle.Result
	switch command {
	case "install":
		result, err = manager.Install()
	case "start":
		result, err = manager.Start(ctx)
	case "stop":
		result, err = manager.Stop(ctx)
	case "restart":
		result, err = manager.Restart(ctx)
	case "status":
		result, err = manager.Status(ctx)
	case "logs":
		result, err = manager.Logs()
	default:
		fmt.Fprintf(stderr, "unknown daemon command: %s\n", command)
		return 64
	}
	if err != nil {
		fmt.Fprintf(stderr, "daemon %s failed: %v\n", command, err)
		return 1
	}

	switch command {
	case "status", "logs":
		fmt.Fprint(stdout, result.Output)
	default:
		fmt.Fprintf(stdout, "%s\n", result.Message)
		if result.PlistPath != "" {
			fmt.Fprintf(stdout, "LaunchAgent: %s\n", result.PlistPath)
		}
	}
	return 0
}

func fetchStatus(ctx context.Context, baseURL string, token string) (api.StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/status", nil)
	if err != nil {
		return api.StatusResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.StatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return api.StatusResponse{}, fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
	}

	var status api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return api.StatusResponse{}, err
	}
	return status, nil
}

func fetchBrowserSearch(ctx context.Context, baseURL string, token string, browser string, query string) (api.BrowserSearchResponse, error) {
	endpoint, err := url.Parse(baseURL + "/v1/browsers/search")
	if err != nil {
		return api.BrowserSearchResponse{}, err
	}
	values := endpoint.Query()
	values.Set("browser", browser)
	if query != "" {
		values.Set("q", query)
	}
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return api.BrowserSearchResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.BrowserSearchResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return api.BrowserSearchResponse{}, fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
	}

	var search api.BrowserSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&search); err != nil {
		return api.BrowserSearchResponse{}, err
	}
	return search, nil
}

func installBrowser(ctx context.Context, baseURL string, token string, request install.Request) (api.BrowserInstallResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return api.BrowserInstallResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/browsers/install", bytes.NewReader(data))
	if err != nil {
		return api.BrowserInstallResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.BrowserInstallResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return api.BrowserInstallResponse{}, decodeProblem(resp)
	}

	var result api.BrowserInstallResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return api.BrowserInstallResponse{}, err
	}
	return result, nil
}

func fetchInstalledBrowsers(ctx context.Context, baseURL string, token string) (api.BrowserListResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/browsers/installed", nil)
	if err != nil {
		return api.BrowserListResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.BrowserListResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return api.BrowserListResponse{}, decodeProblem(resp)
	}

	var list api.BrowserListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return api.BrowserListResponse{}, err
	}
	return list, nil
}

func fetchGridStatus(ctx context.Context, baseURL string, token string) (api.GridStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/grid/status", nil)
	if err != nil {
		return api.GridStatusResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.GridStatusResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return api.GridStatusResponse{}, decodeProblem(resp)
	}

	var status api.GridStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return api.GridStatusResponse{}, err
	}
	return status, nil
}

func sendGridCommand(ctx context.Context, baseURL string, token string, command string) (api.GridStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/grid/"+command, nil)
	if err != nil {
		return api.GridStatusResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.GridStatusResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return api.GridStatusResponse{}, decodeProblem(resp)
	}

	var status api.GridStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return api.GridStatusResponse{}, err
	}
	return status, nil
}

func fetchGridConfig(ctx context.Context, baseURL string, token string) (api.GridConfigResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/grid/config", nil)
	if err != nil {
		return api.GridConfigResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.GridConfigResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return api.GridConfigResponse{}, decodeProblem(resp)
	}

	var config api.GridConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return api.GridConfigResponse{}, err
	}
	return config, nil
}

func createManualSession(ctx context.Context, baseURL string, token string, request api.ManualSessionRequest) (api.ManualSessionResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return api.ManualSessionResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/sessions/manual", bytes.NewReader(data))
	if err != nil {
		return api.ManualSessionResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return api.ManualSessionResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return api.ManualSessionResponse{}, decodeProblem(resp)
	}

	var result api.ManualSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return api.ManualSessionResponse{}, err
	}
	return result, nil
}

func decodeProblem(resp *http.Response) error {
	var problem api.StatusProblem
	if err := json.NewDecoder(resp.Body).Decode(&problem); err == nil && problem.Message != "" {
		return fmt.Errorf("%s: %s", problem.Code, problem.Message)
	}
	return fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
}

func writeStopped(stdout io.Writer, asJSON bool, code string, message string) {
	if asJSON {
		_ = json.NewEncoder(stdout).Encode(api.StatusResponse{
			State:   "stopped",
			Service: "browserlabd",
			API: api.APIStatus{
				Bind:          api.DefaultListenAddr,
				LocalhostOnly: true,
			},
			Error: &api.StatusProblem{Code: code, Message: message},
		})
		return
	}
	fmt.Fprintln(stdout, "BrowserLab daemon: stopped")
}

type searchArgs struct {
	browser    string
	query      string
	jsonOutput bool
	baseURL    string
}

type installArgs struct {
	browser    string
	version    string
	imageTag   string
	jsonOutput bool
	baseURL    string
}

type sessionOpenArgs struct {
	browser    string
	version    string
	url        string
	jsonOutput bool
	baseURL    string
}

func parseSessionOpenArgs(args []string) (sessionOpenArgs, error) {
	parsed := sessionOpenArgs{baseURL: defaultBaseURL}
	var positionals []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			parsed.jsonOutput = true
		case "--base-url":
			if i+1 >= len(args) {
				return sessionOpenArgs{}, fmt.Errorf("--base-url requires a URL")
			}
			parsed.baseURL = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return sessionOpenArgs{}, fmt.Errorf("unknown flag: %s", args[i])
			}
			positionals = append(positionals, args[i])
		}
	}
	if len(positionals) < 2 {
		return sessionOpenArgs{}, fmt.Errorf("browser and version are required")
	}
	if strings.ToLower(positionals[0]) != "chrome" {
		return sessionOpenArgs{}, fmt.Errorf("only chrome manual sessions are supported")
	}
	parsed.browser = "chrome"
	parsed.version = positionals[1]
	if len(positionals) > 2 {
		parsed.url = positionals[2]
	}
	if len(positionals) > 3 {
		return sessionOpenArgs{}, fmt.Errorf("too many positional arguments")
	}
	return parsed, nil
}

func parseInstallArgs(args []string) (installArgs, error) {
	parsed := installArgs{baseURL: defaultBaseURL}
	var positionals []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			parsed.jsonOutput = true
		case "--base-url":
			if i+1 >= len(args) {
				return installArgs{}, fmt.Errorf("--base-url requires a URL")
			}
			parsed.baseURL = args[i+1]
			i++
		case "--image-tag":
			if i+1 >= len(args) {
				return installArgs{}, fmt.Errorf("--image-tag requires a tag")
			}
			parsed.imageTag = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return installArgs{}, fmt.Errorf("unknown flag: %s", args[i])
			}
			positionals = append(positionals, args[i])
		}
	}
	if len(positionals) == 0 {
		return installArgs{}, fmt.Errorf("browser is required")
	}
	if strings.ToLower(positionals[0]) != "chrome" {
		return installArgs{}, fmt.Errorf("only chrome install is supported")
	}
	parsed.browser = "chrome"
	if len(positionals) > 1 {
		parsed.version = positionals[1]
	}
	if len(positionals) > 2 {
		return installArgs{}, fmt.Errorf("too many positional arguments")
	}
	if parsed.version == "" && parsed.imageTag == "" {
		return installArgs{}, fmt.Errorf("version or --image-tag is required")
	}
	return parsed, nil
}

func parseSearchArgs(args []string) (searchArgs, error) {
	parsed := searchArgs{baseURL: defaultBaseURL}
	var positionals []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			parsed.jsonOutput = true
		case "--base-url":
			if i+1 >= len(args) {
				return searchArgs{}, fmt.Errorf("--base-url requires a URL")
			}
			parsed.baseURL = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return searchArgs{}, fmt.Errorf("unknown flag: %s", args[i])
			}
			positionals = append(positionals, args[i])
		}
	}
	if len(positionals) == 0 {
		return searchArgs{}, fmt.Errorf("browser is required")
	}
	if strings.ToLower(positionals[0]) != "chrome" {
		return searchArgs{}, fmt.Errorf("only chrome search is supported")
	}
	parsed.browser = "chrome"
	if len(positionals) > 1 {
		parsed.query = positionals[1]
	}
	if len(positionals) > 2 {
		return searchArgs{}, fmt.Errorf("too many positional arguments")
	}
	return parsed, nil
}

func writeSearchHuman(stdout io.Writer, search api.BrowserSearchResponse) {
	if len(search.Results) == 0 {
		fmt.Fprintf(stdout, "No %s versions found", search.Browser)
		if search.Query != "" {
			fmt.Fprintf(stdout, " for %q", search.Query)
		}
		fmt.Fprintln(stdout, ".")
		return
	}
	for _, result := range search.Results {
		fmt.Fprintf(stdout, "Chrome %s", result.BrowserVersion)
		if result.Recommended {
			fmt.Fprint(stdout, " (recommended)")
		}
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "  Image: %s\n", result.ImageTag)
		details := []string{}
		if result.DriverVersion != "" {
			details = append(details, "Driver: "+result.DriverVersion)
		}
		if result.GridVersion != "" {
			details = append(details, "Grid: "+result.GridVersion)
		}
		if result.ReleaseDate != "" {
			details = append(details, "Released: "+result.ReleaseDate)
		}
		if len(details) > 0 {
			fmt.Fprintf(stdout, "  %s\n", strings.Join(details, "  "))
		}
		if len(result.Platforms) > 0 {
			fmt.Fprintf(stdout, "  Platforms: %s\n", strings.Join(result.Platforms, ", "))
		}
		for _, warning := range result.Warnings {
			fmt.Fprintf(stdout, "  Warning: %s\n", warning)
		}
	}
}

func writeInstallHuman(stdout io.Writer, result api.BrowserInstallResponse) {
	record := result.Record
	if result.AlreadyInstalled {
		fmt.Fprintf(stdout, "Chrome %s is already installed.\n", record.Version)
	} else {
		fmt.Fprintf(stdout, "Installed Chrome %s.\n", record.Version)
	}
	fmt.Fprintf(stdout, "Image: %s\n", record.ImageTag)
	fmt.Fprintf(stdout, "Platform: %s\n", record.Platform)
	fmt.Fprintf(stdout, "Source: %s\n", record.Source)
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "Warning: %s\n", warning)
	}
	for _, event := range result.Progress {
		if event.Message != "" {
			fmt.Fprintf(stdout, "Progress: %s\n", event.Message)
		}
	}
}

func writeBrowsersHuman(stdout io.Writer, list api.BrowserListResponse) {
	if len(list.Browsers) == 0 {
		fmt.Fprintln(stdout, "No browsers installed.")
		return
	}
	for _, browser := range list.Browsers {
		state := "disabled"
		if browser.Enabled {
			state = "enabled"
		}
		fmt.Fprintf(stdout, "Chrome %s (%s)\n", browser.Version, state)
		fmt.Fprintf(stdout, "  Image: %s\n", browser.ImageTag)
		fmt.Fprintf(stdout, "  Platform: %s\n", browser.Platform)
		fmt.Fprintf(stdout, "  Source: %s\n", browser.Source)
	}
}

func writeGridStatusHuman(stdout io.Writer, status grid.Status) {
	fmt.Fprintf(stdout, "Selenium Grid: %s\n", status.State)
	fmt.Fprintf(stdout, "WebDriver: %s\n", status.WebDriverEndpoint)
	fmt.Fprintf(stdout, "Grid UI: %s\n", status.GridURL)
	if status.ConfigPath != "" {
		fmt.Fprintf(stdout, "Config: %s\n", status.ConfigPath)
	}
	if len(status.Browsers) == 0 {
		fmt.Fprintln(stdout, "Browsers: none enabled")
		return
	}
	fmt.Fprintln(stdout, "Browsers:")
	for _, browser := range status.Browsers {
		fmt.Fprintf(stdout, "- %s %s -> %s\n", browser.BrowserName, browser.BrowserVersion, browser.ImageTag)
	}
}

func writeManualSessionHuman(stdout io.Writer, result api.ManualSessionResponse) {
	fmt.Fprintf(stdout, "Manual session: %s\n", result.SessionID)
	fmt.Fprintf(stdout, "Browser: %s %s\n", displayBrowserName(result.BrowserName), result.BrowserVersion)
	fmt.Fprintf(stdout, "Requested URL: %s\n", result.RequestedURL)
	if result.CurrentURL != "" {
		fmt.Fprintf(stdout, "Current URL: %s\n", result.CurrentURL)
	}
	if result.Title != "" {
		fmt.Fprintf(stdout, "Title: %s\n", result.Title)
	}
	fmt.Fprintf(stdout, "Grid: %s\n", result.GridURL)
	fmt.Fprintf(stdout, "WebDriver: %s\n", result.WebDriverEndpoint)
	if result.NoVNC.URL != "" {
		fmt.Fprintf(stdout, "noVNC: %s\n", result.NoVNC.URL)
	}
	if result.NoVNC.VNCWebSocketURL != "" {
		fmt.Fprintf(stdout, "VNC websocket: %s\n", result.NoVNC.VNCWebSocketURL)
	}
}

func displayBrowserName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func writeSearchError(stdout io.Writer, asJSON bool, code string, message string) {
	if asJSON {
		writeProblem(stdout, true, code, message)
		return
	}
	fmt.Fprintf(stdout, "BrowserLab search failed: %s\n", message)
}

func writeProblem(stdout io.Writer, asJSON bool, code string, message string) {
	if asJSON {
		_ = json.NewEncoder(stdout).Encode(api.StatusProblem{Code: code, Message: message})
		return
	}
	fmt.Fprintf(stdout, "BrowserLab error: %s\n", message)
}

func Main() {
	code := Run(os.Args[1:], os.Stdout, os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
}
