package workflow

import (
	"strings"
)

func isGoStack(s string) bool {
	// Only explicit Go signals — never infer Go from generic words like "backend"
	// or a stray " go " token, which appear in plenty of non-Go (e.g. React) specs.
	return strings.Contains(s, "language: go") || strings.Contains(s, "- language: go") ||
		strings.Contains(s, "golang") || strings.Contains(s, "go no backend") ||
		strings.Contains(s, "go backend") || strings.Contains(s, "go (") ||
		(strings.Contains(s, " go ") && strings.Contains(s, "cobra"))
}

func isJSStack(s string) bool {
	return strings.Contains(s, "react") || strings.Contains(s, "javascript") ||
		strings.Contains(s, "node") || strings.Contains(s, "npm") || strings.Contains(s, "jsx")
}

// GreenfieldPathHint tells the model which roots to use (avoid wrong dirs like app/).
func GreenfieldPathHint(projectStack string) string {
	combined := strings.ToLower(projectStack)
	if isJSStack(combined) && isGoStack(combined) {
		return "GREENFIELD full-stack: landing/UI files go under src/, public/, package.json (React). " +
			"API/backend files go under cmd/, internal/, go.mod. NEVER use app/ or create_dir — write_file creates parents."
	}
	if isGoStack(combined) {
		return "GREENFIELD Go layout: use cmd/, internal/, go.mod — NEVER use app/, src/, frontend/ unless spec explicitly requires them."
	}
	if isJSStack(combined) {
		return "GREENFIELD JS/React layout: use src/, public/, package.json — not cmd/ or internal/."
	}
	return "GREENFIELD: create source files under paths matching the PROJECT tech stack above."
}

// detectStackLabel returns a short human label for the project stack, for logs.
func detectStackLabel(projectStack string) string {
	combined := strings.ToLower(projectStack)
	js, gostk := isJSStack(combined), isGoStack(combined)
	switch {
	case js && gostk:
		return "full-stack (React/JS + Go)"
	case js:
		return "React/JS"
	case gostk:
		return "Go"
	default:
		return ""
	}
}

// stackLanguageGuard returns a hard rule forbidding the wrong language, so a small
// local model does not hallucinate Go in a React project (or vice-versa).
func stackLanguageGuard(projectStack string) string {
	combined := strings.ToLower(projectStack)
	js, gostk := isJSStack(combined), isGoStack(combined)
	switch {
	case js && !gostk:
		return "HARD RULE: This is a JavaScript/React project. Generate ONLY .js/.jsx/.ts/.tsx/.css/.html/.json files. " +
			"NEVER create .go files, go.mod, cmd/, or internal/ — any Go code is WRONG for this project."
	case gostk && !js:
		return "HARD RULE: This is a Go project. Generate ONLY Go files (cmd/, internal/, go.mod). " +
			"NEVER create package.json or React/JS files."
	default:
		return ""
	}
}
