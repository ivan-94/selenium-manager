package grid

import (
	"context"
	"strings"
	"testing"

	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

func TestGenerateConfigUsesEnabledRegistryRecords(t *testing.T) {
	config, err := GenerateConfig([]registry.BrowserRecord{
		{
			Family:   "chrome",
			Version:  "119.0",
			ImageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
			Platform: "linux/amd64",
			Source:   "selenium-dockerhub",
			Enabled:  true,
		},
		{
			Family:   "chrome",
			Version:  "118.0",
			ImageTag: "selenium/standalone-chrome:118.0-chromedriver-118.0-grid-4.43.0-20260404",
			Platform: "linux/amd64",
			Source:   "selenium-dockerhub",
			Enabled:  false,
		},
	}, ConfigOptions{
		MaxSessions: 2,
	})
	if err != nil {
		t.Fatalf("GenerateConfig() error = %v", err)
	}

	for _, want := range []string{
		"[node]",
		"detect-drivers = false",
		"max-sessions = 2",
		"[docker]",
		`"selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404"`,
		`"{\"browserName\":\"chrome\",\"browserVersion\":\"119.0\",\"platformName\":\"linux\"}"`,
		`url = "http://127.0.0.1:2375"`,
	} {
		if !strings.Contains(config.TOML, want) {
			t.Fatalf("config missing %q:\n%s", want, config.TOML)
		}
	}
	if strings.Contains(config.TOML, "118.0") {
		t.Fatalf("disabled browser was included in config:\n%s", config.TOML)
	}
	if config.WebDriverEndpoint != "http://127.0.0.1:4444/wd/hub" {
		t.Fatalf("WebDriverEndpoint = %q, want stable localhost endpoint", config.WebDriverEndpoint)
	}
	if len(config.Browsers) != 1 {
		t.Fatalf("len(Browsers) = %d, want enabled browser only", len(config.Browsers))
	}
	got := config.Browsers[0]
	if got.BrowserName != "chrome" || got.BrowserVersion != "119.0" || got.ImageTag == "" {
		t.Fatalf("browser = %+v, want chrome 119.0 image mapping", got)
	}
}

func TestManagerStartStatusStopUsesRuntimeRunner(t *testing.T) {
	records := []registry.BrowserRecord{{
		Family:   "chrome",
		Version:  "119.0",
		ImageTag: "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404",
		Platform: "linux/amd64",
		Source:   "selenium-dockerhub",
		Enabled:  true,
	}}
	runner := &recordingRuntimeRunner{}
	manager := Manager{
		Root:   t.TempDir(),
		Runner: runner,
		Options: ConfigOptions{
			MaxSessions: 1,
		},
	}

	started, err := manager.Start(context.Background(), records)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.State != "running" {
		t.Fatalf("started state = %q, want running", started.State)
	}
	if runner.startRequest.ConfigPath == "" || !strings.HasSuffix(runner.startRequest.ConfigPath, "selenium-docker.toml") {
		t.Fatalf("start config path = %q, want generated selenium-docker.toml", runner.startRequest.ConfigPath)
	}
	if !strings.Contains(runner.startRequest.Config.TOML, "browserVersion") {
		t.Fatalf("start config = %q, want generated browser capabilities", runner.startRequest.Config.TOML)
	}

	status, err := manager.Status(context.Background(), records)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.State != "running" || status.ContainerID != "container-123" {
		t.Fatalf("status = %+v, want running container", status)
	}
	if status.WebDriverEndpoint != "http://127.0.0.1:4444/wd/hub" {
		t.Fatalf("status webdriver endpoint = %q", status.WebDriverEndpoint)
	}

	stopped, err := manager.Stop(context.Background(), records)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if stopped.State != "stopped" {
		t.Fatalf("stopped state = %q, want stopped", stopped.State)
	}
	if runner.stoppedName != DefaultContainerName {
		t.Fatalf("stopped name = %q, want default container", runner.stoppedName)
	}
}

type recordingRuntimeRunner struct {
	startRequest StartRequest
	stoppedName  string
	running      bool
}

func (runner *recordingRuntimeRunner) Start(_ context.Context, request StartRequest) (RuntimeState, error) {
	runner.startRequest = request
	runner.running = true
	return RuntimeState{
		State:       "running",
		ContainerID: "container-123",
	}, nil
}

func (runner *recordingRuntimeRunner) Status(_ context.Context, containerName string) (RuntimeState, error) {
	if runner.running {
		return RuntimeState{
			State:         "running",
			ContainerName: containerName,
			ContainerID:   "container-123",
		}, nil
	}
	return RuntimeState{State: "stopped", ContainerName: containerName}, nil
}

func (runner *recordingRuntimeRunner) Stop(_ context.Context, containerName string) (RuntimeState, error) {
	runner.stoppedName = containerName
	runner.running = false
	return RuntimeState{State: "stopped", ContainerName: containerName}, nil
}
