package grid

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

const (
	DefaultContainerName     = "browserlab-selenium-grid"
	DefaultDockerURL         = "http://127.0.0.1:2375"
	DefaultGridImage         = "selenium/standalone-docker:4.43.0-20260404"
	DefaultGridPort          = 4444
	DefaultWebDriverEndpoint = "http://127.0.0.1:4444/wd/hub"
)

type ConfigOptions struct {
	GridPort      int
	DockerURL     string
	MaxSessions   int
	VideoImage    string
	AssetsDir     string
	ContainerName string
	GridImage     string
}

type GeneratedConfig struct {
	TOML              string          `json:"toml"`
	WebDriverEndpoint string          `json:"webdriverEndpoint"`
	GridURL           string          `json:"gridUrl"`
	Browsers          []BrowserConfig `json:"browsers"`
}

type BrowserConfig struct {
	BrowserName    string         `json:"browserName"`
	BrowserVersion string         `json:"browserVersion"`
	ImageTag       string         `json:"imageTag"`
	PlatformName   string         `json:"platformName"`
	Capabilities   map[string]any `json:"capabilities"`
}

func GenerateConfig(records []registry.BrowserRecord, options ConfigOptions) (GeneratedConfig, error) {
	options = normalizeConfigOptions(options)
	browsers := enabledBrowsers(records)
	sort.Slice(browsers, func(i, j int) bool {
		if browsers[i].BrowserName != browsers[j].BrowserName {
			return browsers[i].BrowserName < browsers[j].BrowserName
		}
		if browsers[i].BrowserVersion != browsers[j].BrowserVersion {
			return browsers[i].BrowserVersion > browsers[j].BrowserVersion
		}
		return browsers[i].ImageTag < browsers[j].ImageTag
	})

	var builder strings.Builder
	builder.WriteString("[node]\n")
	builder.WriteString("detect-drivers = false\n")
	builder.WriteString(fmt.Sprintf("max-sessions = %d\n\n", options.MaxSessions))
	builder.WriteString("[docker]\n")
	builder.WriteString("configs = [\n")
	for _, browser := range browsers {
		stereotype, err := json.Marshal(browser.Capabilities)
		if err != nil {
			return GeneratedConfig{}, fmt.Errorf("encode browser stereotype: %w", err)
		}
		builder.WriteString("  ")
		builder.WriteString(strconv.Quote(browser.ImageTag))
		builder.WriteString(", ")
		builder.WriteString(strconv.Quote(string(stereotype)))
		builder.WriteString(",\n")
	}
	builder.WriteString("]\n")
	builder.WriteString("host-config-keys = [\"Dns\", \"DnsOptions\", \"DnsSearch\", \"ExtraHosts\", \"Binds\"]\n")
	builder.WriteString("url = ")
	builder.WriteString(strconv.Quote(options.DockerURL))
	builder.WriteString("\n")
	if options.VideoImage != "" {
		builder.WriteString("video-image = ")
		builder.WriteString(strconv.Quote(options.VideoImage))
		builder.WriteString("\n")
	}

	return GeneratedConfig{
		TOML:              builder.String(),
		WebDriverEndpoint: webDriverEndpoint(options.GridPort),
		GridURL:           gridURL(options.GridPort),
		Browsers:          browsers,
	}, nil
}

func enabledBrowsers(records []registry.BrowserRecord) []BrowserConfig {
	browsers := []BrowserConfig{}
	for _, record := range records {
		family := strings.ToLower(strings.TrimSpace(record.Family))
		version := strings.TrimSpace(record.Version)
		imageTag := strings.TrimSpace(record.ImageTag)
		if !record.Enabled || family == "" || version == "" || imageTag == "" {
			continue
		}
		platformName := platformName(record.Platform)
		capabilities := map[string]any{
			"browserName":    family,
			"browserVersion": version,
			"platformName":   platformName,
		}
		browsers = append(browsers, BrowserConfig{
			BrowserName:    family,
			BrowserVersion: version,
			ImageTag:       imageTag,
			PlatformName:   platformName,
			Capabilities:   capabilities,
		})
	}
	return browsers
}

func normalizeConfigOptions(options ConfigOptions) ConfigOptions {
	if options.GridPort == 0 {
		options.GridPort = DefaultGridPort
	}
	if options.DockerURL == "" {
		options.DockerURL = DefaultDockerURL
	}
	if options.MaxSessions == 0 {
		options.MaxSessions = 1
	}
	if options.ContainerName == "" {
		options.ContainerName = DefaultContainerName
	}
	if options.GridImage == "" {
		options.GridImage = DefaultGridImage
	}
	return options
}

func webDriverEndpoint(port int) string {
	return gridURL(port) + "/wd/hub"
}

func gridURL(port int) string {
	return (&url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", port),
	}).String()
}

func platformName(platform string) string {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return "linux"
	}
	if before, _, found := strings.Cut(platform, "/"); found && before != "" {
		return before
	}
	return platform
}
