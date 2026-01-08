package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

// EnvPickerModel is a lightweight overlay that lists available endpoints and
// lets the user pick one when they navigate to a screen that requires an
// active environment.  After selection it sets the endpoint then re-dispatches
// the original NavigateMsg that triggered the picker.
type EnvPickerModel struct {
	client    *api.Client
	table     table.Model
	endpoints []api.Endpoint
	loading   bool
	// pendingNav is the NavigateMsg we will replay after the user selects.
	pendingNav NavigateMsg
	width      int
	height     int
}

// envPickerLoadedMsg carries the endpoint list once fetched.
type envPickerLoadedMsg struct{ endpoints []api.Endpoint }

// EnvPickerSelectedMsg is handled in app.go: it sets the active endpoint and
// immediately processes the pending navigation.
type EnvPickerSelectedMsg struct {
	Endpoint   api.Endpoint
	PendingNav NavigateMsg
}

func NewEnvPickerModel(client *api.Client, pending NavigateMsg) EnvPickerModel {
	cols := []table.Column{
		{Title: "ID", Width: 5},
		{Title: "Name", Width: 30},
		{Title: "URL", Width: 30},
		{Title: "Status", Width: 8},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	s := table.DefaultStyles()
	s.Header = HeaderStyle
	s.Selected = SelectedRowStyle
	t.SetStyles(s)
	return EnvPickerModel{
		client:     client,
		table:      t,
		pendingNav: pending,
		loading:    true,
	}
}

func (m EnvPickerModel) Init() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		eps, err := client.ListEndpoints()
		if err != nil {
			return ErrMsg{err}
		}
		return envPickerLoadedMsg{eps}
	}
}

func (m EnvPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case envPickerLoadedMsg:
		m.endpoints = msg.endpoints
		m.loading = false
		rows := make([]table.Row, len(msg.endpoints))
		for i, ep := range msg.endpoints {
			status := InactiveStyle.Render("Down")
			if ep.Status == 1 {
				status = ActiveStyle.Render("Up")
			}
			rows[i] = table.Row{
				fmt.Sprintf("%d", ep.ID),
				ep.Name,
				ep.URL,
				status,
			}
		}
		m.table.SetRows(rows)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if len(m.endpoints) > 0 {
				idx := m.table.Cursor()
				if idx < len(m.endpoints) {
					ep := m.endpoints[idx]
					pending := m.pendingNav
					return m, func() tea.Msg {
						return EnvPickerSelectedMsg{Endpoint: ep, PendingNav: pending}
					}
				}
			}
		case "esc":
			return m, func() tea.Msg { return NavigateMsg{Screen: ScreenDashboard} }
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m EnvPickerModel) View() string {
	title := HeaderStyle.Render("🌐  Select Environment")
	hint := SubtitleStyle.Render("  No environment selected — pick one to continue")
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, hint, "", "  Loading environments...")
	}
	help := HelpStyle.Render("  [↑↓] navigate  [enter] select  [esc] cancel")
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		hint,
		"",
		m.table.View(),
		"",
		help,
	)
}

func (m *EnvPickerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
}
