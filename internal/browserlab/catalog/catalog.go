package catalog

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const StandaloneChromeRepository = "selenium/standalone-chrome"

var chromeTagPattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+){1,3})(?:-chromedriver-([0-9]+(?:\.[0-9]+){1,3}))?(?:-grid-([0-9]+(?:\.[0-9]+){1,3}))?(?:-([0-9]{8}))?$`)

type SearchOptions struct {
	Query            string
	HostArchitecture string
}

type ChromeVersionResult struct {
	BrowserName    string   `json:"browserName"`
	BrowserVersion string   `json:"browserVersion"`
	ImageTag       string   `json:"imageTag"`
	Repository     string   `json:"repository"`
	Tag            string   `json:"tag"`
	DriverVersion  string   `json:"driverVersion,omitempty"`
	GridVersion    string   `json:"gridVersion,omitempty"`
	ReleaseDate    string   `json:"releaseDate,omitempty"`
	Platforms      []string `json:"platforms"`
	Warnings       []string `json:"warnings,omitempty"`
	Recommended    bool     `json:"recommended"`
}

type TagSource interface {
	FetchChromeTags(ctx context.Context, query string) (DockerHubTagsPage, error)
}

type StaticTagSource struct {
	Page DockerHubTagsPage
	Err  error
}

func (source StaticTagSource) FetchChromeTags(context.Context, string) (DockerHubTagsPage, error) {
	return source.Page, source.Err
}

type DockerHubTagsPage struct {
	Next    string         `json:"next"`
	Results []DockerHubTag `json:"results"`
}

type DockerHubTag struct {
	Name        string           `json:"name"`
	LastUpdated string           `json:"last_updated"`
	Images      []DockerHubImage `json:"images"`
}

type DockerHubImage struct {
	Architecture string  `json:"architecture"`
	OS           string  `json:"os"`
	Variant      *string `json:"variant"`
}

func SearchChromeVersions(ctx context.Context, source TagSource, options SearchOptions) ([]ChromeVersionResult, error) {
	if source == nil {
		return nil, fmt.Errorf("tag source is required")
	}

	page, err := source.FetchChromeTags(ctx, strings.TrimSpace(options.Query))
	if err != nil {
		return nil, err
	}

	bestByVersion := map[string]ChromeVersionResult{}
	for _, tag := range page.Results {
		result, ok := normalizeChromeTag(tag, options.HostArchitecture)
		if !ok {
			continue
		}
		if query := strings.TrimSpace(options.Query); query != "" && !strings.HasPrefix(result.BrowserVersion, query) {
			continue
		}
		current, exists := bestByVersion[result.BrowserVersion]
		if !exists || betterChromeTag(result, current) {
			bestByVersion[result.BrowserVersion] = result
		}
	}

	results := make([]ChromeVersionResult, 0, len(bestByVersion))
	for _, result := range bestByVersion {
		result.Recommended = true
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		return compareVersion(results[i].BrowserVersion, results[j].BrowserVersion) > 0
	})
	return results, nil
}

func normalizeChromeTag(tag DockerHubTag, hostArchitecture string) (ChromeVersionResult, bool) {
	matches := chromeTagPattern.FindStringSubmatch(tag.Name)
	if matches == nil {
		return ChromeVersionResult{}, false
	}
	if matches[2] == "" && matches[3] == "" {
		return ChromeVersionResult{}, false
	}

	platforms := normalizedPlatforms(tag.Images)
	result := ChromeVersionResult{
		BrowserName:    "chrome",
		BrowserVersion: matches[1],
		ImageTag:       StandaloneChromeRepository + ":" + tag.Name,
		Repository:     StandaloneChromeRepository,
		Tag:            tag.Name,
		DriverVersion:  matches[2],
		GridVersion:    matches[3],
		ReleaseDate:    matches[4],
		Platforms:      platforms,
	}
	if requiresAMD64Emulation(hostArchitecture, platforms) {
		result.Warnings = append(result.Warnings, "Chrome Selenium images are linux/amd64 only; Apple Silicon will run this image through amd64 emulation.")
	}
	return result, true
}

func normalizedPlatforms(images []DockerHubImage) []string {
	seen := map[string]bool{}
	var platforms []string
	for _, image := range images {
		if image.OS == "" || image.OS == "unknown" || image.Architecture == "" || image.Architecture == "unknown" {
			continue
		}
		platform := image.OS + "/" + image.Architecture
		if image.Variant != nil && *image.Variant != "" {
			platform += "/" + *image.Variant
		}
		if !seen[platform] {
			seen[platform] = true
			platforms = append(platforms, platform)
		}
	}
	sort.Strings(platforms)
	return platforms
}

func requiresAMD64Emulation(hostArchitecture string, platforms []string) bool {
	hostArchitecture = strings.ToLower(strings.TrimSpace(hostArchitecture))
	if hostArchitecture != "arm64" && hostArchitecture != "aarch64" {
		return false
	}
	hasAMD64 := false
	for _, platform := range platforms {
		switch platform {
		case "linux/arm64", "linux/aarch64":
			return false
		case "linux/amd64":
			hasAMD64 = true
		}
	}
	return hasAMD64
}

func betterChromeTag(candidate ChromeVersionResult, current ChromeVersionResult) bool {
	if candidate.GridVersion != "" && current.GridVersion == "" {
		return true
	}
	if candidate.GridVersion == "" && current.GridVersion != "" {
		return false
	}
	if candidate.DriverVersion != "" && current.DriverVersion == "" {
		return true
	}
	if candidate.DriverVersion == "" && current.DriverVersion != "" {
		return false
	}
	if candidate.ReleaseDate != current.ReleaseDate {
		return candidate.ReleaseDate > current.ReleaseDate
	}
	if cmp := compareVersion(candidate.GridVersion, current.GridVersion); cmp != 0 {
		return cmp > 0
	}
	return candidate.Tag > current.Tag
}

func compareVersion(left string, right string) int {
	leftParts := splitVersion(left)
	rightParts := splitVersion(right)
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		var l, r int
		if i < len(leftParts) {
			l = leftParts[i]
		}
		if i < len(rightParts) {
			r = rightParts[i]
		}
		if l > r {
			return 1
		}
		if l < r {
			return -1
		}
	}
	return 0
}

func splitVersion(version string) []int {
	if version == "" {
		return nil
	}
	parts := strings.Split(version, ".")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		var value int
		for _, char := range part {
			if char < '0' || char > '9' {
				break
			}
			value = value*10 + int(char-'0')
		}
		values = append(values, value)
	}
	return values
}
