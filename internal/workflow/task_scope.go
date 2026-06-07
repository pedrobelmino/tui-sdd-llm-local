package workflow

import (
	"path/filepath"
	"strconv"
	"strings"
)

// TaskScope classifies how many files a task should touch.
type TaskScope string

const (
	ScopeScaffold TaskScope = "scaffold"
	ScopeFocused  TaskScope = "focused" // single section: header, footer, contact…
	ScopeSection  TaskScope = "section"
	ScopeDefault  TaskScope = "default"
)

func classifyTaskScope(taskBlock string) TaskScope {
	lower := strings.ToLower(taskBlock)
	for _, kw := range []string{
		"scaffold", "scaffolding", "bootstrap", "project structure",
		"setup project", "initial setup", "boilerplate", "skeleton", "foundation",
	} {
		if strings.Contains(lower, kw) {
			return ScopeScaffold
		}
	}
	for _, kw := range []string{
		"header", "footer", "contact", "banner", "carousel",
		"about us", "about", "product list", "featured product",
		"categories", "home page", "homepage",
	} {
		if strings.Contains(lower, kw) {
			return ScopeFocused
		}
	}
	if strings.Contains(lower, "section") || strings.Contains(lower, "component") ||
		strings.Contains(lower, "develop") || strings.Contains(lower, "implement") {
		return ScopeSection
	}
	return ScopeDefault
}

func maxPlanFilesForScope(scope TaskScope) int {
	switch scope {
	case ScopeScaffold:
		return 80
	case ScopeFocused:
		return 10
	case ScopeSection:
		return 10
	default:
		return 15
	}
}

// focusedTaskScopeText limits allowlist derivation to title + What + Where.
// Gate/Depends/Tests mention future sections (product, contact…) and must not widen scope.
func focusedTaskScopeText(taskBlock string) string {
	var parts []string
	for _, line := range strings.Split(taskBlock, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "### ") {
			parts = append(parts, t)
		}
	}
	for _, key := range []string{"**What**", "**Where**"} {
		if v := extractTaskField(taskBlock, key); v != "" {
			parts = append(parts, v)
		}
	}
	text := strings.Join(parts, "\n")
	if strings.TrimSpace(text) == "" {
		return taskBlock
	}
	return text
}

func extractTaskField(taskBlock, fieldKey string) string {
	lines := strings.Split(taskBlock, "\n")
	prefix := strings.ToLower(fieldKey)
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(trim), prefix) {
			continue
		}
		if idx := strings.Index(trim, ":"); idx >= 0 {
			rest := strings.TrimSpace(trim[idx+1:])
			if rest != "" {
				return rest
			}
		}
		var body []string
		for _, follow := range lines[i+1:] {
			f := strings.TrimSpace(follow)
			if strings.HasPrefix(f, "**") && strings.Contains(f, ":") {
				break
			}
			if f != "" && !strings.HasPrefix(f, "---") {
				body = append(body, f)
			}
		}
		return strings.Join(body, " ")
	}
	return ""
}

// focusedAllowedFragments returns path substrings allowed for a focused task.
// Empty slice means no allowlist (only max plan size applies).
func focusedAllowedFragments(taskBlock string) []string {
	lower := strings.ToLower(focusedTaskScopeText(taskBlock))
	type rule struct {
		key   string
		frags []string
	}
	rules := []rule{
		{"header", []string{"header", "logo", "nav"}},
		{"footer", []string{"footer", "logo"}},
		{"contact", []string{"contact"}},
		{"carousel", []string{"carousel", "banner"}},
		{"banner", []string{"banner", "carousel"}},
		{"about", []string{"about"}},
		{"product", []string{"product"}},
		{"categor", []string{"categor"}},
		{"home page", []string{"home"}},
		{"homepage", []string{"home"}},
	}
	seen := map[string]bool{}
	var out []string
	for _, r := range rules {
		if strings.Contains(lower, r.key) {
			for _, f := range r.frags {
				if !seen[f] {
					seen[f] = true
					out = append(out, f)
				}
			}
		}
	}
	return out
}

func isPathAllowedForFocusedTask(path string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	lower := strings.ToLower(filepath.ToSlash(path))
	for _, a := range allowed {
		if strings.Contains(lower, a) {
			return true
		}
	}
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "app.js", "index.js", "main.js", "app.jsx", "index.jsx", "app.css", "index.css":
		return true
	}
	if strings.HasPrefix(lower, "src/") || strings.HasPrefix(lower, "public/") {
		for _, ext := range []string{".png", ".svg", ".jpg", ".jpeg", ".webp", ".ico", ".css"} {
			if strings.HasSuffix(lower, ext) {
				for _, kw := range allowed {
					if strings.Contains(lower, kw) {
						return true
					}
				}
				for _, kw := range []string{"logo", "nav", "assets", "images", "icon"} {
					if strings.Contains(lower, kw) {
						for _, a := range allowed {
							if a == "header" || a == "footer" || a == "logo" || a == "nav" {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

func scopeHint(taskBlock string, scope TaskScope) string {
	max := maxPlanFilesForScope(scope)
	switch scope {
	case ScopeFocused:
		frags := focusedAllowedFragments(taskBlock)
		hint := "FOCUSED TASK: implement ONLY what this task title describes — not the whole app.\n"
		hint += "Your <file_plan> may list AT MOST " + strconv.Itoa(max) + " paths.\n"
		if len(frags) > 0 {
			hint += "Allowed path keywords for this task: " + strings.Join(frags, ", ") + ", plus App.js/index.js for wiring.\n"
			hint += "Static assets (logo, icons): use src/assets/, public/, or paths containing logo/nav — not .specs/.\n"
			hint += "Do NOT create pages/, services/, other components/, package.json, mock.js, unrelated sections, or anything under .specs/.\n"
		}
		return hint
	case ScopeScaffold:
		return "SCAFFOLD TASK: many files allowed. Prefer one-shot <task_plan>{\"files\":[...]}</task_plan> to avoid 30+ turns.\n"
	default:
		return "Keep <file_plan> to AT MOST " + strconv.Itoa(max) + " paths for this task.\n"
	}
}
