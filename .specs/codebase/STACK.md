# Tech Stack

**Analyzed:** 2026-06-07

## Core

- **Language:** Go 1.22 (`go.mod`)
- **Module:** `github.com/pedrobelmino/tui-sdd-llm-local`
- **Runtime:** Binário nativo Linux amd64 (sem servidor embutido)
- **Package manager:** Go modules (`go mod`)
- **Build:** `Makefile` — `make build`, `make install`, `make test`

## CLI & TUI

- **CLI framework:** `github.com/spf13/cobra` v1.8.1 — subcomandos (`init`, `specify`, `tasks`, `run`, `doctor`, `validate`)
- **TUI loop:** `github.com/charmbracelet/bubbletea` v1.2.4 — dashboard full-screen estilo k9s
- **TUI components:** `github.com/charmbracelet/bubbles` v0.20.0 — `textinput`, `textarea`, `table`, `key`
- **Styling:** `github.com/charmbracelet/lipgloss` v1.0.0 + `muesli/termenv` v0.15.2
- **TTY detection:** `github.com/mattn/go-isatty` v0.0.20 — decide TUI vs plain mode

## Backend / Domínio

- **API style:** Nenhuma API HTTP exposta; cliente HTTP para Ollama
- **Persistência:** Arquivos markdown/YAML no filesystem (`.specs/`, `~/.tsll/config.yaml`)
- **Config:** `gopkg.in/yaml.v3` v3.0.1
- **Autenticação:** Nenhuma (dev solo, local-only)

## LLM Runtime

- **Provider:** Ollama (HTTP local, default `http://127.0.0.1:11434`)
- **Modelo padrão:** `qwen2.5-coder`
- **Integração:** `internal/ollama/` — `/api/tags`, `/api/ps`, `/api/chat` (streaming)

## Observabilidade

- **GPU AMD:** sysfs `/sys/class/drm/card*/device` + fallback `rocm-smi`
- **GPU NVIDIA:** `nvidia-smi` CSV parsing
- **Host metrics:** `/proc`, `statfs` via `internal/system/` (stdlib only)
- **Tokens:** contador em memória na sessão TUI (`internal/tokens/`)

## Testing

- **Unit/Integration:** Go `testing` package (stdlib) — `*_test.go` colocados junto ao código
- **E2E:** Nenhum framework dedicado; sem testes de TTY end-to-end
- **Coverage:** Não configurado (sem `go test -cover` no Makefile)
- **Test doubles:** `httptest` para Ollama; variáveis injetáveis em `internal/tui/commands.go`

## External Services

- **LLM:** Ollama daemon local
- **GPU drivers:** amdgpu kernel module / ROCm / NVIDIA driver (opcional)
- **Skill reference:** `.cursor/skills/tlc-spec-driven/` — prompts e workflow

## Development Tools

- **Build:** `Makefile` com `LDFLAGS` para injetar versão
- **Toolchain local:** `.tools/go/` (Go vendored no repo — não é dependência do app)
- **Spec workflow:** skill `tlc-spec-driven` v2.0.0

## Platform Constraints

- **OS:** Linux apenas (v1)
- **Terminal:** mínimo 80×24, ANSI colors
- **Env vars:** `OLLAMA_HOST`, `TSLL_MODEL`, `TSLL_GPU_PREFER`, `TSLL_TUI`, `NO_COLOR`
