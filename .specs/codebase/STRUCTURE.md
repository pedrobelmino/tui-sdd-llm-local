# Project Structure

**Root:** `/home/pedro/dev/cursor/tui-sdd-llm-local`

## Directory Tree

```
tui-sdd-llm-local/
├── cmd/tsll/                 # Entry point
├── internal/
│   ├── cmd/                 # Cobra commands
│   ├── tui/                 # Bubble Tea dashboard
│   │   └── views/           # Pure render functions
│   ├── ui/                  # lipgloss layout primitives
│   ├── workflow/            # Business orchestration
│   ├── ollama/              # Ollama HTTP client
│   ├── project/             # .specs/ discovery & parsing
│   ├── prompts/             # LLM system prompts
│   ├── templates/           # Embedded markdown templates
│   ├── state/               # STATE.md updates
│   ├── config/              # User/project config
│   ├── gpu/                 # GPU metrics adapters
│   ├── system/              # Host metrics (/proc)
│   └── tokens/              # Session token counter
├── .specs/
│   ├── project/             # PROJECT, ROADMAP, STATE
│   ├── features/            # Per-feature specs
│   └── codebase/            # Brownfield analysis (this folder)
├── .cursor/skills/          # tlc-spec-driven skill
├── bin/                     # Compiled binary
├── Makefile
└── go.mod
```

## Module Organization

### Entry (`cmd/tsll/`)

**Purpose:** Minimal `main()` delegating to `internal/cmd.Execute()`
**Key files:** `main.go`

### CLI layer (`internal/cmd/`)

**Purpose:** Cobra subcommands and TUI launch gate
**Key files:** `root.go`, `init.go`, `specify.go`, `tasks.go`, `run.go`, `doctor.go`, `validate.go`, `helpers.go`

### TUI layer (`internal/tui/`)

**Purpose:** Interactive dashboard, forms, async workflow actions
**Key files:** `model.go`, `interactive.go`, `commands.go`, `page.go`, `action_scroll.go`, `keys.go`, `messages.go`, `app.go`

### View renderers (`internal/tui/views/`)

**Purpose:** Stateless panel rendering (Overview, Features, Models, Metrics, System, Monitor)
**Key files:** `overview.go`, `features.go`, `feature_detail.go`, `models.go`, `metrics.go`, `system.go`, `monitor.go`

### Workflow (`internal/workflow/`)

**Purpose:** Shared business logic for TUI and CLI
**Key files:** `init.go`, `feature.go`

### Integrations (`internal/ollama/`, `internal/gpu/`, `internal/system/`)

**Purpose:** External system adapters with snapshot structs
**Key files:** `ollama/client.go`, `ollama/generate.go`, `gpu/query.go`, `system/system.go`

### Project domain (`internal/project/`, `internal/state/`, `internal/templates/`)

**Purpose:** Filesystem-backed spec-driven artifacts
**Key files:** `project/detect.go`, `project/parse.go`, `project/tasks_parse.go`, `state/state.go`, `templates/templates.go`

## Where Things Live

**Interactive dashboard:**

- UI/Interface: `internal/tui/`, `internal/tui/views/`, `internal/ui/`
- Business Logic: `internal/tui/interactive.go` → `internal/workflow/`
- Data Access: `internal/project/`, `internal/ollama/`, `internal/gpu/`, `internal/system/`
- Configuration: `internal/config/`

**CLI automation (`tsll specify`, etc.):**

- UI/Interface: `internal/cmd/*.go` (stdout/stderr plain)
- Business Logic: `internal/workflow/` (shared with TUI)
- Data Access: same as TUI
- Configuration: `internal/config/`

**Spec artifacts:**

- UI/Interface: TUI forms + CLI prompts
- Business Logic: `internal/workflow/`, `internal/templates/`
- Data Access: `.specs/project/`, `.specs/features/<name>/`
- Configuration: `internal/prompts/` (loads skill references)

**Monitoring (tokens, GPU, Ollama, host):**

- UI/Interface: `internal/tui/views/metrics.go`, `monitor.go`, header mini-lines in `model.go`
- Business Logic: polling em `internal/tui/commands.go`
- Data Access: `internal/ollama/`, `internal/gpu/`, `internal/system/`, `internal/tokens/`
- Configuration: `internal/config/` (`gpu_prefer`, `ollama_host`)

## Special Directories

**`.specs/`**
**Purpose:** Spec-driven project memory — vision, roadmap, state, features
**Examples:** `project/PROJECT.md`, `features/cli-scaffold/spec.md`

**`.cursor/skills/tlc-spec-driven/`**
**Purpose:** Workflow reference; prompts load `references/*.md`
**Examples:** `references/specify.md`, `references/brownfield-mapping.md`

**`.tools/go/`**
**Purpose:** Local Go toolchain — not part of application architecture
**Examples:** vendored Go SDK for offline builds

**`internal/gpu/testdata/`**
**Purpose:** Fixture files for GPU parser tests
**Examples:** `nvidia_smi.csv`
