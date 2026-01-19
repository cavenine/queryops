---
name: ui-components
description: Create reusable UI components using templui patterns, DaisyUI theming, and Tailwind CSS.
---

# UI Components Skill

Create reusable, type-safe UI components following the project's **templui** architecture and styled with **DaisyUI**.

## When to Use

- Creating new reusable UI elements (Cards, Buttons, Stats, Modals)
- Implementing design system changes
- Styling components with the application theme
- Extending the existing component library

## Architecture

The project combines three layers:
1.  **Structure**: [Templ](https://templ.guide/) (Type-safe HTML)
2.  **Pattern**: [TemplUI](https://templui.io/) (Props structs, Composition, TwMerge)
3.  **Styling**: [DaisyUI](https://daisyui.com/) (Semantic classes, Theming)

## Component Pattern

All components must follow the **TemplUI** pattern to ensure consistency and type safety.

### 1. File Structure
Create components in `features/common/components/<name>/<name>.templ`.

### 2. The Pattern
Every component should define a `Props` struct and use `utils.TwMerge` for class merging.

```go
package mycomponent

import "github.com/cavenine/queryops/utils"

// 1. Define Props struct
type Props struct {
	ID         string
	Class      string
	Attributes templ.Attributes
    // Add custom props here
    Variant    string 
}

// 2. Define Component
templ MyComponent(props ...Props) {
	// 3. Unpack Props (Standard Boilerplate)
	{{ var p Props }}
	if len(props) > 0 {
		{{ p = props[0] }}
	}

	// 4. Render with TwMerge and Props
	<div
		if p.ID != "" {
			id={ p.ID }
		}
		// Merge default classes with user-provided p.Class
		class={ utils.TwMerge(
            "bg-base-100 border border-base-200 rounded-lg p-4", // Defaults
            p.Class,                                             // Overrides
        ) }
		{ p.Attributes... }
	>
		{ children... }
	</div>
}
```

## DaisyUI Integration

Use **semantic classes** instead of hardcoded colors to respect the theme (e.g., Sunset/Synthwave).

### Color Variables
| Variable | Usage | Tailwind Class |
|----------|-------|----------------|
| `base-100` | Page/Card Background | `bg-base-100` |
| `base-200` | Secondary Background | `bg-base-200` |
| `base-content` | Main Text | `text-base-content` |
| `primary` | Main Action/Brand | `bg-primary`, `text-primary` |
| `secondary` | Highlights/Accents | `bg-secondary`, `text-secondary` |
| `accent` | Call to Action | `bg-accent` |

### Utility Classes
- **Loading**: Add `loading loading-spinner` to an element.
- **Buttons**: `btn btn-primary`, `btn btn-ghost`, `btn btn-sm`.
- **Inputs**: `input input-bordered`.
- **Layouts**: `drawer`, `modal`, `stats`.

**Example:**
```html
<!-- GOOD: Semantic, theme-aware -->
<button class="btn btn-primary">Save</button>

<!-- BAD: Hardcoded, breaks themes -->
<button class="bg-orange-500 text-white py-2 px-4 rounded">Save</button>
```

## Creating a Sub-Component (Composition)

Use sub-components to structure complex UI elements (like a Stats row).

```go
// Main container
templ Stats(props ...Props) {
    // ... boilerplate ...
    <div class={ utils.TwMerge("stats shadow", p.Class) }>
        { children... }
    </div>
}

// Child item
templ StatItem(props ...ItemProps) {
    // ... boilerplate ...
    <div class="stat">
        { children... }
    </div>
}
```

## Usage in Pages

Import the component and pass props.

```go
import "github.com/cavenine/queryops/features/common/components/mycomponent"

templ Page() {
    @mycomponent.MyComponent(mycomponent.Props{
        ID: "demo-id",
        Class: "shadow-xl", // Adds to default classes
    }) {
        <p>This is the content</p>
    }
}
```

## Checklist for New Components

- [ ] Created `features/common/components/<name>/<name>.templ`
- [ ] Defined `Props` struct with `ID`, `Class`, `Attributes`
- [ ] Used `utils.TwMerge` for class management
- [ ] Used DaisyUI semantic classes (`base-100`, `primary`)
- [ ] Ran `go tool templ generate`
- [ ] Ran `go tool gotailwind ...` (if new CSS classes were added)
