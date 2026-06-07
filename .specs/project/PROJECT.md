# tui-sdd-llm-local

**Vision:** Uma CLI interativa estilo k9s que guia devs solo na construção de software com spec-driven development, exibindo em tempo real estatísticas de tokens, uso do modelo local (Ollama) e GPU — tudo em um dashboard de terminal full-screen.

**For:** Desenvolvedores solo que querem estrutura, visibilidade de recursos e consistência ao construir software com IA local.

**Solves:** Desenvolvimento assistido por IA costuma ser ad hoc — sem specs rastreáveis, sem memória entre sessões, sem visão do consumo de tokens/GPU, e sem fluxo claro do requisito à implementação. O tui-sdd-llm-local formaliza esse ciclo numa interface interativa, sem dependência de APIs na nuvem.

## Goals

- Reduzir o tempo entre "tenho uma ideia" e "tenho código funcionando" com um fluxo guiado (`init` → `specify` → `tasks` → `run`) acessível via dashboard e subcomandos
- Oferecer visibilidade operacional: tokens consumidos, modelos carregados, VRAM/GPU — como o k9s faz para Kubernetes
- Garantir rastreabilidade: todo requisito tem ID, toda tarefa tem critério de verificação, decisões persistem em `STATE.md`
- Funcionar 100% offline com modelos locais (padrão: `qwen2.5-coder` via Ollama)

## Tech Stack

**Core:**

- Language: Go (latest stable)
- CLI: Cobra (subcomandos) + Bubble Tea (TUI full-screen estilo k9s)
- Styling: lipgloss + bubbles (tabelas, viewports, spinners)
- LLM runtime: Ollama (API local)
- GPU metrics: AMD/Radeon (sysfs `amdgpu` + `rocm-smi` fallback) e NVIDIA (`nvidia-smi`); degrada graciosamente sem GPU
- Host metrics: leitura direta de `/proc` (CPU, memória, swap, load) e `statfs` (disco)

**Key dependencies:**

- `spf13/cobra` — subcomandos (`init`, `specify`, `tasks`, `run`, flags)
- `charmbracelet/bubbletea` — loop TUI interativo
- `charmbracelet/lipgloss` + `charmbracelet/bubbles` — layout k9s-like
- Ollama HTTP API — modelos, sessões, contadores de tokens
- Templates embutidos para `.specs/` (PROJECT, ROADMAP, STATE, spec, tasks)
- Skill `tlc-spec-driven` como referência de workflow e estrutura de docs

## Scope

**v1 includes:**

- `tsll` (sem args) — dashboard TUI interativo estilo k9s
- Painéis de estatísticas: tokens, modelo local (Ollama), GPU (VRAM/utilização)
- Navegação por teclado entre views (projeto, features, modelos, métricas)
- `tsll init` — criar `.specs/project/` (PROJECT.md, ROADMAP.md, STATE.md)
- `tsll specify` — gerar `spec.md` de uma feature com requisitos rastreáveis
- `tsll tasks` — quebrar feature em tarefas atômicas com critérios de verificação
- `tsll run` — executar tarefa com modelo local (Ollama)
- Memória persistente entre sessões via `STATE.md`
- Integração com convenções da skill `tlc-spec-driven`

**Explicitly out of scope:**

- Suporte a múltiplos provedores de LLM (apenas Ollama na v1)
- Interface gráfica web ou desktop (apenas terminal)
- Orquestração multi-agente paralela
- Hospedagem, deploy ou CI/CD
- Suporte a times/colaboração multi-usuário
- Substituição completa do Cursor IDE — tui-sdd-llm-local complementa, não substitui
- Suporte a macOS ou Windows
- Métricas Intel GPU (apenas NVIDIA e AMD/Radeon na v1)

## Constraints

- Timeline: flexível (sem prazo fixo definido)
- Technical: Ollama instalado e rodando localmente; modelo padrão `qwen2.5-coder`; terminal com suporte ANSI (mín. 80×24)
- Platform: Linux apenas (sem suporte a macOS ou Windows na v1)
- Resources: dev solo — simplicidade e manutenibilidade acima de features avançadas
