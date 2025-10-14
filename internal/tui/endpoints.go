package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type EndpointsModel struct {
	client    *api.Client
	table     table.Model
	endpoints []api.Endpoint
	loading   bool
	status    string
	width     int
	height    int
}

type endpointsLoadedMsg struct{ endpoints []api.Endpoint }

func NewEndpointsModel(client *api.Client) EndpointsModel {
	cols := []table.Column{
		{Title: "ID", Width: 6},
		{Title: "Name", Width: 25},
		{Title: "URL", Width: 30},
		{Title: "Status", Width: 10},
		{Title: "Type", Width: 12},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(15),
	)
	s := table.DefaultStyles()
	s.Header = HeaderStyle
	s.Selected = SelectedRowStyle
	t.SetStyles(s)

	return EndpointsModel{client: client, table: t}
}

func (m EndpointsModel) Init() tea.Cmd {
	return m.loadEndpoints()
}

func (m EndpointsModel) loadEndpoints() tea.Cmd {
	return func() tea.Msg {
		eps, err := m.client.ListEndpoints()
		if err != nil {
			return ErrMsg{err}
		}
		return endpointsLoadedMsg{eps}
	}
}

func (m EndpointsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case endpointsLoadedMsg:
		m.endpoints = msg.endpoints
		m.table.SetRows(m.buildRows())
		m.loading = false
		m.status = fmt.Sprintf("%d environment(s)", len(msg.endpoints))

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadEndpoints()
		case "enter":
			if len(m.endpoints) > 0 {
				idx := m.table.Cursor()
				if idx < len(m.endpoints) {
					ep := m.endpoints[idx]
					return m, func() tea.Msg {
						return EndpointSelectedMsg{Endpoint: ep}
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m EndpointsModel) buildRows() []table.Row {
	rows := make([]table.Row, len(m.endpoints))
	for i, ep := range m.endpoints {
		status := InactiveStyle.Render("●  Down")
		if ep.Status == 1 {
			status = ActiveStyle.Render("●  Up")
		}
		epType := endpointType(ep.Type)
		rows[i] = table.Row{
			fmt.Sprintf("%d", ep.ID),
			ep.Name,
			ep.URL,
			status,
			epType,
		}
	}
	return rows
}

func endpointType(t int) string {
	switch t {
	case 1:
		return "Docker"
	case 2:
		return "Agent"
	case 3:
		return "Azure ACI"
	case 4:
		return "Edge Agent"
	case 5:
		return "Local"
	default:
		return fmt.Sprintf("Type %d", t)
	}
}

func (m EndpointsModel) View() string {
	title := HeaderStyle.Render("🌐  Environments")
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading environments...")
	}
	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [enter] select endpoint  [r] refresh  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		status,
		"",
		m.table.View(),
		"",
		help,
	)
}

func (m *EndpointsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
}
