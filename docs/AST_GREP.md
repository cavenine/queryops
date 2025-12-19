# AST Grep (ast-grep) in QueryOps

This repo includes `ast-grep` for structural (AST-based) searching and rewriting.

Unlike plain-text search (`rg`), `ast-grep` understands language syntax via Tree-sitter, which makes it great for:

- Finding code patterns regardless of formatting
- Safer refactors (rename/replace call sites, wrap errors, update APIs)
- Hunting for “conceptual” shapes (handlers, routes, Render calls)

## Quick Start

### Show help

- `ast-grep run --help`

### Search Go code

- `ast-grep run -l go -p '<PATTERN>' --globs '**/*.go' .`

Useful knobs:

- `--globs` include/exclude paths (supports `!` exclusions)
- `-C 2` add context lines around matches
- `--json` machine-readable output
- `--debug-query=pattern|ast|cst|sexp` inspect how your pattern parses

## Pattern Basics

`ast-grep` patterns look like real code with metavariables.

### Metavariables

- `$X` matches a single node (expression, identifier, string literal, etc.)
- `$$$X` matches a list of nodes (arguments, statements, struct fields, etc.)

Examples:

- Match `fmt.Errorf("...", err)` style wrappers:

  - `ast-grep run -l go -p 'fmt.Errorf($MSG, $ERR)' --globs '**/*.go' .`

- Match template render calls:

  - `ast-grep run -l go -p '$T.Render($CTX, $W)' --globs '**/*.go' features router`

- Match `chi.URLParam(r, "id")` usage:

  - `ast-grep run -l go -p 'chi.URLParam($R, $NAME)' --globs '**/*.go' features router`

### Matching handlers

Find methods shaped like HTTP handlers (receiver + `(w http.ResponseWriter, r *http.Request)`):

- `ast-grep run -l go -p 'func ($RECV *$T) $NAME($W http.ResponseWriter, $R *http.Request) { $$$BODY }' --globs '**/*handlers.go' features`

## Common QueryOps Searches

### Find chi routes

The repo uses `chi.Router` routing heavily.

- All `Get` registrations:
  - `ast-grep run -l go -p '$R.Get($PATH, $HANDLER)' --globs '**/routes.go' .`

- All `Post` registrations:
  - `ast-grep run -l go -p '$R.Post($PATH, $HANDLER)' --globs '**/routes.go' .`

### Find `http.Error` usages

- `ast-grep run -l go -p 'http.Error($W, $MSG, $CODE)' --globs '**/*.go' .`

### Find slog logs (simple, 2-arg style)

- `ast-grep run -l go -p 'slog.$LEVEL($MSG, $ARG)' --globs '**/*.go' features router internal`

Note: slog calls with many args can be matched too, but you’ll want a list metavariable. For example patterns, see "Go gotchas" below.

## Go Gotchas (Tree-sitter + syntax ambiguity)

Go has a syntax ambiguity between:

- A function call: `r.Use(mw)`
- A type conversion: `T(x)`

When your query pattern contains metavariables like `$R.Use($MW)`, the pattern parser can interpret it as a type conversion instead of a call expression, which means it won’t match real `call_expression` nodes in code.

Practical guidance:

- Prefer patterns that are *less ambiguous* (e.g. `router.Get(path, handler)` is reliable).
- If a pattern “should match” but returns nothing, inspect the pattern parse:
  - `ast-grep run -l go --debug-query=ast -p '<PATTERN>'`
- When you hit this ambiguity, fall back to:
  - A different AST shape (e.g. match surrounding code)
  - Or a plain text search (`rg '\\.Use\('`) if you just need locations

## Project Config (`sgconfig.yml`)

This repo includes an `sgconfig.yml` at the root so agents can:

- Add shared rules under `rules/`
- Add rule tests under `rule-tests/`
- Add reusable utilities under `utils/`

You can create new rules/tests with:

- `ast-grep new rule -l go <rule_id>`
- `ast-grep new test <rule_id>`

## Scanning With Rules (optional, but powerful)

Once rules exist in `rules/`, you can run:

- `ast-grep scan .`

To run only a subset of rules by id:

- `ast-grep scan --filter go-http-error-leaks-internal .`

Some rules are intentionally marked `severity: off` (utility rules) to avoid noisy scans.
Run them explicitly with `--include-off`:

- `ast-grep scan --include-off --filter go-chi-route-get .`
- `ast-grep scan --include-off --filter go-chi-route-post .`

Run a single rule file:

- `ast-grep scan --rule rules/<rule_file>.yml .`

Run the repo’s rule tests:

- `ast-grep test --include-off --skip-snapshot-tests`

Prototype a rule without creating a file:

- `ast-grep scan --inline-rules $'id: demo\nlanguage: go\nrule:\n  pattern: fmt.Errorf($MSG, $ERR)\n' .`

## Rules In This Repo

- `go-http-error-leaks-internal`: warns on `http.Error(w, err.Error(), http.StatusInternalServerError)`.
- `go-fmt-errorf-missing-wrap`: warns when `fmt.Errorf` uses `%v/%s/%q` with an `error` argument.
- `go-chi-route-get`: utility rule to enumerate `chi` `Get` routes (off by default).
- `go-chi-route-post`: utility rule to enumerate `chi` `Post` routes (off by default).
