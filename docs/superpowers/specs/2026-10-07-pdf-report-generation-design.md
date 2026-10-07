# PDF Report Generation — Design

Date: 2026-10-07

## Purpose

Add a system for generating PDF reports: an admin defines an HTML template and a
MongoDB aggregation query ("report definition"), and the frontend can trigger
generation with runtime parameters (e.g. a date range) to get back a rendered
PDF. Rendering HTML → PDF is done via headless Chrome (`chromedp`).

## Scope

In scope:
- CRUD for HTML templates
- CRUD for report definitions (template + aggregation pipeline + param schema)
- Param-driven report rendering (HTML preview and PDF generation)
- chromedp-based HTML→PDF conversion

Out of scope (not needed for this iteration):
- Async job queue / polling for long-running reports (generation is synchronous)
- A real SQL database — all data access stays on the existing MongoDB
  aggregation pipelines, consistent with the rest of this codebase
- Reusing one template across multiple report definitions, or one report
  definition across multiple templates — each report definition owns exactly
  one template pairing
- Scheduled/recurring report generation
- A template authoring UI (frontend concern, not part of this API design)

## Architecture

New packages, following the repo's existing handler → service → repository
layering:

- `internal/model` — `Template`, `ReportDefinition`, `ReportParam`
- `internal/repository` — `TemplateRepository`, `ReportDefinitionRepository`,
  one Mongo collection each, wrapping `mongo.ErrNoDocuments` into
  `apperr.ErrNotFound` (matching `user_repository.go`)
- `internal/service` — `TemplateService` (CRUD + save-time template
  validation), `ReportService` (CRUD + the render/generate pipeline)
- `internal/pdf` — new package exposing a `Renderer` interface, with a
  chromedp-backed implementation. The interface boundary keeps `ReportService`
  unit-testable without launching headless Chrome in every test.
- `internal/handler` — `TemplateHandler`, `ReportHandler`

## Data Model

```go
// internal/model/template.go
type Template struct {
    ID          primitive.ObjectID `bson:"_id" json:"id"`
    Name        string             `bson:"name" json:"name"`
    Description string             `bson:"description" json:"description"`
    HTMLContent string             `bson:"html_content" json:"htmlContent"`
    CreatedAt   time.Time          `bson:"created_at" json:"createdAt"`
    UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}
```

```go
// internal/model/report_definition.go
type ReportDefinition struct {
    ID               primitive.ObjectID `bson:"_id" json:"id"`
    Name             string             `bson:"name" json:"name"`
    TemplateID       primitive.ObjectID `bson:"template_id" json:"templateId"`
    Collection       string             `bson:"collection" json:"collection"`
    PipelineTemplate string             `bson:"pipeline_template" json:"pipelineTemplate"`
    ParamSchema      []ReportParam      `bson:"param_schema" json:"paramSchema"`
    CreatedAt        time.Time          `bson:"created_at" json:"createdAt"`
    UpdatedAt        time.Time          `bson:"updated_at" json:"updatedAt"`
}

type ReportParam struct {
    Name     string `bson:"name" json:"name"`
    Type     string `bson:"type" json:"type"` // "string" | "number" | "date" | "bool"
    Required bool   `bson:"required" json:"required"`
    Label    string `bson:"label" json:"label"`
}
```

`Template.HTMLContent` is a Go `html/template` source (auto-escaped on
execution). `PipelineTemplate` is a MongoDB Extended JSON string with
`text/template` placeholders, e.g.:

```json
[
  {"$match": {"createdAt": {"$gte": {{json .from}}, "$lte": {{json .to}}}}},
  {"$group": {"_id": "$status", "count": {"$sum": 1}}}
]
```

A `json` template func marshals each param to its declared `ReportParam.Type`
before substitution (string → quoted JSON string, date → ISO-8601 string,
number/bool → literal), so every substitution is a well-formed JSON value —
this is what keeps the substitution injection-safe, as opposed to raw string
concatenation. The result is parsed via `bson.UnmarshalExtJSON` into `bson.A`
before being passed to `Collection.Aggregate`.

## API Surface

Permissions follow the existing `RequirePermission` middleware pattern:
`PermTemplateCreate/Read/Update/Delete`, `PermReportCreate/Read/Update/Delete`,
`PermReportGenerate`.

```
# Templates (admin authoring)
POST   /api/v1/templates              create
GET    /api/v1/templates              list
GET    /api/v1/templates/:id          get (raw HTML + metadata)
PATCH  /api/v1/templates/:id          update
DELETE /api/v1/templates/:id          delete

# Report definitions (template + query pairing)
POST   /api/v1/reports                create  { name, templateId, collection, pipelineTemplate, paramSchema }
GET    /api/v1/reports                list    (includes paramSchema, for building the params form)
GET    /api/v1/reports/:id            get
PATCH  /api/v1/reports/:id            update
DELETE /api/v1/reports/:id            delete

# Rendering
POST   /api/v1/reports/:id/preview    { params } -> 200 text/html   (template render only, no chromedp)
POST   /api/v1/reports/:id/generate   { params } -> 200 application/pdf
```

`GET /reports/:id` is the primary "get model and template" lookup for the
frontend: it returns `paramSchema` (what params to collect, i.e. the "model")
and `templateId`. The frontend can separately `GET /templates/:id` for the raw
HTML if it wants a client-side preview in addition to the server-rendered
`/preview` endpoint.

## Generation Flow

`ReportService.render(ctx, reportID, params) (html string, err error)` is
shared by `preview` and `generate`:

1. Load `ReportDefinition` by ID. Not found → `apperr.ErrNotFound` → 404.
2. Validate `params` against `ParamSchema` (missing required param, or a value
   that doesn't match its declared type) → `apperr.ErrInvalidInput` → 400.
3. Substitute `params` into `PipelineTemplate` via `text/template` with the
   `json` func described above.
4. `bson.UnmarshalExtJSON` the result into `bson.A`. A parse failure here is a
   defensive check only (it's validated at save time, see below) — if it still
   happens, 500, logged with the report ID, no raw Mongo error leaked to the
   client.
5. Run the aggregation against `ReportDefinition.Collection`, bounded by a
   context timeout and `SetBatchSize` — no unbounded cursors.
6. Decode cursor results into `[]bson.M`, execute `Template.HTMLContent`
   (`html/template`) with `{Rows, Params, GeneratedAt}`. A template execution
   error is an authoring bug, not a client error → 500, logged for the admin
   to fix the template.

`generate` adds one step: pass the rendered HTML to
`pdf.Renderer.RenderHTML(ctx, html)`. The chromedp implementation navigates a
pooled/reused browser context to a `data:` URI of the HTML and invokes
`page.PrintToPDF`, rather than launching a fresh Chrome process per request
(headless Chrome startup is expensive). chromedp failure (timeout, crash) →
500, wrapped via `fmt.Errorf("render pdf: %w", err)` and logged, generic
message to the client.

**Save-time validation:** `TemplateService.Create/Update` parses
`HTMLContent` as a Go template (rejects on parse error, 400).
`ReportService.Create/Update` executes `PipelineTemplate` with dummy values
matching each declared param's type and confirms the result round-trips
through `UnmarshalExtJSON` (rejects on error, 400) — so a broken
template/pipeline is caught on save, not discovered later at generation time.

## Operational Note

chromedp requires a Chromium/Chrome binary in the runtime environment. This
needs to be added to the Dockerfile/deploy image, not just `go.mod` — flagging
here so it isn't a surprise at deploy time.

## Testing

Following the existing pattern (`testify` + testcontainers Mongo, as in
`user_service_test.go` / `user_repository_test.go`):

- `internal/pdf`: `Renderer` interface defined first. The chromedp
  implementation gets one smoke test (skipped via `testing.Short()` if Chrome
  isn't available) asserting it returns non-empty `%PDF-`-prefixed bytes for
  trivial HTML.
- `internal/service` (`ReportService`): unit tests using a fake `pdf.Renderer`
  and a real Mongo via testcontainers — covering param validation errors,
  pipeline substitution correctness, template execution, and not-found
  propagation.
- `internal/service` (`TemplateService`): save-time template parse validation
  (valid/invalid Go template syntax).
- `internal/repository`: `TemplateRepository` / `ReportDefinitionRepository`
  tests against testcontainers Mongo.
- `internal/handler`: request binding/validation and status-code mapping
  tests, mirroring `user_handler_test.go`.
- One end-to-end-ish test: create template + report definition → generate →
  assert valid PDF bytes, using the fake renderer (the real chromedp path
  stays isolated to the one smoke test above, so the suite doesn't need
  headless Chrome to pass in CI).

## Decisions Log

- **Query engine: MongoDB aggregation, not SQL.** The codebase has no SQL
  anywhere; introducing a second database just for reporting would add
  infrastructure with no other benefit here.
- **Param injection: templated Extended-JSON**, not a structured query
  builder (too limited) or native `$expr`/`let` (awkward outside
  `$lookup`/`$facet`, steeper authoring curve).
- **Template ↔ query pairing is fixed** per report definition (not a
  mix-and-match catalog) — simpler to reason about and matches the "one
  report = one document" mental model the frontend needs.
- **Generation is synchronous**, returning PDF bytes directly — no job queue,
  since reports are expected to render in a few seconds. Revisit if reports
  grow large enough to need async handling.
