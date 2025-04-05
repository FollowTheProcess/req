// Package tui implements the terminal user interface for selecting and filtering .http files and requests.
package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const errorClearAfter = 2 * time.Second

// TODO(@FollowTheProcess): I want to understand all this a bit more, atm it's basically copy pasted from the bubbles filepicker example
// with a bit of bodgery to show the help. Perhaps I need to make my own bubbles to do all this, then I'll understand it a lot more
// would also let me play with some ideas like:
// - Reading the file and showing in a preview window on hover (files only)
// - Once selected a file, parse it and then have a fancy list bubble of the http request the cursor is on
// - On enter, it's basically now just `req do <file> <request>` so close the TUI and do the request

// Run runs the TUI, this is what happens when users call `req` with no arguments.
func Run() error {
	picker := filepicker.New()
	picker.AllowedTypes = []string{".http", ".rest"}
	picker.CurrentDirectory = "."

	m := model{
		filepicker: picker,
		help:       help.New(),
		keys:       keyMap(picker.KeyMap),
	}

	tm, err := tea.NewProgram(&m).Run()
	if err != nil {
		return err
	}

	mm, ok := tm.(model)
	if !ok {
		return fmt.Errorf("tui error, final model was not as expected: %T", tm)
	}
	fmt.Println("You selected: " + m.filepicker.Styles.Selected.Render(mm.selectedFile))
	return nil
}

type keyMap filepicker.KeyMap

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Back,
		k.Select,
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Back, k.Select},
		{k.GoToTop, k.GoToLast, k.PageUp, k.PageDown, k.Open},
	}
}

type model struct {
	filepicker   filepicker.Model
	help         help.Model
	err          error
	selectedFile string
	keys         keyMap
	quitting     bool
}

type clearErrorMsg struct{}

func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.filepicker.Height = msg.Height
		m.help.Width = msg.Width

	case clearErrorMsg:
		m.err = nil
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a disabled file?
	// This is only necessary to display an error to the user.
	if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.err = errors.New(path + " is not valid.")
		m.selectedFile = ""
		return m, tea.Batch(cmd, clearErrorAfter(errorClearAfter))
	}

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		m.selectedFile = path
		m.quitting = true
		return m, tea.Quit
	}

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	s.WriteByte('\n')
	switch {
	case m.err != nil:
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.err.Error()))
	case m.selectedFile == "":
		s.WriteString("Pick a file:")
	default:
		s.WriteString("Selected file: " + m.filepicker.Styles.Selected.Render(m.selectedFile))
	}

	s.WriteByte('\n')
	s.WriteString(m.filepicker.View())

	helpView := m.help.View(m.keys)
	s.WriteString(helpView)
	return s.String()
}
