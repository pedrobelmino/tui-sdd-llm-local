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
type GenerateClientWithTools interface {
	GenerateClient
	// ChatWithTools streams a tool-use loop until the model stops calling tools.
	// onChunk receives streamed text tokens and status lines (🔧 tool calls, ✓ results).
	ChatWithTools(ctx context.Context, msgs []ChatMessageWithTools, tools []ToolDef, exec ToolExecutor, onChunk func(string)) (string, TokenUsage, error)
}

// ChatMessageWithTools is a plain chat message used in the tool-calling loop.
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

// emit is a helper to send a status line via onChunk, adding a trailing newline.
func emit(onChunk func(string), format string, args ...any) {
	if onChunk == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	onChunk(msg)
}

// ChatWithTools runs a streaming tool-use loop.
//
// The model is instructed via a system-prompt addendum to reply with:
//
//	<tool_call>{"tool":"name","args":{...}}</tool_call>
//
// when it wants to invoke a tool. We detect that tag in each streamed response,
// execute the corresponding ToolExecutor, and feed the result back as a user
// message — repeating until the model produces a final text answer.
//
// Project layout is pre-loaded via list_dir and appended to the last user
// message so the model has context without a fake assistant turn.
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

	// Inject tool instructions into system prompt (or prepend system message).
	history := injectToolInstructions(msgs, tools)

	// Pre-load project layout and append to the last user message so the model
	// starts with directory context — no fake assistant/tool turn needed.
	if exec != nil && len(tools) > 0 {
		emit(onChunk, "📂 loading project layout…")
		ldResult := exec("list_dir", map[string]any{"path": "."})
		if ldResult == "" {
			ldResult = "(empty)"
		}
		emit(onChunk, "   %s\n", ldResult)

		// Find the last user message and append layout + clear instruction.
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == "user" {
				history[i].Content += "\n\nProject root layout (list_dir \".\"):\n" + ldResult +
					"\n\nNOTE: All spec/design/tasks files for the feature are already provided above in the message. " +
					"Do NOT read them via read_file — use tools only for SOURCE CODE files." +
					"\n\nStart making file changes now — respond with a <tool_call> block."
				break
			}
		}
	}

	for iter := 0; iter < maxToolIter; iter++ {
		// Convert to plain ChatMessages for ChatStream.
		plain := make([]ChatMessage, len(history))
		for i, h := range history {
			plain[i] = ChatMessage{Role: h.Role, Content: h.Content}
		}

		// Stream the response into buf; suppress raw model text from the visible log
		// (it's mostly the tool-call JSON or an intermediate plan — not user-friendly).
		var buf strings.Builder
		_, usage, err := c.ChatStream(ctx, ChatRequest{
			Model:    model,
			Messages: plain,
		}, func(chunk string) { buf.WriteString(chunk) })
		totalUsage.PromptTokens += usage.PromptTokens
		totalUsage.CompletionTokens += usage.CompletionTokens

		response := strings.TrimSpace(buf.String())

		if err != nil {
			emit(onChunk, "❌ error on turn %d: %v", iter+1, err)
			return response, totalUsage, err
		}

		if response == "" {
			msg := fmt.Sprintf("model returned empty response on turn %d (%d prompt tokens)", iter+1, usage.PromptTokens)
			emit(onChunk, "❌ %s", msg)
			return response, totalUsage, fmt.Errorf("%s", msg)
		}

		// Check for tool call tag in the response.
		tc, found := parseToolCall(response)
		if !found {
			// If iter==0 and the model wrote a plan instead of a tool call, nudge once.
			if iter == 0 && looksLikePlan(response) {
				emit(onChunk, "⚠  model wrote a plan instead of calling tools — retrying…")
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: "Do not describe. Respond with a <tool_call> block to start making file changes."},
				)
				continue
			}
			// Final answer — stream it to the log now that we know it's the summary.
			if onChunk != nil {
				onChunk(response)
			}
			return response, totalUsage, nil
		}

		// Strip any nested tool-call JSON from write_file/edit_file content
		// to prevent the model from embedding the next call inside file content.
		tc = sanitizeToolArgs(tc)

		// Execute the tool and show it in the log.
		emit(onChunk, "🔧 %s(%s)", tc.Tool, formatArgs(tc.Args))
		result := exec(tc.Tool, tc.Args)

		var continuation string
		if strings.HasPrefix(result, "ERROR") {
			emit(onChunk, "❌ %s", result)
			continuation = "Tool result (FAILED):\n" + result +
				"\n\nThe tool failed. Do NOT retry the same call blindly. " +
				"If the file does not exist, use list_dir to discover the correct path. " +
				"If you cannot recover, stop and explain what is missing."
		} else {
			emit(onChunk, "   ✓ %s", result)
			continuation = "Tool result:\n" + result + "\n\nContinue."
		}

		// Append assistant response + tool result and loop.
		history = append(history,
			ChatMessageWithTools{Role: "assistant", Content: response},
			ChatMessageWithTools{Role: "user", Content: continuation},
		)
	}

	return "", totalUsage, fmt.Errorf("tool-calling loop exceeded %d iterations", maxToolIter)
}

// parseToolCall tries multiple formats the model might use for tool calls:
//  1. <tool_call>{"tool":"...","args":{...}}</tool_call>   (preferred)
//  2. ```json{"tool":"...","args":{...}}```                (code block, json tag)
//  3. ```{"tool":"...","args":{...}}```                    (code block, no tag)
//  4. {"tool":"...","args":{...}}                          (bare JSON anywhere)
func parseToolCall(text string) (toolCallJSON, bool) {
	// 1. Preferred: <tool_call> tags
	if tc, ok := parseTaggedToolCall(text); ok {
		return tc, true
	}
	// 2 & 3: code block with or without "json" language hint
	for _, sep := range []string{"```json\n", "```json", "```\n", "```"} {
		idx := strings.Index(text, sep)
		if idx < 0 {
			continue
		}
		after := text[idx+len(sep):]
		endIdx := strings.Index(after, "```")
		if endIdx < 0 {
			continue
		}
		raw := strings.TrimSpace(after[:endIdx])
		if tc, ok := unmarshalToolCall(raw); ok {
			return tc, true
		}
	}
	// 4. Bare JSON object anywhere in text
	if start := strings.Index(text, `{"tool"`); start >= 0 {
		// Find matching closing brace
		depth, end := 0, -1
		for i, ch := range text[start:] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					end = start + i + 1
					break
				}
			}
		}
		if end > start {
			if tc, ok := unmarshalToolCall(text[start:end]); ok {
				return tc, true
			}
		}
	}
	return toolCallJSON{}, false
}

// sanitizeToolArgs strips any embedded tool-call JSON from string arguments
// (e.g. a model that puts the next tool call inside write_file's "content").
func sanitizeToolArgs(tc toolCallJSON) toolCallJSON {
	for k, v := range tc.Args {
		s, ok := v.(string)
		if !ok {
			continue
		}
		// If the string value itself contains a tool call, truncate at that point.
		for _, marker := range []string{toolCallOpen, "```{\"tool\"", "```json\n{\"tool\""} {
			if idx := strings.Index(s, marker); idx >= 0 {
				tc.Args[k] = s[:idx]
			}
		}
	}
	return tc
}

func parseTaggedToolCall(text string) (toolCallJSON, bool) {
	start := strings.Index(text, toolCallOpen)
	if start < 0 {
		return toolCallJSON{}, false
	}
	inner := text[start+len(toolCallOpen):]
	end := strings.Index(inner, toolCallClose)
	if end < 0 {
		return toolCallJSON{}, false
	}
	return unmarshalToolCall(strings.TrimSpace(inner[:end]))
}

func unmarshalToolCall(raw string) (toolCallJSON, bool) {
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

	result := append([]ChatMessageWithTools(nil), msgs...)
	for i, m := range result {
		if m.Role == "system" {
			result[i].Content = m.Content + "\n\n" + addendum
			return result
		}
	}
	return append([]ChatMessageWithTools{{Role: "system", Content: addendum}}, result...)
}

// buildToolsAddendum returns the text block appended to the system prompt.
func buildToolsAddendum(tools []ToolDef) string {
	var sb strings.Builder
	sb.WriteString("## File Tools\n\n")
	sb.WriteString("When you need to perform a file operation, your ENTIRE response must be this block:\n\n")
	sb.WriteString(toolCallOpen + "\n")
	sb.WriteString("{\"tool\": \"TOOL_NAME\", \"args\": {\"param\": \"value\"}}\n")
	sb.WriteString(toolCallClose + "\n\n")
	sb.WriteString("After the tool runs you will see the result and must continue.\n")
	sb.WriteString("When ALL changes are applied, write a plain-text summary.\n\n")
	sb.WriteString("Example:\n")
	sb.WriteString("  → You want to create internal/foo.go:\n")
	sb.WriteString("  " + toolCallOpen + "{\"tool\":\"write_file\",\"args\":{\"path\":\"internal/foo.go\",\"content\":\"package foo\\n\"}}" + toolCallClose + "\n")
	sb.WriteString("  → Result: wrote internal/foo.go\n")
	sb.WriteString("  → You continue with the next tool call or write a summary.\n\n")
	sb.WriteString("Available tools:\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Function.Name, t.Function.Description))
		for pname, pschema := range t.Function.Parameters.Properties {
			req := ""
			for _, r := range t.Function.Parameters.Required {
				if r == pname {
					req = "*"
				}
			}
			sb.WriteString(fmt.Sprintf("  - `%s`%s (%s): %s\n", pname, req, pschema.Type, pschema.Description))
		}
	}
	sb.WriteString("\nRule: ONE tool call per response. No explanatory text before or after the tool call block.")
	return sb.String()
}

// looksLikePlan returns true when the model response sounds like a plan rather than action.
func looksLikePlan(text string) bool {
	lower := strings.ToLower(text)
	for _, p := range []string{
		"i will", "i'll", "i would", "i'm going to", "i plan",
		"next, i", "first, i", "let me", "proceed to",
		"will create", "will write", "will implement", "will add",
		"steps:", "step 1", "step 2",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
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
	return strings.Join(parts, ", ")
}
