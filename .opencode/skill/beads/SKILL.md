---
name: beads
description: AI-native issue tracking with beads CLI for task management, dependencies, and multi-agent coordination
---

# Beads Issue Tracking Skill

Use the Beads CLI for strategic task tracking, dependency management, and multi-agent coordination in this project.

## When to Use

- Planning multi-step work that spans sessions
- Tracking dependencies between tasks
- Coordinating work across multiple agents
- Managing discovered work during implementation
- Checking project status and priorities

## Core Principles

1. **Track strategic work in Beads** - Multi-session tasks, dependencies, discovered work
2. **Use TodoWrite for simple execution** - Single-session, linear task lists
3. **Never leave TODO comments in code** - Use `bd create` instead
4. **Land the plane** - Always `bd sync` and `git push` before ending session

## Essential Commands

### Finding Work

```bash
# Show issues ready to work (no blockers)
bd ready

# All open issues
bd list --status=open

# Your active work
bd list --status=in_progress

# Detailed issue view with dependencies
bd show <id>

# Project statistics
bd stats
```

### Creating Issues

```bash
# Basic task
bd create --title="Add user authentication" --type=task --priority=2

# Bug report
bd create --title="Fix login redirect" --type=bug --priority=1

# Feature request
bd create --title="Add dark mode" --type=feature --priority=3

# Epic (large work item)
bd create --title="Passkey Authentication" --type=epic --priority=1
```

**Priority Scale:**
- `0` or `P0`: Critical (production down)
- `1` or `P1`: High (blocking work)
- `2` or `P2`: Medium (default)
- `3` or `P3`: Low (nice to have)
- `4` or `P4`: Backlog

**Issue Types:** `task`, `bug`, `feature`, `epic`, `chore`

### Updating Issues

```bash
# Claim work (mark in progress)
bd update <id> --status=in_progress

# Assign to someone
bd update <id> --assignee=username

# Change priority
bd update <id> --priority=1

# Add description
bd update <id> --description="Detailed explanation..."
```

### Closing Issues

```bash
# Simple close
bd close <id>

# Close with reason
bd close <id> --reason="Implemented in PR #123"

# Close multiple at once (efficient)
bd close <id1> <id2> <id3>
```

### Managing Dependencies

```bash
# Add dependency (issue depends on depends-on)
bd dep add <issue> <depends-on>

# Example: Tests depend on feature implementation
bd create --title="Implement feature X" --type=feature  # Creates beads-abc
bd create --title="Write tests for X" --type=task       # Creates beads-xyz
bd dep add beads-xyz beads-abc  # Tests blocked by feature

# Show blocked issues
bd blocked

# See what's blocking an issue
bd show <id>
```

### Syncing with Git

```bash
# Sync beads data with remote
bd sync

# Check sync status
bd sync --status

# Check for issues
bd doctor
```

## Workflow Patterns

### Starting a Session

```bash
# 1. Recover context
bd prime

# 2. Find available work
bd ready

# 3. Review issue details
bd show <id>

# 4. Claim work
bd update <id> --status=in_progress
```

### During Work

```bash
# Discovered a new task? Create it immediately
bd create --title="Refactor auth middleware" --type=task

# Found a bug? Track it
bd create --title="Session timeout not working" --type=bug --priority=1

# Add notes for future reference
bd comment <id> -d "Found that X depends on Y, need to refactor first"

# Break down large work
bd create --title="Subtask: Database schema" --type=task
bd dep add <subtask-id> <parent-id>
```

### Completing Work

```bash
# 1. Close completed issues
bd close <id1> <id2> --reason="Implemented and tested"

# 2. Sync beads data
bd sync

# 3. Push to remote
git push
```

### Session End Protocol (CRITICAL)

Before ending any session, run this checklist:

```bash
[ ] git status              # Check what changed
[ ] git add <files>         # Stage code changes
[ ] bd sync                 # Commit beads changes
[ ] git commit -m "..."     # Commit code (reference bd-* IDs)
[ ] bd sync                 # Commit any new beads changes
[ ] git push                # Push to remote
```

**NEVER skip this. Work is not done until pushed.**

## Commit Message Format

Always reference beads IDs in commit messages:

```bash
git commit -m "Add user authentication [bd-abc123]"
git commit -m "Fix login redirect bug [bd-xyz789]"
git commit -m "Implement passkey flow [bd-a05] [bd-pn9]"
```

## Advanced Usage

### Viewing Priority Analysis

```bash
# Get AI-optimized priority recommendations
bv --robot-priority

# Get insights on critical path
bv --robot-insights
```

### Breaking Down Epics

```bash
# Create epic
bd create --title="User Management System" --type=epic --priority=1
# Returns: bd-epic123

# Create child tasks
bd create --title="Database schema" --type=task
bd dep add bd-task1 bd-epic123

bd create --title="API endpoints" --type=task
bd dep add bd-task2 bd-epic123

bd create --title="UI components" --type=task
bd dep add bd-task3 bd-epic123
```

### Parallel Task Creation

When creating many issues, use parallel subagents for efficiency:

```bash
# Run these in parallel (not sequentially)
bd create --title="Task 1" --type=task &
bd create --title="Task 2" --type=task &
bd create --title="Task 3" --type=task &
wait
```

## Integration with Project

### Beads Storage

- Issues stored in `.beads/issues.jsonl`
- Metadata in `.beads/metadata.json`
- Config in `.beads/config.yaml`

### Git Hooks

Beads auto-syncs via git hooks. Manual sync needed when:
- Ending a session
- After creating many issues
- Before switching branches

## Troubleshooting

### Sync Issues

```bash
# Check for problems
bd doctor

# Force sync
bd sync --force
```

### Worktree Conflicts

If `bd sync` fails with worktree error:
```bash
# Commit beads files directly
git add .beads/
git commit -m "Update beads tracking"
```

### Missing Issues

```bash
# Pull latest beads data
git pull
bd sync --status
```

## Best Practices

1. **Be specific in titles** - "Add user authentication" not "Auth stuff"
2. **Set realistic priorities** - Not everything is P0
3. **Track dependencies early** - Prevents blocked work
4. **Close atomically** - Don't batch close at session end
5. **Reference IDs in commits** - Maintains traceability
6. **Use comments for handoff** - Help future agents/developers
