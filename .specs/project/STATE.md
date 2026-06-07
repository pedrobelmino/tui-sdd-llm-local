# State

**Last Updated:** 2026-06-07
**Current Work:** Renomeado para tui-sdd-llm-local — publicado em github.com/pedrobelmino/tui-sdd-llm-local

---

## Recent Decisions (Last 60 days)

### AD-001: Go + Ollama como stack (2026-06-06)

**Decision:** Linguagem Go com runtime Ollama para modelos locais
**Reason:** Binário único, performance, distribuição simples; Ollama é o runtime local mais comum
**Trade-off:** Sem abstração multi-provider na v1
**Impact:** Client HTTP/Go para API Ollama; sem suporte a OpenAI/Anthropic na v1

### AD-002: Modelo padrão qwen2.5-coder (2026-06-06)

**Decision:** `qwen2.5-coder` como modelo default via `ollama run`
**Reason:** Bom custo-benefício para geração de código em ambiente local
**Trade-off:** Requer RAM/GPU adequada para o modelo
**Impact:** Config default em `tsll`; usuário pode override via `TSLL_MODEL` ou `~/.tsll/config.yaml`

### AD-003: Aderência à skill tlc-spec-driven (2026-06-06)

**Decision:** tui-sdd-llm-local implementa o workflow da skill já instalada no projeto
**Reason:** Skill define estrutura `.specs/`, fases adaptativas e convenções — evita reinventar
**Trade-off:** Acoplamento à estrutura da skill (versão 2.0.0)
**Impact:** Templates, comandos e prompts seguem referências em `.cursor/skills/tlc-spec-driven/`

### AD-004: Público dev solo (2026-06-06)

**Decision:** UX e escopo otimizados para um único desenvolvedor
**Reason:** Sem necessidade de colaboração multi-usuário na v1
**Trade-off:** Features de time (review, assign, sync) ficam fora
**Impact:** CLI simples, sem auth, sem servidor

### AD-005: Linux apenas (2026-06-06)

**Decision:** Suporte exclusivo a Linux na v1
**Reason:** Ambiente de desenvolvimento e deploy do autor; simplifica testes e distribuição
**Trade-off:** Sem builds ou testes para macOS/Windows
**Impact:** Pode usar APIs e paths específicos de Linux; CI e releases focados em Linux

### AD-006: CLI interativa estilo k9s (2026-06-06)

**Decision:** `tsll` sem args abre dashboard TUI full-screen; subcomandos permanecem em Cobra
**Reason:** Usuário quer visibilidade de tokens, modelo e GPU numa interface interativa familiar (k9s)
**Trade-off:** Mais complexidade que CLI plain-only; requer Bubble Tea + leitura Ollama/GPU no scaffold
**Impact:** `internal/tui/` como pacote central; lipgloss-only (AD anterior) revogado

### AD-007: Root model como router, views como funções puras (2026-06-06)

**Decision:** RootModel roteia mensagens; view renderers são funções puras (não sub-models Bubble Tea)
**Reason:** Menos boilerplate no scaffold; services testáveis sem TTY
**Trade-off:** Menos reutilização de bubbles components com estado próprio
**Impact:** `internal/tui/views/*.go` recebem structs de dados; I/O isolado em `tea.Cmd`

### AD-008: GPU primária AMD/Radeon via sysfs (2026-06-06)

**Decision:** AMD/Radeon é o caminho default; NVIDIA é fallback. Ordem controlada por `TSLL_GPU_PREFER`.
**Reason:** Máquina-alvo do autor tem Radeon (RX 540/550 série Lexa PRO). NVIDIA continua suportada via `nvidia-smi`.
**Trade-off:** Dois adapters distintos (sysfs/rocm-smi vs nvidia-smi CSV) — mais código e parsing.
**Impact:** `internal/gpu/amdgpu.go` lê `/sys/class/drm/card*/device`; nomes amigáveis via `lspci -mm` com strip de PCI ID; rocm-smi como fallback.

### AD-009: Monitoramento local via /proc puro stdlib (2026-06-06)

**Decision:** CPU/RAM/disk/load lidos diretamente de `/proc` e `statfs`, sem dependências externas (gopsutil etc).
**Reason:** Linux-only já é constraint; menos deps reduz surface area e simplifica build.
**Trade-off:** Reimplementar parsing trivial; menos features (sem rede, sem por-processo).
**Impact:** `internal/system/system.go` com Monitor stateful (delta CPU). View 5 mostra CPU%, MEM, Swap, Load, Disk com barras.

### AD-011: Workflow spec-driven inteiro na TUI (2026-06-06)

**Decision:** Criar features, gerar spec/tasks, executar tasks e init de projeto direto no dashboard Bubble Tea — subcomandos CLI permanecem para automação.
**Reason:** Usuário quer acompanhar e evoluir features sem sair da interface k9s-style.
**Trade-off:** Forms e ações async no RootModel; log longo exige scroll manual (j/k) se usuário subir do auto-tail.
**Impact:** `internal/tui/interactive.go`, `internal/workflow/`, views Features + FeatureDetail; atalhos `n/s/t/e` e `i` (init).

### AD-010: View System (tecla 5) separada de Metrics (2026-06-06)

**Decision:** Adicionar uma quinta view para recursos do host, mantendo Metrics focado em tokens + GPU.
**Reason:** Cada view com responsabilidade clara e legível em 80×24 sem scroll.
**Trade-off:** Mais teclas para memorizar; mas é estilo k9s (multi-painel).
**Impact:** `internal/tui/views/system.go`, key `5`, `SystemSnapshotMsg`, tick 2s independente do GPU tick.

### AD-012: Brownfield mapping em `.specs/codebase/` (2026-06-07)

**Decision:** Mapear o codebase existente com os 7 artefatos da skill (`STACK`, `ARCHITECTURE`, `CONVENTIONS`, `STRUCTURE`, `TESTING`, `INTEGRATIONS`, `CONCERNS`)
**Reason:** Base documentada para planejar features sem re-explorar o repo a cada sessão
**Trade-off:** Manutenção manual — docs podem ficar stale se código mudar sem atualizar
**Impact:** `.specs/codebase/*.md`; carregar on-demand antes de Specify/Design/Tasks

### AD-013: Monitor strip + layout unificado em todas as telas TUI (2026-06-07)

**Decision:** Faixa "Monitor" (Ollama/GPU/tokens) visível em Form, Action e Feature Detail via `composePage`
**Reason:** Usuário precisa ver modelo e GPU durante geração de spec/tasks
**Trade-off:** Menos altura para painel principal; polling Ollama a cada 2s
**Impact:** `internal/tui/page.go`, `internal/tui/views/monitor.go`, `tickOllamaCmd`

### AD-015: Renomeação bel-cli → tui-sdd-llm-local (2026-06-07)

**Decision:** Projeto renomeado; binário/comando `tsll`; módulo `github.com/pedrobelmino/tui-sdd-llm-local`; config `~/.tsll/`
**Reason:** Publicação no GitHub do autor; nome descritivo (TUI + spec-driven + LLM local)
**Trade-off:** Breaking change para quem usava `bel` / `~/.bel/` / `BEL_*`
**Impact:** `cmd/tsll/`, README.md, AGENTS.md, imports Go, env `TSLL_*`

### AD-014: Scroll no painel Action durante streaming (2026-06-07)

**Decision:** `j/k`, `g/G` rolam o log da LLM; auto-tail por padrão; `PanelViewport` com wrap de linhas longas
**Reason:** Specs longos não cabem na viewport; usuário não conseguia ver output completo
**Trade-off:** `actionLog` como `string` (append) — cópia por valor no RootModel
**Impact:** `internal/tui/action_scroll.go`, `internal/ui/layout.go` (`PanelViewport`, `WrapLines`)

---

## Active Blockers

_Nenhum blocker ativo._

---

## Lessons Learned

### L-001: `cmd.Execute()` sempre traversa do root (2026-06-06)

Em testes Cobra, `subCmd.SetArgs(["foo"]) + subCmd.Execute()` não executa o subcomando — `ExecuteC()` redireciona para `cmd.Root()`. Usar sempre `rootCmd.SetArgs(["sub", "foo"])`. Sintomas: testes "passavam" por silêncio do root.

### L-002: Testes contra arquivos vivos do projeto são frágeis (2026-06-06)

`internal/project/parse_test.go` originalmente lia a `STATE.md` real do repo, quebrando a cada atualização da própria `STATE.md`. Migrado para fixtures em `t.TempDir()`.

### L-003: lspci `-mm` sem `-nn` não tem trailing PCI ID (2026-06-06)

O parser de nomes AMD precisa só strip-ar `[hex_id]` quando o conteúdo entre `[]` for 4-hex; caso contrário preserva nomes descritivos como `[Radeon RX 6700]`.

### L-004: `http.Client.Timeout` mata streaming Ollama (2026-06-07)

`GenerateClient` não pode reutilizar `clientTimeout` (5s) do metadata client — streaming pode levar minutos. Usar `http.Client{}` sem timeout global; deadline só via `context.WithTimeout` (10 min).

### L-005: `strings.Builder` em struct com value receiver causa panic (2026-06-07)

`RootModel.Update` copia o struct a cada mensagem Bubble Tea. Após primeiro `WriteString`, copiar `strings.Builder` dispara `illegal use of non-zero Builder copied by value`. Usar `string` para `actionLog` ou migrar para pointer receiver.

---

## Quick Tasks Completed

| #   | Description                                              | Date       | Commit | Status   |
| --- | -------------------------------------------------------- | ---------- | ------ | -------- |
| 1   | Fix parse_test.go to use fixture instead of live STATE.md| 2026-06-06 | —      | ✅ Done  |
| 2   | Fix metrics test: drop redundant `(vendor)` in device line| 2026-06-06 | —      | ✅ Done  |
| 3   | Header GPU mini-line uppercase "GPU" / vendor            | 2026-06-06 | —      | ✅ Done  |
| 4   | Move nvidia tests to `queryNVIDIALegacy` (AMD-first env) | 2026-06-06 | —      | ✅ Done  |
| 5   | Fix cmd tests to go through rootCmd                      | 2026-06-06 | —      | ✅ Done  |
| 6   | Add internal/system: CPU/Mem/Swap/Load/Disk via /proc    | 2026-06-06 | —      | ✅ Done  |
| 7   | Add views.RenderSystem + System view key `5`             | 2026-06-06 | —      | ✅ Done  |
| 8   | Wire SystemSnapshotMsg, fetchSystemCmd, tickSystemCmd    | 2026-06-06 | —      | ✅ Done  |
| 9   | Header mini-line shows CPU/MEM/GPU                       | 2026-06-06 | —      | ✅ Done  |
| 10  | AMD device-name via lspci -mm with hex-ID strip          | 2026-06-06 | —      | ✅ Done  |
| 11  | TUI interactive: features, detail, specify/tasks/run    | 2026-06-06 | —      | ✅ Done  |
| 12  | TUI init project (i) + workflow.InitProject shared       | 2026-06-06 | —      | ✅ Done  |
| 13  | Fix Ollama generate client timeout (5s → context-based)  | 2026-06-07 | —      | ✅ Done  |
| 14  | Fix actionLog panic (strings.Builder → string)           | 2026-06-07 | —      | ✅ Done  |
| 15  | Monitor strip em todas as telas + tick Ollama 2s         | 2026-06-07 | —      | ✅ Done  |
| 16  | Scroll j/k/g/G no painel Action durante streaming        | 2026-06-07 | —      | ✅ Done  |
| 17  | Brownfield mapping: 7 docs em `.specs/codebase/`         | 2026-06-07 | —      | ✅ Done  |
| 18  | Rename bel-cli → tui-sdd-llm-local + README + AGENTS.md  | 2026-06-07 | —      | ✅ Done  |

---

## Deferred Ideas

- [ ] Suporte a llama.cpp / LM Studio como providers alternativos
- [ ] `tsll validate` com UAT interativo (P3)
- [ ] Commits atômicos automáticos por tarefa em `tsll run`
- [ ] Persistência de contadores de tokens em `~/.tsll/tokens.json` para sessões cross-process
- [ ] Métricas por-processo (RSS do `ollama serve`, child workers do modelo)
- [ ] Configuração de tema (dark / light) via `~/.tsll/config.yaml`
- [ ] Overlay modal para erros longos (banner trunca atualmente)
- [ ] Painel network I/O (se vier a ser útil para Ollama remoto)

---

## Todos

- [x] Confirmar restrições de plataforma — Linux apenas (AD-005)
- [x] Definir biblioteca de UX terminal — bubbletea + lipgloss + bubbles (AD-006)
- [x] Especificar feature M1: CLI Scaffold
- [x] Design + tasks + implement cli-scaffold (T1-T22)
- [x] Implement `tsll init` (M1)
- [x] Implement `tsll specify` + prompts (M2)
- [x] Implement `tsll tasks` (M2)
- [x] Implement `tsll run` + state updates (M3)
- [x] Implement `tsll doctor` + `tsll validate` (M4)
- [x] Implement Radeon/AMD GPU pipeline (M4)
- [x] Implement System view + resource monitoring (M4)
- [x] TUI interactive workflow (features, tasks, run, init)
- [x] Brownfield mapping — `.specs/codebase/` (7 artefatos)
- [ ] Refatorar CLI specify/tasks/run para delegar 100% a `workflow.Service` (ver CONCERNS.md)
- [ ] Testes para `internal/workflow/` (ver CONCERNS.md)
- [ ] Tema dark/light configurável
- [ ] Persistência cross-process de tokens
