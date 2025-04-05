// Package theme provides lipgloss style palettes for some nice well known themes, specifically for use in
// the req TUI.
package theme

import "github.com/charmbracelet/lipgloss"

// CatpuccinPalette is the colour palette for a Catpuccin theme.
// See https://catppuccin.com/palette/.
type CatpuccinPalette struct {
	Rosewater lipgloss.Color
	Flamingo  lipgloss.Color
	Pink      lipgloss.Color
	Mauve     lipgloss.Color
	Red       lipgloss.Color
	Maroon    lipgloss.Color
	Peach     lipgloss.Color
	Yellow    lipgloss.Color
	Green     lipgloss.Color
	Teal      lipgloss.Color
	Sky       lipgloss.Color
	Sapphire  lipgloss.Color
	Blue      lipgloss.Color
	Lavender  lipgloss.Color
	Text      lipgloss.Color
	Subtext1  lipgloss.Color
	Subtext0  lipgloss.Color
	Overlay2  lipgloss.Color
	Overlay1  lipgloss.Color
	Overlay0  lipgloss.Color
	Surface2  lipgloss.Color
	Surface1  lipgloss.Color
	Surface0  lipgloss.Color
	Base      lipgloss.Color
	Mantle    lipgloss.Color
	Crust     lipgloss.Color
}

var CatpuccinMacchiato = CatpuccinPalette{
	Rosewater: lipgloss.Color("#f4dbd6"),
	Flamingo:  lipgloss.Color("#f0c6c6"),
	Pink:      lipgloss.Color("#f5bde6"),
	Mauve:     lipgloss.Color("#c6a0f6"),
	Red:       lipgloss.Color("#ed8796"),
	Maroon:    lipgloss.Color("#ee99a0"),
	Peach:     lipgloss.Color("#f5a97f"),
	Yellow:    lipgloss.Color("#eed49f"),
	Green:     lipgloss.Color("#a6da95"),
	Teal:      lipgloss.Color("#8bd5ca"),
	Sky:       lipgloss.Color("#91d7e3"),
	Sapphire:  lipgloss.Color("#7dc4e4"),
	Blue:      lipgloss.Color("#8aadf4"),
	Lavender:  lipgloss.Color("#b7bdf8"),
	Text:      lipgloss.Color("#cad3f5"),
	Subtext1:  lipgloss.Color("#b8c0e0"),
	Subtext0:  lipgloss.Color("#a5adcb"),
	Overlay2:  lipgloss.Color("#939ab7"),
	Overlay1:  lipgloss.Color("#8087a2"),
	Overlay0:  lipgloss.Color("#6e738d"),
	Surface2:  lipgloss.Color("#5b6078"),
	Surface1:  lipgloss.Color("#494d64"),
	Surface0:  lipgloss.Color("#363a4f"),
	Base:      lipgloss.Color("#24273a"),
	Mantle:    lipgloss.Color("#1e2030"),
	Crust:     lipgloss.Color("#181926"),
}
