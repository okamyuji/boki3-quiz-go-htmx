// Package domain holds value objects and pure business logic.
// No package under internal/domain may import other internal packages.
package domain

import "errors"

// Sentinel errors are exposed so service/transport layers can map to HTTP statuses.
var (
	ErrNotFound          = errors.New("domain: not found")
	ErrAlreadyExists     = errors.New("domain: already exists")
	ErrInvalidInput      = errors.New("domain: invalid input")
	ErrUnauthorized      = errors.New("domain: unauthorized")
	ErrForbidden         = errors.New("domain: forbidden")
	ErrPasswordMismatch  = errors.New("domain: password mismatch")
	ErrPasswordTooWeak   = errors.New("domain: password too weak")
	ErrUsernameInvalid   = errors.New("domain: username invalid")
	ErrRateLimited       = errors.New("domain: rate limited")
	ErrSessionExpired    = errors.New("domain: session expired")
	ErrTokenInvalid      = errors.New("domain: token invalid")
	ErrTokenExpired      = errors.New("domain: token expired")
	ErrIntegrityMismatch = errors.New("domain: integrity mismatch")
)
