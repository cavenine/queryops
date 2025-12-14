## Beads Issue Tracking
We use `bd` (beads) as the primary issue tracker for this repo. Agents should use beads instead of ad-hoc markdown TODO lists to track work.

### Issue Conventions
- Types:
  - `bug`: incorrect behavior or regression
  - `feature`: new user-visible capability
  - `task`: refactors, infra, cleanup, docs, etc.
  - `epic`: multi-step work with child tasks
- Priority (0-4, 0 = highest):
  - `0` (P0): urgent breakage, security, or blocked main workflows
  - `1` (P1): important for current milestone
  - `2` (P2): default priority
  - `3-4` (P3/P4): nice-to-have / low
- Status lifecycle:
  - `open`: default when an issue is created
  - `in_progress`: when you are actively working on it
  - `closed`: when done, always with a `--reason` explaining the outcome

### Start of a Session
At the beginning of each session:
- Ensure beads is healthy (optional but recommended):
  - `bd doctor` or `bd status`
- Find work using ready issues:
  - `bd ready --json` to list unblocked work
  - Pick ONE primary `bd-*` issue to focus on
- If the user request does not yet have an issue:
  - Create one first: `bd create "<short title>" -t task -p 2 --json`
- Explicitly reference the primary `bd-*` issue in your initial plan.

### During Work
While working in this repo:
- Stay anchored to a primary issue:
  - Every meaningful code change must be associated with a `bd-*` issue
  - If you are doing work without an issue, pause and create one: `bd create "<short title>" -t task -p 2 --json`
- Keep issue status up to date:
  - When you start working: `bd update <id> --status in_progress --json`
  - If you become blocked (tests failing, missing infra, etc.):
    - Create a blocking issue: `bd create "<short description>" -t bug -p 0 --json`
    - Link as a blocker: `bd dep add <current-id> <blocker-id> --type blocks`
- Track discovered work instead of leaving hidden TODOs:
  - When you find follow-up work, bugs, or refactors:
    - Create a new issue: `bd create "Discovered <thing>" -t bug|task|feature -p <priority> --json`
    - Link it back to the current issue: `bd dep add <new-id> <parent-id> --type discovered-from`
- Use epics for larger efforts where helpful:
  - Create an epic: `bd create "<epic title>" -t epic -p 1 --json`
  - Create child tasks under the epic; use `bd dep tree <epic-id>` to inspect the hierarchy.

### Beads Do and Don't
- Do use `bd create` / `bd update` / `bd close` / `bd dep` to track all non-trivial work.
- Do keep statuses current and dependencies accurate as you go.
- Do use `--json` output when integrating with tools or when plans reference specific details.
- Don't rely on markdown plans or TODO lists as the primary tracker; they are secondary to beads.
- Don't edit files under `.beads/` directly; always use `bd` commands.
- Don't leave a session with untracked work; every follow-up must have a `bd-*` issue.

## Landing the Plane
When the user says "let's land the plane", you MUST complete ALL steps below. The plane is NOT landed until git push succeeds. NEVER stop before pushing. NEVER say "ready to push when you are!" - that is a FAILURE.

MANDATORY WORKFLOW - COMPLETE ALL STEPS:

File or update beads issues for any remaining work that needs follow-up (follow the Beads Issue Tracking guidelines above).

Ensure all quality gates pass (only if code changes were made) - run tests, linters, builds; if something is broken and not fixed, file a P0 `bug` issue with details using `bd create`. 

Update beads issues - close finished work with `bd close ... --reason`, and ensure statuses (`open`/`in_progress`/`closed`) are accurate.

PUSH TO REMOTE - NON-NEGOTIABLE - This step is MANDATORY. Execute ALL commands below:

# Pull first to catch any remote changes
git pull --rebase

# If conflicts in .beads/beads.jsonl, resolve thoughtfully:
#   - git checkout --theirs .beads/beads.jsonl (accept remote)
#   - bd import -i .beads/beads.jsonl (re-import)
#   - Or manual merge, then import

# Sync the database (exports to JSONL, commits)
bd sync

# MANDATORY: Push everything to remote
# DO NOT STOP BEFORE THIS COMMAND COMPLETES
git push

# MANDATORY: Verify push succeeded
git status  # MUST show "up to date with origin/main"
CRITICAL RULES:

The plane has NOT landed until git push completes successfully
NEVER stop before git push - that leaves work stranded locally
NEVER say "ready to push when you are!" - YOU must push, not the user
If git push fails, resolve the issue and retry until it succeeds
The user is managing multiple agents - unpushed work breaks their coordination workflow
Clean up git state - Clear old stashes and prune dead remote branches:

git stash clear                    # Remove old stashes
git remote prune origin            # Clean up deleted remote branches
Verify clean state - Ensure all changes are committed AND PUSHED, no untracked files remain

Choose a follow-up issue for next session

Provide a prompt for the user to give to you in the next session
Format: "Continue work on bd-X: [issue title]. [Brief context about what's been done and what's next]"
REMEMBER: Landing the plane means EVERYTHING is pushed to remote. No exceptions. No "ready when you are". PUSH IT.

Example "land the plane" session:

# 1. File remaining work
bd create "Add integration tests for sync" -t task -p 2 --json

# 2. Run quality gates (only if code changes were made)
go test -short ./...
golangci-lint run ./...

# 3. Close finished issues
bd close bd-42 bd-43 --reason "Completed" --json

# 4. PUSH TO REMOTE - MANDATORY, NO STOPPING BEFORE THIS IS DONE
git pull --rebase
# If conflicts in .beads/beads.jsonl, resolve thoughtfully:
#   - git checkout --theirs .beads/beads.jsonl (accept remote)
#   - bd import -i .beads/beads.jsonl (re-import)
#   - Or manual merge, then import
bd sync        # Export/import/commit
git push       # MANDATORY - THE PLANE IS STILL IN THE AIR UNTIL THIS SUCCEEDS
git status     # MUST verify "up to date with origin/main"

# 5. Clean up git state
git stash clear
git remote prune origin

# 6. Verify everything is clean and pushed
git status

# 7. Choose next work
bd ready --json
bd show bd-44 --json
Then provide the user with:

Summary of what was completed this session
What issues were filed for follow-up
Status of quality gates (all passing / issues filed)
Confirmation that ALL changes have been pushed to remote
Recommended prompt for next session
CRITICAL: Never end a "land the plane" session without successfully pushing. The user is coordinating multiple agents and unpushed work causes severe rebase conflicts.

