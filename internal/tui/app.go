package tui

import (
	"fmt"
	"strings"

	"github.com/zrougamed/portainer-cli/internal/api"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen identifies which view is active
type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenEndpoints
	ScreenContainers
	ScreenStacks
	ScreenImages
	ScreenVolumes
	ScreenNetworks
	ScreenLogs
	ScreenConfirm
	ScreenError
	ScreenLogin
)

// App is the root BubbleTea model
type App struct {
	client     *api.Client
	width      int
	height     int
	screen     Screen
	prevScreen Screen
	err        error

	// Sub-models
	dashboard  DashboardModel
	endpoints  EndpointsModel
	containers ContainersModel
	stacks     StacksModel
	images     ImagesModel
	volumes    VolumesModel
	networks   NetworksModel
	logs       LogsModel
	confirm    ConfirmModel
	errModal   ErrorModalModel
	login      LoginModel

	// Active endpoint context
	activeEndpoint *api.Endpoint
}

func NewApp(client *api.Client) App {
	return App{
		client:     client,
		screen:     ScreenDashboard,
		dashboard:  NewDashboardModel(client),
		endpoints:  NewEndpointsModel(client),
		containers: NewContainersModel(client),
		stacks:     NewStacksModel(client),
		images:     NewImagesModel(client),
		volumes:    NewVolumesModel(client),
		networks:   NewNetworksModel(client),
		logs:       NewLogsModel(client),
		login:      NewLoginModel(client),
	}
}

// ─── Messages ─────────────────────────────────────────────────────────────────

type NavigateMsg struct{ Screen Screen }
type EndpointSelectedMsg struct{ Endpoint api.Endpoint }
type ShowLogsMsg struct {
	EndpointID  int
	ContainerID string
	Name        string
}
type ConfirmMsg struct {
	Prompt string
	OnYes  tea.Cmd
}
type ConfirmResultMsg struct{ Confirmed bool }
type ErrMsg struct{ Err error }
type StatusMsg struct{ Text string }
type LoginSuccessMsg struct{ Client *api.Client }

// ─── Init ─────────────────────────────────────────────────────────────────────

func (a App) Init() tea.Cmd {
	return a.dashboard.Init()
}

// isTextInputActive returns true when a text-editing subview is focused.
func (a App) isTextInputActive() bool {
	return (a.screen == ScreenStacks && a.stacks.subview == stacksDeploy) ||
		(a.screen == ScreenVolumes && a.volumes.subview == volumesCreate) ||
		(a.screen == ScreenNetworks && a.networks.subview == networksCreate) ||
		a.screen == ScreenLogin
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.propagateSize()

	case ErrMsg:
		if msg.Err == nil {
			break
		}
		if api.IsAuthError(msg.Err) {
			a.prevScreen = a.screen
			a.screen = ScreenLogin
			a.login = NewLoginModel(a.client)
			a.login.SetError(msg.Err.Error())
			return a, nil
		}
		a.err = msg.Err
		a.errModal = NewErrorModalModel(msg.Err.Error(), a.width, a.height)

	case LoginSuccessMsg:
		a.client = msg.Client
		a.dashboard = NewDashboardModel(a.client)
		a.endpoints = NewEndpointsModel(a.client)
		a.containers = NewContainersModel(a.client)
		a.stacks = NewStacksModel(a.client)
		a.images = NewImagesModel(a.client)
		a.volumes = NewVolumesModel(a.client)
		a.networks = NewNetworksModel(a.client)
		a.logs = NewLogsModel(a.client)
		a.err = nil
		a.screen = ScreenDashboard
		return a, a.dashboard.Init()

	case CopyDoneMsg:
		m, cmd := a.errModal.Update(msg)
		a.errModal = m.(ErrorModalModel)
		cmds = append(cmds, cmd)

	case NavigateMsg:
		a.prevScreen = a.screen
		a.screen = msg.Screen
		switch msg.Screen {
		case ScreenEndpoints:
			return a, a.endpoints.Init()
		case ScreenContainers:
			if a.activeEndpoint != nil {
				return a, a.containers.LoadContainers(a.activeEndpoint.ID)
			}
		case ScreenStacks:
			if a.activeEndpoint != nil {
				a.stacks.activeEndpointID = a.activeEndpoint.ID
				a.stacks.activeEndpointName = a.activeEndpoint.Name
			}
			return a, a.stacks.Init()
		case ScreenImages:
			if a.activeEndpoint != nil {
				return a, a.images.LoadImages(a.activeEndpoint.ID)
			}
		case ScreenVolumes:
			if a.activeEndpoint != nil {
				return a, a.volumes.LoadVolumes(a.activeEndpoint.ID)
			}
		case ScreenNetworks:
			if a.activeEndpoint != nil {
				return a, a.networks.LoadNetworks(a.activeEndpoint.ID)
			}
		}

	case EndpointSelectedMsg:
		ep := msg.Endpoint
		a.activeEndpoint = &ep
		a.stacks.activeEndpointID = ep.ID
		a.stacks.activeEndpointName = ep.Name
		a.dashboard.activeEndpoint = a.activeEndpoint
		a.prevScreen = ScreenEndpoints
		a.screen = ScreenDashboard
		return a, nil

	case ShowLogsMsg:
		a.prevScreen = a.screen
		a.screen = ScreenLogs
		return a, a.logs.Load(msg.EndpointID, msg.ContainerID, msg.Name)

	case ConfirmMsg:
		a.prevScreen = a.screen
		a.screen = ScreenConfirm
		a.confirm = NewConfirmModel(msg.Prompt, msg.OnYes)
		return a, nil

	case ConfirmResultMsg:
		a.screen = a.prevScreen
		if msg.Confirmed {
			return a, a.confirm.onYes
		}

	case tea.KeyMsg:
		if !a.isTextInputActive() {
			switch msg.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "q":
				if a.screen == ScreenDashboard {
					return a, tea.Quit
				}
			case "esc":
				if a.screen == ScreenError {
					a.screen = a.prevScreen
					return a, nil
				}
				if a.screen != ScreenDashboard {
					a.screen = a.prevScreen
				}
				return a, nil
			case "e":
				if a.err != nil && a.screen != ScreenError {
					a.prevScreen = a.screen
					a.screen = ScreenError
					a.errModal = NewErrorModalModel(a.err.Error(), a.width, a.height)
					return a, nil
				}
			case "x":
				if a.err != nil {
					a.err = nil
				}
				if a.screen == ScreenError {
					a.screen = a.prevScreen
				}
				return a, nil
			case "m":
				if a.screen != ScreenDashboard && a.screen != ScreenConfirm && a.screen != ScreenError {
					a.prevScreen = a.screen
					a.screen = ScreenDashboard
					return a, nil
				}
			}
		}
	}

	// Delegate to active sub-model
	switch a.screen {
	case ScreenDashboard:
		m, cmd := a.dashboard.Update(msg)
		a.dashboard = m.(DashboardModel)
		cmds = append(cmds, cmd)

	case ScreenEndpoints:
		m, cmd := a.endpoints.Update(msg)
		a.endpoints = m.(EndpointsModel)
		cmds = append(cmds, cmd)

	case ScreenContainers:
		m, cmd := a.containers.Update(msg)
		a.containers = m.(ContainersModel)
		cmds = append(cmds, cmd)

	case ScreenStacks:
		m, cmd := a.stacks.Update(msg)
		a.stacks = m.(StacksModel)
		cmds = append(cmds, cmd)

	case ScreenImages:
		m, cmd := a.images.Update(msg)
		a.images = m.(ImagesModel)
		cmds = append(cmds, cmd)

	case ScreenVolumes:
		m, cmd := a.volumes.Update(msg)
		a.volumes = m.(VolumesModel)
		cmds = append(cmds, cmd)

	case ScreenNetworks:
		m, cmd := a.networks.Update(msg)
		a.networks = m.(NetworksModel)
		cmds = append(cmds, cmd)

	case ScreenLogs:
		m, cmd := a.logs.Update(msg)
		a.logs = m.(LogsModel)
		cmds = append(cmds, cmd)

	case ScreenConfirm:
		m, cmd := a.confirm.Update(msg)
		a.confirm = m.(ConfirmModel)
		cmds = append(cmds, cmd)

	case ScreenError:
		m, cmd := a.errModal.Update(msg)
		a.errModal = m.(ErrorModalModel)
		cmds = append(cmds, cmd)

	case ScreenLogin:
		m, cmd := a.login.Update(msg)
		a.login = m.(LoginModel)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (a App) View() string {
	if a.screen == ScreenLogin {
		return a.login.View()
	}

	header := a.renderHeader()

	if a.screen == ScreenError {
		footer := HelpStyle.Render("[c] copy  [x] dismiss  [esc] back")
		return lipgloss.JoinVertical(lipgloss.Left, header, a.errModal.View(), footer)
	}

	var body string
	switch a.screen {
	case ScreenDashboard:
		body = a.dashboard.View()
	case ScreenEndpoints:
		body = a.endpoints.View()
	case ScreenContainers:
		body = a.containers.View()
	case ScreenStacks:
		body = a.stacks.View()
	case ScreenImages:
		body = a.images.View()
	case ScreenVolumes:
		body = a.volumes.View()
	case ScreenNetworks:
		body = a.networks.View()
	case ScreenLogs:
		body = a.logs.View()
	case ScreenConfirm:
		body = a.confirm.View()
	default:
		body = "Unknown screen"
	}

	footer := a.renderFooter()

	if a.err != nil {
		errBanner := a.renderErrorBanner()
		body = lipgloss.JoinVertical(lipgloss.Left, errBanner, body)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (a App) renderErrorBanner() string {
	width := a.width
	if width == 0 {
		width = 80
	}
	prefix := "⚠  "
	hint := "  [e] details  [x] dismiss"
	msgWidth := width - lipgloss.Width(prefix) - 4
	if msgWidth < 20 {
		msgWidth = 20
	}
	msg := a.err.Error()
	wrapped := wordWrap(msg, msgWidth)
	lines := strings.Split(wrapped, "\n")
	var sb strings.Builder
	for i, line := range lines {
		if i == 0 {
			sb.WriteString(prefix + line)
		} else {
			sb.WriteString("\n   " + line)
		}
	}
	banner := lipgloss.NewStyle().
		Foreground(colorDanger).
		Bold(true).
		Width(width).
		Render(sb.String())
	hintLine := HelpStyle.Render(hint)
	return lipgloss.JoinVertical(lipgloss.Left, banner, hintLine)
}

func wordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= maxWidth {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}

func (a App) renderHeader() string {
	title := TitleStyle.Render("⚓ Portainer TUI")
	var ctx string
	if a.activeEndpoint != nil {
		ctx = SubtitleStyle.Render(fmt.Sprintf(" › %s", a.activeEndpoint.Name))
	}
	nav := a.screenName()
	right := SubtitleStyle.Render(nav)

	width := a.width
	if width == 0 {
		width = 80
	}
	gap := width - lipgloss.Width(title+ctx) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return lipgloss.NewStyle().
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorMuted).
		Width(width).
		Render(title + ctx + fmt.Sprintf("%*s", gap, "") + right)
}

func (a App) renderFooter() string {
	help := "[↑↓] navigate  [enter] select  [esc] back  [m] menu  [q] quit  [r] refresh"
	if a.err != nil {
		help += "  [e] error detail  [x] dismiss error"
	}
	return HelpStyle.Render(help)
}

func (a App) screenName() string {
	switch a.screen {
	case ScreenDashboard:
		return "Dashboard"
	case ScreenEndpoints:
		return "Environments"
	case ScreenContainers:
		return "Containers"
	case ScreenStacks:
		return "Stacks"
	case ScreenImages:
		return "Images"
	case ScreenVolumes:
		return "Volumes"
	case ScreenNetworks:
		return "Networks"
	case ScreenLogs:
		return "Logs"
	case ScreenConfirm:
		return "Confirm"
	case ScreenError:
		return "Error Detail"
	case ScreenLogin:
		return "Login"
	}
	return ""
}

func (a *App) propagateSize() {
	inner := a.height - 4
	a.containers.SetSize(a.width, inner)
	a.stacks.SetSize(a.width, inner)
	a.endpoints.SetSize(a.width, inner)
	a.images.SetSize(a.width, inner)
	a.volumes.SetSize(a.width, inner)
	a.networks.SetSize(a.width, inner)
	a.logs.SetSize(a.width, inner)
	a.errModal.SetSize(a.width, inner)
	a.login.SetSize(a.width, a.height)
}
