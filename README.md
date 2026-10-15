# mockingbird

A Go 1.26 drop-in replacement for [CastleMock](https://github.com/castlemock/castlemock) 1.68:
same mock URLs, same admin REST API, same export/import files — with a new
server-rendered admin UI (templ + htmx) and Git-friendly JSON storage.

| Surface | CastleMock compatibility |
|---|---|
| Mock URLs `{ctx}/mock/rest/**`, `{ctx}/mock/soap/**`, `?wsdl` | **Yes** — identical paths and matching semantics |
| Admin REST API `{ctx}/api/rest/**` | **Yes** — paths, methods, JSON bodies and auth kept for scripts and CI |
| Project export/import | **Yes** — CastleMock XML export files import; exports are CastleMock XML by default |
| Port 8080, context path `/castlemock` | **Yes** |
| Admin UI | New implementation at `{ctx}/web/**` (functional parity, new URLs) |
| On-disk data dir | **No** — native JSON storage; the legacy `~/.castlemock` XML dir is imported once (see [Migrating from CastleMock](#migrating-from-castlemock)) |

Deliberate deviations from CastleMock behavior are listed in `SPEC.md` §0.4
(sessions survive restarts, deterministic REST method matching, `text/plain;charset=UTF-8`
default content type, `/healthz` and `/readyz` endpoints, 400/404/409 validation
codes, …).

## Quick start

### Docker

```sh
docker build --build-arg VERSION="$(git describe --tags --always --dirty)" -t mockingbird:latest .
docker run -d --name mockingbird -p 8080:8080 -v mockingbird-data:/data mockingbird:latest
```

Open <http://localhost:8080/castlemock/web> and sign in with `admin` / `admin`
(or the value of `MOCKINGBIRD_ADMIN_PASSWORD` before the first start).

The image is distroless `nonroot` (UID 65532), serves port 8080 and keeps all
state on the `/data` volume (`MOCKINGBIRD_DATA_DIR=/data`). Health endpoints
live outside the context path: `/healthz` and `/readyz`.

### From source

Requirements: Go 1.26 (`/opt/homebrew/bin/go` on macOS Homebrew setups) and
templ v0.3.977 (`go install github.com/a-h/templ/cmd/templ@v0.3.977`).

```sh
make build            # templ generate + static binary in bin/mockingbird
MOCKINGBIRD_DATA_DIR=~/.mockingbird ./bin/mockingbird serve
```

## CLI

```
mockingbird [serve]                                        start the server (default)
mockingbird import --castlemock <dir> [--data-dir <dir>] [--overwrite] [--skip-events]
mockingbird version
```

`import` converts a CastleMock data dir into the native store **without
seeding**, prints the import report (counts, warnings, skipped items) and never
writes to the source dir. Exit code is 0 on success, 1 on error. See
[Migrating from CastleMock](#migrating-from-castlemock).

## Configuration

All configuration comes from environment variables (SPEC.md §7.1):

| Variable | Default | Meaning |
|---|---|---|
| `MOCKINGBIRD_PORT` | `8080` | listen port |
| `MOCKINGBIRD_CONTEXT_PATH` | `/castlemock` | `/` serves at the root |
| `MOCKINGBIRD_DATA_DIR` | `~/.mockingbird` (`/data` in Docker) | native JSON store location |
| `MOCKINGBIRD_SEED_DIR` | — | read-only dir of seed projects (see below) |
| `MOCKINGBIRD_SEED_MODE` | `create-only` | `create-only` \| `overwrite` |
| `MOCKINGBIRD_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `MOCKINGBIRD_LOG_FORMAT` | `text` | `text` \| `json` |
| `MOCKINGBIRD_REST_EVENT_MAX` / `MOCKINGBIRD_SOAP_EVENT_MAX` | `100` | recorded events kept per kind |
| `MOCKINGBIRD_UPLOAD_MAX_SIZE` | `20MB` | admin API upload limit (`http.MaxBytesReader`) |
| `MOCKINGBIRD_TLS_VERIFY` | `true` | TLS verification for forwarded requests |
| `MOCKINGBIRD_FORWARD_TIMEOUT` | `60s` | outbound timeout for forwarded requests |
| `MOCKINGBIRD_ADMIN_PASSWORD` | `admin` | password of the auto-created admin (empty store only) |
| `MOCKINGBIRD_JWT_KEY_FILE` | `$DATA_DIR/secrets/jwt.key` | persisted HS256 key: sessions survive restarts |
| `MOCKINGBIRD_SHOW_DATA_DIR` | `true` | show the data dir on the System screen |

## Storage layout

Native storage is versioned, pretty-printed JSON — one file per project, meant
to be diffed in Git (SPEC.md §3.1):

```
$MOCKINGBIRD_DATA_DIR/
├── meta.json
├── projects/rest/<projectId>.json          # one REST project tree
├── projects/soap/<projectId>.json          # one SOAP project tree
├── projects/soap/<projectId>/resources/    # WSDL / XSD content files
├── users.json
├── events/rest/<eventId>.json
├── events/soap/<eventId>.json
├── state/sequence.json                     # SEQUENCE strategy cursors
└── secrets/jwt.key                         # HS256 key, mode 0600
```

### Declarative seeding

`MOCKINGBIRD_SEED_DIR` points at a read-only directory (e.g. a Kubernetes
ConfigMap) walked at startup: `*.json` files are native projects, `*.xml` files
are CastleMock export files; anything else is ignored. With the default
`create-only` mode a project is added only if its id and name are free.
With `MOCKINGBIRD_SEED_MODE=overwrite` every seed project is written on every
start — edits made through the UI to seeded projects are lost on restart.

## Migrating from CastleMock

mockingbird does not read the CastleMock XML layout at runtime. The migration
is a one-shot import, then you run mockingbird on the same port and URLs.

1. **Stop CastleMock** (you are migrating the data while nothing writes to it).

2. **Locate the CastleMock data dir** (`base.file.directory`, default
   `${user.home}/.castlemock`):
   - official Docker image: the volume, mounted inside the container at
     `/root/.castlemock` (the image runs Tomcat as root);
   - standalone Tomcat/WAR: `~/.castlemock` of the user running Tomcat, or the
     `BASE_FILE_DIRECTORY` / `-Dbase.file.directory` override in use.

3. **Import** it into a fresh mockingbird data dir, with the legacy dir
   mounted read-only:

   ```sh
   docker volume create mockingbird-data
   docker run --rm \
     -v castlemock-data:/legacy:ro \
     -v mockingbird-data:/data \
     mockingbird:latest \
     import --castlemock /legacy
   ```

   or, with the binary against a plain directory:

   ```sh
   ./bin/mockingbird import --castlemock ~/.castlemock --data-dir ~/.mockingbird
   ```

   Flags: `--data-dir` (target store), `--overwrite` (replace projects/users
   that already exist in the target), `--skip-events` (leave recorded
   events behind). The command prints the import report and never modifies
   the source dir, so you can re-run it; by default already-present projects
   and users are skipped with a warning.

4. **Start mockingbird on the same port and context path** (both default to
   CastleMock's 8080 / `/castlemock`):

   ```sh
   docker run -d --name mockingbird -p 8080:8080 -v mockingbird-data:/data mockingbird:latest
   ```

   - Mock URLs are unchanged:
     `http://host:8080/castlemock/mock/rest/project/{id}/application/{id}/**`,
     `http://host:8080/castlemock/mock/soap/project/{id}/**`, `...?wsdl`.
   - The admin REST API keeps its paths and JSON, so scripts keep working.
   - Users are imported with their bcrypt password hashes — log in with the
     same credentials. CastleMock sessions and remember-me tokens are not
     carried over (and are not needed; mockingbird sessions survive restarts).
   - The admin UI is new and lives at `{ctx}/web`; the old React app is not
     shipped (`http://host:8080/castlemock/` redirects to it).

**What is imported:** REST and SOAP project trees (resources, methods,
operations, mock responses, WSDL resources and their content), users, recorded
events (honoring the configured maximum). **What is ignored:** the
`configuration/` directory, remember-me tokens, upload temp files.

**Rollback:** the legacy dir is only ever read. Keep the old volume and image
around until mockingbird is validated; to go back, stop mockingbird and start
CastleMock on port 8080 again.

## Kubernetes example

Complete manifests in [`docs/kubernetes.yaml`](docs/kubernetes.yaml):
PersistentVolumeClaim, seed ConfigMap, Deployment (distroless `nonroot`,
UID 65532, probes on `/healthz` and `/readyz`) and a Service.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mockingbird-data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: mockingbird-seed
data:
  demo.json: |
    {
      "schemaVersion": 1,
      "kind": "RestProject",
      "id": "SeedD0",
      "name": "Kubernetes demo",
      "created": "2026-01-01T00:00:00.000Z",
      "updated": "2026-01-01T00:00:00.000Z",
      "applications": [
        {
          "id": "SeedA0",
          "name": "demo",
          "resources": [
            {
              "id": "SeedU0",
              "name": "/hello",
              "uri": "/hello",
              "methods": [
                {
                  "id": "SeedM0",
                  "name": "sayHello",
                  "httpMethod": "GET",
                  "mockResponses": [
                    {
                      "id": "SeedR0",
                      "name": "hello",
                      "httpStatusCode": 200,
                      "body": "hello from the seed ConfigMap"
                    }
                  ]
                }
              ]
            }
          ]
        }
      ]
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mockingbird
  labels:
    app: mockingbird
spec:
  replicas: 1                     # single replica: the store is a PVC, not a database
  strategy:
    type: Recreate                # the PVC is ReadWriteOnce
  selector:
    matchLabels:
      app: mockingbird
  template:
    metadata:
      labels:
        app: mockingbird
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532           # distroless nonroot
        fsGroup: 65532             # chown the PVC so the nonroot user can write to it
      containers:
        - name: mockingbird
          image: mockingbird:latest   # make docker, then push to your registry
          ports:
            - name: http
              containerPort: 8080
          env:
            - name: MOCKINGBIRD_DATA_DIR
              value: /data
            - name: MOCKINGBIRD_SEED_DIR
              value: /seeds
            # Only used when the store has no users yet:
            # - name: MOCKINGBIRD_ADMIN_PASSWORD
            #   valueFrom:
            #     secretKeyRef: { name: mockingbird-admin, key: password }
          volumeMounts:
            - name: data
              mountPath: /data
            - name: seeds
              mountPath: /seeds
              readOnly: true
          livenessProbe:
            httpGet: { path: /healthz, port: http }
          readinessProbe:
            httpGet: { path: /readyz, port: http }
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: [ALL]
          resources:
            requests: { cpu: 50m, memory: 64Mi }
            limits: { cpu: 500m, memory: 256Mi }
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: mockingbird-data
        - name: seeds
          configMap:
            name: mockingbird-seed
---
apiVersion: v1
kind: Service
metadata:
  name: mockingbird
spec:
  selector:
    app: mockingbird
  ports:
    - name: http
      port: 8080
      targetPort: http
```

Notes:

- `/data` is the only writable path (the JWT key lives at
  `/data/secrets/jwt.key`); uploads up to `MOCKINGBIRD_UPLOAD_MAX_SIZE` are
  buffered in memory, so `readOnlyRootFilesystem: true` is safe.
- The seed ConfigMap is mounted read-only at `/seeds`; Kubernetes `..data`
  symlinks are skipped, so no project is seeded twice. Default
  `MOCKINGBIRD_SEED_MODE=create-only` seeds the project only while its name/id
  is free; use `overwrite` (from a Git-controlled ConfigMap) to make the seed
  the source of truth on every start.

## Development

```sh
make generate   # templ generate (run from the repo root)
make build      # generate + static binary bin/mockingbird with version ldflags
make test       # go vet + go test ./...
make lint       # gofmt check + go vet
make run        # generate + go run ./cmd/mockingbird
make docker     # build the container image
```

CI (`.github/workflows/ci.yml`) fails when `*_templ.go` are stale
(`templ generate` + `git diff --exit-code`), runs `gofmt` + `go vet` +
`go test -race ./...` on Go 1.26 and builds the Docker image.

The project layout and the full contract live in `SPEC.md`; conventions in
`AGENTS.md`.