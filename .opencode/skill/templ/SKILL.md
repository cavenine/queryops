---
name: templ
description: Create Templ components and pages with Datastar integration for reactive server-driven UI
---

# Templ Components Skill

Create Templ templates with Datastar integration for reactive server-driven UI.

## When to Use

- Creating new page layouts
- Building reusable UI components
- Adding Datastar-powered reactive elements
- Implementing SSE-connected real-time views

## Project Context

- **Templating**: [templ](https://templ.guide/) - Type-safe Go HTML templates
- **Reactivity**: [Datastar](https://data-star.dev/) - Server-driven reactive UI
- **Styling**: Tailwind CSS via gotailwind
- **Components**: templui component library

## File Conventions

| Type | Location | Naming |
|------|----------|--------|
| Pages | `features/<feature>/pages/` | `<name>.templ` |
| Components | `features/<feature>/components/` | `<name>.templ` |
| Shared | `features/common/components/` | `<name>.templ` |
| Layouts | `features/common/layouts/` | `<name>.templ` |

## Basic Page Template

```templ
package pages

import (
	"github.com/cavenine/queryops/features/common/layouts"
)

templ IndexPage(title string) {
	@layouts.Base(title) {
		<div class="container mx-auto p-4">
			<h1 class="text-2xl font-bold">{ title }</h1>
			<p>Welcome to the page.</p>
		</div>
	}
}
```

## Datastar Integration Patterns

### 1. Reactive Signals (State)

Initialize reactive state with `data-signals`:

```templ
templ CounterPage() {
	@layouts.Base("Counter") {
		<div data-signals="{count: 0, name: ''}">
			<p>Count: <span data-text="$count"></span></p>
			<input data-bind:input placeholder="Your name"/>
			<p>Hello, <span data-text="$name"></span>!</p>
		</div>
	}
}
```

### 2. SSE Initialization

Load initial data via SSE:

```templ
package pages

import (
	"github.com/starfederation/datastar-go/datastar"
)

templ HostsPage() {
	@layouts.Base("Hosts") {
		<div id="hosts-container" data-init={ datastar.GetSSE("/api/hosts") }>
			<p>Loading...</p>
		</div>
	}
}
```

### 3. Button Actions

Trigger server actions with `data-on:click`:

```templ
import (
	"github.com/starfederation/datastar-go/datastar"
)

templ CounterButtons() {
	<div class="space-x-2">
		<button 
			class="btn btn-primary"
			data-on:click={ datastar.PostSSE("/counter/increment") }
		>
			Increment
		</button>
		<button 
			class="btn btn-secondary"
			data-on:click={ datastar.PostSSE("/counter/decrement") }
		>
			Decrement
		</button>
	</div>
}
```

### 4. Form Handling

Two-way binding with form elements:

```templ
templ CreateForm() {
	<form data-signals="{name: '', email: ''}">
		<div class="space-y-4">
			<div>
				<label for="name">Name</label>
				<input 
					id="name"
					data-bind:input 
					placeholder="Enter name"
					class="input"
				/>
			</div>
			<div>
				<label for="email">Email</label>
				<input 
					id="email"
					data-bind:input
					type="email"
					placeholder="Enter email"
					class="input"
				/>
			</div>
			<button 
				type="button"
				class="btn btn-primary"
				data-on:click={ datastar.PostSSE("/api/users/create") }
			>
				Create User
			</button>
		</div>
	</form>
}
```

### 5. Conditional Rendering

Show/hide elements based on signals:

```templ
templ ConditionalExample() {
	<div data-signals="{isLoggedIn: false, isLoading: true}">
		<div data-show="$isLoading">
			<p>Loading...</p>
		</div>
		<div data-show="!$isLoading && $isLoggedIn">
			<p>Welcome back!</p>
		</div>
		<div data-show="!$isLoading && !$isLoggedIn">
			<p>Please log in.</p>
		</div>
	</div>
}
```

### 6. Dynamic Classes

Apply classes conditionally:

```templ
templ TodoItem(completed bool) {
	<div 
		data-signals={ templ.JSONString(map[string]bool{"completed": completed}) }
		data-class="{'line-through text-gray-400': $completed}"
	>
		<input type="checkbox" data-bind:checked/>
		<span>Todo item text</span>
	</div>
}
```

### 7. Loading Indicators

Show loading state during async operations:

```templ
templ SubmitButton() {
	<button 
		class="btn btn-primary"
		data-on:click={ datastar.PostSSE("/api/submit") }
		data-indicator="submitting"
	>
		<span data-show="!$submitting">Submit</span>
		<span data-show="$submitting">Loading...</span>
	</button>
}
```

### 8. Real-Time Updates (SSE Pattern)

For components that receive live updates:

```templ
package pages

import (
	"github.com/starfederation/datastar-go/datastar"
	"github.com/cavenine/queryops/features/<feature>/services"
)

type MonitorSignals struct {
	CPUUsage    string `json:"cpuUsage"`
	MemoryUsed  string `json:"memoryUsed"`
	MemoryTotal string `json:"memoryTotal"`
}

templ MonitorPage() {
	@layouts.Base("System Monitor") {
		<div 
			id="monitor-container"
			data-init={ datastar.GetSSE("/monitor/events") }
			data-signals={ templ.JSONString(MonitorSignals{}) }
		>
			<div class="grid grid-cols-2 gap-4">
				<div class="card p-4">
					<h3>CPU</h3>
					<p data-text="$cpuUsage"></p>
				</div>
				<div class="card p-4">
					<h3>Memory</h3>
					<p>
						<span data-text="$memoryUsed"></span> / 
						<span data-text="$memoryTotal"></span>
					</p>
				</div>
			</div>
		</div>
	}
}
```

## Component with Props

```templ
package components

import (
	"github.com/cavenine/queryops/features/<feature>/services"
)

templ HostCard(host *services.Host) {
	<div class="card p-4 hover:shadow-lg transition-shadow">
		<div class="flex justify-between items-center">
			<div>
				<h3 class="font-semibold">{ host.Hostname }</h3>
				<p class="text-sm text-gray-500">{ host.Platform }</p>
			</div>
			<a 
				href={ templ.SafeURL("/hosts/" + host.ID.String()) }
				class="btn btn-sm"
			>
				View Details
			</a>
		</div>
	</div>
}

templ HostList(hosts []*services.Host) {
	<div class="space-y-2">
		for _, host := range hosts {
			@HostCard(host)
		}
		if len(hosts) == 0 {
			<p class="text-gray-500">No hosts found.</p>
		}
	</div>
}
```

## Layout Template

```templ
package layouts

import (
	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/web/resources"
)

templ Base(title string) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="UTF-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
			<title>{ title } | QueryOps</title>
			<link rel="stylesheet" href={ resources.StaticPath("index.css") }/>
			<script defer type="module" src={ resources.StaticPath("datastar/datastar.js") }></script>
		</head>
		<body class="min-h-screen bg-background">
			@if config.Global.Environment == config.Dev {
				<div data-init="@get('/reload', {retryMaxCount: 1000, retryInterval:20, retryMaxWaitMs:200})"></div>
			}
			{ children... }
		</body>
	</html>
}
```

## Datastar Attribute Reference

| Attribute | Purpose | Example |
|-----------|---------|---------|
| `data-signals` | Initialize reactive state | `data-signals="{count:0}"` |
| `data-bind:input` | Two-way text binding | `<input data-bind:input/>` |
| `data-bind:checked` | Checkbox binding | `<input type="checkbox" data-bind:checked/>` |
| `data-text` | Bind text content | `data-text="$count"` |
| `data-class` | Conditional classes | `data-class="{'active':$isActive}"` |
| `data-show` | Conditional visibility | `data-show="$isVisible"` |
| `data-on:click` | Click handler | `data-on:click="@post('/api')"` |
| `data-on:submit` | Form submit | `data-on:submit__prevent="@post('/api')"` |
| `data-init` | Initialization action | `data-init="@get('/api/data')"` |
| `data-indicator` | Loading state signal | `data-indicator="loading"` |
| `data-attr:<name>` | Reactive attribute | `data-attr:disabled="$isDisabled"` |

## Event Modifiers

```templ
// Prevent default
data-on:click__prevent="@post('/api')"

// Stop propagation
data-on:click__stop="@post('/api')"

// Debounce (500ms)
data-on:input__debounce.500ms="@post('/api/search')"

// Throttle (1s)
data-on:scroll__throttle.1s="@get('/api/more')"

// Outside click
data-on:click__outside="$menuOpen = false"
```

## Generation Commands

After creating or modifying `.templ` files:

```bash
# Generate Go code from templ files
go tool templ generate

# Or use live reload during development
go tool task live
```

## Checklist

- [ ] Created `.templ` file in correct location
- [ ] Added proper package declaration
- [ ] Imported required packages (layouts, datastar, services)
- [ ] Used `data-signals` for reactive state
- [ ] Used `datastar.GetSSE()`/`PostSSE()` for server actions
- [ ] Generated Go code (`go tool templ generate`)
- [ ] Created corresponding handler in `handlers.go`
- [ ] Tested with live reload (`go tool task live`)
