# tui-sdd-llm-local

CLI interativa estilo **k9s** para desenvolvimento **spec-driven** com modelos locais via **Ollama**. Dashboard full-screen com monitoramento de GPU, tokens e sistema — 100% offline no Linux.

**Comando:** `tsll` (abreviação de *TUI Spec-Driven Development — LLM Local*)

## Agradecimentos

Este projeto implementa o workflow da skill **[tlc-spec-driven](https://github.com/tech-leads-club/agent-skills/blob/main/packages/skills-catalog/skills/(development)/tlc-spec-driven/SKILL.md)** do [Tech Lead's Club](https://github.com/tech-leads-club/agent-skills), por Felipe Rodrigues ([@felipfr](https://github.com/felipfr)).

A estrutura `.specs/`, as fases Specify → Design → Tasks → Execute e os artefatos de brownfield mapping seguem as convenções documentadas na skill. Obrigado ao projeto open source por definir um fluxo claro e reutilizável para spec-driven development.

## Requisitos

- **Linux** (v1 — sem macOS/Windows)
- **Go 1.22+** (para build)
- **Ollama** instalado e rodando
- Terminal ANSI, mínimo **80×24**
- GPU AMD ou NVIDIA (opcional — métricas degradam graciosamente)

## Instalação

```bash
git clone https://github.com/pedrobelmino/tui-sdd-llm-local.git
cd tui-sdd-llm-local
make build
sudo cp bin/tsll /usr/local/bin/tsll   # ou: make install
```

## Ollama — subir e verificar

### 1. Instalar Ollama (se ainda não tiver)

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

### 2. Iniciar o serviço

```bash
sudo systemctl enable ollama
sudo systemctl start ollama
sudo systemctl status ollama
```

Ou manualmente: `ollama serve`

### 3. Baixar o modelo padrão

```bash
ollama pull qwen2.5-coder
```

### 4. Verificar que está respondendo

```bash
# Serviço no ar?
curl -s http://127.0.0.1:11434/api/tags

# Modelo carregado na GPU/VRAM?
ollama run qwen2.5-coder "diga ok"
curl -s http://127.0.0.1:11434/api/ps
# size_vram > 0 = modelo na memória da GPU

# Manter modelo carregado entre requests
export OLLAMA_KEEP_ALIVE=30m
```

### 5. Checagem completa com tsll

```bash
tsll doctor
```

Saída esperada:

```
✓ Ollama reachable at http://127.0.0.1:11434
✓ model qwen2.5-coder available
✓ GPU [amd] ... — util X% VRAM X/X MiB
✓ .specs/project at /caminho/do/projeto
```

## Uso rápido

```bash
# Dashboard interativo (TUI)
tsll

# Inicializar projeto spec-driven
tsll init

# Subcomandos (automação / scripts)
tsll specify minha-feature --brief "descrição"
tsll tasks minha-feature
tsll run minha-feature T1
tsll validate
tsll doctor
```

### Atalhos no dashboard

| Tecla | Ação |
|-------|------|
| `1`–`5` | Overview, Features, Models, Metrics, System |
| `n` | Nova feature |
| `s` / `t` | Gerar spec / tasks |
| `e` | Executar task |
| `i` | Init projeto |
| `j` / `k` | Scroll (durante geração LLM) |
| `?` | Ajuda |
| `q` | Sair |

## Configuração

Arquivo global: `~/.tsll/config.yaml`

```yaml
model: qwen2.5-coder
ollama_host: http://127.0.0.1:11434
gpu_prefer: amd   # ou nvidia
theme: k9s
```

Projeto local: `.tsllrc` no diretório do projeto.

Variáveis de ambiente:

| Variável | Efeito |
|----------|--------|
| `OLLAMA_HOST` | URL do Ollama |
| `TSLL_MODEL` | Modelo override |
| `TSLL_GPU_PREFER` | `amd` ou `nvidia` |
| `TSLL_TUI=0` | Desliga TUI no `tsll` sem args |
| `NO_COLOR` | Saída sem cores |

## Estrutura do projeto

```
.specs/
├── project/     # PROJECT.md, ROADMAP.md, STATE.md
├── codebase/    # Brownfield mapping (7 docs)
└── features/    # spec.md, tasks.md por feature
```

Documentação de arquitetura: `.specs/codebase/`

## Desenvolvimento

```bash
make test
make build
go test ./internal/tui/ -v
```

## Licença

Skill `tlc-spec-driven`: [CC-BY-4.0](https://creativecommons.org/licenses/by/4.0/) — Tech Lead's Club.
