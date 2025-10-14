package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type LogsModel struct {
	client      *api.Client
	viewport    viewport.Model
	containerID string
	name        string
	endpointID  int
	loading     bool
	tail        int
	width       int
	height      int
}

type logsLoadedMsg struct{ content string }

func NewLogsModel(client *api.Client) LogsModel {
	vp := viewport.New(80, 30)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1)

	return LogsModel{
		client:   client,
		viewport: vp,
		tail:     200,
	}
}

func (m LogsModel) Load(endpointID int, containerID, name string) tea.Cmd {
	m.endpointID = endpointID
	m.containerID = containerID
	m.name = name
	m.loading = true
	return func() tea.Msg {
		logs, err := m.client.ContainerLogs(endpointID, containerID, m.tail)
		if err != nil {
			return ErrMsg{err}
		}
		return logsLoadedMsg{logs}
	}
}

func (m LogsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logsLoadedMsg:
		m.loading = false
		// Strip docker log stream header bytes (first 8 bytes of each line)
		cleaned := stripDockerLogHeaders(msg.content)
		m.viewport.SetContent(cleaned)
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, m.Load(m.endpointID, m.containerID, m.name)
		case "+":
			m.tail *= 2
			return m, m.Load(m.endpointID, m.containerID, m.name)
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// stripDockerLogHeaders removes the 8-byte multiplexed stream header Docker adds
func stripDockerLogHeaders(raw string) string {
	var sb strings.Builder
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		if len(line) > 8 {
			// Check if it looks like a Docker log header
			b := line[0]
			if b == 1 || b == 2 { // stdout=1, stderr=2
				line = line[8:]
			}
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m LogsModel) View() string {
	title := HeaderStyle.Render(fmt.Sprintf("📋  Logs: %s", m.name))
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Fetching logs...")
	}
	info := SubtitleStyle.Render(fmt.Sprintf("  Last %d lines  %d%%",
		m.tail, int(m.viewport.ScrollPercent()*100)))
	help := HelpStyle.Render("  [↑↓/pgup/pgdn] scroll  [r] refresh  [+] more lines  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		info,
		m.viewport.View(),
		help,
	)
}

func (m LogsModel) Init() tea.Cmd {
	return nil
}

func (m *LogsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w - 4
	m.viewport.Height = h - 6
}
