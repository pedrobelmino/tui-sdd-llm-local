# Codebase Concerns

**Analysis Date:** 2026-06-07

## Tech Debt

**Duplicação CLI ↔ TUI para workflow:**

- Issue: `internal/cmd/specify.go`, `tasks.go`, `run.go` reimplementam fluxo similar ao `internal/workflow/feature.go` em vez de delegar 100% ao `workflow.Service`
- Files: `internal/cmd/specify.go`, `internal/workflow/feature.go`
- Why: Scaffold evoluiu com CLI primeiro, TUI depois
- Impact: Mudanças de prompt/output precisam ser feitas em dois lugares
- Fix approach: Refatorar subcomandos para chamar `workflow.New().Specify/Tasks/Run` exclusivamente

**RootModel value receiver:**

- Issue: Todo o TUI usa `func (m RootModel)` — cópia por valor a cada `Update`
- Files: `internal/tui/model.go`, `internal/tui/interactive.go`
- Why: Padrão inicial Bubble Tea simplificado
- Impact: Campos não copiáveis (ex: antigo `strings.Builder`) causam panic; structs grandes copiados a cada frame
- Fix approach: Migrar para `*RootModel` ou ponteiros em campos pesados se performance virar problema
- Note: `actionLog` já corrigido de `strings.Builder` → `string` (2026-06-07)

**STATE.md menciona truncamento de 4k no action log:**

- Issue: AD-011 documenta "streaming truncado em ~4k chars" — código atual usa scroll sem truncar por chars
- Files: `.specs/project/STATE.md`, `internal/tui/interactive.go`
- Impact: Documentação desatualizada pode confundir
- Fix approach: Atualizar STATE.md para refletir scroll + PanelViewport

## Known Bugs

Nenhum bug aberto com reprodução confirmada no código após fixes de 2026-06-07 (timeout Ollama 5s, panic strings.Builder, scroll).

## Security Considerations

**Ollama localhost trust:**

- Risk: Qualquer processo local pode chamar Ollama; tsll não valida respostas do modelo antes de escrever arquivos
- Files: `internal/workflow/feature.go`, `internal/ollama/generate.go`
- Current mitigation: Escopo dev solo, offline, sem rede exposta pelo tsll
- Recommendations: Validar path traversal em nomes de feature; sandbox opcional para `tsll run` output

**Escrita em filesystem:**

- Risk: Modelo gera conteúdo escrito diretamente em `.specs/features/<name>/`
- Files: `internal/workflow/feature.go`, `internal/templates/templates.go`
- Current mitigation: `os.MkdirAll` com paths construídos do input do usuário
- Recommendations: Sanitizar `feature` name (reject `..`, slashes)

## Performance Bottlenecks

**Geração LLM em GPU limitada (4GB VRAM):**

- Problem: `qwen2.5-coder` com offload parcial pode gerar tokens lentos (minutos por spec)
- Files: `internal/ollama/generate.go` (`streamTimeout = 10min`)
- Cause: Hardware constraint do ambiente alvo, não bug de código
- Improvement path: Documentar modelos menores (`phi3:mini`); `OLLAMA_KEEP_ALIVE`; configurar `TSLL_MODEL`

**Polling 2s × 3 fontes:**

- Problem: TUI faz 3 polls a cada 2s (GPU, system, Ollama)
- Files: `internal/tui/commands.go`, `internal/tui/model.go`
- Cause: Monitoramento em tempo real
- Improvement path: Unificar tick único; aumentar intervalo quando idle

## Fragile Areas

**Bubble Tea async workflow:**

- Files: `internal/tui/interactive.go` (`runWorkflow`, `startAction`, channel buffer 64)
- Why fragile: Goroutine + channel; se channel enche, pode bloquear; sem cancelamento de context
- Common failures: Panic em tipos não copiáveis; timeout Ollama
- Safe modification: Testar com `TestModel_ActionLogAppendsAcrossUpdates`; manter `actionLog` como `string`
- Test coverage: Parcial — sem teste de channel overflow ou cancel mid-stream

**GPU detection multi-vendor:**

- Files: `internal/gpu/query.go`, `amdgpu.go`, `nvidia.go`, `rocm.go`
- Why fragile: Parsing de CLI output e sysfs varia por driver/kernel
- Common failures: Métricas unavailable em VMs ou GPUs não suportadas
- Safe modification: Manter degradação graciosa (`Available: false`); adicionar fixtures antes de mudar parsers
- Test coverage: Bom para NVIDIA mock; AMD depende de host

**Markdown parsing (tasks):**

- Files: `internal/project/tasks_parse.go`, `internal/workflow/feature.go` (`extractTaskBlock`)
- Why fragile: Regex-based parsing de `tasks.md` gerado por LLM — formato pode variar
- Safe modification: Adicionar testes com fixtures reais de tasks gerados
- Test coverage: `tasks_parse_test.go` existe; workflow `Run` não testado

## Scaling Limits

**Single developer / single machine:**

- Current capacity: 1 sessão TUI, 1 Ollama, contadores de tokens em memória
- Limit: Sem persistência cross-process de tokens; sem multi-feature parallel generation
- Symptoms at limit: Tokens resetam ao fechar TUI; segunda instância tsll não compartilha estado
- Scaling path: `~/.tsll/tokens.json` (deferred em STATE.md)

## Dependencies at Risk

**Bubble Tea / Charm ecosystem:**

- Risk: API changes entre versões; acoplamento a `bubbles` components
- Impact: Breaking changes em upgrades de `bubbletea` v1.x → futuras
- Migration plan: Pin versions em `go.mod`; testar TUI após upgrades

**Ollama API:**

- Risk: Schema de `/api/chat` NDJSON pode evoluir
- Impact: Parser em `generate.go` ignora linhas JSON inválidas silenciosamente
- Migration plan: Monitorar changelog Ollama; expandir `types.go`

## Missing Critical Features

**`tsll run` não aplica código automaticamente:**

- Problem: Run gera sugestões de código mas não escreve arquivos do projeto alvo
- Current workaround: Dev copia manualmente do output
- Blocks: Automação completa specify → tasks → implement
- Implementation complexity: Alta — precisa parser de blocos de código + confirmação

**Commits atômicos por task:**

- Problem: Deferred em STATE.md — não implementado
- Blocks: Traceability git por task
- Implementation complexity: Média

**Persistência cross-process de tokens:**

- Problem: `internal/tokens/` só vive na sessão TUI
- Blocks: Métricas históricas entre execuções
- Implementation complexity: Baixa

## Test Coverage Gaps

**`internal/workflow/` — sem testes:**

- What's not tested: `Specify`, `Tasks`, `Run`, `InitProject` end-to-end
- Risk: Regressões em escrita de arquivos e prompts
- Priority: High
- Difficulty: Média — mock `GenerateClient` interface

**`internal/cmd/` specify/tasks/run — cobertura parcial:**

- What's not tested: Fluxo completo com Ollama real
- Risk: Divergência CLI vs workflow service
- Priority: Medium

**TUI E2E — sem testes:**

- What's not tested: Fluxo completo feature creation em TTY
- Risk: Regressões de UX (scroll, monitor, forms)
- Priority: Medium
- Difficulty: Alta — requer `teatest` ou similar

**`internal/prompts/`, `internal/state/`, `internal/config/` — sem testes:**

- What's not tested: Skill discovery, STATE.md updates, config merge
- Risk: Baixo a médio
- Priority: Low–Medium

---

_Concerns audit: 2026-06-07_
_Update as issues are fixed or new ones discovered_
