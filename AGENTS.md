# Agent Instructions
- Dev server with live reload: `go tool task live`.
- Production build: `go tool task build` (templ/styles/web-components).
- Run server binary: `go tool task run` (runs `./bin/queryops web`); CLI uses `spf13/cobra`.
- Migrations: `go tool task migrate` and related `migrate:*` tasks.
- Lint/typecheck: `go vet ./...` and `staticcheck ./...` (config in `staticcheck.conf`); format: `gofmt -w .`.
- Tests: `go test ./...`; single test: `go test ./path -run TestName`.
- AST search/rewrites: `ast-grep run -l go -p '<pattern>' --globs '**/*.go' .` (see `docs/AST_GREP.md`); project config in `sgconfig.yml`.
- Client assets only: `go tool task build:templ|build:styles|build:wc`.
- Debugging: `go tool task debug` (Delve).
- Imports: stdlib, blank line, external deps, blank line, internal packages.
- Naming: exported PascalCase; unexported camelCase; constants UPPER_SNAKE_CASE.
- Types: favor concrete types; depend on small interfaces at boundaries for testing.
- Errors: always check/return early; wrap with context `fmt.Errorf("doing X: %w", err)`.
- HTTP handlers: avoid panics; use `http.Error` with appropriate status codes.
- Logging: structured `slog` with context fields (route, user, request IDs).
- Concurrency: honor `context.Context` for cancellation; clean up goroutines on shutdown.
- Templates: `.templ` generates `_templ.go`; run `go tool templ generate` (or `live` task) after edits.
- Dependency injection: prefer constructors (e.g., `NewHandlers`) over globals or package-level state.
- Layout: keep files small and feature-focused; name handlers `ResourceAction` where practical.
- No Cursor or Copilot repo rules are present; this file is the primary agent guide.

# Task Management (Beads System)

This project uses the **Beads** ecosystem for task tracking. You have two tools:
1. **`bv` (Beads Viewer)**: The *Reader*. Use this to decide *what* to do (planning, prioritizing).
2. **`bd` (Beads CLI)**: The *Writer*. Use this to *do* the work (create, update, close tasks).

**CRITICAL**: Do not maintain a separate markdown plan. Do not parse `.beads/beads.jsonl` manually. Use the tools below as your source of truth.

## 1. Triage & Planning (Read)
**Always** start here. `bv` uses graph theory to calculate the optimal path.

| Command               | Purpose                                                                     |
|:----------------------|:----------------------------------------------------------------------------|
| `bv --robot-priority` | **Primary Entry Point.** Returns the recommended task order and priorities. |
| `bv --robot-plan`     | Returns parallel execution tracks and dependency chains.                    |
| `bv --robot-insights` | Returns graph metrics (PageRank, critical path, cycles, etc.).              |
| `bv --check-drift`    | Checks if the current state has deviated from the baseline.                 |

*Note: Always use `--robot-*` flags. Running `bv` without flags launches an interactive TUI that will hang your session.*

## 2. Execution & State (Write)
Once you know *what* to do, use `bd` to manage the task lifecycle.

- **View Context**: `bd show <id>` (Read full description and comments)
- **Create Task**: `bd create "Title" -d "Description" -p <priority>`
    - *Constraint:* The `-d` description **MUST** be detailed. Include context, acceptance criteria, or reproduction steps. Do not create empty issues.
- **Close Task**: `bd close <id>`
- **Add Dependency**: `bd dep add <blocker_id> <blocked_id>`
    - *Example:* `bd dep add 5 10` means task 5 blocks task 10.

## 3. The Agent Workflow Loop

1. **Analyze**: Run `bv --robot-priority` to receive your direct orders.
2. **Context**: Run `bd show <id>` to read the specifics of your assigned task.
3. **Refine**: If the task is too large, break it down:
    - `bd create "Subtask A" -d "Implementation details for A..." -p 0`
    - `bd dep add <subtask_id> <original_task_id>`
4. **Act**: Write code, run tests.
5. **Update**: Immediately run `bd close <id>` when finished.

## Rules for Agents
- **Trust the Graph**: If `bv --robot-insights` reports cycles, fixing them is your top priority.
- **Critical Path**: Tasks identified as "Critical Path" by `bv` take precedence over all others.
- **No Hallucinations**: Never invent task IDs. Always read them from `bv` or `bd` output.
- **Atomic Work**: Close tasks as you go. Do not batch-close tasks at the end of a session.
