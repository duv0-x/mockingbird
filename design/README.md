# mockingbird — design

| | |
|---|---|
| [`DESIGN.md`](DESIGN.md) | Design system spec: tokens, components, layout, htmx patterns |
| [`tokens.css`](tokens.css) | CSS custom properties, light + dark |
| [`logo/`](logo/) | Logo SVGs (+ `png/` previews and `contact-sheet.png`) |
| [`screens/`](screens/) | Stitch screens (HTML + PNG) and their IDs |

Stitch: project `projects/6436094703429753238`, design system `assets/4440339641923495644`.

## Identity concept

The northern mockingbird imitates the songs of other birds. mockingbird imitates other people's APIs. The mark is a perched, geometric bird built from four primitives: a circle for the head, a triangle for the beak, a teardrop for the body and a bar for the tail. It carries two signals:

- **The wing is a `>` chevron**, cut out of the body. Real mockingbirds have white wing patches; here the patch is a shell prompt, which says "developer tool" without a terminal cliché.
- **The teal arc in front of the beak** is the *echo*: the bird repeats what it heard. It is the only coloured element of the mark, so the brand colour literally means "the mocked response".

The wordmark is hand-drawn as monoline strokes (2.4 units, round caps and joins, geometric bowls), so it matches the stroke weight of the wing and the arc. Everything is plain SVG with no font dependency. Below 24px use `favicon.svg`: a teal tile, a dark bird with no eye, and a thicker wing cut. It was checked at 16, 32 and 64px.

## Palette reasoning

- **Graphite neutrals** (`#0F1115` → `#232835` in dark, `#F7F8FA` → `#FFFFFF` in light), with a hint of blue so the UI doesn't look dead grey next to code. The tool stays open all day next to an IDE, so the base is calm and low-glare.
- **One accent, teal** (`#2DD4BF` dark / `#0F766E` light). It is distinct from the tools users already have open: Postman orange, Grafana orange/yellow, Linear indigo, Swagger green. It also stays clear of every semantic hue. It reads as "signal" and belongs to the logo's echo.
- **Colour means something, or it isn't used.** HTTP methods follow the de-facto Swagger/Postman convention (GET green, POST blue, PUT amber, PATCH violet, DELETE red), so users don't have to relearn it. Status classes reuse the same hues (2xx green … 5xx red). Mock modes get their own mapping: MOCKED teal (the brand: "this is fake and working"), FORWARDED blue (traffic goes elsewhere), RECORDING red (the universal REC dot), RECORD_ONCE orange, ECHO violet, DISABLED gray.
- Every text/background pair meets WCAG AA in both themes (table in DESIGN.md §2.4). Badges never rely on colour alone.

## Navigation model

The data is a deep tree (Project → Application → Resource → Method → Response for REST; Project → Port → Operation → Response for SOAP). CastleMock made you click down through a page per level. mockingbird gives you three ways to move:

1. **Sidebar context tree.** The current project's hierarchy is always visible and expandable, with method badges and mode dots, so any method is one click away.
2. **Breadcrumbs** that mirror the URL. Every ancestor is one click away, and URLs are shareable (`hx-push-url`).
3. **⌘K command palette**, a server-side search across projects, methods, operations and responses.

The leaf pages (method / operation) answer the three questions people actually come with, all above the fold: *what does it return* (responses table), *how does it decide* (mode + strategy strip), *how do I call it* (endpoint + curl). Events are global but filterable, and a detail drawer with a diff keeps the list in context.

Everything is designed for templ + htmx: full pages plus partial swaps (rows, tree, dialogs, toasts), native `<dialog>` and `<details>` elements, and no client-side state.
