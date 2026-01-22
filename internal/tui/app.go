package tui

import (
	"fmt"
	"strings"

	"github.com/zrougamed/portainer-cli/internal/api"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen identifies which view is active.
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
	ScreenEnvPicker // overlay: pick an environment before navigating
)

// App is the root BubbleTea model.
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
	envPicker  EnvPickerModel

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

// needsEndpoint returns true when the target screen requires an active endpoint.
func needsEndpoint(s Screen) bool {
	switch s {
	case ScreenContainers, ScreenImages, ScreenVolumes, ScreenNetworks:
		return true
	}
	return false
}

// isTextInputActive returns true when a text-editing subview is focused
// (suppresses global key shortcuts so they reach the text input instead).
func (a App) isTextInputActive() bool {
	return (a.screen == ScreenStacks && a.stacks.subview == stacksDeploy) ||
		(a.screen == ScreenVolumes && a.volumes.subview == volumesCreate) ||
		(a.screen == ScreenNetworks && a.networks.subview == networksCreate) ||
		(a.screen == ScreenImages && a.images.subview == imagesPullView) ||
		a.screen == ScreenLogin
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Terminal resize ───────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.propagateSize()

	// ── Errors ────────────────────────────────────────────────────────────────
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

	// ── Login success ─────────────────────────────────────────────────────────
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
		a.propagateSize()
		return a, a.dashboard.Init()

	case CopyDoneMsg:
		m, cmd := a.errModal.Update(msg)
		a.errModal = m.(ErrorModalModel)
		cmds = append(cmds, cmd)

		// ── Navigation ────────────────────────────────────────────────────────────
	case NavigateMsg:
		if needsEndpoint(msg.Screen) && a.activeEndpoint == nil {
			a.prevScreen = a.screen
			a.screen = ScreenEnvPicker
			a.envPicker = NewEnvPickerModel(a.client, msg)
			a.envPicker.SetSize(a.width, a.height-4)
			return a, tea.Batch(tea.ClearScreen, a.envPicker.Init())
		}

		a.prevScreen = a.screen
		a.screen = msg.Screen
		switch msg.Screen {
		case ScreenEndpoints:
			return a, tea.Batch(tea.ClearScreen, a.endpoints.Init())
		case ScreenContainers:
			if a.activeEndpoint != nil {
				return a, tea.Batch(tea.ClearScreen, a.containers.LoadContainers(a.activeEndpoint.ID))
			}
		case ScreenStacks:
			if a.activeEndpoint != nil {
				a.stacks.activeEndpointID = a.activeEndpoint.ID
				a.stacks.activeEndpointName = a.activeEndpoint.Name
			}
			return a, tea.Batch(tea.ClearScreen, a.stacks.Init())
		case ScreenImages:
			if a.activeEndpoint != nil {
				return a, tea.Batch(tea.ClearScreen, a.images.LoadImages(a.activeEndpoint.ID))
			}
		case ScreenVolumes:
			if a.activeEndpoint != nil {
				return a, tea.Batch(tea.ClearScreen, a.volumes.LoadVolumes(a.activeEndpoint.ID))
			}
		case ScreenNetworks:
			if a.activeEndpoint != nil {
				return a, tea.Batch(tea.ClearScreen, a.networks.LoadNetworks(a.activeEndpoint.ID))
			}
		}
	// ── Env picker selected ───────────────────────────────────────────────────
	case EnvPickerSelectedMsg:
		ep := msg.Endpoint
		a.activeEndpoint = &ep
		a.stacks.activeEndpointID = ep.ID
		a.stacks.activeEndpointName = ep.Name
		a.dashboard.activeEndpoint = a.activeEndpoint
		// Now replay the navigation that triggered the picker.
		a.screen = a.prevScreen
		return a.Update(msg.PendingNav)

	// ── Endpoint selected from the Environments screen ────────────────────────
	case EndpointSelectedMsg:
		ep := msg.Endpoint
		a.activeEndpoint = &ep
		a.stacks.activeEndpointID = ep.ID
		a.stacks.activeEndpointName = ep.Name
		a.dashboard.activeEndpoint = a.activeEndpoint
		a.prevScreen = ScreenEndpoints
		a.screen = ScreenDashboard
		return a, nil

	// ── Show logs ─────────────────────────────────────────────────────────────
	case ShowLogsMsg:
		a.prevScreen = a.screen
		a.screen = ScreenLogs
		// logs.Load is now a pointer receiver so state persists.
		return a, a.logs.Load(msg.EndpointID, msg.ContainerID, msg.Name)

	// ── Confirm dialog ────────────────────────────────────────────────────────
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

	// ── Global key handling ───────────────────────────────────────────────────
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
				if a.screen != ScreenDashboard {
					a.prevScreen = a.screen
					a.screen = ScreenDashboard
					return a, a.dashboard.Init()
				}
			}
		}
	}

	// ── Route remaining messages to the active sub-model ──────────────────────
	var cmd tea.Cmd
	switch a.screen {
	case ScreenDashboard:
		var m tea.Model
		m, cmd = a.dashboard.Update(msg)
		a.dashboard = m.(DashboardModel)
	case ScreenEndpoints:
		var m tea.Model
		m, cmd = a.endpoints.Update(msg)
		a.endpoints = m.(EndpointsModel)
	case ScreenContainers:
		var m tea.Model
		m, cmd = a.containers.Update(msg)
		a.containers = m.(ContainersModel)
	case ScreenStacks:
		var m tea.Model
		m, cmd = a.stacks.Update(msg)
		a.stacks = m.(StacksModel)
	case ScreenImages:
		var m tea.Model
		m, cmd = a.images.Update(msg)
		a.images = m.(ImagesModel)
	case ScreenVolumes:
		var m tea.Model
		m, cmd = a.volumes.Update(msg)
		a.volumes = m.(VolumesModel)
	case ScreenNetworks:
		var m tea.Model
		m, cmd = a.networks.Update(msg)
		a.networks = m.(NetworksModel)
	case ScreenLogs:
		var m tea.Model
		m, cmd = a.logs.Update(msg)
		a.logs = m.(LogsModel)
	case ScreenConfirm:
		var m tea.Model
		m, cmd = a.confirm.Update(msg)
		a.confirm = m.(ConfirmModel)
	case ScreenError:
		var m tea.Model
		m, cmd = a.errModal.Update(msg)
		a.errModal = m.(ErrorModalModel)
	case ScreenLogin:
		var m tea.Model
		m, cmd = a.login.Update(msg)
		a.login = m.(LoginModel)
	case ScreenEnvPicker:
		var m tea.Model
		m, cmd = a.envPicker.Update(msg)
		a.envPicker = m.(EnvPickerModel)
	}

	cmds = append(cmds, cmd)
	return a, tea.Batch(cmds...)
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (a App) View() string {
	header := a.renderHeader()
	footer := a.renderFooter()

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
	case ScreenError:
		body = a.errModal.View()
	case ScreenLogin:
		body = a.login.View()
	case ScreenEnvPicker:
		body = a.envPicker.View()
	default:
		body = "Unknown screen"
	}

	// Show inline error banner when not in error detail screen
	if a.err != nil && a.screen != ScreenError {
		errBanner := a.renderErrBanner()
		body = errBanner + "\n" + body
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (a App) renderHeader() string {
	title := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		Render("⚓ Portainer TUI")

	ctx := ""
	if a.activeEndpoint != nil {
		ctx = "  " + ActiveStyle.Render("●") + "  " + SubtitleStyle.Render(a.activeEndpoint.Name)
	} else {
		ctx = "  " + InactiveStyle.Render("○") + "  " + SubtitleStyle.Render("no environment")
	}

	right := ""
	if a.width > 0 {
		right = SubtitleStyle.Render(a.screenName())
	}

	width := a.width
	if width == 0 {
		width = 80
	}

	gap := width - lipgloss.Width(title) - lipgloss.Width(ctx) - lipgloss.Width(right) - 2
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

func (a App) renderErrBanner() string {
	msg := a.err.Error()
	if len(msg) > 120 {
		msg = msg[:117] + "..."
	}
	return ErrorStyle.Render("△ "+msg) + "\n" +
		HelpStyle.Render("  [e] details  [x] dismiss")
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
	case ScreenEnvPicker:
		return "Select Environment"
	}
	return ""
}

func (a *App) propagateSize() {
	inner := a.height - 4 // subtract header + footer rows
	if inner < 4 {
		inner = 4
	}
	a.containers.SetSize(a.width, inner)
	a.stacks.SetSize(a.width, inner)
	a.endpoints.SetSize(a.width, inner)
	a.images.SetSize(a.width, inner)
	a.volumes.SetSize(a.width, inner)
	a.networks.SetSize(a.width, inner)
	a.logs.SetSize(a.width, inner)
	a.errModal.SetSize(a.width, inner)
	a.login.SetSize(a.width, a.height)
	a.envPicker.SetSize(a.width, inner)
	// Resize dashboard list to fill the terminal
	a.dashboard.list.SetSize(a.width-4, inner-6)
}

// wordWrap is a simple word-wrap helper used by ErrorModalModel.
func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	var sb strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if len(line) <= width {
			sb.WriteString(line)
			sb.WriteByte('\n')
			continue
		}
		for len(line) > width {
			sb.WriteString(line[:width])
			sb.WriteByte('\n')
			line = line[width:]
		}
		if len(line) > 0 {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
