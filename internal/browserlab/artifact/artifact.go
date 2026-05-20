package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Store struct {
	Root string
}

type ScreenshotInput struct {
	GroupType      string
	GroupID        string
	SessionID      string
	RunID          string
	BrowserName    string
	BrowserVersion string
	RequestedURL   string
	CurrentURL     string
	Title          string
	CapturedAt     time.Time
	PNG            []byte
}

type ScreenshotResult struct {
	GroupType      string `json:"groupType"`
	GroupID        string `json:"groupId"`
	SessionID      string `json:"sessionId,omitempty"`
	RunID          string `json:"runId,omitempty"`
	BrowserName    string `json:"browserName"`
	BrowserVersion string `json:"browserVersion"`
	RequestedURL   string `json:"requestedUrl,omitempty"`
	CurrentURL     string `json:"currentUrl,omitempty"`
	Title          string `json:"title,omitempty"`
	CapturedAt     string `json:"capturedAt"`
	GroupDir       string `json:"groupDir"`
	ScreenshotPath string `json:"screenshotPath"`
	MetadataPath   string `json:"metadataPath"`
	ResultPath     string `json:"resultPath"`
	SummaryPath    string `json:"summaryPath"`
}

type ScreenshotMetadata = ScreenshotResult

func (store Store) WriteScreenshot(input ScreenshotInput) (ScreenshotResult, error) {
	if store.Root == "" {
		return ScreenshotResult{}, fmt.Errorf("artifact root is required")
	}
	groupType := strings.ToLower(strings.TrimSpace(input.GroupType))
	if groupType != "session" && groupType != "run" {
		return ScreenshotResult{}, fmt.Errorf("unsupported artifact group type %q", input.GroupType)
	}
	groupID := sanitizePathPart(input.GroupID)
	if groupID == "" {
		return ScreenshotResult{}, fmt.Errorf("artifact group ID is required")
	}
	capturedAt := input.CapturedAt
	if capturedAt.IsZero() {
		capturedAt = time.Now()
	}
	capturedAt = capturedAt.UTC()
	prefix := capturedAt.Format("20060102T150405Z")

	groupRoot := "sessions"
	if groupType == "run" {
		groupRoot = "runs"
	}
	groupDir := filepath.Join(store.Root, groupRoot, groupID)
	if err := os.MkdirAll(groupDir, 0o700); err != nil {
		return ScreenshotResult{}, fmt.Errorf("create artifact group: %w", err)
	}

	result := ScreenshotResult{
		GroupType:      groupType,
		GroupID:        groupID,
		SessionID:      strings.TrimSpace(input.SessionID),
		RunID:          strings.TrimSpace(input.RunID),
		BrowserName:    strings.ToLower(strings.TrimSpace(input.BrowserName)),
		BrowserVersion: strings.TrimSpace(input.BrowserVersion),
		RequestedURL:   strings.TrimSpace(input.RequestedURL),
		CurrentURL:     strings.TrimSpace(input.CurrentURL),
		Title:          strings.TrimSpace(input.Title),
		CapturedAt:     capturedAt.Format(time.RFC3339),
		GroupDir:       groupDir,
		ScreenshotPath: filepath.Join(groupDir, prefix+"-screenshot.png"),
		MetadataPath:   filepath.Join(groupDir, prefix+"-metadata.json"),
		ResultPath:     filepath.Join(groupDir, prefix+"-result.json"),
		SummaryPath:    filepath.Join(groupDir, prefix+"-summary.md"),
	}

	if err := os.WriteFile(result.ScreenshotPath, input.PNG, 0o600); err != nil {
		return ScreenshotResult{}, fmt.Errorf("write screenshot: %w", err)
	}
	if err := writeJSONFile(result.MetadataPath, result); err != nil {
		return ScreenshotResult{}, fmt.Errorf("write metadata: %w", err)
	}
	if err := writeJSONFile(result.ResultPath, result); err != nil {
		return ScreenshotResult{}, fmt.Errorf("write result: %w", err)
	}
	if err := os.WriteFile(result.SummaryPath, []byte(summaryMarkdown(result)), 0o600); err != nil {
		return ScreenshotResult{}, fmt.Errorf("write summary: %w", err)
	}
	return result, nil
}

func writeJSONFile(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func summaryMarkdown(result ScreenshotResult) string {
	lines := []string{
		"# BrowserLab Screenshot",
		"",
		"- Browser: " + strings.TrimSpace(result.BrowserName+" "+result.BrowserVersion),
		"- Captured at: " + result.CapturedAt,
		"- Screenshot: " + result.ScreenshotPath,
		"- Metadata: " + result.MetadataPath,
	}
	if result.SessionID != "" {
		lines = append(lines, "- Session: "+result.SessionID)
	}
	if result.RunID != "" {
		lines = append(lines, "- Run: "+result.RunID)
	}
	if result.CurrentURL != "" {
		lines = append(lines, "- Current URL: "+result.CurrentURL)
	}
	if result.Title != "" {
		lines = append(lines, "- Title: "+result.Title)
	}
	return strings.Join(lines, "\n") + "\n"
}

var unsafePathPart = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sanitizePathPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, ".")
	value = strings.ReplaceAll(value, string(filepath.Separator), "-")
	value = unsafePathPart.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-.")
	return value
}
