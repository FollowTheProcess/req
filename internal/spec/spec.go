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

	"github.com/FollowTheProcess/req/internal/syntax"
)

const (
	DefaultConnectionTimeout = 10 * time.Second // Default connection timeout for HTTP requests
	DefaultTimeout           = 30 * time.Second // Default overall timeout for HTTP requests
)

// A File is a single .http file.
//
// It may be constructed with [ResolveFile] from a [syntax.File].
type File struct {
	Name              string            `json:"name,omitempty"`              // Name of the file (or @name in global scope if given)
	Vars              map[string]string `json:"vars,omitempty"`              // Global variables defined at the top level, e.g. base url
	Requests          []Request         `json:"requests,omitempty"`          // 1 or more HTTP requests
	Timeout           time.Duration     `json:"timeout,omitempty"`           // Global timeout for all requests
	ConnectionTimeout time.Duration     `json:"connectionTimeout,omitempty"` // Global connection timeout
	NoRedirect        bool              `json:"noRedirect,omitempty"`        // Disable following redirects globally
}

// A Request represents a single HTTP request described in a [File].
type Request struct {
	Vars              map[string]string `json:"vars,omitempty"`              // Request scoped variables, override globals if specified
	Headers           map[string]string `json:"headers,omitempty"`           // Request headers, may have variable interpolation in values but not keys
	Name              string            `json:"name,omitempty"`              // Name of the request, either set at parse time or #1 after it's index (1 based)
	Method            string            `json:"method,omitempty"`            // The HTTP method e.g. "GET", "POST"
	URL               string            `json:"url,omitempty"`               // The complete URL with any variable interpolation evaluated
	HTTPVersion       string            `json:"httpVersion,omitempty"`       // Version of the HTTP protocol to use e.g. 1.2
	BodyFile          string            `json:"bodyFile,omitempty"`          // If the body is to be populated from a local file, this is the path to that file (relative to the .http file)
	ResponseRef       string            `json:"responseRef,omitempty"`       // If a response reference was provided, this is it's filepath (relative to the .http file)
	Body              []byte            `json:"body,omitempty"`              // Request body, if provided inline. Again, variable interpolation and special things like {{ $random.uuid }} have been evaluated
	Timeout           time.Duration     `json:"timeout,omitempty"`           // Request specific timeout, overrides global if set
	ConnectionTimeout time.Duration     `json:"connectionTimeout,omitempty"` // Request specific connection timeout, overrides global if set
	NoRedirect        bool              `json:"noRedirect,omitempty"`        // Disable following redirects on this specific request, overrides global if set
}

// ResolveFile converts a [syntax.File] to a [File], performing variable
// resolution and other validation.
func ResolveFile(in syntax.File) (File, error) {
	resolved := File{
		Name:              in.Name,
		Timeout:           time.Duration(in.Timeout),
		ConnectionTimeout: time.Duration(in.ConnectionTimeout),
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
	resolved := Request{
		Name:              in.Name,
		Method:            in.Method,
		Timeout:           time.Duration(in.Timeout),
		ConnectionTimeout: time.Duration(in.ConnectionTimeout),
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
		a.Name != b.Name,
		a.Method != b.Method,
		a.URL != b.URL,
		a.HTTPVersion != b.HTTPVersion,
		a.BodyFile != b.BodyFile,
		a.ResponseRef != b.ResponseRef,
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
			return "", fmt.Errorf("unterminated variable interpolation: %q", replaced[interpStart:end])
		}

		// Undeclared variable
		return "", fmt.Errorf("use of undeclared variable %q in interpolation", replaced[interpStart:interpEnd+2])
	}

	return replaced, nil
}
