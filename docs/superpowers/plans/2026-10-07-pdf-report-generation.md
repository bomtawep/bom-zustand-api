# PDF Report Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin-managed HTML templates and MongoDB-aggregation-backed "report definitions" that the frontend can look up (template + param schema) and render to PDF via a chromedp-backed headless-Chrome renderer.

**Architecture:** Follows the existing layered Echo service exactly: `handler` (HTTP) → `service` (business logic, depends on narrow repository interfaces it defines itself) → `repository` (one struct per Mongo collection). Two new leaf packages: `internal/reportquery` (pure functions: param validation, param→pipeline substitution — no Mongo/HTTP dependency, easiest layer to unit test) and `internal/pdf` (a `Renderer` interface + a chromedp-backed implementation, isolated behind the interface so `ReportService` never needs real Chrome in tests).

**Tech Stack:** Go 1.27.1, Echo v5, MongoDB via `go.mongodb.org/mongo-driver` v1, `github.com/chromedp/chromedp` (new dependency, added in Task 9), `go-playground/validator/v10`, `stretchr/testify`, `testcontainers-go` (Mongo module) for repository integration tests.

## Global Constraints

- Go module `bom-zustand-api`, Go 1.27.1 (from `go.mod`). Mongo driver is v1 (`go.mongodb.org/mongo-driver`), not v2 — use `bson.M`, `bson.A`, `primitive.ObjectID`, `options.Find()`/`options.Aggregate()` builder style throughout, matching existing repositories.
- Every persisted struct declares explicit `bson` and `json` tags.
- Every repository/service method takes `context.Context` as its first argument and passes it straight down.
- Repository/service boundaries never leak `mongo.*` types or errors — only domain errors from `internal/apperr`. **No raw `*mongo.Database`/`*mongo.Collection` access from `service` or `handler`** — all aggregation runs go through a repository (this plan adds `AggregationRepository` for exactly this, Task 6).
- No pipeline is ever built by raw string concatenation of user/admin input — every substitution goes through `text/template` with the `json` func (marshals the value before it's substituted), per Task 8.
- List endpoints always cap page size server-side (reuse `defaultPageSize = 20`, `maxPageSize = 100` already defined in `internal/service`).
- Interfaces are declared in the consuming package (`service`), not the implementing one (`repository`/`pdf`).
- Unit tests use `testify` (`assert`/`require`); repository integration tests against a real MongoDB run behind `//go:build integration` (matches `user_repository_test.go`, `db/mongo_test.go`).
- Errors are wrapped with `fmt.Errorf("context: %w", err)`, never silently dropped.
- Centralize error → HTTP status mapping in `internal/middleware/error_handler.go` (Task 1) — handlers/services never write their own status-code switch.
- Permissions follow the existing fixed-role RBAC model in `internal/auth/permissions.go` (Task 2) — no new permission model.
- chromedp requires a Chromium/Chrome binary present in whatever environment runs the API; this plan exposes it as an optional `CHROME_EXEC_PATH` env var (Task 9/16) and does not touch deployment/Docker config, since none exists in this repo yet.

---

## Task 1: Domain errors and central error-status mapping

**Files:**
- Modify: `internal/apperr/errors.go`
- Modify: `internal/middleware/error_handler.go`
- Test: `internal/middleware/error_handler_test.go`

**Interfaces:**
- Produces: `apperr.ErrTemplateNotFound`, `apperr.ErrReportNotFound`, `apperr.ErrNameAlreadyExists`, `apperr.ErrInvalidReportParams`, `apperr.ErrTemplateInvalid`, `apperr.ErrPipelineInvalid` — used by every repository/service task below and by `ErrorHandler`.

- [ ] **Step 1: Write the failing test**

Add these cases to the existing table in `internal/middleware/error_handler_test.go`'s `TestErrorHandler_MapsDomainErrorsToStatusCodes`:

```go
		{apperr.ErrTemplateNotFound, http.StatusNotFound},
		{apperr.ErrReportNotFound, http.StatusNotFound},
		{apperr.ErrNameAlreadyExists, http.StatusConflict},
		{apperr.ErrInvalidReportParams, http.StatusBadRequest},
		{apperr.ErrTemplateInvalid, http.StatusBadRequest},
		{apperr.ErrPipelineInvalid, http.StatusBadRequest},
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/middleware/... -run TestErrorHandler_MapsDomainErrorsToStatusCodes -v`
Expected: build failure — `apperr.ErrTemplateNotFound` etc. undefined.

- [ ] **Step 3: Add the domain errors**

```go
// internal/apperr/errors.go
package apperr

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user is inactive")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrTokenInvalid       = errors.New("token invalid")
	ErrTokenExpired       = errors.New("token expired")
	ErrPermissionDenied   = errors.New("permission denied")

	ErrTemplateNotFound     = errors.New("template not found")
	ErrReportNotFound       = errors.New("report definition not found")
	ErrNameAlreadyExists    = errors.New("name already exists")
	ErrInvalidReportParams  = errors.New("invalid report params")
	ErrTemplateInvalid      = errors.New("template content is not valid")
	ErrPipelineInvalid      = errors.New("pipeline template is not valid")
)
```

- [ ] **Step 4: Extend the status-code mapping**

In `internal/middleware/error_handler.go`, extend the `switch`:

```go
	switch {
	case errors.Is(err, apperr.ErrInvalidCredentials),
		errors.Is(err, apperr.ErrTokenInvalid),
		errors.Is(err, apperr.ErrTokenExpired):
		status, message = http.StatusUnauthorized, err.Error()
	case errors.Is(err, apperr.ErrUserInactive),
		errors.Is(err, apperr.ErrPermissionDenied):
		status, message = http.StatusForbidden, err.Error()
	case errors.Is(err, apperr.ErrUserNotFound),
		errors.Is(err, apperr.ErrTemplateNotFound),
		errors.Is(err, apperr.ErrReportNotFound):
		status, message = http.StatusNotFound, err.Error()
	case errors.Is(err, apperr.ErrEmailAlreadyExists),
		errors.Is(err, apperr.ErrNameAlreadyExists):
		status, message = http.StatusConflict, err.Error()
	case errors.Is(err, apperr.ErrInvalidReportParams),
		errors.Is(err, apperr.ErrTemplateInvalid),
		errors.Is(err, apperr.ErrPipelineInvalid):
		status, message = http.StatusBadRequest, err.Error()
	default:
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/middleware/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/apperr/errors.go internal/middleware/error_handler.go internal/middleware/error_handler_test.go
git commit -m "feat: add domain errors for templates and report definitions"
```

---

## Task 2: RBAC permissions for templates and reports

**Files:**
- Modify: `internal/auth/permissions.go`
- Test: `internal/auth/permissions_test.go`

**Interfaces:**
- Produces: `auth.PermTemplateCreate/Read/Update/Delete`, `auth.PermReportCreate/Read/Update/Delete`, `auth.PermReportGenerate` — consumed by `RequirePermission` in the router (Task 15).
- Role grants: `RoleAdmin` gets all nine; `RoleManager` additionally gets `PermTemplateRead`, `PermReportRead`, `PermReportGenerate` (can run reports, can't author templates/pipelines); `RoleStaff` additionally gets `PermReportRead`, `PermReportGenerate` (can generate PDFs day-to-day); `RoleViewer` gets none (matches its existing all-empty grant).

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/permissions_test.go`:

```go
func TestHasPermission_AdminHasAllTemplateAndReportPermissions(t *testing.T) {
	perms := []Permission{
		PermTemplateCreate, PermTemplateRead, PermTemplateUpdate, PermTemplateDelete,
		PermReportCreate, PermReportRead, PermReportUpdate, PermReportDelete, PermReportGenerate,
	}
	for _, p := range perms {
		assert.True(t, HasPermission(RoleAdmin, p), "admin should have %s", p)
	}
}

func TestHasPermission_ManagerCanReadAndGenerateButNotAuthor(t *testing.T) {
	assert.True(t, HasPermission(RoleManager, PermTemplateRead))
	assert.True(t, HasPermission(RoleManager, PermReportRead))
	assert.True(t, HasPermission(RoleManager, PermReportGenerate))
	assert.False(t, HasPermission(RoleManager, PermTemplateCreate))
	assert.False(t, HasPermission(RoleManager, PermReportCreate))
}

func TestHasPermission_StaffCanReadAndGenerateReportsOnly(t *testing.T) {
	assert.True(t, HasPermission(RoleStaff, PermReportRead))
	assert.True(t, HasPermission(RoleStaff, PermReportGenerate))
	assert.False(t, HasPermission(RoleStaff, PermTemplateRead))
	assert.False(t, HasPermission(RoleStaff, PermReportCreate))
}

func TestHasPermission_ViewerHasNoTemplateOrReportPermissions(t *testing.T) {
	perms := []Permission{
		PermTemplateCreate, PermTemplateRead, PermTemplateUpdate, PermTemplateDelete,
		PermReportCreate, PermReportRead, PermReportUpdate, PermReportDelete, PermReportGenerate,
	}
	for _, p := range perms {
		assert.False(t, HasPermission(RoleViewer, p), "viewer should not have %s", p)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -v`
Expected: build failure — new `Perm*` identifiers undefined.

- [ ] **Step 3: Add the permissions and role grants**

```go
// internal/auth/permissions.go
package auth

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleStaff   Role = "staff"
	RoleViewer  Role = "viewer"
)

type Permission string

const (
	PermUserCreate Permission = "user:create"
	PermUserRead   Permission = "user:read"
	PermUserUpdate Permission = "user:update"
	PermUserDelete Permission = "user:delete"

	PermTemplateCreate Permission = "template:create"
	PermTemplateRead   Permission = "template:read"
	PermTemplateUpdate Permission = "template:update"
	PermTemplateDelete Permission = "template:delete"

	PermReportCreate   Permission = "report:create"
	PermReportRead     Permission = "report:read"
	PermReportUpdate   Permission = "report:update"
	PermReportDelete   Permission = "report:delete"
	PermReportGenerate Permission = "report:generate"
)

var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete,
		PermTemplateCreate, PermTemplateRead, PermTemplateUpdate, PermTemplateDelete,
		PermReportCreate, PermReportRead, PermReportUpdate, PermReportDelete, PermReportGenerate,
	},
	RoleManager: {PermUserRead, PermTemplateRead, PermReportRead, PermReportGenerate},
	RoleStaff:   {PermReportRead, PermReportGenerate},
	RoleViewer:  {},
}

func HasPermission(role Role, perm Permission) bool {
	for _, p := range rolePermissions[role] {
		if p == perm {
			return true
		}
	}
	return false
}

// PermissionsForRole returns the list of effective permissions granted to the
// given role. Unknown roles return a nil/empty slice.
func PermissionsForRole(role Role) []Permission {
	return rolePermissions[role]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/permissions.go internal/auth/permissions_test.go
git commit -m "feat: add template and report RBAC permissions"
```

---

## Task 3: Mongo indexes for templates and report_definitions

**Files:**
- Modify: `internal/db/mongo.go`
- Test: `internal/db/mongo_test.go`

**Interfaces:**
- Produces: `EnsureIndexes` now also creates a unique index on `templates.name` and `report_definitions.name` — relied on by Task 4/5 repositories to turn duplicate names into `apperr.ErrNameAlreadyExists`.

- [ ] **Step 1: Write the failing test**

Append to `internal/db/mongo_test.go`:

```go
func TestEnsureIndexes_CreatesUniqueNameIndexes(t *testing.T) {
	ctx := context.Background()

	container, err := tcmongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)
	defer container.Terminate(ctx)

	uri, err := container.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := Connect(ctx, uri)
	require.NoError(t, err)
	defer client.Disconnect(ctx)

	database := client.Database("testdb")
	require.NoError(t, EnsureIndexes(ctx, database))

	for _, collection := range []string{"templates", "report_definitions"} {
		cursor, err := database.Collection(collection).Indexes().List(ctx)
		require.NoError(t, err)
		var indexes []map[string]interface{}
		require.NoError(t, cursor.All(ctx, &indexes))

		found := false
		for _, idx := range indexes {
			if key, ok := idx["key"].(map[string]interface{}); ok {
				if _, hasName := key["name"]; hasName {
					found = true
					assert.Equal(t, true, idx["unique"])
				}
			}
		}
		assert.True(t, found, "expected a unique index on %s.name", collection)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/db/... -run TestEnsureIndexes_CreatesUniqueNameIndexes -v`
Expected: FAIL — no index on `name` found (requires Docker for testcontainers; skip locally if Docker isn't running and verify in CI).

- [ ] **Step 3: Add the indexes**

Append to `EnsureIndexes` in `internal/db/mongo.go`, before the final `return nil`:

```go
	if _, err := database.Collection("templates").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	if _, err := database.Collection("report_definitions").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags=integration ./internal/db/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/db/mongo.go internal/db/mongo_test.go
git commit -m "feat: add unique name indexes for templates and report_definitions"
```

---

## Task 4: Template model and repository

**Files:**
- Create: `internal/model/template.go`
- Create: `internal/repository/template_repository.go`
- Test: `internal/repository/template_repository_test.go`

**Interfaces:**
- Produces: `model.Template` struct; `repository.NewTemplateRepository(database *mongo.Database) *TemplateRepository` with methods `Create(ctx, *model.Template) error`, `FindByID(ctx, primitive.ObjectID) (*model.Template, error)`, `List(ctx, limit, skip int64) ([]*model.Template, error)`, `Update(ctx, primitive.ObjectID, bson.M) error`, `Delete(ctx, primitive.ObjectID) error`. Consumed by `TemplateService` (Task 10) and by `ReportService` (Task 11/12, which needs a template lookup).

- [ ] **Step 1: Write the failing test**

```go
// internal/repository/template_repository_test.go
//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func newTestTemplate(name string) *model.Template {
	now := time.Now().UTC()
	return &model.Template{
		ID:          primitive.NewObjectID(),
		Name:        name,
		Description: "a test template",
		HTMLContent: "<html><body>{{.Rows}}</body></html>",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestTemplateRepository_CreateAndFindByID(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	tmpl := newTestTemplate("invoice-template")

	require.NoError(t, repo.Create(ctx, tmpl))

	found, err := repo.FindByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, tmpl.Name, found.Name)
}

func TestTemplateRepository_CreateDuplicateNameFails(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestTemplate("dup-template")))

	err := repo.Create(ctx, newTestTemplate("dup-template"))

	require.ErrorIs(t, err, apperr.ErrNameAlreadyExists)
}

func TestTemplateRepository_FindByIDNotFoundReturnsDomainError(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))

	_, err := repo.FindByID(context.Background(), primitive.NewObjectID())

	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestTemplateRepository_UpdateChangesFields(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	tmpl := newTestTemplate("update-template")
	require.NoError(t, repo.Create(ctx, tmpl))

	require.NoError(t, repo.Update(ctx, tmpl.ID, bson.M{"description": "updated"}))

	found, err := repo.FindByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated", found.Description)
}

func TestTemplateRepository_DeleteRemovesDocument(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	tmpl := newTestTemplate("delete-template")
	require.NoError(t, repo.Create(ctx, tmpl))

	require.NoError(t, repo.Delete(ctx, tmpl.ID))

	_, err := repo.FindByID(ctx, tmpl.ID)
	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestTemplateRepository_List(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestTemplate("list-template-1")))
	require.NoError(t, repo.Create(ctx, newTestTemplate("list-template-2")))

	found, err := repo.List(ctx, 10, 0)

	require.NoError(t, err)
	assert.Len(t, found, 2)
}
```

Add the missing `"go.mongodb.org/mongo-driver/bson/primitive"` import used by `primitive.NewObjectID()` above.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/repository/... -run TestTemplateRepository -v`
Expected: build failure — `model.Template`, `NewTemplateRepository` undefined.

- [ ] **Step 3: Add the model**

```go
// internal/model/template.go
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Template struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	HTMLContent string             `bson:"html_content" json:"htmlContent"`
	CreatedAt   time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}
```

- [ ] **Step 4: Add the repository**

```go
// internal/repository/template_repository.go
package repository

import (
	"context"
	"errors"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TemplateRepository struct {
	col *mongo.Collection
}

func NewTemplateRepository(database *mongo.Database) *TemplateRepository {
	return &TemplateRepository{col: database.Collection("templates")}
}

func (r *TemplateRepository) Create(ctx context.Context, t *model.Template) error {
	_, err := r.col.InsertOne(ctx, t)
	if mongo.IsDuplicateKeyError(err) {
		return apperr.ErrNameAlreadyExists
	}
	return err
}

func (r *TemplateRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	var t model.Template
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TemplateRepository) List(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	opts := options.Find().SetLimit(limit).SetSkip(skip).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var templates []*model.Template
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

func (r *TemplateRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperr.ErrNameAlreadyExists
		}
		return err
	}
	if res.MatchedCount == 0 {
		return apperr.ErrTemplateNotFound
	}
	return nil
}

func (r *TemplateRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apperr.ErrTemplateNotFound
	}
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -tags=integration ./internal/repository/... -run TestTemplateRepository -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/model/template.go internal/repository/template_repository.go internal/repository/template_repository_test.go
git commit -m "feat: add Template model and repository"
```

---

## Task 5: ReportDefinition model and repository

**Files:**
- Create: `internal/model/report_definition.go`
- Create: `internal/repository/report_definition_repository.go`
- Test: `internal/repository/report_definition_repository_test.go`

**Interfaces:**
- Consumes: nothing new (structurally identical to Task 4's `TemplateRepository`).
- Produces: `model.ReportDefinition`, `model.ReportParam`; `repository.NewReportDefinitionRepository(database *mongo.Database) *ReportDefinitionRepository` with the same five methods as `TemplateRepository`, operating on `model.ReportDefinition`. Consumed by `ReportService` (Task 11/12).

- [ ] **Step 1: Write the failing test**

```go
// internal/repository/report_definition_repository_test.go
//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newTestReportDefinition(name string) *model.ReportDefinition {
	now := time.Now().UTC()
	return &model.ReportDefinition{
		ID:               primitive.NewObjectID(),
		Name:             name,
		TemplateID:       primitive.NewObjectID(),
		Collection:       "orders",
		PipelineTemplate: `[{"$match": {"status": {{json .status}}}}]`,
		ParamSchema: []model.ReportParam{
			{Name: "status", Type: "string", Required: true, Label: "Status"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestReportDefinitionRepository_CreateAndFindByID(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	def := newTestReportDefinition("orders-by-status")

	require.NoError(t, repo.Create(ctx, def))

	found, err := repo.FindByID(ctx, def.ID)
	require.NoError(t, err)
	assert.Equal(t, def.Name, found.Name)
	assert.Equal(t, def.ParamSchema, found.ParamSchema)
}

func TestReportDefinitionRepository_CreateDuplicateNameFails(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestReportDefinition("dup-report")))

	err := repo.Create(ctx, newTestReportDefinition("dup-report"))

	require.ErrorIs(t, err, apperr.ErrNameAlreadyExists)
}

func TestReportDefinitionRepository_FindByIDNotFoundReturnsDomainError(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))

	_, err := repo.FindByID(context.Background(), primitive.NewObjectID())

	require.ErrorIs(t, err, apperr.ErrReportNotFound)
}

func TestReportDefinitionRepository_UpdateChangesFields(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	def := newTestReportDefinition("update-report")
	require.NoError(t, repo.Create(ctx, def))

	require.NoError(t, repo.Update(ctx, def.ID, bson.M{"collection": "shipments"}))

	found, err := repo.FindByID(ctx, def.ID)
	require.NoError(t, err)
	assert.Equal(t, "shipments", found.Collection)
}

func TestReportDefinitionRepository_DeleteRemovesDocument(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	def := newTestReportDefinition("delete-report")
	require.NoError(t, repo.Create(ctx, def))

	require.NoError(t, repo.Delete(ctx, def.ID))

	_, err := repo.FindByID(ctx, def.ID)
	require.ErrorIs(t, err, apperr.ErrReportNotFound)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/repository/... -run TestReportDefinitionRepository -v`
Expected: build failure — `model.ReportDefinition`, `NewReportDefinitionRepository` undefined.

- [ ] **Step 3: Add the model**

```go
// internal/model/report_definition.go
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
	Type     string `bson:"type" json:"type"`
	Required bool   `bson:"required" json:"required"`
	Label    string `bson:"label" json:"label"`
}
```

- [ ] **Step 4: Add the repository**

```go
// internal/repository/report_definition_repository.go
package repository

import (
	"context"
	"errors"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ReportDefinitionRepository struct {
	col *mongo.Collection
}

func NewReportDefinitionRepository(database *mongo.Database) *ReportDefinitionRepository {
	return &ReportDefinitionRepository{col: database.Collection("report_definitions")}
}

func (r *ReportDefinitionRepository) Create(ctx context.Context, def *model.ReportDefinition) error {
	_, err := r.col.InsertOne(ctx, def)
	if mongo.IsDuplicateKeyError(err) {
		return apperr.ErrNameAlreadyExists
	}
	return err
}

func (r *ReportDefinitionRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	var def model.ReportDefinition
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&def)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.ErrReportNotFound
	}
	if err != nil {
		return nil, err
	}
	return &def, nil
}

func (r *ReportDefinitionRepository) List(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	opts := options.Find().SetLimit(limit).SetSkip(skip).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var defs []*model.ReportDefinition
	if err := cursor.All(ctx, &defs); err != nil {
		return nil, err
	}
	return defs, nil
}

func (r *ReportDefinitionRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperr.ErrNameAlreadyExists
		}
		return err
	}
	if res.MatchedCount == 0 {
		return apperr.ErrReportNotFound
	}
	return nil
}

func (r *ReportDefinitionRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apperr.ErrReportNotFound
	}
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -tags=integration ./internal/repository/... -run TestReportDefinitionRepository -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/model/report_definition.go internal/repository/report_definition_repository.go internal/repository/report_definition_repository_test.go
git commit -m "feat: add ReportDefinition model and repository"
```

---

## Task 6: Aggregation repository

**Files:**
- Create: `internal/repository/aggregation_repository.go`
- Test: `internal/repository/aggregation_repository_test.go`

**Interfaces:**
- Produces: `repository.NewAggregationRepository(database *mongo.Database) *AggregationRepository` with `Run(ctx context.Context, collection string, pipeline bson.A) ([]bson.M, error)`. This is the only place `ReportService` (Task 11/12) is allowed to reach Mongo, keeping the "no raw `*mongo.Database` in service" constraint from Global Constraints.

- [ ] **Step 1: Write the failing test**

```go
// internal/repository/aggregation_repository_test.go
//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestAggregationRepository_RunExecutesPipelineAgainstNamedCollection(t *testing.T) {
	database := newTestDatabase(t)
	ctx := context.Background()
	_, err := database.Collection("orders").InsertMany(ctx, []interface{}{
		bson.M{"status": "paid", "amount": 10},
		bson.M{"status": "paid", "amount": 20},
		bson.M{"status": "pending", "amount": 5},
	})
	require.NoError(t, err)

	repo := NewAggregationRepository(database)
	pipeline := bson.A{
		bson.M{"$match": bson.M{"status": "paid"}},
		bson.M{"$group": bson.M{"_id": "$status", "total": bson.M{"$sum": "$amount"}}},
	}

	rows, err := repo.Run(ctx, "orders", pipeline)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "paid", rows[0]["_id"])
	assert.EqualValues(t, 30, rows[0]["total"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/repository/... -run TestAggregationRepository -v`
Expected: build failure — `NewAggregationRepository` undefined.

- [ ] **Step 3: Add the repository**

```go
// internal/repository/aggregation_repository.go
package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AggregationRepository runs ad-hoc aggregation pipelines against a named
// collection. It exists so ReportService can execute admin-authored report
// queries without any service/handler code touching *mongo.Database
// directly.
type AggregationRepository struct {
	database *mongo.Database
}

func NewAggregationRepository(database *mongo.Database) *AggregationRepository {
	return &AggregationRepository{database: database}
}

func (r *AggregationRepository) Run(ctx context.Context, collection string, pipeline bson.A) ([]bson.M, error) {
	cursor, err := r.database.Collection(collection).Aggregate(ctx, pipeline, options.Aggregate().SetBatchSize(500))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []bson.M
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags=integration ./internal/repository/... -run TestAggregationRepository -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repository/aggregation_repository.go internal/repository/aggregation_repository_test.go
git commit -m "feat: add AggregationRepository for report queries"
```

---

## Task 7: Report param validation (`reportquery.ValidateParams`)

**Files:**
- Create: `internal/reportquery/params.go`
- Test: `internal/reportquery/params_test.go`

**Interfaces:**
- Consumes: `model.ReportParam` (Task 5).
- Produces: `reportquery.ValidateParams(schema []model.ReportParam, params map[string]interface{}) error` — returns nil if every required param is present and every present param's type matches its schema; a plain `error` with a descriptive message otherwise (wrapped with `apperr.ErrInvalidReportParams` by the caller in `ReportService.render`, Task 12 — this package does not import `apperr`, keeping it dependency-free).

- [ ] **Step 1: Write the failing test**

```go
// internal/reportquery/params_test.go
package reportquery

import (
	"testing"

	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateParams_PassesWhenAllRequiredParamsPresentWithCorrectTypes(t *testing.T) {
	schema := []model.ReportParam{
		{Name: "from", Type: "date", Required: true},
		{Name: "limit", Type: "number", Required: false},
	}
	params := map[string]interface{}{
		"from":  "2026-09-01T00:00:00Z",
		"limit": float64(50),
	}

	assert.NoError(t, ValidateParams(schema, params))
}

func TestValidateParams_FailsWhenRequiredParamMissing(t *testing.T) {
	schema := []model.ReportParam{{Name: "from", Type: "date", Required: true}}

	err := ValidateParams(schema, map[string]interface{}{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"from"`)
}

func TestValidateParams_PassesWhenOptionalParamMissing(t *testing.T) {
	schema := []model.ReportParam{{Name: "limit", Type: "number", Required: false}}

	assert.NoError(t, ValidateParams(schema, map[string]interface{}{}))
}

func TestValidateParams_FailsWhenParamTypeDoesNotMatchSchema(t *testing.T) {
	schema := []model.ReportParam{{Name: "limit", Type: "number", Required: true}}

	err := ValidateParams(schema, map[string]interface{}{"limit": "fifty"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "number")
}

func TestValidateParams_FailsWhenDateParamIsNotRFC3339(t *testing.T) {
	schema := []model.ReportParam{{Name: "from", Type: "date", Required: true}}

	err := ValidateParams(schema, map[string]interface{}{"from": "not-a-date"})

	require.Error(t, err)
}

func TestValidateParams_PassesForBoolAndStringTypes(t *testing.T) {
	schema := []model.ReportParam{
		{Name: "active", Type: "bool", Required: true},
		{Name: "status", Type: "string", Required: true},
	}
	params := map[string]interface{}{"active": true, "status": "paid"}

	assert.NoError(t, ValidateParams(schema, params))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/reportquery/... -v`
Expected: build failure — package `reportquery` / `ValidateParams` doesn't exist yet.

- [ ] **Step 3: Implement**

```go
// internal/reportquery/params.go
package reportquery

import (
	"fmt"
	"time"

	"bom-zustand-api/internal/model"
)

// ValidateParams checks that params satisfies schema: every required param
// is present, and every present param's decoded JSON type matches its
// declared schema type ("string", "number", "bool", or "date" — an
// RFC3339 string).
func ValidateParams(schema []model.ReportParam, params map[string]interface{}) error {
	for _, p := range schema {
		v, ok := params[p.Name]
		if !ok {
			if p.Required {
				return fmt.Errorf("missing required param %q", p.Name)
			}
			continue
		}
		if err := checkParamType(p, v); err != nil {
			return err
		}
	}
	return nil
}

func checkParamType(p model.ReportParam, v interface{}) error {
	switch p.Type {
	case "string":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("param %q must be a string", p.Name)
		}
	case "number":
		if _, ok := v.(float64); !ok {
			return fmt.Errorf("param %q must be a number", p.Name)
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("param %q must be a bool", p.Name)
		}
	case "date":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("param %q must be an RFC3339 date string", p.Name)
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return fmt.Errorf("param %q must be an RFC3339 date string: %w", p.Name, err)
		}
	default:
		return fmt.Errorf("param %q has unknown schema type %q", p.Name, p.Type)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reportquery/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/reportquery/params.go internal/reportquery/params_test.go
git commit -m "feat: add report param schema validation"
```

---

## Task 8: Pipeline param substitution (`reportquery.BuildPipeline`)

**Files:**
- Create: `internal/reportquery/pipeline.go`
- Test: `internal/reportquery/pipeline_test.go`

**Interfaces:**
- Produces: `reportquery.BuildPipeline(pipelineTemplate string, params map[string]interface{}) (bson.A, error)` — consumed by `ReportService` (Task 11, for save-time validation, and Task 12, for render-time execution).

Authoring note (also goes in the Create/Update report swagger doc comments in Task 14): a param substituted via `{{json .x}}` becomes a plain JSON value — a `date`-typed param becomes a JSON string, **not** a BSON `Date`. To compare against a BSON `Date` field, wrap it in Extended JSON's date form in the pipeline template: `{"$date": {{json .from}}}`.

- [ ] **Step 1: Write the failing test**

```go
// internal/reportquery/pipeline_test.go
package reportquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestBuildPipeline_SubstitutesStringParamAsQuotedJSON(t *testing.T) {
	tmpl := `[{"$match": {"status": {{json .status}}}}]`

	pipeline, err := BuildPipeline(tmpl, map[string]interface{}{"status": "paid"})

	require.NoError(t, err)
	require.Len(t, pipeline, 1)
	stage := pipeline[0].(bson.M)
	match := stage["$match"].(bson.M)
	assert.Equal(t, "paid", match["status"])
}

func TestBuildPipeline_SubstitutesDateParamAsExtendedJSONDate(t *testing.T) {
	tmpl := `[{"$match": {"createdAt": {"$gte": {"$date": {{json .from}}}}}}]`

	pipeline, err := BuildPipeline(tmpl, map[string]interface{}{"from": "2026-09-01T00:00:00Z"})

	require.NoError(t, err)
	stage := pipeline[0].(bson.M)
	match := stage["$match"].(bson.M)
	createdAt := match["createdAt"].(bson.M)
	_, isDateTime := createdAt["$gte"].(primitive.DateTime)
	assert.True(t, isDateTime, "expected $gte to decode as a BSON DateTime, got %T", createdAt["$gte"])
}

func TestBuildPipeline_FailsOnMalformedTemplateSyntax(t *testing.T) {
	_, err := BuildPipeline(`[{"$match": {{.unterminated}`, map[string]interface{}{})

	require.Error(t, err)
}

func TestBuildPipeline_FailsOnInvalidJSONAfterSubstitution(t *testing.T) {
	_, err := BuildPipeline(`[{"$match": not valid json}]`, map[string]interface{}{})

	require.Error(t, err)
}

func TestBuildPipeline_StringParamCannotBreakOutOfItsJSONPosition(t *testing.T) {
	tmpl := `[{"$match": {"status": {{json .status}}}}]`

	pipeline, err := BuildPipeline(tmpl, map[string]interface{}{"status": `", "$where": "malicious"`})

	require.NoError(t, err)
	stage := pipeline[0].(bson.M)
	match := stage["$match"].(bson.M)
	assert.Equal(t, `", "$where": "malicious"`, match["status"], "the injected value must stay a literal string value, not become new pipeline syntax")
	_, hasWhere := match["$where"]
	assert.False(t, hasWhere)
}
```

Add the `"go.mongodb.org/mongo-driver/bson/primitive"` import used by the second test.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/reportquery/... -run TestBuildPipeline -v`
Expected: build failure — `BuildPipeline` undefined.

- [ ] **Step 3: Implement**

```go
// internal/reportquery/pipeline.go
package reportquery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"go.mongodb.org/mongo-driver/bson"
)

// BuildPipeline executes pipelineTemplate (a MongoDB Extended JSON string
// containing {{json .paramName}} placeholders) against params, then parses
// the result into a bson.A aggregation pipeline. Every param is JSON-encoded
// before substitution via the "json" template func, so a param value can
// never break out of its JSON string/number/bool position into surrounding
// pipeline syntax.
func BuildPipeline(pipelineTemplate string, params map[string]interface{}) (bson.A, error) {
	tmpl, err := template.New("pipeline").Funcs(template.FuncMap{
		"json": func(v interface{}) (string, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}).Parse(pipelineTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("execute pipeline template: %w", err)
	}

	var pipeline bson.A
	if err := bson.UnmarshalExtJSON(buf.Bytes(), false, &pipeline); err != nil {
		return nil, fmt.Errorf("parse pipeline json: %w", err)
	}
	return pipeline, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reportquery/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/reportquery/pipeline.go internal/reportquery/pipeline_test.go
git commit -m "feat: add injection-safe pipeline param substitution"
```

---

## Task 9: PDF renderer (`pdf.Renderer` + chromedp implementation)

**Files:**
- Create: `internal/pdf/renderer.go`
- Create: `internal/pdf/chromedp_renderer.go`
- Test: `internal/pdf/chromedp_renderer_test.go`
- Modify: `go.mod`, `go.sum` (via `go get`)

**Interfaces:**
- Produces: `pdf.Renderer` interface (`RenderHTML(ctx context.Context, html string) ([]byte, error)`); `pdf.NewChromedpRenderer(execPath string) *ChromedpRenderer` (implements `Renderer`) and `(*ChromedpRenderer).Close()`. Consumed by `ReportService` (Task 12, via a locally-declared interface — this package's exported `Renderer` type is what `cmd/api/main.go`, Task 16, instantiates and passes in).

- [ ] **Step 1: Add the chromedp dependency**

Run: `go get github.com/chromedp/chromedp@v0.20.1 && go mod tidy`
Expected: `go.mod` gains a direct require on `github.com/chromedp/chromedp v0.20.1` (and `go.sum` updates); run `go build ./...` afterward to confirm it fetches cleanly.

- [ ] **Step 2: Write the failing test**

```go
// internal/pdf/chromedp_renderer_test.go
package pdf

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chromeAvailable() bool {
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func TestChromedpRenderer_RenderHTML_ReturnsValidPDFBytes(t *testing.T) {
	if !chromeAvailable() {
		t.Skip("no Chrome/Chromium binary found on PATH")
	}

	r := NewChromedpRenderer("")
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pdfBytes, err := r.RenderHTML(ctx, "<html><body><h1>hello</h1></body></html>")

	require.NoError(t, err)
	require.True(t, len(pdfBytes) > 4)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/pdf/... -v`
Expected: build failure — `NewChromedpRenderer` undefined (the test itself will `t.Skip` on machines without Chrome once the code compiles, which is expected and fine).

- [ ] **Step 4: Implement the interface**

```go
// internal/pdf/renderer.go
package pdf

import "context"

// Renderer converts a complete HTML document into PDF bytes.
type Renderer interface {
	RenderHTML(ctx context.Context, html string) ([]byte, error)
}
```

- [ ] **Step 5: Implement the chromedp-backed renderer**

```go
// internal/pdf/chromedp_renderer.go
package pdf

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// ChromedpRenderer renders HTML to PDF using a single headless Chrome
// process, started once at construction and reused across RenderHTML calls
// (each call opens its own tab) — launching a fresh Chrome process per
// request is too slow for a request/response cycle.
type ChromedpRenderer struct {
	allocCtx context.Context
	cancel   context.CancelFunc
}

// NewChromedpRenderer starts a headless Chrome instance. If execPath is
// empty, chromedp searches common install locations/names on PATH
// (google-chrome, chromium, etc). The caller must call Close when done to
// shut the browser process down.
func NewChromedpRenderer(execPath string) *ChromedpRenderer {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Headless)
	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	return &ChromedpRenderer{allocCtx: allocCtx, cancel: cancel}
}

func (r *ChromedpRenderer) Close() {
	r.cancel()
}

func (r *ChromedpRenderer) RenderHTML(ctx context.Context, html string) ([]byte, error) {
	tabCtx, tabCancel := chromedp.NewContext(r.allocCtx)
	defer tabCancel()
	tabCtx, timeoutCancel := context.WithTimeout(tabCtx, 30*time.Second)
	defer timeoutCancel()

	dataURI := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))

	var pdfBytes []byte
	err := chromedp.Run(tabCtx,
		chromedp.Navigate(dataURI),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			if err != nil {
				return err
			}
			pdfBytes = buf
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return pdfBytes, nil
}
```

Note: `ctx` (the caller's context, e.g. the HTTP request context) is intentionally not threaded into `tabCtx` here beyond its deadline — `tabCtx` is derived from the long-lived `r.allocCtx`, not from the per-request `ctx`, since the browser allocator must outlive any single request. If request cancellation needs to abort an in-flight render, that's a follow-up, not required for this plan.

- [ ] **Step 6: Re-sync go.mod/go.sum for the newly-imported `cdproto/page` package**

Run: `go mod tidy`
Expected: `go.sum` gains entries for `github.com/chromedp/cdproto` (pulled in transitively by Step 1's `go get`, but only recorded as a direct import's dependency once the code in Step 5 actually imports it).

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./internal/pdf/... -v`
Expected: PASS (either a real render if Chrome is on PATH, or a `SKIP`).

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/pdf/renderer.go internal/pdf/chromedp_renderer.go internal/pdf/chromedp_renderer_test.go
git commit -m "feat: add chromedp-backed HTML to PDF renderer"
```

---

## Task 10: TemplateService

**Files:**
- Create: `internal/service/template_service.go`
- Test: `internal/service/template_service_test.go`

**Interfaces:**
- Consumes: `repository.TemplateRepository`'s method set (Task 4), via a locally-declared `templateRepository` interface.
- Produces: `service.NewTemplateService(templates templateRepository) *TemplateService` with `CreateTemplate(ctx, name, description, htmlContent string) (*model.Template, error)`, `ListTemplates(ctx, limit, skip int64) ([]*model.Template, error)`, `GetTemplate(ctx, primitive.ObjectID) (*model.Template, error)`, `UpdateTemplate(ctx, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error)`, `DeleteTemplate(ctx, primitive.ObjectID) error`. Consumed by `TemplateHandler` (Task 13) and `cmd/api/main.go` (Task 16). Also implements the narrower `templateLookup` interface `ReportService` (Task 11) declares for itself.

- [ ] **Step 1: Write the failing test**

```go
// internal/service/template_service_test.go
package service

import (
	"context"
	"testing"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeTemplateRepo struct {
	byID           map[primitive.ObjectID]*model.Template
	lastUpdateID   primitive.ObjectID
	lastUpdateDoc  bson.M
	deletedID      primitive.ObjectID
	createErr      error
}

func newFakeTemplateRepo() *fakeTemplateRepo {
	return &fakeTemplateRepo{byID: map[primitive.ObjectID]*model.Template{}}
}

func (f *fakeTemplateRepo) Create(ctx context.Context, t *model.Template) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[t.ID] = t
	return nil
}
func (f *fakeTemplateRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, apperr.ErrTemplateNotFound
	}
	return t, nil
}
func (f *fakeTemplateRepo) List(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	var out []*model.Template
	for _, t := range f.byID {
		out = append(out, t)
	}
	return out, nil
}
func (f *fakeTemplateRepo) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	t, ok := f.byID[id]
	if !ok {
		return apperr.ErrTemplateNotFound
	}
	f.lastUpdateID, f.lastUpdateDoc = id, update
	if desc, ok := update["description"].(string); ok {
		t.Description = desc
	}
	if html, ok := update["html_content"].(string); ok {
		t.HTMLContent = html
	}
	return nil
}
func (f *fakeTemplateRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.ErrTemplateNotFound
	}
	f.deletedID = id
	delete(f.byID, id)
	return nil
}

func TestTemplateService_CreateTemplate_Succeeds(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)

	tmpl, err := svc.CreateTemplate(context.Background(), "invoice", "an invoice", "<html><body>{{.Rows}}</body></html>")

	require.NoError(t, err)
	assert.Equal(t, "invoice", tmpl.Name)
	assert.Contains(t, repo.byID, tmpl.ID)
}

func TestTemplateService_CreateTemplate_RejectsInvalidHTMLTemplateSyntax(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)

	_, err := svc.CreateTemplate(context.Background(), "broken", "", "<html>{{.Unterminated</html>")

	require.ErrorIs(t, err, apperr.ErrTemplateInvalid)
}

func TestTemplateService_UpdateTemplate_RejectsInvalidHTMLTemplateSyntax(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)
	tmpl, err := svc.CreateTemplate(context.Background(), "invoice", "", "<html></html>")
	require.NoError(t, err)

	badHTML := "<html>{{.Unterminated</html>"
	_, err = svc.UpdateTemplate(context.Background(), tmpl.ID, nil, nil, &badHTML)

	require.ErrorIs(t, err, apperr.ErrTemplateInvalid)
}

func TestTemplateService_DeleteTemplate_RemovesIt(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)
	tmpl, err := svc.CreateTemplate(context.Background(), "invoice", "", "<html></html>")
	require.NoError(t, err)

	require.NoError(t, svc.DeleteTemplate(context.Background(), tmpl.ID))

	assert.Equal(t, tmpl.ID, repo.deletedID)
}

func TestTemplateService_GetTemplate_NotFoundPropagatesDomainError(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)

	_, err := svc.GetTemplate(context.Background(), primitive.NewObjectID())

	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/... -run TestTemplateService -v`
Expected: build failure — `NewTemplateService` undefined.

- [ ] **Step 3: Implement**

```go
// internal/service/template_service.go
package service

import (
	"context"
	"fmt"
	"html/template"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type templateRepository interface {
	Create(ctx context.Context, t *model.Template) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error)
	List(ctx context.Context, limit, skip int64) ([]*model.Template, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type TemplateService struct {
	templates templateRepository
}

func NewTemplateService(templates templateRepository) *TemplateService {
	return &TemplateService{templates: templates}
}

func validateTemplateHTML(html string) error {
	if _, err := template.New("validate").Parse(html); err != nil {
		return fmt.Errorf("%w: %v", apperr.ErrTemplateInvalid, err)
	}
	return nil
}

func (s *TemplateService) CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error) {
	if err := validateTemplateHTML(htmlContent); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	t := &model.Template{
		ID:          primitive.NewObjectID(),
		Name:        name,
		Description: description,
		HTMLContent: htmlContent,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.templates.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TemplateService) ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if skip < 0 {
		skip = 0
	}
	return s.templates.List(ctx, limit, skip)
}

func (s *TemplateService) GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	return s.templates.FindByID(ctx, id)
}

func (s *TemplateService) UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error) {
	update := bson.M{}
	if name != nil {
		update["name"] = *name
	}
	if description != nil {
		update["description"] = *description
	}
	if htmlContent != nil {
		if err := validateTemplateHTML(*htmlContent); err != nil {
			return nil, err
		}
		update["html_content"] = *htmlContent
	}
	if len(update) > 0 {
		if err := s.templates.Update(ctx, id, update); err != nil {
			return nil, err
		}
	}
	return s.templates.FindByID(ctx, id)
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	return s.templates.Delete(ctx, id)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/... -run TestTemplateService -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/template_service.go internal/service/template_service_test.go
git commit -m "feat: add TemplateService with save-time HTML validation"
```

---

## Task 11: ReportService — CRUD

**Files:**
- Create: `internal/service/report_service.go`
- Test: `internal/service/report_service_test.go`

**Interfaces:**
- Consumes: `repository.ReportDefinitionRepository`'s method set (Task 5) via a locally-declared `reportDefinitionRepository` interface; `TemplateService`'s `GetTemplate` (Task 10) via a locally-declared `templateLookup` interface (`FindByID(ctx, primitive.ObjectID) (*model.Template, error)` — satisfied directly by `*repository.TemplateRepository` too, so `cmd/api/main.go` in Task 16 can wire either); `reportquery.BuildPipeline` (Task 8).
- Produces (this task): `service.NewReportService(reports reportDefinitionRepository, templates templateLookup, data reportDataRunner, renderer pdf.Renderer) *ReportService` (the last two params are threaded through now but only used starting Task 12) with `CreateReport`, `ListReports`, `GetReport`, `UpdateReport`, `DeleteReport`. `reportDataRunner` and `pdf.Renderer` are declared/imported here because the struct needs the fields, but their methods are exercised by Task 12's tests, not this task's.

- [ ] **Step 1: Write the failing test**

```go
// internal/service/report_service_test.go
package service

import (
	"context"
	"testing"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeReportRepo struct {
	byID      map[primitive.ObjectID]*model.ReportDefinition
	deletedID primitive.ObjectID
}

func newFakeReportRepo() *fakeReportRepo {
	return &fakeReportRepo{byID: map[primitive.ObjectID]*model.ReportDefinition{}}
}

func (f *fakeReportRepo) Create(ctx context.Context, r *model.ReportDefinition) error {
	f.byID[r.ID] = r
	return nil
}
func (f *fakeReportRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	r, ok := f.byID[id]
	if !ok {
		return nil, apperr.ErrReportNotFound
	}
	return r, nil
}
func (f *fakeReportRepo) List(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	var out []*model.ReportDefinition
	for _, r := range f.byID {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeReportRepo) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	r, ok := f.byID[id]
	if !ok {
		return apperr.ErrReportNotFound
	}
	if name, ok := update["name"].(string); ok {
		r.Name = name
	}
	if collection, ok := update["collection"].(string); ok {
		r.Collection = collection
	}
	if pt, ok := update["pipeline_template"].(string); ok {
		r.PipelineTemplate = pt
	}
	if ps, ok := update["param_schema"].([]model.ReportParam); ok {
		r.ParamSchema = ps
	}
	return nil
}
func (f *fakeReportRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.ErrReportNotFound
	}
	f.deletedID = id
	delete(f.byID, id)
	return nil
}

type fakeTemplateLookup struct {
	byID map[primitive.ObjectID]*model.Template
}

func newFakeTemplateLookup(templates ...*model.Template) *fakeTemplateLookup {
	l := &fakeTemplateLookup{byID: map[primitive.ObjectID]*model.Template{}}
	for _, t := range templates {
		l.byID[t.ID] = t
	}
	return l
}

func (f *fakeTemplateLookup) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, apperr.ErrTemplateNotFound
	}
	return t, nil
}

func newTestReportService(reports *fakeReportRepo, templates *fakeTemplateLookup) *ReportService {
	return NewReportService(reports, templates, nil, nil)
}

func validTemplate() *model.Template {
	return &model.Template{ID: primitive.NewObjectID(), Name: "t", HTMLContent: "<html></html>"}
}

func TestReportService_CreateReport_Succeeds(t *testing.T) {
	tmpl := validTemplate()
	svc := newTestReportService(newFakeReportRepo(), newFakeTemplateLookup(tmpl))
	schema := []model.ReportParam{{Name: "status", Type: "string", Required: true}}

	def, err := svc.CreateReport(context.Background(), "orders-by-status", tmpl.ID, "orders",
		`[{"$match": {"status": {{json .status}}}}]`, schema)

	require.NoError(t, err)
	assert.Equal(t, "orders-by-status", def.Name)
}

func TestReportService_CreateReport_FailsWhenTemplateDoesNotExist(t *testing.T) {
	svc := newTestReportService(newFakeReportRepo(), newFakeTemplateLookup())

	_, err := svc.CreateReport(context.Background(), "r", primitive.NewObjectID(), "orders",
		`[{"$match": {}}]`, nil)

	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestReportService_CreateReport_FailsWhenPipelineTemplateIsInvalid(t *testing.T) {
	tmpl := validTemplate()
	svc := newTestReportService(newFakeReportRepo(), newFakeTemplateLookup(tmpl))

	_, err := svc.CreateReport(context.Background(), "r", tmpl.ID, "orders", `[{"$match": {{.broken}`, nil)

	require.ErrorIs(t, err, apperr.ErrPipelineInvalid)
}

func TestReportService_UpdateReport_RevalidatesPipelineAgainstEffectiveSchema(t *testing.T) {
	tmpl := validTemplate()
	reports := newFakeReportRepo()
	svc := newTestReportService(reports, newFakeTemplateLookup(tmpl))
	schema := []model.ReportParam{{Name: "status", Type: "string", Required: true}}
	def, err := svc.CreateReport(context.Background(), "r", tmpl.ID, "orders",
		`[{"$match": {"status": {{json .status}}}}]`, schema)
	require.NoError(t, err)

	newPipeline := `[{"$match": {"status": {{json .status}}, "extra": {{json .missing}}}}]`
	_, err = svc.UpdateReport(context.Background(), def.ID, nil, nil, &newPipeline, nil)

	require.NoError(t, err, "a dummy-value map missing an undeclared key .missing passes the zero interface{} (nil) to the json func, which marshals to the JSON literal null — still valid JSON, so this is not expected to fail validation")
}

func TestReportService_DeleteReport_RemovesIt(t *testing.T) {
	tmpl := validTemplate()
	reports := newFakeReportRepo()
	svc := newTestReportService(reports, newFakeTemplateLookup(tmpl))
	def, err := svc.CreateReport(context.Background(), "r", tmpl.ID, "orders", `[{"$match": {}}]`, nil)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteReport(context.Background(), def.ID))

	assert.Equal(t, def.ID, reports.deletedID)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/... -run TestReportService -v`
Expected: build failure — `NewReportService`, `ReportService` undefined.

- [ ] **Step 3: Implement**

```go
// internal/service/report_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"
	"bom-zustand-api/internal/pdf"
	"bom-zustand-api/internal/reportquery"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type reportDefinitionRepository interface {
	Create(ctx context.Context, r *model.ReportDefinition) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error)
	List(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type templateLookup interface {
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error)
}

type reportDataRunner interface {
	Run(ctx context.Context, collection string, pipeline bson.A) ([]bson.M, error)
}

type ReportService struct {
	reports   reportDefinitionRepository
	templates templateLookup
	data      reportDataRunner
	renderer  pdf.Renderer
}

func NewReportService(reports reportDefinitionRepository, templates templateLookup, data reportDataRunner, renderer pdf.Renderer) *ReportService {
	return &ReportService{reports: reports, templates: templates, data: data, renderer: renderer}
}

func dummyValueForType(t string) interface{} {
	switch t {
	case "number":
		return float64(0)
	case "bool":
		return false
	case "date":
		return time.Now().UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

func validatePipelineTemplate(pipelineTemplate string, schema []model.ReportParam) error {
	dummyParams := make(map[string]interface{}, len(schema))
	for _, p := range schema {
		dummyParams[p.Name] = dummyValueForType(p.Type)
	}
	if _, err := reportquery.BuildPipeline(pipelineTemplate, dummyParams); err != nil {
		return fmt.Errorf("%w: %v", apperr.ErrPipelineInvalid, err)
	}
	return nil
}

func (s *ReportService) CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	if _, err := s.templates.FindByID(ctx, templateID); err != nil {
		return nil, err
	}
	if err := validatePipelineTemplate(pipelineTemplate, paramSchema); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	def := &model.ReportDefinition{
		ID:               primitive.NewObjectID(),
		Name:             name,
		TemplateID:       templateID,
		Collection:       collection,
		PipelineTemplate: pipelineTemplate,
		ParamSchema:      paramSchema,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.reports.Create(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

func (s *ReportService) ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if skip < 0 {
		skip = 0
	}
	return s.reports.List(ctx, limit, skip)
}

func (s *ReportService) GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	return s.reports.FindByID(ctx, id)
}

func (s *ReportService) UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	existing, err := s.reports.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	update := bson.M{}
	if name != nil {
		update["name"] = *name
	}
	if collection != nil {
		update["collection"] = *collection
	}
	effectiveSchema := existing.ParamSchema
	if paramSchema != nil {
		effectiveSchema = paramSchema
		update["param_schema"] = paramSchema
	}
	if pipelineTemplate != nil {
		if err := validatePipelineTemplate(*pipelineTemplate, effectiveSchema); err != nil {
			return nil, err
		}
		update["pipeline_template"] = *pipelineTemplate
	}
	if len(update) > 0 {
		if err := s.reports.Update(ctx, id, update); err != nil {
			return nil, err
		}
	}
	return s.reports.FindByID(ctx, id)
}

func (s *ReportService) DeleteReport(ctx context.Context, id primitive.ObjectID) error {
	return s.reports.Delete(ctx, id)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/... -run TestReportService -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "feat: add ReportService CRUD with save-time pipeline validation"
```

---

## Task 12: ReportService — preview and PDF generation

**Files:**
- Modify: `internal/service/report_service.go`
- Modify: `internal/service/report_service_test.go`

**Interfaces:**
- Consumes: `reportquery.ValidateParams` (Task 7), `reportquery.BuildPipeline` (Task 8), `s.data.Run` and `s.renderer.RenderHTML` (both already fields on `ReportService` from Task 11).
- Produces: `(*ReportService).PreviewReport(ctx, reportID primitive.ObjectID, params map[string]interface{}) (string, error)` and `(*ReportService).GenerateReportPDF(ctx, reportID primitive.ObjectID, params map[string]interface{}) ([]byte, error)`. Consumed by `ReportHandler` (Task 14).

- [ ] **Step 1: Write the failing test**

Append to `internal/service/report_service_test.go`:

```go
type fakeDataRunner struct {
	rows         []bson.M
	lastCollName string
	lastPipeline bson.A
}

func (f *fakeDataRunner) Run(ctx context.Context, collection string, pipeline bson.A) ([]bson.M, error) {
	f.lastCollName, f.lastPipeline = collection, pipeline
	return f.rows, nil
}

type fakeRenderer struct {
	lastHTML string
	pdfBytes []byte
	err      error
}

func (f *fakeRenderer) RenderHTML(ctx context.Context, html string) ([]byte, error) {
	f.lastHTML = html
	if f.err != nil {
		return nil, f.err
	}
	return f.pdfBytes, nil
}

func newReportServiceWithRenderPath(reports *fakeReportRepo, templates *fakeTemplateLookup, data *fakeDataRunner, renderer *fakeRenderer) *ReportService {
	return NewReportService(reports, templates, data, renderer)
}

func setUpOrdersByStatusReport(t *testing.T) (*ReportService, *model.ReportDefinition, *fakeDataRunner, *fakeRenderer) {
	t.Helper()
	tmpl := &model.Template{
		ID:          primitive.NewObjectID(),
		Name:        "orders-report",
		HTMLContent: `<html><body><p>{{range .Rows}}{{.status}}: {{.total}}{{end}}</p></body></html>`,
	}
	reports := newFakeReportRepo()
	data := &fakeDataRunner{rows: []bson.M{{"status": "paid", "total": int32(30)}}}
	renderer := &fakeRenderer{pdfBytes: []byte("%PDF-fake")}
	svc := newReportServiceWithRenderPath(reports, newFakeTemplateLookup(tmpl), data, renderer)
	schema := []model.ReportParam{{Name: "status", Type: "string", Required: true}}
	def, err := svc.CreateReport(context.Background(), "orders-by-status", tmpl.ID, "orders",
		`[{"$match": {"status": {{json .status}}}}]`, schema)
	require.NoError(t, err)
	return svc, def, data, renderer
}

func TestReportService_PreviewReport_RendersTemplateWithQueryResults(t *testing.T) {
	svc, def, data, _ := setUpOrdersByStatusReport(t)

	html, err := svc.PreviewReport(context.Background(), def.ID, map[string]interface{}{"status": "paid"})

	require.NoError(t, err)
	assert.Contains(t, html, "paid: 30")
	assert.Equal(t, "orders", data.lastCollName)
}

func TestReportService_PreviewReport_FailsWhenRequiredParamMissing(t *testing.T) {
	svc, def, _, _ := setUpOrdersByStatusReport(t)

	_, err := svc.PreviewReport(context.Background(), def.ID, map[string]interface{}{})

	require.ErrorIs(t, err, apperr.ErrInvalidReportParams)
}

func TestReportService_PreviewReport_FailsWhenReportNotFound(t *testing.T) {
	svc := newTestReportService(newFakeReportRepo(), newFakeTemplateLookup())

	_, err := svc.PreviewReport(context.Background(), primitive.NewObjectID(), map[string]interface{}{})

	require.ErrorIs(t, err, apperr.ErrReportNotFound)
}

func TestReportService_GenerateReportPDF_RendersHTMLThenPipesThroughRenderer(t *testing.T) {
	svc, def, _, renderer := setUpOrdersByStatusReport(t)

	pdfBytes, err := svc.GenerateReportPDF(context.Background(), def.ID, map[string]interface{}{"status": "paid"})

	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF-fake"), pdfBytes)
	assert.Contains(t, renderer.lastHTML, "paid: 30")
}

func TestReportService_GenerateReportPDF_PropagatesRendererError(t *testing.T) {
	svc, def, _, renderer := setUpOrdersByStatusReport(t)
	renderer.err = assert.AnError

	_, err := svc.GenerateReportPDF(context.Background(), def.ID, map[string]interface{}{"status": "paid"})

	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/... -run "TestReportService_PreviewReport|TestReportService_GenerateReportPDF" -v`
Expected: build failure — `PreviewReport`/`GenerateReportPDF` undefined.

- [ ] **Step 3: Implement**

Append to `internal/service/report_service.go` (add `"bytes"` and `"html/template"` to the import block):

```go
func (s *ReportService) render(ctx context.Context, reportID primitive.ObjectID, params map[string]interface{}) (string, error) {
	def, err := s.reports.FindByID(ctx, reportID)
	if err != nil {
		return "", err
	}
	if err := reportquery.ValidateParams(def.ParamSchema, params); err != nil {
		return "", fmt.Errorf("%w: %v", apperr.ErrInvalidReportParams, err)
	}
	pipeline, err := reportquery.BuildPipeline(def.PipelineTemplate, params)
	if err != nil {
		return "", fmt.Errorf("report %s has an invalid pipeline: %w", def.ID.Hex(), err)
	}

	tpl, err := s.templates.FindByID(ctx, def.TemplateID)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	rows, err := s.data.Run(runCtx, def.Collection, pipeline)
	if err != nil {
		return "", fmt.Errorf("run report aggregation: %w", err)
	}

	htmlTpl, err := template.New("report").Parse(tpl.HTMLContent)
	if err != nil {
		return "", fmt.Errorf("report %s has an invalid template: %w", def.ID.Hex(), err)
	}

	var buf bytes.Buffer
	data := struct {
		Rows        []bson.M
		Params      map[string]interface{}
		GeneratedAt time.Time
	}{Rows: rows, Params: params, GeneratedAt: time.Now().UTC()}
	if err := htmlTpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render report template: %w", err)
	}
	return buf.String(), nil
}

func (s *ReportService) PreviewReport(ctx context.Context, reportID primitive.ObjectID, params map[string]interface{}) (string, error) {
	return s.render(ctx, reportID, params)
}

func (s *ReportService) GenerateReportPDF(ctx context.Context, reportID primitive.ObjectID, params map[string]interface{}) ([]byte, error) {
	html, err := s.render(ctx, reportID, params)
	if err != nil {
		return nil, err
	}
	return s.renderer.RenderHTML(ctx, html)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/... -v`
Expected: PASS (full `service` package, since this task's tests share the package with Task 10/11's)

- [ ] **Step 5: Commit**

```bash
git add internal/service/report_service.go internal/service/report_service_test.go
git commit -m "feat: add ReportService preview and PDF generation"
```

---

## Task 13: TemplateHandler

**Files:**
- Create: `internal/handler/template_handler.go`
- Test: `internal/handler/template_handler_test.go`

**Interfaces:**
- Consumes: `TemplateService`'s method set (Task 10) via a locally-declared `templateServicer` interface; reuses `paramObjectID` (already defined in `internal/handler/user_handler.go`) and `newTestEcho` (already defined in `internal/handler/auth_handler_test.go`).
- Produces: `handler.NewTemplateHandler(svc templateServicer) *TemplateHandler` with `Create`, `List`, `Get`, `Update`, `Delete` — wired into routes in Task 15.

- [ ] **Step 1: Write the failing test**

```go
// internal/handler/template_handler_test.go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeTemplateServicer struct {
	created   *model.Template
	createErr error
	templates []*model.Template
	got       *model.Template
	getErr    error
}

func (f *fakeTemplateServicer) CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error) {
	return f.created, f.createErr
}
func (f *fakeTemplateServicer) ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	return f.templates, nil
}
func (f *fakeTemplateServicer) GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	return f.got, f.getErr
}
func (f *fakeTemplateServicer) UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error) {
	return f.got, f.getErr
}
func (f *fakeTemplateServicer) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	return f.getErr
}

func TestTemplateHandler_Create_Success(t *testing.T) {
	e := newTestEcho()
	svc := &fakeTemplateServicer{created: &model.Template{Name: "invoice"}}
	h := NewTemplateHandler(svc)
	body := strings.NewReader(`{"name":"invoice","description":"d","htmlContent":"<html></html>"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestTemplateHandler_Create_MissingHTMLContentFailsValidation(t *testing.T) {
	e := newTestEcho()
	h := NewTemplateHandler(&fakeTemplateServicer{})
	body := strings.NewReader(`{"name":"invoice"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.Error(t, err)
}

func TestTemplateHandler_Get_NotFoundPropagatesDomainError(t *testing.T) {
	e := newTestEcho()
	svc := &fakeTemplateServicer{getErr: apperr.ErrTemplateNotFound}
	h := NewTemplateHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Get(c)

	assert.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestTemplateHandler_Delete_Success(t *testing.T) {
	e := newTestEcho()
	h := NewTemplateHandler(&fakeTemplateServicer{})
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/... -run TestTemplateHandler -v`
Expected: build failure — `NewTemplateHandler` undefined.

- [ ] **Step 3: Implement**

```go
// internal/handler/template_handler.go
package handler

import (
	"context"
	"net/http"
	"strconv"

	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type templateServicer interface {
	CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error)
	ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error)
	GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error)
	UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error)
	DeleteTemplate(ctx context.Context, id primitive.ObjectID) error
}

type TemplateHandler struct {
	service templateServicer
}

func NewTemplateHandler(svc templateServicer) *TemplateHandler {
	return &TemplateHandler{service: svc}
}

type createTemplateRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	HTMLContent string `json:"htmlContent" validate:"required"`
}

// Create godoc
//
//	@Summary		Create a template
//	@Description	Creates an HTML report template. Requires the template:create permission.
//	@Tags			templates
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		createTemplateRequest	true	"New template"
//	@Success		201		{object}	model.Template
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		409		{object}	handler.errorResponse
//	@Router			/templates [post]
func (h *TemplateHandler) Create(c *echo.Context) error {
	var req createTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	t, err := h.service.CreateTemplate(c.Request().Context(), req.Name, req.Description, req.HTMLContent)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, t)
}

// List godoc
//
//	@Summary		List templates
//	@Description	Lists templates with pagination. Requires the template:read permission.
//	@Tags			templates
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query		int	false	"Max number of templates to return"
//	@Param			skip	query		int	false	"Number of templates to skip"
//	@Success		200		{array}		model.Template
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Router			/templates [get]
func (h *TemplateHandler) List(c *echo.Context) error {
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	skip, _ := strconv.ParseInt(c.QueryParam("skip"), 10, 64)

	templates, err := h.service.ListTemplates(c.Request().Context(), limit, skip)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, templates)
}

// Get godoc
//
//	@Summary		Get a template
//	@Description	Returns a single template, including its raw HTML content. Requires the template:read permission.
//	@Tags			templates
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Template ID"
//	@Success		200	{object}	model.Template
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/templates/{id} [get]
func (h *TemplateHandler) Get(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	t, err := h.service.GetTemplate(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, t)
}

type updateTemplateRequest struct {
	Name        *string `json:"name" validate:"omitempty"`
	Description *string `json:"description" validate:"omitempty"`
	HTMLContent *string `json:"htmlContent" validate:"omitempty"`
}

// Update godoc
//
//	@Summary		Update a template
//	@Description	Partially updates a template's name, description, and/or HTML content. Requires the template:update permission.
//	@Tags			templates
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string					true	"Template ID"
//	@Param			body	body		updateTemplateRequest	true	"Fields to update"
//	@Success		200		{object}	model.Template
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		404		{object}	handler.errorResponse
//	@Router			/templates/{id} [patch]
func (h *TemplateHandler) Update(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	var req updateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	t, err := h.service.UpdateTemplate(c.Request().Context(), id, req.Name, req.Description, req.HTMLContent)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, t)
}

// Delete godoc
//
//	@Summary		Delete a template
//	@Description	Deletes a template. Requires the template:delete permission.
//	@Tags			templates
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Template ID"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/templates/{id} [delete]
func (h *TemplateHandler) Delete(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	if err := h.service.DeleteTemplate(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/... -run TestTemplateHandler -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/template_handler.go internal/handler/template_handler_test.go
git commit -m "feat: add TemplateHandler CRUD endpoints"
```

---

## Task 14: ReportHandler

**Files:**
- Create: `internal/handler/report_handler.go`
- Test: `internal/handler/report_handler_test.go`

**Interfaces:**
- Consumes: `ReportService`'s method set (Tasks 11-12) via a locally-declared `reportServicer` interface.
- Produces: `handler.NewReportHandler(svc reportServicer) *ReportHandler` with `Create`, `List`, `Get`, `Update`, `Delete`, `Preview`, `Generate` — wired into routes in Task 15.

- [ ] **Step 1: Write the failing test**

```go
// internal/handler/report_handler_test.go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeReportServicer struct {
	created      *model.ReportDefinition
	createErr    error
	got          *model.ReportDefinition
	getErr       error
	previewHTML  string
	previewErr   error
	generatedPDF []byte
	generateErr  error
}

func (f *fakeReportServicer) CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return f.created, f.createErr
}
func (f *fakeReportServicer) ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	return nil, nil
}
func (f *fakeReportServicer) GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	return f.got, f.getErr
}
func (f *fakeReportServicer) UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return f.got, f.getErr
}
func (f *fakeReportServicer) DeleteReport(ctx context.Context, id primitive.ObjectID) error {
	return f.getErr
}
func (f *fakeReportServicer) PreviewReport(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) (string, error) {
	return f.previewHTML, f.previewErr
}
func (f *fakeReportServicer) GenerateReportPDF(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) ([]byte, error) {
	return f.generatedPDF, f.generateErr
}

func TestReportHandler_Create_Success(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{created: &model.ReportDefinition{Name: "orders-by-status"}}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"name":"orders-by-status","templateId":"` + primitive.NewObjectID().Hex() +
		`","collection":"orders","pipelineTemplate":"[{\"$match\":{}}]"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestReportHandler_Create_InvalidTemplateIdFails(t *testing.T) {
	e := newTestEcho()
	h := NewReportHandler(&fakeReportServicer{})
	body := strings.NewReader(`{"name":"r","templateId":"not-an-objectid","collection":"orders","pipelineTemplate":"[]"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.Error(t, err)
}

func TestReportHandler_Preview_ReturnsHTML(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{previewHTML: "<html>ok</html>"}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"params":{"status":"paid"}}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Preview(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html>ok</html>")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestReportHandler_Generate_ReturnsPDFBytes(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{generatedPDF: []byte("%PDF-fake")}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"params":{"status":"paid"}}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Generate(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, []byte("%PDF-fake"), rec.Body.Bytes())
}

func TestReportHandler_Generate_InvalidParamsPropagatesDomainError(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{generateErr: apperr.ErrInvalidReportParams}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Generate(c)

	assert.ErrorIs(t, err, apperr.ErrInvalidReportParams)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/... -run TestReportHandler -v`
Expected: build failure — `NewReportHandler` undefined.

- [ ] **Step 3: Implement**

```go
// internal/handler/report_handler.go
package handler

import (
	"context"
	"net/http"
	"strconv"

	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type reportServicer interface {
	CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error)
	ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error)
	GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error)
	UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error)
	DeleteReport(ctx context.Context, id primitive.ObjectID) error
	PreviewReport(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) (string, error)
	GenerateReportPDF(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) ([]byte, error)
}

type ReportHandler struct {
	service reportServicer
}

func NewReportHandler(svc reportServicer) *ReportHandler {
	return &ReportHandler{service: svc}
}

type createReportRequest struct {
	Name             string              `json:"name" validate:"required"`
	TemplateID       string              `json:"templateId" validate:"required"`
	Collection       string              `json:"collection" validate:"required"`
	PipelineTemplate string              `json:"pipelineTemplate" validate:"required"`
	ParamSchema      []model.ReportParam `json:"paramSchema"`
}

// Create godoc
//
//	@Summary		Create a report definition
//	@Description	Pairs a template with a MongoDB aggregation pipeline and a param schema. Requires the report:create permission. Pipeline placeholders use {{json .paramName}}; wrap date params needing BSON Date comparison as {"$date": {{json .paramName}}}.
//	@Tags			reports
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		createReportRequest	true	"New report definition"
//	@Success		201		{object}	model.ReportDefinition
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		404		{object}	handler.errorResponse
//	@Failure		409		{object}	handler.errorResponse
//	@Router			/reports [post]
func (h *ReportHandler) Create(c *echo.Context) error {
	var req createReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	templateID, err := primitive.ObjectIDFromHex(req.TemplateID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid templateId")
	}

	def, err := h.service.CreateReport(c.Request().Context(), req.Name, templateID, req.Collection, req.PipelineTemplate, req.ParamSchema)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, def)
}

// List godoc
//
//	@Summary		List report definitions
//	@Description	Lists report definitions with pagination, including each one's param schema. Requires the report:read permission.
//	@Tags			reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query		int	false	"Max number of report definitions to return"
//	@Param			skip	query		int	false	"Number of report definitions to skip"
//	@Success		200		{array}		model.ReportDefinition
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Router			/reports [get]
func (h *ReportHandler) List(c *echo.Context) error {
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	skip, _ := strconv.ParseInt(c.QueryParam("skip"), 10, 64)

	reports, err := h.service.ListReports(c.Request().Context(), limit, skip)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, reports)
}

// Get godoc
//
//	@Summary		Get a report definition
//	@Description	Returns a report definition's param schema and template reference — this is the primary lookup the frontend uses to build a generation form. Requires the report:read permission.
//	@Tags			reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Report definition ID"
//	@Success		200	{object}	model.ReportDefinition
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id} [get]
func (h *ReportHandler) Get(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	def, err := h.service.GetReport(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, def)
}

type updateReportRequest struct {
	Name             *string             `json:"name" validate:"omitempty"`
	Collection       *string             `json:"collection" validate:"omitempty"`
	PipelineTemplate *string             `json:"pipelineTemplate" validate:"omitempty"`
	ParamSchema      []model.ReportParam `json:"paramSchema"`
}

// Update godoc
//
//	@Summary		Update a report definition
//	@Description	Partially updates a report definition's name, collection, pipeline, and/or param schema. Requires the report:update permission.
//	@Tags			reports
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"Report definition ID"
//	@Param			body	body		updateReportRequest	true	"Fields to update"
//	@Success		200		{object}	model.ReportDefinition
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		404		{object}	handler.errorResponse
//	@Router			/reports/{id} [patch]
func (h *ReportHandler) Update(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	var req updateReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	def, err := h.service.UpdateReport(c.Request().Context(), id, req.Name, req.Collection, req.PipelineTemplate, req.ParamSchema)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, def)
}

// Delete godoc
//
//	@Summary		Delete a report definition
//	@Description	Deletes a report definition. Requires the report:delete permission.
//	@Tags			reports
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Report definition ID"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id} [delete]
func (h *ReportHandler) Delete(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	if err := h.service.DeleteReport(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type generateReportRequest struct {
	Params map[string]interface{} `json:"params"`
}

// Preview godoc
//
//	@Summary		Preview a report as HTML
//	@Description	Runs the report's query and renders its template, returning raw HTML (no PDF conversion) — used by the frontend to show a live preview before generating. Requires the report:generate permission.
//	@Tags			reports
//	@Accept			json
//	@Produce		html
//	@Security		BearerAuth
//	@Param			id		path	string					true	"Report definition ID"
//	@Param			body	body	generateReportRequest	true	"Report params"
//	@Success		200
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id}/preview [post]
func (h *ReportHandler) Preview(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	var req generateReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	html, err := h.service.PreviewReport(c.Request().Context(), id, req.Params)
	if err != nil {
		return err
	}
	return c.HTML(http.StatusOK, html)
}

// Generate godoc
//
//	@Summary		Generate a report PDF
//	@Description	Runs the report's query, renders its template, and converts the result to PDF via headless Chrome. Requires the report:generate permission.
//	@Tags			reports
//	@Accept			json
//	@Produce		application/pdf
//	@Security		BearerAuth
//	@Param			id		path	string					true	"Report definition ID"
//	@Param			body	body	generateReportRequest	true	"Report params"
//	@Success		200
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id}/generate [post]
func (h *ReportHandler) Generate(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	var req generateReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	pdfBytes, err := h.service.GenerateReportPDF(c.Request().Context(), id, req.Params)
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/... -v`
Expected: PASS (full `handler` package)

- [ ] **Step 5: Commit**

```bash
git add internal/handler/report_handler.go internal/handler/report_handler_test.go
git commit -m "feat: add ReportHandler CRUD, preview, and generate endpoints"
```

---

## Task 15: Wire routes and permissions

**Files:**
- Modify: `internal/router/router.go`
- Modify: `internal/router/router_test.go`
- Modify: `internal/router/stubs_test.go`

**Interfaces:**
- Consumes: `handler.NewTemplateHandler`/`handler.TemplateHandler` (Task 13), `handler.NewReportHandler`/`handler.ReportHandler` (Task 14), `auth.PermTemplate*`/`auth.PermReport*` (Task 2).
- Produces: `router.New` now takes two more params (`*handler.TemplateHandler`, `*handler.ReportHandler`) and registers `/api/v1/templates*` and `/api/v1/reports*` routes behind JWT auth + the matching permission. `cmd/api/main.go` (Task 16) passes the new handlers in.

- [ ] **Step 1: Write the failing test**

Append to `internal/router/stubs_test.go`:

```go
type stubTemplateServicer struct{}

func (s *stubTemplateServicer) CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error) {
	return &model.Template{}, nil
}
func (s *stubTemplateServicer) ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	return nil, nil
}
func (s *stubTemplateServicer) GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	return &model.Template{}, nil
}
func (s *stubTemplateServicer) UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error) {
	return &model.Template{}, nil
}
func (s *stubTemplateServicer) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	return nil
}

type stubReportServicer struct{}

func (s *stubReportServicer) CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return &model.ReportDefinition{}, nil
}
func (s *stubReportServicer) ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	return nil, nil
}
func (s *stubReportServicer) GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	return &model.ReportDefinition{}, nil
}
func (s *stubReportServicer) UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return &model.ReportDefinition{}, nil
}
func (s *stubReportServicer) DeleteReport(ctx context.Context, id primitive.ObjectID) error {
	return nil
}
func (s *stubReportServicer) PreviewReport(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) (string, error) {
	return "<html></html>", nil
}
func (s *stubReportServicer) GenerateReportPDF(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) ([]byte, error) {
	return []byte("%PDF-fake"), nil
}
```

Append to `internal/router/router_test.go` (and update the two existing `New(...)` calls — see Step 3):

```go
func TestRouter_CreateTemplate_WithStaffTokenReturns403(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	token, err := auth.GenerateAccessToken("user-1", "staff", "secret", time.Minute)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRouter_GenerateReport_WithStaffTokenReturns200(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	token, err := auth.GenerateAccessToken("user-1", "staff", "secret", time.Minute)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/"+primitive.NewObjectID().Hex()+"/generate", strings.NewReader(`{"params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
```

Add `"go.mongodb.org/mongo-driver/bson/primitive"` to `router_test.go`'s imports for the second test.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/router/... -v`
Expected: build failure — `New` called with the wrong number of arguments; `stubTemplateServicer`/`stubReportServicer` wired against `handler.NewTemplateHandler`/`handler.NewReportHandler` which don't accept them yet (they do, from Task 13/14 — the failure here is purely the `router.New` arity mismatch).

- [ ] **Step 3: Update router.New and the two existing calls in router_test.go**

```go
// internal/router/router.go
package router

import (
	"bom-zustand-api/internal/auth"
	"bom-zustand-api/internal/handler"
	"bom-zustand-api/internal/httpvalidator"
	appmiddleware "bom-zustand-api/internal/middleware"

	"github.com/labstack/echo/v5"
)

func New(jwtSecret string, authHandler *handler.AuthHandler, userHandler *handler.UserHandler, templateHandler *handler.TemplateHandler, reportHandler *handler.ReportHandler) *echo.Echo {
	e := echo.New()
	e.Validator = httpvalidator.New()
	e.HTTPErrorHandler = appmiddleware.ErrorHandler

	e.GET("/healthz", handler.Healthz)
	e.GET("/swagger/*", handler.SwaggerUI)

	authGroup := e.Group("/api/v1/auth")
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/forgot-password", authHandler.ForgotPassword)
	authGroup.POST("/reset-password", authHandler.ResetPassword)

	jwtAuth := appmiddleware.JWTAuth(jwtSecret)
	authGroup.GET("/me", authHandler.Me, jwtAuth)
	authGroup.POST("/change-password", authHandler.ChangePassword, jwtAuth)
	authGroup.POST("/logout", authHandler.Logout, jwtAuth)
	authGroup.POST("/logout-all", authHandler.LogoutAll, jwtAuth)

	usersGroup := e.Group("/api/v1/users", jwtAuth)
	usersGroup.POST("", userHandler.Create, appmiddleware.RequirePermission(auth.PermUserCreate))
	usersGroup.GET("", userHandler.List, appmiddleware.RequirePermission(auth.PermUserRead))
	usersGroup.GET("/:id", userHandler.Get, appmiddleware.RequirePermission(auth.PermUserRead))
	usersGroup.PATCH("/:id", userHandler.Update, appmiddleware.RequirePermission(auth.PermUserUpdate))
	usersGroup.DELETE("/:id", userHandler.Delete, appmiddleware.RequirePermission(auth.PermUserDelete))

	templatesGroup := e.Group("/api/v1/templates", jwtAuth)
	templatesGroup.POST("", templateHandler.Create, appmiddleware.RequirePermission(auth.PermTemplateCreate))
	templatesGroup.GET("", templateHandler.List, appmiddleware.RequirePermission(auth.PermTemplateRead))
	templatesGroup.GET("/:id", templateHandler.Get, appmiddleware.RequirePermission(auth.PermTemplateRead))
	templatesGroup.PATCH("/:id", templateHandler.Update, appmiddleware.RequirePermission(auth.PermTemplateUpdate))
	templatesGroup.DELETE("/:id", templateHandler.Delete, appmiddleware.RequirePermission(auth.PermTemplateDelete))

	reportsGroup := e.Group("/api/v1/reports", jwtAuth)
	reportsGroup.POST("", reportHandler.Create, appmiddleware.RequirePermission(auth.PermReportCreate))
	reportsGroup.GET("", reportHandler.List, appmiddleware.RequirePermission(auth.PermReportRead))
	reportsGroup.GET("/:id", reportHandler.Get, appmiddleware.RequirePermission(auth.PermReportRead))
	reportsGroup.PATCH("/:id", reportHandler.Update, appmiddleware.RequirePermission(auth.PermReportUpdate))
	reportsGroup.DELETE("/:id", reportHandler.Delete, appmiddleware.RequirePermission(auth.PermReportDelete))
	reportsGroup.POST("/:id/preview", reportHandler.Preview, appmiddleware.RequirePermission(auth.PermReportGenerate))
	reportsGroup.POST("/:id/generate", reportHandler.Generate, appmiddleware.RequirePermission(auth.PermReportGenerate))

	return e
}
```

Update the four existing `New(...)` calls in `internal/router/router_test.go` (`TestRouter_Healthz_NoAuthRequired`, `TestRouter_CreateUser_WithoutTokenReturns401`, `TestRouter_CreateUser_WithStaffTokenReturns403`, `TestRouter_Me_WithValidTokenReturns200`) to pass the two new args:

```go
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/router/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/router/router.go internal/router/router_test.go internal/router/stubs_test.go
git commit -m "feat: wire template and report routes with RBAC"
```

---

## Task 16: Wire main.go and config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Consumes: everything produced in Tasks 1-15.
- Produces: a fully wired `cmd/api/main.go` that starts the chromedp renderer once at startup and closes it on shutdown; `config.Config.ChromeExecPath` (optional, defaults to `""` meaning "let chromedp auto-detect").

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go` (mirroring its existing optional-env-var test style):

```go
func TestLoad_ChromeExecPathDefaultsToEmpty(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "", cfg.ChromeExecPath)
}

func TestLoad_ChromeExecPathReadsFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CHROME_EXEC_PATH", "/usr/bin/chromium")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/chromium", cfg.ChromeExecPath)
}
```

(If `setRequiredEnv` isn't already the exact helper name in the existing file, use whatever helper `config_test.go` already defines for setting the required env vars — don't introduce a second one.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: build failure — `cfg.ChromeExecPath` undefined.

- [ ] **Step 3: Add the config field**

In `internal/config/config.go`, add to the `Config` struct:

```go
	ChromeExecPath    string
```

And in `Load()`, add alongside the other optional fields:

```go
		ChromeExecPath:    os.Getenv("CHROME_EXEC_PATH"),
```

(`CHROME_EXEC_PATH` is intentionally not added to the required-vars map — empty means "let chromedp auto-detect".)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 5: Wire cmd/api/main.go**

```go
// cmd/api/main.go
package main

import (
	"context"
	"log"
	"time"

	"bom-zustand-api/internal/bootstrap"
	"bom-zustand-api/internal/config"
	"bom-zustand-api/internal/db"
	"bom-zustand-api/internal/handler"
	"bom-zustand-api/internal/mailer"
	"bom-zustand-api/internal/pdf"
	"bom-zustand-api/internal/repository"
	"bom-zustand-api/internal/router"
	"bom-zustand-api/internal/service"
)

// @title			bom-zustand-api
// @version		1.0
// @description	HTTP API for bom-zustand-api, built on Echo.
// @BasePath		/api/v1
//
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and the JWT access token.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := db.Connect(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	database := client.Database(cfg.MongoDBName)

	if err := db.EnsureIndexes(ctx, database); err != nil {
		log.Fatalf("ensure indexes: %v", err)
	}

	userRepo := repository.NewUserRepository(database)
	refreshTokenRepo := repository.NewRefreshTokenRepository(database)
	resetTokenRepo := repository.NewPasswordResetTokenRepository(database)
	templateRepo := repository.NewTemplateRepository(database)
	reportRepo := repository.NewReportDefinitionRepository(database)
	aggregationRepo := repository.NewAggregationRepository(database)

	if err := bootstrap.SeedAdmin(ctx, userRepo, cfg.SeedAdminEmail, cfg.SeedAdminPassword); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	adminExists, err := userRepo.ExistsActiveAdmin(ctx)
	if err != nil {
		log.Fatalf("check existing admin: %v", err)
	}
	if !adminExists {
		log.Println("warning: no active admin exists and SEED_ADMIN_EMAIL/SEED_ADMIN_PASSWORD were not both set — /api/v1/users is unreachable until an admin is created")
	}

	mailerClient := mailer.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	pdfRenderer := pdf.NewChromedpRenderer(cfg.ChromeExecPath)
	defer pdfRenderer.Close()

	authService := service.NewAuthService(
		userRepo, refreshTokenRepo, resetTokenRepo, mailerClient,
		cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL, cfg.ResetTokenTTL, cfg.AppBaseURL,
	)
	userService := service.NewUserService(userRepo, resetTokenRepo, mailerClient, cfg.ResetTokenTTL, cfg.AppBaseURL)
	templateService := service.NewTemplateService(templateRepo)
	reportService := service.NewReportService(reportRepo, templateRepo, aggregationRepo, pdfRenderer)

	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	templateHandler := handler.NewTemplateHandler(templateService)
	reportHandler := handler.NewReportHandler(reportService)

	e := router.New(cfg.JWTSecret, authHandler, userHandler, templateHandler, reportHandler)

	// Echo v5's Start blocks the whole request/response/graceful-shutdown
	// lifecycle internally: it installs its own SIGINT/SIGTERM handler and
	// only returns once the HTTP server has finished a graceful shutdown
	// (or failed to start in the first place). There is no separate
	// e.Shutdown to call from application code — unlike Echo v4, no
	// goroutine/signal.Notify/manual-Shutdown dance is needed or possible.
	if err := e.Start(":" + cfg.Port); err != nil {
		log.Printf("server stopped: %v", err)
	}

	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer disconnectCancel()
	if err := client.Disconnect(disconnectCtx); err != nil {
		log.Printf("mongo disconnect: %v", err)
	}
}
```

Note: `templateRepo` (`*repository.TemplateRepository`) is passed directly where `ReportService` wants a `templateLookup` (just `FindByID`) — it satisfies that interface structurally, no adapter needed. Likewise `aggregationRepo` satisfies `reportDataRunner`.

- [ ] **Step 6: Verify the whole module builds**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/api/main.go
git commit -m "feat: wire template/report services and chromedp renderer into main"
```

---

## Task 17: Full verification pass

**Files:** none (verification only)

- [ ] **Step 1: Format check**

Run: `gofmt -l .`
Expected: no output (no files need formatting). If any file is listed, run `gofmt -w .` and re-check.

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no errors.

- [ ] **Step 3: Unit tests (excludes integration-tagged repository/db tests)**

Run: `go test ./...`
Expected: all packages PASS (or `?   ... [no test files]` where applicable). The one gated chromedp smoke test (Task 9) reports PASS or SKIP depending on whether Chrome is on the machine's PATH — either is acceptable here.

- [ ] **Step 4: Integration tests (requires Docker for testcontainers)**

Run: `go test -tags=integration ./...`
Expected: all `repository` and `db` package tests PASS.

- [ ] **Step 5: Build**

Run: `go build -o bin/api ./cmd/api`
Expected: no errors; `bin/api` produced.

- [ ] **Step 6: Commit (only if Steps 1-5 required code changes)**

```bash
git add -A
git commit -m "chore: fix formatting/vet issues found during verification"
```

If nothing needed fixing, skip this commit — there's nothing to commit.
