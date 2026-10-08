// internal/auth/permissions_test.go
package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPermission_AdminHasAllUserPermissions(t *testing.T) {
	for _, p := range []Permission{PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete} {
		assert.True(t, HasPermission(RoleAdmin, p), "admin should have %s", p)
	}
}

func TestHasPermission_ManagerOnlyHasRead(t *testing.T) {
	assert.True(t, HasPermission(RoleManager, PermUserRead))
	assert.False(t, HasPermission(RoleManager, PermUserCreate))
	assert.False(t, HasPermission(RoleManager, PermUserUpdate))
	assert.False(t, HasPermission(RoleManager, PermUserDelete))
}

func TestHasPermission_StaffAndViewerHaveNoUserPermissions(t *testing.T) {
	for _, role := range []Role{RoleStaff, RoleViewer} {
		for _, p := range []Permission{PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete} {
			assert.False(t, HasPermission(role, p), "%s should not have %s", role, p)
		}
	}
}

func TestHasPermission_UnknownRoleHasNoPermissions(t *testing.T) {
	assert.False(t, HasPermission(Role("nonexistent"), PermUserRead))
}

func TestPermissionsForRole_ReturnsExpectedPermissionsPerRole(t *testing.T) {
	assert.ElementsMatch(t,
		[]Permission{
			PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete,
			PermTemplateCreate, PermTemplateRead, PermTemplateUpdate, PermTemplateDelete,
			PermReportCreate, PermReportRead, PermReportUpdate, PermReportDelete, PermReportGenerate,
		},
		PermissionsForRole(RoleAdmin),
	)
	assert.ElementsMatch(t, []Permission{PermUserRead, PermTemplateRead, PermReportRead, PermReportGenerate}, PermissionsForRole(RoleManager))
	assert.ElementsMatch(t, []Permission{PermReportRead, PermReportGenerate}, PermissionsForRole(RoleStaff))
	assert.Empty(t, PermissionsForRole(RoleViewer))
}

func TestPermissionsForRole_UnknownRoleReturnsEmpty(t *testing.T) {
	assert.Empty(t, PermissionsForRole(Role("nonexistent")))
}

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
