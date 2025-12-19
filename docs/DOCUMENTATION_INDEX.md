# QueryOps Documentation Index

This directory contains comprehensive architectural analysis of the QueryOps codebase.

## Generated Documentation

### 1. ARCHITECTURE_ANALYSIS.md (870 lines)
**Comprehensive deep-dive into the architecture**

Contains:
- Executive summary of SDUI paradigm
- Datastar integration & configuration
- HTML/template patterns with Datastar
- Backend handler communication patterns (5 patterns identified)
- Design patterns evident (8 secondary patterns plus primary SDUI)
- State management across 4 layers
- Middleware & routing patterns
- Complete data attribute reference
- Architectural strengths & philosophy
- Summary architectural model diagram

**Best for**: Understanding the overall design philosophy and all patterns

### 2. QUICK_REFERENCE.md (550+ lines)
**Developer cheat sheet and quick start guide**

Contains:
- Key concepts at a glance
- Three-tier architecture diagram
- Core building blocks (Datastar attributes, HTTP helpers, patterns)
- State management 4-layer explanation
- Request patterns with examples
- Feature architecture and directory structure
- Common patterns with code (3 complete examples)
- Data binding deep dive
- Session & multi-user isolation
- Development workflow (live reload, tasks)
- Differences from alternatives (HTMX, React, Phoenix LiveView, Next.js)
- Debugging tips
- Performance considerations
- Quick start instructions

**Best for**: Daily development reference and onboarding new developers

### 3. ANTIBOT.md
**Lightweight server-validated anti-bot protection**

Contains:
- Honeypot + JS token + timing checks
- How to add protection to new forms
- DataStar-compatible token injection notes

### 4. TEMPLUI.md
**How we integrated templui components into QueryOps**

Contains:
- templui configuration in this repo (`.templui.json`)
- Where components and JS assets are installed
- How to wire JS-backed components (e.g. dialog) into layouts
- Notes on production `hashfs` vs templui script URLs
- Tailwind/DaisyUI token mapping for templui classnames

### 5. AST_GREP.md
**Structural (AST) search and rewrite with ast-grep**

Contains:
- How to use `ast-grep run` effectively on Go
- Useful patterns for routing/handlers/errors
- Known Go parser gotchas and workarounds
- How to set up `ast-grep scan` via `sgconfig.yml`

### 6. osquery.md
**Integration and management of osquery agents**

Contains:
- Overview of TLS remote API implementation
- Database schema for hosts, logs, and distributed queries
- Local development setup with `cloudflared`
- Instructions for running `osqueryd` against the backend
- Dynamic configuration and live query management

## Key Findings

### Architectural Paradigm
**Server-Driven UI (SDUI) with Reactive Data Binding**

This is a modern evolution of server-side rendering that brings reactive UI capabilities without building a client-side SPA. Most similar to Phoenix LiveView (Elixir).

### Core Technologies
- **Datastar** (v1.0.3) - Client-side reactive framework with signals
- **Templ** - Go template language with type safety
- **Server-Sent Events (SSE)** - Real-time updates via HTTP streaming
- **PostgreSQL** - Per-session state persistence

### Primary Design Patterns
1. **Server-Driven UI (SDUI)** - Server owns all state and logic
2. **Component-Based** - Templ reusable components with type safety
3. **MVC-Adjacent** (backend) - Model/View/Controller loosely applied
4. **Reactive MVVM** (client) - Datastar signals auto-update DOM
5. **Service Layer** - Business logic separation
6. **Repository** - Data access abstraction
7. **Dependency Injection** - Constructor-based wiring
8. **Feature-Based Modular** - Self-contained feature packages

### State Management
Four distinct layers:
1. **Frontend Signals** (ephemeral) - Browser memory
2. **HTML Attributes** (initial) - data-signals seed values
3. **Server Memory** (transient) - Handler/Service runtime structs
4. **Database** (persistent) - PostgreSQL JSONB storage

### Handler Patterns Identified
1. Initial page render (IndexPage)
2. SSE stream initialization (TodosSSE, MonitorEvents)
3. State mutation without response (ToggleTodo)
4. Direct signal patch (IncrementGlobal)
5. Read client signals (SaveEdit)

## File Organization

```
queryops/
├── ARCHITECTURE_ANALYSIS.md      ← Start here for deep understanding
├── QUICK_REFERENCE.md             ← Use during development
├── DOCUMENTATION_INDEX.md          ← This file
├── AGENTS.md                       ← Development guidelines
├── README.md                       ← Setup & deployment
│
├── features/
│   ├── index/                      ← Todo app (complex example)
│   │   ├── handlers.go             ← HTTP request handlers
│   │   ├── routes.go               ← Route registration
│   │   ├── services/               ← Business logic + data access
│   │   ├── components/             ← Reusable Templ components
│   │   └── pages/                  ← Full page Templ templates
│   ├── counter/                    ← Counter app (simple example)
│   ├── monitor/                    ← Monitor app (real-time example)
│   └── common/                     ← Shared components & layouts
│
├── cmd/web/
│   └── web.go                      ← Server setup & initialization
│
├── router/
│   └── router.go                   ← Global router & middleware setup
│
└── db/
    └── db.go                       ← Database connection setup
```

## Reading Order (Recommended)

1. **Start**: QUICK_REFERENCE.md → Section 1-3 (Concepts, Architecture, Building Blocks)
2. **Understand**: ARCHITECTURE_ANALYSIS.md → Section 1-3 (Integration, Templates, Handlers)
3. **Deep Dive**: ARCHITECTURE_ANALYSIS.md → Section 4-5 (Patterns, State Management)
4. **Reference**: Keep both docs open during development
5. **Code Review**: features/counter/ → features/index/ → features/monitor/ (in order of complexity)

## Data Attributes Reference

### State Management
| Attribute | Purpose | Example |
|-----------|---------|---------|
| `data-signals` | Initialize reactive state | `data-signals="{count:0}"` |
| `data-text` | Bind text content | `<span data-text="$count"></span>` |
| `data-class` | Reactive classes | `data-class="{'active':$isActive}"` |
| `data-attr:*` | Reactive attributes | `<img data-attr:src="$url"/>` |
| `data-show` | Conditional visibility | `<div data-show="$isVisible"></div>` |

### Event Handling & Binding
| Attribute | Purpose | Example |
|-----------|---------|---------|
| `data-on:event` | Event handler | `data-on:click="@post('/api')"` |
| `data-bind:*` | Two-way binding | `<input data-bind:input/>` |
| `data-indicator` | Loading indicator | `<button data-indicator="fetching">` |

### Initialization
| Attribute | Purpose | Example |
|-----------|---------|---------|
| `data-init` | Run on element insertion | `<div data-init="@get('/data')"></div>` |

## Common Patterns with Code

### Todo Toggle (3-phase with polling)
```templ
<button data-on:click={ datastar.PostSSE("/api/todos/0/toggle") }>
  Toggle
</button>

<span data-text="$todos[0].text"></span>
```

```go
func (h *Handlers) ToggleTodo(w http.ResponseWriter, r *http.Request) {
  mvc := h.todoService.GetSessionMVC(w, r)
  h.todoService.ToggleTodo(mvc, 0)
  h.todoService.SaveMVC(r.Context(), sessionID, mvc)
  // Handler returns silently
  // Polling detects change and sends patch
}
```

### Counter Increment (Direct signal patch)
```templ
<button data-on:click={ datastar.PostSSE("/counter/increment/global") }>
  +
</button>
<span data-text="$global"></span>
```

```go
func (h *Handlers) IncrementGlobal(w http.ResponseWriter, r *http.Request) {
  h.globalCounter.Add(1)
  datastar.NewSSE(w, r).MarshalAndPatchSignals(
    map[string]interface{}{"global": h.globalCounter.Load()},
  )
}
```

### Real-time Monitor (Continuous streaming)
```templ
<div data-init={ datastar.GetSSE("/monitor/events") }
     data-signals="{memUsed: '', cpuUsed: ''}">
  <p>Memory: <span data-text="$memUsed"></span></p>
</div>
```

```go
func (h *Handlers) MonitorEvents(w http.ResponseWriter, r *http.Request) {
  sse := datastar.NewSSE(w, r)
  ticker := time.NewTicker(time.Second)
  for {
    select {
    case <-ticker.C:
      stats := getSystemStats()
      sse.MarshalAndPatchSignals(stats)
    }
  }
}
```

## Architecture Diagram

```
┌─ BROWSER ────────────────────────────────────────┐
│ Datastar Reactive Engine                        │
│  Signals, Watchers, DOM Morphing                │
│  HTML with data-on:*, data-text, etc            │
│  SSE Listener (auto-patching)                   │
└─────────────────────────────────────────────────┘
       ↑ SSE Stream (HTML patches, signal updates)
       ↓ HTTP Requests (GET /api, POST /action)

┌─ GO SERVER ──────────────────────────────────────┐
│ Features: Handlers, Services, Repository        │
│ Templates: Templ components for rendering       │
│ Middleware: Logging, Recovery                   │
│ Sessions: Cookie-based per-user isolation       │
└─────────────────────────────────────────────────┘
       ↑ Upsert (JSONB state)
       ↓ Select (by session_id)

┌─ DATABASE ───────────────────────────────────────┐
│ PostgreSQL: todos(session_id, state, timestamps)│
└─────────────────────────────────────────────────┘
```

## Comparison Chart

| Aspect | QueryOps | HTMX | React | Phoenix LiveView |
|--------|----------|------|-------|-----------------|
| Rendering | Server-side | Server-side | Client-side | Server-side |
| State | Server-owned | Server-owned | Client-owned | Server-owned |
| Real-time | SSE/polling | Not built-in | Works (3rd party) | WebSocket |
| Framework | Datastar | None + Alpine | React | Phoenix/Elixir |
| Learn curve | Low | Very low | Medium-High | Medium |
| Similar? | YES | Similar | Different | Most similar |

## Performance Notes

- **SSE Polling**: ~1 request/sec per active connection (configurable)
- **DOM Morphing**: <100ms for typical UIs (preserves focus, handlers)
- **Network**: Works through proxies, firewalls (unlike WebSocket)
- **Scalability**: Stateless handlers, scales horizontally with DB

## Next Steps

1. Read QUICK_REFERENCE.md sections 1-6 for immediate understanding
2. Read ARCHITECTURE_ANALYSIS.md section 2-3 for template patterns
3. Review features/counter/ code (simplest example)
4. Review features/index/ code (most complex example)
5. Review features/monitor/ code (real-time example)
6. Modify a simple feature and see hot reload work

Happy coding!
