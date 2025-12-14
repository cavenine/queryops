# QueryOps Architecture Analysis

## Executive Summary

QueryOps is a **Server-Driven UI (SDUI) / Hypermedia-Driven Application** built on a **modern data-binding reactive paradigm**. It uses Datastar as the client-side reactive framework paired with Go templates (Templ) and Server-Sent Events (SSE) to create interactive applications without traditional client-side SPA frameworks. The architecture demonstrates a hybrid approach combining elements of MVC, reactive programming, and event-driven patterns.

---

## 1. Datastar Integration & Configuration

### 1.1 Integration Overview

**Version**: `github.com/starfederation/datastar-go v1.0.3`

Datastar is integrated at three layers:

#### Base Template Configuration (features/common/layouts/base.templ)
```
<script defer type="module" src={ resources.StaticPath("datastar/datastar.js") }></script>
```
- Datastar JS is loaded early in the document lifecycle via module script
- Live reload SSE endpoint configured for development:
  ```
  data-init="@get('/reload', {retryMaxCount: 1000, retryInterval:20, retryMaxWaitMs:200})"
  ```

#### SDK Usage (Backend - Go)
```go
import "github.com/starfederation/datastar-go/datastar"
```
- SDK provides helper functions for:
  - `datastar.NewSSE(w, r)` - Creates SSE response stream
  - `datastar.GetSSE("/path")`, `datastar.PostSSE("/path")`, etc. - URL binding helpers
  - `datastar.PatchElementTempl(component)` - Patches HTML elements via SSE
  - `datastar.MarshalAndPatchSignals(update)` - Patches reactive signals via SSE
  - `datastar.ReadSignals(r, store)` - Reads frontend signals from requests

### 1.2 Client-Side Architecture

Datastar on the client side provides:

1. **Reactive Signal System**: Central reactive store (`$` object in templates)
   - Computed properties with dependency tracking
   - Automatic UI updates on signal changes
   - Signal validation and filtering

2. **Attribute-Based Declarative Syntax**:
   - `data-signals="{key: value}"` - Initialize/update reactive signals
   - `data-bind:input` - Two-way binding to form inputs
   - `data-text="$signalName"` - Bind text content to signals
   - `data-class="{...}"` - Reactive class binding
   - `data-on:event="@action()"` - Event handlers
   - `data-on:click__outside` - Modifier support
   - `data-attr:*="$signalName"` - Reactive attribute binding
   - `data-init="@get(...)"` - Initialization effects

3. **Built-in Actions**:
   - `@get()`, `@post()`, `@put()`, `@patch()`, `@delete()` - HTTP methods
   - Content negotiation based on Accept headers
   - Automatic retry logic with exponential backoff
   - Form data, JSON, and multipart support

---

## 2. HTML/Template Patterns with Datastar

### 2.1 Data Attributes Overview

#### Signals (Reactive State)
```html
<div data-signals="{global:0, user:0, input:''}">
  <div data-text="$global"></div>
</div>
```
- Initialize reactive state in containers
- Bind with `$signalName` references

#### Data Binding
```html
<input data-bind:input placeholder="What needs to be done?"/>
```
- Automatic two-way binding for form elements
- Supports `data-bind:input` (text inputs)
- Supports `data-bind:checked` (checkboxes)
- Supports `data-bind:value` (selects, textareas)

#### Event Handlers
```html
<button data-on:click={ datastar.PostSSE("/counter/increment/global") }>
  Increment Global
</button>
```
- Declarative event handlers using `data-on:eventName`
- HTTP method helpers: `GetSSE()`, `PostSSE()`, `PutSSE()`, `DeleteSSE()`, `PatchSSE()`
- Event modifiers: `__prevent`, `__stop`, `__outside`, `__capture`, `__once`, `__passive`
- Delay/debounce/throttle support: `__debounce.500ms`, `__throttle.1s`, `__delay.100ms`

#### Reactive Text & Classes
```html
<div class="text-2xl" data-text="$global"></div>
<div data-class="{'loading ml-4': $fetching}"></div>
```
- `data-text` - Bind text content
- `data-class` - Reactive class objects
- `data-attr:name` - Reactive attributes

#### Conditional Rendering
```html
<div data-show="$isVisible"></div>
```
- `data-show` hides/shows elements via `display: none`

#### Initialization
```html
<div id="todos-container" data-init={ datastar.GetSSE("/api/todos") }></div>
```
- `data-init` runs on element insertion
- Typically triggers SSE connection
- Runs once per element

### 2.2 Server-Sent Events Pattern

#### SSE Endpoints (Backend)
```go
func (h *Handlers) TodosSSE(w http.ResponseWriter, r *http.Request) {
  sse := datastar.NewSSE(w, r)
  
  // Send initial state
  if err := sse.PatchElementTempl(c); err != nil { ... }
  
  // Continuous updates via ticker
  for {
    select {
    case <-ctx.Done(): return
    case <-ticker.C:
      current, _ := h.todoService.GetMVCBySessionID(ctx, sessionID)
      if err := sse.PatchElementTempl(...); err != nil { return }
    }
  }
}
```

#### Two-Phase Communication:
1. **Initial Render**: GET request returns HTML with `data-signals` and setup
2. **SSE Stream**: Continuous stream of `datastar-patch-elements` and `datastar-patch-signals` events

#### Event Types (Browser receives):
- `datastar-patch-elements` - Replace/patch HTML fragments
- `datastar-patch-signals` - Merge reactive signal updates
- Server can send multiple updates; client merges them

### 2.3 Real-World Examples

#### Todo App Pattern (Counter Example)
```templ
templ CounterPage() {
  @layouts.Base("Counter") {
    <div id="container" data-init={ datastar.GetSSE("/counter/data") }>
    </div>
  }
}

templ Counter(signals CounterSignals) {
  <div id="container" data-signals={ templ.JSONString(signals) }>
    <button data-on:click={ datastar.PostSSE("/counter/increment/global") }>
      Increment Global
    </button>
    <div data-text="$global"></div>
  </div>
}
```

Handler returns:
```go
func (h *Handlers) CounterData(w http.ResponseWriter, r *http.Request) {
  store := pages.CounterSignals{
    Global: h.globalCounter.Load(),
    User:   userCount,
  }
  sse := datastar.NewSSE(w, r)
  sse.PatchElementTempl(pages.Counter(store))
}
```

#### Monitor Pattern (Real-time Updates)
```templ
<div id="container" data-init={ datastar.GetSSE("/monitor/events") }
     data-signals="{memTotal:'', memUsed:'', memUsedPercent:'', ...}">
  <p>Used: <span data-text="$memUsed"></span></p>
</div>
```

Handler streams updates:
```go
func (h *Handlers) MonitorEvents(w http.ResponseWriter, r *http.Request) {
  sse := datastar.NewSSE(w, r)
  for {
    select {
    case <-memT.C:
      memStats := pages.SystemMonitorSignals{ ... }
      sse.MarshalAndPatchSignals(memStats)
    }
  }
}
```

#### Web Components Integration
```templ
<sortable-example
  data-signals="{title: 'Item Info', items: [...]}"
  data-attr:title="$title"
  data-attr:value="$info"
  data-on:change="event.detail && console.log(...)"
/>
```

---

## 3. Backend Handler Communication

### 3.1 Request Flow Architecture

```
Browser                           Go Server
   |                                 |
   |---- GET /api/todos (SSE) ------>|
   |                                 |  Load TodoMVC from DB
   |                                 |
   |<--- PatchElements (HTML) -------|
   |                                 |  Poll state every 1s
   |<--- PatchSignals (JSON) --------|  Compare with last
   |<--- PatchSignals (JSON) --------|
   |                                 |
   |---- POST /api/todos/1/toggle ---->|
   |  (with signals in body)          |  Update TodoMVC in memory
   |                                 |  Save to DB
   |<--- PatchSignals (JSON) --------|  (no response body needed)
   |                                 |
```

### 3.2 Handler Patterns

#### Pattern 1: Page Initial Load
```go
func (h *Handlers) IndexPage(w http.ResponseWriter, r *http.Request) {
  if err := pages.IndexPage("QueryOps").Render(r.Context(), w); err != nil {
    http.Error(w, ..., http.StatusInternalServerError)
  }
}
```
Returns HTML with `<div id="todos-container" data-init={ datastar.GetSSE("/api/todos") }></div>`

#### Pattern 2: SSE Stream Initialization
```go
func (h *Handlers) TodosSSE(w http.ResponseWriter, r *http.Request) {
  sessionID, mvc, _ := h.todoService.GetSessionMVC(w, r)
  sse := datastar.NewSSE(w, r)
  
  // Send initial state
  if err := sse.PatchElementTempl(components.TodosMVCView(mvc)); err != nil {
    http.Error(w, ..., http.StatusInternalServerError)
  }
  
  // Stream updates
  ticker := time.NewTicker(time.Second)
  for {
    select {
    case <-ctx.Done(): return
    case <-ticker.C:
      current := h.todoService.GetMVCBySessionID(ctx, sessionID)
      if bytes.Equal(marshal(current), lastSent) { continue }
      sse.PatchElementTempl(components.TodosMVCView(current))
    }
  }
}
```

#### Pattern 3: State Mutation Action
```go
func (h *Handlers) ToggleTodo(w http.ResponseWriter, r *http.Request) {
  sessionID, mvc, _ := h.todoService.GetSessionMVC(w, r)
  idx := chi.URLParam(r, "idx")
  
  // Mutate in-memory state
  h.todoService.ToggleTodo(mvc, i)
  
  // Persist (no response sent - polling will pick it up)
  h.todoService.SaveMVC(r.Context(), sessionID, mvc)
}
```
- Handler doesn't send response
- Client-side indicator shows loading state
- SSE stream picks up the change via polling

#### Pattern 4: Signal-Based Mutations
```go
func (h *Handlers) IncrementGlobal(w http.ResponseWriter, r *http.Request) {
  h.globalCounter.Add(1)
  
  update := gabs.New()
  update.Set(h.globalCounter.Load(), "global")
  
  // Direct signal patch (no HTML)
  datastar.NewSSE(w, r).MarshalAndPatchSignals(update)
}
```

#### Pattern 5: Reading Signals from Client
```go
func (h *Handlers) SaveEdit(w http.ResponseWriter, r *http.Request) {
  type Store struct {
    Input string `json:"input"`
  }
  store := &Store{}
  
  // Read frontend signals from request
  if err := datastar.ReadSignals(r, store); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
  }
  
  // Use the data
  h.todoService.EditTodo(mvc, i, store.Input)
}
```

### 3.3 State Management on Backend

#### Per-Session State
```go
type TodoMVC struct {
  Todos      []*Todo      `json:"todos"`
  EditingIdx int          `json:"editingIdx"`
  Mode       TodoViewMode `json:"mode"`
}
```

#### Persistence Layer
```go
// TodoRepository - Data access layer
func (r *TodoRepository) SaveMVC(ctx context.Context, sessionID string, mvc *TodoMVC) error {
  data, _ := json.Marshal(mvc)
  r.pool.Exec(ctx, `
    INSERT INTO todos (session_id, state, created_at, updated_at)
    VALUES ($1, $2, NOW(), NOW())
    ON CONFLICT (session_id) DO UPDATE SET state = EXCLUDED.state
  `, sessionID, data)
}
```

#### Service Layer
```go
// TodoService - Business logic
type TodoService struct {
  repo  *TodoRepository
  store sessions.Store
}

func (s *TodoService) GetSessionMVC(w http.ResponseWriter, r *http.Request) (string, *TodoMVC, error) {
  sessionID := s.upsertSessionID(r, w)
  mvc := s.repo.GetMVC(ctx, sessionID)
  if mvc == nil {
    mvc = &TodoMVC{}
    s.resetMVC(mvc)
    s.saveMVC(ctx, sessionID, mvc)
  }
  return sessionID, mvc, nil
}
```

---

## 4. Design Patterns Evident

### 4.1 Primary Pattern: **Server-Driven UI (SDUI) with Reactive Data Binding**

This is NOT traditional MVC or MVVM. It's a **streaming HTML with reactive signals** pattern:

```
┌─────────────────────────────────────────────────────────┐
│                    Datastar Pattern                     │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  S = Signals (Reactive State) - Managed by Datastar    │
│  T = Templates (Templ) - Server-side rendering         │
│  E = Events (data-on:*) - Declarative handlers         │
│  A = Actions (@get, @post, etc.) - HTTP methods        │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 4.2 Supporting Patterns

#### 1. **Feature-Based Modular Architecture**
```
features/
├── index/           ← Todo app
│   ├── handlers.go
│   ├── routes.go
│   ├── components/   ← Templ components
│   ├── pages/        ← Templ pages
│   └── services/     ← Business logic
├── counter/         ← Counter app
├── monitor/         ← System monitoring
└── common/          ← Shared components
```

**Principles**:
- Each feature is self-contained
- Explicit route setup in `SetupRoutes()`
- Dependency injection via constructors
- Clean separation of concerns

#### 2. **MVC-Adjacent (Server-Side)**
The **backend** follows a loose MVC pattern:
- **Model**: `TodoMVC` struct (defines data shape)
- **View**: Templ components (`TodosMVCView()`, `Counter()`)
- **Controller**: Handlers (`IndexPage()`, `TodosSSE()`, `ToggleTodo()`)

However, **NOT traditional MVC** because:
- No view rendering based on HTTP response code
- Views are streamed via SSE, not returned in response
- Multiple views can be in a single SSE stream

#### 3. **Reactive MVVM (Client-Side)**
Datastar client is **MVVM-like** but simpler:
- **Model**: `data-signals` state
- **View**: DOM
- **ViewModel**: Implicit in Datastar's reactivity engine

Differences from MVVM:
- No explicit ViewModel class
- Signals are plain objects
- Binding is attribute-based, not property-based

#### 4. **Component-Based (Templ)**
Templates are composable components:
```go
@CounterButtons()
@CounterCounts()
@Counter(signals)
```
- Strongly-typed parameters
- Compiler-checked composition
- No runtime type errors
- Tree-structured component hierarchy

#### 4.5. **Service Layer Pattern**
Business logic separated from handlers:
```go
handlers := NewHandlers(todoService)
// Handler delegates to service
h.todoService.ToggleTodo(mvc, i)
h.todoService.SaveMVC(ctx, sessionID, mvc)
```

#### 4.6. **Repository Pattern**
Data access abstracted:
```go
repo := services.NewTodoRepository(pool)
todoService := services.NewTodoService(repo, store)
```

Allows testing without real DB.

#### 4.7. **Dependency Injection**
All dependencies passed via constructors:
```go
func NewHandlers(todoService *services.TodoService) *Handlers
func NewTodoService(repo *TodoRepository, store sessions.Store) *TodoService
func NewTodoRepository(pool *pgxpool.Pool) *TodoRepository
```

#### 4.8. **Session-Based Multi-User State**
```go
sessionStore := sessions.NewCookieStore([]byte(sessionSecret))
```
- Each user gets cookie with session ID
- State persisted per session in database
- Concurrent users have isolated state

---

## 5. State Management Across the Application

### 5.1 State Layers

#### Layer 1: Frontend Signals (Client Memory)
```javascript
// Datastar maintains reactive state
$global = 5        // Counter value
$user = 3          // Per-user counter
$input = "..."     // Form input
$fetching0 = false // Loading indicator
```
- Ephemeral (lost on page reload)
- Reactive (computed properties auto-update)
- Synced with backend via SSE patches

#### Layer 2: HTML Data Attributes (Initial State)
```html
<div data-signals="{global:5, user:3}">
  <!-- State seeded on page load -->
</div>
```
- Set once during server render
- Contains both initial data and UI metadata

#### Layer 3: Server Memory (Active Session)
```go
type TodoMVC struct {
  Todos      []*Todo
  EditingIdx int
  Mode       TodoViewMode
}
```
- In-memory during SSE stream
- Updated by handlers
- Compared with last-sent to detect changes

#### Layer 4: Database (Persistent)
```sql
CREATE TABLE todos (
  session_id TEXT PRIMARY KEY,
  state JSONB,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```
- Permanent storage per session
- Loaded on SSE initialization
- Updated on state mutations

### 5.2 Data Flow Lifecycle

#### Example: Toggle Todo

**1. User Clicks Checkbox**
```html
<input data-on:click={ datastar.PostSSE("/api/todos/0/toggle", 0) }/>
```
- Click event fires
- Datastar sends POST with signal context

**2. Request Includes Frontend State**
```json
{
  "input": "buy milk",
  "fetching0": false
}
```
- `datastar.ReadSignals(r, store)` extracts these

**3. Handler Mutates Backend State**
```go
h.todoService.ToggleTodo(mvc, 0)  // Toggle in-memory
h.todoService.SaveMVC(ctx, sessionID, mvc)  // Persist to DB
```

**4. No Direct Response Sent**
- Handler returns silently
- Client shows loading indicator (via `data-indicator`)

**5. SSE Stream Polls Every 1 Second**
```go
ticker := time.NewTicker(time.Second)
for range ticker.C {
  current := h.todoService.GetMVCBySessionID(ctx, sessionID)
  if bytes.Equal(marshal(current), lastSent) { continue }  // Skip if unchanged
  sse.PatchElementTempl(components.TodosMVCView(current))   // Send update
}
```

**6. Client Receives Patch**
```
event: datastar-patch-elements
data: <li id="todo0" class="line-through">...</li>
```
- Datastar morphs DOM (preserves focus, handlers, etc.)
- Update completes

### 5.3 State Synchronization Strategies

#### Strategy 1: Polling (Todo App)
```go
// Backend polls state every 1 second
ticker := time.NewTicker(time.Second)
for range ticker.C {
  // Compare with last-sent
  // Send only if changed
}
```
**Pros**: Simple, no message queue needed
**Cons**: Latency up to 1 second, wasted CPU if no changes

#### Strategy 2: Direct Signal Patch (Counter App)
```go
// Handler directly patches signals
update := gabs.New()
update.Set(h.globalCounter.Load(), "global")
sse.MarshalAndPatchSignals(update)
```
**Pros**: Immediate response, efficient
**Cons**: Doesn't sync state across clients (global counter only)

#### Strategy 3: Continuous Real-Time (Monitor App)
```go
// Ticker fires on interval
memT := time.NewTicker(time.Second)
cpuT := time.NewTicker(time.Second)

for {
  select {
  case <-memT.C:
    memStats := getSystemMemory()
    sse.MarshalAndPatchSignals(memStats)
  case <-cpuT.C:
    cpuStats := getSystemCPU()
    sse.MarshalAndPatchSignals(cpuStats)
  }
}
```
**Pros**: Real-time monitoring, clear update sources
**Cons**: Multiple tickers, complex logic

### 5.4 Multi-User Isolation

**Session Cookie**:
```go
sess, _ := s.store.Get(r, "connections")
id, ok := sess.Values["id"].(string)
if !ok {
  id = toolbelt.NextEncodedID()
  sess.Values["id"] = id
  sess.Save(r, w)
}
```

**Per-Session Database**:
```sql
SELECT state FROM todos WHERE session_id = $1
```

**Result**: Each browser/user gets isolated TodoMVC state
- User A's todos ≠ User B's todos
- No shared state between sessions
- Scalable to many concurrent users

### 5.5 Reactive Computed Properties (Client-Side)

Datastar supports computed properties:
```javascript
computed(() => $todos.filter(t => !t.completed).length)
```
But the app mostly uses direct signal binding:
```html
<div data-text="$global"></div>
```

---

## 6. Middleware & Routing Patterns

### 6.1 Chi Router Setup
```go
r := chi.NewMux()
r.Use(
  middleware.Logger,
  middleware.Recoverer,
)

// Feature routes
indexFeature.SetupRoutes(router, sessionStore, pool)
counterFeature.SetupRoutes(router, sessionStore)
```

**Key Points**:
- Middleware applied globally
- Features setup independently
- No request body parsing middleware (form data handled per-handler)

### 6.2 Feature Route Organization
```go
// features/index/routes.go
func SetupRoutes(router chi.Router, store sessions.Store, pool *pgxpool.Pool) error {
  repo := services.NewTodoRepository(pool)
  todoService := services.NewTodoService(repo, store)
  handlers := NewHandlers(todoService)

  router.Get("/", handlers.IndexPage)
  
  router.Route("/api", func(apiRouter chi.Router) {
    apiRouter.Route("/todos", func(todosRouter chi.Router) {
      todosRouter.Get("/", handlers.TodosSSE)
      todosRouter.Put("/reset", handlers.ResetTodos)
      todosRouter.Route("/{idx}", func(todoRouter chi.Router) {
        todoRouter.Post("/toggle", handlers.ToggleTodo)
        todoRouter.Delete("/", handlers.DeleteTodo)
      })
    })
  })
  return nil
}
```

**REST-like Design**:
- GET `/api/todos/` - SSE stream
- POST `/api/todos/{idx}/toggle` - Mutation
- DELETE `/api/todos/{idx}` - Deletion
- PUT `/api/todos/mode/{mode}` - Mode change

### 6.3 Hot Reload Support (Development)
```go
func setupReload(router chi.Router) {
  router.Get("/reload", func(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r)
    select {
    case <-reloadChan:
      sse.ExecuteScript("window.location.reload()")
    case <-r.Context().Done():
    }
  })

  router.Get("/hotreload", func(w http.ResponseWriter, r *http.Request) {
    select {
    case reloadChan <- struct{}{}:
    default:
    }
    w.WriteHeader(http.StatusOK)
  })
}
```

Triggered from base template:
```html
@if config.Global.Environment == config.Dev {
  <div data-init="@get('/reload', {retryMaxCount: 1000})"></div>
}
```

---

## 7. Data Attribute Reference

### Core Attributes

| Attribute | Purpose | Example |
|-----------|---------|---------|
| `data-signals` | Initialize reactive state | `data-signals="{count:0}"` |
| `data-bind:input` | Two-way binding | `<input data-bind:input/>` |
| `data-text` | Bind text content | `<span data-text="$count"></span>` |
| `data-class` | Reactive classes | `data-class="{'active':$isActive}"` |
| `data-on:event` | Event handler | `data-on:click="@post(...)"` |
| `data-attr:name` | Reactive attribute | `data-attr:title="$title"` |
| `data-show` | Conditional visibility | `data-show="$isVisible"` |
| `data-init` | Initialization effect | `data-init="@get('/data')"` |
| `data-indicator` | Loading state indicator | `data-indicator="fetching"` |
| `data-style` | Reactive styles | `data-style="{'color':$color}"` |
| `data-computed` | Computed properties | `data-computed:sum="$a + $b"` |

### Action Helpers

| Helper | Method | Example |
|--------|--------|---------|
| `GetSSE()` | GET | `datastar.GetSSE("/api/data")` |
| `PostSSE()` | POST | `datastar.PostSSE("/api/todos/toggle")` |
| `PutSSE()` | PUT | `datastar.PutSSE("/api/todos/mode/1")` |
| `PatchSSE()` | PATCH | `datastar.PatchSSE("/api/item/1")` |
| `DeleteSSE()` | DELETE | `datastar.DeleteSSE("/api/todos/0")` |

---

## 8. Architectural Strengths & Design Philosophy

### Strengths

1. **Progressive Enhancement**: Works without JavaScript (initial HTML render)
2. **Type Safety**: Go + Templ = compile-time checked components
3. **SEO-Friendly**: Server renders semantic HTML
4. **Minimal Client Code**: No JavaScript framework needed
5. **Real-Time Capable**: SSE allows continuous updates
6. **Session Isolation**: Built-in multi-user support
7. **Testability**: Service layer separated, dependency injection used
8. **Developer Experience**: Hot reload, clear request/response flow

### Philosophy

**"Hypermedia as the Engine of Application State"** (HATEOAS-adjacent)
- Server owns state and business logic
- Client is a dumb terminal (enhanced with reactivity)
- HTML is the transport mechanism
- Signals are the glue between HTML and business logic
- SSE is the update channel

This is closer to:
- **HTMX** (HTML-over-the-wire)
- **Hotwire/Turbo** (server-driven navigation)
- **Modern Server Components** (React Server Components)

But with **Datastar's reactive signals** as a unique addition.

---

## 9. Summary: Architectural Model

```
┌────────────────────────────────────────────────────────────┐
│                       Browser                             │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Datastar Reactive Engine                                │
│  ├── Signals ($global, $user, $todos, etc.)             │
│  ├── Watchers (data-on:*, data-bind:*, etc.)            │
│  ├── Morph Algorithm (DOM patching)                      │
│  └── SSE Listener (incoming patches)                     │
│                                                            │
│  HTML with Data Attributes                               │
│  ├── data-signals (state)                                │
│  ├── data-on:click/@post (actions)                       │
│  ├── data-bind:input (forms)                             │
│  └── data-text (content binding)                         │
│                                                            │
└────────────────────────────────────────────────────────────┘
                          ↕ SSE
┌────────────────────────────────────────────────────────────┐
│                   Go Web Server                            │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Feature Modules                                          │
│  ├── Handlers (HTTP request/response)                    │
│  │   ├── IndexPage() - Render initial HTML              │
│  │   ├── TodosSSE() - Stream updates                     │
│  │   └── ToggleTodo() - Mutate state                     │
│  │                                                        │
│  ├── Services (Business Logic)                           │
│  │   ├── TodoService.ToggleTodo()                       │
│  │   ├── TodoService.SaveMVC()                          │
│  │   └── TodoService.GetMVCBySessionID()               │
│  │                                                        │
│  ├── Repository (Data Access)                            │
│  │   └── TodoRepository.SaveMVC(sessionID, mvc)         │
│  │                                                        │
│  └── Components (Templ Templates)                        │
│      ├── TodosMVCView(mvc)                              │
│      ├── CounterButtons()                                │
│      └── Counter(signals)                                │
│                                                            │
│  Database (PostgreSQL)                                    │
│  └── todos(session_id, state)                            │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

---

## 10. Design Paradigm Classification

**Primary**: **Reactive Server-Driven UI (SDUI)**
**Secondary**: **Component-Based** (Templ)
**Tertiary**: **MVC** (backend only, loose interpretation)

**Not**: Traditional SPA, MVVM, Redux, or Client-Heavy Architecture

**Closest Analogues**:
1. Server Components (Next.js, Rails ViewComponent)
2. HTMX + Alpine.js (but integrated as one framework)
3. Hotwire Turbo + Stimulus (but simpler)
4. Phoenix LiveView (Elixir, similar pattern)

This is a **modern evolution of server-side rendering** that brings **reactive UI capabilities without building a client-side SPA**.

