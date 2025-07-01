// Package list implements a simple bubbletea list component to pick HTTP requests.
package list

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"go.followtheprocess.codes/req/internal/spec"
)

// Model is the list tea Model.
type Model struct {
	l        list.Model // The base list bubble
	selected string     // The name of the selected HTTP request
}

// New returns a new [Model].
func New(title string, requests []spec.Request) Model {
	items := make([]list.Item, 0, len(requests))
	for _, request := range requests {
		items = append(items, request)
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = title

	return Model{
		l: l,
	}
}

// Init helps implement [tea.Model] for [Model].
func (m Model) Init() tea.Cmd {
	return nil
}

// Update updates the UI in response to messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.l.SelectedItem() != nil {
				m.selected = m.l.SelectedItem().FilterValue()
			}

			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.l.SetSize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd

	m.l, cmd = m.l.Update(msg)

	return m, cmd
}

// View renders the UI to the user.
func (m Model) View() string {
	return m.l.View()
}

// Selected returns the picked item from the list.
func (m Model) Selected() string {
	return m.selected
}
