package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// --- Tool types ---

// ToolDef defines a function available to the model.
type ToolDef struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction holds name, description, and JSON schema for parameters.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

// ToolParameters is a JSON-Schema object for tool input.
type ToolParameters struct {
	Type       string                       `json:"type"` // "object"
	Properties map[string]ToolPropertySchema `json:"properties"`
	Required   []string                     `json:"required,omitempty"`
}

// ToolPropertySchema describes a single parameter field.
type ToolPropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolCall is a tool invocation returned by the model.
type ToolCall struct {
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the called tool name and its arguments.
type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// --- Extended message and request types ---

// ChatMessageWithTools extends ChatMessage to carry optional tool_calls.
type ChatMessageWithTools struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequestWithTools is the Ollama /api/chat payload that includes tools.
type ChatRequestWithTools struct {
	Model    string                 `json:"model"`
	Messages []ChatMessageWithTools `json:"messages"`
	Tools    []ToolDef              `json:"tools,omitempty"`
	Stream   bool                   `json:"stream"`
}

// ChatResponseWithTools is the response chunk when tools may be involved.
type ChatResponseWithTools struct {
	Model           string               `json:"model"`
	Message         ChatMessageWithTools `json:"message"`
	Done            bool                 `json:"done"`
	PromptEvalCount int                  `json:"prompt_eval_count"`
	EvalCount       int                  `json:"eval_count"`
	Error           string               `json:"error,omitempty"`
}

// ToolExecutor is called for each tool invocation; returns a result string.
type ToolExecutor func(toolName string, args map[string]any) string

// --- GenerateClient extension ---

// GenerateClientWithTools extends GenerateClient with tool-calling support.
// NewGenerateClient returns a value that satisfies this interface.
type GenerateClientWithTools interface {
	GenerateClient
	// ChatWithTools runs a tool-use loop: the model can invoke tools until it
	// stops calling them and produces a final text answer.
	// onChunk receives both streamed text tokens and tool-invocation status lines.
	ChatWithTools(ctx context.Context, msgs []ChatMessageWithTools, tools []ToolDef, exec ToolExecutor, onChunk func(string)) (string, TokenUsage, error)
}

// Ensure genClient satisfies GenerateClientWithTools at compile time.
var _ GenerateClientWithTools = (*genClient)(nil)

// ChatWithTools is implemented on genClient so it satisfies the interface.
func (c *genClient) ChatWithTools(
	ctx context.Context,
	msgs []ChatMessageWithTools,
	tools []ToolDef,
	exec ToolExecutor,
	onChunk func(string),
) (string, TokenUsage, error) {
	var totalUsage TokenUsage
	history := append([]ChatMessageWithTools(nil), msgs...)

	for {
		req := ChatRequestWithTools{
			Model:    "",    // caller sets model via msgs context; set below
			Messages: history,
			Tools:    tools,
			Stream:   false,
		}
		// Extract model from context value if set, else leave empty (Ollama uses default).
		if m := ctx.Value(ctxKeyModel{}); m != nil {
			req.Model, _ = m.(string)
		}

		body, err := json.Marshal(req)
		if err != nil {
			return "", totalUsage, err
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
		if err != nil {
			return "", totalUsage, err
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(httpReq)
		if err != nil {
			return "", totalUsage, err
		}
		body2, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", totalUsage, err
		}
		if resp.StatusCode != http.StatusOK {
			return "", totalUsage, fmt.Errorf("ollama tools: %d: %s", resp.StatusCode, string(body2))
		}

		var out ChatResponseWithTools
		if err := json.Unmarshal(body2, &out); err != nil {
			return "", totalUsage, fmt.Errorf("ollama tools decode: %w", err)
		}
		if out.Error != "" {
			return "", totalUsage, fmt.Errorf("ollama: %s", out.Error)
		}

		totalUsage.PromptTokens += out.PromptEvalCount
		totalUsage.CompletionTokens += out.EvalCount

		// Append assistant message to history.
		history = append(history, out.Message)

		if len(out.Message.ToolCalls) == 0 {
			// No tool calls — final text response.
			if onChunk != nil && out.Message.Content != "" {
				onChunk(out.Message.Content)
			}
			return out.Message.Content, totalUsage, nil
		}

		// Execute each tool call and collect results.
		for _, tc := range out.Message.ToolCalls {
			if onChunk != nil {
				onChunk(fmt.Sprintf("\n🔧 %s(%s)\n", tc.Function.Name, formatArgs(tc.Function.Arguments)))
			}
			result := exec(tc.Function.Name, tc.Function.Arguments)
			if onChunk != nil {
				onChunk("   ✓ " + result + "\n")
			}
			history = append(history, ChatMessageWithTools{
				Role:    "tool",
				Content: result,
			})
		}
	}
}

type ctxKeyModel struct{}

// WithModel attaches the model name to a context for ChatWithTools.
func WithModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, ctxKeyModel{}, model)
}

func formatArgs(args map[string]any) string {
	parts := make([]string, 0, len(args))
	for k, v := range args {
		s := fmt.Sprintf("%v", v)
		if len(s) > 40 {
			s = s[:40] + "…"
		}
		parts = append(parts, k+"="+s)
	}
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}
