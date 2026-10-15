# AGENTS.md — mockingbird

Conventions for every agent working in this repo. The contract is `SPEC.md`; read the
sections your task owns plus §0, §2, §10 before writing code.

## Toolchain

- Go **1.26**: always use `/opt/homebrew/bin/go` (or put `/opt/homebrew/bin` first in `PATH`);
  `/usr/local/go/bin/go` is 1.23 and comes first in the default `PATH`.
- templ **v0.3.977** (`templ` in `~/go/bin`). After editing any `.templ`, run `make generate`
  **from the repo root** (the path is baked into the generated code). `*_templ.go` are committed;
  CI fails when they are stale. Inside an element, text starting with `//` is parsed as a Go
  comment: write it as `{ "// text" }`.
- Before finishing: `make generate && go build ./... && go vet ./... && go test ./...`.

## Layout

```
cmd/mockingbird/            main, gin router wiring, CLI (serve | import | version)      T6
internal/domain/            entities, enums, JSON wire format of the admin API           T0
internal/config/            env configuration                                             T0/T6
internal/storage/           repository interfaces                                         T0
internal/storage/jsonfile/  native JSON store + seeding                                   T1
internal/importer/castlemock/  legacy XML data dir + CastleMock export files              T1
internal/mock/{rest,expression,match,forward}/, internal/mock/rest/definition/            T2
internal/mock/soap/, internal/mock/soap/wsdl/                                             T3
internal/service/           admin business operations (shared by API and UI)              T4
internal/api/, internal/auth/                                                             T4
internal/web/ (+ views/, static/)   templ + htmx UI                                       T5a/T5b
design/                     visual design by the UI/UX agent — read only, never edit
.castlemock-src/            CastleMock reference source — read only, gitignored
```

Only touch packages your task owns. Need a change in someone else's signature? Update
`SPEC.md` in the same change and call it out; don't edit their package.

## Code style

- Standard library first; third-party modules only from SPEC.md §9.
- No comments that restate the code. Comment the *why*, CastleMock quirks (cite the Java file),
  and exported identifiers. Code, identifiers, comments and UI text in English.
- Errors: wrap with context, `fmt.Errorf("load project %s: %w", id, err)`; compare with
  `errors.Is` against `storage.ErrNotFound` / `storage.ErrConflict`; no panics outside `main`/init.
- Logging: `log/slog` only, structured keys (`"project", id, "err", err`). Never log passwords,
  tokens, `Authorization` or cookie values.
- `context.Context` is the first parameter of anything that does I/O.
- HTTP: gin for routing (API and UI); mock engines stay `http.Handler`.
- Keep functions small and packages cohesive; no global mutable state except the embedded FS.

## Tests

- `go test ./...`; table-driven tests with `t.Run` names; `t.Parallel()` where safe.
- Use package-local fakes for `storage.Store` / service interfaces; never depend on another task's
  unfinished implementation.
- Golden files under `testdata/` next to the package:
  - `internal/importer/castlemock/testdata/castlemock-home/**` — legacy XML data dir fixtures
    (copy real shapes from SPEC §3.6) and the expected native JSON output.
  - `internal/importer/castlemock/testdata/export/*.xml` — CastleMock export files.
  - `internal/storage/jsonfile/testdata/*.json` — native file format.
  - `internal/mock/rest/definition/testdata/` and `internal/mock/soap/wsdl/testdata/` — copies of
    CastleMock's converter fixtures (`.castlemock-src/service/service-mock/*/src/test/resources`).
  - Update goldens with `go test ./... -update` (each package defines the `-update` flag).
- HTTP handlers are tested with `httptest` against the gin engine; UI tests assert status, key
  fragments and `HX-*` headers, not full markup.

## Reference lookups

- Frontend calls CastleMock clients use (for the API coverage test, SPEC §5.6):
  ```
  grep -rhoE 'axios\s*\.(get|post|put|delete)\(process\.env\.PUBLIC_URL \+ "[^"]+"' \
    .castlemock-src/web/web-frontend/src
  ```
- Screen behavior for the UI: `.castlemock-src/web/web-frontend/src/components/**`.

## Git

- Branch `main` for the skeleton; feature work in `feature/<task>-<slug>`.
- Commit messages in Spanish describing the change, without AI signatures or co-author lines.
- Never commit data dirs, `bin/`, `.castlemock-src/` or secrets.
