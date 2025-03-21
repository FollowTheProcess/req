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
		cli.SubCommands(check, show),
	)
}

// check returns the check subcommand.
func check() (*cli.Command, error) {
	return cli.New(
		"check",
		cli.Short("Check a .http file for syntax errors"),
		cli.RequiredArg("file", "Path of the .http file to check"),
		cli.Run(func(cmd *cli.Command, args []string) error {
			req := req.New(cmd.Stdout(), cmd.Stderr())
			return req.Check(cmd.Arg("file"))
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
