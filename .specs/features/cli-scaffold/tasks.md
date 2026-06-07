# CLI Scaffold Tasks

**Design**: `.specs/features/cli-scaffold/design.md`
**Status**: Done

---

## Testing Convention (greenfield)

No `TESTING.md` exists yet. Gate commands for this feature:

| Gate | Command |
| ---- | ------- |
| **quick** | `go build ./... && go vet ./...` |
| **full** | `go test ./... -count=1` |

All service/view/cmd packages require **co-located unit tests** in the same task.

---

## Execution Plan

### Phase 1: Module & UI Foundation (Sequential)

```
T1 ──→ T2 ──→ T3
```

### Phase 2: Services (Parallel after T1)

```
T1 complete, then:
  ├── T4 [P]  tokens
  ├── T5 [P]  ollama types
  ├── T7 [P]  gpu
  ├── T8 [P]  project detect
  └── T21 [P] Makefile
T5 ──→ T6      ollama client
T8 ──→ T9      project parse
```

### Phase 3: View Renderers (Parallel after T3)

```
T3 complete, then:
  ├── T10 [P] overview view
  ├── T11 [P] features view
  ├── T12 [P] models view
  ├── T13 [P] metrics view
  └── T15 [P] keymap
```

### Phase 4: TUI Core (Sequential)

```
T4,T6,T7,T9,T10-T13,T15 complete, then:
  T14 ──→ T16 ──→ T17 ──→ T18
```

### Phase 5: CLI & Integration (Sequential)

```
T18 complete, then:
  T19 ──→ T20 ──→ T22
```

### Parallel Execution Map

```
Phase 1:  T1 → T2 → T3

Phase 2:  T1 ─┬→ T4 [P]
              ├→ T5 [P] → T6
              ├→ T7 [P]
              ├→ T8 [P] → T9
              └→ T21 [P]

Phase 3:  T3 ─┬→ T10 [P]
              ├→ T11 [P]
              ├→ T12 [P]
              ├→ T13 [P]
              └→ T15 [P]

Phase 4:  T14 → T16 → T17 → T18

Phase 5:  T19 → T20 → T22
```

---

## Task Breakdown

### T1: Initialize Go module and entrypoint

**What**: Create `go.mod`, `cmd/tsll/main.go` calling `cmd.Execute()`
**Where**: `go.mod`, `cmd/tsll/main.go`, stub `internal/cmd/root.go` (returns nil)
**Depends on**: None
**Reuses**: design package structure
**Requirements**: SCAFF-06, SCAFF-15

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Module `github.com/pedrobelmino/tui-sdd-llm-local` with Go 1.22+
- [ ] `go build -o /dev/null ./cmd/tsll` succeeds
- [ ] Gate check passes: `go build ./... && go vet ./...`

**Tests**: none (skeleton only)
**Gate**: quick

**Verify**:
```bash
go build -o bin/tsll ./cmd/tsll && ./bin/tsll 2>/dev/null || true
```

**Commit**: `feat(cli): initialize go module and entrypoint`

---

### T2: UI lipgloss styles (k9s palette)

**What**: Centralized styles — header, error, success, dim, panel border
**Where**: `internal/ui/styles.go`
**Depends on**: T1
**Reuses**: design palette table
**Requirements**: SCAFF-16, SCAFF-17, SCAFF-18

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `StyleHeader`, `StyleError`, `StyleSuccess`, `StyleDim`, `StylePanel` exported
- [ ] `NoColor()` returns unstyled fallback when `NO_COLOR` set
- [ ] Unit test verifies `NoColor()` strips ANSI
- [ ] Gate check passes: `go test ./internal/ui/... -count=1`
- [ ] Test count: ≥2 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(ui): add k9s-inspired lipgloss styles`

---

### T3: UI layout helpers

**What**: `Header`, `Footer`, `Panel`, `BannerError`, `PlainError` functions
**Where**: `internal/ui/layout.go`, `internal/ui/layout_test.go`
**Depends on**: T2
**Reuses**: `internal/ui/styles.go`
**Requirements**: SCAFF-02, SCAFF-03, SCAFF-05, SCAFF-12

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Header renders tabs with active highlight
- [ ] Footer renders keybinding string truncated to width
- [ ] Panel renders titled box with content
- [ ] Snapshot tests for 80-col layout
- [ ] Gate check passes: `go test ./internal/ui/... -count=1`
- [ ] Test count: ≥4 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(ui): add header footer and panel layout helpers`

---

### T4: Token session counter [P]

**What**: `SessionCounter` with `Add()` and `FromOllamaResponse()`
**Where**: `internal/tokens/session.go`, `internal/tokens/session_test.go`
**Depends on**: T1
**Reuses**: design token mapping
**Requirements**: SCAFF-20, SCAFF-21, SCAFF-22, SCAFF-23

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Add(promptEval, evalCount)` accumulates session totals
- [ ] `TotalTokens = PromptTokens + CompletionTokens`
- [ ] `RequestCount` increments per Add
- [ ] Tests cover zero state and multiple adds
- [ ] Gate check passes: `go test ./internal/tokens/... -count=1`
- [ ] Test count: ≥3 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tokens): add session token accumulator`

---

### T5: Ollama API types [P]

**What**: JSON structs for `/api/tags` and `/api/ps` responses
**Where**: `internal/ollama/types.go`, `internal/ollama/types_test.go`
**Depends on**: T1
**Reuses**: Ollama API docs
**Requirements**: SCAFF-25, SCAFF-26

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `ListResponse`, `TagModel`, `ProcessResponse`, `RunningModel` defined
- [ ] JSON unmarshal test with fixture from docs
- [ ] Gate check passes: `go test ./internal/ollama/... -count=1`
- [ ] Test count: ≥2 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(ollama): add API response types`

---

### T6: Ollama HTTP client

**What**: `Client` interface, `NewClient`, `Tags`, `Ps`, `FetchSnapshot`
**Where**: `internal/ollama/client.go`, `internal/ollama/client_test.go`
**Depends on**: T5
**Reuses**: `internal/ollama/types.go`
**Requirements**: SCAFF-25, SCAFF-26, SCAFF-27, SCAFF-28, SCAFF-29

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] HTTP client with 5s timeout, `OLLAMA_HOST` env override
- [ ] `httptest.Server` tests for tags, ps, connection refused
- [ ] `FetchSnapshot` sets `Reachable`, `Error`, `FetchedAt`
- [ ] `DefaultModelMissing` detectable when `qwen2.5-coder` absent
- [ ] Gate check passes: `go test ./internal/ollama/... -count=1`
- [ ] Test count: ≥5 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(ollama): add HTTP client for tags and ps`

---

### T7: NVIDIA GPU query service [P]

**What**: `Query(ctx)` parsing `nvidia-smi` CSV output
**Where**: `internal/gpu/nvidia.go`, `internal/gpu/nvidia_test.go`
**Depends on**: T1
**Reuses**: design nvidia-smi command
**Requirements**: SCAFF-30, SCAFF-31, SCAFF-33, SCAFF-34

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Device` and `Snapshot` structs defined
- [ ] Parse test with fixture CSV (no real GPU required)
- [ ] `LookPath` failure → `Available=false` without error
- [ ] Context timeout 3s honored
- [ ] Gate check passes: `go test ./internal/gpu/... -count=1`
- [ ] Test count: ≥4 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(gpu): add nvidia-smi query parser`

---

### T8: Project detection (walk-up) [P]

**What**: `FindProject(cwd)` with symlink resolution
**Where**: `internal/project/detect.go`, `internal/project/detect_test.go`
**Depends on**: T1
**Reuses**: design detection algorithm
**Requirements**: SCAFF-11, SCAFF-12, SCAFF-14, SCAFF-15

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Detects `.specs/project/PROJECT.md` in cwd or ancestor
- [ ] `Corrupted=true` when dir exists without PROJECT.md
- [ ] Symlink cwd resolved via `EvalSymlinks`
- [ ] Temp dir table tests
- [ ] Gate check passes: `go test ./internal/project/... -count=1`
- [ ] Test count: ≥5 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(project): add tsll project detection`

---

### T9: Project parse and feature listing

**What**: `ParseCurrentWork`, `ParseMilestone`, `ListFeatures`
**Where**: `internal/project/parse.go`, `internal/project/parse_test.go`
**Depends on**: T8
**Reuses**: `.specs/project/*.md` as test fixtures
**Requirements**: SCAFF-08, SCAFF-09, SCAFF-10

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Parses `Current Work` from STATE.md
- [ ] Parses `Current Milestone` from ROADMAP.md
- [ ] `ListFeatures` returns spec/tasks/design badges
- [ ] Gate check passes: `go test ./internal/project/... -count=1`
- [ ] Test count: ≥4 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(project): parse state roadmap and list features`

---

### T10: Overview view renderer [P]

**What**: `RenderOverview(OverviewData)` pure function
**Where**: `internal/tui/views/overview.go`, `internal/tui/views/overview_test.go`
**Depends on**: T3
**Reuses**: `internal/ui/layout.go`
**Requirements**: SCAFF-08, SCAFF-09, SCAFF-14

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Renders valid project: root, current work, milestone
- [ ] Renders invalid project CTA "Press i to init"
- [ ] Renders corrupted project warning
- [ ] Snapshot golden test ≥80 cols
- [ ] Gate check passes: `go test ./internal/tui/views/... -count=1 -run Overview`
- [ ] Test count: ≥3 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tui): add overview view renderer`

---

### T11: Features view renderer [P]

**What**: `RenderFeatures(FeaturesData)` with table of features
**Where**: `internal/tui/views/features.go`, `internal/tui/views/features_test.go`
**Depends on**: T3
**Reuses**: `internal/ui/layout.go`
**Requirements**: SCAFF-10, SCAFF-13

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Lists feature name + spec/tasks/design badges
- [ ] Empty state when no features
- [ ] Snapshot test
- [ ] Gate check passes: `go test ./internal/tui/views/... -count=1 -run Features`
- [ ] Test count: ≥2 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tui): add features view renderer`

---

### T12: Models view renderer [P]

**What**: `RenderModels(ModelsData)` with installed + running tables
**Where**: `internal/tui/views/models.go`, `internal/tui/views/models_test.go`
**Depends on**: T3, T5
**Reuses**: `internal/ollama/types.go`, bubbles/table
**Requirements**: SCAFF-25, SCAFF-26, SCAFF-27, SCAFF-28

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Shows unreachable banner when `Reachable=false`
- [ ] Shows installed models table
- [ ] Shows running models with VRAM and expires_at
- [ ] Warning when default model missing
- [ ] Gate check passes: `go test ./internal/tui/views/... -count=1 -run Models`
- [ ] Test count: ≥4 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tui): add models view renderer`

---

### T13: Metrics view renderer [P]

**What**: `RenderMetrics(MetricsData)` tokens + GPU panels
**Where**: `internal/tui/views/metrics.go`, `internal/tui/views/metrics_test.go`
**Depends on**: T3, T4
**Reuses**: `internal/tokens`, `internal/gpu` types
**Requirements**: SCAFF-20, SCAFF-24, SCAFF-30, SCAFF-32, SCAFF-33

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Token panel: prompt, completion, total, "no requests yet"
- [ ] GPU panel: util%, VRAM, temp per device
- [ ] GPU unavailable graceful message
- [ ] Stale GPU shows timestamp
- [ ] Gate check passes: `go test ./internal/tui/views/... -count=1 -run Metrics`
- [ ] Test count: ≥5 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tui): add metrics view renderer`

---

### T15: TUI keymap and help content [P]

**What**: `KeyMap` struct, help overlay text, footer string
**Where**: `internal/tui/keys.go`, `internal/tui/keys_test.go`
**Depends on**: T3
**Reuses**: design keymap table
**Requirements**: SCAFF-03, SCAFF-04, SCAFF-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] All keys documented: q, ctrl+c, ?, 1-4, r, i
- [ ] `FooterBindings()` returns design string
- [ ] `HelpOverlay()` returns full help text
- [ ] Gate check passes: `go test ./internal/tui/... -count=1 -run Key`
- [ ] Test count: ≥2 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tui): add keymap and help overlay content`

---

### T14: TUI custom message types

**What**: `OllamaSnapshotMsg`, `GPUSnapshotMsg`, `ProjectLoadedMsg`, `TokenUsageMsg`
**Where**: `internal/tui/messages.go`
**Depends on**: T4, T6, T7, T9
**Reuses**: service Snapshot types
**Requirements**: SCAFF-01, SCAFF-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] All message types defined per design
- [ ] Compile-time type check (assign to `tea.Msg`)
- [ ] Gate check passes: `go build ./... && go vet ./...`

**Tests**: none (types only)
**Gate**: quick

**Commit**: `feat(tui): add custom bubbletea message types`

---

### T16: TUI async commands (tea.Cmd factories)

**What**: `loadProjectCmd`, `fetchOllamaCmd`, `fetchGPUCmd`, tick scheduler
**Where**: `internal/tui/commands.go`, `internal/tui/commands_test.go`
**Depends on**: T6, T7, T9, T14
**Reuses**: project, ollama, gpu packages
**Requirements**: SCAFF-06, SCAFF-28, SCAFF-32

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Commands return correct Msg types on success/failure
- [ ] Ollama failure returns Snapshot with `Reachable=false`
- [ ] GPU failure returns stale snapshot
- [ ] Tests invoke cmds and assert msg type (no TTY)
- [ ] Gate check passes: `go test ./internal/tui/... -count=1 -run Cmd`
- [ ] Test count: ≥4 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tui): add async tea commands for data fetching`

---

### T17: RootModel — Update router and View composer

**What**: `RootModel` with Init, Update (keys, resize, msgs, tick), View
**Where**: `internal/tui/model.go`, `internal/tui/model_test.go`
**Depends on**: T10, T11, T12, T13, T15, T16
**Reuses**: all views, keys, commands
**Requirements**: SCAFF-01..14, SCAFF-20..24, SCAFF-29..34

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Keys 1-4 switch `activeView`
- [ ] `q` and `ctrl+c` return `tea.Quit`
- [ ] `?` toggles help overlay
- [ ] `r` triggers refresh batch
- [ ] `i` on Overview shows init hint (no crash)
- [ ] View composes header (with mini GPU), tabs, active view, footer
- [ ] `WindowSizeMsg` updates dimensions
- [ ] Injected msg tests (no `tea.NewProgram`)
- [ ] Gate check passes: `go test ./internal/tui/... -count=1 -run Model`
- [ ] Test count: ≥8 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(tui): add root model with update routing and view composition`

---

### T18: TUI bootstrap (alt-screen program)

**What**: `tui.Run()` with `tea.NewProgram`, `tea.WithAltScreen()`
**Where**: `internal/tui/app.go`
**Depends on**: T17
**Reuses**: `internal/tui/model.go`
**Requirements**: SCAFF-01, SCAFF-07

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Run()` starts program and returns error from tea
- [ ] Alt-screen enabled
- [ ] Gate check passes: `go build ./... && go vet ./...`

**Tests**: none (manual smoke in T22)
**Gate**: quick

**Commit**: `feat(tui): add bubbletea program bootstrap`

---

### T19: Cobra root — dispatch TUI vs plain

**What**: `Execute`, `ShouldLaunchTUI`, version flag, plain mode
**Where**: `internal/cmd/root.go`, `internal/cmd/root_test.go`
**Depends on**: T18, T3
**Reuses**: `internal/tui`, `internal/ui`
**Requirements**: SCAFF-15..19, SCAFF-35..39

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `ShouldLaunchTUI`: TTY + no subcmd + `TSLL_TUI!=0`
- [ ] Non-TTY prints plain hint, exit 0
- [ ] `TSLL_TUI=0` prints help summary
- [ ] `--version` prints semver via ldflags var
- [ ] `--help` lists subcommands + TUI note
- [ ] Table tests for ShouldLaunchTUI matrix
- [ ] Gate check passes: `go test ./internal/cmd/... -count=1 -run Launch`
- [ ] Test count: ≥5 tests pass

**Tests**: unit
**Gate**: full

**Commit**: `feat(cmd): add cobra root with TUI dispatch logic`

---

### T20: Subcommand stubs

**What**: `init`, `specify`, `tasks`, `run` stubs returning exit 1
**Where**: `internal/cmd/init.go`, `specify.go`, `tasks.go`, `run.go`
**Depends on**: T19
**Reuses**: Cobra patterns from root
**Requirements**: SCAFF-10, SCAFF-19

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Each stub prints "not implemented yet"
- [ ] Exit code 1
- [ ] Registered in root command
- [ ] Gate check passes: `go test ./internal/cmd/... -count=1`
- [ ] Test count: ≥4 tests pass (one per stub)

**Tests**: unit
**Gate**: full

**Commit**: `feat(cmd): add stub subcommands init specify tasks run`

---

### T21: Makefile build targets [P]

**What**: `build`, `install`, `test`, `clean` with version ldflags
**Where**: `Makefile`
**Depends on**: T1
**Reuses**: design tech decisions
**Requirements**: SCAFF-40, SCAFF-41, SCAFF-42, SCAFF-43

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `make build` → `bin/tsll` linux/amd64
- [ ] `make test` → `go test ./...`
- [ ] `make clean` removes `bin/`
- [ ] Version injected via `-ldflags`
- [ ] Gate check passes: `make build && ./bin/tsll --version`

**Tests**: none (verified via make targets)
**Gate**: quick

**Commit**: `chore: add Makefile with build install test clean`

---

### T22: Integration smoke verification

**What**: Wire `cmd/tsll/main.go`, run full gate, document smoke checklist
**Where**: `cmd/tsll/main.go` (final wire), update spec traceability
**Depends on**: T19, T20, T21
**Reuses**: all packages
**Requirements**: SCAFF-01..43 (verification)

**Tools**:
- MCP: NONE
- Skill: `tlc-spec-driven` (validate phase)

**Done when**:
- [ ] `make build` produces working `bin/tsll`
- [ ] `./bin/tsll --help` shows subcommands (plain)
- [ ] `./bin/tsll --version` shows version
- [ ] `./bin/tsll init` returns stub message exit 1
- [ ] `echo | ./bin/tsll` prints plain hint (non-TTY)
- [ ] `TSLL_TUI=0 ./bin/tsll` no TUI
- [ ] Full gate: `go test ./... -count=1` all pass
- [ ] All 43 requirements marked Verified in spec traceability

**Tests**: integration (smoke)
**Gate**: full

**Verify**:
```bash
make build
./bin/tsll --help
./bin/tsll --version
./bin/tsll init; echo exit:$?
echo | ./bin/tsll
go test ./... -count=1
# Manual: run ./bin/tsll in TTY — dashboard opens, keys 1-4 work, q quits
```

**Commit**: `feat(cli): complete cli-scaffold integration`

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Go module + main | 2 files | ✅ Granular |
| T2: UI styles | 1 file | ✅ Granular |
| T3: UI layout | 1 package | ✅ Granular |
| T4: Token counter | 1 service | ✅ Granular |
| T5: Ollama types | 1 file | ✅ Granular |
| T6: Ollama client | 1 service | ✅ Granular |
| T7: GPU parser | 1 service | ✅ Granular |
| T8: Project detect | 1 function file | ✅ Granular |
| T9: Project parse | 1 function file | ✅ Granular |
| T10-T13: Views | 1 view each | ✅ Granular |
| T14: Messages | 1 file types | ✅ Granular |
| T15: Keymap | 1 file | ✅ Granular |
| T16: Commands | 1 file | ✅ Granular |
| T17: RootModel | 1 file (cohesive) | ✅ Granular |
| T18: app bootstrap | 1 file | ✅ Granular |
| T19: Cobra root | 1 file | ✅ Granular |
| T20: Stubs | 4 small files | ✅ Granular |
| T21: Makefile | 1 file | ✅ Granular |
| T22: Integration | wire + verify | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (body) | Diagram Shows | Status |
| ---- | ----------------- | ------------- | ------ |
| T1 | None | Phase 1 start | ✅ Match |
| T2 | T1 | T1 → T2 | ✅ Match |
| T3 | T2 | T2 → T3 | ✅ Match |
| T4 | T1 | T1 → T4 [P] | ✅ Match |
| T5 | T1 | T1 → T5 [P] | ✅ Match |
| T6 | T5 | T5 → T6 | ✅ Match |
| T7 | T1 | T1 → T7 [P] | ✅ Match |
| T8 | T1 | T1 → T8 [P] | ✅ Match |
| T9 | T8 | T8 → T9 | ✅ Match |
| T10 | T3 | T3 → T10 [P] | ✅ Match |
| T11 | T3 | T3 → T11 [P] | ✅ Match |
| T12 | T3 | T3 → T12 [P] | ✅ Match |
| T13 | T3 | T3 → T13 [P] | ✅ Match |
| T15 | T3 | T3 → T15 [P] | ✅ Match |
| T14 | T4,T6,T7,T9 | Phase 4 after services | ✅ Match |
| T16 | T6,T7,T9,T14 | T14 → T16 | ✅ Match |
| T17 | T10-T13,T15,T16 | T16 → T17 | ✅ Match |
| T18 | T17 | T17 → T18 | ✅ Match |
| T19 | T18,T3 | T18 → T19 | ✅ Match |
| T20 | T19 | T19 → T20 | ✅ Match |
| T21 | T1 | T1 → T21 [P] | ✅ Match |
| T22 | T19,T20,T21 | T20 → T22 | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| ---- | ---------- | --------------- | --------- | ------ |
| T1 | skeleton | none | none | ✅ OK |
| T2 | ui/styles | unit | unit | ✅ OK |
| T3 | ui/layout | unit | unit | ✅ OK |
| T4 | tokens | unit | unit | ✅ OK |
| T5 | ollama/types | unit | unit | ✅ OK |
| T6 | ollama/client | unit | unit | ✅ OK |
| T7 | gpu | unit | unit | ✅ OK |
| T8 | project/detect | unit | unit | ✅ OK |
| T9 | project/parse | unit | unit | ✅ OK |
| T10-T13 | tui/views | unit | unit | ✅ OK |
| T14 | tui/messages | none (types) | none | ✅ OK |
| T15 | tui/keys | unit | unit | ✅ OK |
| T16 | tui/commands | unit | unit | ✅ OK |
| T17 | tui/model | unit | unit | ✅ OK |
| T18 | tui/app | none (smoke T22) | none | ✅ OK |
| T19 | cmd/root | unit | unit | ✅ OK |
| T20 | cmd/stubs | unit | unit | ✅ OK |
| T21 | Makefile | none | none | ✅ OK |
| T22 | integration | full gate | integration | ✅ OK |

---

## Requirement Traceability (Task → Spec)

| Task | Requirements |
| ---- | ------------ |
| T1 | SCAFF-06, 15 |
| T2 | SCAFF-16, 17, 18 |
| T3 | SCAFF-02, 03, 05, 12 |
| T4 | SCAFF-20..23 |
| T5-T6 | SCAFF-25..29 |
| T7 | SCAFF-30, 31, 33, 34 |
| T8-T9 | SCAFF-08..12, 14 |
| T10 | SCAFF-08, 09, 14 |
| T11 | SCAFF-10, 13 |
| T12 | SCAFF-25..28 |
| T13 | SCAFF-20, 24, 30..33 |
| T15 | SCAFF-03..05 |
| T14-T18 | SCAFF-01..07, 20..24, 29..34 |
| T19-T20 | SCAFF-10, 15..19, 35..39 |
| T21 | SCAFF-40..43 |
| T22 | SCAFF-01..43 |

**Coverage:** 43 requirements → 22 tasks. 0 unmapped ✅

---

## Tools for Execute (pre-asked)

| Task group | MCPs | Skills |
| ---------- | ---- | ------ |
| T1-T22 | NONE | NONE (T22 may use tlc-spec-driven validate) |

---

## Status Tracker

| Task | Status | Commit |
| ---- | ------ | ------ |
| T1 | ✅ Done | feat(cli): initialize go module |
| T2 | ✅ Done | feat(ui): lipgloss styles |
| T3 | ✅ Done | feat(ui): layout helpers |
| T4 | ✅ Done | feat(tokens): session counter |
| T5 | ✅ Done | feat(ollama): API types |
| T6 | ✅ Done | feat(ollama): HTTP client |
| T7 | ✅ Done | feat(gpu): nvidia-smi parser |
| T8 | ✅ Done | feat(project): detection |
| T9 | ✅ Done | feat(project): parse + features |
| T10 | ✅ Done | feat(tui): overview view |
| T11 | ✅ Done | feat(tui): features view |
| T12 | ✅ Done | feat(tui): models view |
| T13 | ✅ Done | feat(tui): metrics view |
| T14 | ✅ Done | feat(tui): message types |
| T15 | ✅ Done | feat(tui): keymap |
| T16 | ✅ Done | feat(tui): async commands |
| T17 | ✅ Done | feat(tui): root model |
| T18 | ✅ Done | feat(tui): app bootstrap |
| T19 | ✅ Done | feat(cmd): cobra root |
| T20 | ✅ Done | feat(cmd): stub subcommands |
| T21 | ✅ Done | chore: Makefile |
| T22 | ✅ Done | feat(cli): integration complete |
