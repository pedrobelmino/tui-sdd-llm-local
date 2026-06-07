package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedrobelmino/tui-sdd-llm-local/internal/config"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/project"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/templates"
)

// InitParams holds arguments for project bootstrap.
type InitParams struct {
	Root     string
	Name     string
	Vision   string
	Target   string
	Solves   string
	Stack    string
	ScopeIn  string
	ScopeOut string
	Force    bool
}

// InitProject creates .specs/project/ files at Root.
func InitProject(p InitParams) error {
	root := p.Root
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = cwd
	}

	ctx, _ := project.FindProject(root)
	if ctx.Valid && !p.Force {
		return fmt.Errorf("project already initialized at %s (use --force)", ctx.Root)
	}

	name := p.Name
	vision := p.Vision
	if name == "" {
		name = filepath.Base(root)
	}
	if vision == "" {
		vision = "A tui-sdd-llm-local managed project"
	}

	target := defaultIfEmpty(p.Target, "dev solo")
	solves := defaultIfEmpty(p.Solves, "structured spec-driven development with local AI")
	stack := defaultIfEmpty(p.Stack, "- Language: Go\n- LLM: Ollama (qwen2.5-coder)")
	scopeIn := defaultIfEmpty(p.ScopeIn, "- tsll workflow: init, specify, tasks, run\n- TUI dashboard")
	scopeOut := defaultIfEmpty(p.ScopeOut, "- Cloud APIs\n- macOS/Windows")

	files := templates.NewProject(name, vision, target, solves, stack, scopeIn, scopeOut)
	dirs := []struct{ dir, file, content string }{
		{".specs/project", "PROJECT.md", files.Project},
		{".specs/project", "ROADMAP.md", files.Roadmap},
		{".specs/project", "STATE.md", files.State},
	}

	for _, d := range dirs {
		path := filepath.Join(root, d.dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		full := filepath.Join(path, d.file)
		if err := os.WriteFile(full, []byte(d.content), 0o644); err != nil {
			return err
		}
	}

	cfg := config.Load()
	return config.Save(cfg)
}

func defaultIfEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
