// Package syntax handles parsing the raw .http file text into meaningful
// data structures and implements the tokeniser and parser as well as some
// language level integration tests.
package syntax

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/FollowTheProcess/hue"
)

// An ErrorHandler may be provided to parts of the parsing pipeline. If a syntax error is encountered and
// a non-nil handler was provided, it is called with the position info and error message.
type ErrorHandler func(pos Position, msg string)

// Position is an arbitrary source file position including file, line
// and column information. It can also express a range of source via StartCol
// and EndCol, this is useful for error reporting.
//
// Position's without filenames are considered invalid, in the case of stdin
// the string "stdin" may be used.
type Position struct {
	Name     string // Filename
	Offset   int    // Byte offset of the position from the start of the file
	Line     int    // Line number (1 indexed)
	StartCol int    // Start column (1 indexed)
	EndCol   int    // End column (1 indexed), EndCol == StartCol when pointing to a single character
}

// IsValid reports whether the [Position] describes a valid source position.
//
// The rules are:
//
//   - At least Name, Line and StartCol must be set (and non zero)
//   - EndCol cannot be 0, it's only allowed values are StartCol or any number greater than StartCol
func (p Position) IsValid() bool {
	if p.Name == "" || p.Line < 1 || p.StartCol < 1 || p.EndCol < 1 || (p.EndCol >= 1 && p.EndCol < p.StartCol) {
		return false
	}
	return true
}

// String returns a string representation of a [Position].
//
// It is formatted such that most text editors/terminals will be able to support clicking on it
// and navigating to the position.
//
// Depending on which fields are set, the string returned will be different:
//
//   - "file:line:start-end": valid position pointing to a range of text on the line
//   - "file:line:start": valid position pointing to a single character on the line (EndCol == StartCol)
//
// At least Name, Line and StartCol must be present for a valid position, and Line and StarCol must be > 0. If not, an error
// string will be returned.
func (p Position) String() string {
	if !p.IsValid() {
		return fmt.Sprintf(
			"BadPosition: {Name: %q, Line: %d, StartCol: %d, EndCol: %d}",
			p.Name,
			p.Line,
			p.StartCol,
			p.EndCol,
		)
	}

	if p.StartCol == p.EndCol {
		// No range, just a single position
		return fmt.Sprintf("%s:%d:%d", p.Name, p.Line, p.StartCol)
	}

	return fmt.Sprintf("%s:%d:%d-%d", p.Name, p.Line, p.StartCol, p.EndCol)
}

// Duration is a [time.Duration] but more JSON friendly.
type Duration time.Duration

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *Duration) UnmarshalText(text []byte) error {
	duration, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}

	*d = Duration(duration)
	return nil
}

// File represents a single .http file as parsed.
//
// It is *nearly* concrete but may have variable interpolation still to evaluate
// in a number of fields, URLs may not be valid etc. This is a structured
// populated from the as-parsed text.
type File struct {
	Name              string            `json:"name,omitempty"`              // Name of the file (or @name in global scope if given)
	Vars              map[string]string `json:"vars,omitempty"`              // Global variables defined at the top level, e.g. base url
	Requests          []Request         `json:"requests,omitempty"`          // 1 or more HTTP requests
	Timeout           Duration          `json:"timeout,omitempty"`           // Global timeout for all requests
	ConnectionTimeout Duration          `json:"connectionTimeout,omitempty"` // Global connection timeout
	NoRedirect        bool              `json:"noRedirect,omitempty"`        // Disable following redirects globally
}

// Request is a single HTTP request as parsed from a .http file.
type Request struct {
	Vars              map[string]string `json:"vars,omitempty"`              // Request scoped variables, override globals if specified
	Headers           map[string]string `json:"headers,omitempty"`           // Request headers, may have variable interpolation in values but not keys
	Name              string            `json:"name,omitempty"`              // Optional name, if empty request should be named after it's index e.g. "#1"
	Method            string            `json:"method,omitempty"`            // The HTTP method e.g. "GET", "POST"
	URL               string            `json:"url,omitempty"`               // The complete URL, may have variable interpolation e.g. {{base}} or not be valid
	HTTPVersion       string            `json:"httpVersion,omitempty"`       // Version of the HTTP protocol to use e.g. 1.2
	BodyFile          string            `json:"bodyFile,omitempty"`          // If the body is to be populated from a local file, this is the path to that file (relative to the .http file)
	ResponseRef       string            `json:"responseRef,omitempty"`       // If a response reference was provided, this is it's filepath (relative to the .http file)
	Body              []byte            `json:"body,omitempty"`              // Request body, if provided inline. Again, may have variable interpolation and special things like {{ $uuid }}
	Timeout           Duration          `json:"timeout,omitempty"`           // Request specific timeout, overrides global if set
	ConnectionTimeout Duration          `json:"connectionTimeout,omitempty"` // Request specific connection timeout, overrides global if set
	NoRedirect        bool              `json:"noRedirect,omitempty"`        // Disable following redirects on this specific request, overrides global if set
}

// PrettyConsoleHandler returns a [ErrorHandler] that formats the syntax error for
// display on the terminal to a user.
func PrettyConsoleHandler(w io.Writer) ErrorHandler {
	return func(pos Position, msg string) {
		// TODO(@FollowTheProcess): This is a bit better but still some improvement I think
		fmt.Fprintf(w, "%s: %s\n\n", pos, msg)

		contents, err := os.ReadFile(pos.Name)
		if err != nil {
			fmt.Fprintf(w, "unable to show src context: %v\n", err)
			return
		}

		lines := bytes.Split(contents, []byte("\n"))

		const contextLines = 3

		startLine := max(pos.Line-contextLines, 0)
		endLine := max(pos.Line+contextLines, len(lines))

		for i, line := range lines {
			i++ // Lines are 1 indexed
			if i >= startLine && i <= endLine {
				margin := fmt.Sprintf("%d | ", i)
				fmt.Fprintf(w, "%s%s\n", margin, line)
				if i == pos.Line {
					hue.Red.Fprintf(
						w,
						"%s%s\n",
						strings.Repeat(" ", len(margin)+pos.StartCol-1),
						strings.Repeat("─", pos.EndCol-pos.StartCol),
					)
				}
			}
		}
	}
}
