// Package cmd implements req's CLI.
package cmd

import (
	"fmt"

	"github.com/FollowTheProcess/cli"
	"github.com/FollowTheProcess/req/internal/req"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

// Build returns the root req CLI command.
func Build() (*cli.Command, error) {
	return cli.New(
		"req",
		cli.Short("Work with .http files on the command line"),
		cli.Allow(cli.NoArgs()),
		cli.Version(version),
		cli.Commit(commit),
		cli.BuildDate(date),
		cli.Run(func(cmd *cli.Command, args []string) error {
			// Long term I'd like bare usage to find all the .http or .rest files recursively under cwd
			// then launch some sort of interactive picker TUI thing, maybe some charm stuff
			fmt.Println("Fun things coming soon...")
			return nil
		}),
		cli.SubCommands(check, show, do),
	)
}

// check returns the check subcommand.
func check() (*cli.Command, error) {
	return cli.New(
		"check",
		cli.Short("Check .http files for syntax errors"),
		cli.Allow(cli.MinArgs(1)),
		cli.Run(func(cmd *cli.Command, args []string) error {
			req := req.New(cmd.Stdout(), cmd.Stderr())
			return req.Check(args)
		}),
	)
}

// show returns the show subcommand.
func show() (*cli.Command, error) {
	var options req.ShowOptions
	return cli.New(
		"show",
		cli.Short("Show the contents of a .http file"),
		cli.RequiredArg("file", "Path of the .http file"),
		cli.Flag(&options.Resolve, "resolve", 'r', false, "Resolve the file handling variable interpolation etc."),
		cli.Flag(&options.JSON, "json", 'j', false, "Output the file as JSON"),
		cli.Run(func(cmd *cli.Command, args []string) error {
			req := req.New(cmd.Stdout(), cmd.Stderr())
			return req.Show(cmd.Arg("file"), options)
		}),
	)
}

const doLong = `
The request headers, body and other settings will be taken from the
file but may be overridden by the use of command line flags like
'--timeout' etc.

Responses can be saved to a file with the '--output' flag.
`

// do returns the do subcommand.
func do() (*cli.Command, error) {
	var options req.DoOptions
	return cli.New(
		"do",
		cli.Short("Execute a http request from a file"),
		cli.Long(doLong),
		cli.RequiredArg("file", ".http file containing the request"),
		cli.RequiredArg("name", "The name of the request to send"),
		cli.Flag(&options.Timeout, "timeout", cli.NoShortHand, 0, "Timeout for the request"),
		cli.Flag(
			&options.ConnectionTimeout,
			"connection-timeout",
			cli.NoShortHand,
			0,
			"Connection timeout for the request",
		),
		cli.Flag(&options.NoRedirect, "no-redirect", cli.NoShortHand, false, "Disable following redirects"),
		cli.Flag(&options.Output, "output", 'o', "", "Name of a file to save the response"),
		cli.Flag(&options.Verbose, "verbose", 'v', false, "Enable debug logging"),
		cli.Run(func(cmd *cli.Command, args []string) error {
			req := req.New(cmd.Stdout(), cmd.Stderr())
			return req.Do(cmd.Arg("file"), cmd.Arg("name"), options)
		}),
	)
}
