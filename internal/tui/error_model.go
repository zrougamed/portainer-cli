package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrorModalModel shows the full error text in a scrollable viewport
// with clipboard copy support and temp-file fallback.
type ErrorModalModel struct {
	viewport   viewport.Model
	rawText    string
	copyStatus string // feedback after copy attempt
	copyIsErr  bool   // true = status is a warning/error colour
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

	wrapped := wordWrap(m.rawText, vpWidth-4)
	vp.SetContent(
		ErrorStyle.Copy().Bold(false).Foreground(colorFg).Render(wrapped),
	)
	m.viewport = vp
}

func (m ErrorModalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CopyDoneMsg:
		m.copyIsErr = !msg.Success
		if msg.Success && msg.FilePath == "" {
			m.copyStatus = "✓ Copied to clipboard!"
		} else if msg.Success && msg.FilePath != "" {
			// Temp-file fallback succeeded
			m.copyStatus = "✓ Saved to: " + msg.FilePath
			m.copyIsErr = false
		} else {
			m.copyStatus = "✗ " + msg.ErrDetail
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
	title := lipgloss.NewStyle().
		Foreground(colorDanger).
		Bold(true).
		Render("⚠  Error Detail")

	scrollPct := fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))
	pct := SubtitleStyle.Render("  scroll " + scrollPct)

	status := ""
	if m.copyStatus != "" {
		color := colorSuccess
		if m.copyIsErr {
			color = colorWarning
		}
		status = "\n" + lipgloss.NewStyle().
			Foreground(color).
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

// ─── Clipboard ────────────────────────────────────────────────────────────────

// CopyDoneMsg is returned after a copy attempt.
type CopyDoneMsg struct {
	Success   bool
	FilePath  string // set when clipboard unavailable and we fell back to a file
	ErrDetail string // human-readable reason on failure
}

// copyToClipboard tries platform clipboard tools in order, then falls back
// to writing a temp file so the content is always accessible.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		// ── 1. Try native clipboard ──────────────────────────────────────────
		if tryClipboard(text) {
			return CopyDoneMsg{Success: true}
		}

		// ── 2. Fallback: write to a temp file ────────────────────────────────
		dir := os.TempDir()
		fname := fmt.Sprintf("portainer-tui-error-%s.txt",
			time.Now().Format("20060102-150405"))
		path := filepath.Join(dir, fname)

		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			return CopyDoneMsg{
				Success:   false,
				ErrDetail: fmt.Sprintf("clipboard unavailable and could not write temp file: %v", err),
			}
		}

		return CopyDoneMsg{Success: true, FilePath: path}
	}
}

// tryClipboard attempts to pipe text into a clipboard command.
// Returns true on success.
func tryClipboard(text string) bool {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		// Linux / BSD — try tools in preference order
		tools := [][]string{
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
			{"wl-copy"},
		}
		for _, args := range tools {
			if path, err := exec.LookPath(args[0]); err == nil {
				cmd = exec.Command(path, args[1:]...)
				break
			}
		}
		if cmd == nil {
			return false
		}
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run() == nil
}

func (m ErrorModalModel) Init() tea.Cmd {
	return nil
}
