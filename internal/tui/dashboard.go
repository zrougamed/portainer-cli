package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type menuItem struct {
	title, desc string
	screen      Screen
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }

type DashboardModel struct {
	client *api.Client
	list   list.Model
}

func NewDashboardModel(client *api.Client) DashboardModel {
	items := []list.Item{
		menuItem{"🌐  Environments", "Browse and select Portainer endpoints", ScreenEndpoints},
		menuItem{"📦  Containers", "Manage containers on active endpoint", ScreenContainers},
		menuItem{"📚  Stacks", "Deploy and manage compose stacks", ScreenStacks},
		menuItem{"🖼   Images", "View Docker images", ScreenImages},
		menuItem{"💾  Volumes", "View Docker volumes", ScreenVolumes},
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = SelectedRowStyle.Copy().PaddingLeft(2)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(colorAccent).PaddingLeft(2)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(colorFg).PaddingLeft(2)

	l := list.New(items, delegate, 60, 20)
	l.Title = "Main Menu"
	l.Styles.Title = TitleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	return DashboardModel{client: client, list: l}
}

func (d DashboardModel) Init() tea.Cmd { return nil }

func (d DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected, ok := d.list.SelectedItem().(menuItem)
			if ok {
				return d, func() tea.Msg {
					return NavigateMsg{Screen: selected.screen}
				}
			}
		}
	}
	var cmd tea.Cmd
	d.list, cmd = d.list.Update(msg)
	return d, cmd
}

func (d DashboardModel) View() string {
	banner := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		Render(`
  ____            _        _
 |  _ \ ___  _ __| |_ __ _(_)_ __   ___ _ __
 | |_) / _ \| '__| __/ _' | | '_ \ / _ \ '__|
 |  __/ (_) | |  | || (_| | | | | |  __/ |
 |_|   \___/|_|   \__\__,_|_|_| |_|\___|_|  TUI
`)
	subtitle := SubtitleStyle.Render(fmt.Sprintf("  Connected to: %s", "Portainer"))
	return lipgloss.JoinVertical(lipgloss.Left,
		banner,
		subtitle,
		"",
		d.list.View(),
	)
}
