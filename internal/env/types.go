package env

import "fmt"

// Scope defines where environment variables are stored
type Scope int

const (
	// ScopeUser represents user-level environment variables
	ScopeUser Scope = iota
	// ScopeSystem represents system-level environment variables (requires admin)
	ScopeSystem
)

func (s Scope) String() string {
	switch s {
	case ScopeUser:
		return "User"
	case ScopeSystem:
		return "System"
	default:
		return "Unknown"
	}
}

// Variable represents an environment variable
type Variable struct {
	Name  string
	Value string
	Scope Scope
}

// ValidationError represents an error during variable validation
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error [%s]: %s", e.Field, e.Message)
}

// PathEntry represents a single entry in a PATH-like variable
type PathEntry struct {
	Index int
	Value string
	Exist bool // Whether the path actually exists on disk
}