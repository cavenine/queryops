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

# Task Management & Coordination (Beads System)

This project uses **Beads** for strategic task tracking and agent coordination.

## 1. The AI Workflow Loop

1. **Bootstrap**: Run `bd prime` to recover session context and protocol rules.
2. **Prioritize**:
   - **Strategy**: Run `bv --robot-priority` to identify the "Critical Path" and most impactful work.
   - **Tactics**: Run `bd ready` to see actionable tasks with no blockers.
3. **Commit**: Select a task, review details (`bd show <id>`), and claim it: `bd update <id> --status=in_progress`.
4. **Execute & Discover**:
   - Break down large tasks: `bd create "Subtask"` + `bd dep add <subid> <parentid>`.
   - Record "discovered" work immediately with `bd create`. NEVER leave `// TODO` comments in code.
   - Use `bd comment <id>` to record findings or interim progress for future agents.
5. **Verify & Land**:
   - Run quality gates: `go test ./...`, `go vet ./...`, `staticcheck ./...`.
   - Close work: `bd close <id> --reason "Description of changes and verification results"`.
   - **LAND THE PLANE**: `bd sync` followed by `git push`. (See Session End below).

## 2. Essential Commands for Agents

| Context | Command | Purpose |
|:---|:---|:---|
| **Context** | `bd prime` | Output AI-optimized workflow context and rules. |
| **Strategy** | `bv --robot-priority` | Identify highest impact work via graph analysis (PageRank/Betweenness). |
| **Tactics** | `bd ready` | List issues that are open and have no blockers. |
| **Context** | `bd show <id> --json` | Get detailed, machine-readable issue context and dependencies. |
| **Action** | `bd update <id> --status=in_progress` | Mark your current work so others don't duplicate effort. |
| **Handoff** | `bd comment <id> -d "..."` | Leave notes for the next session or agent. |

## 3. Rules of Engagement
- **Reference IDs**: Every commit message must reference the relevant `bd-*` ID.
- **Trust the Graph**: Fix cycles and "Critical Path" blockers reported by `bv --robot-insights` before any other work.
- **Priority Scale**: Use `0` (Critical/P0) through `4` (Backlog/P4). Default is `2`.
- **Atomic Work**: Close tasks as you go. Do not batch-close tasks at the end of a session.

## 4. Session End: "Landing the Plane" (MANDATORY)
The session is not finished until the "plane is landed" (remote push succeeds).
1. **Sync**: `bd sync` (updates JSONL and commits beads data).
2. **Push**: `git pull --rebase && git push`.
3. **Verify**: `git status` must show "Your branch is up to date with 'origin/main'".
**NEVER exit with unpushed work. It breaks multi-agent coordination.**
