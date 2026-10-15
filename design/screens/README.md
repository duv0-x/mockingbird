# Screens

Stitch project **mockingbird**: `projects/6436094703429753238` · design system `assets/4440339641923495644`.

Each screen has an `.html` file (the Stitch export, Tailwind CDN + Google Fonts) and a `.png` at 1440×1152, rendered with headless Chrome from that HTML. The only post-processing: Stitch's placeholder icon (`bolt` / layers) was replaced with the real mockingbird mark from `../logo/`. The untouched exports, with Stitch's own screenshots, are in `stitch-original/`.

| File | Screen | Stitch screen ID |
|---|---|---|
| `login` | Login | `0a7129e8115d43be83350504c51c3af0` |
| `projects-dashboard` | Projects dashboard | `74261a659b96488f96f21ab6de30cca5` |
| `project-import` | New project modal, Import tab | `36d43d7a42c743f2b48659eb8d0b2c94` |
| `rest-project` | REST project: applications + tree | `1093c82dbefe44809ca890109da49fa7` |
| `rest-method-detail` | Method detail: mode, strategy, responses | `485d03df39374b7886e879b1b3dcb261` (final, iteration 3) |
| `mock-response-editor` | Mock response editor + expressions help | `aea094c8355f442b954c8832269aa08b` |
| `soap-project` | SOAP project: ports, nested operations, WSDL | `235ca9d83ff8412e93fde9675195b5e1` |
| `soap-operation-detail` | SOAP operation detail | `4ad5a3295c5e4be1b502466c7dd092bd` |
| `events-log` | Events log + detail drawer with diff | `baccc14f5c49473f862f1d45a52a2564` (final, iteration 2) |
| `users-roles` | Users & roles + row menu | `29a25bb06d334fed8005d019fcc6986d` |
| `settings` | System / settings + toast | `0c01a8719b0f4a62a96e9f15824235d0` |

Superseded iterations (kept in Stitch, not exported): method detail `4c97ea3f08bc4bb3bcf102b963751486` and `1ae372f14cf94aa18ac8e050d96b658d`; events log `f79a33baf9e04728951c8a6f9d6cbcfd`.

## Known deviations from DESIGN.md

The screens are a **visual reference**. When they disagree with `../DESIGN.md` or `../tokens.css`, the spec wins:

- Stitch maps colours to Material roles (`primary`, `surface-container`…) with slightly different hex values (`#111317` instead of `#0F1115`). Use the tokens.
- Content Stitch invented that is not part of the product: "Restart Service" and an "Operational" badge (settings), "New Mock" + "Docs · v1.4.2-preview" in the sidebar footer, a notifications bell, the "Workspace" breadcrumb root. The version shown should be the build version; there is no restart action.
- Rest method detail: rows 1–2 still wrap, and the "Mock endpoint" / "Recent events" cards are below the fold. The intended layout is in DESIGN.md §9.5.
- Events diff: Stitch duplicated the changed line on both sides. A correct diff shows `−` lines only on the left and `+` lines only on the right (§7.6).
- Some status badges show reason phrases ("200 OK"). The spec is number only, with the phrase in a tooltip.
- Dark theme only. The light theme is defined in `tokens.css`, but no screen renders it.
