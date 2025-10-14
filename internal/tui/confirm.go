package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmModel struct {
	prompt string
	onYes  tea.Cmd
}

func NewConfirmModel(prompt string, onYes tea.Cmd) ConfirmModel {
	return ConfirmModel{prompt: prompt, onYes: onYes}
}

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: true} }
		case "n", "N", "esc":
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: false} }
		}
	}
	return m, nil
}

func (m ConfirmModel) Init() tea.Cmd {
	return nil
}

func (m ConfirmModel) View() string {
	box := BoxStyle.Copy().
		BorderForeground(colorWarning).
		Padding(1, 3).
		Render(
			lipgloss.JoinVertical(lipgloss.Center,
				KeyStyle.Render(m.prompt),
				"",
				lipgloss.JoinHorizontal(lipgloss.Center,
					ActiveStyle.Copy().
						Background(colorSuccess).
						Foreground(colorFg).
						Padding(0, 2).
						MarginRight(4).
						Render("  [Y] Yes  "),
					StoppedStyle.Copy().
						Background(colorDanger).
						Foreground(colorFg).
						Padding(0, 2).
						Render("  [N] No  "),
				),
			),
		)
	return "\n\n" + lipgloss.NewStyle().Width(60).Align(lipgloss.Center).Render(box)
}
