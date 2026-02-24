package crypto

import "errors"

var (
	// ErrPasswordTooShort is returned when password is too short
	ErrPasswordTooShort = errors.New("password must be at least 8 characters long")

	// ErrPasswordTooLong is returned when password is too long
	ErrPasswordTooLong = errors.New("password must be at most 72 characters long")

	// ErrPasswordMismatch is returned when passwords don't match
	ErrPasswordMismatch = errors.New("passwords do not match")
)
