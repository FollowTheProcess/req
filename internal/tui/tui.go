// Package tui implements the terminal user interface for selecting and filtering .http files and requests.
package tui

import (
	"fmt"
	"os"

	"github.com/FollowTheProcess/req/internal/req"
	"github.com/FollowTheProcess/req/internal/spec"
	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/parser"
	"github.com/FollowTheProcess/req/internal/tui/components/filepicker"
	"github.com/FollowTheProcess/req/internal/tui/components/list"
	tea "github.com/charmbracelet/bubbletea"
)

// TODO(@FollowTheProcess): I want to understand all this a bit more, atm it's basically copy pasted from the bubbles filepicker example
// with a bit of bodgery to show the help. Perhaps I need to make my own bubbles to do all this, then I'll understand it a lot more
// would also let me play with some ideas like:
// - Reading the file and showing in a preview window on hover (files only)
// - Once selected a file, parse it and then have a fancy list bubble of the http request the cursor is on
// - On enter, it's basically now just `req do <file> <request>` so close the TUI and do the request

// Run runs the TUI, this is what happens when users call `req` with no arguments.
func Run() error {
	model := filepicker.New()

	tm, err := tea.NewProgram(&model).Run()
	if err != nil {
		return err
	}

	final, ok := tm.(filepicker.Model)
	if !ok {
		return fmt.Errorf("tui error, final model was not as expected: %T", tm)
	}

	file := final.Selected()

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	parser, err := parser.New(file, f, syntax.PrettyConsoleHandler(os.Stderr))
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

	listModel := list.New("HTTP Requests in "+file, resolved.Requests)

	tm, err = tea.NewProgram(&listModel, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}

	finalListModel, ok := tm.(list.Model)
	if !ok {
		return fmt.Errorf("tui error, list final model was not as expected: %T", tm)
	}

	request := finalListModel.Selected()

	// TODO(@FollowTheProcess): This parses the file again

	app := req.New(os.Stdout, os.Stderr, false)
	options := req.DoOptions{
		Timeout:           req.DefaultTimeout,
		ConnectionTimeout: req.DefaultConnectionTimeout,
	}

	return app.Do(file, request, options)
}
