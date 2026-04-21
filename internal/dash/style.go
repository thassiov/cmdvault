// Package dash implements the persistent TUI dashboard.
package dash

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent = lipgloss.Color("#7aa2f7")
	colorDim    = lipgloss.Color("240")
	colorMuted  = lipgloss.Color("244")
	colorBg     = lipgloss.Color("235")
	colorWarn   = lipgloss.Color("#e0af68") // yellow
	colorOK     = lipgloss.Color("#9ece6a") // green

	styleTopBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(colorBg).
			Padding(0, 1).
			Bold(true)

	styleStatusLine = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorBg).
			Padding(0, 1)

	styleOutputPane = lipgloss.NewStyle().
			Padding(0, 1)

	stylePickerPane = lipgloss.NewStyle().
			Padding(0, 1).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorDim)

	stylePlaceholder = lipgloss.NewStyle().
				Foreground(colorDim).
				Italic(true)
)
