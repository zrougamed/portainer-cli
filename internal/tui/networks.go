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

type networksSubview int

const (
	networksList   networksSubview = iota
	networksCreate                 // inline creation form
)

// networksCreateField tracks which input is focused during creation.
type networksCreateField int

const (
	netFieldName    networksCreateField = iota
	netFieldDriver                      // bridge | host | overlay | macvlan | none
	netFieldSubnet                      // optional CIDR e.g. 10.10.0.0/24
	netFieldGateway                     // optional e.g. 10.10.0.1
	netFieldCount
)

type NetworksModel struct {
	client     *api.Client
	table      table.Model
	networks   []api.Network
	endpointID int
	loading    bool
	status     string
	subview    networksSubview

	// Create form
	createInputs   [netFieldCount]textinput.Model
	createFocus    networksCreateField
	createInternal bool

	width  int
	height int
}

type networksLoadedMsg struct {
	networks   []api.Network
	endpointID int
}
type networkActionDoneMsg struct{ msg string }

func NewNetworksModel(client *api.Client) NetworksModel {
	cols := []table.Column{
		{Title: "ID", Width: 14},
		{Title: "Name", Width: 25},
		{Title: "Driver", Width: 10},
		{Title: "Scope", Width: 8},
		{Title: "Subnet", Width: 18},
		{Title: "Internal", Width: 9},
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

	// Build create-form inputs
	nameIn := textinput.New()
	nameIn.Placeholder = "my-network"
	nameIn.Focus()
	nameIn.Width = 40

	driverIn := textinput.New()
	driverIn.Placeholder = "bridge"
	driverIn.SetValue("bridge")
	driverIn.Width = 20

	subnetIn := textinput.New()
	subnetIn.Placeholder = "172.20.0.0/24  (optional)"
	subnetIn.Width = 40

	gatewayIn := textinput.New()
	gatewayIn.Placeholder = "172.20.0.1  (optional)"
	gatewayIn.Width = 40

	inputs := [netFieldCount]textinput.Model{nameIn, driverIn, subnetIn, gatewayIn}

	return NetworksModel{
		client:       client,
		table:        t,
		createInputs: inputs,
	}
}

func (m NetworksModel) Init() tea.Cmd {
	return nil
}

func (m NetworksModel) LoadNetworks(endpointID int) tea.Cmd {
	return func() tea.Msg {
		nets, err := m.client.ListNetworks(endpointID)
		if err != nil {
			return ErrMsg{err}
		}
		return networksLoadedMsg{networks: nets, endpointID: endpointID}
	}
}

func (m *NetworksModel) refocusCreate() {
	for i := range m.createInputs {
		if networksCreateField(i) == m.createFocus {
			m.createInputs[i].Focus()
		} else {
			m.createInputs[i].Blur()
		}
	}
}

func (m NetworksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case networksLoadedMsg:
		m.endpointID = msg.endpointID
		m.networks = msg.networks
		m.table.SetRows(m.buildRows())
		m.loading = false
		m.status = fmt.Sprintf("%d network(s)", len(m.networks))

	case networkActionDoneMsg:
		m.status = msg.msg
		m.subview = networksList
		return m, m.LoadNetworks(m.endpointID)

	case tea.KeyMsg:
		// ── Create form ──────────────────────────────────────────────────────
		if m.subview == networksCreate {
			switch msg.String() {
			case "esc":
				m.subview = networksList
				return m, nil
			case "tab", "down":
				m.createFocus = (m.createFocus + 1) % netFieldCount
				m.refocusCreate()
				return m, textinput.Blink
			case "shift+tab", "up":
				m.createFocus = (m.createFocus + netFieldCount - 1) % netFieldCount
				m.refocusCreate()
				return m, textinput.Blink
			case "ctrl+i": // toggle internal
				m.createInternal = !m.createInternal
				return m, nil
			case "ctrl+s", "enter":
				// Only submit when all required fields are filled
				if msg.String() == "ctrl+s" || m.createFocus == netFieldGateway {
					return m, m.submitCreate()
				}
				// Move to next field on enter
				m.createFocus = (m.createFocus + 1) % netFieldCount
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
			return m, m.LoadNetworks(m.endpointID)

		case "n": // new network
			m.subview = networksCreate
			m.createFocus = netFieldName
			m.createInternal = false
			for i := range m.createInputs {
				m.createInputs[i].SetValue("")
			}
			m.createInputs[netFieldDriver].SetValue("bridge")
			m.refocusCreate()
			return m, textinput.Blink

		case "d", "D": // delete network
			if len(m.networks) == 0 {
				return m, nil
			}
			idx := m.table.Cursor()
			if idx >= len(m.networks) {
				return m, nil
			}
			net := m.networks[idx]
			// Protect built-in networks
			if isBuiltinNetwork(net.Name) {
				return m, func() tea.Msg {
					return ErrMsg{fmt.Errorf("cannot delete built-in network: %s", net.Name)}
				}
			}
			endpointID := m.endpointID
			return m, func() tea.Msg {
				return ConfirmMsg{
					Prompt: fmt.Sprintf("DELETE network '%s'? Containers will lose connectivity!", net.Name),
					OnYes: func() tea.Msg {
						if err := m.client.DeleteNetwork(endpointID, net.ID); err != nil {
							return ErrMsg{err}
						}
						return networkActionDoneMsg{"✓ Deleted network: " + net.Name}
					},
				}
			}
		}
	}

	var cmd tea.Cmd
	if m.subview == networksList {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m NetworksModel) submitCreate() tea.Cmd {
	name := strings.TrimSpace(m.createInputs[netFieldName].Value())
	driver := strings.TrimSpace(m.createInputs[netFieldDriver].Value())
	subnet := strings.TrimSpace(m.createInputs[netFieldSubnet].Value())
	gateway := strings.TrimSpace(m.createInputs[netFieldGateway].Value())
	internal := m.createInternal
	endpointID := m.endpointID

	if name == "" {
		return func() tea.Msg { return ErrMsg{fmt.Errorf("network name is required")} }
	}
	if driver == "" {
		driver = "bridge"
	}

	return func() tea.Msg {
		req := api.CreateNetworkRequest{
			Name:     name,
			Driver:   driver,
			Internal: internal,
		}
		if subnet != "" || gateway != "" {
			req.IPAM = &api.NetworkIPAM{
				Driver: "default",
			}
			if subnet != "" || gateway != "" {
				conf := api.NetworkIPAMConf{Subnet: subnet, Gateway: gateway}
				req.IPAM.Config = []api.NetworkIPAMConf{conf}
			}
		}
		if err := m.client.CreateNetwork(endpointID, req); err != nil {
			return ErrMsg{err}
		}
		return networkActionDoneMsg{"✓ Created network: " + name}
	}
}

func isBuiltinNetwork(name string) bool {
	switch name {
	case "bridge", "host", "none":
		return true
	}
	return false
}

func (m NetworksModel) buildRows() []table.Row {
	rows := make([]table.Row, len(m.networks))
	for i, n := range m.networks {
		shortID := n.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		subnet := ""
		if len(n.IPAM.Config) > 0 {
			subnet = n.IPAM.Config[0].Subnet
		}
		internal := ""
		if n.Internal {
			internal = ActiveStyle.Render("yes")
		} else {
			internal = InactiveStyle.Render("no")
		}
		rows[i] = table.Row{shortID, n.Name, n.Driver, n.Scope, subnet, internal}
	}
	return rows
}

func (m NetworksModel) View() string {
	title := HeaderStyle.Render("🔗  Networks")

	if m.subview == networksCreate {
		return m.viewCreateForm()
	}

	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading networks...")
	}

	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [n] new network  [d/D] delete  [r] refresh  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		status,
		"",
		m.table.View(),
		"",
		help,
	)
}

func (m NetworksModel) viewCreateForm() string {
	title := HeaderStyle.Render("🔗  Create Network")

	label := func(s string, active bool) string {
		if active {
			return KeyStyle.Render(s)
		}
		return SubtitleStyle.Render(s)
	}

	internalVal := InactiveStyle.Render("no")
	if m.createInternal {
		internalVal = ActiveStyle.Render("yes")
	}

	form := lipgloss.JoinVertical(lipgloss.Left,
		label("  Name:", m.createFocus == netFieldName),
		"  "+m.createInputs[netFieldName].View(),
		"",
		label("  Driver (bridge|host|overlay|macvlan|none):", m.createFocus == netFieldDriver),
		"  "+m.createInputs[netFieldDriver].View(),
		"",
		label("  Subnet (optional, e.g. 172.20.0.0/24):", m.createFocus == netFieldSubnet),
		"  "+m.createInputs[netFieldSubnet].View(),
		"",
		label("  Gateway (optional, e.g. 172.20.0.1):", m.createFocus == netFieldGateway),
		"  "+m.createInputs[netFieldGateway].View(),
		"",
		KeyStyle.Render("  Internal: ")+internalVal+HelpStyle.Render("  [ctrl+i] toggle"),
		"",
		HelpStyle.Render("  [tab/↓] next field  [shift+tab/↑] prev  [ctrl+s] create  [esc] cancel"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, "", form)
}

func (m *NetworksModel) SetSize(w, h int) {
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
