package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Type       string                        `json:"type"` // "object"
	Properties map[string]ToolPropertySchema `json:"properties"`
	Required   []string                      `json:"required,omitempty"`
}

// ToolPropertySchema describes a single parameter field.
type ToolPropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolExecutor is called for each tool invocation; returns a result string.
type ToolExecutor func(toolName string, args map[string]any) string

// --- GenerateClient extension ---

// GenerateClientWithTools extends GenerateClient with text-based tool-calling support.
// Works with any model (does not require native function-calling support).
// NewGenerateClient returns a value that satisfies this interface.
type GenerateClientWithTools interface {
	GenerateClient
	// ChatWithTools streams a tool-use loop until the model stops calling tools.
	// Tool invocations are parsed from the model's text output using a
	// structured tag format — no native function-calling API required.
	ChatWithTools(ctx context.Context, msgs []ChatMessageWithTools, tools []ToolDef, exec ToolExecutor, onChunk func(string)) (string, TokenUsage, error)
}

// ChatMessageWithTools is a regular chat message (tool_calls field kept for
// future compatibility but not used in text-based mode).
type ChatMessageWithTools struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Ensure genClient satisfies GenerateClientWithTools at compile time.
var _ GenerateClientWithTools = (*genClient)(nil)

const (
	toolCallOpen  = "<tool_call>"
	toolCallClose = "</tool_call>"
	maxToolIter   = 30 // prevent infinite loops
)

// toolCallJSON is the JSON payload inside a <tool_call> block.
type toolCallJSON struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// ChatWithTools runs a streaming tool-use loop.
//
// The model is instructed (via the system prompt injected by injectToolInstructions)
// to respond with <tool_call>{"tool":"name","args":{...}}</tool_call> when it wants
// to invoke a tool. We stream the response, detect tool-call blocks, execute them,
// and continue the conversation until the model produces a final text answer.
//
// Bootstrap: we always pre-seed the conversation by executing list_dir(".") ourselves
// and showing the result to the model. This gives the model the project structure
// upfront and primes the few-shot pattern so it knows the expected format.
func (c *genClient) ChatWithTools(
	ctx context.Context,
	msgs []ChatMessageWithTools,
	tools []ToolDef,
	exec ToolExecutor,
	onChunk func(string),
) (string, TokenUsage, error) {
	var totalUsage TokenUsage

	model := ""
	if m := ctx.Value(ctxKeyModel{}); m != nil {
		model, _ = m.(string)
	}

	// Inject tool instructions into the last system message (or prepend one).
	history := injectToolInstructions(msgs, tools)

	// Bootstrap: pre-execute list_dir(".") and inject it as a completed exchange.
	// This gives the model the project layout and demonstrates the tool-call format.
	if exec != nil && len(tools) > 0 {
		ldResult := exec("list_dir", map[string]any{"path": "."})
		if onChunk != nil {
			onChunk("🔧 list_dir(.)\n   ✓ " + ldResult + "\n\n")
		}
		history = append(history,
			ChatMessageWithTools{
				Role:    "assistant",
				Content: toolCallOpen + "\n{\"tool\":\"list_dir\",\"args\":{\"path\":\".\"}}"+toolCallClose,
			},
			ChatMessageWithTools{
				Role:    "user",
				Content: "Tool result:\n" + ldResult + "\n\nUse the file tools to make all necessary changes now.",
			},
		)
	}

	for iter := 0; iter < maxToolIter; iter++ {
		// Convert to plain ChatMessages for ChatStream.
		plain := make([]ChatMessage, len(history))
		for i, m := range history {
			plain[i] = ChatMessage{Role: m.Role, Content: m.Content}
		}

		// Stream the response, accumulating full text.
		var buf strings.Builder
		streamOnChunk := func(chunk string) {
			buf.WriteString(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
		}

		_, usage, err := c.ChatStream(ctx, ChatRequest{
			Model:    model,
			Messages: plain,
		}, streamOnChunk)
		totalUsage.PromptTokens += usage.PromptTokens
		totalUsage.CompletionTokens += usage.CompletionTokens
		if err != nil {
			return buf.String(), totalUsage, err
		}

		response := buf.String()

		// Check for tool call tag in the response.
		tc, found := parseToolCall(response)
		if !found {
			// No tool call found. If this is the very first model turn (iter==0)
			// and the response looks like a plan/description rather than a summary,
			// nudge the model to actually start calling tools.
			if iter == 0 && looksLikePlan(response) {
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: "Stop describing. Start making file changes now — respond with a <tool_call> block."},
				)
				continue
			}
			// Final answer.
			return response, totalUsage, nil
		}

		// Strip the tool_call block from the displayed text (already streamed).
		// Notify about execution.
		if onChunk != nil {
			onChunk(fmt.Sprintf("\n🔧 %s(%s)\n", tc.Tool, formatArgs(tc.Args)))
		}
		result := exec(tc.Tool, tc.Args)
		if onChunk != nil {
			onChunk("   ✓ " + result + "\n")
		}

		// Append assistant response + tool result to history and loop.
		history = append(history,
			ChatMessageWithTools{Role: "assistant", Content: response},
			ChatMessageWithTools{Role: "user", Content: "Tool result: " + result + "\n\nContinue."},
		)
	}

	return "", totalUsage, fmt.Errorf("tool-calling loop exceeded %d iterations", maxToolIter)
}

// parseToolCall extracts the first <tool_call>...</tool_call> block from text.
func parseToolCall(text string) (toolCallJSON, bool) {
	start := strings.Index(text, toolCallOpen)
	if start < 0 {
		return toolCallJSON{}, false
	}
	inner := text[start+len(toolCallOpen):]
	end := strings.Index(inner, toolCallClose)
	if end < 0 {
		return toolCallJSON{}, false
	}
	raw := strings.TrimSpace(inner[:end])
	var tc toolCallJSON
	if err := json.Unmarshal([]byte(raw), &tc); err != nil {
		return toolCallJSON{}, false
	}
	if tc.Tool == "" {
		return toolCallJSON{}, false
	}
	return tc, true
}

// injectToolInstructions adds the tool-usage instructions to the system prompt.
func injectToolInstructions(msgs []ChatMessageWithTools, tools []ToolDef) []ChatMessageWithTools {
	if len(tools) == 0 {
		return msgs
	}
	addendum := buildToolsAddendum(tools)

	// Find existing system message and extend it.
	result := append([]ChatMessageWithTools(nil), msgs...)
	for i, m := range result {
		if m.Role == "system" {
			result[i].Content = m.Content + "\n\n" + addendum
			return result
		}
	}
	// No system message — prepend one.
	return append([]ChatMessageWithTools{{Role: "system", Content: addendum}}, result...)
}

// buildToolsAddendum returns the text block appended to the system prompt.
func buildToolsAddendum(tools []ToolDef) string {
	var sb strings.Builder
	sb.WriteString("## File Tools\n\n")
	sb.WriteString("You MUST use the file tools below to create and modify files.\n")
	sb.WriteString("Each tool call is your ENTIRE response — no extra text before or after.\n\n")
	sb.WriteString("Format:\n")
	sb.WriteString(toolCallOpen + "\n")
	sb.WriteString("{\"tool\": \"TOOL_NAME\", \"args\": {\"param\": \"value\"}}\n")
	sb.WriteString(toolCallClose + "\n\n")
	sb.WriteString("After execution you receive the result and MUST continue with the next tool call or final summary.\n\n")
	sb.WriteString("Example session:\n")
	sb.WriteString("  User: implement the feature\n")
	sb.WriteString("  You: " + toolCallOpen + "{\"tool\":\"read_file\",\"args\":{\"path\":\"go.mod\"}}" + toolCallClose + "\n")
	sb.WriteString("  User: Tool result: module github.com/example/app go 1.22\n")
	sb.WriteString("  You: " + toolCallOpen + "{\"tool\":\"write_file\",\"args\":{\"path\":\"internal/foo.go\",\"content\":\"package foo\\n\"}}" + toolCallClose + "\n")
	sb.WriteString("  User: Tool result: wrote internal/foo.go\n")
	sb.WriteString("  You: Done. Created internal/foo.go with package foo.\n\n")
	sb.WriteString("Available tools:\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Function.Name, t.Function.Description))
		sb.WriteString("  Parameters:\n")
		for pname, pschema := range t.Function.Parameters.Properties {
			req := ""
			for _, r := range t.Function.Parameters.Required {
				if r == pname {
					req = " (required)"
				}
			}
			sb.WriteString(fmt.Sprintf("  - `%s` (%s)%s: %s\n", pname, pschema.Type, req, pschema.Description))
		}
	}
	sb.WriteString("\nCRITICAL: Do NOT describe what you will do. Do NOT write explanations. Call the tools directly.")
	return sb.String()
}

type ctxKeyModel struct{}

// WithModel attaches the model name to a context for ChatWithTools.
func WithModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, ctxKeyModel{}, model)
}

// looksLikePlan returns true when the model response looks like a plan/description
// rather than a final summary — heuristic to detect "I will do X" style responses.
func looksLikePlan(text string) bool {
	lower := strings.ToLower(text)
	planPhrases := []string{
		"i will", "i'll", "i would", "i'm going to", "i plan to",
		"next, i", "first, i", "let me", "proceed to",
		"will create", "will write", "will implement", "will add",
		"steps:", "step 1", "step 2",
	}
	for _, p := range planPhrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
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
	return strings.Join(parts, ", ")
}
