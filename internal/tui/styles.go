package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary  = lipgloss.Color("#00BCD4") // Portainer cyan
	colorAccent   = lipgloss.Color("#26C6DA")
	colorSuccess  = lipgloss.Color("#66BB6A")
	colorWarning  = lipgloss.Color("#FFA726")
	colorDanger   = lipgloss.Color("#EF5350")
	colorMuted    = lipgloss.Color("#546E7A")
	colorFg       = lipgloss.Color("#ECEFF1")
	colorBg       = lipgloss.Color("#263238")
	colorSelected = lipgloss.Color("#004D61")

	// Styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	StatusBarStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorMuted).
			Padding(0, 1)

	SelectedRowStyle = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorFg).
				Bold(true)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorMuted)

	KeyStyle   = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	ValueStyle = lipgloss.NewStyle().Foreground(colorFg)

	RunningStyle  = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	StoppedStyle  = lipgloss.NewStyle().Foreground(colorDanger)
	PausedStyle   = lipgloss.NewStyle().Foreground(colorWarning)
	ActiveStyle   = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	InactiveStyle = lipgloss.NewStyle().Foreground(colorMuted)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().Foreground(colorMuted)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)
)

func StateStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return RunningStyle
	case "exited", "dead":
		return StoppedStyle
	case "paused":
		return PausedStyle
	default:
		return ValueStyle
	}
}
