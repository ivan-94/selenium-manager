package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type DockerHubTagSource struct {
	Client  HTTPDoer
	BaseURL string
}

func NewDockerHubTagSource() DockerHubTagSource {
	return DockerHubTagSource{
		Client:  http.DefaultClient,
		BaseURL: "https://hub.docker.com/v2/repositories",
	}
}

func (source DockerHubTagSource) FetchChromeTags(ctx context.Context, query string) (DockerHubTagsPage, error) {
	client := source.Client
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := source.BaseURL
	if baseURL == "" {
		baseURL = "https://hub.docker.com/v2/repositories"
	}

	nextURL := fmt.Sprintf("%s/%s/tags?page_size=100", baseURL, StandaloneChromeRepository)
	if query != "" {
		nextURL += "&name=" + url.QueryEscape(query)
	}

	var merged DockerHubTagsPage
	for page := 0; page < 5 && nextURL != ""; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return DockerHubTagsPage{}, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return DockerHubTagsPage{}, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return DockerHubTagsPage{}, fmt.Errorf("docker hub returned HTTP %d", resp.StatusCode)
		}
		var fetched DockerHubTagsPage
		err = json.NewDecoder(resp.Body).Decode(&fetched)
		resp.Body.Close()
		if err != nil {
			return DockerHubTagsPage{}, err
		}
		merged.Results = append(merged.Results, fetched.Results...)
		nextURL = fetched.Next
	}
	return merged, nil
}
