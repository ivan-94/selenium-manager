package install

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/ivan-94/selenium-manager/internal/browserlab/catalog"
	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

const SourceSeleniumDockerHub = "selenium-dockerhub"

var chromeImagePattern = regexp.MustCompile(`^(?:selenium/standalone-chrome:)?([0-9]+(?:\.[0-9]+){1,3})(?:-chromedriver-[^-]+)?(?:-grid-[^-]+)?(?:-[0-9]{8})?$`)

type PullRequest struct {
	ImageTag string
	Platform string
}

type ProgressEvent struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type ImagePuller interface {
	PullImage(ctx context.Context, request PullRequest, report func(ProgressEvent)) error
}

type DockerPuller struct{}

func (DockerPuller) PullImage(ctx context.Context, request PullRequest, report func(ProgressEvent)) error {
	args := []string{"pull"}
	if request.Platform != "" {
		args = append(args, "--platform", request.Platform)
	}
	args = append(args, request.ImageTag)

	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("docker pull stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("docker pull stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start docker pull: %w", err)
	}

	var wg sync.WaitGroup
	scan := func(scanner *bufio.Scanner) {
		defer wg.Done()
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && report != nil {
				report(ProgressEvent{Stage: "pulling", Message: line})
			}
		}
	}
	wg.Add(2)
	go scan(bufio.NewScanner(stdout))
	go scan(bufio.NewScanner(stderr))
	if err := cmd.Wait(); err != nil {
		wg.Wait()
		return fmt.Errorf("docker pull %s: %w", request.ImageTag, err)
	}
	wg.Wait()
	return nil
}

type Installer struct {
	Store            registry.FileStore
	CatalogSource    catalog.TagSource
	ImagePuller      ImagePuller
	HostArchitecture string
}

type Request struct {
	BrowserName    string   `json:"browserName"`
	BrowserVersion string   `json:"browserVersion"`
	ImageTag       string   `json:"imageTag"`
	Repository     string   `json:"repository,omitempty"`
	Tag            string   `json:"tag,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`
}

type Result struct {
	Record           registry.BrowserRecord `json:"record"`
	AlreadyInstalled bool                   `json:"alreadyInstalled"`
	Progress         []ProgressEvent        `json:"progress"`
	Warnings         []string               `json:"warnings,omitempty"`
}

type Problem struct {
	Code    string
	Message string
}

func (problem Problem) Error() string {
	return problem.Message
}

func (installer Installer) Install(ctx context.Context, request Request) (Result, error) {
	request = normalizeRequest(request)
	if request.BrowserName == "" {
		request.BrowserName = "chrome"
	}
	if request.BrowserName != "chrome" {
		return Result{}, Problem{Code: "unsupported_browser", Message: "only official Selenium Chrome install is supported in this slice"}
	}

	if request.ImageTag == "" {
		resolved, err := installer.resolveByVersion(ctx, request.BrowserVersion)
		if err != nil {
			return Result{}, err
		}
		request = resolved
	}
	if request.BrowserVersion == "" {
		request.BrowserVersion = versionFromImageTag(request.ImageTag)
	}
	if request.BrowserVersion == "" {
		return Result{}, Problem{Code: "invalid_install_request", Message: "browserVersion is required when imageTag cannot be parsed"}
	}

	platform, warnings, err := SelectPlatform(installer.hostArchitecture(), request.Platforms)
	if err != nil {
		return Result{}, err
	}

	record := registry.BrowserRecord{
		Family:   request.BrowserName,
		Version:  request.BrowserVersion,
		ImageTag: request.ImageTag,
		Platform: platform,
		Source:   SourceSeleniumDockerHub,
		Enabled:  true,
	}
	if existing, ok, err := installer.Store.FindByImageTag(request.ImageTag); err != nil {
		return Result{}, err
	} else if ok {
		return Result{
			Record:           existing,
			AlreadyInstalled: true,
			Progress: []ProgressEvent{{
				Stage:   "already_installed",
				Message: "browser image is already installed in Browser Registry",
			}},
			Warnings: warnings,
		}, nil
	}

	puller := installer.ImagePuller
	if puller == nil {
		puller = DockerPuller{}
	}

	progress := []ProgressEvent{{
		Stage:   "pull_queued",
		Message: "pulling " + request.ImageTag,
	}}
	if err := puller.PullImage(ctx, PullRequest{ImageTag: request.ImageTag, Platform: platform}, func(event ProgressEvent) {
		progress = append(progress, event)
	}); err != nil {
		return Result{}, Problem{Code: "pull_failed", Message: err.Error()}
	}

	saved, err := installer.Store.Save(record)
	if err != nil {
		return Result{}, err
	}
	progress = append(progress, ProgressEvent{Stage: "registered", Message: "browser recorded in Browser Registry"})
	return Result{
		Record:           saved.Record,
		AlreadyInstalled: saved.AlreadyInstalled,
		Progress:         progress,
		Warnings:         warnings,
	}, nil
}

func (installer Installer) resolveByVersion(ctx context.Context, version string) (Request, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return Request{}, Problem{Code: "invalid_install_request", Message: "browserVersion or imageTag is required"}
	}
	if installer.CatalogSource == nil {
		return Request{}, Problem{Code: "catalog_unavailable", Message: "catalog source is unavailable"}
	}
	results, err := catalog.SearchChromeVersions(ctx, installer.CatalogSource, catalog.SearchOptions{
		Query:            version,
		HostArchitecture: installer.hostArchitecture(),
	})
	if err != nil {
		return Request{}, Problem{Code: "catalog_search_failed", Message: err.Error()}
	}
	for _, result := range results {
		if result.BrowserVersion == version {
			return Request{
				BrowserName:    result.BrowserName,
				BrowserVersion: result.BrowserVersion,
				ImageTag:       result.ImageTag,
				Repository:     result.Repository,
				Tag:            result.Tag,
				Platforms:      result.Platforms,
			}, nil
		}
	}
	return Request{}, Problem{Code: "browser_not_found", Message: "no official Selenium Chrome image found for version " + version}
}

func SelectPlatform(hostArchitecture string, platforms []string) (string, []string, error) {
	hostArchitecture = normalizeArchitecture(hostArchitecture)
	if len(platforms) == 0 {
		if hostArchitecture == "arm64" {
			return "linux/amd64", []string{"Chrome Selenium image platform was not reported; BrowserLab will pull linux/amd64, which may run through emulation on Apple Silicon."}, nil
		}
		return "linux/" + hostArchitecture, nil, nil
	}

	hostPlatform := "linux/" + hostArchitecture
	if contains(platforms, hostPlatform) {
		return hostPlatform, nil, nil
	}
	if hostArchitecture == "arm64" && contains(platforms, "linux/amd64") {
		return "linux/amd64", []string{"Chrome Selenium images are linux/amd64 only; Apple Silicon will run this image through amd64 emulation."}, nil
	}
	return "", nil, Problem{
		Code:    "platform_mismatch",
		Message: fmt.Sprintf("image platforms %s do not include %s or a supported amd64 emulation fallback", strings.Join(platforms, ", "), hostPlatform),
	}
}

func (installer Installer) hostArchitecture() string {
	if installer.HostArchitecture != "" {
		return installer.HostArchitecture
	}
	return runtime.GOARCH
}

func normalizeRequest(request Request) Request {
	request.BrowserName = strings.ToLower(strings.TrimSpace(request.BrowserName))
	request.BrowserVersion = strings.TrimSpace(request.BrowserVersion)
	request.ImageTag = strings.TrimSpace(request.ImageTag)
	request.Repository = strings.TrimSpace(request.Repository)
	request.Tag = strings.TrimSpace(request.Tag)
	return request
}

func normalizeArchitecture(architecture string) string {
	architecture = strings.ToLower(strings.TrimSpace(architecture))
	switch architecture {
	case "aarch64":
		return "arm64"
	case "":
		return runtime.GOARCH
	default:
		return architecture
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func versionFromImageTag(imageTag string) string {
	matches := chromeImagePattern.FindStringSubmatch(strings.TrimSpace(imageTag))
	if matches == nil {
		return ""
	}
	return matches[1]
}
