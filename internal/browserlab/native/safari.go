package native

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
)

const SafariOldVersionsOutOfScope = "Old Safari versions are out of scope for the Docker Selenium MVP. BrowserLab only detects the current macOS Safari and SafariDriver."

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type RuntimeStatus struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"displayName"`
	Kind             string   `json:"kind"`
	Installable      bool     `json:"installable"`
	Status           string   `json:"status"`
	BrowserAvailable bool     `json:"browserAvailable"`
	BrowserVersion   string   `json:"browserVersion,omitempty"`
	DriverAvailable  bool     `json:"driverAvailable"`
	DriverVersion    string   `json:"driverVersion,omitempty"`
	Message          string   `json:"message"`
	SetupGuidance    []string `json:"setupGuidance"`
	OutOfScope       string   `json:"outOfScope"`
}

type SafariDetector struct {
	OS     string
	Runner CommandRunner
}

type ExecRunner struct{}

func NewSafariDetector() SafariDetector {
	return SafariDetector{
		OS:     runtime.GOOS,
		Runner: ExecRunner{},
	}
}

func DetectAll(ctx context.Context) []RuntimeStatus {
	return []RuntimeStatus{NewSafariDetector().Detect(ctx)}
}

func (d SafariDetector) Detect(ctx context.Context) RuntimeStatus {
	osName := d.OS
	if osName == "" {
		osName = runtime.GOOS
	}

	status := RuntimeStatus{
		ID:            "safari",
		DisplayName:   "Safari",
		Kind:          "native",
		Installable:   false,
		Status:        "unavailable",
		Message:       "Safari native runtime detection is only available on macOS.",
		SetupGuidance: []string{"Use a macOS host to run native Safari checks."},
		OutOfScope:    SafariOldVersionsOutOfScope,
	}
	if osName != "darwin" {
		return status
	}

	runner := d.Runner
	if runner == nil {
		runner = ExecRunner{}
	}

	safariVersion, safariErr := runner.Run(ctx, "/usr/bin/defaults", "read", "/Applications/Safari.app/Contents/Info", "CFBundleShortVersionString")
	if safariErr != nil || strings.TrimSpace(safariVersion) == "" {
		status.Message = "Safari.app was not detected in /Applications."
		status.SetupGuidance = []string{"Install or restore Safari through macOS Software Update. BrowserLab does not install Safari."}
		return status
	}

	status.BrowserAvailable = true
	status.BrowserVersion = strings.TrimSpace(safariVersion)

	driverVersion, driverErr := runner.Run(ctx, "/usr/bin/safaridriver", "--version")
	if driverErr != nil || strings.TrimSpace(driverVersion) == "" {
		status.Status = "setup_required"
		status.Message = "Safari is installed, but SafariDriver is not available to BrowserLab."
		status.SetupGuidance = []string{
			"Enable Safari WebDriver support with: safaridriver --enable",
			"Use Safari > Settings > Advanced to show Develop tools if manual WebDriver setup is needed.",
			"Use Docker Selenium browsers for old-version testing; BrowserLab does not install old Safari versions.",
		}
		return status
	}

	status.Status = "ready"
	status.DriverAvailable = true
	status.DriverVersion = strings.TrimSpace(driverVersion)
	status.Message = "Current macOS Safari is available as a native, non-installable runtime."
	status.SetupGuidance = []string{
		"Use the current macOS Safari for native checks.",
		"Use Docker Selenium browsers for installable old-version testing.",
	}
	return status
}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}
