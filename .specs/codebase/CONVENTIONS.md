# Code Conventions

**Observed from:** representative files across `internal/cmd`, `internal/tui`, `internal/workflow`, `internal/ollama`, `internal/project`

## Naming Conventions

**Files:**

- Snake_case implícito via palavras compostas: `feature_detail.go`, `tasks_parse.go`, `action_scroll.go`
- Testes colocados ao lado: `model_test.go`, `generate_test.go`
- Um pacote por diretório; subpacote `views/` para renderers TUI

Examples: `interactive.go`, `brownfield-mapping.md` (specs only), `amdgpu.go`

**Functions/Methods:**

- Exported: PascalCase — `FindProject`, `RenderMonitorStrip`, `ChatStream`
- Unexported: camelCase — `loadCfg`, `runWorkflow`, `formatOllamaLine`
- Constructors: `New*` — `NewClient`, `NewGenerateClient`, `NewRootModel`
- Renderers: `Render*` — `RenderFeatures`, `RenderMetrics`
- Handlers TUI: `handle*` — `handleActionKey`, `handleFormKey`
- Commands Bubble Tea: `*Cmd` — `fetchOllamaCmd`, `tickGPUCmd`

**Variables:**

- camelCase local: `projectRoot`, `actionLog`, `streamCtx`
- Struct fields exported quando parte da API do pacote: `ProjectContext.Valid`, `MonitorData.Width`
- Receivers curtos: `m` (RootModel), `c` (client), `d` (data struct)

**Constants:**

- PascalCase exported em blocos `const (` — `DefaultBaseURL`, `streamTimeout`
- Unexported camelCase para internos — `clientTimeout`, `monitorPanelTitle`
- Enums como typed int + iota — `Screen`, `ViewID`, `ActionKind`

## Code Organization

**Import ordering:**

1. Stdlib (`context`, `fmt`, `strings`)
2. Blank line
3. Third-party (`cobra`, `bubbletea`, `lipgloss`)
4. Blank line
5. Internal (`github.com/pedrobelmino/tui-sdd-llm-local/internal/...`)

Example from `internal/cmd/specify.go`:

```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"github.com/pedrobelmino/tui-sdd-llm-local/internal/ollama"
	...
	"github.com/spf13/cobra"
)
```

Note: alguns arquivos agrupam internal antes de third-party (`interactive.go`) — inconsistência menor.

**File structure within packages:**

1. Types / constants
2. Constructors (`New*`)
3. Public API
4. Unexported helpers
5. Tests in `*_test.go`

**Package boundaries:**

- `views/` — funções puras, sem imports de `bubbletea` (exceto `table` em `models.go`)
- `workflow/` — sem dependência de `tui` ou `cmd`
- `cmd/` e `tui/` — orquestram `workflow/`

## Type Safety / Documentation

**Approach:** Go structs tipados; interfaces pequenas onde há mocking

- `ollama.Client` / `ollama.GenerateClient` — separação metadata vs geração
- Data structs para views: `FeaturesData`, `MonitorData` — subset explícito do `RootModel`
- Comentários godoc em tipos exportados: `// RootModel is the top-level Bubble Tea model`

## Error Handling

**Pattern:** Retornar `error` com contexto; `fmt.Errorf` com `%w` para wrap

```go
return ollama.TokenUsage{}, fmt.Errorf("read spec.md: %w (run specify first)", err)
```

- CLI: `RunE` propaga erro; `SilenceUsage: true` no root
- TUI: erros viram `statusMsg` ou append no `actionLog` (`ActionFinishedMsg`)
- Degradação graciosa: GPU/system unavailable → snapshot com `Available: false` + `Error` string

## TUI Patterns

**RootModel value receiver:** Todos os métodos usam `func (m RootModel)` — structs copiados por valor; campos mutáveis são value types (`string`, não `strings.Builder`)

**Async I/O:** Goroutine + channel → `tea.Cmd` via `waitActionMsg`

```go
go runWorkflow(..., ch)
return m, tea.Batch(waitActionMsg(ch), actionTickCmd())
```

**Polling:** `tea.Tick(2*time.Second)` para GPU, system, Ollama

## Comments / Documentation

**Style:** Comentários curtos em exportados; lógica de negócio raramente comentada inline

- Decisões arquiteturais vão para `.specs/project/STATE.md` (AD-001…)
- Prompts documentados via skill references, não no código
- Sem TODO/FIXME no `internal/` (verificado 2026-06-07)

## Testing Conventions

- Table-driven e snapshot strings para views (`overview_test.go`)
- `httptest.NewServer` para Ollama (`client_test.go`, `generate_test.go`)
- `t.Setenv` para env vars em testes
- Injeção via vars em `commands.go`: `loadProjectFn`, `fetchOllamaFn`
