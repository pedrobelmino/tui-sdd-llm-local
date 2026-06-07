package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "http://127.0.0.1:11434"
	DefaultModel   = "qwen2.5-coder"
	clientTimeout  = 5 * time.Second
)

// Client fetches model metadata from a running Ollama instance.
type Client interface {
	Tags(ctx context.Context) ([]TagModel, error)
	Ps(ctx context.Context) ([]RunningModel, error)
	Reachable(ctx context.Context) bool
}

// Snapshot aggregates installed and running models plus reachability state.
type Snapshot struct {
	Tags                []TagModel
	Running             []RunningModel
	Reachable           bool
	Error               string
	DefaultModelMissing bool
	FetchedAt           time.Time
}

type httpClient struct {
	baseURL string
	http    *http.Client
}

// NewClient returns an HTTP client for Ollama. When baseURL is empty, OLLAMA_HOST
// is used, falling back to DefaultBaseURL.
func NewClient(baseURL string) Client {
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = DefaultBaseURL
		}
	}

	return &httpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: clientTimeout,
		},
	}
}

func (c *httpClient) Tags(ctx context.Context) ([]TagModel, error) {
	var resp ListResponse
	if err := c.getJSON(ctx, "/api/tags", &resp); err != nil {
		return nil, err
	}
	return resp.Models, nil
}

func (c *httpClient) Ps(ctx context.Context) ([]RunningModel, error) {
	var resp ProcessResponse
	if err := c.getJSON(ctx, "/api/ps", &resp); err != nil {
		return nil, err
	}
	return resp.Models, nil
}

func (c *httpClient) Reachable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK
}

func (c *httpClient) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if len(body) > 0 {
			return fmt.Errorf("ollama %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return fmt.Errorf("ollama %s: status %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("ollama %s: decode response: %w", path, err)
	}

	return nil
}

// FetchSnapshot loads installed and running models, setting reachability and error state.
func FetchSnapshot(ctx context.Context, c Client) Snapshot {
	snap := Snapshot{
		FetchedAt: time.Now(),
		Reachable: c.Reachable(ctx),
	}

	tags, err := c.Tags(ctx)
	if err != nil {
		snap.Error = err.Error()
		snap.Reachable = false
		return snap
	}
	snap.Tags = tags
	snap.DefaultModelMissing = !hasDefaultModel(tags)

	running, err := c.Ps(ctx)
	if err != nil {
		snap.Error = err.Error()
		return snap
	}
	snap.Running = running

	return snap
}

func hasDefaultModel(tags []TagModel) bool {
	for _, tag := range tags {
		if strings.Contains(tag.Name, DefaultModel) {
			return true
		}
	}
	return false
}
