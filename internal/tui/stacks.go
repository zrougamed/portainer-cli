package tui

import (
	"fmt"
	"unicode"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

// capitalize upper-cases the first letter of s.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

type stacksSubview int

const (
	stacksList       stacksSubview = iota
	stacksDeploy                   // compose editor
	stacksContainers               // containers belonging to a stack
)

type StacksModel struct {
	client             *api.Client
	table              table.Model
	stacks             []api.Stack
	loading            bool
	status             string
	subview            stacksSubview
	deployArea         textarea.Model
	deployName         string
	activeEndpointID   int
	activeEndpointName string
	width              int
	height             int

	// stack containers subview
	stackContainersTable  table.Model
	stackContainers       []api.Container
	stackContainerName    string // which stack we're looking at
	stackContainerLoading bool
}

type stacksLoadedMsg struct{ stacks []api.Stack }
type stackActionDoneMsg struct{ msg string }
type stackContainersLoadedMsg struct {
	containers []api.Container
	stackName  string
}

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

	// Containers sub-table
	cCols := []table.Column{
		{Title: "ID", Width: 14},
		{Title: "Name", Width: 28},
		{Title: "Image", Width: 30},
		{Title: "State", Width: 10},
		{Title: "Status", Width: 22},
	}
	ct := table.New(
		table.WithColumns(cCols),
		table.WithFocused(true),
		table.WithHeight(15),
	)
	ct.SetStyles(s)

	ta := textarea.New()
	ta.Placeholder = "Paste your docker-compose.yml here..."
	ta.SetWidth(80)
	ta.SetHeight(20)

	return StacksModel{
		client:               client,
		table:                t,
		stackContainersTable: ct,
		deployArea:           ta,
	}
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

func (m StacksModel) loadStackContainers(stackName string) tea.Cmd {
	endpointID := m.activeEndpointID
	client := m.client
	return func() tea.Msg {
		containers, err := client.ListStackContainers(endpointID, stackName)
		if err != nil {
			return ErrMsg{err}
		}
		return stackContainersLoadedMsg{containers: containers, stackName: stackName}
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

	case stackContainersLoadedMsg:
		m.stackContainerLoading = false
		m.stackContainers = msg.containers
		m.stackContainerName = msg.stackName
		rows := make([]table.Row, len(msg.containers))
		for i, c := range msg.containers {
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
		m.stackContainersTable.SetRows(rows)

	case tea.KeyMsg:
		// ── Stack containers subview ──────────────────────────────────────────
		if m.subview == stacksContainers {
			switch msg.String() {
			case "esc", "q":
				m.subview = stacksList
				return m, nil
			}
			var cmd tea.Cmd
			m.stackContainersTable, cmd = m.stackContainersTable.Update(msg)
			return m, cmd
		}

		// ── Deploy / compose editor subview ───────────────────────────────────
		if m.subview == stacksDeploy {
			switch msg.String() {
			case "esc":
				m.subview = stacksList
				m.deployArea.Blur()
				return m, nil

			case "ctrl+s":
				content := m.deployArea.Value()
				name := m.deployName
				endpointID := m.activeEndpointID
				endpointName := m.activeEndpointName
				if endpointID == 0 {
					return m, func() tea.Msg {
						return ErrMsg{fmt.Errorf("no environment selected — go to Environments first and select one")}
					}
				}
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Deploy stack '%s' to endpoint '%s'?", name, endpointName),
						OnYes: func() tea.Msg {
							_, err := m.client.DeployStack(endpointID, name, content, nil)
							if err != nil {
								return ErrMsg{err}
							}
							return stackActionDoneMsg{"✓ Stack deployed: " + name}
						},
					}
				}
			}
			var cmd tea.Cmd
			m.deployArea, cmd = m.deployArea.Update(msg)
			return m, cmd
		}

		// ── List subview ──────────────────────────────────────────────────────
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadStacks()

		case "n":
			m.subview = stacksDeploy
			m.deployArea.Reset()
			m.deployArea.Focus()
			m.deployName = fmt.Sprintf("stack-%d", len(m.stacks)+1)
			return m, textarea.Blink

		case "s":
			return m, m.stackAction("stop")

		case "S":
			return m, m.stackAction("start")

		case "d":
			return m, m.stackActionDelete()

		case "c": // view stack containers
			if len(m.stacks) == 0 {
				return m, nil
			}
			idx := m.table.Cursor()
			if idx >= len(m.stacks) {
				return m, nil
			}
			stack := m.stacks[idx]
			if m.activeEndpointID == 0 {
				return m, func() tea.Msg {
					return ErrMsg{fmt.Errorf("no environment selected — select an environment first")}
				}
			}
			m.subview = stacksContainers
			m.stackContainerLoading = true
			m.stackContainerName = stack.Name
			return m, m.loadStackContainers(stack.Name)
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
			Prompt: fmt.Sprintf("%s stack '%s'?", capitalize(action), stack.Name),
			OnYes: func() tea.Msg {
				err := m.client.StackAction(stack.ID, action, stack.EndpointID)
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

	// ── Stack containers subview ──────────────────────────────────────────────
	if m.subview == stacksContainers {
		subTitle := HeaderStyle.Render(fmt.Sprintf("📦  Containers for stack: %s", m.stackContainerName))
		if m.stackContainerLoading {
			return lipgloss.JoinVertical(lipgloss.Left, title, subTitle, "  Loading containers...")
		}
		countLabel := SubtitleStyle.Render(fmt.Sprintf("  %d container(s)", len(m.stackContainers)))
		help := HelpStyle.Render("  [↑↓] navigate  [esc] back to stacks")
		return lipgloss.JoinVertical(lipgloss.Left,
			title,
			subTitle,
			countLabel,
			"",
			m.stackContainersTable.View(),
			"",
			help,
		)
	}

	// ── Deploy subview ────────────────────────────────────────────────────────
	if m.subview == stacksDeploy {
		help := HelpStyle.Render("[ctrl+s] deploy  [esc] cancel  (all other keys go to editor)")
		nameLabel := KeyStyle.Render("Stack name: ") + ValueStyle.Render(m.deployName)

		var endpointLabel string
		if m.activeEndpointID == 0 {
			endpointLabel = ErrorStyle.Render("⚠  No environment selected — go to Environments and select one first")
		} else {
			endpointLabel = KeyStyle.Render("Environment: ") + ValueStyle.Render(fmt.Sprintf("%s (id=%d)", m.activeEndpointName, m.activeEndpointID))
		}

		return lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			nameLabel,
			endpointLabel,
			"",
			BoxStyle.Render(m.deployArea.View()),
			"",
			help,
		)
	}

	// ── List subview ──────────────────────────────────────────────────────────
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading stacks...")
	}
	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [n] new  [S] start  [s] stop  [d] delete  [c] containers  [r] refresh  [esc] back")
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
	m.stackContainersTable.SetHeight(h - 10)
	m.deployArea.SetWidth(w - 4)
	m.deployArea.SetHeight(h - 12)

	// Expand name column to fill stacks table
	nameWidth := w - 6 - 10 - 10 - 12 - 10
	if nameWidth < 15 {
		nameWidth = 15
	}
	m.table.SetColumns([]table.Column{
		{Title: "ID", Width: 6},
		{Title: "Name", Width: nameWidth},
		{Title: "Type", Width: 10},
		{Title: "Status", Width: 10},
		{Title: "Endpoint", Width: 12},
	})

	// Expand container name column
	cNameWidth := w - 14 - 30 - 10 - 22 - 10
	if cNameWidth < 15 {
		cNameWidth = 15
	}
	m.stackContainersTable.SetColumns([]table.Column{
		{Title: "ID", Width: 14},
		{Title: "Name", Width: cNameWidth},
		{Title: "Image", Width: 30},
		{Title: "State", Width: 10},
		{Title: "Status", Width: 22},
	})
}
