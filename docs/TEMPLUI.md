# templui Integration Guide

This document captures how QueryOps integrates [templui](https://templui.io/) so future sessions can quickly add and use ready-to-ship components.

## What We Integrated

- templui CLI installed via Go toolchain (`go get -tool`).
- templui components installed into the shared component area under `features/common/components/`.
- JS assets for JS-backed components placed under `web/resources/static/templui/` and served at `/static/templui/`.
- Tailwind token mapping added so templui’s shadcn-style classes (e.g. `bg-card`) work with our existing DaisyUI theme.
- A working demo `dialog` wired into the Index page.

## Quick Commands

- Check templui version: `go tool templui -v`
- List components: `go tool templui list`
- Add components: `go tool templui add button card dialog`

After adding/updating templ templates:
- Regenerate templ: `go tool templ generate` (or `go tool task build:templ`)
- Rebuild styles: `go tool task -f build:styles`

## Configuration (.templui.json)

We keep templui configured for this repo at `.templui.json:1`:

- `componentsDir`: `features/common/components`
- `utilsDir`: `utils`
- `moduleName`: `queryops`
- `jsDir`: `web/resources/static/templui`
- `jsPublicPath`: `/static/templui`

Rationale:
- `features/common/components/` is the existing “shared UI” home for cross-feature components.
- The server only serves embedded/static assets from `web/resources/static` (mounted at `/static/*`), so templui JS must land under that tree.
- We intentionally avoided `web/resources/static/libs/...` because that path is ignored by git (`.gitignore:17`).

## Where Components Land (Packages)

templui installs each component into its own Go package under `features/common/components/<name>/`.

Examples:
- `features/common/components/button` → import path `queryops/features/common/components/button`
- `features/common/components/card` → import path `queryops/features/common/components/card`
- `features/common/components/dialog` → import path `queryops/features/common/components/dialog`

This keeps component APIs isolated and avoids giant “everything in one package” files.

## Using Components in templ

Example usage patterns:

### Button

```templ
import (
  "queryops/features/common/components/button"
  "queryops/features/common/components/icon"
)

@button.Button(button.Props{Size: button.SizeSm}) {
  @icon.Plus(icon.Props{Class: "w-4 h-4"})
  New Task
}
```

### Card

```templ
import "queryops/features/common/components/card"

@card.Card(card.Props{Class: "shadow-sm"}) {
  @card.Content(card.ContentProps{Class: "p-0"}) {
    <!-- children -->
  }
}
```

## JS-backed Components (Dialog)

Some templui components ship JS and add a `Script()` helper. For example, `dialog` installs:

- `web/resources/static/templui/dialog.min.js`
- `features/common/components/dialog/dialog.templ:330` provides `templ Script()`

To enable it, we must include the script in the layout used by the page.

In this repo, most pages use `@layouts.Dashboard(...)`, so we wired it into:

- `features/common/layouts/dashboard.templ:3` (imports `dialog`)
- `features/common/layouts/dashboard.templ:35` (calls `@dialog.Script()`)

If you add other JS-backed components later, follow the same approach:
- Add the component import in the layout.
- Call its `@component.Script()` in `<head>`.

## Production Note: hashfs + Asset URLs

In production, `/static/*` is served by `hashfs` (see `web/resources/static_prod.go:19`).

- `resources.StaticPath("...")` returns a hashed filename in prod for aggressive caching.
- templui’s `Script()` emits a fixed `/static/...` URL (plus `?v=` cache-buster from `utils.ScriptVersion`).

Result:
- It works in dev and prod.
- It does not take advantage of hashfs long-cache semantics (because the filename is not hashed).

When we care about production caching for templui JS:
- Prefer changing templui component `Script()` implementations to use `resources.StaticPath(...)`.
- That likely means standardizing how templui components reference static assets in this codebase (a small wrapper helper can help).

## Styling: Making templui Classes Work With DaisyUI

templui components use shadcn-style Tailwind tokens such as:
- `bg-card`, `text-card-foreground`
- `text-muted-foreground`
- `bg-background`, `text-foreground`

Our app uses Tailwind + DaisyUI theme variables. To bridge the systems, we mapped templui tokens to DaisyUI variables in:

- `web/resources/styles/styles.css:1`

This ensures `build:styles` generates the needed Tailwind utilities (e.g. `bg-card`) without changing the DaisyUI theme.

## Demo: Index Page Dialog

A simple “New Task” demo dialog is wired into:

- `features/index/pages/index.templ:13`

In dev (`go tool task live`), clicking “New Task” should open a modal.

## Troubleshooting

- Dialog doesn’t open:
  - Confirm `/static/templui/dialog.min.js` loads (DevTools → Network).
  - Confirm you added `@dialog.Script()` to the layout your page uses.
- Styling looks wrong / classes missing:
  - Run `go tool task -f build:styles`.
  - Confirm the token mapping remains in `web/resources/styles/styles.css:1`.
- Components compile but templates not updating:
  - Run `go tool templ generate` (or `go tool task build:templ`).
