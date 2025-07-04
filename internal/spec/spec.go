// Package spec provides the [File] and [Request] data structure which together represent
// a .http file.
//
// They differ from their counterparts in the syntax package in that they are "resolved". This means:
//   - Variable interpolation e.g. `{{base}}` has been performed
//   - Default configuration has been put in place if not provided in the raw file
//
// This resolution means that the requests described can be correctly made via http.
package spec

import (
	"bytes"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	"go.followtheprocess.codes/req/internal/syntax"
)

const (
	DefaultConnectionTimeout = 10 * time.Second // Default connection timeout for HTTP requests
	DefaultTimeout           = 30 * time.Second // Default overall timeout for HTTP requests
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

// Prompt represents a variable that requires the user to specify by responding to a prompt.
type Prompt struct {
	// Name of the variable into which to store the user provided value
	Name string `json:"name,omitempty"`

	// Description of the prompt, optional
	Description string `json:"description,omitempty"`
}

// String implements [fmt.Stringer] for a [Prompt].
func (p Prompt) String() string {
	if p.Description != "" {
		return fmt.Sprintf("@prompt %s %s\n", p.Name, p.Description)
	}

	return fmt.Sprintf("@prompt %s\n", p.Name)
}

// ResolveFile converts a [syntax.File] to a [File], performing variable
// resolution and other validation.
func ResolveFile(in syntax.File) (File, error) {
	resolved := File{
		Name:              in.Name,
		Timeout:           in.Timeout,
		ConnectionTimeout: in.ConnectionTimeout,
		NoRedirect:        in.NoRedirect,
	}

	// Note: We could use something like text/template but I wanted to try and be compatible with
	// things like the VSCode rest extension which uses {{something}} syntax as opposed to Go's {{.Something}}

	// TODO(@FollowTheProcess): I think we might need to do something else when it comes to things like {{ $random.uuid }}
	// but this is fine for now
	oldnew := make([]string, 0, len(in.Vars))
	for key, value := range in.Vars {
		// e.g. strings.NewReplace("{{base}}", "https://api.com")
		oldnew = append(oldnew, fmt.Sprintf("{{%s}}", key))
		oldnew = append(oldnew, value)
	}

	replacer := strings.NewReplacer(oldnew...)

	globals := make(map[string]string, len(in.Vars))

	for key, value := range in.Vars {
		replaced, err := replaceAndValidate(replacer, value)
		if err != nil {
			return File{}, err
		}

		globals[key] = replaced
	}

	resolved.Vars = globals

	resolvedRequests := make([]Request, 0, len(in.Requests))
	for _, request := range in.Requests {
		resolved, err := resolveRequest(request, globals)
		if err != nil {
			return File{}, fmt.Errorf("could not resolve request %s: %w", request.Name, err)
		}

		resolvedRequests = append(resolvedRequests, resolved)
	}

	resolved.Requests = resolvedRequests

	// Ensure we have sensible default timeouts if none were set
	if resolved.Timeout == 0 {
		resolved.Timeout = DefaultTimeout
	}

	if resolved.ConnectionTimeout == 0 {
		resolved.ConnectionTimeout = DefaultConnectionTimeout
	}

	return resolved, nil
}

// resolveRequest converts a [syntax.Request] to a [Request], performing variable
// resolution and other validation.
func resolveRequest(in syntax.Request, globals map[string]string) (Request, error) {
	// All stuff that needs no transformation
	resolved := Request{
		Name:              in.Name,
		Comment:           in.Comment,
		Method:            in.Method,
		BodyFile:          in.BodyFile,
		ResponseFile:      in.ResponseFile,
		Timeout:           in.Timeout,
		ConnectionTimeout: in.ConnectionTimeout,
		NoRedirect:        in.NoRedirect,
	}

	// TODO(@FollowTheProcess): I think we should actually scan interpolation tokens in the scanner
	// and parse them "properly"

	// Replace local request scoped vars but also globals because global variables
	// could be used in request variables
	oldnew := make([]string, 0, len(in.Vars)+len(globals))
	for key, value := range in.Vars {
		// e.g. strings.NewReplace("{{request_var}}", "something")
		oldnew = append(oldnew, fmt.Sprintf("{{%s}}", key))
		oldnew = append(oldnew, value)
	}

	for key, value := range globals {
		oldnew = append(oldnew, fmt.Sprintf("{{%s}}", key))
		oldnew = append(oldnew, value)
	}

	replacer := strings.NewReplacer(oldnew...)

	vars := make(map[string]string, len(in.Vars))
	for key, value := range in.Vars {
		replaced, err := replaceAndValidate(replacer, value)
		if err != nil {
			return Request{}, err
		}

		vars[key] = replaced
	}

	resolved.Vars = vars

	prompts := make([]Prompt, 0, len(in.Prompts))
	for _, prompt := range in.Prompts {
		prompts = append(prompts, Prompt{Name: prompt.Name, Description: prompt.Description})
	}

	resolved.Prompts = prompts

	headers := make(map[string]string, len(in.Headers))
	for key, value := range in.Headers {
		replaced, err := replaceAndValidate(replacer, value)
		if err != nil {
			return Request{}, err
		}

		headers[key] = replaced
	}

	resolved.Headers = headers

	// Global vars may be used in request URL, and we can now strictly validate it
	// as it should be absolute
	replacedURL, err := replaceAndValidate(replacer, in.URL)
	if err != nil {
		return Request{}, err
	}

	_, err = url.ParseRequestURI(replacedURL)
	if err != nil {
		return Request{}, fmt.Errorf("invalid URL for request %s: %w", in.Name, err)
	}

	resolved.URL = replacedURL

	replacedBody, err := replaceAndValidate(replacer, string(in.Body))
	if err != nil {
		return Request{}, err
	}

	resolved.Body = []byte(replacedBody)

	// Ensure we have sensible default timeouts if none were set
	if resolved.Timeout == 0 {
		resolved.Timeout = DefaultTimeout
	}

	if resolved.ConnectionTimeout == 0 {
		resolved.ConnectionTimeout = DefaultConnectionTimeout
	}

	return resolved, nil
}

// Equal reports whether two [File]s are equal.
func Equal(a, b File) bool {
	switch {
	case a.Name != b.Name,
		!maps.Equal(a.Vars, b.Vars),
		!slices.Equal(a.Prompts, b.Prompts),
		!slices.EqualFunc(a.Requests, b.Requests, requestEqual),
		a.Timeout != b.Timeout,
		a.ConnectionTimeout != b.ConnectionTimeout,
		a.NoRedirect != b.NoRedirect:
		return false
	default:
		return true
	}
}

// requestEqual reports whether two [Request]s are equal.
func requestEqual(a, b Request) bool {
	switch {
	case !maps.Equal(a.Vars, b.Vars),
		!maps.Equal(a.Headers, b.Headers),
		!slices.Equal(a.Prompts, b.Prompts),
		a.Name != b.Name,
		a.Comment != b.Comment,
		a.Method != b.Method,
		a.URL != b.URL,
		a.HTTPVersion != b.HTTPVersion,
		a.BodyFile != b.BodyFile,
		a.ResponseFile != b.ResponseFile,
		!bytes.Equal(a.Body, b.Body),
		a.Timeout != b.Timeout,
		a.ConnectionTimeout != b.ConnectionTimeout,
		a.NoRedirect != b.NoRedirect:
		return false
	default:
		return true
	}
}

// replaceAndValidate performs string variable replacement using the passed in replacer
// and ensures there are no template tags remaining.
func replaceAndValidate(replacer *strings.Replacer, in string) (out string, err error) {
	replaced := replacer.Replace(in)
	interpStart := strings.Index(replaced, "{{")
	interpEnd := strings.Index(replaced, "}}")

	const cutoff = 15

	end := min(interpStart+cutoff, len(replaced)-1)

	if interpStart != -1 {
		// There are template tags remaining
		if interpEnd == -1 {
			// Unterminated
			return "", fmt.Errorf(
				"unterminated variable interpolation: %q",
				replaced[interpStart:end],
			)
		}

		// Undeclared variable
		return "", fmt.Errorf(
			"use of undeclared variable %q in interpolation",
			replaced[interpStart:interpEnd+2],
		)
	}

	return replaced, nil
}
