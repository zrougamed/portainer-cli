package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type loginField int

const (
	loginFieldURL loginField = iota
	loginFieldUser
	loginFieldPass
)

// LoginModel is shown when authentication fails so the user can re-auth
// without restarting the TUI.
type LoginModel struct {
	client *api.Client
	inputs [3]textinput.Model
	focus  loginField
	errMsg string
	width  int
	height int
}

func NewLoginModel(client *api.Client) LoginModel {
	baseURL := ""
	if client != nil {
		baseURL = client.BaseURL
	}

	urlInput := textinput.New()
	urlInput.Placeholder = "http://localhost:9000"
	urlInput.SetValue(baseURL)
	urlInput.Focus()
	urlInput.Width = 50

	userInput := textinput.New()
	userInput.Placeholder = "admin"
	userInput.Width = 50

	passInput := textinput.New()
	passInput.Placeholder = "password"
	passInput.EchoMode = textinput.EchoPassword
	passInput.EchoCharacter = '•'
	passInput.Width = 50

	return LoginModel{
		client: client,
		inputs: [3]textinput.Model{urlInput, userInput, passInput},
		focus:  loginFieldURL,
	}
}

func (m *LoginModel) SetError(msg string) {
	m.errMsg = msg
}

func (m LoginModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focus = (m.focus + 1) % 3
			m.refocus()
			return m, textinput.Blink
		case "shift+tab", "up":
			m.focus = (m.focus + 2) % 3 // wrap backwards
			m.refocus()
			return m, textinput.Blink
		case "enter":
			if m.focus < loginFieldPass {
				m.focus++
				m.refocus()
				return m, textinput.Blink
			}
			// Submit
			url := strings.TrimRight(m.inputs[loginFieldURL].Value(), "/")
			user := m.inputs[loginFieldUser].Value()
			pass := m.inputs[loginFieldPass].Value()
			if url == "" || user == "" {
				m.errMsg = "URL and username are required"
				return m, nil
			}
			return m, func() tea.Msg {
				c := api.NewClient(url, "")
				if err := c.Authenticate(user, pass); err != nil {
					return ErrMsg{err}
				}
				return LoginSuccessMsg{Client: c}
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m *LoginModel) refocus() {
	for i := range m.inputs {
		if loginField(i) == m.focus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m LoginModel) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	title := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		Render("⚓ Portainer TUI — Login")

	subtitle := SubtitleStyle.Render("Session expired or authentication required")

	errLine := ""
	if m.errMsg != "" {
		errLine = "\n" + ErrorStyle.Render("  ✗ "+m.errMsg) + "\n"
	}

	label := func(s string, active bool) string {
		if active {
			return KeyStyle.Render(s)
		}
		return SubtitleStyle.Render(s)
	}

	form := lipgloss.JoinVertical(lipgloss.Left,
		label("  Portainer URL:", m.focus == loginFieldURL),
		"  "+m.inputs[loginFieldURL].View(),
		"",
		label("  Username:", m.focus == loginFieldUser),
		"  "+m.inputs[loginFieldUser].View(),
		"",
		label("  Password:", m.focus == loginFieldPass),
		"  "+m.inputs[loginFieldPass].View(),
		"",
		HelpStyle.Render("  [tab] next field  [enter] login  [ctrl+c] quit"),
	)

	box := BoxStyle.Copy().
		Width(w - 4).
		BorderForeground(colorPrimary).
		Render(form)

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(title),
		lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(subtitle),
		fmt.Sprintf("%s", errLine),
		box,
	)
}

func (m *LoginModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	for i := range m.inputs {
		m.inputs[i].Width = w - 10
		if m.inputs[i].Width < 20 {
			m.inputs[i].Width = 20
		}
	}
}
