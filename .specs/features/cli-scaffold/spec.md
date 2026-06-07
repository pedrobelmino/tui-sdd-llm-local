# CLI Scaffold Specification

## Problem Statement

O tui-sdd-llm-local ainda não existe como binário executável. O dev solo precisa de mais que subcomandos estáticos: uma **CLI interativa estilo k9s** que centralize o workflow spec-driven e mostre em tempo real o que está acontecendo com tokens, modelo local e GPU. Sem esse esqueleto TUI + Cobra, não há onde implementar `init`, `specify`, `tasks` ou `run`.

## Goals

- [ ] Binário `tsll` compilável em Linux; `tsll` sem args abre dashboard TUI full-screen
- [ ] Layout k9s-like: header, painéis navegáveis, footer com keybindings
- [ ] Painéis de métricas: tokens, modelo Ollama, GPU (VRAM/utilização)
- [ ] Subcomandos Cobra (`--help`, `--version`, stubs) funcionam fora do modo TUI
- [ ] Detecção automática de projeto tsll (`.specs/project/`)

## Out of Scope

| Feature | Reason |
| ------- | ------ |
| `tsll init` (criar `.specs/project/`) | Feature separada: Project Init |
| Lógica de `tsll specify`, `tsll tasks`, `tsll run` | M2/M3 — apenas stubs ou atalhos no TUI |
| Streaming de geração de código | M3 — Task Runner |
| Métricas AMD/Intel GPU | v1 apenas NVIDIA via `nvidia-smi` |
| Mouse support | Navegação apenas por teclado (como k9s) |
| Instalador / package manager | Distribuição manual via binário na v1 |
| Config file persistente | M4 — Polish |

---

## User Stories

### P1: Dashboard TUI estilo k9s ⭐ MVP

**User Story**: Como dev solo, quero executar `tsll` e entrar num dashboard interativo full-screen, como o k9s, para ver e navegar meu projeto sem decorar subcomandos.

**Why P1**: É o diferencial central da ferramenta — CLI interativa, não apenas comandos.

**Acceptance Criteria**:

1. WHEN o usuário executa `tsll` em TTY THEN o sistema SHALL abrir TUI full-screen ocupando o terminal
2. WHEN a TUI está ativa THEN o sistema SHALL exibir header com nome `tsll`, versão, projeto detectado e modelo ativo
3. WHEN a TUI está ativa THEN o sistema SHALL exibir footer fixo com keybindings (`?` help, `q` quit, `1-4` views, `r` refresh)
4. WHEN o usuário pressiona `q` THEN o sistema SHALL restaurar o terminal e sair com exit code 0
5. WHEN o usuário pressiona `?` THEN o sistema SHALL exibir overlay/modal de ajuda com todos os atalhos
6. WHEN stdout não é TTY (pipe/redirect) THEN o sistema SHALL exibir mensagem plain sugerindo `tsll --help` e NÃO abrir TUI
7. WHEN o terminal é menor que 80×24 THEN o sistema SHALL exibir aviso e degradar layout sem panic

**Independent Test**: `tsll` em terminal interativo abre dashboard; `tsll | cat` exibe mensagem plain.

---

### P1: Views navegáveis (k9s-like) ⭐ MVP

**User Story**: Como dev solo, quero alternar entre views com teclas numéricas para ver projeto, features, modelos e métricas — padrão familiar do k9s.

**Why P1**: Navegação multi-painel é essência da experiência k9s.

**Acceptance Criteria**:

1. WHEN o usuário pressiona `1` THEN o sistema SHALL exibir view **Overview**: status do projeto, `Current Work` de STATE.md, milestone do ROADMAP
2. WHEN o usuário pressiona `2` THEN o sistema SHALL exibir view **Features**: lista de `.specs/features/` com status (spec/tasks)
3. WHEN o usuário pressiona `3` THEN o sistema SHALL exibir view **Models**: modelos Ollama disponíveis e carregados (`/api/tags`, `/api/ps`)
4. WHEN o usuário pressiona `4` THEN o sistema SHALL exibir view **Metrics**: painel consolidado tokens + GPU
5. WHEN view ativa muda THEN o sistema SHALL destacar tab/view atual no header (estilo k9s)
6. WHEN dados estão carregando THEN o sistema SHALL exibir spinner no painel sem bloquear navegação
7. WHEN projeto não inicializado THEN view Overview SHALL exibir CTA "Press `i` to init" (stub → `tsll init`)

**Independent Test**: Navegar `1`→`2`→`3`→`4` no dashboard; cada view renderiza conteúdo distinto.

---

### P1: Estrutura Go — Cobra + Bubble Tea ⭐ MVP

**User Story**: Como dev solo, quero arquitetura que separe modo TUI (`tsll`) de subcomandos (`tsll init`), extensível sem refatoração.

**Why P1**: Dois modos de interação coexistem; estrutura errada gera acoplamento.

**Acceptance Criteria**:

1. WHEN o repositório é clonado THEN o sistema SHALL conter `cmd/tsll/main.go`, `internal/tui/` e `internal/cmd/`
2. WHEN `tsll` é invocado sem subcomando THEN Cobra SHALL delegar para `internal/tui.Run()`
3. WHEN `tsll --version` ou `tsll init` são invocados THEN Cobra SHALL executar subcomando sem iniciar TUI
4. WHEN `go build ./...` é executado THEN o sistema SHALL compilar sem erros
5. WHEN stubs existem (`init`, `specify`, `tasks`, `run`) THEN cada subcomando SHALL retornar "not implemented yet" com exit code 1

**Independent Test**: `tsll` → TUI; `tsll --version` → texto plain; `go build ./...` passa.

---

### P2: Painel de estatísticas de tokens ⭐ MVP

**User Story**: Como dev solo, quero ver quantos tokens consumi na sessão e na última requisição, para controlar uso do modelo local.

**Why P2**: Visibilidade de tokens é requisito explícito do usuário.

**Acceptance Criteria**:

1. WHEN view Metrics (`4`) está ativa THEN o sistema SHALL exibir contadores: `prompt_tokens`, `completion_tokens`, `total_tokens` da sessão
2. WHEN uma requisição Ollama completa (futuro `tsll run`; mock OK no scaffold) THEN o sistema SHALL acumular tokens na sessão TUI
3. WHEN Ollama retorna `prompt_eval_count` e `eval_count` THEN o sistema SHALL mapear para contadores exibidos
4. WHEN nenhuma requisição foi feita na sessão THEN o sistema SHALL exibir zeros com label "no requests yet"
5. WHEN dados de tokens atualizam THEN o sistema SHALL refrescar painel sem sair da TUI (tick ou evento)

**Independent Test**: Simular resposta Ollama com token counts; Metrics view exibe totais corretos.

---

### P2: Painel de uso do modelo local (Ollama)

**User Story**: Como dev solo, quero ver qual modelo está carregado, há quanto tempo, e quanto de RAM consome — como k9s mostra pods.

**Why P2**: Monitorar modelo local evita surpresas de memória e confirma que Ollama está pronto.

**Acceptance Criteria**:

1. WHEN view Models (`3`) está ativa THEN o sistema SHALL listar modelos instalados via `GET /api/tags`
2. WHEN modelos estão carregados em memória THEN o sistema SHALL listar via `GET /api/ps` com nome, size, `expires_at`
3. WHEN Ollama não está rodando THEN o sistema SHALL exibir banner vermelho "Ollama unreachable" com hint `systemctl status ollama`
4. WHEN usuário pressiona `r` THEN o sistema SHALL re-fetch dados Ollama sem reiniciar TUI
5. WHEN modelo padrão `qwen2.5-coder` está ausente THEN o sistema SHALL destacar warning "model not pulled"

**Independent Test**: Com Ollama rodando, view Models lista tags; com Ollama parado, exibe erro amigável.

---

### P2: Painel de GPU (NVIDIA)

**User Story**: Como dev solo, quero ver utilização de GPU e VRAM em tempo real para saber se o modelo local está pressionando o hardware.

**Why P2**: Requisito explícito; essencial em máquinas com GPU dedicada.

**Acceptance Criteria**:

1. WHEN view Metrics (`4`) está ativa e `nvidia-smi` disponível THEN o sistema SHALL exibir GPU name, utilização %, VRAM used/total, temperatura
2. WHEN `nvidia-smi` não está instalado ou GPU ausente THEN o sistema SHALL exibir "GPU metrics unavailable" sem erro fatal
3. WHEN TUI está ativa THEN o sistema SHALL atualizar métricas GPU a cada 2s (configurável via tick)
4. WHEN múltiplas GPUs existem THEN o sistema SHALL listar todas em tabela (bubbles/table)
5. WHEN `nvidia-smi` falha (driver error) THEN o sistema SHALL exibir último erro em dim text no painel

**Independent Test**: Em máquina com NVIDIA, Metrics mostra VRAM; sem GPU, degrada graciosamente.

---

### P2: Subcomandos CLI plain (help, version, erros)

**User Story**: Como dev solo, quero `--help` e `--version` em modo plain text para scripts e automação, fora da TUI.

**Why P2**: TUI para humanos; flags para scripts — coexistência necessária.

**Acceptance Criteria**:

1. WHEN `tsll --help` THEN o sistema SHALL exibir help Cobra plain (sem TUI) listando subcomandos e nota "run `tsll` for interactive dashboard"
2. WHEN `tsll --version` THEN o sistema SHALL exibir semver plain
3. WHEN subcomando inválido THEN o sistema SHALL exibir erro plain com sugestões
4. WHEN `NO_COLOR=1` em modo plain THEN o sistema SHALL suprimir ANSI
5. WHEN `TSLL_TUI=0` THEN o sistema SHALL desabilitar TUI mesmo em TTY (fallback para help)

**Independent Test**: `tsll --help`, `NO_COLOR=1 tsll --version`, `TSLL_TUI=0 tsll` funcionam sem TUI.

---

### P3: Makefile e instruções de build

**User Story**: Como dev solo, quero buildar com `make build` sem memorizar flags.

**Acceptance Criteria**:

1. WHEN `make build` THEN o sistema SHALL gerar `bin/tsll` para Linux amd64
2. WHEN `make install` THEN o sistema SHALL copiar para `$(GOBIN)` ou `~/go/bin`
3. WHEN `make test` THEN o sistema SHALL rodar `go test ./...`
4. WHEN `make clean` THEN o sistema SHALL remover `bin/`

**Independent Test**: `make build && ./bin/tsll --version`.

---

## Edge Cases

- WHEN SIGWINCH (resize terminal) THEN TUI SHALL re-render layout sem crash
- WHEN Ollama responde lento (>5s) THEN painel Models SHALL manter spinner e não travar teclado
- WHEN `.specs/project/` existe mas `PROJECT.md` ausente THEN Overview SHALL mostrar "corrupted project"
- WHEN usuário pressiona `Ctrl+C` na TUI THEN SHALL restaurar terminal e sair code 130
- WHEN `nvidia-smi` timeout THEN Metrics GPU SHALL mostrar "stale" com timestamp da última leitura
- WHEN projeto em path com symlink THEN detecção SHALL resolver path real

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------ |
| SCAFF-01..43 | (see tasks.md) | Execute | Verified |

**Coverage:** 43 total, 43 mapped to tasks (T1-T22), 43 verified ✅

---

## Layout Reference (k9s-inspired)

```
┌─ tsll v0.1.0 ── tui-sdd-llm-local ── qwen2.5-coder ─────────────────────────────┐
│ [1] Overview  [2] Features  [3] Models  [4] Metrics                  │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─ Project ──────────────┐  ┌─ Session Tokens ─────────────────┐  │
│   │ ✓ .specs/project/       │  │ Prompt:      1,240              │  │
│   │ Current: cli-scaffold   │  │ Completion:    890              │  │
│   │ Milestone: M1           │  │ Total:       2,130              │  │
│   └─────────────────────────┘  └────────────────────────────────┘  │
│                                                                      │
│   ┌─ GPU ─────────────────────────────────────────────────────────┐  │
│   │ RTX 3060 │ Util: 45% │ VRAM: 6.2/12 GB │ Temp: 62°C          │  │
│   └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│ r: refresh │ 1-4: views │ ?: help │ q: quit                          │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Technical Decisions

| Decision | Choice | Rationale |
| -------- | ------ | --------- |
| TUI framework | `charmbracelet/bubbletea` | Padrão Go para TUIs; usado em muitas CLIs modernas |
| Layout/styling | lipgloss + bubbles/table | Layout k9s-like com tabelas e viewports |
| CLI framework | Cobra | Subcomandos + flags; TUI no default run |
| Ollama client | HTTP REST (`/api/tags`, `/api/ps`) | Leve; sem SDK pesado no scaffold |
| GPU metrics | `nvidia-smi` subprocess | Confiável em Linux/NVIDIA; parsing CSV |
| Default entry | `tsll` → TUI | Como `k9s` — comando sem args abre dashboard |
| Escape hatch | `TSLL_TUI=0` | Scripts/automação sem TUI |
| Refresh interval | 2s GPU, on-demand Ollama (`r`) | Balanceio responsividade vs CPU |

---

## Success Criteria

- [ ] `tsll` abre dashboard full-screen em < 1s em máquina de dev
- [ ] Usuário navega 4 views sem consultar documentação (familiaridade k9s)
- [ ] Tokens, modelo e GPU visíveis na view Metrics com Ollama + NVIDIA disponíveis
- [ ] `tsll --help` funciona para automação sem abrir TUI
- [ ] Zero panics em resize, Ollama down, ou GPU ausente
- [ ] Estrutura `internal/tui/` pronta para integrar ações de `init`/`run` nas views
