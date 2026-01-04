# Task: Codebase Skills Discovery & Recommendation

## Objective

Analyze this codebase and recommend **5–10 high-impact, reusable Agent Skills** that will accelerate future development work (refactoring, feature development, bug fixing, and test writing).

---

## What Are Agent Skills?

Agent Skills are modular, executable workflows packaged as folders containing:

| File | Purpose |
|------|---------|
| `SKILL.md` | Step-by-step instructions for executing a recurring task |
| `reference.md` | Conventions, APIs, or patterns the skill relies on |
| `examples.md` | 2–3 worked examples from the codebase |
| `scripts/` | Python, Node, or Bash helpers for deterministic automation |

Skills are activated by **semantic intent**—when a developer describes a task, the agent matches the description to the appropriate skill and executes the workflow.

---

## Discovery Process

Complete these steps in order:

### Step 1: Scan Project Structure

- Walk the directory tree and note file organization, naming conventions, and module boundaries.
- Identify the primary language(s), frameworks, and architectural patterns.

### Step 2: Read Key Files

- **Project config**: `README.md`, `package.json`, `go.mod`, `Gemfile`, `pyproject.toml`
- **Build/test configs**: `Makefile`, `pytest.ini`, `jest.config.js`, `.github/workflows/`
- **Architecture docs**: Any `docs/` folder or architecture decision records

### Step 3: Extract Recurring Patterns

Search for code that repeats across modules:
- Error handling and validation patterns
- Test setup, fixtures, or mock data generation
- Data transformation or migration logic
- Dependency injection or configuration patterns
- Logging, auth, or middleware conventions

### Step 4: Identify Development Friction Points

Look for boilerplate and pain points:
- Entity/model creation (CRUD scaffolding)
- API endpoint setup with standard middleware
- Database migrations or schema changes
- Comments like `TODO`, `FIXME`, or repeated inline explanations

### Step 5: Map to Development Scenarios

For each pattern, determine which workflow it accelerates:

| Scenario | Example Skill |
|----------|---------------|
| **Refactoring** | "Rename a database column across schema, ORM, migrations, and tests" |
| **Feature Development** | "Add a new API endpoint with auth, validation, logging, and tests" |
| **Bug Fixing** | "Trace a production error, generate a regression test, deploy a fix" |
| **Test Writing** | "Generate unit test fixtures and integration test setups" |

### Step 6: Prioritize

Rank candidate skills by:
1. **Impact**: How much time or errors does this save?
2. **Frequency**: How often is this task performed?

Select the top **5–10** skills.

---

## Output Format

For each recommended skill, provide the following structure:

```
### Skill: [Name]

**Purpose:**  
One-sentence description of what this skill does.

**When to Use:**  
Scenario or trigger that activates this skill.

**Workflow Steps:**
1. [Action]
2. [Action]
3. ...
N. [Expected Output]

**Tools & Scripts Needed:**

| Script/Tool | Description | Input | Output |
|-------------|-------------|-------|--------|
| `scripts/[name]` | What it does | Expected input | Generated artifact |
| [Dependency] | e.g., TypeORM CLI, Docker | — | — |

**Key Files to Create:**

- `reference.md`: Document conventions, API structures, or error handling patterns this skill relies on.
- `examples.md`: Include 2–3 worked examples from the codebase.
- `scripts/[helper]`: Automate deterministic or complex subtasks (parsing, code generation, testing).

**Integration Notes:**
- Dependencies on other skills: [if any]
- Prerequisites: [e.g., "Requires database connection"]
- **Estimated time saved per use:** [X minutes]

**Example Invocation:**  
> "[Natural language request a developer would make to trigger this skill]"
```

---

## Quality Checklist

Before finalizing, verify each skill:

- [ ] Solves a real, recurring problem observed in this codebase
- [ ] Description is specific enough for intent matching
- [ ] Workflow steps are deterministic and partially automatable
- [ ] Scripts are practical (not toy examples)
- [ ] Integrates cleanly with existing tools (no conflicting dependencies)
- [ ] Includes a concrete example derived from this codebase
- [ ] Time savings estimate is realistic

---

## Example Skill (Reference)

### Skill: Generate API Endpoint Scaffold

**Purpose:**  
Bootstrap a new REST endpoint with route, handler, validation, middleware, logging, and test stub.

**When to Use:**  
"Create a new GET endpoint for fetching users" or "Add a POST endpoint for creating a resource"

**Workflow Steps:**
1. Parse the request (HTTP method, path, query params, body schema)
2. Generate route handler file using template in `examples.md`
3. Auto-wire validation middleware (matching `src/middleware/` patterns)
4. Add structured logging (matching existing logger config)
5. Generate test stub in `__tests__/` with mocks
6. Run `npm run lint` to fix formatting
7. **Output:** Fully formed endpoint ready for business logic

**Tools & Scripts Needed:**

| Script/Tool | Description | Input | Output |
|-------------|-------------|-------|--------|
| `scripts/scaffold-endpoint.js` | Template-based code generator | Endpoint name, method, schema | Handler file, test file, route registration |
| `joi` or `zod` | Existing validation library | — | — |

**Key Files to Create:**

- `reference.md`: Logging conventions, error codes, middleware order
- `examples.md`: Three example endpoints (GET, POST, DELETE) with auth patterns
- `scripts/scaffold-endpoint.js`: Handlebars template + Node script

**Integration Notes:**
- Depends on project's test setup (Jest vs. Mocha)
- Requires route registration file to already exist
- **Estimated time saved per use:** 10–15 minutes

**Example Invocation:**  
> "Create a new GET /api/products endpoint that returns a paginated list"

---

## Deliverable

Produce a markdown document containing:

1. **Project Summary**: Language(s), frameworks, key dependencies, architecture overview
2. **5–10 Recommended Skills**: Each following the output format above
3. **Priority Ranking**: Ordered by impact × frequency
```
