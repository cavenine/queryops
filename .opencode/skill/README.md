# QueryOps Agent Skills

This directory contains reusable Agent Skills for accelerating development in the QueryOps codebase.

## Project Summary

| Attribute | Value |
|-----------|-------|
| **Language** | Go 1.25 |
| **Framework** | Chi router, Templ templates, Datastar reactivity |
| **Database** | PostgreSQL 18 with pgxpool |
| **Migrations** | golang-migrate with embedded SQL |
| **Task Runner** | go-task (Taskfile.yml) |
| **Issue Tracking** | Beads (AI-native, git-integrated) |
| **Architecture** | Feature-based modular, Server-Driven UI |

## Available Skills

### 1. migrations
**Purpose:** Create, modify, and manage PostgreSQL database migrations using golang-migrate with idempotent patterns.

**When to use:**
- "Create a new database table for campaigns"
- "Add a column to the hosts table"
- "Write a migration for the new feature"

**Estimated time saved:** 10-15 minutes per migration

---

### 2. beads
**Purpose:** AI-native issue tracking with beads CLI for task management, dependencies, and multi-agent coordination.

**When to use:**
- "Track this work in beads"
- "What tasks are ready to work on?"
- "Create issues for this feature breakdown"

**Estimated time saved:** 5-10 minutes per session

---

### 3. feature
**Purpose:** Scaffold a new feature module with handlers, routes, pages, services, and repository following project conventions.

**When to use:**
- "Create a new feature for campaigns"
- "Add a new module for notifications"
- "Scaffold the reports feature"

**Estimated time saved:** 30-45 minutes per feature

---

### 4. templ
**Purpose:** Create Templ components and pages with Datastar integration for reactive server-driven UI.

**When to use:**
- "Create a new page template"
- "Add a component with Datastar signals"
- "Build an SSE-connected real-time view"

**Estimated time saved:** 15-20 minutes per component

---

### 5. testing
**Purpose:** Write unit and integration tests for handlers, services, and repositories using httptest and testcontainers.

**When to use:**
- "Add tests for the campaign handlers"
- "Write integration tests for the repository"
- "Create table-driven tests for this function"

**Estimated time saved:** 20-30 minutes per test suite

---

### 6. api-endpoint
**Purpose:** Create REST-like API endpoints with JSON responses, SSE streaming, and proper error handling.

**When to use:**
- "Add a new API endpoint for campaigns"
- "Create an SSE endpoint for real-time updates"
- "Build a CRUD API for the new entity"

**Estimated time saved:** 15-20 minutes per endpoint

---

### 7. repository
**Purpose:** Create PostgreSQL repository layer with pgxpool for CRUD operations, transactions, and query patterns.

**When to use:**
- "Create a repository for the campaigns table"
- "Add database queries for the new feature"
- "Implement a batch insert operation"

**Estimated time saved:** 20-30 minutes per repository

---

### 8. handler
**Purpose:** Create HTTP handlers with dependency injection, error handling, and proper response patterns.

**When to use:**
- "Add handlers for the campaign feature"
- "Create form handling for the create page"
- "Implement the detail page handler"

**Estimated time saved:** 10-15 minutes per handler

---

## Priority Ranking (by Impact x Frequency)

| Rank | Skill | Impact | Frequency | Score |
|------|-------|--------|-----------|-------|
| 1 | **feature** | High | Medium | Creating new features is foundational |
| 2 | **migrations** | High | High | Database changes happen frequently |
| 3 | **beads** | Medium | Very High | Every session needs task tracking |
| 4 | **templ** | High | High | UI work is constant |
| 5 | **api-endpoint** | High | Medium | APIs are core to the app |
| 6 | **repository** | Medium | Medium | Data layer is reusable |
| 7 | **handler** | Medium | Medium | Handler patterns are consistent |
| 8 | **testing** | Medium | Medium | Tests ensure quality |

## Usage

Skills are loaded on-demand via the `skill` tool. The agent will automatically discover available skills and can load them when relevant tasks are detected.

**Example invocations:**
- "Create a new campaigns feature with CRUD operations"
- "Write a migration for adding the reports table"
- "Add tests for the osquery handlers"

## Directory Structure

```
.opencode/skill/
├── README.md           # This file
├── migrations/
│   └── SKILL.md        # Database migration skill
├── beads/
│   └── SKILL.md        # Issue tracking skill
├── feature/
│   └── SKILL.md        # Feature scaffolding skill
├── templ/
│   └── SKILL.md        # Templ component skill
├── testing/
│   └── SKILL.md        # Testing skill
├── api-endpoint/
│   └── SKILL.md        # API endpoint skill
├── repository/
│   └── SKILL.md        # Repository pattern skill
└── handler/
    └── SKILL.md        # HTTP handler skill
```

## Adding New Skills

1. Create a directory: `.opencode/skill/<skill-name>/`
2. Add a `SKILL.md` file with YAML frontmatter:
   ```yaml
   ---
   name: skill-name
   description: Brief description (1-1024 chars)
   ---
   ```
3. Write detailed instructions in markdown

**Naming rules:**
- Lowercase alphanumeric with hyphens
- No consecutive hyphens
- 1-64 characters
- Match directory name

## Related Documentation

- [AGENTS.md](/AGENTS.md) - Agent instructions and workflow
- [docs/ARCHITECTURE_ANALYSIS.md](/docs/ARCHITECTURE_ANALYSIS.md) - Architecture overview
- [migrations/README.md](/migrations/README.md) - Migration details
- [.beads/README.md](/.beads/README.md) - Beads documentation
