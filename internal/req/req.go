// Package req implements the actual functionality exposed via the CLI.
package req

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/FollowTheProcess/msg"
	"github.com/FollowTheProcess/req/internal/spec"
	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/parser"
)

// Req holds the state of the program.
type Req struct {
	stdout io.Writer // Normal program output is written here
	stderr io.Writer // Logs and debug info
}

// New returns a new instance of [Req].
func New(stdout, stderr io.Writer) Req {
	return Req{
		stdout: stdout,
		stderr: stderr,
	}
}

// Check implements the `req check` subcommand.
func (r Req) Check(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	parser, err := parser.New(file, f, syntax.PrettyConsoleHandler(r.stderr))
	if err != nil {
		return err
	}

	_, err = parser.Parse()
	if err != nil {
		return fmt.Errorf("%w: %s is not valid http syntax", err, file)
	}

	msg.Fsuccess(r.stdout, "%s is valid", file)
	return nil
}

// ShowOptions are the flags passed to the `req show` subcommand.
type ShowOptions struct {
	Resolve bool // Resolve variables and do replacements
	JSON    bool // Output the file in JSON
}

// Show implements the `req show` subcommand.
func (r Req) Show(file string, options ShowOptions) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	parser, err := parser.New(file, f, syntax.PrettyConsoleHandler(r.stderr))
	if err != nil {
		return err
	}

	raw, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("%w: %s is not valid http syntax", err, file)
	}

	if options.Resolve {
		resolved, err := spec.ResolveFile(raw)
		if err != nil {
			return err
		}

		if options.JSON {
			return json.NewEncoder(r.stdout).Encode(resolved)
		}

		fmt.Fprintln(r.stdout, strings.TrimSpace(resolved.String()))
		return nil
	}

	if options.JSON {
		return json.NewEncoder(r.stdout).Encode(raw)
	}

	fmt.Fprintln(r.stdout, strings.TrimSpace(raw.String()))
	return nil
}
