# Anti-bot Form Protection

This repo uses a lightweight, privacy-friendly anti-bot system for server-rendered forms.

It is designed to block common automated submissions without third-party CAPTCHA scripts.

## Layers

### 1) Honeypot

- Field name: `website`
- Must be left empty by humans.
- If non-empty, the submission is rejected.

### 2) JS challenge token (required)

- On GET render, the server issues a random per-render token and stores it in the session.
- The token is emitted into HTML as `data-antibot-token`.
- Client JS copies it into the posted field `js_token`.
- On POST, the server validates `js_token` exists and matches a session-stored token.
- Tokens are single-use (consumed on successful validation).

### 3) Timing check

- Each issued token stores its `renderedAt` timestamp.
- POST must arrive at least `MinDelay` after render (default: 2s).

## How to protect a new form

1) Ensure the route group is wrapped with `sessionManager.LoadAndSave`.
2) On the GET handler, issue a token and pass it to the template:

- Use `queryops/internal/antibot`.
- Call `Protector.Issue(ctx, "<formID>")`.

3) In the `.templ` form markup, add:

- `data-antibot-token={ antibotToken }` on the `<form>`
- Honeypot field:

```html
<div class="hp-field" aria-hidden="true">
  <label for="website">Website</label>
  <input id="website" name="website" type="text" tabindex="-1" autocomplete="off" />
</div>
```

- JS token field:

```html
<input name="js_token" type="text" class="hp-field" tabindex="-1" autocomplete="off" required />
```

4) On the POST handler, validate before any side effects:

- `Protector.Validate(r, "<formID>", r.FormValue("js_token"), r.FormValue("website"))`

If blocked:

- Return a hard failure (422/400) with a generic message.
- Log the reason and request metadata.

## Frontend integration

- `web/resources/static/antibot.js` populates `js_token` on `DOMContentLoaded`.
- It also re-applies after DataStar DOM patches by listening to the `datastar-fetch` event.

## Observability

- Blocked submissions should log `reason` (honeypot/token_missing/token_mismatch/too_fast).
- Do not log tokens.
