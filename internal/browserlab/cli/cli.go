package cli

import (
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
)

const defaultBaseURL = "http://" + api.DefaultListenAddr

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: browserlab status [--json] [--base-url URL]")
		return 64
	}

	switch args[0] {
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, "usage: browserlab status [--json] [--base-url URL]")
		fmt.Fprintln(stdout, "       browserlab search chrome [query] [--json] [--base-url URL]")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 64
	}
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

func writeSearchError(stdout io.Writer, asJSON bool, code string, message string) {
	if asJSON {
		_ = json.NewEncoder(stdout).Encode(api.StatusProblem{Code: code, Message: message})
		return
	}
	fmt.Fprintf(stdout, "BrowserLab search failed: %s\n", message)
}

func Main() {
	code := Run(os.Args[1:], os.Stdout, os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
}
