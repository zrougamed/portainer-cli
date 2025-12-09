package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type volumesSubview int

const (
	volumesList   volumesSubview = iota
	volumesCreate                // inline creation form
)

type volumesCreateField int

const (
	volFieldName   volumesCreateField = iota
	volFieldDriver                    // local | nfs | etc.
	volFieldCount
)

type VolumesModel struct {
	client     *api.Client
	table      table.Model
	volumes    []api.Volume
	endpointID int
	loading    bool
	status     string
	subview    volumesSubview

	createInputs [volFieldCount]textinput.Model
	createFocus  volumesCreateField

	width  int
	height int
}

type volumesLoadedMsg struct {
	volumes    []api.Volume
	endpointID int
}
type volumeActionDoneMsg struct{ msg string }

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

	nameIn := textinput.New()
	nameIn.Placeholder = "my-volume"
	nameIn.Focus()
	nameIn.Width = 40

	driverIn := textinput.New()
	driverIn.Placeholder = "local"
	driverIn.SetValue("local")
	driverIn.Width = 20

	inputs := [volFieldCount]textinput.Model{nameIn, driverIn}

	return VolumesModel{
		client:       client,
		table:        t,
		createInputs: inputs,
	}
}

func (m VolumesModel) LoadVolumes(endpointID int) tea.Cmd {
	return func() tea.Msg {
		volumes, err := m.client.ListVolumes(endpointID)
		if err != nil {
			return ErrMsg{err}
		}
		return volumesLoadedMsg{volumes: volumes, endpointID: endpointID}
	}
}

func (m *VolumesModel) refocusCreate() {
	for i := range m.createInputs {
		if volumesCreateField(i) == m.createFocus {
			m.createInputs[i].Focus()
		} else {
			m.createInputs[i].Blur()
		}
	}
}

func (m VolumesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case volumesLoadedMsg:
		m.endpointID = msg.endpointID
		m.volumes = msg.volumes
		m.table.SetRows(m.buildRows())
		m.loading = false
		m.status = fmt.Sprintf("%d volume(s)", len(m.volumes))

	case volumeActionDoneMsg:
		m.status = msg.msg
		m.subview = volumesList
		return m, m.LoadVolumes(m.endpointID)

	case tea.KeyMsg:
		// ── Create form ──────────────────────────────────────────────────────
		if m.subview == volumesCreate {
			switch msg.String() {
			case "esc":
				m.subview = volumesList
				return m, nil
			case "tab", "down":
				m.createFocus = (m.createFocus + 1) % volFieldCount
				m.refocusCreate()
				return m, textinput.Blink
			case "shift+tab", "up":
				m.createFocus = (m.createFocus + volFieldCount - 1) % volFieldCount
				m.refocusCreate()
				return m, textinput.Blink
			case "ctrl+s", "enter":
				if msg.String() == "ctrl+s" || m.createFocus == volFieldDriver {
					return m, m.submitCreate()
				}
				m.createFocus = (m.createFocus + 1) % volFieldCount
				m.refocusCreate()
				return m, textinput.Blink
			}
			var cmd tea.Cmd
			m.createInputs[m.createFocus], cmd = m.createInputs[m.createFocus].Update(msg)
			return m, cmd
		}

		// ── List view ────────────────────────────────────────────────────────
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.LoadVolumes(m.endpointID)

		case "n": // new volume
			m.subview = volumesCreate
			m.createFocus = volFieldName
			for i := range m.createInputs {
				m.createInputs[i].SetValue("")
			}
			m.createInputs[volFieldDriver].SetValue("local")
			m.refocusCreate()
			return m, textinput.Blink

		case "d", "D": // delete (D = force)
			if len(m.volumes) == 0 {
				return m, nil
			}
			idx := m.table.Cursor()
			if idx >= len(m.volumes) {
				return m, nil
			}
			vol := m.volumes[idx]
			force := msg.String() == "D"
			endpointID := m.endpointID
			confirmMsg := fmt.Sprintf("Delete volume '%s'?", vol.Name)
			if force {
				confirmMsg = fmt.Sprintf("FORCE delete volume '%s'? (even if in use)", vol.Name)
			}
			return m, func() tea.Msg {
				return ConfirmMsg{
					Prompt: confirmMsg,
					OnYes: func() tea.Msg {
						if err := m.client.DeleteVolume(endpointID, vol.Name, force); err != nil {
							return ErrMsg{err}
						}
						return volumeActionDoneMsg{"✓ Deleted volume: " + vol.Name}
					},
				}
			}
		}
	}

	var cmd tea.Cmd
	if m.subview == volumesList {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m VolumesModel) submitCreate() tea.Cmd {
	name := strings.TrimSpace(m.createInputs[volFieldName].Value())
	driver := strings.TrimSpace(m.createInputs[volFieldDriver].Value())
	endpointID := m.endpointID

	if name == "" {
		return func() tea.Msg { return ErrMsg{fmt.Errorf("volume name is required")} }
	}
	if driver == "" {
		driver = "local"
	}

	return func() tea.Msg {
		req := api.CreateVolumeRequest{
			Name:   name,
			Driver: driver,
		}
		if err := m.client.CreateVolume(endpointID, req); err != nil {
			return ErrMsg{err}
		}
		return volumeActionDoneMsg{"✓ Created volume: " + name}
	}
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

	if m.subview == volumesCreate {
		return m.viewCreateForm()
	}

	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading volumes...")
	}
	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [n] new volume  [d] delete  [D] force-delete  [r] refresh  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left, title, status, "", m.table.View(), "", help)
}

func (m VolumesModel) viewCreateForm() string {
	title := HeaderStyle.Render("💾  Create Volume")

	label := func(s string, active bool) string {
		if active {
			return KeyStyle.Render(s)
		}
		return SubtitleStyle.Render(s)
	}

	form := lipgloss.JoinVertical(lipgloss.Left,
		label("  Name:", m.createFocus == volFieldName),
		"  "+m.createInputs[volFieldName].View(),
		"",
		label("  Driver (local | nfs | ...):", m.createFocus == volFieldDriver),
		"  "+m.createInputs[volFieldDriver].View(),
		"",
		HelpStyle.Render("  [tab/↓] next  [shift+tab/↑] prev  [ctrl+s] create  [esc] cancel"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, "", form)
}

func (m VolumesModel) Init() tea.Cmd {
	return nil
}

func (m *VolumesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
	for i := range m.createInputs {
		m.createInputs[i].Width = w - 10
		if m.createInputs[i].Width < 20 {
			m.createInputs[i].Width = 20
		}
	}
}
