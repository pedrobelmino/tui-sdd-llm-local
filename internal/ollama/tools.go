package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
	toolCallOpen       = "<tool_call>"
	toolCallClose      = "</tool_call>"
	maxToolIter        = 30 // default safeguard against infinite loops
	maxMalformedStreak = 2  // abort on 3rd malformed tool-call JSON
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
	loopLimit := maxToolIter
	if v := ctx.Value(ctxKeyToolLoopLimit{}); v != nil {
		if n, ok := v.(int); ok && n > 0 {
			loopLimit = n
		}
	}

	// Pre-load project layout and append to the last user message so the model
	// starts with directory context — no fake assistant/tool turn needed.
	greenfield := false
	rootListed := false
	layout := ""
	ldResult := ""
	if exec != nil && len(tools) > 0 {
		emit(onChunk, "📂 loading project layout…")
		ldResult = exec("list_dir", map[string]any{"path": "."})
		if ldResult == "" {
			ldResult = "(empty)"
		}
		emit(onChunk, "   %s\n", ldResult)
		rootListed = true

		layout := buildBootstrapLayout(exec, ldResult)
		greenfield = isGreenfieldLayout(ldResult)
		if greenfield {
			emit(onChunk, "   🌱 greenfield project — no source tree yet")
		} else if bootstrapSatisfiesInspection(ldResult, layout) {
			emit(onChunk, "   ✓ source tree pre-loaded from bootstrap")
		}

		// Find the last user message and append layout + clear instruction.
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == "user" {
				history[i].Content += "\n\n" + layout
				if greenfield {
					history[i].Content += "\n\nGREENFIELD PROJECT: only .specs/ exists — no source files yet. " +
						"Create files with write_file using paths from spec/design/tasks. " +
						"Do NOT read_file paths that do not exist yet. Do NOT repeat list_dir(\".\")."
				} else {
					history[i].Content += "\n\nNOTE: All spec/design/tasks files for the feature are already provided above in the message. " +
						"Do NOT read them via read_file — use tools only for SOURCE CODE files."
				}
				history[i].Content += "\n\nStart making file changes now — respond with a <tool_call> block."
				break
			}
		}
	}

	lastToolSig := ""
	lastFailedSig := ""
	sameFailStreak := 0
	malformedStreak := 0
	stallScore := 0
	consecutiveCreateDirs := 0
	filesWritten := 0
	lastCreateDirPath := ""
	lastFailedReadPath := ""
	inspectedSource := greenfield || bootstrapSatisfiesInspection(ldResult, layout)
	guardExistingFirst := len(tools) > 0 && !greenfield
	const maxSameFailStreak = 2 // abort on 3rd identical failure (fail fast)
	minFileWrites := 1
	if v := ctx.Value(ctxKeyMinFileWrites{}); v != nil {
		if n, ok := v.(int); ok && n > 0 {
			minFileWrites = n
		}
	}

	for iter := 0; iter < loopLimit; iter++ {
		// Convert to plain ChatMessages for ChatStream.
		plain := make([]ChatMessage, len(history))
		for i, h := range history {
			plain[i] = ChatMessage{Role: h.Role, Content: h.Content}
		}

		// Stream the response into buf; suppress raw model text from the visible log
		// (it's mostly the tool-call JSON or an intermediate plan — not user-friendly).
		var buf strings.Builder
		emit(onChunk, "🤔 thinking… (turn %d/%d)", iter+1, loopLimit)
		stopHeartbeat := make(chan struct{})
		heartbeatDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(4 * time.Second)
			defer func() {
				ticker.Stop()
				close(heartbeatDone)
			}()
			elapsed := 0
			for {
				select {
				case <-stopHeartbeat:
					return
				case <-ticker.C:
					elapsed += 4
					emit(onChunk, "… still working (%ds)", elapsed)
				}
			}
		}()
		_, usage, err := c.ChatStream(ctx, ChatRequest{
			Model:    model,
			Messages: plain,
		}, func(chunk string) { buf.WriteString(chunk) })
		close(stopHeartbeat)
		<-heartbeatDone
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
			// Model attempted a tool call but emitted malformed JSON/code block.
			if looksLikeMalformedToolCall(response) {
				malformedStreak++
				emit(onChunk, "⚠ malformed tool-call JSON (attempt %d) — retrying…", malformedStreak)
				if malformedStreak > maxMalformedStreak {
					snippet := truncateForLog(response, 180)
					emit(onChunk, "❌ model stuck on invalid tool-call JSON after %d attempts", malformedStreak)
					if snippet != "" {
						emit(onChunk, "   model output: %s", snippet)
					}
					emit(onChunk, "Hint: escape newlines as \\n inside JSON, or run one task: tsll run <feature> <task>")
					return "", totalUsage, fmt.Errorf("malformed tool-call JSON loop (%d attempts)", malformedStreak)
				}
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: malformedToolCallNudge(malformedStreak)},
				)
				continue
			}
			malformedStreak = 0
			// If iter==0 and the model wrote a plan instead of a tool call, nudge once.
			if iter == 0 && looksLikePlan(response) {
				emit(onChunk, "⚠  model wrote a plan instead of calling tools — retrying…")
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: "Do not describe. Respond with a <tool_call> block to start making file changes."},
				)
				continue
			}
			if looksLikePrematureDone(response, filesWritten, minFileWrites) {
				emit(onChunk, "⚠ premature completion — spec/tasks require more file changes")
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: prematureDoneNudge(filesWritten, minFileWrites)},
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
		tc = normalizeToolArgs(sanitizeToolArgs(tc))

		// Execute the tool and show it in the log.
		emit(onChunk, "🔧 %s", formatToolInvocation(tc))

		if reason := validateToolCallArgs(tc); reason != "" {
			if abort, err := recordToolFailure(&lastFailedSig, &sameFailStreak, &stallScore, &lastToolSig, &history, tc, response, reason, maxSameFailStreak, onChunk); abort {
				return "", totalUsage, err
			}
			continue
		}

		if blocked, reason := preflightToolBlock(tc, loopPreflightState{
			lastCreateDirPath:     lastCreateDirPath,
			consecutiveCreateDirs: consecutiveCreateDirs,
			filesWritten:          filesWritten,
			greenfield:            greenfield,
			rootListed:            rootListed,
			lastFailedReadPath:    lastFailedReadPath,
			lastToolSig:           lastToolSig,
			lastFailedSig:         lastFailedSig,
		}); blocked {
			if abort, err := recordToolFailure(&lastFailedSig, &sameFailStreak, &stallScore, &lastToolSig, &history, tc, response, reason, maxSameFailStreak, onChunk); abort {
				return "", totalUsage, err
			}
			continue
		}

		autoInspectNote := ""
		if guardExistingFirst && requiresSourceInspection(tc) && !inspectedSource {
			inspectPath := sourceInspectPathFor(toolPath(tc))
			emit(onChunk, "📂 auto-inspecting %s before write…", inspectPath)
			ld := exec("list_dir", map[string]any{"path": inspectPath})
			if isToolFailure(ld) {
				reason := "inspect existing source first: call list_dir/read_file under " + strings.Join(sourceTreeRoots(), ", ") + " before write_file"
				if abort, err := recordToolFailure(&lastFailedSig, &sameFailStreak, &stallScore, &lastToolSig, &history, tc, response, reason, maxSameFailStreak, onChunk); abort {
					return "", totalUsage, err
				}
				continue
			}
			emit(onChunk, "   ✓ %s", ld)
			inspectedSource = true
			autoInspectNote = "Auto-inspected list_dir(" + inspectPath + "):\n" + ld + "\n\n"
		}

		result := exec(tc.Tool, tc.Args)
		if marksSourceInspection(tc) && !isToolFailure(result) {
			inspectedSource = true
		}
		if tc.Tool == "read_file" && isToolFailure(result) && strings.Contains(result, "no such file") {
			lastFailedReadPath = toolPath(tc)
		} else if tc.Tool == "write_file" || tc.Tool == "edit_file" {
			lastFailedReadPath = ""
		}
		if tc.Tool == "list_dir" && toolPath(tc) == "." {
			rootListed = true
		}
		if tc.Tool == "write_file" || tc.Tool == "edit_file" {
			if !isToolFailure(result) {
				filesWritten++
				consecutiveCreateDirs = 0
				stallScore = 0
				sameFailStreak = 0
				lastFailedSig = ""
			}
		}
		if tc.Tool == "create_dir" {
			lastCreateDirPath = toolPath(tc)
			consecutiveCreateDirs++
		}
		sig := toolSignature(tc)

		var continuation string
		if isToolFailure(result) {
			reason := strings.TrimPrefix(result, "ERROR: ")
			if abort, err := recordToolFailure(&lastFailedSig, &sameFailStreak, &stallScore, &lastToolSig, &history, tc, response, reason, maxSameFailStreak, onChunk); abort {
				return "", totalUsage, err
			}
			continue
		} else {
			emit(onChunk, "   ✓ %s", result)
			continuation = autoInspectNote + buildSuccessContinuation(tc, result, greenfield)
			sameFailStreak = 0
			lastFailedSig = ""
			if tc.Tool == "create_dir" || tc.Tool == "list_dir" {
				stallScore++
			}
		}
		lastToolSig = sig

		malformedStreak = 0

		// Append assistant response + tool result and loop.
		history = append(history,
			ChatMessageWithTools{Role: "assistant", Content: response},
			ChatMessageWithTools{Role: "user", Content: continuation},
		)
	}

	emit(onChunk, "✗ tool-calling loop exceeded %d iterations", loopLimit)
	emit(onChunk, "Hint: model may be stuck on empty dirs or directory read_file. Split into smaller tasks (tsll run) or retry implement.")
	return "", totalUsage, fmt.Errorf("tool-calling loop exceeded %d iterations", loopLimit)
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
		if idx := strings.Index(raw, `{`); idx >= 0 {
			if balanced, ok := extractBraceBalancedJSON(raw, idx); ok {
				if tc, ok := unmarshalToolCall(balanced); ok {
					return tc, true
				}
			}
		}
	}
	// 4. Bare JSON object anywhere in text
	for _, needle := range []string{`{"tool"`, `{ "tool"`} {
		if start := strings.Index(text, needle); start >= 0 {
			if raw, ok := extractBraceBalancedJSON(text, start); ok {
				if tc, ok := unmarshalToolCall(raw); ok {
					return tc, true
				}
			}
		}
	}
	// 5. Lenient field extraction (models often break JSON with literal newlines in content)
	if tc, ok := parseToolCallRelaxed(text); ok {
		return tc, true
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

// normalizeToolArgs maps common alternate arg names and trims content fields.
func normalizeToolArgs(tc toolCallJSON) toolCallJSON {
	switch tc.Tool {
	case "write_file":
		if _, ok := tc.Args["content"].(string); !ok {
			for _, alt := range []string{"body", "text", "data", "contents"} {
				if v, ok := tc.Args[alt].(string); ok && strings.TrimSpace(v) != "" {
					tc.Args["content"] = v
					break
				}
			}
		}
	case "edit_file":
		if _, ok := tc.Args["old_content"].(string); !ok {
			if v, ok := tc.Args["old"].(string); ok {
				tc.Args["old_content"] = v
			}
		}
		if _, ok := tc.Args["new_content"].(string); !ok {
			if v, ok := tc.Args["new"].(string); ok {
				tc.Args["new_content"] = v
			}
		}
	}
	return tc
}

func validateToolCallArgs(tc toolCallJSON) string {
	switch tc.Tool {
	case "write_file":
		path := toolPath(tc)
		if path == "" {
			return "write_file requires path"
		}
		content, ok := tc.Args["content"].(string)
		if !ok || strings.TrimSpace(content) == "" {
			return "write_file content is empty for " + path + " — include full file body in args.content. " + writeFileContentHint(path)
		}
		if len(strings.TrimSpace(content)) < 3 {
			return "write_file content too short for " + path + " — provide complete file content. " + writeFileContentHint(path)
		}
	case "edit_file":
		if toolPath(tc) == "" {
			return "edit_file requires path"
		}
		old, okOld := tc.Args["old_content"].(string)
		newC, okNew := tc.Args["new_content"].(string)
		if !okOld || strings.TrimSpace(old) == "" {
			return "edit_file requires non-empty old_content"
		}
		if !okNew {
			return "edit_file requires new_content"
		}
		_ = newC
	}
	return ""
}

func writeFileContentHint(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".css"):
		return `Example content: "/* landing */\nbody { margin: 0; font-family: sans-serif; }\n.header { padding: 1rem; }\n"`
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".tmpl"):
		return `Example content: "<!DOCTYPE html>\n<html><body><h1>Title</h1></body></html>\n"`
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".ts"):
		return `Example content: "export function init() { console.log('ok'); }\n"`
	case strings.HasSuffix(lower, ".go"):
		return `Example content: "package app\n\nfunc Run() {}\n"`
	default:
		return "Put the entire file contents in the JSON content field as a string."
	}
}

// recordToolFailure logs a failed tool attempt, updates stall counters, and aborts when stuck.
// Returns (true, err) when the loop should stop immediately.
func recordToolFailure(
	lastFailedSig *string,
	sameFailStreak *int,
	stallScore *int,
	lastToolSig *string,
	history *[]ChatMessageWithTools,
	tc toolCallJSON,
	response string,
	reason string,
	maxSameFailStreak int,
	onChunk func(string),
) (bool, error) {
	emit(onChunk, "❌ %s", reason)
	failSig := toolFailureSignature(tc)
	if failSig == *lastFailedSig {
		*sameFailStreak++
	} else {
		*lastFailedSig = failSig
		*sameFailStreak = 1
	}
	*stallScore++
	continuation := "Tool result (FAILED):\nERROR: " + reason + "\n\n" + failureRecoveryHint(tc, reason)
	if *sameFailStreak >= 2 {
		continuation += "\n\nYou already failed a similar call — change file path or provide full non-empty content."
	}
	if *stallScore >= 2 {
		continuation += "\n\n" + stallRecoveryHint(*stallScore)
	}
	*history = append(*history,
		ChatMessageWithTools{Role: "assistant", Content: response},
		ChatMessageWithTools{Role: "user", Content: continuation},
	)
	*lastToolSig = toolSignature(tc)
	if *sameFailStreak > maxSameFailStreak {
		msg := fmt.Sprintf("stuck: repeated failing tool call (%s)", formatToolInvocation(tc))
		emit(onChunk, "✗ %s", msg)
		emit(onChunk, "Hint: split work with tsll run <feature> <task>, or retry with a smaller scope.")
		return true, fmt.Errorf("%s", msg)
	}
	return false, nil
}

func failureRecoveryHint(tc toolCallJSON, reason string) string {
	base := "The tool failed. Do NOT retry the same call blindly."
	if tc.Tool == "write_file" && strings.Contains(reason, "empty") {
		return base + "\n- write_file MUST include args.content with the full file text (not empty)." +
			"\n- If this file is hard, write a different required file first, then return to this one." +
			"\n- " + writeFileContentHint(toolPath(tc))
	}
	return base +
		"\n- If path is missing: use write_file to create the file (greenfield) or list_dir on the parent directory." +
		"\n- If path is a directory: call list_dir(path=<that directory>) instead of read_file." +
		"\n- write_file auto-creates parent dirs — skip create_dir unless truly needed."
}

func writeFileContentLen(tc toolCallJSON) int {
	c, ok := tc.Args["content"].(string)
	if !ok {
		return -1
	}
	return len(strings.TrimSpace(c))
}

func toolFailureSignature(tc toolCallJSON) string {
	if tc.Tool == "write_file" {
		return fmt.Sprintf("write_file|%s|len=%d", toolPath(tc), writeFileContentLen(tc))
	}
	return toolSignature(tc)
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
	inner = strings.TrimSpace(inner[:end])
	if tc, ok := unmarshalToolCall(inner); ok {
		return tc, true
	}
	if idx := strings.Index(inner, `{`); idx >= 0 {
		if balanced, ok := extractBraceBalancedJSON(inner, idx); ok {
			if tc, ok := unmarshalToolCall(balanced); ok {
				return tc, true
			}
		}
	}
	return toolCallJSON{}, false
}

// extractBraceBalancedJSON returns a JSON object starting at start, respecting quoted strings.
func extractBraceBalancedJSON(text string, start int) (string, bool) {
	if start < 0 || start >= len(text) || text[start] != '{' {
		return "", false
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if ch == '\\' {
				escape = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1], true
			}
		}
	}
	return "", false
}

func parseToolCallRelaxed(text string) (toolCallJSON, bool) {
	if start := strings.Index(text, toolCallOpen); start >= 0 {
		inner := text[start+len(toolCallOpen):]
		if end := strings.Index(inner, toolCallClose); end >= 0 {
			if tc, ok := lenientParseToolJSON(inner[:end]); ok {
				return tc, true
			}
		}
	}
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
		if tc, ok := lenientParseToolJSON(after[:endIdx]); ok {
			return tc, true
		}
	}
	if strings.Contains(text, `"tool"`) {
		if tc, ok := lenientParseToolJSON(text); ok {
			return tc, true
		}
	}
	return toolCallJSON{}, false
}

func lenientParseToolJSON(s string) (toolCallJSON, bool) {
	tool, ok := extractJSONStringField(s, "tool")
	if !ok || tool == "" {
		return toolCallJSON{}, false
	}
	tc := toolCallJSON{Tool: tool, Args: map[string]any{}}
	path, _ := extractJSONStringFieldLenient(s, "path")
	if path != "" {
		tc.Args["path"] = path
	}
	switch tool {
	case "write_file":
		content, ok := extractJSONStringFieldLenient(s, "content")
		if !ok || strings.TrimSpace(content) == "" {
			return toolCallJSON{}, false
		}
		tc.Args["content"] = content
	case "read_file", "list_dir", "create_dir", "delete_file":
		if path == "" {
			if p, ok := extractJSONStringFieldLenient(s, "path"); ok && p != "" {
				tc.Args["path"] = p
			} else {
				tc.Args["path"] = "."
			}
		}
	case "edit_file":
		old, _ := extractJSONStringFieldLenient(s, "old_content")
		newC, _ := extractJSONStringFieldLenient(s, "new_content")
		if old != "" {
			tc.Args["old_content"] = old
		}
		if newC != "" {
			tc.Args["new_content"] = newC
		}
	default:
		return toolCallJSON{}, false
	}
	return tc, true
}

func indexOfJSONField(s, field string) int {
	key := `"` + field + `"`
	return strings.Index(s, key)
}

func extractJSONStringField(s, field string) (string, bool) {
	return extractJSONStringFieldImpl(s, field, false)
}

func extractJSONStringFieldLenient(s, field string) (string, bool) {
	return extractJSONStringFieldImpl(s, field, true)
}

func extractJSONStringFieldImpl(s, field string, lenient bool) (string, bool) {
	idx := indexOfJSONField(s, field)
	if idx < 0 {
		return "", false
	}
	rest := s[idx+len(`"`+field+`"`):]
	rest = strings.TrimLeft(rest, " \t\n\r:")
	rest = strings.TrimLeft(rest, " \t\n\r")
	if len(rest) == 0 || rest[0] != '"' {
		return "", false
	}
	rest = rest[1:]
	var b strings.Builder
	for i := 0; i < len(rest); i++ {
		ch := rest[i]
		if ch == '\\' && i+1 < len(rest) {
			b.WriteByte(ch)
			i++
			b.WriteByte(rest[i])
			continue
		}
		if ch == '"' {
			j := i + 1
			for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t' || rest[j] == '\n' || rest[j] == '\r') {
				j++
			}
			if j >= len(rest) || rest[j] == ',' || rest[j] == '}' {
				return b.String(), true
			}
			if !lenient {
				return "", false
			}
		}
		b.WriteByte(ch)
	}
	if lenient && b.Len() > 0 {
		return b.String(), true
	}
	return "", false
}

func malformedToolCallNudge(streak int) string {
	example := toolCallOpen + `{"tool":"list_dir","args":{"path":"app"}}` + toolCallClose
	if streak > 1 {
		example = toolCallOpen + `{"tool":"write_file","args":{"path":"app/x.go","content":"package app\n"}}` + toolCallClose
		return "STILL INVALID. Reply with ONLY one tool_call block. JSON must be one object — use \\n for newlines, never literal line breaks inside the JSON.\nExample:\n" + example
	}
	return "Your tool call JSON was invalid. Reply with ONLY this block (valid JSON, one line):\n" + example
}

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
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
	sb.WriteString("CRITICAL: write_file args.content MUST be a non-empty string with the FULL file body inline.\n")
	sb.WriteString("Never call write_file with empty content or 0 bytes. For CSS include actual rules (e.g. body{margin:0}).\n")
	sb.WriteString("JSON strings must use \\n for newlines — never put literal line breaks inside the JSON object.\n\n")
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
	sb.WriteString("\nAnti-patterns (NEVER):\n")
	sb.WriteString("- read_file on a directory — use list_dir instead\n")
	sb.WriteString("- create_dir for placeholder paths (foo, bar, baz) — implement real spec paths\n")
	sb.WriteString("- create_dir before write_file — write_file creates parent directories\n")
	sb.WriteString("- read_file on empty dirs you just created — write the actual source file next\n")
	sb.WriteString("\nRule: ONE tool call per response. No explanatory text before or after the tool call block.")
	return sb.String()
}

// looksLikePlan returns true when the model response sounds like a plan rather than action.
func looksLikePrematureDone(text string, filesWritten, minWrites int) bool {
	if filesWritten >= minWrites {
		return false
	}
	t := strings.TrimSpace(text)
	lower := strings.ToLower(t)
	if strings.HasPrefix(t, "{") && strings.Contains(lower, "summary") {
		return filesWritten < minWrites || minWrites > 1
	}
	if filesWritten == 0 {
		return !strings.Contains(t, toolCallOpen)
	}
	return filesWritten < minWrites && (strings.HasPrefix(t, "{") || len(t) < 400)
}

func prematureDoneNudge(filesWritten, minWrites int) string {
	return fmt.Sprintf(
		"You tried to finish too early (%d file change(s), need at least %d). "+
			"The task is NOT complete. Respond with a <tool_call> to implement the next required file per spec.md, design.md, and tasks.md. "+
			"Do NOT output JSON summaries until all required code exists.",
		filesWritten, minWrites,
	)
}

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
type ctxKeyToolLoopLimit struct{}
type ctxKeyMinFileWrites struct{}

// WithModel attaches the model name to a context for ChatWithTools.
func WithModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, ctxKeyModel{}, model)
}

// WithToolLoopLimit sets max tool-calling turns for ChatWithTools.
// Useful for fast mode to fail-fast on loops and reduce latency.
func WithToolLoopLimit(ctx context.Context, limit int) context.Context {
	if limit <= 0 {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyToolLoopLimit{}, limit)
}

// WithMinFileWrites requires at least n successful write_file/edit_file calls
// before accepting a final non-tool summary.
func WithMinFileWrites(ctx context.Context, n int) context.Context {
	if n <= 0 {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyMinFileWrites{}, n)
}

func formatToolInvocation(tc toolCallJSON) string {
	switch tc.Tool {
	case "write_file":
		n := 0
		if c, ok := tc.Args["content"].(string); ok {
			n = len(c)
		}
		return fmt.Sprintf("write_file(path=%s, %d bytes)", toolPath(tc), n)
	case "edit_file":
		return fmt.Sprintf("edit_file(path=%s)", toolPath(tc))
	case "read_file", "list_dir", "create_dir", "delete_file":
		return fmt.Sprintf("%s(path=%s)", tc.Tool, toolPath(tc))
	default:
		return fmt.Sprintf("%s(%s)", tc.Tool, formatArgs(tc.Args))
	}
}

func isToolFailure(result string) bool {
	return strings.HasPrefix(result, "ERROR")
}

func isUnproductiveFailure(tc toolCallJSON, result string) bool {
	if tc.Tool == "read_file" && strings.Contains(result, "is a directory") {
		return true
	}
	return tc.Tool == "create_dir"
}

type loopPreflightState struct {
	lastCreateDirPath     string
	consecutiveCreateDirs int
	filesWritten          int
	greenfield            bool
	rootListed            bool
	lastFailedReadPath    string
	lastToolSig           string
	lastFailedSig         string
}

func preflightToolBlock(tc toolCallJSON, st loopPreflightState) (bool, string) {
	path := toolPath(tc)
	sig := toolSignature(tc)
	failSig := toolFailureSignature(tc)

	if st.lastFailedSig != "" && failSig == st.lastFailedSig {
		return true, "blocked repeat of failed call " + formatToolInvocation(tc) + " — fix content/path or work on a different file"
	}
	if tc.Tool == "list_dir" && path == "." && st.rootListed {
		return true, "root layout was already loaded — inspect a specific directory or write_file the next source file"
	}
	if tc.Tool == "read_file" && st.lastFailedReadPath != "" && path == st.lastFailedReadPath {
		return true, "file " + path + " does not exist — use write_file to create it with full content from the spec"
	}
	if tc.Tool == "read_file" && st.greenfield && strings.Contains(path, "/") {
		// Greenfield: reading deep paths that were never created is almost always wrong.
		parent := path
		if i := strings.LastIndex(path, "/"); i > 0 {
			parent = path[:i]
		}
		if st.lastCreateDirPath != "" && parent == st.lastCreateDirPath && st.filesWritten == 0 {
			return true, "directory " + parent + " is empty — write_file " + path + " now instead of read_file"
		}
	}
	if tc.Tool == "read_file" && st.lastCreateDirPath != "" && path == st.lastCreateDirPath {
		return true, "you just created " + path + " as a directory — do not read_file on it; write a real source file or list_dir to explore"
	}
	if sig == st.lastToolSig && (tc.Tool == "list_dir" || tc.Tool == "read_file") {
		return true, "you already ran " + formatToolInvocation(tc) + " — choose a different next step (write_file or edit_file)"
	}
	if tc.Tool == "create_dir" && st.consecutiveCreateDirs >= 2 && st.filesWritten == 0 {
		return true, "stop creating directories without writing files — use write_file(path=..., content=...) for source files (parent dirs are created automatically)"
	}
	if tc.Tool == "create_dir" && isPlaceholderPath(path) && st.filesWritten == 0 {
		return true, "do not create placeholder directory " + path + " — implement paths from spec/design/tasks"
	}
	return false, ""
}

func isPlaceholderPath(path string) bool {
	base := strings.ToLower(strings.Trim(path, "/"))
	for _, part := range strings.Split(base, "/") {
		switch part {
		case "foo", "bar", "baz", "test", "tmp", "example", "sample", "placeholder":
			return true
		}
	}
	return false
}

func buildBootstrapLayout(exec ToolExecutor, rootListing string) string {
	var b strings.Builder
	b.WriteString("Project root layout (list_dir \".\"):\n")
	b.WriteString(rootListing)
	for _, dir := range sourceTreeRoots() {
		if !listingContainsDir(rootListing, dir) {
			continue
		}
		sub := exec("list_dir", map[string]any{"path": dir})
		if sub == "" {
			sub = "(empty)"
		}
		b.WriteString("\n\n")
		b.WriteString(dir)
		b.WriteString("/:\n")
		b.WriteString(sub)
	}
	return b.String()
}

func listingContainsDir(listing, dir string) bool {
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		if line == dir || line == dir+"/" {
			return true
		}
	}
	return false
}

func isGreenfieldLayout(rootListing string) bool {
	for _, dir := range []string{"internal", "src", "cmd", "pkg", "lib", "app"} {
		if listingContainsDir(rootListing, dir) {
			return false
		}
	}
	return true
}

func buildSuccessContinuation(tc toolCallJSON, result string, greenfield bool) string {
	switch tc.Tool {
	case "list_dir":
		if greenfield {
			return "Tool result:\n" + result + "\n\nGREENFIELD: write_file the source files named in the spec — do not keep listing directories."
		}
		return "Tool result:\n" + result + "\n\nPick a relevant source file from this listing and read_file it, or write_file the next required file."
	case "read_file":
		return "Tool result:\n" + result + "\n\nApply the required change with edit_file or write_file, then continue with the next file."
	case "create_dir":
		return "Tool result:\n" + result + "\n\nDirectory exists. Next: write_file the actual source file — do not read_file the empty directory."
	case "write_file", "edit_file":
		return "Tool result:\n" + result + "\n\nGood. Continue with the next required file change, or write a plain-text summary when done."
	default:
		return "Tool result:\n" + result + "\n\nContinue with the next required change."
	}
}

func stallRecoveryHint(stallScore int) string {
	if stallScore < 2 {
		return ""
	}
	msg := "STUCK LOOP DETECTED: stop repeating list_dir/read_file. "
	msg += "Re-read the spec paths above, then write_file or edit_file the concrete files required."
	return msg
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

func toolSignature(tc toolCallJSON) string {
	return tc.Tool + "|" + formatArgs(tc.Args)
}

func toolPath(tc toolCallJSON) string {
	v, ok := tc.Args["path"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func sourceTreeRoots() []string {
	return []string{"internal", "src", "app", "cmd", "pkg", "web", "frontend"}
}

func isSourceTreePath(p string) bool {
	p = strings.ToLower(strings.Trim(p, "/"))
	if p == "" || p == "." || strings.HasPrefix(p, ".specs") {
		return false
	}
	for _, root := range sourceTreeRoots() {
		if p == root || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}

func sourceInspectPathFor(filePath string) string {
	filePath = strings.Trim(strings.TrimSpace(filePath), "/")
	lower := strings.ToLower(filePath)
	for _, root := range sourceTreeRoots() {
		if lower == root {
			return root
		}
		if strings.HasPrefix(lower, root+"/") {
			return root
		}
	}
	if i := strings.Index(filePath, "/"); i > 0 {
		return filePath[:i]
	}
	if filePath != "" {
		return filePath
	}
	return "."
}

func bootstrapSatisfiesInspection(rootListing, layout string) bool {
	if isGreenfieldLayout(rootListing) {
		return true
	}
	for _, dir := range sourceTreeRoots() {
		if listingContainsDir(rootListing, dir) && strings.Contains(layout, dir+"/:\n") {
			return true
		}
	}
	return false
}

func marksSourceInspection(tc toolCallJSON) bool {
	if tc.Tool != "read_file" && tc.Tool != "list_dir" {
		return false
	}
	return isSourceTreePath(toolPath(tc))
}

func requiresSourceInspection(tc toolCallJSON) bool {
	switch tc.Tool {
	case "write_file", "create_dir", "edit_file", "delete_file":
		return true
	default:
		return false
	}
}

func looksLikeMalformedToolCall(text string) bool {
	if strings.Contains(text, toolCallOpen) {
		return true
	}
	l := strings.ToLower(text)
	if !strings.Contains(l, `"tool"`) {
		return false
	}
	for _, hint := range []string{`"args"`, "write_file", "read_file", "list_dir", "create_dir", "edit_file", "delete_file"} {
		if strings.Contains(l, hint) {
			return true
		}
	}
	return strings.Contains(text, "```") && strings.Contains(l, `"tool"`)
}
