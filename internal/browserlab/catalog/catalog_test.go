package catalog

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestSearchChromeVersionsNormalizesDockerHubTags(t *testing.T) {
	fixture, err := os.ReadFile("testdata/dockerhub_standalone_chrome_tags.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var page DockerHubTagsPage
	if err := json.Unmarshal(fixture, &page); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	results, err := SearchChromeVersions(context.Background(), StaticTagSource{Page: page}, SearchOptions{
		Query:            "119",
		HostArchitecture: "arm64",
	})
	if err != nil {
		t.Fatalf("SearchChromeVersions() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	result := results[0]
	if result.BrowserName != "chrome" {
		t.Fatalf("BrowserName = %q, want chrome", result.BrowserName)
	}
	if result.BrowserVersion != "119.0" {
		t.Fatalf("BrowserVersion = %q, want 119.0", result.BrowserVersion)
	}
	if result.ImageTag != "selenium/standalone-chrome:119.0-chromedriver-119.0-grid-4.43.0-20260404" {
		t.Fatalf("ImageTag = %q, want full Selenium tag", result.ImageTag)
	}
	if !result.Recommended {
		t.Fatal("Recommended = false, want true")
	}
	if result.DriverVersion != "119.0" {
		t.Fatalf("DriverVersion = %q, want 119.0", result.DriverVersion)
	}
	if result.GridVersion != "4.43.0" {
		t.Fatalf("GridVersion = %q, want 4.43.0", result.GridVersion)
	}
	if result.ReleaseDate != "20260404" {
		t.Fatalf("ReleaseDate = %q, want 20260404", result.ReleaseDate)
	}
	if len(result.Platforms) != 1 || result.Platforms[0] != "linux/amd64" {
		t.Fatalf("Platforms = %#v, want linux/amd64 only", result.Platforms)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one Apple Silicon warning", result.Warnings)
	}
}
