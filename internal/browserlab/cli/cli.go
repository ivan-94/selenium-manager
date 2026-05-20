package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ivan-94/selenium-manager/internal/browserlab/api"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
	"github.com/ivan-94/selenium-manager/internal/browserlab/native"
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
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, "usage: browserlab status [--json] [--base-url URL]")
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 64
	}
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

func Main() {
	code := Run(os.Args[1:], os.Stdout, os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
}
