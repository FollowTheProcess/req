package spec

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// A File is a single .http file.
//
// It may be constructed with [ResolveFile] from a [syntax.File].
type File struct {
	// Name of the file (or @name in global scope if given)
	Name string `json:"name,omitempty"`

	// Global variables defined at the top level, e.g. base url
	Vars map[string]string `json:"vars,omitempty"`

	// Global prompts, the user will be asked to provide values for each of these each time the
	// file is parsed.
	//
	// The provided values will then be stored in Vars.
	Prompts []Prompt `json:"prompts,omitempty"`

	// The HTTP requests described in the file
	Requests []Request `json:"requests,omitempty"`

	// Global timeout for all requests
	Timeout time.Duration `json:"timeout,omitempty"`

	// Global connection timeout for all requests
	ConnectionTimeout time.Duration `json:"connectionTimeout,omitempty"`

	// Disable following redirects globally
	NoRedirect bool `json:"noRedirect,omitempty"`
}

// String implements [fmt.Stringer] for a [File].
func (f File) String() string {
	builder := &strings.Builder{}

	if f.Name != "" {
		fmt.Fprintf(builder, "@name = %s\n\n", f.Name)
	}

	for _, prompt := range f.Prompts {
		builder.WriteString(prompt.String())
	}

	for _, key := range slices.Sorted(maps.Keys(f.Vars)) {
		fmt.Fprintf(builder, "@%s = %s\n", key, f.Vars[key])
	}

	// Only show timeouts if they are non-default
	if f.Timeout != 0 {
		fmt.Fprintf(builder, "@timeout = %s\n", f.Timeout)
	}

	if f.ConnectionTimeout != 0 {
		fmt.Fprintf(builder, "@connection-timeout = %s\n", f.ConnectionTimeout)
	}

	// Same with no-redirect
	if f.NoRedirect {
		fmt.Fprintf(builder, "@no-redirect = %v\n", f.NoRedirect)
	}

	// Separate the request start from the globals by a newline
	builder.WriteByte('\n')

	for _, request := range f.Requests {
		builder.WriteString(request.String())
	}

	return builder.String()
}

// GetRequest returns the request by name from a File.
func (f File) GetRequest(name string) (Request, bool) {
	for _, request := range f.Requests {
		if request.Name == name {
			return request, true
		}
	}

	return Request{}, false
}
