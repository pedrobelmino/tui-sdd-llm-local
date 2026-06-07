# Testing Infrastructure

## Test Frameworks

**Unit/Integration:** Go `testing` (stdlib) — Go 1.22
**E2E:** Nenhum
**Coverage:** Não configurado no Makefile ou CI

## Test Organization

**Location:** `*_test.go` colocado no mesmo pacote que o código (`package tui`, `package ollama`, etc.)
**Naming:** `Test<Function>_<Scenario>` — ex: `TestChatStream_AllowsSlowChunks`, `TestModel_ActionScrollWhileRunning`
**Structure:** Um arquivo de teste por área; subpacote `views/` tem testes próprios

## Testing Patterns

### Unit Tests

**Approach:** Assertions diretas + snapshot strings para output TUI
**Location:** Todos os pacotes com lógica testável
**Pattern:** `newTestModel()` helper; `updated.(RootModel)` type assert após `Update`

Example: `internal/tui/model_test.go` — `TestModel_SwitchView` pressiona tecla `3`, verifica `activeView`

### Integration Tests (HTTP/GPU)

**Approach:** `httptest.NewServer` simula Ollama; fixtures em `gpu/testdata/`
**Location:** `internal/ollama/`, `internal/gpu/`
**Pattern:** Slow-chunk test valida timeout de streaming (6s gap)

Example: `internal/ollama/generate_test.go` — `TestChatStream_AllowsSlowChunks`

### View Snapshot Tests

**Approach:** String contains / full snapshot match com `NO_COLOR=1`
**Location:** `internal/tui/views/*_test.go`, `internal/ui/layout_test.go`
**Pattern:** `t.Setenv("NO_COLOR", "1")` para output determinístico

Example: `internal/tui/views/overview_test.go` — verifica painel "Workflow (TUI)"

### CLI Tests

**Approach:** `root_test.go`, `stubs_test.go` — execução de comandos com stubs
**Location:** `internal/cmd/`

## Test Execution

**Commands:**

```bash
make test          # go test ./...
go test ./...      # all packages
go test ./internal/tui/ -v -run TestModel_Action
go test ./internal/ollama/ -run TestChatStream
```

**Configuration:** Sem tags de build; sem `testing.Short()` gates observados

## Coverage Targets

**Current:** Não medido
**Goals:** Não documentados
**Enforcement:** Nenhuma

## Test Coverage Matrix

| Code Layer | Required Test Type | Location Pattern | Run Command | Current State |
| ---------- | ------------------ | ---------------- | ----------- | ------------- |
| Entry `cmd/tsll` | none (thin) | `cmd/tsll/main.go` | — | No tests |
| CLI commands | unit + stub | `internal/cmd/*_test.go` | `go test ./internal/cmd/...` | Partial (`root_test`, `stubs_test`) |
| TUI model | unit | `internal/tui/model_test.go` | `go test ./internal/tui/...` | Good |
| TUI views | snapshot unit | `internal/tui/views/*_test.go` | `go test ./internal/tui/views/...` | Good |
| UI layout | snapshot unit | `internal/ui/*_test.go` | `go test ./internal/ui/...` | Good |
| Workflow | unit | `internal/workflow/*_test.go` | `go test ./internal/workflow/...` | **None** |
| Ollama client | integration | `internal/ollama/*_test.go` | `go test ./internal/ollama/...` | Good |
| GPU adapters | unit + fixture | `internal/gpu/*_test.go` | `go test ./internal/gpu/...` | Good (3s timeout tests) |
| System metrics | unit | `internal/system/*_test.go` | `go test ./internal/system/...` | Good |
| Project parsing | unit | `internal/project/*_test.go` | `go test ./internal/project/...` | Good |
| Config | unit | — | — | **None** |
| Prompts | unit | — | — | **None** |
| State/templates | unit | — | — | **None** |
| Tokens | unit | `internal/tokens/*_test.go` | `go test ./internal/tokens/...` | Good |
| E2E TUI | e2e | — | — | **None** |

## Parallelism Assessment

| Test Type | Parallel-Safe? | Isolation Model | Evidence |
| --------- | -------------- | --------------- | -------- |
| Unit (tui, ui, tokens) | Yes | Pure structs, no shared state | `model_test.go` creates fresh `newTestModel()` |
| Ollama httptest | Yes | Per-test `httptest.Server` | `client_test.go`, `generate_test.go` |
| GPU nvidia timeout | Yes | Mock exec via injectable `queryNVIDIALegacy` | `nvidia_test.go` |
| GPU amdgpu | Mostly | Reads sysfs — depends on host hardware | May skip/fail on machines without amdgpu |
| System | Yes | `/proc` read-only; Monitor per-test instance | `system_test.go` |
| Full `go test ./...` | Yes | Go runs packages in parallel by default | No shared DB or global mutable test state |

## Gate Check Commands

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | Após mudança local em um pacote | `go test ./internal/<pkg>/...` |
| Full | Antes de commit/PR | `make test` ou `go test ./...` |
| Build | Após mudança estrutural | `make build && make test` |
| Manual smoke | Validar TUI + Ollama | `tsll doctor` + `tsll` (TTY) |
