package ollama

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// planPathChars matches a string made up only of characters valid in a file path.
// It rejects code fragments (parens, =, ;, {}, etc.) the small model sometimes
// emits inside a <file_plan> block.
var planPathChars = regexp.MustCompile(`^[A-Za-z0-9_./@+\-]+$`)

// fileExtRe requires a real-looking extension on the final path segment.
var fileExtRe = regexp.MustCompile(`\.[A-Za-z0-9]{1,8}$`)

// knownNoExtFiles are legitimate files without an extension.
var knownNoExtFiles = map[string]bool{
	"Dockerfile": true, "Makefile": true, "Procfile": true,
	"LICENSE": true, ".gitignore": true, ".dockerignore": true,
	".env": true, ".npmrc": true, ".nvmrc": true,
}

const (
	filePlanOpen  = "<file_plan>"
	filePlanClose = "</file_plan>"
	taskPlanOpen  = "<task_plan>"
	taskPlanClose = "</task_plan>"
)

// TaskPlanFile is one file entry in a batch task plan.
type TaskPlanFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// BatchTaskPlan is a one-shot plan with full file contents.
type BatchTaskPlan struct {
	Files []TaskPlanFile `json:"files"`
}

// parseBatchTaskPlan extracts a batch plan with file contents from model output.
func parseBatchTaskPlan(text string) (BatchTaskPlan, bool) {
	inner := extractTaggedBlock(text, taskPlanOpen, taskPlanClose)
	if inner == "" {
		if idx := strings.Index(text, `"files"`); idx >= 0 {
			if start := strings.LastIndex(text[:idx], "{"); start >= 0 {
				if raw, ok := extractBraceBalancedJSON(text, start); ok {
					inner = raw
				}
			}
		}
	}
	if inner == "" {
		return BatchTaskPlan{}, false
	}
	var plan BatchTaskPlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(inner)), &plan); err != nil {
		return BatchTaskPlan{}, false
	}
	if len(plan.Files) == 0 {
		return BatchTaskPlan{}, false
	}
	for i := range plan.Files {
		plan.Files[i].Path = strings.TrimSpace(plan.Files[i].Path)
		if plan.Files[i].Path == "" || strings.TrimSpace(plan.Files[i].Content) == "" {
			return BatchTaskPlan{}, false
		}
	}
	return plan, true
}

// parseFilePlan extracts a path-only plan (one path per line or JSON array).
func parseFilePlan(text string) ([]string, bool) {
	inner := extractTaggedBlock(text, filePlanOpen, filePlanClose)
	if inner == "" {
		return nil, false
	}
	inner = strings.TrimSpace(inner)
	if strings.HasPrefix(inner, "[") {
		var paths []string
		if err := json.Unmarshal([]byte(inner), &paths); err != nil {
			return nil, false
		}
		return normalizePlanPaths(paths), len(paths) > 0
	}
	var paths []string
	for _, line := range strings.Split(inner, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		paths = append(paths, line)
	}
	paths = normalizePlanPaths(paths)
	return paths, len(paths) > 0
}

func extractTaggedBlock(text, open, close string) string {
	start := strings.Index(text, open)
	if start < 0 {
		return ""
	}
	rest := text[start+len(open):]
	end := strings.Index(rest, close)
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func normalizePlanPaths(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "`")
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"',`)
		p = strings.TrimSpace(p)
		if !looksLikePath(p) || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// looksLikePath filters out prose, markdown fences, code fragments, and other
// non-file lines that the 3B model sometimes emits inside a <file_plan> block.
func looksLikePath(p string) bool {
	if p == "" {
		return false
	}
	// Markdown code fences: ```, ```plaintext, ```js, etc.
	if strings.HasPrefix(p, "```") || p == "```" {
		return false
	}
	if strings.HasPrefix(p, "#") || strings.HasPrefix(p, "<") {
		return false
	}
	// Real file paths don't contain spaces; prose lines do.
	if strings.ContainsAny(p, " \t") {
		return false
	}
	// Only path-safe characters — rejects code like w.WriteHeader(http.StatusX),
	// foo = bar, obj.method(), arrays[0], etc.
	if !planPathChars.MatchString(p) {
		return false
	}
	// Must be a file: a known extensionless file or a final segment with an extension.
	base := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		base = p[i+1:]
	}
	if knownNoExtFiles[base] {
		return true
	}
	return fileExtRe.MatchString(base)
}

func remainingPlanFiles(plan []string, written map[string]bool) []string {
	var out []string
	for _, p := range plan {
		if !written[p] {
			out = append(out, p)
		}
	}
	return out
}

func bumpLoopLimitForPlan(limit *int, plannedFiles int) {
	if plannedFiles <= 0 {
		return
	}
	needed := plannedFiles + 8 // plan turn + reads/nudges buffer
	if needed > *limit {
		*limit = needed
	}
	if *limit > maxToolIter {
		*limit = maxToolIter
	}
}

func bumpLoopLimitForProgress(limit *int, filesWritten int, plan []string, written map[string]bool) {
	remaining := len(remainingPlanFiles(plan, written))
	needed := filesWritten + remaining + 6
	if remaining == 0 && filesWritten > 0 {
		needed = filesWritten + 4 // summary turn
	}
	if needed > *limit {
		*limit = needed
	}
	if *limit > maxToolIter {
		*limit = maxToolIter
	}
}

func pathMatchesFocusedAllowlist(path string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	lower := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	for _, a := range allowed {
		if strings.Contains(lower, a) {
			return true
		}
	}
	base := lower
	if i := strings.LastIndex(lower, "/"); i >= 0 {
		base = lower[i+1:]
	}
	switch base {
	case "app.js", "index.js", "main.js", "app.jsx", "index.jsx", "app.css", "index.css":
		return true
	}
	// Static assets (logo.png, icons) under src/ or public/ for the active section keywords.
	if isFocusedUIAssetPath(lower) {
		for _, kw := range focusedAssetKeywords(allowed) {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	return false
}

func isFocusedUIAssetPath(p string) bool {
	if !strings.HasPrefix(p, "src/") && !strings.HasPrefix(p, "public/") {
		return false
	}
	for _, ext := range []string{".png", ".svg", ".jpg", ".jpeg", ".webp", ".ico", ".gif", ".css"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

func focusedAssetKeywords(allowed []string) []string {
	extra := map[string][]string{
		"header": {"logo", "nav", "assets", "images", "icon"},
		"footer": {"logo", "assets", "images"},
		"contact": {"icon", "assets"},
		"product": {"assets", "images"},
		"categor": {"assets", "images"},
	}
	seen := map[string]bool{}
	var out []string
	for _, a := range allowed {
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
		for _, kw := range extra[a] {
			if !seen[kw] {
				seen[kw] = true
				out = append(out, kw)
			}
		}
	}
	return out
}

// dropOffScopePaths keeps only paths matching the focused allowlist.
func dropOffScopePaths(paths, allowed []string) []string {
	if len(allowed) == 0 {
		return paths
	}
	var kept []string
	for _, p := range paths {
		if pathMatchesFocusedAllowlist(p, allowed) {
			kept = append(kept, p)
		}
	}
	return kept
}

func offScopePlanPaths(paths, allowed []string) []string {
	if len(allowed) == 0 {
		return nil
	}
	var bad []string
	for _, p := range paths {
		if !pathMatchesFocusedAllowlist(p, allowed) {
			bad = append(bad, p)
		}
	}
	return bad
}

func filePlanTooLargeNudge(max int) string {
	return "Your <file_plan> lists too many files for this single task. " +
		"Re-send <file_plan> with AT MOST " + strconv.Itoa(max) + " paths — only what THIS task title requires, not the whole app."
}

func filePlanOffScopeNudge(bad, allowed []string) string {
	return "Remove off-scope paths from <file_plan>: " + strings.Join(bad, ", ") +
		". This task only allows: " + strings.Join(allowed, ", ") + ", plus App.js/index.js for wiring."
}

func filePlanNudge() string {
	return "Before any tool call, respond with ONLY a <file_plan> block listing every source file path this task needs (one path per line).\n" +
		"Derive the paths from tasks.md (this task's What/Where) plus what spec.md/design.md require.\n" +
		"Example:\n" + filePlanOpen + "\nsrc/App.jsx\nsrc/components/Header.jsx\n" + filePlanClose
}

// directWriteNudge gives the model the exact tool call format to write the next file.
// Used after planWriteNudge fails to get a tool call from the model.
func directWriteNudge(nextPath string) string {
	return "STOP. Do not output text. Output ONLY this exact structure (replace CONTENT with the real file content):\n" +
		"<tool_call>{\"tool\":\"write_file\",\"args\":{\"path\":\"" + nextPath + "\",\"content\":\"CONTENT\"}}</tool_call>\n" +
		"No explanation. No markdown. Just the <tool_call> block."
}

func planWriteNudge(plan []string, written map[string]bool) string {
	rem := remainingPlanFiles(plan, written)
	if len(rem) == 0 {
		return "All planned files written. Respond with a plain-text summary (no tool call)."
	}
	msg := "Write the next file from your plan with ONE write_file (complete content, no re-reads):\n"
	for i, p := range rem {
		if i >= 3 {
			msg += "  ...\n"
			break
		}
		msg += "  - " + p + "\n"
	}
	msg += "Already written paths must NOT be touched again this task."
	return msg
}
