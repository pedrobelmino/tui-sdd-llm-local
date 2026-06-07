# Architecture

**Pattern:** Monólito CLI modular — camadas finas (cmd/tui → workflow → adapters) com persistência filesystem

## High-Level Structure

```
┌─────────────────────────────────────────────────────────────┐
│                      cmd/tsll (main)                         │
└──────────────────────────┬──────────────────────────────────┘
                           │
              ┌────────────┴────────────┐
              ▼                         ▼
     ┌─────────────────┐      ┌─────────────────┐
     │  internal/cmd   │      │  internal/tui   │
     │  (Cobra/plain)  │      │  (Bubble Tea)   │
     └────────┬────────┘      └────────┬────────┘
              │                         │
              └────────────┬────────────┘
                           ▼
                 ┌─────────────────┐
                 │ internal/workflow│
                 │ init / feature   │
                 └────────┬────────┘
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
   ┌────────────┐  ┌────────────┐  ┌────────────┐
   │  ollama    │  │  project   │  │ templates  │
   │  prompts   │  │  state     │  │  config    │
   └────────────┘  └────────────┘  └────────────┘
          │                │
          ▼                ▼
   Ollama HTTP API    .specs/ on disk
```

Observabilidade (TUI only): `gpu/`, `system/`, `tokens/` alimentam views e monitor strip em paralelo ao workflow.

## Identified Patterns

### Dual Entry, Shared Workflow

**Location:** `internal/cmd/*.go`, `internal/tui/interactive.go`, `internal/workflow/`
**Purpose:** Mesma lógica de negócio para automação CLI e dashboard interativo
**Implementation:** `workflow.Service` com `Specify`, `Tasks`, `Run`; CLI chama direto; TUI chama via `runWorkflow` em goroutine
**Example:** `internal/workflow/feature.go` — `Specify()` usado por `tsll specify` e `ActionSpecify` no TUI

### Pure View Renderers

**Location:** `internal/tui/views/`
**Purpose:** Testabilidade sem TTY; separar estado de apresentação
**Implementation:** Funções `Render*(data Struct)` recebem subset de campos; sem `tea.Model` nas views
**Example:** `views.RenderMonitorStrip(MonitorData)` — chamado de `page.go:composePage`

### Message-Driven TUI

**Location:** `internal/tui/model.go`, `messages.go`, `commands.go`
**Purpose:** I/O assíncrono sem bloquear o loop Bubble Tea
**Implementation:** `tea.Cmd` retorna mensagens tipadas; polling com `tea.Tick`
**Example:** `OllamaSnapshotMsg`, `ActionChunkMsg` → `RootModel.Update`

### Snapshot Structs for External State

**Location:** `ollama.Snapshot`, `gpu.Snapshot`, `system.Snapshot`
**Purpose:** Estado observável com timestamp, erro e flag de disponibilidade
**Implementation:** Fetch functions retornam struct completo; UI decide como exibir
**Example:** `ollama.FetchSnapshot(ctx, client)` em `commands.go`

### Project Root Discovery

**Location:** `internal/project/detect.go`
**Purpose:** Encontrar `.specs/project/PROJECT.md` subindo diretórios
**Implementation:** Walk parent até root filesystem; detecta `Corrupted` se dir existe sem PROJECT.md
**Example:** `project.FindProject(cwd)` usado no TUI load e `RequireRoot()` no CLI

## Data Flow

### Flow 1: Launch TUI (`tsll` sem args)

```
main → cmd.Execute → rootCmd.RunE
  → ShouldLaunchTUI() [isatty + TSLL_TUI≠0]
  → tui.Run(version)
  → tea.NewProgram(RootModel)
  → Init: loadProject, fetchOllama/GPU/System, start ticks
  → View: composePage(header + monitor + body + footer)
```

### Flow 2: Create Feature + Generate Spec (TUI)

```
User: Features(2) → n → FormNewFeatureName → FormFeatureBrief
  → startAction(ActionSpecify)
  → goroutine: workflow.Service.Specify()
      → prompts.SpecifySystem(root)
      → ollama.ChatStream(/api/chat)
      → onChunk → ActionChunkMsg → actionLog +=
      → templates.Spec() → write .specs/features/<name>/spec.md
      → state.UpdateCurrentWork()
  → ActionFinishedMsg → reload project
```

### Flow 3: CLI Specify (`tsll specify <feature>`)

```
cobra specifyCmd.RunE
  → project.RequireRoot()
  → prompts + ollama.ChatStream (stdout streaming)
  → templates.Spec → write spec.md
  → state update
```

### Flow 4: Monitoring Poll (background, TUI)

```
tickGPUCmd (2s) → gpu.Query() → GPUSnapshotMsg → model.gpu
tickSystemCmd (2s) → system.Monitor.Query() → SystemSnapshotMsg
tickOllamaCmd (2s) → ollama.FetchSnapshot() → OllamaSnapshotMsg
  → RenderMonitorStrip em todas as telas via composePage
```

### Flow 5: Config Resolution

```
config.Load()
  → Default() [qwen2.5-coder, localhost:11434, gpu_prefer=amd]
  → merge ~/.tsll/config.yaml
  → merge .tsllrc (cwd)
  → override OLLAMA_HOST, TSLL_MODEL env
```

## Code Organization

**Approach:** Layer-based com pacote `workflow` como domínio compartilhado; views por capacidade de UI

**Structure:**

| Layer | Packages |
|-------|----------|
| Entry | `cmd/tsll` |
| Interface | `cmd`, `tui`, `ui` |
| Application | `workflow` |
| Domain | `project`, `state`, `prompts`, `templates` |
| Infrastructure | `ollama`, `gpu`, `system`, `config`, `tokens` |

**Module boundaries:**

- `tui` **não importa** `cmd` (inversão correta)
- `workflow` **não importa** `tui` nem `cmd`
- `views` **não importa** `workflow` — apenas tipos de dados
- `prompts` lê skill files do filesystem — acoplamento leve ao layout `.cursor/skills/`

## Key Architectural Decisions

Documented in `.specs/project/STATE.md`:

- **AD-006:** TUI k9s-style como interface principal
- **AD-007:** RootModel router + views puras
- **AD-008:** GPU AMD-first, NVIDIA fallback
- **AD-009:** /proc direto, sem gopsutil
- **AD-011:** Workflow completo na TUI; CLI para automação
