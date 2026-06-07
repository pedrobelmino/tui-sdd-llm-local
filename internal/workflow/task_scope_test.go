package workflow

import "testing"

func TestClassifyTaskScope_HeaderIsFocused(t *testing.T) {
	block := "### 3. Develop Header Section (T3)\n**What**: header with logo"
	if classifyTaskScope(block) != ScopeFocused {
		t.Fatalf("want focused")
	}
	if maxPlanFilesForScope(ScopeFocused) != 10 {
		t.Fatalf("max plan")
	}
}

func TestFocusedAllowed_HeaderOnly(t *testing.T) {
	allowed := focusedAllowedFragments("Develop Header Section")
	if len(allowed) == 0 || allowed[0] != "header" {
		t.Fatalf("allowed=%v", allowed)
	}
	if isPathAllowedForFocusedTask("internal/components/Footer.js", allowed) {
		t.Fatal("footer should be blocked")
	}
	if !isPathAllowedForFocusedTask("internal/components/Header.js", allowed) {
		t.Fatal("header should be allowed")
	}
	if !isPathAllowedForFocusedTask("src/logo.png", allowed) {
		t.Fatal("logo asset should be allowed for header task")
	}
	if !isPathAllowedForFocusedTask("src/components/Header.jsx", allowed) {
		t.Fatal("header component should be allowed")
	}
	if !isPathAllowedForFocusedTask("internal/App.js", allowed) {
		t.Fatal("app wiring should be allowed")
	}
}

func TestFocusedAllowed_GateDoesNotPolluteScope(t *testing.T) {
	block := "### 3. Develop Header Section (T3)\n" +
		"**What**: Implement the header section with company logo and navigation menu.\n" +
		"**Where**: Landing page codebase\n" +
		"**Gate**: Move to Featured Products/categories Section development.\n"
	allowed := focusedAllowedFragments(block)
	for _, bad := range []string{"product", "categor", "contact"} {
		for _, a := range allowed {
			if a == bad {
				t.Fatalf("gate must not add %q to allowlist, got %v", bad, allowed)
			}
		}
	}
	if len(allowed) == 0 {
		t.Fatal("expected header-related fragments")
	}
}

func TestEstimateTaskLoopLimit_FocusedSmall(t *testing.T) {
	got := estimateTaskLoopLimit("### T3: Develop Header", true)
	if got > 22 {
		t.Fatalf("focused limit too high: %d", got)
	}
	if got < 18 {
		t.Fatalf("focused limit too low: %d", got)
	}
}
