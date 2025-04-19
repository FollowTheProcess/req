// Package filepicker implements a custom filepicker bubbletea component.
package filepicker

import (
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	errorClearAfter = 2 * time.Second
	fileSizeWidth   = 7
	paddingLeft     = 2
)

// Model is the file picker tea Model.
type Model struct {
	fp       filepicker.Model // The base filepicker we build off and customise
	help     help.Model       // The tea model providing the keymap help
	err      error            // Any error encountered during picking
	selected string           // The path to the file that was selected
	keys     keyMap           // The key bindings
	quitting bool             // Whether the TUI is quitting
}

// New returns a new [Model].
func New() Model {
	picker := filepicker.New()
	picker.AllowedTypes = []string{".http", ".rest"}
	picker.CurrentDirectory = "."
	picker.KeyMap = filepicker.KeyMap{
		GoToTop:  key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first")),
		GoToLast: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last")),
		Down:     key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("↓/j", "down")),
		Up:       key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("↑/k", "up")),
		PageUp:   key.NewBinding(key.WithKeys("K", "pgup"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("J", "pgdown"), key.WithHelp("pgdown", "page down")),
		Back:     key.NewBinding(key.WithKeys("h", "backspace", "left", "esc"), key.WithHelp("h", "back")),
		Open:     key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("l/→/enter", "open")),
		Select:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	}

	helpModel := help.New()

	return Model{
		fp:   picker,
		help: helpModel,
		keys: keyMap(picker.KeyMap),
	}
}

// Selected returns the file that was eventually selected by the picker.
func (m Model) Selected() string {
	return m.selected
}

// keyMap builds on the bubbles filepicker key map by implementing the [help.KeyMap]
// interface which enables a nice keybinding help bar at the bottom of the page.
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
		{k.GoToTop, k.GoToLast, k.PageUp},
		{k.PageDown, k.Open},
	}
}

// clearErrorMsg is a [tea.Msg] that tells the TUI to clear any existing
// error message, this happens with [clearErrorAfter].
type clearErrorMsg struct{}

// Send a clearErrorMsg after some duration.
func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

// Init helps implement [tea.Model] for [Model] and initialises the TUI.
func (m Model) Init() tea.Cmd {
	return m.fp.Init()
}

// Update is part of implementing [tea.Model] and updates the UI in response to
// messages, in the case of a filepicker, the messages are keybindings moving
// the cursor up and down, and selecting files/directories.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.fp.SetHeight(msg.Height)
		m.help.Width = msg.Width

	case clearErrorMsg:
		m.err = nil
	}

	var cmd tea.Cmd
	m.fp, cmd = m.fp.Update(msg)

	// Did the user select a disabled file?
	// This is only necessary to display an error to the user.
	if didSelect, path := m.fp.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.err = errors.New(path + " is not valid.")
		m.selected = ""
		return m, tea.Batch(cmd, clearErrorAfter(errorClearAfter))
	}

	// Did the user select a file?
	if didSelect, path := m.fp.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		m.selected = path
		m.quitting = true
		return m, tea.Quit
	}

	return m, cmd
}

// View is the last part of implementing [tea.Model] and shows the model to the user.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	s.WriteByte('\n')
	switch {
	case m.err != nil:
		s.WriteString(m.fp.Styles.DisabledFile.Render(m.err.Error()))
	case m.selected == "":
		s.WriteString("Pick a file:")
	default:
		s.WriteString("Selected file: " + m.fp.Styles.Selected.Render(m.selected))
	}

	s.WriteByte('\n')
	s.WriteString(m.fp.View())

	helpView := m.help.View(m.keys)
	s.WriteString(helpView)
	return s.String()
}
