package core

import "fmt"

type ErrConfigLoad struct {
	Cause error
}

func (e *ErrConfigLoad) Error() string {
	return fmt.Sprintf("config load failed: %v", e.Cause)
}

func (e *ErrConfigLoad) Unwrap() error {
	return e.Cause
}

type ErrResourceInit struct {
	Cause error
}

func (e *ErrResourceInit) Error() string {
	return fmt.Sprintf("resource init failed: %v", e.Cause)
}

func (e *ErrResourceInit) Unwrap() error {
	return e.Cause
}
