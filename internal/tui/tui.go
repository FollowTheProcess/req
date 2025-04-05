// Package tui implements the terminal user interface for selecting and filtering .http files and requests.
package tui

import (
	"fmt"

	"github.com/FollowTheProcess/req/internal/tui/components/filepicker"
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
	fmt.Printf("You selected %s\n", final.Selected())
	return nil
}
