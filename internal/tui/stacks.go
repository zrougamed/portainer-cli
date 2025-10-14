package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type stacksSubview int

const (
	stacksList stacksSubview = iota
	stacksDeploy
)

type StacksModel struct {
	client     *api.Client
	table      table.Model
	stacks     []api.Stack
	loading    bool
	status     string
	subview    stacksSubview
	deployArea textarea.Model
	deployName string
	width      int
	height     int
}

type stacksLoadedMsg struct{ stacks []api.Stack }
type stackActionDoneMsg struct{ msg string }

func NewStacksModel(client *api.Client) StacksModel {
	cols := []table.Column{
		{Title: "ID", Width: 6},
		{Title: "Name", Width: 25},
		{Title: "Type", Width: 10},
		{Title: "Status", Width: 10},
		{Title: "Endpoint", Width: 12},
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

	ta := textarea.New()
	ta.Placeholder = "Paste your docker-compose.yml here..."
	ta.SetWidth(80)
	ta.SetHeight(20)

	return StacksModel{client: client, table: t, deployArea: ta}
}

func (m StacksModel) Init() tea.Cmd {
	return m.loadStacks()
}

func (m StacksModel) loadStacks() tea.Cmd {
	return func() tea.Msg {
		stacks, err := m.client.ListStacks()
		if err != nil {
			return ErrMsg{err}
		}
		return stacksLoadedMsg{stacks}
	}
}

func (m StacksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stacksLoadedMsg:
		m.stacks = msg.stacks
		m.table.SetRows(m.buildRows())
		m.loading = false
		m.status = fmt.Sprintf("%d stack(s)", len(m.stacks))

	case stackActionDoneMsg:
		m.status = msg.msg
		m.subview = stacksList
		return m, m.loadStacks()

	case tea.KeyMsg:
		if m.subview == stacksDeploy {
			switch msg.String() {
			case "esc":
				m.subview = stacksList
				return m, nil
			case "ctrl+s":
				content := m.deployArea.Value()
				name := m.deployName
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Deploy stack '%s'?", name),
						OnYes: func() tea.Msg {
							// endpointID 1 as default - in production would pick from active endpoint
							_, err := m.client.DeployStack(1, name, content, nil)
							if err != nil {
								return ErrMsg{err}
							}
							return stackActionDoneMsg{"Stack deployed: " + name}
						},
					}
				}
			}
			var cmd tea.Cmd
			m.deployArea, cmd = m.deployArea.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadStacks()
		case "n":
			m.subview = stacksDeploy
			m.deployArea.Reset()
			m.deployArea.Focus()
			m.deployName = fmt.Sprintf("stack-%d", len(m.stacks)+1)
			return m, nil
		case "s":
			return m, m.stackAction("stop")
		case "S":
			return m, m.stackAction("start")
		case "d":
			return m, m.stackActionDelete()
		}
	}

	var cmd tea.Cmd
	if m.subview == stacksList {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m StacksModel) stackAction(action string) tea.Cmd {
	idx := m.table.Cursor()
	if idx >= len(m.stacks) {
		return nil
	}
	stack := m.stacks[idx]
	return func() tea.Msg {
		return ConfirmMsg{
			Prompt: fmt.Sprintf("%s stack '%s'?", action, stack.Name),
			OnYes: func() tea.Msg {
				err := m.client.StackAction(stack.ID, action)
				if err != nil {
					return ErrMsg{err}
				}
				return stackActionDoneMsg{fmt.Sprintf("✓ Stack %s: %s", action, stack.Name)}
			},
		}
	}
}

func (m StacksModel) stackActionDelete() tea.Cmd {
	idx := m.table.Cursor()
	if idx >= len(m.stacks) {
		return nil
	}
	stack := m.stacks[idx]
	return func() tea.Msg {
		return ConfirmMsg{
			Prompt: fmt.Sprintf("DELETE stack '%s'? This is irreversible!", stack.Name),
			OnYes: func() tea.Msg {
				err := m.client.DeleteStack(stack.ID, stack.EndpointID)
				if err != nil {
					return ErrMsg{err}
				}
				return stackActionDoneMsg{"✓ Deleted: " + stack.Name}
			},
		}
	}
}

func (m StacksModel) buildRows() []table.Row {
	rows := make([]table.Row, len(m.stacks))
	for i, s := range m.stacks {
		stackType := "Compose"
		if s.Type == 1 {
			stackType = "Swarm"
		}
		status := InactiveStyle.Render("Inactive")
		if s.Status == 1 {
			status = ActiveStyle.Render("Active")
		}
		rows[i] = table.Row{
			fmt.Sprintf("%d", s.ID),
			s.Name,
			stackType,
			status,
			fmt.Sprintf("%d", s.EndpointID),
		}
	}
	return rows
}

func (m StacksModel) View() string {
	title := HeaderStyle.Render("📚  Stacks")

	if m.subview == stacksDeploy {
		help := HelpStyle.Render("[ctrl+s] deploy  [esc] cancel")
		nameLabel := KeyStyle.Render("Stack name: ") + ValueStyle.Render(m.deployName)
		return lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			nameLabel,
			"",
			BoxStyle.Render(m.deployArea.View()),
			"",
			help,
		)
	}

	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading stacks...")
	}
	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [n] new stack  [S] start  [s] stop  [d] delete  [r] refresh  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		status,
		"",
		m.table.View(),
		"",
		help,
	)
}

func (m *StacksModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
	m.deployArea.SetWidth(w - 4)
	m.deployArea.SetHeight(h - 12)
}
