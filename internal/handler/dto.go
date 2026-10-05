// internal/handler/dto.go
package handler

// tokenResponse is returned by endpoints that issue a new token pair.
type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// messageResponse is a generic human-readable status message.
type messageResponse struct {
	Message string `json:"message"`
}

// errorResponse documents the shape written by middleware.ErrorHandler.
type errorResponse struct {
	Error string `json:"error"`
}
