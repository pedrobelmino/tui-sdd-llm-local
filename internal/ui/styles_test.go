package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestNoColor_WhenUnset(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if NoColor() {
		t.Fatal("expected NoColor false when NO_COLOR unset")
	}
}

func TestNoColor_StripsANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := render(StyleError, "boom")
	if strings.Contains(out, "\x1b") {
		t.Fatalf("expected no ANSI escapes, got %q", out)
	}
	if out != "boom" {
		t.Fatalf("expected plain text %q, got %q", "boom", out)
	}
}

func TestStyleSuccess_RendersWithColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	out := render(StyleSuccess, "ok")
	if !strings.Contains(out, "\x1b") {
		t.Fatalf("expected ANSI styling when NO_COLOR unset, got %q", out)
	}
}
