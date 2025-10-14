package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zrougamed/portainer-cli/internal/api"
)

type ImagesModel struct {
	client     *api.Client
	table      table.Model
	images     []api.Image
	endpointID int
	loading    bool
	status     string
	width      int
	height     int
}

type imagesLoadedMsg struct{ images []api.Image }

func NewImagesModel(client *api.Client) ImagesModel {
	cols := []table.Column{
		{Title: "ID", Width: 14},
		{Title: "Repository:Tag", Width: 40},
		{Title: "Size", Width: 12},
		{Title: "Containers", Width: 12},
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

	return ImagesModel{client: client, table: t}
}

func (m ImagesModel) Init() tea.Cmd {
	return nil
}

func (m ImagesModel) LoadImages(endpointID int) tea.Cmd {
	m.endpointID = endpointID
	return func() tea.Msg {
		images, err := m.client.ListImages(endpointID)
		if err != nil {
			return ErrMsg{err}
		}
		return imagesLoadedMsg{images}
	}
}

func (m ImagesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case imagesLoadedMsg:
		m.images = msg.images
		m.table.SetRows(m.buildRows())
		m.loading = false
		var totalSize int64
		for _, img := range m.images {
			totalSize += img.Size
		}
		m.status = fmt.Sprintf("%d image(s), total %s", len(m.images), formatBytes(totalSize))

	case tea.KeyMsg:
		if msg.String() == "r" {
			m.loading = true
			return m, m.LoadImages(m.endpointID)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m ImagesModel) buildRows() []table.Row {
	rows := make([]table.Row, len(m.images))
	for i, img := range m.images {
		shortID := img.ID
		if len(shortID) > 19 {
			shortID = shortID[7:19] // strip "sha256:" prefix
		}
		tag := "<none>"
		if len(img.RepoTags) > 0 && img.RepoTags[0] != "<none>:<none>" {
			tag = img.RepoTags[0]
		}
		rows[i] = table.Row{
			shortID,
			tag,
			formatBytes(img.Size),
			fmt.Sprintf("%d", img.Containers),
		}
	}
	return rows
}

func (m ImagesModel) View() string {
	title := HeaderStyle.Render("🖼   Images")
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading images...")
	}
	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [r] refresh  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left, title, status, "", m.table.View(), "", help)
}

func (m *ImagesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
