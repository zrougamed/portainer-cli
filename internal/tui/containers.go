package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type ContainersModel struct {
	client     *api.Client
	table      table.Model
	containers []api.Container
	endpointID int
	loading    bool
	status     string
	showAll    bool
	width      int
	height     int
}

type containersLoadedMsg struct {
	containers []api.Container
	endpointID int
}
type containerActionDoneMsg struct{ action, containerID string }

func NewContainersModel(client *api.Client) ContainersModel {
	cols := []table.Column{
		{Title: "ID", Width: 14},
		{Title: "Name", Width: 28},
		{Title: "Image", Width: 30},
		{Title: "State", Width: 10},
		{Title: "Status", Width: 22},
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

	return ContainersModel{client: client, table: t, showAll: true}
}

func (m ContainersModel) LoadContainers(endpointID int) tea.Cmd {
	showAll := m.showAll
	return func() tea.Msg {
		containers, err := m.client.ListContainers(endpointID, showAll)
		if err != nil {
			return ErrMsg{err}
		}
		return containersLoadedMsg{containers: containers, endpointID: endpointID}
	}
}

func (m ContainersModel) selectedContainer() (api.Container, bool) {
	if len(m.containers) == 0 {
		return api.Container{}, false
	}
	idx := m.table.Cursor()
	if idx >= len(m.containers) {
		return api.Container{}, false
	}
	return m.containers[idx], true
}

func (m ContainersModel) doAction(action string) tea.Cmd {
	c, ok := m.selectedContainer()
	if !ok {
		return nil
	}
	endpointID := m.endpointID
	return func() tea.Msg {
		err := m.client.ContainerAction(endpointID, c.ID, action)
		if err != nil {
			return ErrMsg{err}
		}
		return containerActionDoneMsg{action: action, containerID: c.ID}
	}
}

func (m ContainersModel) doDelete(force bool) tea.Cmd {
	c, ok := m.selectedContainer()
	if !ok {
		return nil
	}
	endpointID := m.endpointID
	return func() tea.Msg {
		err := m.client.DeleteContainer(endpointID, c.ID, force, false)
		if err != nil {
			return ErrMsg{err}
		}
		return containerActionDoneMsg{action: "delete", containerID: c.ID}
	}
}

func (m ContainersModel) doRecreate() tea.Cmd {
	c, ok := m.selectedContainer()
	if !ok {
		return nil
	}
	endpointID := m.endpointID
	return func() tea.Msg {
		err := m.client.RecreateContainer(endpointID, c.ID, true)
		if err != nil {
			return ErrMsg{err}
		}
		return containerActionDoneMsg{action: "recreate", containerID: c.ID}
	}
}

func (m ContainersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case containersLoadedMsg:
		m.endpointID = msg.endpointID
		m.containers = msg.containers
		m.table.SetRows(m.buildRows())
		m.loading = false
		running := 0
		for _, c := range m.containers {
			if c.State == "running" {
				running++
			}
		}
		m.status = fmt.Sprintf("%d containers (%d running)", len(m.containers), running)

	case containerActionDoneMsg:
		m.status = fmt.Sprintf("✓ %s: %s", msg.action, msg.containerID[:12])
		return m, m.LoadContainers(m.endpointID)

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.LoadContainers(m.endpointID)

		case "a":
			m.showAll = !m.showAll
			return m, m.LoadContainers(m.endpointID)

		case "s": // stop
			c, ok := m.selectedContainer()
			if ok {
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Stop container %s?", containerName(c)),
						OnYes:  m.doAction("stop"),
					}
				}
			}

		case "S": // start
			c, ok := m.selectedContainer()
			if ok {
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Start container %s?", containerName(c)),
						OnYes:  m.doAction("start"),
					}
				}
			}

		case "R": // restart
			c, ok := m.selectedContainer()
			if ok {
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Restart container %s?", containerName(c)),
						OnYes:  m.doAction("restart"),
					}
				}
			}

		case "p": // pause / unpause toggle
			c, ok := m.selectedContainer()
			if ok {
				if c.State == "paused" {
					return m, func() tea.Msg {
						return ConfirmMsg{
							Prompt: fmt.Sprintf("Unpause container %s?", containerName(c)),
							OnYes:  m.doAction("unpause"),
						}
					}
				}
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Pause container %s?", containerName(c)),
						OnYes:  m.doAction("pause"),
					}
				}
			}

		case "e": // recreate (pull + remove + run)
			c, ok := m.selectedContainer()
			if ok {
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Recreate container %s (pulls latest image)?", containerName(c)),
						OnYes:  m.doRecreate(),
					}
				}
			}

		case "D": // delete (shift+d)
			c, ok := m.selectedContainer()
			if ok {
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("DELETE container %s? This is irreversible!", containerName(c)),
						OnYes:  m.doDelete(false),
					}
				}
			}

		case "ctrl+d": // force delete
			c, ok := m.selectedContainer()
			if ok {
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("FORCE DELETE container %s (even if running)?", containerName(c)),
						OnYes:  m.doDelete(true),
					}
				}
			}

		case "l", "enter": // logs
			c, ok := m.selectedContainer()
			if ok {
				return m, func() tea.Msg {
					return ShowLogsMsg{
						EndpointID:  m.endpointID,
						ContainerID: c.ID,
						Name:        containerName(c),
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func containerName(c api.Container) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	return c.ID[:12]
}

func (m ContainersModel) buildRows() []table.Row {
	rows := make([]table.Row, len(m.containers))
	for i, c := range m.containers {
		shortID := c.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		name := containerName(c)
		image := c.Image
		if len(image) > 28 {
			image = image[:25] + "..."
		}
		state := StateStyle(c.State).Render(c.State)
		rows[i] = table.Row{shortID, name, image, state, c.Status}
	}
	return rows
}

func (m ContainersModel) Init() tea.Cmd {
	return nil
}

func (m ContainersModel) View() string {
	title := HeaderStyle.Render("📦  Containers")
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading containers...")
	}
	allFlag := ""
	if m.showAll {
		allFlag = " (all)"
	}
	status := SubtitleStyle.Render("  " + m.status + allFlag)
	help := HelpStyle.Render(
		"  [l/↵] logs  [S] start  [s] stop  [R] restart  [p] pause/unpause" +
			"  [e] recreate  [D] delete  [ctrl+d] force-delete  [a] toggle-all  [r] refresh  [esc] back",
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		status,
		"",
		m.table.View(),
		"",
		help,
	)
}

func (m *ContainersModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
}
