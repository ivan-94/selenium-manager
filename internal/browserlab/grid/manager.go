package grid

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

const generatedConfigFile = "selenium-docker.toml"

type RuntimeRunner interface {
	Start(ctx context.Context, request StartRequest) (RuntimeState, error)
	Status(ctx context.Context, containerName string) (RuntimeState, error)
	Stop(ctx context.Context, containerName string) (RuntimeState, error)
}

type StartRequest struct {
	ContainerName string
	GridImage     string
	GridPort      int
	ConfigPath    string
	AssetsDir     string
	Config        GeneratedConfig
}

type RuntimeState struct {
	State         string `json:"state"`
	ContainerName string `json:"containerName"`
	ContainerID   string `json:"containerId,omitempty"`
	Message       string `json:"message,omitempty"`
}

type Status struct {
	State             string          `json:"state"`
	ContainerName     string          `json:"containerName"`
	ContainerID       string          `json:"containerId,omitempty"`
	WebDriverEndpoint string          `json:"webdriverEndpoint"`
	GridURL           string          `json:"gridUrl"`
	ConfigPath        string          `json:"configPath"`
	Browsers          []BrowserConfig `json:"browsers"`
	Error             *Problem        `json:"error,omitempty"`
}

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (problem Problem) Error() string {
	return problem.Message
}

type Manager struct {
	Root    string
	Runner  RuntimeRunner
	Options ConfigOptions
}

func (manager Manager) Start(ctx context.Context, records []registry.BrowserRecord) (Status, error) {
	paths, options := manager.pathsAndOptions()
	config, err := GenerateConfig(records, options)
	if err != nil {
		return Status{}, err
	}
	if len(config.Browsers) == 0 {
		return Status{}, Problem{Code: "no_enabled_browsers", Message: "no enabled installed browsers are available for Selenium Grid"}
	}
	if err := os.MkdirAll(filepath.Dir(paths.configPath), 0o700); err != nil {
		return Status{}, fmt.Errorf("create grid config directory: %w", err)
	}
	if err := os.MkdirAll(paths.assetsDir, 0o700); err != nil {
		return Status{}, fmt.Errorf("create grid assets directory: %w", err)
	}
	if err := os.WriteFile(paths.configPath, []byte(config.TOML), 0o600); err != nil {
		return Status{}, fmt.Errorf("write grid config: %w", err)
	}

	runtime, err := manager.runner().Start(ctx, StartRequest{
		ContainerName: options.ContainerName,
		GridImage:     options.GridImage,
		GridPort:      options.GridPort,
		ConfigPath:    paths.configPath,
		AssetsDir:     paths.assetsDir,
		Config:        config,
	})
	if err != nil {
		return statusFromRuntime(runtime, config, paths.configPath), err
	}
	return statusFromRuntime(runtime, config, paths.configPath), nil
}

func (manager Manager) Status(ctx context.Context, records []registry.BrowserRecord) (Status, error) {
	paths, options := manager.pathsAndOptions()
	config, err := GenerateConfig(records, options)
	if err != nil {
		return Status{}, err
	}
	runtime, err := manager.runner().Status(ctx, options.ContainerName)
	if err != nil {
		var problem Problem
		if errors.As(err, &problem) {
			status := statusFromRuntime(runtime, config, paths.configPath)
			status.Error = &problem
			return status, nil
		}
		return Status{}, err
	}
	return statusFromRuntime(runtime, config, paths.configPath), nil
}

func (manager Manager) Stop(ctx context.Context, records []registry.BrowserRecord) (Status, error) {
	paths, options := manager.pathsAndOptions()
	config, err := GenerateConfig(records, options)
	if err != nil {
		return Status{}, err
	}
	runtime, err := manager.runner().Stop(ctx, options.ContainerName)
	if err != nil {
		return statusFromRuntime(runtime, config, paths.configPath), err
	}
	return statusFromRuntime(runtime, config, paths.configPath), nil
}

func (manager Manager) runner() RuntimeRunner {
	if manager.Runner != nil {
		return manager.Runner
	}
	return DockerRunner{}
}

func (manager Manager) pathsAndOptions() (gridPaths, ConfigOptions) {
	root := manager.Root
	if root == "" {
		root = "."
	}
	options := normalizeConfigOptions(manager.Options)
	assetsDir := options.AssetsDir
	if assetsDir == "" {
		assetsDir = filepath.Join(root, "artifacts", "selenium-grid")
	}
	return gridPaths{
		configPath: filepath.Join(root, "config", generatedConfigFile),
		assetsDir:  assetsDir,
	}, options
}

type gridPaths struct {
	configPath string
	assetsDir  string
}

func statusFromRuntime(runtime RuntimeState, config GeneratedConfig, configPath string) Status {
	state := runtime.State
	if state == "" {
		state = "stopped"
	}
	containerName := runtime.ContainerName
	if containerName == "" {
		containerName = DefaultContainerName
	}
	return Status{
		State:             state,
		ContainerName:     containerName,
		ContainerID:       runtime.ContainerID,
		WebDriverEndpoint: config.WebDriverEndpoint,
		GridURL:           config.GridURL,
		ConfigPath:        configPath,
		Browsers:          config.Browsers,
	}
}

type DockerRunner struct{}

func (DockerRunner) Start(ctx context.Context, request StartRequest) (RuntimeState, error) {
	_, _ = runDocker(ctx, "rm", "-f", request.ContainerName)
	output, err := runDocker(ctx,
		"run",
		"-d",
		"--name", request.ContainerName,
		"-p", fmt.Sprintf("%d:4444", request.GridPort),
		"-v", request.ConfigPath+":/opt/selenium/docker.toml:ro",
		"-v", request.AssetsDir+":/opt/selenium/assets",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		request.GridImage,
	)
	if err != nil {
		return RuntimeState{State: "error", ContainerName: request.ContainerName, Message: output}, Problem{
			Code:    "grid_start_failed",
			Message: dockerMessage(output, err),
		}
	}
	return RuntimeState{
		State:         "running",
		ContainerName: request.ContainerName,
		ContainerID:   strings.TrimSpace(output),
	}, nil
}

func (DockerRunner) Status(ctx context.Context, containerName string) (RuntimeState, error) {
	output, err := runDocker(ctx, "inspect", "-f", "{{.Id}} {{.State.Running}}", containerName)
	if err != nil {
		return RuntimeState{State: "stopped", ContainerName: containerName}, Problem{
			Code:    "grid_not_running",
			Message: dockerMessage(output, err),
		}
	}
	fields := strings.Fields(output)
	state := "stopped"
	if len(fields) >= 2 && fields[1] == "true" {
		state = "running"
	}
	containerID := ""
	if len(fields) > 0 {
		containerID = fields[0]
	}
	return RuntimeState{
		State:         state,
		ContainerName: containerName,
		ContainerID:   containerID,
	}, nil
}

func (DockerRunner) Stop(ctx context.Context, containerName string) (RuntimeState, error) {
	output, err := runDocker(ctx, "rm", "-f", containerName)
	if err != nil {
		return RuntimeState{State: "error", ContainerName: containerName, Message: output}, Problem{
			Code:    "grid_stop_failed",
			Message: dockerMessage(output, err),
		}
	}
	return RuntimeState{State: "stopped", ContainerName: containerName}, nil
}

func runDocker(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func dockerMessage(output string, err error) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return err.Error()
	}
	return output + ": " + err.Error()
}
