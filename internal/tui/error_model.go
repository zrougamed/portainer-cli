package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrorModalModel shows the full error text in a scrollable viewport
// with clipboard copy support.
type ErrorModalModel struct {
	viewport   viewport.Model
	rawText    string
	copyStatus string // feedback after copy attempt
	width      int
	height     int
}

func NewErrorModalModel(errText string, width, height int) ErrorModalModel {
	m := ErrorModalModel{
		rawText: errText,
		width:   width,
		height:  height,
	}
	m.rebuildViewport()
	return m
}

func (m *ErrorModalModel) rebuildViewport() {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 20
	}

	// Viewport occupies most of the screen; leave room for title + help bar
	vpWidth := w - 4
	vpHeight := h - 6
	if vpHeight < 4 {
		vpHeight = 4
	}

	vp := viewport.New(vpWidth, vpHeight)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorDanger).
		Padding(0, 1)

	// Word-wrap the raw text to fit the viewport
	wrapped := wordWrap(m.rawText, vpWidth-4)
	vp.SetContent(
		ErrorStyle.Copy().Bold(false).Foreground(colorFg).Render(wrapped),
	)
	m.viewport = vp
}

func (m ErrorModalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CopyDoneMsg:
		if msg.Success {
			m.copyStatus = "✓ Copied to clipboard!"
		} else {
			m.copyStatus = "✗ Copy failed (no clipboard tool found)"
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			return m, copyToClipboard(m.rawText)
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m ErrorModalModel) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	title := lipgloss.NewStyle().
		Foreground(colorDanger).
		Bold(true).
		Render("⚠  Error Detail")

	scrollPct := fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))
	pct := SubtitleStyle.Render("  scroll " + scrollPct)

	status := ""
	if m.copyStatus != "" {
		status = "\n" + lipgloss.NewStyle().
			Foreground(colorSuccess).
			Render("  "+m.copyStatus)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		pct,
		m.viewport.View(),
		status,
	)
}

func (m *ErrorModalModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.rebuildViewport()
}

func (m ErrorModalModel) Init() tea.Cmd {
	return nil
}

// ─── Clipboard ────────────────────────────────────────────────────────────────

// copyToClipboard runs the platform clipboard command in the background
// and returns a CopyDoneMsg.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "windows":
			cmd = exec.Command("clip")
		default: // Linux — try xclip, then xsel, then wl-clipboard
			if path, err := exec.LookPath("xclip"); err == nil {
				cmd = exec.Command(path, "-selection", "clipboard")
			} else if path, err := exec.LookPath("xsel"); err == nil {
				cmd = exec.Command(path, "--clipboard", "--input")
			} else if path, err := exec.LookPath("wl-copy"); err == nil {
				cmd = exec.Command(path)
			} else {
				return CopyDoneMsg{Success: false}
			}
		}

		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return CopyDoneMsg{Success: false}
		}
		return CopyDoneMsg{Success: true}
	}
}
