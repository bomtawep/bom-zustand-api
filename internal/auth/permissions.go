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
