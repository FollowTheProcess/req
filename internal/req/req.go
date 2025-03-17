// Package req implements the actual functionality exposed via the CLI.
package req

import (
	"fmt"
	"io"
	"os"

	"github.com/FollowTheProcess/msg"
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
