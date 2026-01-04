---
description: Analyze a Go codebase and identify 3-5 relevant skills based on patterns, tech stack, and architecture
---

# Go Codebase Skills Analyzer

You are analyzing a Go codebase to identify and recommend 3-5 skills that would be most relevant for this project. These skills will help AI agents work effectively with this codebase by providing domain-specific guidance, patterns, and conventions.

## Context

$ARGUMENTS

## Analysis Framework

### 1. Architecture & Structure Analysis

Examine project structure to identify:
- **Directory organization**: Is it feature-based, layered, or another pattern?
- **Module organization**: How are packages organized and related?
- **Entry points**: Where does application start (main.go, cmd/, etc.)?
- **Dependency injection patterns**: How are dependencies wired up (constructors, wire, etc.)?
- **Middleware pattern**: How is cross-cutting logic applied (logging, auth, etc.)?

### 2. Technology Stack Analysis

Identify key technologies and libraries:
- **Web framework**: Chi, Gin, Echo, stdlib http, Fiber, etc.
- **Database**: PostgreSQL, MySQL, SQLite, MongoDB, etc. (which driver/ORM?)
- **Database tools**: golang-migrate, sql-migrate, gorm, ent, etc.
- **Frontend/UI**: Templ, html/template, React, Vue, static assets only?
- **Authentication**: JWT, sessions, OAuth, passkeys, etc.
- **Testing**: testify, httptest, testcontainers, ginkgo, etc.
- **Job queues**: River, Celery, sidekiq-go, etc.
- **Configuration**: viper, env, toml, yaml, etc.
- **CLI**: cobra, urfave/cli, etc.

### 3. Code Patterns Analysis

Look for recurring patterns that deserve skill documentation:
- **HTTP handlers**: How are requests handled and responses formatted?
- **Database access**: Repository pattern, ORM, raw SQL, query builders?
- **Error handling**: How are errors propagated and wrapped?
- **Validation**: Struct validation, JSON schema, custom validators?
- **Logging**: slog, logrus, zap, or stdlib? What fields are logged?
- **Context usage**: How is context.Context passed and used for cancellation?
- **Testing patterns**: Table-driven tests, fixtures, mock/stub patterns?
- **Migration patterns**: How is schema evolution handled?
- **API design**: REST, gRPC, GraphQL, event-driven?

### 4. Domain-Specific Patterns

Look for industry or domain-specific conventions:
- **Multi-tenancy**: Organization/user scoping patterns
- **Real-time features**: SSE, WebSockets, pub/sub
- **Cron jobs/schedulers**: How periodic tasks are managed
- **File handling**: Uploads, storage, streaming
- **External integrations**: APIs, webhooks, third-party services
- **Background jobs**: Queue patterns, worker pools

## Recommended Skills Selection

Based on your analysis, recommend 3-5 skills that would provide the most value. For each skill, provide:

### Skill Template

**Skill Name**: [concise name, lowercase with hyphens]

**Description**: [1-2 sentence summary of what the skill covers]

**When to Use**: [when should this skill be loaded]

**Key Patterns**:
1. [Pattern 1]: [brief description]
2. [Pattern 2]: [brief description]
3. [Pattern 3]: [brief description]

**Example Code Snippet**:
```go
// Brief example showing the pattern
```

**Related Files/Paths**:
- `path/to/relevant/file.go`
- `path/to/relevant/directory/`

---

## Output Format

Provide your analysis in this structure:

```markdown
# Codebase Analysis: [Project Name]

## Overview
[Brief summary of project purpose, tech stack, and architecture]

## Architecture Patterns
[Key architectural patterns observed]

## Technology Stack
[List major technologies and versions from go.mod]

## Recommended Skills

### 1. [Skill Name]
**Description**: [...]
**When to Use**: [...]
**Key Patterns**:
- [...]
**Example**:
```go
...
```

### 2. [Skill Name]
[Same structure as above]

### 3. [Skill Name]
[Same structure as above]

[Additional skills as needed]

## File Structure Summary
[Notable directories and their purposes]

## Next Steps
[How to implement these skills in .opencode/skill/ directory]
```

## Examples for Reference

Use these existing skills as templates for formatting and depth:

- **handler**: HTTP handlers with DI and error handling
- **repository**: PostgreSQL data access layer with pgxpool
- **testing**: Unit and integration tests with testcontainers
- **migrations**: Database schema migrations
- **api-endpoint**: REST-like APIs with JSON/SSE

## Guidelines

1. **Be specific**: Don't suggest generic skills. Focus on patterns specific to this codebase.
2. **Prioritize impact**: Focus on skills that would help with 80% of common tasks.
3. **Evidence-based**: Reference actual code from the codebase to justify recommendations.
4. **Actionable**: Each skill should have clear, implementable guidance.
5. **Concise**: Keep descriptions focused and avoid fluff.
6. **Consistency**: Match the tone and structure of existing skills in this project.

## Begin

Start by examining:
1. `go.mod` and `go.sum` to understand dependencies
2. Directory structure and main entry points
3. Key handler/service/repository files
4. Existing test files to understand testing patterns
5. Configuration files and documentation

Then provide your analysis and skill recommendations.
