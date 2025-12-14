# QueryOps Architecture - Quick Reference Guide

## Key Concepts at a Glance

### 1. What Is QueryOps?
A **Server-Driven UI (SDUI)** application framework combining:
- **Go** backend with type-safe templates (Templ)
- **Datastar** client-side reactive framework
- **Server-Sent Events (SSE)** for real-time updates
- **PostgreSQL** for persistent state storage

**Philosophy**: Server owns the state and UI logic; client renders and reacts to changes.

---

## 2. Three-Tier Architecture

```
FRONTEND (Browser)
├─ Datastar Reactive Engine ($signals)
├─ HTML with data-* attributes
└─ SSE listener (auto-patching)

    ↓ HTTP + SSE ↑

BACKEND (Go Server)
├─ Handlers (HTTP endpoints)
├─ Services (business logic)
├─ Repository (data access)
└─ Templ (template components)

    ↓ SQL ↑

DATABASE (PostgreSQL)
└─ Session state per user
```

---

## 3. Core Building Blocks

### Datastar Attributes (Client)

| What | Attribute | Example |
|------|-----------|---------|
| Initialize state | `data-signals` | `data-signals="{count:0}"` |
| Two-way bind | `data-bind:*` | `<input data-bind:input/>` |
| Show text | `data-text` | `<div data-text="$count"></div>` |
| Toggle class | `data-class` | `data-class="{'active':$isActive}"` |
| Handle event | `data-on:*` | `<button data-on:click="@post('/api')">` |
| Set attribute | `data-attr:*` | `<img data-attr:src="$url"/>` |
| Conditional show | `data-show` | `<div data-show="$isVisible"></div>` |
| Trigger on load | `data-init` | `<div data-init="@get('/data')"></div>` |
| Show loading | `data-indicator` | `<button data-indicator="loading">` |

### HTTP Action Helpers

```go
datastar.GetSSE("/api/todos")        // GET
datastar.PostSSE("/api/todos/toggle") // POST
datastar.PutSSE("/api/todos/mode/1")  // PUT
datastar.PatchSSE("/api/todos/1")     // PATCH
datastar.DeleteSSE("/api/todos/1")    // DELETE
```

### Backend Handler Patterns

```go
// 1. Render initial page (returns HTML)
func (h *Handlers) IndexPage(w http.ResponseWriter, r *http.Request) {
  pages.IndexPage("Title").Render(r.Context(), w)
}

// 2. SSE stream (continuous updates)
func (h *Handlers) TodosSSE(w http.ResponseWriter, r *http.Request) {
  sse := datastar.NewSSE(w, r)
  sse.PatchElementTempl(pages.TodosMVCView(mvc))
  // ... polling loop
}

// 3. State mutation (no response body)
func (h *Handlers) ToggleTodo(w http.ResponseWriter, r *http.Request) {
  h.todoService.ToggleTodo(mvc, idx)
  h.todoService.SaveMVC(ctx, sessionID, mvc)
  // Handler returns silently; polling picks up change
}

// 4. Direct signal patch
func (h *Handlers) Increment(w http.ResponseWriter, r *http.Request) {
  counter.Add(1)
  datastar.NewSSE(w, r).MarshalAndPatchSignals(
    map[string]interface{}{"global": counter.Load()},
  )
}

// 5. Read client signals
func (h *Handlers) SaveEdit(w http.ResponseWriter, r *http.Request) {
  var store struct{ Input string }
  datastar.ReadSignals(r, &store)
  h.todoService.EditTodo(mvc, idx, store.Input)
}
```

---

## 4. State Management

### Four Layers of State

1. **Frontend Signals** (ephemeral)
   - `$count`, `$input`, `$fetching` etc.
   - Live in browser memory
   - Lost on reload
   - Updated by server via SSE

2. **HTML Attributes** (initial)
   - `data-signals="{count: 5}"`
   - Set once on page load
   - Seeds frontend state

3. **Server Memory** (transient)
   - Backend struct: `TodoMVC{Todos, EditingIdx, Mode}`
   - Loaded when SSE starts
   - Compared for changes each poll cycle

4. **Database** (persistent)
   - PostgreSQL `todos` table
   - `(session_id, state)`
   - Upserted on each mutation

### Data Flow Example: Toggle Todo

```
1. Click → data-on:click sends POST to /api/todos/0/toggle
2. Handler receives request + frontend signals (input, fetching)
3. Handler updates backend state (toggle todo at index 0)
4. Handler persists to DB via SaveMVC()
5. Handler returns (no response body)
6. Client shows loading indicator (data-indicator)
7. SSE polling detects state changed
8. Server sends PatchElements with new HTML
9. Client DOM morphs (preserves focus, bindings)
10. Loading indicator disappears automatically
```

---

## 5. Request Patterns

### Single GET Request (Todo App)

```
Initial: GET /
  ↓ Server renders IndexPage()
  ↓ Returns HTML with <div id="todos" data-init="@get('/api/todos')"></div>

User sees page with empty container

Browser runs data-init

Init: GET /api/todos (SSE)
  ↓ Server creates SSE stream
  ↓ Sends initial TodosMVCView (patched HTML)
  ↓ Starts polling state every 1 second
  ↓ Sends PatchSignals when state changes
  ↓ Stream stays open until user leaves

User sees fully populated todo app
```

### User Clicks Button (Counter App)

```
Click: POST /counter/increment/global
  ├─ Body: {global: 5, user: 3} (current signals)
  ├─ Handler: h.globalCounter.Add(1)
  ├─ Handler: sse.MarshalAndPatchSignals({global: 6})
  └─ Response: SSE event "datastar-patch-signals"

Browser receives patch
  ├─ Datastar merges signals: $global = 6
  ├─ DOM auto-updates: <div data-text="$global">6</div>
  └─ User sees new count instantly
```

---

## 6. Feature Architecture

### Directory Structure

```
features/
├── [feature]/
│   ├── handlers.go       # HTTP handlers (request/response)
│   ├── routes.go         # Route registration
│   ├── components/       # Templ components (reusable UI pieces)
│   │   └── *.templ
│   ├── pages/            # Templ pages (full page templates)
│   │   └── *.templ
│   └── services/         # Business logic + data access
│       ├── *_service.go
│       └── *_repository.go
├── common/               # Shared across features
│   ├── components/
│   │   ├── navigation.templ
│   │   └── shared.templ
│   └── layouts/
│       └── base.templ
```

### Initialization Chain

```go
// 1. Route registration
func SetupRoutes(router chi.Router, ...) error {
  repo := services.NewTodoRepository(pool)
  svc := services.NewTodoService(repo, store)
  h := NewHandlers(svc)
  
  router.Get("/", h.IndexPage)
  router.Get("/api/todos", h.TodosSSE)
  return nil
}

// 2. Dependency injection (all the way down)
// Handlers → Services → Repository → DB
```

---

## 7. Common Patterns

### Pattern 1: Button → Handler → DB Update

```templ
<button data-on:click={ datastar.PostSSE("/api/todos/0/toggle") }>
  Toggle
</button>
```

```go
func (h *Handlers) ToggleTodo(w http.ResponseWriter, r *http.Request) {
  sessionID, mvc, _ := h.todoService.GetSessionMVC(w, r)
  idx := chi.URLParam(r, "idx")
  h.todoService.ToggleTodo(mvc, idx)
  h.todoService.SaveMVC(r.Context(), sessionID, mvc)
  // No response; polling detects change and sends patch
}
```

### Pattern 2: Form Input → Signal Binding

```templ
<input data-bind:input placeholder="Add task..."/>
<button data-on:click={ datastar.PutSSE("/api/todos/-1/edit") }>
  Add
</button>
```

```go
func (h *Handlers) SaveEdit(w http.ResponseWriter, r *http.Request) {
  var store struct{ Input string }
  datastar.ReadSignals(r, &store)
  h.todoService.EditTodo(mvc, -1, store.Input)
  h.todoService.SaveMVC(r.Context(), sessionID, mvc)
}
```

### Pattern 3: Real-Time Monitoring

```templ
<div data-init={ datastar.GetSSE("/monitor/events") }
     data-signals="{memUsed: '', cpuUsed: ''}">
  Memory: <span data-text="$memUsed"></span>
</div>
```

```go
func (h *Handlers) MonitorEvents(w http.ResponseWriter, r *http.Request) {
  sse := datastar.NewSSE(w, r)
  ticker := time.NewTicker(time.Second)
  for {
    select {
    case <-ticker.C:
      memStats := getMemory()
      sse.MarshalAndPatchSignals(memStats)
    }
  }
}
```

---

## 8. Data Binding Deep Dive

### Input Binding
```templ
<input id="title" data-bind:input/>
```
- Automatically syncs to `$title`
- Two-way: form → signal, signal → form
- Triggers on `input` and `change` events

### Class Toggle
```templ
<li data-class="{'line-through': $completed}">Item</li>
```
- Add `line-through` class when `$completed === true`
- Reactive: updates when signal changes

### Attribute Binding
```templ
<img data-attr:src="$imageUrl" data-attr:alt="$imageAlt"/>
```
- Update `src` when `$imageUrl` changes
- Update `alt` when `$imageAlt` changes

### Text Content
```templ
<div data-text="$message"></div>
```
- Update innerText when `$message` changes
- Safe from XSS

---

## 9. Session & Multi-User

### Session Creation

```go
sessionStore := sessions.NewCookieStore([]byte(secret))
sessionStore.MaxAge(86400 * 30)  // 30 days
sessionStore.Options.HttpOnly = true
sessionStore.Options.Secure = false
sessionStore.Options.SameSite = http.SameSiteLaxMode
```

### Session Usage

```go
func (s *TodoService) GetSessionMVC(w http.ResponseWriter, r *http.Request) (string, *TodoMVC, error) {
  sess, _ := s.store.Get(r, "connections")
  id, ok := sess.Values["id"].(string)
  if !ok {
    id = toolbelt.NextEncodedID()
    sess.Values["id"] = id
    sess.Save(r, w)
  }
  mvc := s.repo.GetMVC(ctx, id)
  return id, mvc, nil
}
```

- Each user gets unique session ID in cookie
- State stored in DB by session ID
- Concurrent users fully isolated

---

## 10. Development Workflow

### Live Reload

```go
@if config.Global.Environment == config.Dev {
  <div data-init="@get('/reload', {retryMaxCount: 1000, retryInterval:20})"></div>
}
```

```sh
go tool task live
# Watches Go, Templ, CSS files
# Auto-reloads page on save
```

### Tasks

```sh
go tool task build      # Build templ + CSS + wasm
go tool task live       # Dev with live reload
go tool task run        # Run binary
go tool task debug      # Delve debugger
go tool task migrate    # Run DB migrations
```

---

## 11. Key Differences from Alternatives

### vs HTMX + Alpine
- Datastar has integrated signals/state management
- No separate alpine.js needed
- Simpler API

### vs React SPA
- Server renders all HTML
- No client-side router
- No build step (Templ compiles to Go)
- Smaller JS bundle

### vs ASGI/Channels (Django WebSockets)
- SSE instead of WebSocket (simpler, works everywhere)
- Go concurrency model simpler than async/await
- Polling pattern easier to understand

### vs Phoenix LiveView (Elixir)
- Similar architecture!
- Go has better static typing
- Templ syntax familiar to web devs

---

## 12. Debugging Tips

### Check Network Tab
```
GET /api/todos → Pending (SSE stream)
POST /counter/increment/global → No response (1 byte)
```

### Check Browser Console
```javascript
$global  // View current signal
// Or just open DevTools → Console and inspect live signals
```

### Check Handler
```go
log.Printf("sessionID: %s, mvc: %+v", sessionID, mvc)
if err != nil {
  http.Error(w, err.Error(), http.StatusInternalServerError)
}
```

### Check Template Render
```go
// Render to buffer first to catch errors
buf := &bytes.Buffer{}
if err := pages.Counter(store).Render(ctx, buf); err != nil {
  log.Printf("render error: %v", err)
}
```

---

## 13. Performance Considerations

1. **Polling overhead**: 1 request/sec per active connection
   - Solution: Increase ticker interval for non-critical apps
   - Or use direct signal patch for immediate updates

2. **Database upserts**: Every state change hits DB
   - Current approach OK for MVP
   - For scale: Consider event sourcing or change data capture

3. **Browser memory**: All frontend signals kept in memory
   - Not an issue for typical apps
   - Consider cleanup if signals get large

4. **DOM morphing**: Preserves focus, handlers, but slower than virtual DOM
   - Fast enough for most UIs (<100ms)
   - Issue only with massive lists; use pagination

---

## 14. Where Things Live

| What | Where |
|------|-------|
| Server code | `cmd/web/web.go` |
| Routes | `router/router.go` + `features/*/routes.go` |
| Templ components | `features/*/components/*.templ` |
| Templ pages | `features/*/pages/*.templ` |
| Handlers | `features/*/handlers.go` |
| Services | `features/*/services/*.go` |
| CSS/Styles | `web/resources/styles/styles.css` |
| Static assets | `web/resources/static/` |
| Database | PostgreSQL (migrations in `migrations/`) |
| Config | `config/config.go` + `.env` |

---

## 15. Quick Start

```bash
# 1. Install dependencies
go mod tidy

# 2. Set up database
createdb queryops
# Run migrations via: go tool task migrate

# 3. Start dev server
go tool task live

# 4. Edit a component
# features/counter/pages/counter.templ

# 5. Refresh browser
# (auto-reloads via data-init in base.templ)

# 6. See your changes live!
```

