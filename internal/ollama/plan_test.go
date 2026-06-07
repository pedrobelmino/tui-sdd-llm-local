package ollama

import (
	"strings"
	"testing"
)

func TestParseFilePlan_Lines(t *testing.T) {
	text := filePlanOpen + `
src/Contact.js
src/Contact.css
src/App.js
` + filePlanClose
	paths, ok := parseFilePlan(text)
	if !ok || len(paths) != 3 {
		t.Fatalf("paths=%v ok=%v", paths, ok)
	}
}

func TestParseFilePlan_StripsMarkdownFences(t *testing.T) {
	text := filePlanOpen + "\n```plaintext\nsrc/components/Header.js\nsrc/App.js\n```\n" + filePlanClose
	paths, ok := parseFilePlan(text)
	if !ok {
		t.Fatalf("expected ok, got paths=%v", paths)
	}
	for _, p := range paths {
		if strings.HasPrefix(p, "`") {
			t.Fatalf("fence leaked into paths: %v", paths)
		}
	}
	if len(paths) != 2 {
		t.Fatalf("paths=%v", paths)
	}
}

func TestParseFilePlan_DropsProseLines(t *testing.T) {
	text := filePlanOpen + "\nHere is the plan for the header:\nsrc/components/Header.js\n" + filePlanClose
	paths, ok := parseFilePlan(text)
	if !ok || len(paths) != 1 || paths[0] != "src/components/Header.js" {
		t.Fatalf("paths=%v ok=%v", paths, ok)
	}
}

func TestParseFilePlan_DropsCodeFragments(t *testing.T) {
	text := filePlanOpen + "\n" +
		"src/components/Header.js\n" +
		"w.WriteHeader(http.StatusServiceUnavailable)\n" +
		"return <div>Header</div>\n" +
		"const x = foo();\n" +
		"src/App.js\n" + filePlanClose
	paths, ok := parseFilePlan(text)
	if !ok {
		t.Fatalf("expected ok, got paths=%v", paths)
	}
	if len(paths) != 2 {
		t.Fatalf("code fragments leaked into plan: %v", paths)
	}
	for _, p := range paths {
		if strings.ContainsAny(p, "()<>= ") {
			t.Fatalf("code-like path kept: %q", p)
		}
	}
}

func TestLooksLikePath(t *testing.T) {
	good := []string{"src/App.js", "go.mod", "package.json", "public/index.html", "Dockerfile", ".gitignore", "src/components/Header.jsx"}
	bad := []string{
		"w.WriteHeader(http.StatusServiceUnavailable)",
		"const x = 1;",
		"return null",
		"obj.method()",
		"items[0]",
		"src/components/", // dir, no file
		"foo bar.js",      // space
	}
	for _, p := range good {
		if !looksLikePath(p) {
			t.Errorf("expected path, rejected: %q", p)
		}
	}
	for _, p := range bad {
		if looksLikePath(p) {
			t.Errorf("expected non-path, accepted: %q", p)
		}
	}
}

func TestDropOffScopePaths_KeepsInScope(t *testing.T) {
	kept := dropOffScopePaths([]string{
		"src/components/Header.js",
		"src/components/Footer.js",
		"src/App.js",
	}, []string{"header"})
	if len(kept) != 2 {
		t.Fatalf("kept=%v (want Header.js + App.js)", kept)
	}
}

func TestPathMatchesFocusedAllowlist_LogoAsset(t *testing.T) {
	allowed := []string{"header", "logo", "nav"}
	if !pathMatchesFocusedAllowlist("src/logo.png", allowed) {
		t.Fatal("src/logo.png should be allowed for header task")
	}
	if pathMatchesFocusedAllowlist("src/services/api.js", allowed) {
		t.Fatal("unrelated service path should be blocked")
	}
}

func TestParseBatchTaskPlan_JSON(t *testing.T) {
	text := taskPlanOpen + `{"files":[{"path":"src/a.js","content":"export const a=1;\n"},{"path":"src/b.css","content":"body{margin:0}\n"}]}` + taskPlanClose
	plan, ok := parseBatchTaskPlan(text)
	if !ok || len(plan.Files) != 2 {
		t.Fatalf("plan=%+v ok=%v", plan, ok)
	}
}

func TestPreflightToolBlock_OffScopeSectionNotFilePlan(t *testing.T) {
	allowed := []string{"product", "categor"}
	blocked, reason := preflightToolBlock(toolCallJSON{
		Tool: "write_file",
		Args: map[string]any{"path": "src/components/Header.jsx", "content": "x"},
	}, loopPreflightState{
		planReceived: true,
		filePlan:     []string{"src/components/FeaturedProductsCategories.jsx"},
		focusedAllow: allowed,
	})
	if !blocked || !strings.Contains(reason, "another section") {
		t.Fatalf("blocked=%v reason=%q", blocked, reason)
	}
}

func TestPreflightToolBlock_InScopeNotInPlanAllowed(t *testing.T) {
	allowed := []string{"product", "categor"}
	blocked, reason := preflightToolBlock(toolCallJSON{
		Tool: "write_file",
		Args: map[string]any{"path": "src/components/ProductListPage.jsx", "content": "x"},
	}, loopPreflightState{
		planReceived: true,
		filePlan:     []string{"src/components/FeaturedProductsCategories.jsx"},
		focusedAllow: allowed,
	})
	if blocked {
		t.Fatalf("in-scope integration file should be allowed, reason=%q", reason)
	}
}

func TestPreflightSingleTouch_BlocksSecondWrite(t *testing.T) {
	written := map[string]bool{"src/App.js": true}
	blocked, reason := preflightToolBlock(toolCallJSON{
		Tool: "write_file",
		Args: map[string]any{"path": "src/App.js", "content": "x"},
	}, loopPreflightState{
		singleTouch:  true,
		pathsWritten: written,
	})
	if !blocked || !strings.Contains(reason, "already wrote") {
		t.Fatalf("blocked=%v reason=%q", blocked, reason)
	}
}

func TestOffScopePlanPaths_HeaderTask(t *testing.T) {
	bad := offScopePlanPaths([]string{
		"internal/components/Header.js",
		"internal/components/Footer.js",
		"internal/pages/HomePage.js",
	}, []string{"header"})
	if len(bad) != 2 {
		t.Fatalf("bad=%v", bad)
	}
}

func TestBumpLoopLimitForPlan(t *testing.T) {
	limit := 28
	bumpLoopLimitForPlan(&limit, 24)
	if limit < 32 {
		t.Fatalf("limit=%d, want >= 32 for 24 planned files", limit)
	}
}

func TestPlanWriteNudge_Remaining(t *testing.T) {
	plan := []string{"a.js", "b.js", "c.js"}
	written := map[string]bool{"a.js": true}
	msg := planWriteNudge(plan, written)
	if !strings.Contains(msg, "b.js") || strings.Contains(msg, "a.js") {
		t.Fatalf("nudge=%q", msg)
	}
}
