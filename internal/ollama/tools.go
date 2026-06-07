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
	maxToolIter        = 100 // hard cap against infinite loops
	maxMalformedStreak = 2   // abort on 3rd malformed tool-call JSON
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
	initialLoopLimit := loopLimit

	taskPlanModeEarly := false
	if v := ctx.Value(ctxKeyTaskPlanMode{}); v != nil {
		taskPlanModeEarly, _ = v.(bool)
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
			emit(onChunk, "   🌱 greenfield project — no source tree yet; model must emit <file_plan> from tasks.md, spec.md and design.md")
		} else if bootstrapSatisfiesInspection(ldResult, layout) {
			emit(onChunk, "   ✓ source tree pre-loaded from bootstrap")
		}

		// Find the last user message and append layout + clear instruction.
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == "user" {
				history[i].Content += "\n\n" + layout
				if greenfield {
					history[i].Content += "\n\nGREENFIELD PROJECT: only .specs/ exists — no source files yet. " +
						"Use write_file directly — write_file creates parent directories. " +
						"Do NOT read_file or list_dir paths that do not exist yet. Do NOT repeat list_dir(\".\")."
				} else {
					history[i].Content += "\n\nNOTE: All spec/design/tasks files for the feature are already provided above in the message. " +
						"Do NOT read them via read_file — use tools only for SOURCE CODE files."
				}
				if taskPlanModeEarly {
					history[i].Content += "\n\nStart with a <file_plan> block listing every source file for this task (one path per line), derived from tasks.md, spec.md and design.md above."
					history[i].Content += "\nFor many files (scaffolding), prefer one-shot <task_plan>{\"files\":[...]}</task_plan> with all contents."
				} else {
					history[i].Content += "\n\nStart making file changes now — respond with a <tool_call> block."
				}
				break
			}
		}
	}

	lastToolSig := ""
	lastFailedSig := ""
	sameFailStreak := 0
	malformedStreak := 0
	planNudgeStreak := 0
	planNoWriteStreak := 0
	stallScore := 0
	pathsWritten := map[string]bool{}
	pathsRead := map[string]bool{}
	var filePlan []string
	taskPlanMode := false
	planReceived := false
	if v := ctx.Value(ctxKeyTaskPlanMode{}); v != nil {
		taskPlanMode, _ = v.(bool)
	}
	singleTouch := false
	if v := ctx.Value(ctxKeySingleTouch{}); v != nil {
		singleTouch, _ = v.(bool)
	}
	maxPlanFiles := 0
	if v := ctx.Value(ctxKeyMaxPlanFiles{}); v != nil {
		maxPlanFiles, _ = v.(int)
	}
	var focusedAllow []string
	if v := ctx.Value(ctxKeyFocusedAllowlist{}); v != nil {
		focusedAllow, _ = v.([]string)
	}
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
	if greenfield && minFileWrites < 4 {
		minFileWrites = 4
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

		// One-shot batch plan: write every file once without further LLM turns.
		if batch, ok := parseBatchTaskPlan(response); ok {
			if maxPlanFiles > 0 && len(batch.Files) > maxPlanFiles {
				emit(onChunk, "⚠ batch plan has %d files (max %d) — rejected", len(batch.Files), maxPlanFiles)
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: filePlanTooLargeNudge(maxPlanFiles)},
				)
				continue
			}
			batchPaths := make([]string, len(batch.Files))
			for i, f := range batch.Files {
				batchPaths[i] = f.Path
			}
			if bad := offScopePlanPaths(batchPaths, focusedAllow); len(bad) > 0 {
				emit(onChunk, "⚠ batch plan off-scope: %s", strings.Join(bad, ", "))
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: filePlanOffScopeNudge(bad, focusedAllow)},
				)
				continue
			}
			emit(onChunk, "📋 batch plan: %d file(s) — writing each once…", len(batch.Files))
			var summary strings.Builder
			for _, f := range batch.Files {
				if pathsWritten[f.Path] {
					emit(onChunk, "❌ duplicate path in batch plan: %s", f.Path)
					return "", totalUsage, fmt.Errorf("duplicate path in batch plan: %s", f.Path)
				}
				emit(onChunk, "🔧 write_file(path=%s, %d bytes)", f.Path, len(f.Content))
				result := exec("write_file", map[string]any{"path": f.Path, "content": f.Content})
				if isToolFailure(result) {
					emit(onChunk, "   ❌ %s", strings.TrimPrefix(result, "ERROR: "))
					return "", totalUsage, fmt.Errorf("batch plan write failed for %s: %s", f.Path, result)
				}
				emit(onChunk, "   ✓ %s", result)
				pathsWritten[f.Path] = true
				filesWritten++
				summary.WriteString(f.Path)
				summary.WriteString(" ")
			}
			emit(onChunk, "✓ batch plan complete (%d files)", len(batch.Files))
			return "Batch plan executed: " + strings.TrimSpace(summary.String()), totalUsage, nil
		}

		// Check for tool call tag in the response.
		tc, found := parseToolCall(response)
		if !found {
			// Path-only plan before any tool calls (per-task workflow).
			if taskPlanMode && !planReceived {
				if paths, ok := parseFilePlan(response); ok {
					// Self-heal: drop off-scope paths instead of rejecting the whole plan.
					if len(focusedAllow) > 0 {
						if kept := dropOffScopePaths(paths, focusedAllow); len(kept) > 0 && len(kept) < len(paths) {
							emit(onChunk, "   ✂ dropped %d off-scope path(s) from plan", len(paths)-len(kept))
							paths = kept
						}
					}
					// Self-heal: cap oversized plans to the task limit.
					if maxPlanFiles > 0 && len(paths) > maxPlanFiles {
						emit(onChunk, "   ✂ trimmed plan from %d to %d path(s)", len(paths), maxPlanFiles)
						paths = paths[:maxPlanFiles]
					}
					if len(paths) == 0 {
						planNudgeStreak++
						emit(onChunk, "⚠ plan had no in-scope paths (attempt %d)", planNudgeStreak)
						history = append(history,
							ChatMessageWithTools{Role: "assistant", Content: response},
							ChatMessageWithTools{Role: "user", Content: filePlanNudge()},
						)
						continue
					}
					filePlan = paths
					planReceived = true
					planNudgeStreak = 0
					bumpLoopLimitForPlan(&loopLimit, len(paths))
					emit(onChunk, "📋 file plan (%d): %s", len(paths), strings.Join(paths, ", "))
					if loopLimit > initialLoopLimit {
						emit(onChunk, "   📎 turn budget raised to %d for planned files", loopLimit)
					}
					history = append(history,
						ChatMessageWithTools{Role: "assistant", Content: response},
						ChatMessageWithTools{Role: "user", Content: "Plan accepted. " + planWriteNudge(filePlan, pathsWritten)},
					)
					continue
				}
				planNudgeStreak++
				emit(onChunk, "⚠ file plan required before tools (attempt %d)…", planNudgeStreak)
				if planNudgeStreak <= 2 {
					history = append(history,
						ChatMessageWithTools{Role: "assistant", Content: response},
						ChatMessageWithTools{Role: "user", Content: filePlanNudge()},
					)
					continue
				}
				taskPlanMode = false
				emit(onChunk, "⚠ continuing without file plan — write each path at most once")
			}
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
			// If the model wrote a plan/description instead of a tool call, escalate nudge.
			if looksLikePlan(response) {
				planNoWriteStreak++
				emit(onChunk, "⚠ model described instead of calling tools (attempt %d)…", planNoWriteStreak)
				var nudge string
				if planNoWriteStreak >= 2 && planReceived && len(remainingPlanFiles(filePlan, pathsWritten)) > 0 {
					nudge = directWriteNudge(remainingPlanFiles(filePlan, pathsWritten)[0])
				} else {
					nudge = "Do not describe. Respond with ONLY a <tool_call> block to start making file changes."
				}
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: nudge},
				)
				continue
			}
			if planReceived && len(remainingPlanFiles(filePlan, pathsWritten)) > 0 {
				rem := remainingPlanFiles(filePlan, pathsWritten)
				planNoWriteStreak++
				emit(onChunk, "⚠ incomplete — %d planned file(s) not written yet (attempt %d)", len(rem), planNoWriteStreak)
				var nudge string
				if planNoWriteStreak >= 2 && len(rem) > 0 {
					nudge = directWriteNudge(rem[0])
				} else {
					nudge = planWriteNudge(filePlan, pathsWritten)
				}
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: nudge},
				)
				continue
			}
			// A completed file plan means the task is done — don't force extra files.
			planComplete := planReceived && len(filePlan) > 0 && len(remainingPlanFiles(filePlan, pathsWritten)) == 0
			if !planComplete && looksLikePrematureDone(response, filesWritten, minFileWrites) {
				emit(onChunk, "⚠ premature completion — spec/tasks require more file changes")
				history = append(history,
					ChatMessageWithTools{Role: "assistant", Content: response},
					ChatMessageWithTools{Role: "user", Content: prematureDoneNudge(filesWritten, minFileWrites, greenfield)},
				)
				continue
			}
			if planComplete {
				emit(onChunk, "✓ all %d planned files written — task complete", len(filePlan))
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

		// Soft <file_plan>: auto-amend in-scope paths; plan size capped by maxPlanFiles.
		if planReceived && len(filePlan) > 0 && (tc.Tool == "write_file" || tc.Tool == "edit_file") {
			if p := toolPath(tc); p != "" && !stringInSlice(p, filePlan) && pathMatchesFocusedAllowlist(p, focusedAllow) {
				capN := maxPlanFiles
				if capN <= 0 {
					capN = len(filePlan) + 6
				}
				if len(filePlan) < capN {
					filePlan = append(filePlan, p)
					emit(onChunk, "   ➕ %s added to file plan (extra/integration file)", p)
				}
			}
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
			singleTouch:           singleTouch,
			pathsWritten:          pathsWritten,
			pathsRead:             pathsRead,
			filePlan:              filePlan,
			planReceived:          planReceived,
			focusedAllow:          focusedAllow,
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
				planNoWriteStreak = 0
				pathsWritten[toolPath(tc)] = true
				bumpLoopLimitForProgress(&loopLimit, filesWritten, filePlan, pathsWritten)
				consecutiveCreateDirs = 0
				stallScore = 0
				sameFailStreak = 0
				lastFailedSig = ""
				// Auto-complete when every planned file is written — don't burn turns waiting for summary.
				if planReceived && len(filePlan) > 0 &&
					len(remainingPlanFiles(filePlan, pathsWritten)) == 0 &&
					filesWritten >= minFileWrites {
					var names []string
					for p := range pathsWritten {
						names = append(names, p)
					}
					summary := fmt.Sprintf("Task complete — wrote %d file(s): %s", filesWritten, strings.Join(names, ", "))
					emit(onChunk, "✓ all %d planned files written — task complete", len(filePlan))
					if onChunk != nil {
						onChunk(summary + "\n")
					}
					return summary, totalUsage, nil
				}
			}
		}
		if tc.Tool == "read_file" && !isToolFailure(result) {
			pathsRead[toolPath(tc)] = true
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
			continuation = autoInspectNote + buildSuccessContinuation(tc, result, greenfield, filePlan, pathsWritten, singleTouch)
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

	emit(onChunk, "✗ tool-calling loop exceeded %d iterations (%d file(s) written)", loopLimit, filesWritten)
	if filesWritten > 0 {
		emit(onChunk, "Hint: partial progress saved — retry the same task: tsll run <feature> <task>")
	} else {
		emit(onChunk, "Hint: model may be stuck on empty dirs or directory read_file. Split into smaller tasks (tsll run) or retry implement.")
	}
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
	if looksLikeTextSummary(text) {
		return true
	}
	t := strings.TrimSpace(text)
	lower := strings.ToLower(t)
	if strings.HasPrefix(t, "{") && strings.Contains(lower, "summary") {
		return true
	}
	if filesWritten == 0 {
		return !strings.Contains(t, toolCallOpen)
	}
	// Any non-tool prose before min writes met is premature.
	return !strings.Contains(t, toolCallOpen)
}

func looksLikeTextSummary(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(text, "```") {
		return true
	}
	for _, p := range []string{
		"next steps", "let me know", "created the following",
		"further assistance", "here is a summary", "summary:",
		"implementation complete", "basic structure", "if you need",
		"do not describe", "the following source files",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func prematureDoneNudge(filesWritten, minWrites int, greenfield bool) string {
	msg := fmt.Sprintf(
		"You tried to finish too early (%d file change(s), need at least %d). "+
			"The task is NOT complete. Respond with a <tool_call> to implement the next required file per spec.md, design.md, and tasks.md. "+
			"Do NOT write prose summaries or markdown until all required code exists.",
		filesWritten, minWrites,
	)
	if greenfield {
		msg += " GREENFIELD: create the main entry point, components, and wiring — not just one stub file."
	}
	if filesWritten > 0 && filesWritten < minWrites {
		msg += " Start with <file_plan> if you have not listed all paths yet."
	}
	return msg
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
type ctxKeyTaskPlanMode struct{}
type ctxKeySingleTouch struct{}
type ctxKeyMaxPlanFiles struct{}
type ctxKeyFocusedAllowlist struct{}

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

// WithTaskPlanMode requires a <file_plan> listing all paths before write_file calls.
func WithTaskPlanMode(ctx context.Context, enabled bool) context.Context {
	if !enabled {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyTaskPlanMode{}, true)
}

// WithSingleTouch blocks second read/write/edit on the same path within one task loop.
func WithSingleTouch(ctx context.Context, enabled bool) context.Context {
	if !enabled {
		return ctx
	}
	return context.WithValue(ctx, ctxKeySingleTouch{}, true)
}

// WithMaxPlanFiles caps paths in <file_plan> for focused tasks.
func WithMaxPlanFiles(ctx context.Context, n int) context.Context {
	if n <= 0 {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyMaxPlanFiles{}, n)
}

// WithFocusedAllowlist restricts write paths to fragments matching the task (e.g. header).
func WithFocusedAllowlist(ctx context.Context, fragments []string) context.Context {
	if len(fragments) == 0 {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyFocusedAllowlist{}, fragments)
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
	singleTouch           bool
	pathsWritten          map[string]bool
	pathsRead             map[string]bool
	filePlan              []string
	planReceived          bool
	focusedAllow          []string
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
	normPath := strings.ReplaceAll(strings.Trim(path, "/"), "\\", "/")
	if tc.Tool == "write_file" || tc.Tool == "edit_file" || tc.Tool == "delete_file" || tc.Tool == "create_dir" {
		if normPath == ".specs" || strings.HasPrefix(normPath, ".specs/") {
			return true, "NEVER write under .specs/ — tsll manages spec/design/tasks automatically. " +
				"Implement SOURCE files in src/ or public/ only. Task IDs (T3) and numbers are NOT spec file paths."
		}
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
	if st.greenfield && tc.Tool == "create_dir" {
		base := strings.Trim(path, "/")
		if base == "internal" || base == "src" || base == "app" || base == "cmd" || base == "pkg" {
			return true, "write_file creates parent directories — write the source file directly (e.g. " + path + "/app.go) instead of create_dir"
		}
	}
	if st.greenfield && tc.Tool == "list_dir" && path != "." {
		return true, "write_file creates missing dirs — write source files directly"
	}
	if st.singleTouch {
		switch tc.Tool {
		case "write_file", "edit_file":
			if st.pathsWritten[path] {
				if st.planReceived && len(st.filePlan) > 0 && len(remainingPlanFiles(st.filePlan, st.pathsWritten)) == 0 {
					return true, "already wrote " + path + " — all planned files are done; respond with a plain-text summary (no tool call)"
				}
				return true, "already wrote " + path + " this task — each file gets ONE write_file with complete content; move to the next planned file or finish"
			}
		case "read_file":
			if st.pathsWritten[path] {
				return true, path + " is already written — do not re-read; continue with remaining files in your plan"
			}
			if st.pathsRead[path] {
				return true, "already read " + path + " — write_file once with full content instead of reading again"
			}
		}
	}
	if st.planReceived && len(st.filePlan) > 0 && (tc.Tool == "write_file" || tc.Tool == "edit_file") {
		if !stringInSlice(path, st.filePlan) {
			// Soft plan: in-scope paths may be added at runtime; off-scope paths are blocked.
			if len(st.focusedAllow) > 0 {
				if !pathMatchesFocusedAllowlist(path, st.focusedAllow) {
					return true, path + " belongs to another section — this task only allows paths matching: " +
						strings.Join(st.focusedAllow, ", ") + " (plus App.js/index.js wiring). Do NOT rewrite Header/Footer/other sections."
				}
				// In-scope but not listed in plan — allowed (plan is a guide, not a cage).
				return false, ""
			}
		}
	}
	if len(st.focusedAllow) > 0 && (tc.Tool == "write_file" || tc.Tool == "edit_file") {
		if !pathMatchesFocusedAllowlist(path, st.focusedAllow) {
			return true, path + " is outside this task's scope — only implement paths matching: " + strings.Join(st.focusedAllow, ", ") + " (plus App.js wiring)"
		}
	}
	return false, ""
}

func stringInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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

func buildSuccessContinuation(tc toolCallJSON, result string, greenfield bool, filePlan []string, written map[string]bool, singleTouch bool) string {
	switch tc.Tool {
	case "list_dir":
		if greenfield {
			return "Tool result:\n" + result + "\n\nGREENFIELD: write_file the source files named in the spec — do not keep listing directories."
		}
		return "Tool result:\n" + result + "\n\nRead each needed file at most once, then write_file each path once with complete content."
	case "read_file":
		msg := "Tool result:\n" + result + "\n\nNow write_file this path ONCE with the complete final content — do not read it again."
		if len(filePlan) > 0 {
			msg += "\n\n" + planWriteNudge(filePlan, written)
		}
		return msg
	case "create_dir":
		return "Tool result:\n" + result + "\n\nDirectory exists. Next: write_file the actual source file — do not read_file the empty directory."
	case "write_file", "edit_file":
		if len(filePlan) > 0 {
			return "Tool result:\n" + result + "\n\n" + planWriteNudge(filePlan, written)
		}
		if singleTouch {
			return "Tool result:\n" + result + "\n\nGood — do not touch this path again. Write the next file once, or summarize when done."
		}
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
