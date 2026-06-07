# Roadmap

**Current Milestone:** M4 — Polish & Integration
**Status:** In Progress

---

## M1 — CLI Foundation — COMPLETE

**Goal:** Dashboard TUI estilo k9s + esqueleto Cobra para subcomandos
**Target:** `tsll` abre dashboard com métricas de tokens, Ollama e GPU

### Features

**CLI Scaffold** — COMPLETE

- Projeto Go com Cobra + Bubble Tea, binário único Linux
- `tsll` sem args → dashboard full-screen (header, views, footer com keybindings)
- Views: Overview, Features, Models (Ollama), Metrics (tokens + GPU), System (CPU/RAM/disk/load)
- Leitura Ollama API (`/api/tags`, `/api/ps`), `nvidia-smi`, sysfs amdgpu, rocm-smi e `/proc` para métricas
- `--help`, `--version` e `TSLL_TUI=0` para modo plain/script

**Project Init (`tsll init`)** — COMPLETE

- Cria `.specs/project/PROJECT.md`, `ROADMAP.md`, `STATE.md`
- Templates alinhados com skill `tlc-spec-driven`
- Modo interativo (Q&A) e modo não-interativo (`--yes` + flags)

---

## M2 — Spec Workflow — COMPLETE

**Goal:** Fluxo completo de especificação e decomposição de features
**Target:** Dev solo vai de ideia a tarefas atômicas sem sair do terminal

### Features

**Feature Specify (`tsll specify`)** — COMPLETE

- Gera `.specs/features/[feature]/spec.md` via modelo local (Ollama chat streaming)
- System prompt carrega referência `tlc-spec-driven/references/specify.md`
- Atualiza `STATE.md → Current Work` após geração
- Imprime contagem de tokens (prompt + completion)

**Task Breakdown (`tsll tasks`)** — COMPLETE

- Gera `.specs/features/[feature]/tasks.md` a partir de `spec.md`
- System prompt carrega `tlc-spec-driven/references/tasks.md`
- Tarefas com `Done when`, dependências e gate checks
- Atualiza `STATE.md → Current Work`

---

## M3 — Local Execution — COMPLETE

**Goal:** Executar tarefas com modelo local e persistir progresso
**Target:** Ciclo fechado: spec → tasks → run → STATE atualizado

### Features

**Ollama Integration** — COMPLETE

- HTTP client com `OLLAMA_HOST` override e timeout
- Endpoints: `/api/tags`, `/api/ps`, `/api/chat` (streaming)
- Modelo padrão configurável (`qwen2.5-coder`)
- Streaming chunk-a-chunk no terminal com captura de `prompt_eval_count` / `eval_count`

**Task Runner (`tsll run`)** — COMPLETE

- Executa task `--task T<n>` extraída de `tasks.md`
- Context loading: spec + task block + coding-principles
- Atualiza status da task em `tasks.md` (✅ Done) e Current Work em `STATE.md`
- Registra AD entry com resumo da execução

**Session Memory** — COMPLETE

- `internal/state` atualiza `Current Work`, AD entries e task status
- `STATE.md` persiste decisões e contexto entre sessões

---

## M4 — Polish & Integration — IN PROGRESS

**Goal:** Experiência refinada, monitoramento completo e aderência à skill tlc-spec-driven
**Target:** tui-sdd-llm-local é a interface natural para spec-driven no terminal

### Features

**tlc-spec-driven Alignment** — COMPLETE

- Comandos mapeiam aos triggers da skill (`init`, `specify`, `tasks`, `run`, `validate`, `doctor`)
- Templates de `.specs/` idênticos à skill v2.0.0
- `internal/prompts` carrega referências instaladas em `.cursor/skills/tlc-spec-driven/references/`

**Config & Doctor** — COMPLETE

- `config.Load()` lê `~/.tsll/config.yaml`, `.tsllrc` e env (`OLLAMA_HOST`, `TSLL_MODEL`, `TSLL_GPU_PREFER`)
- `tsll doctor` checa Ollama, modelo padrão, GPU (NVIDIA/AMD) e estrutura `.specs/`
- `tsll validate <feature>` valida presença de spec/tasks

**Resource Monitoring** — COMPLETE

- View **System** (`5`): CPU%, memória, swap, load average, disco com barras de progresso
- View **Metrics** (`4`): tokens por sessão + último uso + GPU (NVIDIA via nvidia-smi, AMD via sysfs amdgpu + rocm-smi)
- Header com mini status: CPU / MEM / GPU vendor + util + VRAM
- Auto-refresh 2s para GPU e sistema; refresh sob demanda com `r`

**Radeon-First GPU Telemetry** — COMPLETE

- Discovery via `/sys/class/drm/card*/device` (vendor 0x1002 + driver `amdgpu`)
- Métricas: `gpu_busy_percent`, `mem_info_vram_used/total`, `hwmon*/temp1_input`
- Nome amigável via `product_name`, fallback `lspci -mm`, fallback genérico
- Fallback `rocm-smi --json`; depois NVIDIA `nvidia-smi --query-gpu` CSV
- `TSLL_GPU_PREFER=nvidia` força ordem inversa

**UX Polish** — IN PROGRESS

- [x] Footer e help overlay com tecla `5: system`
- [x] Mini header com CPU/MEM/GPU
- [ ] Tema dark/light configurável
- [ ] Overlays modais para erros longos

---

## Future Considerations

- Suporte a outros runtimes (llama.cpp, LM Studio) via abstração de provider
- Modo multi-feature paralelo com sub-agentes
- Integração com git (commits atômicos por tarefa)
- Plugin system para skills customizadas
- `tsll validate` com UAT interativo para features user-facing
- Persistência cross-process de contadores de tokens (`~/.tsll/tokens.json`)
- Métricas por-processo (RSS do Ollama server, child workers)
