package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	stack := defaultIfEmpty(p.Stack, "- Language: Go\n- LLM: Ollama (qwen2.5-coder:3b)")
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

	agentsPath := filepath.Join(root, "AGENTS.md")
	agentsBody, err := buildAgentsIndex(root)
	if err != nil {
		return err
	}
	if err := os.WriteFile(agentsPath, []byte(agentsBody), 0o644); err != nil {
		return err
	}

	cfg := config.Load()
	return config.Save(cfg)
}

func buildAgentsIndex(root string) (string, error) {
	var docs []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		// AGENTS.md itself is generated from this index; avoid recursive listing.
		if rel == "AGENTS.md" {
			return nil
		}
		docs = append(docs, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(docs)

	var b strings.Builder
	b.WriteString("# AGENTS.md\n\n")
	b.WriteString("Guia de navegação para agentes no projeto.\n\n")
	b.WriteString("## Workflow tsll\n\n")
	b.WriteString("- Inicializar projeto: `tsll init`\n")
	b.WriteString("- Especificar feature: `tsll specify <feature>`\n")
	b.WriteString("- Gerar design: `tsll design <feature>`\n")
	b.WriteString("- Gerar tasks: `tsll tasks <feature>`\n")
	b.WriteString("- Implementar: `tsll implement <feature>`\n")
	b.WriteString("- Executar task: `tsll run <feature> --task T1`\n\n")
	b.WriteString("## Índice Markdown do Projeto\n\n")
	if len(docs) == 0 {
		b.WriteString("_Nenhum arquivo .md encontrado._\n")
		return b.String(), nil
	}
	for _, doc := range docs {
		b.WriteString("- `" + doc + "`\n")
	}
	b.WriteString("\n")
	b.WriteString("## Observações\n\n")
	b.WriteString("- Este índice é gerado no `tsll init`.\n")
	b.WriteString("- Quando novos `.md` forem adicionados, regenere com `tsll init --force`.\n")
	return b.String(), nil
}

func defaultIfEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
