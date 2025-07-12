package spec

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// A Request represents a single HTTP request described in a [File].
type Request struct {
	// Request scoped variables, override globals if specified
	Vars map[string]string `json:"vars,omitempty"`

	// Request headers, may have variable interpolation in values but not keys
	Headers map[string]string `json:"headers,omitempty"`

	// Request scoped prompts, the user will be asked to provide values for each of these
	// whenever this particular request is invoked.
	//
	// The provided values will then be stored in Vars for future use e.g. as interpolation
	// in the request body.
	Prompts []Prompt `json:"prompts,omitempty"`

	// Optional name, if empty request should be named after it's index e.g. "#1"
	Name string `json:"name,omitempty"`

	// Optional request comment
	Comment string `json:"comment,omitempty"`

	// The HTTP method
	Method string `json:"method,omitempty"`

	// The complete URL with any variable interpolation evaluated
	URL string `json:"url,omitempty"`

	// Version of the HTTP protocol to use e.g. "1.2"
	HTTPVersion string `json:"httpVersion,omitempty"`

	// If the body is to be populated by reading a local file, this is the path
	// to that local file (relative to the .http file)
	BodyFile string `json:"bodyFile,omitempty"`

	// If a response redirect was provided, this is the path to the local file into
	// which to write the response (relative to the .http file)
	ResponseFile string `json:"responseFile,omitempty"`

	// Request body, if provided inline. Again, variable interpolation and special things like {{ $random.uuid }} have been evaluated
	Body []byte `json:"body,omitempty"`

	// Request scoped timeout, overrides global if set
	Timeout time.Duration `json:"timeout,omitempty"`

	// Request scoped connection timeout, overrides global if set
	ConnectionTimeout time.Duration `json:"connectionTimeout,omitempty"`

	// Disable following redirects for this request, overrides global if set
	NoRedirect bool `json:"noRedirect,omitempty"`
}

// String implements [fmt.Stringer] for a [Request].
func (r Request) String() string {
	builder := &strings.Builder{}

	if r.Comment != "" {
		fmt.Fprintf(builder, "### %s\n", r.Comment)
	} else {
		builder.WriteString("###\n")
	}

	if r.Name != "" {
		fmt.Fprintf(builder, "# @name = %s\n", r.Name)
	}

	for _, prompt := range r.Prompts {
		builder.WriteString(prompt.String())
	}

	for _, key := range slices.Sorted(maps.Keys(r.Vars)) {
		fmt.Fprintf(builder, "# @%s = %s\n", key, r.Vars[key])
	}

	// Only show timeouts if they are non-default
	if r.Timeout != 0 {
		fmt.Fprintf(builder, "# @timeout = %s\n", r.Timeout)
	}

	if r.ConnectionTimeout != 0 {
		fmt.Fprintf(builder, "# @connection-timeout = %s\n", r.ConnectionTimeout)
	}

	// Same with no-redirect
	if r.NoRedirect {
		fmt.Fprintf(builder, "# @no-redirect = %v\n", r.NoRedirect)
	}

	if r.HTTPVersion != "" {
		fmt.Fprintf(builder, "%s %s %s\n", r.Method, r.URL, r.HTTPVersion)
	} else {
		fmt.Fprintf(builder, "%s %s\n", r.Method, r.URL)
	}

	for _, key := range slices.Sorted(maps.Keys(r.Headers)) {
		fmt.Fprintf(builder, "%s: %s\n", key, r.Headers[key])
	}

	// Separate the body section
	if r.Body != nil || r.BodyFile != "" || r.ResponseFile != "" {
		builder.WriteString("\n")
	}

	if r.BodyFile != "" {
		fmt.Fprintf(builder, "< %s\n", r.BodyFile)
	}

	if r.Body != nil {
		fmt.Fprintf(builder, "%s\n", string(r.Body))
	}

	if r.ResponseFile != "" {
		fmt.Fprintf(builder, "> %s\n", r.ResponseFile)
	}

	return builder.String()
}

// FilterValue helps implement tea.list.Item.
//
// See https://github.com/charmbracelet/bubbles/tree/master/list#adding-custom-items.
func (r Request) FilterValue() string {
	return r.Name
}

// Title returns the request's name.
func (r Request) Title() string {
	return r.Name
}

// Description returns a description of the request, in this case the method and URL.
func (r Request) Description() string {
	return fmt.Sprintf("%s %s", r.Method, r.URL)
}
