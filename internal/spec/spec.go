// Package spec provides the [File] and [Request] data structure which together represent
// a .http file.
//
// They differ from their counterparts in the syntax package in that they are "resolved". This means:
//   - Variable interpolation e.g. `{{...}}` has been performed
//   - Default configuration has been put in place if not provided in the raw file
//
// This resolution means that the requests described can be correctly made via http.
package spec

import (
	"bytes"
	"fmt"
	"net/url"
	"text/template"
	"time"

	"go.followtheprocess.codes/req/internal/syntax"
)

const (
	DefaultConnectionTimeout = 10 * time.Second // Default connection timeout for HTTP requests
	DefaultTimeout           = 30 * time.Second // Default overall timeout for HTTP requests
)

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
		Prompts:           resolvePrompts(in.Prompts),
	}

	// TODO(@FollowTheProcess): When the prompts get answered, we need to store the answers
	// in the global scope here, but the local scope when processing request prompts

	// Currently, this works because we don't actually allow template tags in the values of
	// global variables at a syntax level, so we *know* that they are all fully resolved
	// already. This is something I'd like to look at but would involve variable resolution
	// in order so that a variable defined on line 1 can be used in another defined on line 2
	// but not vice versa
	scope := NewScope()
	scope.Global = in.Vars
	resolved.Vars = in.Vars

	resolvedRequests := make([]Request, 0, len(in.Requests))
	for _, request := range in.Requests {
		resolved, err := resolveRequest(request, scope)
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

// resolvePrompts converts a []syntax.Prompt to a []Prompt.
func resolvePrompts(in []syntax.Prompt) []Prompt {
	resolved := make([]Prompt, 0, len(in))
	for _, prompt := range in {
		resolved = append(resolved, Prompt{Name: prompt.Name, Description: prompt.Description})
	}

	return resolved
}

// resolveRequest converts a [syntax.Request] to a [Request], performing variable
// resolution and other validation.
//
// Note that scope is passed by value, this is because we want local variable isolation
// in each request, and this is a nice easy way of doing that.
func resolveRequest(in syntax.Request, scope Scope) (Request, error) {
	// All stuff that needs no transformation
	resolved := Request{
		Name:              in.Name,
		Comment:           in.Comment,
		Prompts:           resolvePrompts(in.Prompts),
		Method:            in.Method,
		BodyFile:          in.BodyFile,
		ResponseFile:      in.ResponseFile,
		Timeout:           in.Timeout,
		ConnectionTimeout: in.ConnectionTimeout,
		NoRedirect:        in.NoRedirect,
	}

	buf := &bytes.Buffer{}

	// No point allocating a Vars map if it has no local variables
	if len(in.Vars) > 0 {
		resolvedVars := make(map[string]string, len(in.Vars))

		for key, value := range in.Vars {
			name := fmt.Sprintf("Request %s/Var %s", in.Name, key)
			tmp, err := template.New(name).Option("missingkey=error").Parse(value)
			if err != nil {
				return Request{}, fmt.Errorf("invalid template syntax in var %s: %w", key, err)
			}
			if err = tmp.Execute(buf, scope); err != nil {
				return Request{}, fmt.Errorf("failed to execute request variable templating for request %s: %w", in.Name, err)
			}

			resolvedVars[key] = buf.String()

			// Clear the buffer for the next iteration
			buf.Reset()
		}

		// Note: Affecting the copy of scope in this function only
		scope.Local = resolvedVars
		resolved.Vars = resolvedVars

		// Might as well reuse the same buffer later
		buf.Reset()
	}

	resolvedHeaders := make(map[string]string, len(in.Headers))

	for key, value := range in.Headers {
		name := fmt.Sprintf("Request %s/Header %s", in.Name, key)
		tmp, err := template.New(name).Option("missingkey=error").Parse(value)
		if err != nil {
			return Request{}, fmt.Errorf("invalid template syntax in header %s: %w", key, err)
		}
		if err = tmp.Execute(buf, scope); err != nil {
			return Request{}, fmt.Errorf("failed to execute request header templating for request %s: %w", in.Name, err)
		}

		resolvedHeaders[key] = buf.String()
		buf.Reset()
	}

	resolved.Headers = resolvedHeaders

	// Now for the URL
	buf.Reset()
	tmp, err := template.New(fmt.Sprintf("Request %s/URL", in.Name)).Option("missingkey=error").Parse(in.URL)
	if err != nil {
		return Request{}, fmt.Errorf("invalid template syntax in URL %s: %w", in.URL, err)
	}
	if err = tmp.Execute(buf, scope); err != nil {
		return Request{}, fmt.Errorf("failed to execute URL templating for request %s: %w", in.Name, err)
	}

	// Now URL templates have been resolved, it must be a valid URL
	resolvedURL := buf.String()
	_, err = url.ParseRequestURI(resolvedURL)
	if err != nil {
		return Request{}, fmt.Errorf("invalid URL for request %s: %w", in.Name, err)
	}

	resolved.URL = resolvedURL

	// Lastly, the body
	buf.Reset()
	tmp, err = template.New(fmt.Sprintf("Request %s/Body", in.Name)).Option("missingkey=error").Parse(string(in.Body))
	if err != nil {
		return Request{}, fmt.Errorf("invalid template syntax in request %s body: %w", in.Name, err)
	}
	if err = tmp.Execute(buf, scope); err != nil {
		return Request{}, fmt.Errorf("failed to execute templating for request %s body: %w", in.Name, err)
	}

	resolved.Body = buf.Bytes()

	// Ensure we have sensible default timeouts if none were set
	if resolved.Timeout == 0 {
		resolved.Timeout = DefaultTimeout
	}

	if resolved.ConnectionTimeout == 0 {
		resolved.ConnectionTimeout = DefaultConnectionTimeout
	}

	return resolved, nil
}
