package spec

// A Scope represents the environmental scope available from within a .http file, e.g
// global variables set at the top of the file, builtin functions and identifiers
// as well as local, request-scoped variables.
type Scope struct {
	// Global variables available to the entire file.
	Global map[string]string

	// Local variables, available only to a single request.
	Local map[string]string
}

// NewScope returns a new [Scope].
func NewScope() Scope {
	return Scope{
		Global: make(map[string]string),
		Local:  make(map[string]string),
	}
}
