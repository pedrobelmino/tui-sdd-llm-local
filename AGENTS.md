# AGENTS.md — tui-sdd-llm-local

Guia para agentes de IA trabalhando neste repositório.

## O que é este projeto

**tui-sdd-llm-local** é uma CLI Go com dashboard TUI (Bubble Tea) para spec-driven development usando Ollama local. O binário se chama **`tsll`**.

- **Módulo Go:** `github.com/pedrobelmino/tui-sdd-llm-local`
- **Entry:** `cmd/tsll/main.go` → `internal/cmd`
- **TUI:** `internal/tui/` + `internal/tui/views/`
- **Workflow:** `internal/workflow/` (init, specify, tasks, run)
- **Specs:** `.specs/` no root de cada projeto bel/tsll

## Agradecimentos — Tech Lead's Club

O workflow spec-driven vem da skill **[tlc-spec-driven](https://github.com/tech-leads-club/agent-skills/blob/main/packages/skills-catalog/skills/(development)/tlc-spec-driven/SKILL.md)** (v2.0.0, CC-BY-4.0, Felipe Rodrigues).

Ao planejar ou implementar features neste repo:

1. Siga as fases **Specify → (Design) → (Tasks) → Execute** com auto-sizing por complexidade
2. Use os artefatos em `.specs/codebase/` (brownfield mapping já feito)
3. Consulte `.cursor/skills/tlc-spec-driven/references/` para cada fase
4. Persista decisões em `.specs/project/STATE.md`

**Referência upstream:** https://github.com/tech-leads-club/agent-skills

## Contexto a carregar

| Tarefa | Arquivos |
|--------|----------|
| Qualquer feature | `.specs/project/PROJECT.md`, `STATE.md` |
| Planejamento | `.specs/codebase/CONCERNS.md`, `ARCHITECTURE.md` |
| Implementação | `.specs/codebase/CONVENTIONS.md`, `TESTING.md` |
| Feature específica | `.specs/features/<name>/spec.md`, `tasks.md` |

**Não carregar** múltiplos specs de features simultaneamente.

## Convenções de código

- Go 1.22, pacotes em `internal/`
- Views TUI: funções puras `Render*` em `internal/tui/views/`
- `RootModel` usa **value receiver** — não usar `strings.Builder` em campos mutáveis
- Cliente Ollama generate: **sem** `http.Client.Timeout` global; deadline via context (10 min)
- Testes: `go test ./...` — ver matriz em `.specs/codebase/TESTING.md`

## Comandos úteis

```bash
make build          # → bin/tsll
make test
tsll doctor         # smoke: Ollama + GPU + .specs/
```

## Verificar Ollama antes de testar LLM

```bash
curl -s http://127.0.0.1:11434/api/tags
ollama ps
tsll doctor
```

## Áreas frágeis (ver CONCERNS.md)

- `internal/workflow/` — sem testes; CLI ainda duplica parte da lógica
- `internal/tui/interactive.go` — async goroutine + channel
- `internal/gpu/` — parsing varia por driver
- `bel run` / `tsll run` — não aplica código automaticamente

## Renomeação histórica

Este projeto era **bel-cli** (`bel` / `github.com/pedro/bel-cli`). Referências antigas devem usar **tui-sdd-llm-local** / **tsll** / `~/.tsll/`.
