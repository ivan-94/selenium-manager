package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/ivan-94/selenium-manager/internal/browserlab/grid"
	"github.com/ivan-94/selenium-manager/internal/browserlab/registry"
)

type CreateRequest struct {
	BrowserName    string `json:"browserName"`
	BrowserVersion string `json:"browserVersion"`
	URL            string `json:"url,omitempty"`
}

type CreateResponse struct {
	SessionID         string          `json:"sessionId"`
	BrowserName       string          `json:"browserName"`
	BrowserVersion    string          `json:"browserVersion"`
	RequestedURL      string          `json:"requestedUrl"`
	CurrentURL        string          `json:"currentUrl,omitempty"`
	Title             string          `json:"title,omitempty"`
	GridURL           string          `json:"gridUrl"`
	WebDriverEndpoint string          `json:"webdriverEndpoint"`
	NoVNC             NoVNCResolution `json:"noVnc"`
	StartedAt         string          `json:"startedAt"`
}

type NoVNCResolution struct {
	URL             string `json:"url,omitempty"`
	VNCWebSocketURL string `json:"vncWebSocketUrl,omitempty"`
	VNCLocalAddress string `json:"vncLocalAddress,omitempty"`
	GridSessionURL  string `json:"gridSessionUrl,omitempty"`
}

type NewSessionRequest struct {
	WebDriverEndpoint string
	Capabilities      map[string]any
}

type NewSessionResult struct {
	SessionID    string
	Capabilities map[string]any
}

type WebDriverClient interface {
	NewSession(ctx context.Context, request NewSessionRequest) (NewSessionResult, error)
	Navigate(ctx context.Context, webDriverEndpoint string, sessionID string, url string) error
	CurrentURL(ctx context.Context, webDriverEndpoint string, sessionID string) (string, error)
	Title(ctx context.Context, webDriverEndpoint string, sessionID string) (string, error)
	Quit(ctx context.Context, webDriverEndpoint string, sessionID string) error
}

type Manager struct {
	Store     registry.FileStore
	Grid      grid.Manager
	WebDriver WebDriverClient
	Now       func() time.Time
}

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (problem Problem) Error() string {
	return problem.Message
}

func (manager Manager) CreateHeldSession(ctx context.Context, request CreateRequest) (CreateResponse, error) {
	request = normalizeCreateRequest(request)
	if request.BrowserName == "" {
		request.BrowserName = "chrome"
	}
	if request.URL == "" {
		request.URL = "about:blank"
	}
	if request.BrowserName != "chrome" {
		return CreateResponse{}, Problem{Code: "unsupported_browser", Message: "only Chrome manual sessions are supported in this slice"}
	}
	if request.BrowserVersion == "" {
		return CreateResponse{}, Problem{Code: "invalid_session_request", Message: "browserVersion is required"}
	}

	records, err := manager.Store.List()
	if err != nil {
		return CreateResponse{}, err
	}
	record, ok := findInstalledBrowser(records, request.BrowserName, request.BrowserVersion)
	if !ok {
		return CreateResponse{}, Problem{
			Code:    "browser_not_installed",
			Message: fmt.Sprintf("%s %s is not installed and enabled in Browser Registry", request.BrowserName, request.BrowserVersion),
		}
	}

	gridStatus, err := manager.ensureGridRunning(ctx, records)
	if err != nil {
		return CreateResponse{}, err
	}

	client := manager.WebDriver
	if client == nil {
		client = HTTPWebDriverClient{Client: http.DefaultClient}
	}
	newSession, err := client.NewSession(ctx, NewSessionRequest{
		WebDriverEndpoint: gridStatus.WebDriverEndpoint,
		Capabilities:      BuildCapabilities(record),
	})
	if err != nil {
		return CreateResponse{}, Problem{Code: "webdriver_session_failed", Message: err.Error()}
	}
	if newSession.SessionID == "" {
		return CreateResponse{}, Problem{Code: "webdriver_session_failed", Message: "WebDriver did not return a session ID"}
	}

	if err := client.Navigate(ctx, gridStatus.WebDriverEndpoint, newSession.SessionID, request.URL); err != nil {
		return CreateResponse{}, Problem{Code: "navigation_failed", Message: err.Error()}
	}

	currentURL, _ := client.CurrentURL(ctx, gridStatus.WebDriverEndpoint, newSession.SessionID)
	title, _ := client.Title(ctx, gridStatus.WebDriverEndpoint, newSession.SessionID)
	startedAt := manager.now().UTC().Format(time.RFC3339)

	return CreateResponse{
		SessionID:         newSession.SessionID,
		BrowserName:       record.Family,
		BrowserVersion:    record.Version,
		RequestedURL:      request.URL,
		CurrentURL:        currentURL,
		Title:             title,
		GridURL:           gridStatus.GridURL,
		WebDriverEndpoint: gridStatus.WebDriverEndpoint,
		NoVNC:             ResolveNoVNC(gridStatus.GridURL, newSession.SessionID, newSession.Capabilities),
		StartedAt:         startedAt,
	}, nil
}

func BuildCapabilities(record registry.BrowserRecord) map[string]any {
	family := strings.ToLower(strings.TrimSpace(record.Family))
	version := strings.TrimSpace(record.Version)
	return map[string]any{
		"browserName":            family,
		"browserVersion":         version,
		"platformName":           platformName(record.Platform),
		"se:name":                "BrowserLab manual " + family + " " + version,
		"browserlab:sessionType": "manual",
	}
}

func ResolveNoVNC(gridURL string, sessionID string, capabilities map[string]any) NoVNCResolution {
	resolution := NoVNCResolution{}
	gridBase, err := url.Parse(strings.TrimRight(gridURL, "/"))
	if err != nil || gridBase.Scheme == "" || gridBase.Host == "" || sessionID == "" {
		return resolution
	}

	uiURL := *gridBase
	uiURL.Path = strings.TrimRight(path.Join(uiURL.Path, "/ui"), "/") + "/"
	uiURL.RawQuery = ""
	uiURL.Fragment = "/sessions/" + sessionID
	resolution.URL = uiURL.String()
	resolution.GridSessionURL = resolution.URL

	if raw, ok := capabilities["se:vncLocalAddress"].(string); ok {
		resolution.VNCLocalAddress = raw
	}
	if raw, ok := capabilities["se:vnc"].(string); ok && raw != "" {
		if vncURL, err := url.Parse(raw); err == nil && vncURL.Path != "" {
			routed := url.URL{
				Scheme: wsScheme(gridBase.Scheme),
				Host:   gridBase.Host,
				Path:   vncURL.Path,
			}
			resolution.VNCWebSocketURL = routed.String()
		}
	}
	if resolution.VNCWebSocketURL == "" {
		routed := url.URL{
			Scheme: wsScheme(gridBase.Scheme),
			Host:   gridBase.Host,
			Path:   "/session/" + sessionID + "/se/vnc",
		}
		resolution.VNCWebSocketURL = routed.String()
	}
	return resolution
}

type HTTPWebDriverClient struct {
	Client *http.Client
}

func (client HTTPWebDriverClient) NewSession(ctx context.Context, request NewSessionRequest) (NewSessionResult, error) {
	payload := map[string]any{
		"capabilities": map[string]any{
			"alwaysMatch": request.Capabilities,
		},
	}
	var response struct {
		Value struct {
			SessionID    string         `json:"sessionId"`
			Capabilities map[string]any `json:"capabilities"`
		} `json:"value"`
		SessionID    string         `json:"sessionId"`
		Capabilities map[string]any `json:"capabilities"`
	}
	if err := client.doJSON(ctx, http.MethodPost, request.WebDriverEndpoint+"/session", payload, &response); err != nil {
		return NewSessionResult{}, err
	}
	result := NewSessionResult{
		SessionID:    response.Value.SessionID,
		Capabilities: response.Value.Capabilities,
	}
	if result.SessionID == "" {
		result.SessionID = response.SessionID
		result.Capabilities = response.Capabilities
	}
	return result, nil
}

func (client HTTPWebDriverClient) Navigate(ctx context.Context, webDriverEndpoint string, sessionID string, targetURL string) error {
	return client.doJSON(ctx, http.MethodPost, sessionEndpoint(webDriverEndpoint, sessionID, "url"), map[string]string{"url": targetURL}, nil)
}

func (client HTTPWebDriverClient) CurrentURL(ctx context.Context, webDriverEndpoint string, sessionID string) (string, error) {
	var response struct {
		Value string `json:"value"`
	}
	if err := client.doJSON(ctx, http.MethodGet, sessionEndpoint(webDriverEndpoint, sessionID, "url"), nil, &response); err != nil {
		return "", err
	}
	return response.Value, nil
}

func (client HTTPWebDriverClient) Title(ctx context.Context, webDriverEndpoint string, sessionID string) (string, error) {
	var response struct {
		Value string `json:"value"`
	}
	if err := client.doJSON(ctx, http.MethodGet, sessionEndpoint(webDriverEndpoint, sessionID, "title"), nil, &response); err != nil {
		return "", err
	}
	return response.Value, nil
}

func (client HTTPWebDriverClient) Quit(ctx context.Context, webDriverEndpoint string, sessionID string) error {
	return client.doJSON(ctx, http.MethodDelete, sessionEndpoint(webDriverEndpoint, sessionID, ""), nil, nil)
}

func (client HTTPWebDriverClient) doJSON(ctx context.Context, method string, endpoint string, body any, target any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := client.Client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var problem struct {
			Value struct {
				Message string `json:"message"`
				Error   string `json:"error"`
			} `json:"value"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&problem); err == nil && problem.Value.Message != "" {
			return fmt.Errorf("%s: %s", problem.Value.Error, problem.Value.Message)
		}
		return fmt.Errorf("webdriver returned HTTP %d", resp.StatusCode)
	}
	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return err
		}
	}
	return nil
}

func (manager Manager) ensureGridRunning(ctx context.Context, records []registry.BrowserRecord) (grid.Status, error) {
	status, err := manager.Grid.Status(ctx, records)
	if err != nil {
		return grid.Status{}, err
	}
	if status.State == "running" {
		return status, nil
	}
	return manager.Grid.Start(ctx, records)
}

func (manager Manager) now() time.Time {
	if manager.Now != nil {
		return manager.Now()
	}
	return time.Now()
}

func findInstalledBrowser(records []registry.BrowserRecord, family string, version string) (registry.BrowserRecord, bool) {
	for _, record := range records {
		if !record.Enabled {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(record.Family), family) && strings.TrimSpace(record.Version) == version {
			return record, true
		}
	}
	return registry.BrowserRecord{}, false
}

func normalizeCreateRequest(request CreateRequest) CreateRequest {
	request.BrowserName = strings.ToLower(strings.TrimSpace(request.BrowserName))
	request.BrowserVersion = strings.TrimSpace(request.BrowserVersion)
	request.URL = strings.TrimSpace(request.URL)
	return request
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

func wsScheme(httpScheme string) string {
	if httpScheme == "https" {
		return "wss"
	}
	return "ws"
}

func sessionEndpoint(webDriverEndpoint string, sessionID string, suffix string) string {
	base := strings.TrimRight(webDriverEndpoint, "/") + "/session/" + url.PathEscape(sessionID)
	if suffix == "" {
		return base
	}
	return base + "/" + strings.TrimLeft(suffix, "/")
}
