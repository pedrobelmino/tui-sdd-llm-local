package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestKeyMap_FooterBindings(t *testing.T) {
	want := "r: refresh │ 1-5: views │ ?: help │ q: quit"
	if got := FooterBindings(); got != want {
		t.Fatalf("FooterBindings() = %q, want %q", got, want)
	}
}

func TestHelpOverlay_DocumentsAllKeys(t *testing.T) {
	got := HelpOverlay()
	for _, phrase := range []string{"new feature", "open feature detail", "implement feature", "run selected task", "esc"} {
		if !strings.Contains(got, phrase) {
			t.Fatalf("HelpOverlay() missing %q:\n%s", phrase, got)
		}
	}
}

func TestDefaultKeyMap_KeyBindings(t *testing.T) {
	km := DefaultKeyMap()

	cases := []struct {
		name string
		b    key.Binding
		keys []string
	}{
		{"Quit", km.Quit, []string{"q"}},
		{"ForceQuit", km.ForceQuit, []string{"ctrl+c"}},
		{"Help", km.Help, []string{"?"}},
		{"Overview", km.Overview, []string{"1"}},
		{"Features", km.Features, []string{"2"}},
		{"Models", km.Models, []string{"3"}},
		{"Metrics", km.Metrics, []string{"4"}},
		{"System", km.System, []string{"5"}},
		{"Refresh", km.Refresh, []string{"r"}},
		{"Init", km.Init, []string{"i"}},
		{"NewFeature", km.NewFeature, []string{"n"}},
		{"Specify", km.Specify, []string{"s"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.b.Keys()
			if len(got) != len(tc.keys) {
				t.Fatalf("keys = %v, want %v", got, tc.keys)
			}
			for i, want := range tc.keys {
				if got[i] != want {
					t.Fatalf("keys[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
