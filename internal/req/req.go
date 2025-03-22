// Package req implements the actual functionality exposed via the CLI.
package req

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

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
func (r Req) Check(files []string) error {
	for _, file := range files {
		return func() error {
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
		}()
	}

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

// DoOptions are the flags passed to the `req do` subcommand.
type DoOptions struct {
	Output            string
	Timeout           time.Duration
	ConnectionTimeout time.Duration
	NoRedirect        bool
	Verbose           bool
}

// Do implements the `req do` subcommand.
func (r Req) Do(file, name string, options DoOptions) error {
	// TODO(@FollowTheProcess): Make an integration test that spins up a fake server and make
	// a .http file that points to that URL and make sure we get the right stuff
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

	resolved, err := spec.ResolveFile(raw)
	if err != nil {
		return err
	}

	request, ok := resolved.GetRequest(name)
	if !ok {
		return fmt.Errorf("%s does not contain request %s", file, name)
	}

	// TODO(@FollowTheProcess): A context with a timeout and listens to ctrl+c
	httpRequest, err := http.NewRequestWithContext(
		context.TODO(),
		request.Method,
		request.URL,
		bytes.NewReader(request.Body),
	)
	if err != nil {
		return err
	}

	for key, value := range request.Headers {
		httpRequest.Header.Add(key, value)
	}

	// TODO(@FollowTheProcess): Make a proper http client
	client := http.Client{
		Timeout: request.Timeout,
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("HTTP: %w", err)
	}

	if response == nil {
		return errors.New("nil response")
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	fmt.Println(response.Status)
	fmt.Println(string(body))
	return nil
}
