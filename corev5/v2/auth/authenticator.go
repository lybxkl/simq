package auth

import (
	"errors"
)

var (
	ErrAuthFailure          = errors.New("auth: Authentication failure")
	ErrAuthProviderNotFound = errors.New("auth: Authentication provider not found")
)

type Authenticator interface {
	Authenticate(id string, cred interface{}) error
}
