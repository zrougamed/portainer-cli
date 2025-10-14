package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type VolumesModel struct {
	client     *api.Client
	table      table.Model
	volumes    []api.Volume
	endpointID int
	loading    bool
	status     string
	width      int
	height     int
}

type volumesLoadedMsg struct{ volumes []api.Volume }

func NewVolumesModel(client *api.Client) VolumesModel {
	cols := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Driver", Width: 12},
		{Title: "Scope", Width: 10},
		{Title: "Mountpoint", Width: 40},
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

	return VolumesModel{client: client, table: t}
}

func (m VolumesModel) LoadVolumes(endpointID int) tea.Cmd {
	m.endpointID = endpointID
	return func() tea.Msg {
		volumes, err := m.client.ListVolumes(endpointID)
		if err != nil {
			return ErrMsg{err}
		}
		return volumesLoadedMsg{volumes}
	}
}

func (m VolumesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case volumesLoadedMsg:
		m.volumes = msg.volumes
		m.table.SetRows(m.buildRows())
		m.loading = false
		m.status = fmt.Sprintf("%d volume(s)", len(m.volumes))

	case tea.KeyMsg:
		if msg.String() == "r" {
			m.loading = true
			return m, m.LoadVolumes(m.endpointID)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m VolumesModel) buildRows() []table.Row {
	rows := make([]table.Row, len(m.volumes))
	for i, v := range m.volumes {
		name := v.Name
		if len(name) > 28 {
			name = name[:25] + "..."
		}
		mount := v.Mountpoint
		if len(mount) > 38 {
			mount = mount[:35] + "..."
		}
		rows[i] = table.Row{name, v.Driver, v.Scope, mount}
	}
	return rows
}

func (m VolumesModel) View() string {
	title := HeaderStyle.Render("💾  Volumes")
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading volumes...")
	}
	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [r] refresh  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left, title, status, "", m.table.View(), "", help)
}

func (m VolumesModel) Init() tea.Cmd {
	return nil
}

func (m *VolumesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
}
