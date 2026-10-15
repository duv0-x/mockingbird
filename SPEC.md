# mockingbird — SPEC

Contract for every task agent. mockingbird is a Go 1.26 reimplementation of
[CastleMock](https://github.com/castlemock/castlemock) 1.68 (archived Java/Spring
project; reference copy in `.castlemock-src/`, read-only), with a gin HTTP
server and a server-rendered templ + htmx admin UI. Java citations below
are relative to `.castlemock-src/` and abbreviate
`<module>/src/main/java/com/castlemock/` as `…/`.

Normative words: **MUST** = required for parity, **SHOULD** = recommended,
**D-n** = deliberate deviation from CastleMock (listed in §0.3).

---

## 0. Scope and decisions

### 0.1 Compatibility guarantees

| Surface | Compatible with CastleMock? |
|---|---|
| Mock URLs `{ctx}/mock/rest/...`, `{ctx}/mock/soap/...`, `?wsdl` | **Yes**, byte-for-byte paths and matching semantics (§4) |
| Admin REST API `{ctx}/api/rest/**` (paths, methods, JSON bodies, auth) | **Yes** — kept for programmatic clients (CI scripts) (§5) |
| Admin UI | **Functional parity, new implementation**: the React frontend is NOT reused; templ + htmx UI under `{ctx}/web/**` covering every CastleMock screen and action (§8) |
| Project export/import files (`.../export`, `.../import`) | **Yes** — CastleMock XML export files import; exports are CastleMock XML by default (§6.3) |
| Port 8080, context path `/castlemock` | **Yes** (§7) |
| On-disk data dir | **No.** Native storage is JSON (§3). The legacy `~/.castlemock` XML dir is supported only through `mockingbird import --castlemock <dir>` (§3.6) |

### 0.2 Storage decision (from the brief, overrides anything else)

- Native storage: versioned, pretty-printed, Git-friendly JSON under
  `MOCKINGBIRD_DATA_DIR` (default `~/.mockingbird`). One file per project.
- Projects can be seeded declaratively at startup from a read-only dir
  (`MOCKINGBIRD_SEED_DIR`, e.g. a Kubernetes ConfigMap).
- The CastleMock XML layout is documented exactly in §3.6 because the importer
  needs it; nothing else reads it.

### 0.3 Stack decision (from the brief)

- Go **1.26** (`go 1.26.0` in `go.mod`). Use `/opt/homebrew/bin/go`: `/usr/local/go/bin/go` (1.23) comes first in `PATH`.
- HTTP: **gin** for the whole server (API and UI); mock engines are `http.Handler`s mounted on gin catch-all routes.
- UI: **templ** (`github.com/a-h/templ` v0.3.977) + **htmx** 2.0.4 (vendored `internal/web/static/js/htmx.min.js`, embedded with `go:embed`), following the user's project `papiro.duv0-x.cloud`: `*_templ.go` are committed, `make generate` (`templ generate`) runs from the repo root, CI fails if generated files are stale.
- Visual design (tokens, mockups, logo) is produced by another agent in `design/` (`DESIGN.md`, Stitch). UI tasks implement it; nobody else edits `design/`.
- The UI calls `internal/service` directly (no JSON round trip); the JSON API is a thin adapter over the same service (§5.0).

### 0.4 Deliberate deviations

| Id | CastleMock behavior | mockingbird |
|---|---|---|
| D-1 | JWT signed with an RSA key generated at each start; all sessions die on restart | HS256 key persisted in `secrets/jwt.key`; sessions survive restarts |
| D-2 | `PUT /core/profile` and `PUT /core/user/{id}` without a password re-hash the stored hash (users get locked out) | Password re-hashed only when a non-empty password is sent |
| D-3 | `DuplicateRestMockResponsesService` sets `id=null` → NPE | Duplicates get a new id (same as SOAP) |
| D-4 | REST method resolution picks an arbitrary matching resource (HashMap order) | Most specific pattern wins (fewest variables/wildcards, then most literal segments), then storage order |
| D-5 | Valid token for a deleted/inactive user → `302` to the context path | `401` and the `token` cookie is cleared |
| D-6 | Forwarded/recorded response bodies are re-assembled line by line (CRLF→LF, trailing LF added) | Bytes are passed through unchanged |
| D-7 | SOAP responses copy a mock response's `Content-Encoding` header although the body is not encoded | `Content-Encoding` is never copied from mock responses (REST already does this) |
| D-8 | `JSON_PATH` strategy throws (HTTP 500) when the expression is a definite path returning a scalar | Definite path matches when it resolves to a non-null value |
| D-9 | Default `Content-Type` of REST mocks without a `Content-Type` header is Spring's `text/plain;charset=ISO-8859-1` | `text/plain;charset=UTF-8` |
| D-10 | SOAP request namespace resolution uses the outermost `xmlns` declaration | Standard XML scoping (innermost declaration wins) |
| D-11 | `PUT /rest/project/{p}/port/endpoint/forwarded` (bug in CastleMock's React frontend; scripts copied from it hit it) → 404 | Accepted as alias of `.../application/endpoint/forwarded` |
| D-12 | Admin validation errors → 500 | 400 (bad input), 404 (missing), 409 (name/username conflict). Clients that mirrored CastleMock treat every non-2xx as failure |
| D-13 | `SEQUENCE` cursor persisted inside each method/operation file | Kept in `state/sequence.json`, so project files only change on real edits |
| D-14 | No health endpoints | `GET /healthz`, `GET /readyz` outside the context path |
| D-15 | `server.mode.demo` disables admin controllers | Not supported |
| D-16 | React SPA at `{ctx}/` with hash routes `#/web/...` | templ + htmx pages at `{ctx}/web/...` (§8); `{ctx}/` redirects to `{ctx}/web` |

Everything not listed here MUST behave like CastleMock.

---

## 1. Source map

| Concern | Java source |
|---|---|
| Domain model | `model/model-core/…/model/core/**`, `model/model-mock/model-mock-{rest,soap}/…/model/mock/{rest,soap}/**` |
| Legacy XML files | `repository/**/file/**/model/*File.java`, `app/src/main/resources/file-repository.properties` |
| REST mock engine | `web/web-mock/web-mock-rest/…/web/mock/rest/controller/mock/AbstractRestServiceController.java`, `service/service-mock/service-mock-rest/…/project/IdentifyRestMethodService.java`, `service/service-core/…/utility/UrlUtility.java` |
| SOAP mock engine | `web/web-mock/web-mock-soap/…/web/mock/soap/{controller/mock,stategy,factory,converter,utility}/*`, `service/service-mock/service-mock-soap/…/{project/IdentifySoapOperationService.java,utility/SoapUtility.java}` |
| Expressions | `model/model-core/…/utility/parser/**`, grammar `model/model-expression/**/Expression.g4` |
| Admin API | `web/web-core/…/web/core/controller/rest/*`, `web/web-mock/*/…/controller/rest/*`, request models `…/model/*Request.java`, services `service/**` |
| Security | `app/…/app/config/{RestSecurityConfig,SecurityInterceptor,SecurityConfig}.java`, `web/web-core/…/config/JWTEncoderDecoder.java` |
| Definition import | `service/service-mock/service-mock-rest/…/project/converter/**`, `service/service-mock/service-mock-soap/…/project/converter/**` |
| UI functional reference | `web/web-frontend/src/**` (React; not shipped) |

---

## 2. Domain model

Go types: `internal/domain` (owner T0). JSON tags there are the **admin API
wire format**. Ids are 6 random alphanumeric chars (`IdUtility.generateId`,
`domain.NewID`). Timestamps are `domain.Time`.

### 2.1 Enums

| Enum | Values |
|---|---|
| `HttpMethod` | GET, POST, PUT, HEAD, DELETE, OPTIONS, TRACE, PATCH |
| `HttpContentEncoding` | GZIP, DEFLATE |
| `RestMethodStatus` / `SoapOperationStatus` | MOCKED, DISABLED, FORWARDED, RECORDING, RECORD_ONCE, ECHO |
| `RestResponseStrategy` | RANDOM, SEQUENCE, QUERY_MATCH, XPATH, JSON_PATH, HEADER_QUERY_MATCH |
| `SoapResponseStrategy` | RANDOM, SEQUENCE, XPATH_INPUT |
| `RestMockResponseStatus` / `SoapMockResponseStatus` | ENABLED, DISABLED |
| `SoapOperationIdentifyStrategy` | ELEMENT, ELEMENT_NAMESPACE |
| `SoapResourceType` | WSDL, SCHEMA, WSDL_IMPORT |
| `SoapVersion` | SOAP11 (`text/xml`), SOAP12 (`application/soap+xml`) |
| `RestDefinitionType` | SWAGGER, OPENAPI, WADL, RAML |
| `Role` | READER, MODIFIER, ADMIN |
| `Status` (user) | ACTIVE, INACTIVE, LOCKED |

### 2.2 Entities

Tree: `RestProject → RestApplication → RestResource → RestMethod → RestMockResponse`;
`SoapProject → SoapPort → SoapOperation → SoapMockResponse`, plus `SoapProject → SoapResource`.

"Computed" fields are filled on read by the API and never persisted.

**Project** (common): `id`, `name` (required, unique case-insensitively on update), `description` (optional), `created`, `updated`.

**RestApplication**: `id`, `name`, `projectId`, `resources[]`, computed `statusCount{status→n}` (methods under it).

**RestResource**: `id`, `name`, `uri` (Spring path pattern, may contain a `?a={x}` query template, §4.2), `applicationId`, `methods[]`, computed `statusCount`, transient `invokeAddress`.

**RestMethod**: `id`, `name`, `resourceId`, `httpMethod`, `status` (default MOCKED), `responseStrategy` (default RANDOM), `defaultBody`, `forwardedEndpoint`, `currentResponseSequenceIndex` (default 0, runtime state, D-13), `simulateNetworkDelay` (bool, nullable), `networkDelay` (ms, nullable), `defaultMockResponseId`, `automaticForward` (bool, nullable), `mockResponses[]`, computed `uri` (= parent resource uri), computed `defaultResponseName`.

**RestMockResponse**: `id`, `name`, `methodId`, `body` (default ""), `httpStatusCode` (default 200), `status` (default ENABLED), `usingExpressions` (default false), `httpHeaders[{name,value}]`, `contentEncodings[]`, `parameterQueries[]`, `xpathExpressions[{expression}]`, `jsonPathExpressions[{expression}]`, `headerQueries[]`.

**RestParameterQuery**: `parameter`, `query`, `matchCase`, `matchAny`, `matchRegex`, `urlEncoded` (all bools default false).
**RestHeaderQuery**: `header`, `query`, `matchCase`, `matchAny`, `matchRegex`.

**SoapPort**: `id`, `name`, `uri` (exact path suffix after `/mock/soap/project/{projectId}/`; equals the WSDL port name on import), `projectId`, `operations[]`, computed `statusCount`, transient `invokeAddress`.

**SoapOperation**: `id`, `name`, `portId`, `operationIdentifier{name, namespace?}`, `responseStrategy` (RANDOM), `status` (MOCKED), `httpMethod` (POST), `soapVersion`, `defaultBody`, `currentResponseSequenceIndex`, `forwardedEndpoint`, `originalEndpoint`, `simulateNetworkDelay`, `networkDelay`, `defaultMockResponseId`, `mockOnFailure`, `identifyStrategy` (ELEMENT_NAMESPACE), `automaticForward`, `mockResponses[]`, computed `defaultResponseName`, transient `invokeAddress`.

**SoapMockResponse**: `id`, `name`, `operationId`, `body`, `status`, `httpStatusCode`, `usingExpressions`, `httpHeaders[]`, `contentEncodings[]` (always empty on create), `xpathExpressions[{expression}]`.

**SoapResource**: `id`, `name` (file name), `projectId`, `type`, `content` (only in `/content`, exports, imports).

**Events** (`Event`): `id`, `resourceName` (method/operation name), `startDate`, `endDate`.
- `RestEvent`: `request{body?, contentType?, uri, httpMethod, httpHeaders[], httpParameters[]}`, `response{body?, mockResponseName?, httpStatusCode, contentType?, httpHeaders[], contentEncodings[]}|null`, `projectId`, `applicationId`, `resourceId`, `methodId`.
- `SoapEvent`: `request{body, envelope?, contentType?, uri, httpMethod, operationName?, soapVersion, httpHeaders[], operationIdentifier}`, `response{body, mockResponseName, httpStatusCode, contentType?, httpHeaders[], contentEncodings[]}|null`, `projectId`, `portId`, `operationId`.
- `OverviewEvent` = Event + `type` (`"rest"`|`"soap"`).

**User**: `id`, `username` (unique, case-insensitive), `password` (bcrypt hash; `""` in API output), `email?`, `fullName?`, `created`, `updated`, `status`, `role`.

**SystemInformation**: see §5.3.

### 2.3 JSON conventions (admin API)

- Dates: `"2024-05-01T10:20:30.123+00:00"` (Spring Boot Jackson default, UTC). Input accepts that, RFC 3339 and epoch millis (`domain.Time`).
- `Optional<String>` fields annotated `NON_EMPTY` (description, body of REST mock responses, defaultBody, forwardedEndpoint, …) are **omitted** when empty.
- `Optional<Boolean|Long>` (simulateNetworkDelay, networkDelay, automaticForward, mockOnFailure, usingExpressions) are `null` when unset.
- Lists are `[]`, maps `{}`; never `null` (call `EnsureSlices`). `statusCount` keys are enum names.
- Enum values are their names.

---

## 3. Storage (T1)

### 3.1 Data dir layout (`MOCKINGBIRD_DATA_DIR`, default `~/.mockingbird`)

```
meta.json                                   {"schemaVersion":1,"createdBy":"mockingbird <version>"}
projects/rest/<projectId>.json              one REST project tree
projects/soap/<projectId>.json              one SOAP project tree (resource metadata only)
projects/soap/<projectId>/resources/<resourceId>.wsdl   WSDL and WSDL_IMPORT content
projects/soap/<projectId>/resources/<resourceId>.xsd    SCHEMA content
users.json                                  {"schemaVersion":1,"users":[...]}
events/rest/<eventId>.json                  one RestEvent
events/soap/<eventId>.json                  one SoapEvent
state/sequence.json                         {"schemaVersion":1,"sequenceIndex":{"<methodOrOperationId>":3}}
secrets/jwt.key                             base64 HS256 key, mode 0600 (owned by T4, path from config)
```

Rules:
- Directories are created on start (0755; `secrets/` 0700). Files 0644 (secrets 0600).
- Every write is atomic: temp file in the same dir, `fsync`, `rename`.
- Encoding: UTF-8, 2-space indent, trailing newline, fixed key order (Go struct
  order as documented below), `encoding/json` with `SetEscapeHTML(false)`.
  Zero/empty optional fields are omitted. Arrays keep insertion order.
- Times in files: RFC 3339 UTC with milliseconds (`2024-05-01T10:20:30.123Z`).
- `schemaVersion` greater than supported → refuse to start with a clear error.
  A file that fails to parse is logged at ERROR, skipped and left untouched.
- All data is loaded into memory at start; reads never hit disk.
- `state/sequence.json` MAY be written behind (≤1 s delay) and MUST be flushed on `Close`.
- Events beyond `MOCKINGBIRD_{REST,SOAP}_EVENT_MAX` (default 100 each): the
  oldest (`startDate`) of that kind is deleted before adding (CreateRestEventService).

### 3.2 File schemas (schemaVersion 1)

#### 3.2.1 Common fragments
`header` = `{"name": "...", "value": "..."}`.

#### 3.2.2 REST project file — `projects/rest/<id>.json`
```json
{
  "schemaVersion": 1,
  "kind": "RestProject",
  "id": "Ab12Cd",
  "name": "Pet store",
  "description": "optional",
  "created": "2024-05-01T10:20:30.123Z",
  "updated": "2024-05-01T10:20:30.123Z",
  "applications": [
    {
      "id": "Ef34Gh",
      "name": "petstore",
      "resources": [
        {
          "id": "Ij56Kl",
          "name": "/pet/{petId}",
          "uri": "/pet/{petId}",
          "methods": [
            {
              "id": "Mn78Op",
              "name": "getPetById",
              "httpMethod": "GET",
              "status": "MOCKED",
              "responseStrategy": "RANDOM",
              "defaultBody": "",
              "forwardedEndpoint": "https://petstore.example.com/v2",
              "simulateNetworkDelay": true,
              "networkDelay": 250,
              "automaticForward": false,
              "defaultMockResponseId": "Qr90St",
              "mockResponses": [
                {
                  "id": "Qr90St",
                  "name": "200 OK",
                  "status": "ENABLED",
                  "httpStatusCode": 200,
                  "usingExpressions": true,
                  "body": "{\"id\": ${PATH_PARAMETER(parameter=\"petId\")}}",
                  "httpHeaders": [{"name": "Content-Type", "value": "application/json"}],
                  "contentEncodings": ["GZIP"],
                  "parameterQueries": [{"parameter": "petId", "query": "1", "matchCase": false, "matchAny": false, "matchRegex": false, "urlEncoded": false}],
                  "headerQueries": [{"header": "X-Env", "query": "qa", "matchCase": false, "matchAny": false, "matchRegex": false}],
                  "xpathExpressions": ["//pet[id=1]"],
                  "jsonPathExpressions": ["$.pet[?(@.id == 1)]"]
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}
```
Parent ids (`projectId`, `applicationId`, …) are implied by nesting and set by
the store on load. `currentResponseSequenceIndex` is NOT here (D-13).
The illustrative example shows every field; real files omit empty/zero ones
(only `schemaVersion`, `kind`, `id`, `name`, `created`, `updated`, collections
and enum fields are always written).

#### 3.2.3 SOAP project file — `projects/soap/<id>.json`
```json
{
  "schemaVersion": 1,
  "kind": "SoapProject",
  "id": "...", "name": "...", "description": "...", "created": "...", "updated": "...",
  "resources": [
    {"id": "Rs01ab", "name": "service.wsdl", "type": "WSDL"}
  ],
  "ports": [
    {
      "id": "...", "name": "CalculatorSoap", "uri": "CalculatorSoap",
      "operations": [
        {
          "id": "...", "name": "Add",
          "operationIdentifier": {"name": "Add", "namespace": "http://tempuri.org/"},
          "httpMethod": "POST", "soapVersion": "SOAP11",
          "status": "MOCKED", "responseStrategy": "RANDOM", "identifyStrategy": "ELEMENT_NAMESPACE",
          "defaultBody": "<soapenv:Envelope ...>",
          "forwardedEndpoint": "http://www.dneonline.com/calculator.asmx",
          "originalEndpoint": "http://www.dneonline.com/calculator.asmx",
          "simulateNetworkDelay": false, "networkDelay": 0,
          "mockOnFailure": false, "automaticForward": false,
          "defaultMockResponseId": "...",
          "mockResponses": [
            {
              "id": "...", "name": "Auto-generated mocked response", "status": "ENABLED",
              "httpStatusCode": 200, "usingExpressions": false, "body": "<soapenv:Envelope ...>",
              "httpHeaders": [], "contentEncodings": [], "xpathExpressions": ["//*[local-name()='intA']"]
            }
          ]
        }
      ]
    }
  ]
}
```
Resource content lives in `projects/soap/<projectId>/resources/<resourceId>.(wsdl|xsd)`.
In **seed files and JSON exports** a resource MAY carry `"content": "..."` inline;
the store moves it to the content file. Deleting a resource (or project) deletes its content file.

#### 3.2.4 `users.json`
```json
{"schemaVersion": 1, "users": [
  {"id": "...", "username": "admin", "password": "$2a$10$...", "email": "", "fullName": "Admin Admin",
   "status": "ACTIVE", "role": "ADMIN", "created": "...", "updated": "..."}
]}
```
If no user exists after loading, the store creates the default admin
(`UserFileRepository.postInitiate`): username `admin`, password
`MOCKINGBIRD_ADMIN_PASSWORD` or `admin`, full name `Admin Admin`, ACTIVE, ADMIN.

#### 3.2.5 Events
`events/rest/<id>.json` = `{"schemaVersion":1,"kind":"RestEvent","event":<RestEvent API JSON>}`;
SOAP likewise with `"kind":"SoapEvent"`.

### 3.3 Repository contract

`internal/storage` (interfaces, owner T0) — `Store{Rest(), Soap(), Events(), Users(), State(), Close()}`.

- Project repositories return persisted fields only; callers compute
  `statusCount`, `uri`, `defaultResponseName` and read the sequence index from `State()`.
- Child entities change through `Update(ctx, projectID, fn)`: atomic per project, serialized by a per-project lock.
- `Create` fails with `ErrConflict` on an existing project id or any child id already used anywhere in the store.
- `ProjectOf(entityID)` resolves any child id to its project (index kept by the store).
- `FindByName` / `Users().GetByUsername` are case-insensitive.
- `ErrNotFound`, `ErrConflict` are the only sentinel errors; wrap others with `%w`.
- Deleting a method/operation SHOULD delete its `State()` key.

### 3.4 Public helpers of `jsonfile` used by others

`Open(ctx, Options) (storage.Store, error)`; `EncodeRestProject`, `DecodeRestProject`,
`EncodeSoapProject(p, contents)`, `DecodeSoapProject(data) (p, contents, err)` —
the §3.2.2/§3.2.3 format, also used for seeds and JSON exports/imports (§6.3).
Decoders MUST generate missing ids and accept missing optional fields.

### 3.5 Declarative seeding (`MOCKINGBIRD_SEED_DIR`, `MOCKINGBIRD_SEED_MODE`)

- At `Open`, walk the seed dir recursively; follow symlinks to regular files;
  skip names starting with `.` (Kubernetes ConfigMap `..data` dirs) to avoid duplicates.
- `*.json` → native project file (`kind` decides REST/SOAP). `*.xml` → CastleMock export file (`castlemock.ParseRestExport` / `ParseSoapExport`, root element decides).
  Anything else is ignored with a DEBUG log.
- Mode `create-only` (default): add a project only if its id does not exist and no
  project of that type has the same name (otherwise WARN and skip).
  Mode `overwrite`: `Put` every seed project on every start (UI edits to seeded projects are lost on restart; document it).
- The seed dir is never written. A seed file that fails to parse aborts startup
  (misconfiguration must be loud).

### 3.6 Legacy CastleMock data dir (import only)

#### 3.6.1 Location
`base.file.directory` = `${user.home}/.castlemock` (`app/src/main/resources/file-repository.properties`).
Overridable with `-Dbase.file.directory=...` or env `BASE_FILE_DIRECTORY` (Spring relaxed binding).
The official Docker image runs Tomcat as root, so the volume is usually `/root/.castlemock`.

#### 3.6.2 Layout (`file-repository.properties`)

| Entity | Directory | Extension | Root element (Java class) |
|---|---|---|---|
| Users | `user/` | `.user` | `user` (`UserFile`) |
| Configuration | `configuration/` | `.conf` | `configurationGroup` — **ignore** |
| Remember-me tokens | `token/tokens.token` | — | `tokens` — **ignore** |
| Upload temp files | `tmpFiles/` | — | **ignore** |
| REST project | `rest/project/v2/` | `.prj` | `restProject` (`RestProjectFile`) |
| REST application | `rest/application/v2/` | `.app` | `restApplication` |
| REST resource | `rest/resource/v2/` | `.res` | `restResource` |
| REST method | `rest/method/v2/` | `.met` | `restMethod` |
| REST mock response | `rest/response/v2/` | `.rest` | `restMockResponse` |
| REST event | `rest/event/v1/` | `.event` | `restEvent` |
| SOAP project | `soap/project/v2/` | `.prj` | `soapProject` |
| SOAP port | `soap/port/v2/` | `.port` | `soapPort` |
| SOAP operation | `soap/operation/v2/` | `.opr` | `soapOperation` |
| SOAP mock response | `soap/response/v2/` | `.res` | `soapMockResponse` |
| SOAP resource meta | `soap/resource/v2/` | `.rsc` | `soapResource` |
| SOAP resource content | `soap/resource/v2/wsdl/<id>.rsc` (WSDL), `soap/resource/v2/schema/<id>.rsc` (SCHEMA), `soap/resource/v2/<id>.rsc` (WSDL_IMPORT — same dir as metadata!) | `.rsc` | raw text |

File name = `<id><extension>`. Older `v1` dirs (pre-1.x) are not supported: WARN if present.
Because WSDL_IMPORT content shares the metadata dir and extension, a `.rsc`
file in `soap/resource/v2/` that does not parse as `<soapResource>` is content, not metadata.

#### 3.6.3 XML format

JAXB, `XmlAccessType.NONE`, formatted, root element per table above, header
`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`. Every field is a child
element (no attributes). Null fields are absent. Lists use a wrapper element.
Dates are `xsd:dateTime` in server local time, e.g. `2023-02-01T10:15:30.123+01:00`
(fraction optional). Booleans `true|false`. The importer MUST be
order-insensitive and ignore unknown elements.

```
user:              id, username, password (bcrypt $2a$), email, fullName, updated, created, status, role
restProject:       id, name, description, updated, created
soapProject:       id, name, description, updated, created
restApplication:   id, name, projectId
restResource:      id, name, uri, applicationId
restMethod:        id, name, resourceId, defaultBody, httpMethod, forwardedEndpoint, status,
                   responseStrategy, currentResponseSequenceIndex, simulateNetworkDelay,
                   networkDelay, defaultMockResponseId, automaticForward
restMockResponse:  id, name, body, methodId, status, httpStatusCode, usingExpressions,
                   httpHeaders/httpHeader{name,value}, contentEncodings/contentEncoding,
                   parameterQueries/parameterQuery{parameter,query,matchCase,matchAny,matchRegex,urlEncoded},
                   xpathExpressions/xpathExpression{expression},
                   jsonPathExpressions/jsonPathExpression{expression},
                   headerQueries/headerQuery{header,query,matchCase,matchAny,matchRegex}
soapPort:          id, name, uri, projectId
soapOperation:     id, name, portId, responseStrategy, identifier (legacy, string),
                   operationIdentifier{name,namespace}, status, httpMethod, soapVersion,
                   defaultBody, currentResponseSequenceIndex, forwardedEndpoint, originalEndpoint,
                   defaultMockResponseId, simulateNetworkDelay, networkDelay, mockOnFailure,
                   identifyStrategy, automaticForward
soapMockResponse:  id, name, body, operationId, status, httpStatusCode, usingExpressions,
                   httpHeaders/httpHeader{name,value}, contentEncodings/contentEncoding,
                   xpathExpressions/xpathExpression{expression}
soapResource:      id, name, projectId, type
restEvent:         id, resourceName, startDate, endDate, request, response, projectId,
                   applicationId, resourceId, methodId
  request:         body, contentType, uri, httpMethod,
                   httpHeaders/httpHeader{name,value}        ← holds the QUERY PARAMETERS (Java bug)
                   httpParameters/httpParameter{name,value}  ← holds the HTTP HEADERS (Java bug)
  response:        body, mockResponseName, httpStatusCode, contentType,
                   httpHeaders/httpHeader{name,value}, contentEncodings/contentEncoding
soapEvent:         id, resourceName, startDate, endDate, request, response, projectId, portId, operationId
  request:         body, envelope, contentType, uri, httpMethod, operationName,
                   operationIdentifier{name,namespace}, soapVersion, httpHeaders/httpHeader{name,value}
  response:        body, mockResponseName, httpStatusCode, contentType,
                   httpHeaders/httpHeader{name,value}, contentEncodings/contentEncoding
```
The swap in `restEvent/request` is real: `RestRequestFile` annotates the
`httpParameters` field with `@XmlElementWrapper("httpHeaders")` and vice versa
(`repository/repository-mock/repository-mock-rest/repository-mock-rest-file/…/event/model/RestRequestFile.java`).
The importer MUST swap them back.

Example (`rest/method/v2/Mn78Op.met`):
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<restMethod>
    <id>Mn78Op</id>
    <name>getPetById</name>
    <resourceId>Ij56Kl</resourceId>
    <httpMethod>GET</httpMethod>
    <status>MOCKED</status>
    <responseStrategy>RANDOM</responseStrategy>
    <currentResponseSequenceIndex>0</currentResponseSequenceIndex>
    <simulateNetworkDelay>false</simulateNetworkDelay>
    <networkDelay>0</networkDelay>
    <automaticForward>false</automaticForward>
</restMethod>
```

#### 3.6.4 Import rules (`castlemock.ImportDataDir`)

1. Parse every file; build trees by parent id. Orphans (missing parent) → `Report.Skipped`.
2. `soapOperation`: if `operationIdentifier` is missing, use `{name: identifier || name}`. Missing enums get the §2.2 defaults.
3. `currentResponseSequenceIndex` → `State().SetSequenceIndex`.
4. Projects: id already in target → skip (WARN) unless `--overwrite` (then `Put`).
5. Users: conflict on id or username → skip unless `--overwrite`; exception: the
   target's auto-created default admin (username `admin`, password still verifies
   against `admin`/`MOCKINGBIRD_ADMIN_PASSWORD`) is replaced by the imported `admin`.
   Password hashes are copied verbatim (Spring BCrypt `$2a$10$` is compatible with `x/crypto/bcrypt`).
6. Events imported unless `--skip-events`, respecting max counts (newest kept).
7. Never modify the source dir. Golden tests: `internal/importer/castlemock/testdata/castlemock-home/**` → expected native files.

---

## 4. Mock engines

### 4.1 REST — URL and request (T2)

`{ctx}/mock/rest/project/{projectId}/application/{applicationId}/**`, all 8 HTTP methods.
No authentication. (`RestServiceController`, `AbstractRestServiceController.prepareRequest`)

- **Resource URI** = raw request path (still percent-encoded, no query string) with the
  prefix `{ctx}/mock/rest/project/{projectId}/application/{applicationId}` removed.
- **Body**: read fully as UTF-8 text; line terminators normalized to `\n`, and exactly
  one final line terminator removed (Java joins `readLine()` output with `\n`).
- **Headers**: one entry per header name with its first value.
- **Parameters**: query string only (`name`,`value` pairs, multi-valued kept).
- `contentType` = request `Content-Type`.

### 4.2 REST — method identification (`IdentifyRestMethodService`, `UrlUtility`)

1. Application: by `applicationId` (resolved with `ProjectOf`; `projectId` from the URL is only recorded in the event).
2. Candidate resources: those whose `uri` pattern (part before `?`) matches the resource URI using
   **Spring 6 `PathPattern`** semantics: case-sensitive; `{name}` = one segment;
   `{name:regex}`; `*` = one segment; `**` / `{*name}` = remaining segments; no
   optional trailing slash (`/a` ≠ `/a/`). Captured values are percent-decoded.
3. Among candidates, the first method whose `httpMethod` equals the request method (order: D-4).
   None → error (§4.8).
4. **pathParameters** (`Map<String, Set<String>>`): pattern variables, plus — for each
   `name={...}` pair in the resource uri query template (`?a={x}&b={y}`) — the request's
   values of query parameter `name`, keyed by `name` (not by the variable).

### 4.3 REST — processing by status

| Status | Behavior |
|---|---|
| DISABLED | error (§4.8) |
| FORWARDED | forward (below); no response → error |
| RECORDING | forward; then asynchronously create a mock response: name `Recorded response yyyy/MM/dd HH:mm:ss` (server local time), body, response headers, status code, ENABLED, usingExpressions=false |
| RECORD_ONCE | as RECORDING, then synchronously set the method status to MOCKED |
| ECHO | 200, body = request body, header `Content-Type` = request content type (if any) |
| MOCKED | mock selection (below) |

**Forwarding** (`RestClient`, `HttpMessageSupport`): URL = `forwardedEndpoint + resourceURI + "?" + name=value&...`
(unencoded, any order, omitted when no params). Request body sent only for POST, PUT, DELETE.
All request headers forwarded (mockingbird drops hop-by-hop headers, `Host` and
`Content-Length`). Response: status, headers except `Transfer-Encoding` and
`Content-Length`, body decoded when `Content-Encoding` contains GZIP or DEFLATE
(the decoded body is what is served/recorded), `mockResponseName = "Forwarded response"`.
TLS verification can be disabled (`MOCKINGBIRD_TLS_VERIFY=false`, was `-Dhttp.sslverify=false`).

**Mock selection** (`mockResponse(...)`):
1. Enabled mock responses, sorted by name with the alphanumeric (natural) comparator.
2. Accept filtering: if the request has `Accept`, split its first value by `,` and lowercase
   (no trimming); keep responses whose `Content-Type` header value (split by `,`, lowercased)
   shares an exact token. If none kept, use all enabled responses.
3. Empty list → error "No mocked response created for operation".
4. Strategy:
   - RANDOM: uniform.
   - SEQUENCE: `i = State(methodId)`; if `i >= len` then `i = 0`; pick `list[i]`; store `i+1`.
   - QUERY_MATCH: first response for which **every** pathParameters entry has at least one value
     matching at least one of the response's `parameterQueries` with the same `parameter`
     (`RestParameterQueryValidator`); no entries → every response matches. Fallback: default response.
   - XPATH: first response with any `xpathExpressions` matching the body (§4.5). No body or no match → default response.
   - JSON_PATH: same with `jsonPathExpressions`.
   - HEADER_QUERY_MATCH: first response where **some** request header matches **some** headerQuery
     with the same name (case-insensitive) (`RestHeaderQueryValidator`). Fallback: default response.
   - Default response = the one with id `defaultMockResponseId` within the filtered list.
   - Query/header value matching: `matchAny` → true; `matchRegex` → full match (case-insensitive
     unless `matchCase`); else equality (case-insensitive unless `matchCase`). `urlEncoded` (parameter
     queries only) → the request value is first URL-encoded (`URLEncoder.encode`, UTF-8, space→`+`).
5. Nothing selected and `automaticForward && forwardedEndpoint != ""` → forward and serve that.
6. Nothing selected → error.
7. Body: when `usingExpressions`, render with §4.4 using pathParameters, query parameters,
   request URL (scheme://host[:port]/path, no query) and request body.

**Response**: status = `httpStatusCode`; headers = mock/forward response headers except
`Content-Encoding` (a repeated name keeps the last value); body as text, no encoding applied
(`contentEncodings` is informational only). No `Content-Type` → D-9.
**Delay**: if `simulateNetworkDelay` and `networkDelay` set, sleep that many ms before writing.
**Event**: always created asynchronously after processing (also on errors after step 3 of §4.2),
`response` null on error; `resourceName` = method name; ids from URL/identification.

### 4.4 Expressions (T2, `internal/mock/expression`)

Applied to mock response bodies with `usingExpressions=true` and by `POST /core/expression/validate`.
Source: `TextParser`, `ExpressionInputParser`, `Expression.g4`, `parser/expression/*`.

**Scanning**: find `${`; the expression ends at the **first** `}` after it; the substring
`${...}` is parsed with the grammar below and replaced; scanning continues after the
start index + 1. Unknown name or parse failure → leave text as is (log).

```
expression := '${' NAME ( '(' WS? (arg (',' WS? arg)*)? ')' )? '}'
NAME       := [A-Za-z0-9_]+
arg        := argName '=' value          argName := [A-Za-z][A-Za-z0-9]*
value      := number | string | array    number  := [0-9]+ ('.' [0-9]+)?
string     := '\'? '"' chars '\'? '"'    array   := '[' WS? (value (',' WS? value)*)? ']'
```
Argument names are case-insensitive. Expression names are case-sensitive (upper case).
External inputs are added as arguments: `BODY` (request body), `URL_ARGUMENT` (request URL),
`PATH_PARAMETERS` (map name→values), `QUERY_STRINGS` (map name→value, last wins).

| Name | Args | Output |
|---|---|---|
| `RANDOM_INTEGER` | `min`=-1000, `max`=2000 | `rand.Intn(max) + min` (Java quirk kept) |
| `RANDOM_DOUBLE` | `min`=0, `max`=2^31-1 (ints) | `float64(rand.Intn(max)+min)` formatted like Java `Double.toString` (e.g. `42.0`) |
| `RANDOM_LONG` | — | random int64 |
| `RANDOM_FLOAT` | — | random float32 in [0,1) (Java `Float.toString`) |
| `RANDOM_PASSWORD` | — | **bug kept**: registered to `RandomDecimalExpression` → same as RANDOM_FLOAT. (`RANDOM_DECIMAL` itself is not registered) |
| `RANDOM_BOOLEAN` | — | `true`/`false` |
| `RANDOM_DATE` | — | `yyyy-MM-dd`, uniform in now ± 10^12 ms |
| `RANDOM_DATE_TIME` | — | `yyyy-MM-dd'T'HH:mm:ss`, same range |
| `RANDOM_STRING` | — | alphanumeric, length `rand.Intn(15)+5` |
| `RANDOM_UUID` | — | UUID v4 |
| `RANDOM_EMAIL` | `domain` (random alnum 5..14), `topDomain`="com", `min`=5, `max`=10 | `<alnum len rand.Intn(max)+min>@domain.topDomain` |
| `RANDOM_ENUM` | `values=[...]` | random element (string or number as text); missing/non-array → error (leave text) |
| `PATH_PARAMETER` | `parameter` | smallest (sorted) value of that path parameter, else `""` |
| `QUERY_STRING` | `query` | value of that query parameter, else `""` |
| `URL_HOST` | — | host of the request URL, else `""` |
| `BODY_XPATH` | `expression` | XPath string value on request body (§4.5), else `""` |
| `BODY_JSON_PATH` | `expression` | JSONPath value: single element → that element; several → JSON-ish list text; scalar → text; none → `""` |
| `FAKER` | `api` (string), `locale`="en" | Java evaluates a SpEL expression on JavaFaker (e.g. `name.firstName`). mockingbird MUST support the common `category.method` paths listed in `internal/mock/expression/faker.go` (name, address, internet, phoneNumber, company, lorem, number, commerce, …) via gofakeit; unsupported → `""` |

Numbers printed by RANDOM_ENUM/arguments follow Java `Double.toString` (`1.0`, `2.5`).

### 4.5 XPath / JSONPath (T2, `internal/mock/match`)

- XPath 1.0 over a **non-namespace-aware** parse (Java `DocumentBuilderFactory` default):
  element names include their prefix literally (`//soapenv:Body/ns:add` matches by qualified
  name; `local-name()` works). `XPathMatches`: true if the result is a non-empty node set, or any
  number/string/boolean result (Java tries NODESET, NUMBER, STRING, NODE, BOOLEAN in order and
  returns on the first that evaluates). Invalid XML → false.
- JSONPath: Jayway semantics. `JSONPathMatches`: indefinite path → non-empty array; definite
  path → non-null value (D-8). Invalid JSON/expression → false.
- `AlphanumCompare`: port `AlphanumComparator` exactly (digit chunks compared by length then digits).

### 4.6 SOAP (T3)

`POST {ctx}/mock/soap/project/{projectId}/**` (`SoapServiceController`, `HttpServletRequestConverter`).

1. **Request**: body as §4.1. If `Content-Type` starts with `multipart/` (MTOM/XOP), the
   envelope is the root MIME part (part whose `Content-ID` equals the `start` parameter, else
   the first part); otherwise envelope = body. `soapVersion` = SOAP12 if `Content-Type`
   contains `application/soap+xml`, else SOAP11. `uri` = request path with prefix
   `{ctx}/mock/soap/project/{projectId}/` removed. `httpMethod` = request method.
2. **Operation identifier** (`SoapUtility.extractSoapRequestName`): parse envelope; among the
   document element's children find the one whose local name equals `Body` (case-insensitive);
   take its first child that is an element; `name` = local name; `namespace` = namespace bound
   to its prefix (or default namespace if unprefixed) — D-10. Parse failure → error.
3. **Port**: in project `projectId`, the port whose `uri` equals `uri` exactly. None → error.
4. **Operation**: first operation of the port with `httpMethod` = request method, `soapVersion`
   = request version, `operationIdentifier.name` equal ignoring case, and (operation namespace
   empty OR `identifyStrategy` = ELEMENT OR namespaces equal ignoring case). None → error.
5. **Strategy by status** (`SoapStrategyFactory`, `stategy/*`):
   - DISABLED → error.
   - ECHO → 200, body = request body, header `Content-Type` = request content type.
   - MOCKED → mock selection; if none and `automaticForward && forwardedEndpoint` → forward (no response → error).
   - FORWARDED → forward; if status ≥ 500 and `mockOnFailure` → mock selection instead.
   - RECORDING → forward; if ≥ 500 and `mockOnFailure` → mock selection (no recording);
     else asynchronously record (name `Recorded response yyyy/MM/dd HH:mm:ss`, ENABLED, headers, status, usingExpressions=false).
   - RECORD_ONCE → as RECORDING, plus set operation status MOCKED (only when recorded).
   - Forward target = `forwardedEndpoint` (no path/query appended), request method, raw body, all headers.
6. **Mock selection** (`SoapMockStrategyResultFactory`): enabled responses sorted by name
   (alphanumeric); empty → none. RANDOM; SEQUENCE (as REST, keyed by operation id);
   XPATH_INPUT: first response with an xpath matching the **envelope**, else the default
   response (`defaultMockResponseId`, non-empty). Body rendered with expressions (inputs: request URL and raw request body only) when `usingExpressions`.
7. **Response**: status; headers: `Content-Type: <versionContentType>; charset="utf-8"`, then each
   response header overrides (except `Content-Encoding`, D-7). Delay as REST. Event as REST
   (`resourceName` = operation name) — created only once an operation is identified.

### 4.7 WSDL serving

- `GET {ctx}/mock/soap/project/{projectId}` and `GET .../{projectId}/**`: if any query parameter
  name equals `wsdl` (case-insensitive) → content of the project's first WSDL resource where the
  first occurrence of `<soap:address location="` ... `"` gets its URL replaced by the request URL
  (scheme://host[:port]/path, no query). `200`, `Content-Type: text/xml; charset="utf-8"`.
  No WSDL resource → `404` empty. Without `wsdl` parameter → error (§4.8).
- `GET {ctx}/resource/soap/project/{projectId}/resource/{resourceId}` (public, owned by T4 using
  `Soap().ResourceContent`): raw content, `200`, `text/xml;charset=UTF-8`; missing → 404.

### 4.8 Errors

Any engine failure → HTTP 500 with Spring Boot's error JSON:
`{"timestamp":"<iso>","status":500,"error":"Internal Server Error","path":"<request path>"}`,
`Content-Type: application/json`. Log at WARN with the cause.

---

## 5. Admin REST API (T4)

### 5.0 Service layer

`internal/service` (owner T4) implements every admin operation once: validation, defaults (§2.2),
cascades, computed read fields, user rules (last admin, password hashing), import/export entry points.
Its exported method set (see the stubs) is the contract for both consumers:
- `internal/api` — JSON adapter described in this section;
- `internal/web` — templ + htmx UI (§8).
Neither consumer writes to storage directly. Errors: `storage.ErrNotFound` → 404, `storage.ErrConflict`
→ 409, `*service.ValidationError` → 400 (UI: inline form error), other → 500.

Base: `{ctx}/api/rest`. JSON in/out (`application/json`). All paths below are relative to the base.

### 5.1 Authentication and authorization

- **Public**: `POST /core/login`, `GET /core/version`, `GET /core/context`, `{ctx}/doc/api/rest`,
  `{ctx}/mock/**`, `{ctx}/resource/**`, `{ctx}/static/**`, `{ctx}/web/login`. The UI (§8) shares
  the same `token` cookie and principal resolution. Everything else under `/api/rest/**` requires a principal.
- **Principal resolution** (`SecurityInterceptor`, `RestSecurityConfig`), in order:
  1. Cookie `token` → JWT.
  2. `Authorization` header with exactly two space-separated parts → second part as JWT (any scheme word).
  3. HTTP Basic (`Authorization: Basic base64(user:pass)`) → bcrypt check; user must be ACTIVE.
  A JWT whose user no longer exists or is not ACTIVE → 401 + clear cookie (D-5).
- **JWT**: HS256 (D-1), claims `iss="castlemock"`, `exp=now+7d`, `userId=<id>`.
- **Login** `POST /core/login` `{"username","password"}` → `200 {"token","username","role"}` and
  `Set-Cookie: token=<jwt>; Max-Age=604800; Path=/; HttpOnly; SameSite=Lax`. Bad credentials or non-ACTIVE → `401`.
  Username lookup is case-insensitive.
- **Logout** `GET /core/logout` → `200`, `Set-Cookie: token=; Max-Age=0; Path=/; HttpOnly` and the same for `JSESSIONID`. No auth required to succeed.
- No principal on a protected path → `401` (no `WWW-Authenticate` header, `NoPopupBasicAuthenticationEntryPoint`).
  Principal lacking the role → `403`.
- Roles: **R** = READER|MODIFIER|ADMIN, **M** = MODIFIER|ADMIN, **A** = ADMIN.
- Default admin: §3.2.4. No CORS. The JSON API has no CSRF protection (parity; cookie is HttpOnly
  and clients send JSON). UI form posts are protected by `SameSite=Lax` on the cookie plus the
  `HX-Request`/same-origin `Origin` check (§8.1).

### 5.2 Conventions

- **Lookup by leaf id**: entities are resolved by the last id in the path (`ProjectOf`), parent ids
  are ignored, exactly like CastleMock. Its React frontend relied on this (`DeleteMethodsModal`,
  `DeleteMockResponsesModal`, `DuplicateMockResponsesModal` send the method id as resource id).
  For creates, the last path id is the parent.
- Not found → `404` empty body. Errors → Spring error JSON (§4.8) with D-12 codes.
- `200` with empty body for `Void` endpoints.
- Bulk status/endpoint updates skip unknown ids silently.
- Reads compute: `statusCount` (applications in a project read, resources in an application read,
  ports in a SOAP project read), `RestMethod.uri`, `defaultResponseName`, `currentResponseSequenceIndex`.
- Responses never include password hashes (`User.Redacted`).

### 5.3 Core

| Method | Path | Role | Request | Response |
|---|---|---|---|---|
| POST | `/core/login` | public | `{username,password}` | `{token,username,role}` + cookie |
| GET | `/core/logout` | public | — | 200, clears cookies |
| GET | `/core/version` | public | — | `{"version":"<mockingbird version>"}` |
| GET | `/core/context` | public | — | `{"context":"/castlemock"}` (context path; `""` at root) |
| GET | `/core/profile` | R | — | `User` (password `""`) of the principal |
| PUT | `/core/profile` | R | `{username,password?,email?,fullName?}` | updated `User`; new username must be free (409); password only if non-empty (D-2) |
| GET | `/core/project` | R | — | `[OverviewProject{id,name,description?,created,updated,type:"rest"|"soap"}]` |
| GET | `/core/system` | A | — | `SystemInformation` (below) |
| GET | `/core/event` | R | — | `[OverviewEvent{id,resourceName,startDate,endDate,type}]` |
| DELETE | `/core/event` | M | — | 200; clears REST and SOAP events |
| POST | `/core/expression/validate` | R | `{requestBody,responseBody}` | `{"output": Render(responseBody, {Body: requestBody})}` |
| GET | `/core/user` | A | — | `[User]` |
| POST | `/core/user` | A | `{username,password,email,fullName,status,role}` | created `User`; duplicate username → 409 |
| GET | `/core/user/{userId}` | A | — | `User` |
| PUT | `/core/user/{userId}` | A | `{username,password?,email?,fullName?,status,role}` | updated `User`; last ADMIN cannot lose role ADMIN or ACTIVE status (400); username conflict 409 |
| DELETE | `/core/user/{userId}` | A | — | 200 empty; last ADMIN → 400; unknown → 404 |
| POST | `/core/project/rest/import` | M | multipart `file` | as `/rest/project/import` (deprecated alias) |
| POST | `/core/project/soap/import` | M | multipart `file` | as `/soap/project/import` (deprecated alias) |

`SystemInformation`: `operatingSystemName`=`runtime.GOOS`, `javaVersion`=`runtime.Version()`,
`javaVendor`=`"Go"`, `tomcatInfo`=`"mockingbird"`, `tomcatVersion`=version, `tomcatBuilt`=build date or `""`,
`totalMemory`/`maxMemory`/`freeMemory` in MB (bytes/1 000 000; from `runtime.MemStats`: Sys, memory limit or Sys, Sys−HeapAlloc),
`availableProcessors`=`runtime.NumCPU()`, `castleMockHomeDirectory`=data dir, `showCastleMockHomeDirectory`=`MOCKINGBIRD_SHOW_DATA_DIR`.

`{ctx}/doc/api/rest` (public): serves an OpenAPI 3 description of this API (embedded file maintained by T4)
instead of CastleMock's Swagger UI redirect.

### 5.4 REST projects

Prefix `/rest/project`. P = `{projectId}`, A = `{applicationId}`, Rs = `{resourceId}`, Me = `{methodId}`, Mr = `{responseId}`.

| Method | Path | Role | Request | Response |
|---|---|---|---|---|
| POST | `/rest/project` | M | `{name,description?,projectType?}` | `RestProject` (new id, created=updated=now, no applications) |
| GET | `/rest/project/P` | R | — | `RestProject` with `applications[]` (each with `statusCount`, `resources` = `[]`) |
| PUT | `/rest/project/P` | M | `{name,description?}` | `RestProject`; `updated`=now; name taken by another project → 409 |
| DELETE | `/rest/project/P` | M | — | deleted `RestProject` (cascade) |
| PUT | `/rest/project/P/application/status` | M | `{applicationIds[],status}` | 200; sets status of every method of those applications |
| PUT | `/rest/project/P/application/endpoint/forwarded` | M | `{applicationIds[],forwardedEndpoint}` | 200; sets forwardedEndpoint of every method |
| PUT | `/rest/project/P/port/endpoint/forwarded` | M | same | alias (D-11) |
| POST | `/rest/project/P/definition/file` | M | multipart `file`, `generateResponse` (bool), `definitionType` | 200; §6.1 |
| POST | `/rest/project/P/definition/link` | M | `{url,generateResponse,definitionType}` | 200; §6.1 |
| POST | `/rest/project/import` | M | multipart `file` | imported `RestProject`; §6.3 |
| GET | `/rest/project/P/export` | R | `?format=json` optional | CastleMock XML (default) or native JSON; §6.3 |
| POST | `/rest/project/P/application` | M | `{name}` | `RestApplication` |
| GET | `/rest/project/P/application/A` | R | — | `RestApplication` with `resources[]` (each with `statusCount`, `methods` = `[]`) |
| PUT | `/rest/project/P/application/A` | M | `{name}` | `RestApplication` |
| DELETE | `/rest/project/P/application/A` | M | — | deleted `RestApplication` |
| PUT | `/rest/project/P/application/A/resource/status` | M | `{resourceIds[],status}` | 200 (all methods of those resources) |
| PUT | `/rest/project/P/application/A/resource/endpoint/forwarded` | M | `{resourceIds[],forwardedEndpoint}` | 200 |
| POST | `/rest/project/P/application/A/resource` | M | `{name,uri}` | `RestResource` |
| GET | `/rest/project/P/application/A/resource/Rs` | R | — | `RestResource` with `methods[]` (each without mock responses) |
| PUT | `/rest/project/P/application/A/resource/Rs` | M | `{name,uri}` | `RestResource` |
| DELETE | `/rest/project/P/application/A/resource/Rs` | M | — | deleted `RestResource` |
| GET | `/rest/project/P/application/A/resource/Rs/parameter` | R | — | `["petId",...]`: names matching `\{(.+?)\}` in the resource uri |
| PUT | `/rest/project/P/application/A/resource/Rs/method/status` | M | `{methodIds[],status}` | 200 |
| PUT | `/rest/project/P/application/A/resource/Rs/method/endpoint/forwarded` | M | `{methodIds[],forwardedEndpoint}` | 200 |
| POST | `/rest/project/P/application/A/resource/Rs/method` | M | `{name,httpMethod}` | `RestMethod` (MOCKED, RANDOM, index 0) |
| GET | `.../method/Me` | R | — | `RestMethod` with `mockResponses[]`, `uri`, `defaultResponseName` |
| PUT | `.../method/Me` | M | `{name,httpMethod,status,responseStrategy,forwardedEndpoint?,simulateNetworkDelay?,networkDelay?,defaultMockResponseId?,automaticForward?}` | `RestMethod`; absent optionals become unset |
| DELETE | `.../method/Me` | M | — | deleted `RestMethod` |
| PUT | `.../method/Me/mockresponse/status` | M | `{mockResponseIds[],status}` | 200 |
| POST | `.../method/Me/mockresponse` | M | `{name,body?,httpStatusCode?,status?,usingExpressions?,httpHeaders?,contentEncodings?,parameterQueries?,xpathExpressions?,jsonPathExpressions?,headerQueries?}` | `RestMockResponse`; defaults body `""`, 200, ENABLED, false |
| POST | `.../method/Me/mockresponse/duplicate` | M | `{mockResponseIds[]}` | 200; copies with new ids, same name (D-3) |
| GET | `.../method/Me/mockresponse/Mr` | R | — | `RestMockResponse` |
| PUT | `.../method/Me/mockresponse/Mr` | M | same fields as create (all lists replace) | `RestMockResponse`; `usingExpressions` absent → false; `contentEncodings` is NOT updated (Java quirk) |
| DELETE | `.../method/Me/mockresponse/Mr` | M | — | deleted `RestMockResponse` |
| GET | `/rest/event/{eventId}` | R | — | `RestEvent` |

Updating the method's `defaultMockResponseId`/deleting responses does not cascade (dangling ids are tolerated).

### 5.5 SOAP projects

Prefix `/soap/project`. P, Po = `{portId}`, O = `{operationId}`, Mr, Rs.

| Method | Path | Role | Request | Response |
|---|---|---|---|---|
| POST | `/soap/project` | M | `{name,description?,projectType?}` | `SoapProject` |
| GET | `/soap/project/P` | R | — | `SoapProject` with `resources[]` (no content) and `ports[]` (each with `statusCount`, `operations` = `[]`) |
| PUT | `/soap/project/P` | M | `{name,description?}` | `SoapProject` (name conflict 409) |
| DELETE | `/soap/project/P` | M | — | deleted `SoapProject` |
| PUT | `/soap/project/P/port/status` | M | `{portIds[],status}` | 200 (all operations of those ports) |
| PUT | `/soap/project/P/port/endpoint/forwarded` | M | `{portIds[],forwardedEndpoint}` | 200 |
| POST | `/soap/project/P/wsdl/file` | M | multipart `file`, `generateResponse` | 200; §6.2 (includeImports=false) |
| POST | `/soap/project/P/wsdl/link` | M | `{url,generateResponse,includeImports}` | 200; §6.2 |
| POST | `/soap/project/import` | M | multipart `file` | imported `SoapProject`; §6.3 |
| GET | `/soap/project/P/export` | R | `?format=json` optional | §6.3 |
| GET | `/soap/project/P/port/Po` | R | — | `SoapPort` with `operations[]` (no mock responses) |
| PUT | `/soap/project/P/port/Po` | M | `{uri}` | `SoapPort` |
| DELETE | `/soap/project/P/port/Po` | M | — | deleted `SoapPort` |
| PUT | `/soap/project/P/port/Po/operation/status` | M | `{operationIds[],status}` | 200 |
| PUT | `/soap/project/P/port/Po/operation/endpoint/forwarded` | M | `{operationIds[],forwardedEndpoint}` | 200 |
| GET | `.../operation/O` | R | — | `SoapOperation` with `mockResponses[]`, `defaultResponseName`, `invokeAddress` not filled (clients build the mock URL themselves) |
| PUT | `.../operation/O` | M | `{responseStrategy,status,identifyStrategy,forwardedEndpoint?,simulateNetworkDelay?,networkDelay?,defaultMockResponseId?,mockOnFailure?,automaticForward?}` | `SoapOperation`; absent bools → false |
| PUT | `.../operation/O/mockresponse/status` | M | `{mockResponseIds[],status}` | 200 |
| POST | `.../operation/O/mockresponse` | M | `{name,body,status,httpStatusCode,usingExpressions?,httpHeaders,xpathExpressions}` | `SoapMockResponse` |
| POST | `.../operation/O/mockresponse/duplicate` | M | `{mockResponseIds[]}` | 200 |
| GET/PUT/DELETE | `.../operation/O/mockresponse/Mr` | R/M/M | PUT: same as create | `SoapMockResponse` |
| GET | `/soap/project/P/resource/Rs` | R | — | `SoapResource` (no content) |
| GET | `/soap/project/P/resource/Rs/content` | R | — | raw text (`text/plain;charset=UTF-8`) |
| POST | `/soap/project/P/resource/Rs/import` | M | multipart `file`, `resourceType` | `SoapResource`; WSDL replaces existing WSDL resources (`ImportSoapResourceService`) — not used by CastleMock's UI |
| GET | `/soap/event/{eventId}` | R | — | `SoapEvent` |

### 5.6 API coverage

Every call made by the CastleMock React frontend (`web/web-frontend/src/**`, axios with
`process.env.PUBLIC_URL + "/api/rest/..."`) maps to a row above. Those calls are what existing scripts
and tools use, so T4 MUST keep a test listing them (`internal/api/testdata/castlemock_calls.txt`,
generated with the grep in AGENTS.md) that asserts each one is routed with the expected role.

---

## 6. Import / export

### 6.1 REST definitions (T2, `internal/mock/rest/definition`)

Port of `service/service-mock/service-mock-rest/…/project/converter/**` and `ImportRestDefinitionService`.
Source = uploaded file or URL (downloaded with the shared client). Types:

| Type | Parser | Application name | Resource | Method | Responses (`generateResponse`) |
|---|---|---|---|---|---|
| OPENAPI (3.x, JSON/YAML) | `OpenApiRestDefinitionConverter` | `info.title` → `info.description` → first server description → first server url | one per path; name = uri = path | GET, POST, PUT, DELETE, HEAD, OPTIONS; name = operationId → summary → method; SEQUENCE; forwardedEndpoint = first server url; delay false/0; autoForward false | one per response code (non-numeric → 200), name = description, `usingExpressions=true`, headers from response headers with empty values; ENABLED only for 200; no responses → one "Auto-generated mocked response" (200, expressions on) |
| SWAGGER (2.0) | `SwaggerRestDefinitionConverter` | `info.title` → host → basePath | per path | as OpenAPI; forward = `scheme://host+basePath` | per response code and per `produces` content type (name `"<description> (<contentType>)"`, `Content-Type` header); bodies generated from schema for JSON and XML with expressions (`getExpressionIdentifier`), arrays up to min(maxItems,10) |
| RAML 0.8/1.0 | `RAML08Parser`, `RAML10Parser` | `title` | nested resources flattened, uri = parent+relative | name = method name / displayName; RANDOM | per response code `"Response N"`; body examples |
| WADL | `WADLRestDefinitionConverter` | file name without `.wadl` | `resources@base` + `resource@path` | name = `method@id`, `HttpMethod(method@name)`, RANDOM | one auto-generated response |

Merge (`ImportRestDefinitionService`): an application with the same name as an existing one is
merged: resources matched by name; new resources/methods added, existing ones kept untouched. New
applications added. Fixtures: copy `service/service-mock/service-mock-rest/src/test/resources/**` to `testdata/`.

### 6.2 WSDL (T3, `internal/mock/soap/wsdl`)

Port of `SoapPortConverter`, `DocumentConverter`, `ServicePortConverter`, `BindingOperationConverter`,
`MessagePartConverter`, `AttributeConverter`, `types/*`, `CreateSoapPortsService`.
- Namespace-aware parse. One `SoapPort` per `wsdl:service/wsdl:port` with a `soap:address`
  (SOAP 1.1 ns `http://schemas.xmlsoap.org/wsdl/soap/`) or `soap12:address`
  (`http://schemas.xmlsoap.org/wsdl/soap12/`); ports without address are skipped.
  Port `name` = `uri` = port name.
- One `SoapOperation` per binding operation: name; identifier = input message part element
  (QName resolved with document namespaces), or the part name, or the operation name (no namespace);
  POST; MOCKED; RANDOM; ELEMENT_NAMESPACE; forwarded = original = address location; version from address;
  delay/mockOnFailure/autoForward false; `defaultBody` built from the **output** identifier:
  ```
  <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:web="{ns}">
     <soapenv:Header/>
     <soapenv:Body>
        <web:{name}>?</web:{name}>
     </soapenv:Body>
  </soapenv:Envelope>
  ```
  `generateResponse` → one mock response "Auto-generated mocked response", 200, ENABLED, body = defaultBody.
- Merge: existing port by name → for each operation, existing by name gets `originalEndpoint` and
  `soapVersion` updated, new ones added with their responses. New ports added.
- Resources: all existing WSDL and WSDL_IMPORT resources are deleted, then one resource per parsed
  document (name = uploaded file name, `UploadedFile-xxxxxx` for links in Java — mockingbird uses the URL's last path segment) with the
  serialized document as content. Link + `includeImports` → follow `wsdl:import@location` recursively
  (relative to the importing URL), type WSDL_IMPORT.
- Fixtures: `service/service-mock/service-mock-soap/src/test/resources/**`.

### 6.3 Project export / import (T1 formats, T4 endpoints)

**CastleMock export format** (`ExportContainerSerializer`: Jackson `XmlMapper` + Jdk8Module, indented,
NO JAXB annotations). Root `<RestExportContainer>` / `<SoapExportContainer>`. Properties come from
getters; **lists are double-wrapped with the property name** (`<applications><applications>…</applications></applications>`),
`null` → empty element, `Optional.empty` with NON_EMPTY omitted, dates as **epoch millis**,
maps as `<statusCount><MOCKED>0</MOCKED>…</statusCount>` or `<statusCount/>`.
```xml
<RestExportContainer>
  <project><id>..</id><name>..</name><updated>1714558830123</updated><created>1714558830123</created><description>..</description><applications/></project>
  <applications><applications><id/><name/><projectId/><resources/><statusCount/></applications></applications>
  <resources><resources><id/><name/><uri/><applicationId/><methods/><statusCount/></resources></resources>
  <methods><methods><id/><name/><resourceId/><httpMethod/>…<mockResponses/>…</methods></methods>
  <mockResponses><mockResponses><id/>…<httpHeaders><httpHeaders><name/><value/></httpHeaders></httpHeaders>
     <contentEncodings><contentEncodings>GZIP</contentEncodings></contentEncodings>…</mockResponses></mockResponses>
</RestExportContainer>
```
SOAP: `project`, `ports`, `resources` (with `content`), `operations`, `mockResponses`.
Entities are flat lists linked by parent ids (`ExportRestProjectService`, `ExportSoapProjectService`).
- Parser (`ParseRestExport`, `ParseSoapExport`) MUST accept both the double-wrapped form and
  single wrapping, dates as millis or ISO, any element order, unknown elements.
- Import (`ImportRestProjectService`): fails (409) if the project id or any child id exists; sequence index reset to 0.
- `POST .../import` sniffs the uploaded file: first non-blank byte `{` → native JSON (§3.2), `<` → CastleMock XML.
- `GET .../export` default = CastleMock XML (`Content-Type: application/xml`), `?format=json` → native JSON
  with inline resource contents. Golden files: `internal/importer/castlemock/testdata/export/*.xml`.

---

## 7. Server, configuration, CLI

### 7.1 Environment variables (`internal/config`)

| Variable | Default | CastleMock equivalent |
|---|---|---|
| `MOCKINGBIRD_PORT` | `8080` | `server.port` |
| `MOCKINGBIRD_CONTEXT_PATH` | `/castlemock` (`/` = root) | WAR name / `server.servlet.context-path` |
| `MOCKINGBIRD_DATA_DIR` | `~/.mockingbird` (`/data` in Docker) | `base.file.directory` |
| `MOCKINGBIRD_SEED_DIR` | — | — |
| `MOCKINGBIRD_SEED_MODE` | `create-only` | — |
| `MOCKINGBIRD_LOG_LEVEL` | `info` | log4j level |
| `MOCKINGBIRD_LOG_FORMAT` | `text` (`json`) | — |
| `MOCKINGBIRD_REST_EVENT_MAX` / `MOCKINGBIRD_SOAP_EVENT_MAX` | `100` | `rest.event.max` / `soap.event.max` |
| `MOCKINGBIRD_UPLOAD_MAX_SIZE` | `20MB` | `server.upload.max-size` |
| `MOCKINGBIRD_TLS_VERIFY` | `true` | `http.sslverify` |
| `MOCKINGBIRD_FORWARD_TIMEOUT` | `60s` | — |
| `MOCKINGBIRD_ADMIN_PASSWORD` | `admin` | — |
| `MOCKINGBIRD_JWT_KEY_FILE` | `$DATA_DIR/secrets/jwt.key` | — |
| `MOCKINGBIRD_SHOW_DATA_DIR` | `true` | — |

### 7.2 Routing (`cmd/mockingbird`, gin)

```
GET  /healthz, /readyz                      200 "ok" (outside the context path)
ANY  {ctx}/mock/rest/*path                  internal/mock/rest  (http.Handler, no auth middleware)
ANY  {ctx}/mock/soap/*path                  internal/mock/soap  (http.Handler, no auth middleware)
     {ctx} group + auth.Middleware():
       api.Register  → /api/rest/**, /doc/api/rest, /resource/soap/project/:p/resource/:r
       web.Register  → /static/*filepath, /web/**, /  (302 → {ctx}/web)
GET  /  → 302 {ctx}/                        (when ctx != "")
```
Mock engines receive the unmodified request path. gin runs in release mode with `gin.Recovery()` and a
slog request logger. API uploads are limited to `MOCKINGBIRD_UPLOAD_MAX_SIZE`
(`http.MaxBytesReader`). Route registrations must not conflict under gin's tree router: only the mock
engines use catch-alls at the top level.

### 7.3 Logging

`log/slog` to stdout; text or JSON. One INFO line at start (version, port, context path, data dir).
Mock requests at DEBUG (method, path, project, matched entity, status, duration). Errors at WARN/ERROR
with `err`. Never log passwords, tokens or `Authorization`.

### 7.4 CLI

```
mockingbird [serve]
mockingbird import --castlemock <dir> [--data-dir <dir>] [--overwrite] [--skip-events]
mockingbird version
```
Exit code 0/1. `import` opens the target store without seeds and prints the `Report`.

---


## 8. Admin UI (T5a, T5b) — templ + htmx

### 8.1 Technical rules

- Package `internal/web` (`Register(g, Deps)`), components in `internal/web/views/*.templ`, generated
  `*_templ.go` committed; `make generate` from the repo root. Assets in `internal/web/static/`
  (`js/htmx.min.js` vendored, `css/mockingbird.css`, images/logo copied from `design/` by T5),
  served at `{ctx}/static/*` with `?v=<version>` cache busting. No Node toolchain, no CDN scripts.
- Visual design: follow `design/DESIGN.md` (tokens, components) and `design/screens/*`. Do not invent
  styles; if a screen has no mockup, compose it from the documented components. Never edit `design/`.
- Every page is a full HTML response for normal navigation; htmx requests (`HX-Request: true`) get the
  fragment only (tables, dialogs, form errors). Mutations are `POST`/`PUT`/`DELETE` via `hx-*`
  attributes, followed by fragment swaps or `HX-Redirect`/`HX-Location`. Forms work without JS where
  practical (plain POST + 303 redirect).
- Dialogs replace Bootstrap modals: `<dialog>` loaded with `hx-get` into a modal slot.
- All behavior goes through `internal/service` (§5.0) and `auth` (session). Handlers live in
  `internal/web/*.go`, one file per area (`rest_*.go`, `soap_*.go`, `events.go`, `users.go`, …).
- Auth: `auth.Middleware()` resolves the principal from the `token` cookie; protected pages use
  `auth.RequireUI(ctx+"/web/login", roleCheck)`. Controls the role cannot use are hidden (React used
  `isOnlyReader`/`canModify`/`isAdministrator`): READER sees read-only pages; MODIFIER can edit
  projects/events; ADMIN also sees Users and System. Mutating UI requests must also pass an
  `Origin`/`Referer` same-origin check (CSRF).
- Mock URLs shown in the UI are absolute: `scheme://host{ctx}/mock/...` from the current request.
- UI text in English (CastleMock parity); code and identifiers in English.

### 8.2 Routes and screens

`{ctx}/web` routes mirror CastleMock's hash routes (`MainContainer.js`) so users find the same structure.
"Bulk" = table with row checkboxes and an action bar enabled when rows are selected.
Status counts columns = Mocked / Disabled / Forwarded / Recording / Record once / Echo.

| Route | Screen (React source) | Content | Actions (dialog fields) | Role |
|---|---|---|---|---|
| `/web/login` | Login (`LoginContainer.js`) | logo, username, password, version, link to `{ctx}/doc/api/rest` | Sign in (POST → set cookie → redirect to `/web`); error message on failure | public |
| `/web/logout` | Logout | — | clears cookies, redirect to login | any |
| layout | Header/Footer (`Header.js`, `Footer.js`) | nav: Home `/web`, Profile, Events, Users (ADMIN), System (ADMIN), Logout; footer: version, REST API doc link | — | — |
| `/web` | Project overview (`ProjectOverview.js`) | searchable, paginated table: Name (link), Description, Type (REST/SOAP) | New project (Name, Description, Type REST/SOAP); Import project (Type, File — CastleMock XML or native JSON); bulk Delete (confirm list) | R / M |
| `/web/rest/project/:p` | REST project (`RestProject.js`) | Name, Description, Type; Applications table: Name (link) + status counts | Update project (Name, Description); Create application (Name); Import definition dropdown RAML / Swagger V2 / OpenAPI V3 / WADL (tabs: File upload or URL, "Generate responses for each operation"); Export (download XML; JSON option); Delete project; bulk on applications: Update status (status select), Update endpoint (Endpoint), Delete | R / M |
| `/web/rest/project/:p/application/:a` | REST application (`RestApplication.js`) | Name; Resources table: Name (link) + status counts | Update application (Name); Create resource (Name, Uri); Delete application; bulk on resources: Update status, Update endpoint, Delete | R / M |
| `.../resource/:r` | REST resource (`RestResource.js`) | URI, Address (full mock URL); Methods table: Name (link), Method type, Method status, Forwarded endpoint | Update resource (Name, Uri); Create method (Name, HTTP method); Delete resource; bulk on methods: Update status, Update endpoint, Delete | R / M |
| `.../method/:m` | REST method (`RestMethod.js`) | Type, Status, Response strategy, Address, Forwarded endpoint, Automatic forward with no match, Simulate network delay, Network delay, Default response; Mock responses table: Name (link), Status, HTTP status code | Update method (Name, Type, Status, Response strategy, Forwarded endpoint, Automatic forward, Simulate network delay, Network delay, Default response select); Create mock response (Name, Status); Delete method; bulk on responses: Update status (Enabled/Disabled), Duplicate, Delete | R / M |
| `.../mockresponse/:mr` | REST mock response (`RestMockResponse.js`) | form: Name, HTTP status code, Status, Use expression; tabs: Body (code editor textarea), Headers (Name/Value list), XPath (expressions list), JSON Path, Parameter Queries (Parameter select from `RestResourceParameters`, Query, Match Any/Case/Regex, URL Encoded), Header Queries (Header, Query, Match Any/Case/Regex) | Save; Discard changes; Validate expression dialog (Request body → rendered output via `ValidateExpression`); Delete | R / M |
| `/web/soap/project/:p` | SOAP project (`SoapProject.js`) | Name, Description, Type; Ports table: Name (link) + status counts; Resources table: Name (link), Type | Update project; Import dropdown WSDL (File or URL, "Import linked WSDLs" for URL, "Generate responses for each operation"); Export; Delete project; bulk on ports: Update status, Update endpoint, Delete | R / M |
| `/web/soap/project/:p/port/:po` | SOAP port (`SoapPort.js`) | URI, Address (mock URL), WSDL link (`...?wsdl`); Operations table: Name (link), Method, Response strategy, Status | Update port (URI); Delete port; bulk on operations: Update status, Update endpoint | R / M |
| `/web/soap/project/:p/resource/:r` | SOAP resource (`SoapResource.js`) | Name, Type, content (read-only, pretty-printed) | — | R |
| `.../operation/:o` | SOAP operation (`SoapOperation.js`) | Identifier (name + namespace), SOAP version, Identify strategy, Status, Response strategy, Address, Original endpoint, Forwarded endpoint, Automatic forward, Simulate network delay, Network delay, Default response, Mock on failure; Mock responses table: Response name (link), Status, HTTP status code | Update operation (Status, Response strategy, Identify strategy, Forwarded endpoint, Automatic forward, Simulate delay, Network delay, Default response, Mock on failure); Create mock response (Name, Status; body prefilled with `defaultBody`); bulk on responses: Update status, Duplicate, Delete | R / M |
| `.../operation/:o/mockresponse/:mr` | SOAP mock response (`SoapMockResponse.js`) | Name, HTTP status code, Status, Use expression; tabs Body, Headers, XPath | Save; Discard; Validate expression; Delete | R / M |
| `/web/event` | Events (`EventOverview.js`) | searchable, paginated table sorted by start date desc: Resource name (link to detail), Type, Start date, End date | Clear all events (M) | R |
| `/web/rest/event/:id`, `/web/soap/event/:id` | Event detail (`RestEvent.js`, `SoapEvent.js`) | Event id, project/application/resource/method (or port/operation) ids as links, start/end; tabs Request (URI, method type, body, headers, parameters) and Response (status, body, headers) | — | R |
| `/web/user` | Users (`UserOverview.js`) | table: Username (link), Name, Email, Role, Status, Created, Updated | New user (Username, Name, Email, Password, Role, Status); bulk Delete | A |
| `/web/user/:id` | User (`User.js`) | Username, Name, Email, Status, Role, Created, Updated | Update user (same fields, empty password = keep); Delete | A |
| `/web/profile` | Profile (`Profile.js`) | User data | Update profile (Username, Name, Email, Password) | R |
| `/web/system` | System (`System.js`) | OS, runtime version/vendor, server info/built/version, memory, processors, data dir (if shown) | — | A |

Body editors: plain `<textarea>` with monospace font; a "Format" action pretty-prints JSON/XML
client-side is optional (small vanilla JS in `static/js/mockingbird.js`, no frameworks).

---

## 9. Dependencies

Standard library first. Allowed third-party modules (anything else needs a SPEC change):

| Module | Used by | Why |
|---|---|---|
| `github.com/gin-gonic/gin` v1.12 | all HTTP (T4, T5, T6) | stack decision; same as papiro |
| `github.com/a-h/templ` v0.3.977 | T5 | stack decision; typed HTML components |
| htmx 2.0.4 (vendored JS, not a Go module) | T5 | stack decision |
| `golang.org/x/crypto/bcrypt` | T4 (T1 importer only verifies) | bcrypt compatible with Spring `$2a$10$` hashes |
| `github.com/golang-jwt/jwt/v5` | T4 | JWT HS256 with standard claim validation |
| `github.com/antchfx/xmlquery` + `github.com/antchfx/xpath` | T2, T3 | XPath 1.0 (stdlib has none); prefix-literal name tests fit §4.5 |
| `github.com/ohler55/ojg` (`jp`) | T2 | Jayway-compatible JSONPath incl. filters |
| `gopkg.in/yaml.v3` | T2 | OpenAPI/Swagger/RAML YAML |
| `github.com/brianvoe/gofakeit/v7` | T2 | `FAKER` expression |
| `github.com/getkin/kin-openapi` (optional) | T2 | OpenAPI 3 / Swagger 2 parsing with `$ref` resolution; T2 may instead parse the subset with yaml.v3 |

RAML and WADL are parsed with `yaml.v3` / `encoding/xml`. `go.sum` committed; `go mod tidy` clean.

---

## 10. Work breakdown

Parallel tasks; each owns its packages exclusively. Shared files (`go.mod`, `go.sum`, `SPEC.md`,
`internal/domain`, `internal/storage/storage.go`, `internal/service` signatures) change only through a
SPEC update stated in the PR (append-only `go.mod` edits are fine; resolve with `go mod tidy`).
Toolchain: `/opt/homebrew/bin/go` 1.26 and `templ` v0.3.977.

| Task | Owns | Delivers | Depends on (by contract only) |
|---|---|---|---|
| **T1 storage** | `internal/storage/jsonfile`, `internal/importer/castlemock` | `Store` (§3.1–3.5), native encoders, seeding, legacy data-dir importer (§3.6), CastleMock export writer/parser (§6.3) with golden files | domain, storage |
| **T2 rest-engine** | `internal/mock/rest`, `internal/mock/rest/definition`, `internal/mock/expression`, `internal/mock/match`, `internal/mock/forward` | §4.1–4.5, §4.8, §6.1 | storage (in-memory fake in tests) |
| **T3 soap-engine** | `internal/mock/soap`, `internal/mock/soap/wsdl` | §4.6–4.8, §6.2 | match, expression, forward (T2 signatures) |
| **T4 service + admin API + auth** | `internal/service`, `internal/api`, `internal/auth` | §5 entirely (service layer, JSON API, `/resource/soap/...`, `/doc/api/rest` OpenAPI file), SystemInformation, CastleMock-call coverage test | storage, definition, wsdl, castlemock, jsonfile encoders, expression |
| **T5a UI — shell + REST** | `internal/web` layout/shared components (`views/layout.templ`, `views/components*.templ`, `static/`), login/logout, project overview, all REST screens (§8.2 rows Login … REST mock response) | templ + htmx pages implementing `design/` | service, auth (signatures) |
| **T5b UI — SOAP + admin** | `internal/web` SOAP screens, events, users, user, profile, system (`soap_*.go`, `events.go`, `users.go`, `profile.go`, `system.go` and their `.templ`) | same | service, auth; T5a shared components (coordinate: T5a lands the layout first, T5b only adds files) |
| **T6 packaging** | `cmd/mockingbird`, `Dockerfile`, `Makefile`, `.github/workflows/`, `README.md`, `docs/` | distroless `nonroot` image (`golang:1.26` builder), port 8080, `/data` volume, `MOCKINGBIRD_DATA_DIR=/data`, context `/castlemock`; Makefile (`generate build test lint run docker`); CI (`templ generate` + `git diff --exit-code`, `go vet`, `go test -race`, image build); README + migration guide (stop CastleMock → `mockingbird import --castlemock /legacy` with the old volume mounted read-only → start mockingbird on the same port/URLs; Kubernetes seed ConfigMap example) | all (wiring) |

Rules for every task:
1. Stubs in the skeleton define the public signatures; keep them (add, don't rename). If a signature must change, update SPEC.md in the same PR and say so.
2. Tests use package-local fakes (`storage.Store`, `*service.Service` behind small interfaces where needed); nobody waits for another task to run tests.
3. `make generate && go build ./... && go vet ./... && go test ./...` green before finishing.
4. Integration (after T1–T6): end-to-end test in `cmd/mockingbird` (`e2e_test.go`, owner T6) that boots the server on a temp data dir with a seed dir, logs in through the API, creates a REST mock and calls it, renders the main UI pages (200), imports a CastleMock data dir fixture and serves a SOAP mock.

Acceptance: every screen/action of §8.2 works; every CastleMock API call of §5.6 is served; a CastleMock
data dir fixture imported with the CLI serves identical responses for its REST/SOAP mocks; CastleMock
export files import, and mockingbird exports are valid CastleMock export files.
