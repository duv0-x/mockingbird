# mockingbird — Design system

Version 1 · 2026-10-15 · Target: server-rendered UI with **templ + htmx** (no SPA).

- Tokens: [`tokens.css`](tokens.css) (light + dark, `prefers-color-scheme` + `[data-theme]`)
- Logo: [`logo/`](logo/) · Screens: [`screens/`](screens/) · Rationale: [`README.md`](README.md)
- Stitch: see [§13 Stitch references](#13-stitch-references)

---

## 1. Principles

1. **Calm by default, loud only when it matters.** Graphite neutrals plus a single teal accent. Colour is reserved for meaning: HTTP method, status code, mock mode, errors.
2. **Dense but readable.** 13px UI text, 36px table rows, 32px controls. Every list is a table you can scan, sort and bulk-select.
3. **Monospace for anything a machine reads.** URIs, IDs, headers, bodies, status codes, XPath/JSONPath, durations and timestamps use `--font-mono`.
4. **The hierarchy is always visible.** Breadcrumbs plus a sidebar tree, so you never get lost five levels deep (Project → Application → Resource → Method → Response).
5. **Server-first interactions.** Every interaction is a full page or an htmx partial swap. No client state that the server can't re-render.
6. **Accessible.** WCAG 2.2 AA contrast in both themes (checked, see §2.4), visible focus rings, full keyboard operation, `prefers-reduced-motion` respected.

## 2. Colour

All colours are CSS custom properties in `tokens.css`. **Never hard-code hex values in templates.**

### 2.1 Neutrals

| Token | Light | Dark | Use |
|---|---|---|---|
| `--bg` | `#F7F8FA` | `#0F1115` | App background, sidebar |
| `--surface` | `#FFFFFF` | `#161920` | Cards, tables, inputs, modals |
| `--surface-2` | `#F1F3F6` | `#1C2029` | Row hover, table header, code gutter |
| `--surface-3` | `#E9ECF1` | `#232835` | Pressed / selected rows |
| `--border` | `#E3E6EB` | `#2A2F3A` | All 1px dividers |
| `--border-strong` | `#CDD2DA` | `#3A4150` | Input borders, hover borders |
| `--text` | `#14171C` | `#E6E8EC` | Primary text |
| `--text-muted` | `#5B6472` | `#9BA3B0` | Secondary text, table headers |
| `--text-faint` | `#6E7785` | `#7D8594` | Placeholders, disabled, meta (never essential copy) |

### 2.2 Accent (teal)

| Token | Light | Dark |
|---|---|---|
| `--accent` | `#0F766E` | `#2DD4BF` |
| `--accent-hover` | `#115E59` | `#5EEAD4` |
| `--accent-fg` (text on accent) | `#FFFFFF` | `#04201C` |
| `--accent-text` (links, active nav) | `#0F766E` | `#5EEAD4` |
| `--accent-subtle` | teal 10% | teal 12% |
| `--focus-ring` | `#0F766E` | `#5EEAD4` |

The accent appears on: primary buttons, active nav item, active tab underline, focus rings, links, toggles in the ON state, the MOCKED mode. Nothing else.

### 2.3 Semantic hues

Each hue `--{green,blue,amber,violet,red,orange,teal,gray}` has a `-subtle` tint used as badge background. On top of them, the semantic mappings:

| Meaning | Token | Hue |
|---|---|---|
| `GET` | `--method-get` | green |
| `POST` | `--method-post` | blue |
| `PUT` | `--method-put` | amber |
| `PATCH` | `--method-patch` | violet |
| `DELETE` | `--method-delete` | red |
| `HEAD` / `OPTIONS` / `TRACE` | `--method-other` | gray |
| 1xx / 2xx / 3xx / 4xx / 5xx | `--status-{1..5}xx` | gray / green / blue / amber / red |
| `MOCKED` | `--mode-mocked` | teal |
| `FORWARDED` | `--mode-forwarded` | blue |
| `RECORDING` | `--mode-recording` | red |
| `RECORD_ONCE` | `--mode-record-once` | orange |
| `ECHO` | `--mode-echo` | violet |
| `DISABLED` | `--mode-disabled` | gray |
| success / info / warning / error | `--success` … `--error` | green / blue / amber / red |

Colour is never the only signal: method badges carry the verb, mode badges carry an icon and a label, status codes show the number.

### 2.4 Contrast (verified)

| Pair | Light | Dark |
|---|---|---|
| text / surface | 17.96 | 14.34 |
| text-muted / surface | 5.98 | 6.92 |
| text-faint / surface | 4.53 | 4.74 |
| accent-fg / accent (primary button) | 5.47 | 9.18 |
| accent-text / surface | 5.47 | 9.45 |
| semantic fg / own subtle tint on surface or surface-2 (worst case) | ≥ 5.03 | ≥ 5.21 |

## 3. Typography

Fonts: **Inter** (UI) and **JetBrains Mono** (code/data). Self-host both as WOFF2 inside the Go binary (`go:embed`); do not depend on Google Fonts at runtime (air-gapped clusters). Use `font-feature-settings: "cv11", "ss01"` for Inter (straight-sided digits) and `font-variant-numeric: tabular-nums` in tables.

| Token | Size / line-height | Weight | Use |
|---|---|---|---|
| `--text-2xl` | 32 / 40 | 600 | Login title only |
| `--text-xl` | 24 / 32 | 600, `-0.01em` | Page title (h1) |
| `--text-lg` | 18 / 26 | 600 | Modal / card title (h2) |
| `--text-md` | 14 / 20 | 600 | Section heading (h3), form inputs |
| `--text-sm` | 13 / 20 | 400 | **Default UI text**, tables |
| `--text-xs` | 12 / 16 | 400 | Help text, meta, breadcrumbs |
| `--text-2xs` | 11 / 16 | 600, uppercase, `0.04em` | Table headers, badges, overlines |
| `--text-code` | 12.5 / 20 (1.6) | 400 | Code blocks, editors |

## 4. Spacing, radius, elevation

- **Spacing** is a 4px scale: `--space-1` (4) … `--space-16` (64). Default gaps: 8 inside controls, 12 between form fields, 16 between card sections, 24 page gutter, 32 between page sections.
- **Radius:** `--radius-sm` 3px (badges, kbd), `--radius-md` 4px (buttons, inputs, tables), `--radius-lg` 6px (cards, modals, menus, toasts), `--radius-full` (dots, avatars, switches).
- **Elevation:** borders do the work. Shadows only on floating layers: `--shadow-md` for menus/popovers, `--shadow-lg` for modals/toasts. In dark mode, `--shadow-sm` is `none`.

## 5. Layout

```
┌───────────────────────────────────────────────────────────────────────┐
│ TOP BAR 48px: [mark] mockingbird   [ Search…  ⌘K ]      ◐  ?  (avatar) │
├──────────────┬────────────────────────────────────────────────────────┤
│ SIDEBAR 240  │ Breadcrumb  Projects / payments-gateway / v1 / /users  │
│ ▣ Projects   │ ┌ Page header ─────────────────────────────────────┐   │
│ ≋ Events     │ │ h1 Title  [badges]                  [sec] [primary] │   │
│ ☺ Users      │ │ meta line (mono): endpoint · id · updated         │   │
│ ⚙ System     │ └────────────────────────────────────────────────────┘ │
│ ──────────── │ Tabs: Overview | Responses | Events | Settings         │
│ PROJECT TREE │ ┌ Content (cards / tables / editors) ───────────────┐ │
│ ▾ payments   │ │                                                    │ │
│   ▾ v1       │ │                                                    │ │
│     ▸ /users │ └────────────────────────────────────────────────────┘ │
│ v0.1.0       │                                                        │
└──────────────┴────────────────────────────────────────────────────────┘
```

- **Shell grid:** `grid-template-columns: var(--sidebar-w) 1fr; grid-template-rows: var(--topbar-h) 1fr;`. Top bar and sidebar are fixed; only `<main>` scrolls.
- **Main:** `padding: var(--space-6)`; content `max-width: var(--content-max)`. Forms and editors use a 12-column grid with a 16px gap. Typical split for detail pages: 8 columns content + 4 columns side panel (summary, metadata).
- **Sidebar:** primary nav on top (4 items), then the **context tree** of the current project: REST shows Applications → Resources → Methods (with method badges); SOAP shows Ports → Operations. Tree rows are 28px with a 12px indent per level, a chevron to expand, and a mode dot on the right. The tree is an htmx partial (`hx-get="/ui/projects/{id}/tree"`); the current node is highlighted with `--accent-subtle` and `aria-current="page"`.
- **Breakpoints:** ≥ 1280 full layout · 1024–1279 sidebar 208px, side panels fold under content · < 1024 sidebar collapses to a 56px icon rail (tree in a drawer) · < 768 (secondary) single column, top bar search becomes an icon.

## 6. Navigation model

| URL | Screen |
|---|---|
| `/ui/login` | Login |
| `/ui/projects` | Projects dashboard |
| `/ui/projects/new`, `/ui/projects/import` | Create / import (modal over the dashboard, or full page when deep-linked) |
| `/ui/rest/projects/{p}` | REST project: applications table + tree |
| `/ui/rest/projects/{p}/applications/{a}` | Resources table |
| `/ui/rest/projects/{p}/applications/{a}/resources/{r}` | Methods table |
| `/ui/rest/…/methods/{m}` | Method detail: mode, strategy, responses |
| `/ui/rest/…/methods/{m}/responses/{id}` | Mock response editor |
| `/ui/soap/projects/{p}` · `/ports/{port}` · `/operations/{o}` · `/responses/{id}` | SOAP equivalents |
| `/ui/events` · `/ui/events/{type}/{id}` | Events log + detail (drawer or page) |
| `/ui/users` | Users & roles (ADMIN only) |
| `/ui/system` | System info + settings (ADMIN only) |

- Breadcrumbs mirror the URL. Each segment is a link; the last one is plain text with `aria-current="page"`. Long names truncate with an ellipsis and keep the full name in `title`.
- **⌘K / Ctrl+K** opens the command palette (`hx-get="/ui/search?q="` with `hx-trigger="input changed delay:150ms"`), which searches projects, methods, operations and responses.
- Keyboard: `g p` projects, `g e` events, `/` focus filter, `n` new item on list pages, `e` edit on detail pages, `Esc` closes modal/drawer.
- Roles: READER sees everything read-only (action buttons are hidden, not disabled). MODIFIER can edit projects. ADMIN also gets Users and System.

## 7. Components

Class names below are suggestions (BEM-ish, no framework). Everything maps to tokens.

### 7.1 Buttons — `.btn`

| Variant | Background | Text | Border | Use |
|---|---|---|---|---|
| `.btn--primary` | `--accent` | `--accent-fg` | none | One per view: Create, Save |
| `.btn--secondary` | `--surface` | `--text` | `--border-strong` | Import, Duplicate, Cancel |
| `.btn--ghost` | transparent | `--text-muted` | none | Toolbar / row actions |
| `.btn--danger` | `--danger` | `--danger-fg` | none | Confirm delete (inside confirm dialogs only) |
| `.btn--danger-ghost` | transparent | `--red` | none | "Delete" entry points |

Height `--control-h` (32px), `--control-h-sm` (28px) in tables. Padding `0 12px`, gap 6px, icon 16px, `--text-sm` 500. Hover: primary → `--accent-hover`; others → `--surface-2`. Focus: `outline: var(--focus-ring-width) solid var(--focus-ring); outline-offset: var(--focus-ring-offset)`. Busy state (`.htmx-request`): label stays, spinner replaces the icon, `aria-busy="true"`, `pointer-events:none`. Icon-only buttons are 32×32 and always have `aria-label`.

### 7.2 Inputs — `.field`, `.input`, `.select`, `.textarea`, `.switch`

- Label above (`--text-xs` 500 `--text-muted`), control, then help text (`--text-xs` `--text-faint`) or an error (`--text-xs` `--error`, with an icon). Error state: `border-color: var(--error)`, `aria-invalid="true"`, `aria-describedby` pointing to the message.
- Control: height 32px, `--surface` bg, 1px `--border-strong`, `--radius-md`, padding `0 10px`. Hover: `--text-faint` border. Focus: border `--accent` + 3px ring `--accent-subtle`.
- Mono inputs (`.input--mono`) for URIs, endpoints, header names, XPath/JSONPath and status codes.
- **Input with prefix**, e.g. forwarded endpoint `https://` or `/castlemock/mock/rest/project/…`: the prefix is in `--surface-2` with mono `--text-muted`.
- **Switch:** 28×16 track, `--radius-full`; on → `--accent`; label to the right. Use it for "Simulate network delay", "Use expressions", "Automatic forward".
- **Segmented control** (`.segmented`): radio group styled as joined buttons. Used for All/REST/SOAP filters and Light/Dark/System.

### 7.3 Tables — `.table`

- Container: `--surface`, 1px `--border`, `--radius-md`, `overflow:auto`.
- Header row: `--surface-2`, sticky (`position:sticky; top:0; z-index: var(--z-sticky)`), `--text-2xs` uppercase `--text-muted`, height 32px. Sortable headers are `<button>`s with an `aria-sort` state on the `<th>`.
- Rows: `--row-h` (36px), 1px bottom `--border`, `--text-sm`. Hover `--surface-2`. Selected (checkbox) `--accent-subtle`. The whole row is clickable via an `<a>` in the name cell stretched with `::after`; the checkbox and kebab menu sit above it (`position:relative; z-index:1`).
- First column: 36px checkbox column for bulk actions. When ≥ 1 row is selected, a **bulk bar** replaces the filter bar: "3 selected · Set mode ▾ · Duplicate · Delete" (`hx-include="closest form"`).
- Numbers and times are right-aligned and use `tabular-nums`.
- Footer: "Showing 1–50 of 214" + pager (`hx-get` with `hx-push-url="true"`).

### 7.4 Tabs — `.tabs`

40px tall bar with a 1px bottom `--border`. Tab: `--text-sm` 500 `--text-muted`, padding `0 12px`. Active tab: `--text` with a 2px `--accent` underline. Counts as small neutral pills ("Responses 4"). Implemented as links (`hx-get`, `hx-target="#tab-panel"`, `hx-push-url="true"`) with `role="tablist"`/`role="tab"`/`aria-selected`.

### 7.5 Badges

All badges are 20px tall, `--radius-sm`, padding `0 6px`, `--font-mono` `--text-2xs` 600 uppercase, `background: var(--X-bg); color: var(--X)`.

| Badge | Example | Notes |
|---|---|---|
| `.badge-method--{get,post,put,patch,delete,other}` | `GET` | Fixed 52px min-width in tables so paths align |
| `.badge-status--{1..5}xx` | `200` `404` `503` | Number only; the reason phrase goes in a tooltip |
| `.badge-mode--{mocked,forwarded,recording,record-once,echo,disabled}` | `● MOCKED` | Leading 6px dot (teal), `→` for FORWARDED, a pulsing red dot for RECORDING (animation off under reduced motion), `↻` for ECHO |
| `.badge-strategy` | `SEQUENCE` | Neutral: transparent bg, 1px `--border-strong`, `--text-muted` |
| `.badge-type` | `REST` / `SOAP` | Neutral filled `--surface-3`, `--text` |
| `.badge-role` | `ADMIN` `MODIFIER` `READER` | violet / blue / gray |
| `.badge-user-status` | `ACTIVE` `INACTIVE` `LOCKED` | green / gray / red |
| `.badge-response` | `ENABLED` / `DISABLED` | Use a switch in editable contexts, a badge in read-only contexts |
| `.badge--default` | `DEFAULT` | Teal outline; marks the method's default mock response |

**Mode summary bar** (dashboard, project and application rows): 6px-tall stacked bar, `--radius-full`, one segment per mode in proportion, with a legend tooltip ("32 mocked · 4 forwarded · 2 recording · 10 disabled").

### 7.6 Code blocks and editors — `.code`

- Read-only blocks: `--code-bg`, 1px `--border`, `--radius-md`, `--font-mono` `--text-code`, line-height 1.6, 12px padding, a 40px line-number gutter in `--code-gutter`. Header strip (28px): language label (JSON/XML/TEXT), byte size, buttons **Copy**, **Format**, **Wrap**.
- Syntax colours: `--code-key`, `--code-string`, `--code-number`, `--code-literal`, `--code-attr`, `--code-punct`. Highlighting is done **server-side** (e.g. `chroma` in Go, emitting classes mapped to these tokens), so no JS highlighter is needed.
- Editable bodies: a plain `<textarea class="code code--edit">` with the same visual style, `spellcheck="false"`, `wrap="off"`, a minimum of 16 rows, resizable vertically. Optional progressive enhancement: CodeMirror 6 loaded only on the response editor. **Format** is a server endpoint (`hx-post="/ui/format?lang=json"` swapping the textarea).
- Expressions such as `${RANDOM_UUID}` and `${PATH_PARAMETER(key="id")}` are highlighted in `--code-expr` with an `--accent-subtle` background.
- **Diff view** (`.diff`): two columns, Expected/Mock vs Actual, or Request vs Forwarded response. Changed lines use `--diff-add-bg`/`--diff-del-bg` with `+`/`−` gutter markers (not colour alone). Toggle between Split and Unified views.

### 7.7 Key–value editor (headers, query/parameter matches)

A table with mono inputs: Name | Value | (match type for queries: `Equals`/`Regex`) | remove icon. "+ Add header" is a ghost button that `hx-get`s a new empty row and swaps it in with `hx-swap="beforeend"` on the `<tbody>`. Rows are removed client-side with `hx-on:click="this.closest('tr').remove()"` — the form is posted as a whole on Save.

### 7.8 Cards and stat tiles

- `.card`: `--surface`, 1px `--border`, `--radius-lg`, header 44px (title `--text-md` 600 + actions), body padding 16px.
- `.stat`: label (`--text-2xs` uppercase muted), value (`--text-xl` 600 tabular-nums), optional delta/meta (`--text-xs`). Error stats use `--error` for the value only.
- `.kv-list` (definition list for metadata): 2 columns, label `--text-muted` 120px, value mono when technical.

### 7.9 Modals — `<dialog class="modal">`

- Native `<dialog>` opened via htmx: the server returns the dialog markup into `#modal-root` (`hx-target="#modal-root"`), and a small helper calls `showModal()` on `htmx:afterSwap`. `Esc` and the ✕ close it; focus returns to the trigger.
- Widths: `--modal-w-sm` 480 (confirmations, simple forms), `--modal-w-md` 640 (create/import), `--modal-w-lg` 896 (diffs).
- Structure: header (title `--text-lg`, ✕ icon button), body (16–24px padding, scrollable), footer right-aligned (Cancel secondary + primary/danger). `::backdrop { background: var(--overlay) }`.
- On a successful submit, the server answers `HX-Trigger: {"closeModal":true,"toast":{…}}` plus an out-of-band swap (`hx-swap-oob`) of the affected table.

### 7.10 Confirm dialogs

- **Low risk** (disable a response): `hx-confirm` is not used; prefer an **undo toast**.
- **Destructive** (delete method/project/user): a modal stating exactly what is deleted ("Delete method **GET /users/{id}** and its 4 mock responses?"). Danger button labelled with the verb ("Delete method"), never "OK".
- **Very destructive** (delete a project, bulk delete ≥ 10 items): additionally require typing the name (`pattern` attribute on the input + the button `disabled` until it matches; server re-validates).

### 7.11 Toasts — `#toasts`

- Bottom-right stack, 360px wide, `--surface` + `--shadow-lg` + `--radius-lg`, a 3px left border coloured by type (success/info/warning/error), icon, one-line message, optional action ("Undo", "View event").
- Delivered by the server through the `HX-Trigger` header (`{"toast":{"type":"success","msg":"Mock response saved"}}`) and rendered by a tiny listener; or via an out-of-band swap of `<div id="toasts" hx-swap-oob="beforeend">`.
- Auto-dismiss after 5s (success/info); errors stay until closed. Container is `role="status" aria-live="polite"` (errors use `role="alert"`).

### 7.12 Empty states

Centered inside the card/table area, max 360px: a 32px line icon in `--text-faint`, a title (`--text-md` 600), one sentence (`--text-sm` muted), one primary action, plus an optional secondary link to the docs. Examples:
- No projects → "Create your first mock project" · [New project] · [Import OpenAPI / WSDL]
- No mock responses → "This method has no responses yet. Requests will return the default body." · [Add response]
- No events → "No requests captured yet. Point your client at `<endpoint>`" (endpoint copyable)

### 7.13 Other primitives

- **Breadcrumb:** `--text-xs`, separator `/` in `--text-faint`, 32px row above the page header.
- **Kebab menu / dropdown:** `<details class="menu">` + `<summary>`, panel with `--shadow-md`, items 32px, the destructive item last in `--red`.
- **Tooltip:** native `title` for truncation; `.tip` (CSS `:hover`/`:focus-visible`) for icon help.
- **Copy button:** inline next to endpoints and IDs; after a click it swaps its icon to a check for 1.5s (`navigator.clipboard`, ~10 lines of JS).
- **Kbd:** `<kbd>` with `--surface-2`, 1px `--border-strong`, `--radius-sm`, mono 11px.
- **Avatar:** 24px circle, initials, `--surface-3`.
- **Spinner:** 14px ring, `currentColor`, used inside buttons and `.htmx-indicator`.
- **Skeleton:** `--surface-2` blocks with a subtle shimmer (off under reduced motion), only for slow partials (> 300ms).

## 8. htmx interaction patterns

| Pattern | Markup | Notes |
|---|---|---|
| **Page navigation** | `hx-boost="true"` on `<body>` | Full pages server-rendered; boost keeps it SPA-fast. Breadcrumb, tree and `<title>` update via the full swap. |
| **Loading indicator** | `hx-indicator="#top-progress"` | A 2px `--accent` progress bar at the top of `<main>`; buttons also get `.htmx-request` (spinner). Show only after 150ms: `.htmx-indicator { transition-delay: 150ms }`. |
| **Filter / search** | `<input hx-get="?" hx-trigger="input changed delay:250ms, search" hx-target="#rows" hx-push-url="true">` | Replaces only `<tbody id="rows">`. |
| **Inline edit** (names, URIs, status code, delay) | Read view `<span hx-get=".../edit" hx-trigger="click, keyup[key=='Enter']" hx-swap="outerHTML" tabindex="0">` → edit form `hx-put`, `Esc` cancels with `hx-get` of the read view | Pencil icon on hover/focus. Saving shows a small check for 1s. |
| **Mode / strategy change** | `<select hx-put=".../mode" hx-trigger="change" hx-target="closest .mode-cell">` | Instant; toast with **Undo**. |
| **Toggle response enabled** | Switch `hx-patch` with `hx-swap="outerHTML"` on the row | Optimistic: the row re-renders from the server's answer. |
| **Bulk actions** | Checkboxes inside `<form id="bulk">`; actions `hx-post` with `hx-include="#bulk"` | Bulk bar appears via `hx-swap-oob` when the selection changes (or with a tiny bit of CSS `:has(:checked)`). |
| **Modals** | `hx-get=".../new" hx-target="#modal-root"` | See §7.9. Validation errors re-render the dialog body (`422` + `hx-target-error`/`HX-Retarget`). |
| **Confirm destructive** | `hx-get=".../delete-confirm"` → dialog → `hx-delete` | See §7.10. After delete: `HX-Location` back to the parent. |
| **Out-of-band updates** | `hx-swap-oob="true"` on `#tree`, `#counts`, `#toasts` | E.g. creating a method also refreshes the sidebar tree and the counts. |
| **Live events** | `<tbody hx-get="/ui/events/rows?after={lastId}" hx-trigger="every 5s [!document.hidden]" hx-swap="afterbegin">` (or SSE ext.) | A "Live" toggle in the header pauses polling. New rows flash with `--accent-subtle` for 1s. |
| **Lazy panels** | `hx-get hx-trigger="revealed"` | Event bodies, large diffs, the system info card. |
| **Errors** | Global `htmx:responseError` → error toast with status + "Retry" | 401 → `HX-Redirect: /ui/login?next=…`. |

## 9. Screen specs (summary)

1. **Login** — centered 400px card on `--bg`, logo (mark + wordmark), username, password, "Sign in" (40px primary, full width), error banner above the fields. Footer: version + "Open source · Apache-2.0".
2. **Projects dashboard** — 4 stat tiles (projects, mocked endpoints, requests 24h, errors 24h); filter + REST/SOAP segmented control; table: name + description, type badge, endpoints, mode summary bar, last request, kebab. Primary action "New project", secondary "Import".
3. **Create / import project** — modal 640px with two tabs: *Blank* (name, type REST/SOAP as two selectable cards, description) and *Import* (format select: OpenAPI 3 / Swagger 2 / RAML / WADL / WSDL / CastleMock export; source: drop zone for file upload **or** URL; options: "Generate mock responses", "Create in existing project"). Import preview lists what will be created before confirming.
4. **REST project** — header with the base endpoint (mono + copy); tabs Applications | Events | Settings; table of applications with resource count + mode bar; sidebar tree expanded.
5. **Method detail** — header: method badge + name + URI (mono, path params highlighted) + mode badge; control strip card: Mode select, Response strategy select, Forwarded endpoint (if FORWARDED/RECORDING/RECORD_ONCE), Network delay (switch + ms), Default response; mock responses table (name, status badge, content-type, matchers summary, enabled switch, DEFAULT badge, sequence position when SEQUENCE); right panel "Mock endpoint" (full URL + copy + curl snippet) and "Recent events".
6. **Mock response editor** — two-column: left (8 cols) Body editor with JSON/XML formatting + tabs Headers | Matching (parameter queries / header queries / JSONPath / XPath depending on strategy) | Content encoding; right (4 cols) Settings: name, HTTP status (input + badge), enabled switch, use expressions switch, delay; **Expressions help** card listing `${…}` functions with insert buttons. Sticky footer: Cancel / Save.
7. **SOAP project / operation** — ports table (name, binding/SOAP version badge, operation count, mode bar); WSDL resources list with download links. Operation detail mirrors method detail: identify strategy (ELEMENT / ELEMENT_NAMESPACE), strategy (RANDOM, SEQUENCE, XPATH_INPUT), SOAP version badge, responses table, mock endpoint + WSDL URL.
8. **Events log** — filter bar (project, type REST/SOAP, method, status class, time range, text search) + Live toggle; table: time (mono), method badge, path, status badge, duration, project › resource, handled-by (mock response name or FORWARDED). Click → right drawer (`--drawer-w`) with request/response tabs and a **Diff** tab (mock response vs actual forwarded response, or request vs matched expectation).
9. **Users & roles** — table: username, email, role badge, status badge, last login; modal for create/edit (role radio cards with a one-line description of permissions); reset password.
10. **System / settings** — cards: About (version, commit, Go version, OS, data directory, uptime), Storage (data dir, read-only mode, schema version, export button), Defaults (default network delay, events retention count, clear events), Appearance (theme segmented Light/Dark/System).

## 10. Icons

[Lucide](https://lucide.dev) (ISC licence), inlined as SVG via a templ component, 16px in controls/rows, 20px in empty states/nav, `stroke-width: 1.75`, `currentColor`. Main set: `folder-tree`, `activity`, `users`, `settings`, `search`, `plus`, `upload`, `copy`, `check`, `pencil`, `trash-2`, `more-horizontal`, `chevron-right`, `arrow-right` (forwarded), `circle-dot` (recording), `repeat` (echo), `shuffle` (random), `list-ordered` (sequence), `braces` (JSONPath), `code-xml` (XPath), `sun`/`moon`.

## 11. Motion

`--duration-fast` 120ms for hover/focus, `--duration-base` 180ms for drawers/modals (fade + 8px translate). No bouncing, no parallax. Everything goes to 0ms under `prefers-reduced-motion`.

## 12. Logo usage

| File | Use |
|---|---|
| `logo/logo.svg` | Mark + wordmark on light backgrounds |
| `logo/logo-dark.svg` | Mark + wordmark on dark backgrounds |
| `logo/logo-mark.svg` / `logo-mark-dark.svg` | Mark alone (top bar, sidebar rail) |
| `logo/logo-mark-currentcolor.svg` | Inline in templ; inherits `color` |
| `logo/logo-mono-black.svg` / `logo-mono-white.svg`, `logo-mark-mono-*.svg` | Single colour (docs, print, stickers) |
| `logo/wordmark.svg` | Wordmark only, `currentColor` |
| `logo/favicon.svg` (+ `png/favicon-{16,32,180,512}.png`) | Browser tab, apple-touch-icon, PWA |

Minimum size: mark 16px (use `favicon.svg` below 24px); full logo 96px wide. Clear space = the height of the head circle on all sides. Don't recolour the echo arc with anything but the accent or the mono colour. When inlining several SVGs on the same page, prefix the mask IDs (`mb-cut`) to keep them unique.

## 13. Stitch references

- Project: **mockingbird** — `projects/6436094703429753238`
- Design system asset: `assets/4440339641923495644`
- Screens: see the table in [`screens/README.md`](screens/README.md) (screen IDs + exported files).

Stitch output is a **visual reference**. These tokens and specs take precedence where they differ (Stitch tends to generate Material-style colour roles and Tailwind classes).
