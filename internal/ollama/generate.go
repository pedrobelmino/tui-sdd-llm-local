package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ChatMessage is a single message in a chat request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the Ollama /api/chat payload.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatResponse chunk from streaming or final response.
type ChatResponse struct {
	Model              string `json:"model"`
	Message            ChatMessage `json:"message"`
	Done               bool   `json:"done"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	EvalCount          int    `json:"eval_count"`
	Error              string `json:"error,omitempty"`
}

// GenerateClient extends metadata client with generation.
type GenerateClient interface {
	Client
	Chat(ctx context.Context, req ChatRequest) (string, TokenUsage, error)
	ChatStream(ctx context.Context, req ChatRequest, onChunk func(string)) (string, TokenUsage, error)
}

// TokenUsage holds token counts from a completed generation.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
}

const streamTimeout = 10 * time.Minute

type genClient struct {
	baseURL string
	http    *http.Client
}

// NewGenerateClient returns a client with chat/generate support.
func NewGenerateClient(baseURL string) GenerateClient {
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = DefaultBaseURL
		}
	}
	return &genClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		// No Client.Timeout — streaming generation can run for minutes; deadlines
		// come from the request context (see Chat / ChatStream).
		http: &http.Client{},
	}
}

func (c *genClient) Tags(ctx context.Context) ([]TagModel, error) {
	return NewClient(c.baseURL).Tags(ctx)
}

func (c *genClient) Ps(ctx context.Context) ([]RunningModel, error) {
	return NewClient(c.baseURL).Ps(ctx)
}

func (c *genClient) Reachable(ctx context.Context) bool {
	return NewClient(c.baseURL).Reachable(ctx)
}

// Chat performs a non-streaming chat completion.
func (c *genClient) Chat(ctx context.Context, req ChatRequest) (string, TokenUsage, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return "", TokenUsage{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, streamTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", TokenUsage{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", TokenUsage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", TokenUsage{}, fmt.Errorf("ollama chat: %d: %s", resp.StatusCode, string(b))
	}

	var out ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", TokenUsage{}, err
	}
	if out.Error != "" {
		return "", TokenUsage{}, fmt.Errorf("ollama: %s", out.Error)
	}

	return out.Message.Content, TokenUsage{
		PromptTokens:     out.PromptEvalCount,
		CompletionTokens: out.EvalCount,
	}, nil
}

// ChatStream streams chat response to onChunk, returns full text and token usage.
func (c *genClient) ChatStream(ctx context.Context, req ChatRequest, onChunk func(string)) (string, TokenUsage, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return "", TokenUsage{}, err
	}

	streamCtx, cancel := context.WithTimeout(ctx, streamTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(streamCtx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", TokenUsage{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", TokenUsage{}, err
	}
	defer resp.Body.Close()

	var full strings.Builder
	var usage TokenUsage
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimSpace(line)
			if len(line) > 0 {
				var chunk ChatResponse
				if json.Unmarshal(line, &chunk) == nil {
					if chunk.Error != "" {
						return full.String(), usage, fmt.Errorf("ollama: %s", chunk.Error)
					}
					if chunk.Message.Content != "" {
						full.WriteString(chunk.Message.Content)
						if onChunk != nil {
							onChunk(chunk.Message.Content)
						}
					}
					if chunk.Done {
						usage.PromptTokens = chunk.PromptEvalCount
						usage.CompletionTokens = chunk.EvalCount
						break
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return full.String(), usage, err
		}
	}
	return full.String(), usage, nil
}
