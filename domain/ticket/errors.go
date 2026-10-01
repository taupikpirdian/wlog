package ticket

import "errors"

var (
	ErrInvalidKey    = errors.New("invalid ticket key")
	ErrAlreadyExists = errors.New("ticket already exists")
	ErrNotFound      = errors.New("ticket not found")
)
