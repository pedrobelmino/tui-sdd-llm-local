# External Integrations

## LLM Runtime

**Service:** Ollama
**Purpose:** Geração de spec, tasks e execução de tasks com modelo local
**Implementation:** `internal/ollama/client.go`, `internal/ollama/generate.go`
**Configuration:**
- Default host: `http://127.0.0.1:11434` (`ollama.DefaultBaseURL`)
- Override: `OLLAMA_HOST` env, `~/.tsll/config.yaml` → `ollama_host`, `.tsllrc`
- Model: `qwen2.5-coder` default; override via `TSLL_MODEL` ou config `model`
**Authentication:** Nenhuma (localhost trust model)

### API Endpoints Used

| Endpoint | Method | Purpose | Client |
|----------|--------|---------|--------|
| `/api/tags` | GET | Modelos instalados | `Client.Tags`, `FetchSnapshot` |
| `/api/ps` | GET | Modelos carregados em VRAM | `Client.Ps`, monitor strip |
| `/api/chat` | POST | Geração (stream/non-stream) | `GenerateClient.ChatStream` |

**Streaming:** NDJSON line-delimited; `stream: true`; timeout via context (10 min) em `generate.go`
**Health check:** `Client.Reachable` — GET `/api/tags` com timeout 5s (metadata client only)

## GPU — AMD / Radeon

**Service:** Linux kernel amdgpu + optional ROCm
**Purpose:** VRAM, utilização, temperatura no dashboard
**Implementation:** `internal/gpu/amdgpu.go`, `internal/gpu/rocm.go`, `internal/gpu/query.go`
**Configuration:** `TSLL_GPU_PREFER=amd` (default) ou `config.GPUPrefer`
**Authentication:** N/A — leitura sysfs e subprocess

**Data sources:**
- `/sys/class/drm/card*/device/gpu_busy_percent`
- `/sys/class/drm/card*/device/mem_info_vram_*`
- `lspci -mm` para nome amigável da GPU
- Fallback: `rocm-smi` parsing

## GPU — NVIDIA

**Service:** NVIDIA driver + `nvidia-smi`
**Purpose:** Mesmas métricas para GPUs NVIDIA
**Implementation:** `internal/gpu/nvidia.go`
**Configuration:** Ativado quando AMD falha ou `TSLL_GPU_PREFER=nvidia`
**Authentication:** N/A

**Data source:** `nvidia-smi --query-gpu=... --format=csv`
**Fixture:** `internal/gpu/testdata/nvidia_smi.csv`

## Host System Metrics

**Service:** Linux `/proc` + `statfs`
**Purpose:** CPU%, memória, swap, load average, disco — View System (tecla 5)
**Implementation:** `internal/system/system.go`
**Configuration:** Nenhuma
**Authentication:** N/A

**Files read:**
- `/proc/stat` (CPU delta via `Monitor` stateful)
- `/proc/meminfo`
- `/proc/loadavg`
- `statfs` em mount points

## Filesystem — Spec Artifacts

**Service:** Local filesystem
**Purpose:** Persistência do workflow spec-driven
**Implementation:** `internal/project/`, `internal/templates/`, `internal/state/`
**Configuration:** Project root via `FindProject()` walk

**Paths written:**
- `.specs/project/{PROJECT,ROADMAP,STATE}.md`
- `.specs/features/<name>/{spec,tasks,design}.md`
- `~/.tsll/config.yaml` (via `config.Save`)

## Skill Reference (Prompts)

**Service:** `.cursor/skills/tlc-spec-driven/`
**Purpose:** System prompts para specify/tasks/run
**Implementation:** `internal/prompts/prompts.go`
**Configuration:** Auto-discovery em `SkillDir(projectRoot)`

**Files loaded:**
- `references/specify.md`
- `references/tasks.md`
- (run prompt inline + spec context)

## API Integrations

Nenhuma API HTTP externa além do Ollama local.

## Webhooks

Nenhum.

## Background Jobs

Nenhum queue system. Polling apenas na TUI:

| Job | Interval | Location |
|-----|----------|----------|
| GPU metrics | 2s | `internal/tui/commands.go` → `tickGPUCmd` |
| System metrics | 2s | `tickSystemCmd` |
| Ollama snapshot | 2s | `tickOllamaCmd` |
| LLM workflow | on-demand | `runWorkflow` goroutine em `interactive.go` |

## Subprocess Dependencies

| Command | Used by | Required |
|---------|---------|----------|
| `ollama serve` (daemon) | All LLM features | Yes |
| `nvidia-smi` | NVIDIA GPU path | If NVIDIA GPU |
| `rocm-smi` | AMD fallback | Optional |
| `lspci` | GPU friendly names | Optional (degrades to PCI ID)
