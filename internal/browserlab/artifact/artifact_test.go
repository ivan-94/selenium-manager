package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreWritesSessionScreenshotArtifactsWithStablePathsAndMetadata(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}
	now := time.Date(2026, 5, 20, 10, 30, 0, 0, time.UTC)

	result, err := store.WriteScreenshot(ScreenshotInput{
		GroupType:      "session",
		GroupID:        "session-123",
		SessionID:      "session-123",
		BrowserName:    "chrome",
		BrowserVersion: "119.0",
		RequestedURL:   "https://example.test/",
		CurrentURL:     "https://example.test/current",
		Title:          "Example",
		CapturedAt:     now,
		PNG:            []byte("png bytes"),
	})
	if err != nil {
		t.Fatalf("WriteScreenshot() error = %v", err)
	}

	wantDir := filepath.Join(root, "sessions", "session-123")
	if result.GroupDir != wantDir {
		t.Fatalf("GroupDir = %q, want %q", result.GroupDir, wantDir)
	}
	if result.ScreenshotPath != filepath.Join(wantDir, "20260520T103000Z-screenshot.png") {
		t.Fatalf("ScreenshotPath = %q", result.ScreenshotPath)
	}
	if result.MetadataPath != filepath.Join(wantDir, "20260520T103000Z-metadata.json") {
		t.Fatalf("MetadataPath = %q", result.MetadataPath)
	}
	if result.ResultPath != filepath.Join(wantDir, "20260520T103000Z-result.json") {
		t.Fatalf("ResultPath = %q", result.ResultPath)
	}
	if result.SummaryPath != filepath.Join(wantDir, "20260520T103000Z-summary.md") {
		t.Fatalf("SummaryPath = %q", result.SummaryPath)
	}

	writtenPNG, err := os.ReadFile(result.ScreenshotPath)
	if err != nil {
		t.Fatalf("read screenshot: %v", err)
	}
	if string(writtenPNG) != "png bytes" {
		t.Fatalf("screenshot bytes = %q", string(writtenPNG))
	}

	var metadata ScreenshotMetadata
	data, err := os.ReadFile(result.MetadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("decode metadata: %v\n%s", err, string(data))
	}
	if metadata.GroupType != "session" || metadata.GroupID != "session-123" || metadata.SessionID != "session-123" {
		t.Fatalf("metadata group fields = %+v", metadata)
	}
	if metadata.BrowserName != "chrome" || metadata.BrowserVersion != "119.0" {
		t.Fatalf("metadata browser fields = %+v", metadata)
	}
	if metadata.RequestedURL != "https://example.test/" || metadata.CurrentURL != "https://example.test/current" || metadata.Title != "Example" {
		t.Fatalf("metadata page fields = %+v", metadata)
	}
	if metadata.CapturedAt != "2026-05-20T10:30:00Z" {
		t.Fatalf("CapturedAt = %q", metadata.CapturedAt)
	}
	if metadata.ScreenshotPath != result.ScreenshotPath {
		t.Fatalf("metadata ScreenshotPath = %q, want %q", metadata.ScreenshotPath, result.ScreenshotPath)
	}
}

func TestStoreSanitizesRunGroupPaths(t *testing.T) {
	root := t.TempDir()
	store := Store{Root: root}

	result, err := store.WriteScreenshot(ScreenshotInput{
		GroupType:      "run",
		GroupID:        "../chrome 119.0",
		BrowserName:    "chrome",
		BrowserVersion: "119.0",
		CapturedAt:     time.Date(2026, 5, 20, 10, 31, 0, 0, time.UTC),
		PNG:            []byte("png"),
	})
	if err != nil {
		t.Fatalf("WriteScreenshot() error = %v", err)
	}

	wantDir := filepath.Join(root, "runs", "chrome-119.0")
	if result.GroupDir != wantDir {
		t.Fatalf("GroupDir = %q, want %q", result.GroupDir, wantDir)
	}
}
