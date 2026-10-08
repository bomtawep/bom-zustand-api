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

	ErrTemplateNotFound    = errors.New("template not found")
	ErrReportNotFound      = errors.New("report definition not found")
	ErrNameAlreadyExists   = errors.New("name already exists")
	ErrInvalidReportParams = errors.New("invalid report params")
	ErrTemplateInvalid     = errors.New("template content is not valid")
	ErrPipelineInvalid     = errors.New("pipeline template is not valid")
)
