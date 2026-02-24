package host

import "fmt"

type ErrMigration struct {
	Cause error
}

func (e *ErrMigration) Error() string {
	return fmt.Sprintf("migration failed: %v", e.Cause)
}

func (e *ErrMigration) Unwrap() error {
	return e.Cause
}

type ErrPluginBootstrap struct {
	Cause error
}

func (e *ErrPluginBootstrap) Error() string {
	return fmt.Sprintf("plugin bootstrap failed: %v", e.Cause)
}

func (e *ErrPluginBootstrap) Unwrap() error {
	return e.Cause
}

type ErrModuleBootstrap struct {
	Cause error
}

func (e *ErrModuleBootstrap) Error() string {
	return fmt.Sprintf("module bootstrap failed: %v", e.Cause)
}

func (e *ErrModuleBootstrap) Unwrap() error {
	return e.Cause
}

type ErrRouteConflict struct {
	Method         string
	Path           string
	OwnerModule    string
	ConflictModule string
}

func (e *ErrRouteConflict) Error() string {
	return fmt.Sprintf("route conflict: %s %s (owner=%s conflict=%s)", e.Method, e.Path, e.OwnerModule, e.ConflictModule)
}
