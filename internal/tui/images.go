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

type imagesSubview int

const (
	imagesListView imagesSubview = iota
	imagesPullView               // input for image:tag to pull
)

type ImagesModel struct {
	client     *api.Client
	table      table.Model
	images     []api.Image
	endpointID int
	loading    bool
	status     string
	subview    imagesSubview
	pullInput  textinput.Model
	width      int
	height     int
}

type imagesLoadedMsg struct {
	images     []api.Image
	endpointID int
}
type imageActionDoneMsg struct{ msg string }

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

	pi := textinput.New()
	pi.Placeholder = "nginx:latest"
	pi.Width = 50
	pi.CharLimit = 200

	return ImagesModel{client: client, table: t, pullInput: pi}
}

func (m ImagesModel) Init() tea.Cmd { return nil }

func (m ImagesModel) LoadImages(endpointID int) tea.Cmd {
	return func() tea.Msg {
		images, err := m.client.ListImages(endpointID)
		if err != nil {
			return ErrMsg{err}
		}
		return imagesLoadedMsg{images: images, endpointID: endpointID}
	}
}

func (m ImagesModel) selectedImage() (api.Image, bool) {
	if len(m.images) == 0 {
		return api.Image{}, false
	}
	idx := m.table.Cursor()
	if idx >= len(m.images) {
		return api.Image{}, false
	}
	return m.images[idx], true
}

func imageTag(img api.Image) string {
	if len(img.RepoTags) > 0 && img.RepoTags[0] != "<none>:<none>" {
		return img.RepoTags[0]
	}
	return "<none>"
}

func (m ImagesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case imagesLoadedMsg:
		m.endpointID = msg.endpointID
		m.images = msg.images
		m.table.SetRows(m.buildRows())
		m.loading = false
		var totalSize int64
		for _, img := range m.images {
			totalSize += img.Size
		}
		m.status = fmt.Sprintf("%d image(s), total %s", len(m.images), formatBytes(totalSize))

	case imageActionDoneMsg:
		m.status = msg.msg
		m.subview = imagesListView
		return m, m.LoadImages(m.endpointID)

	case tea.KeyMsg:
		// ── Pull form ────────────────────────────────────────────────────────
		if m.subview == imagesPullView {
			switch msg.String() {
			case "esc":
				m.subview = imagesListView
				m.pullInput.Blur()
				return m, nil
			case "enter", "ctrl+s":
				image := strings.TrimSpace(m.pullInput.Value())
				if image == "" {
					return m, func() tea.Msg { return ErrMsg{fmt.Errorf("image name is required (e.g. nginx:latest)")} }
				}
				endpointID := m.endpointID
				return m, func() tea.Msg {
					return ConfirmMsg{
						Prompt: fmt.Sprintf("Pull image '%s'? This may take a while.", image),
						OnYes: func() tea.Msg {
							if err := m.client.PullImage(endpointID, image); err != nil {
								return ErrMsg{err}
							}
							return imageActionDoneMsg{"✓ Pulled: " + image}
						},
					}
				}
			}
			var cmd tea.Cmd
			m.pullInput, cmd = m.pullInput.Update(msg)
			return m, cmd
		}

		// ── List view ────────────────────────────────────────────────────────
		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.LoadImages(m.endpointID)

		case "p": // pull new image
			m.subview = imagesPullView
			m.pullInput.SetValue("")
			m.pullInput.Focus()
			return m, textinput.Blink

		case "d": // delete selected image
			img, ok := m.selectedImage()
			if !ok {
				return m, nil
			}
			tag := imageTag(img)
			endpointID := m.endpointID
			imgID := img.ID
			return m, func() tea.Msg {
				return ConfirmMsg{
					Prompt: fmt.Sprintf("Delete image '%s'?", tag),
					OnYes: func() tea.Msg {
						if err := m.client.DeleteImage(endpointID, imgID, false); err != nil {
							return ErrMsg{err}
						}
						return imageActionDoneMsg{"✓ Deleted: " + tag}
					},
				}
			}

		case "D": // force delete
			img, ok := m.selectedImage()
			if !ok {
				return m, nil
			}
			tag := imageTag(img)
			endpointID := m.endpointID
			imgID := img.ID
			return m, func() tea.Msg {
				return ConfirmMsg{
					Prompt: fmt.Sprintf("FORCE delete image '%s'? (removes even if containers use it)", tag),
					OnYes: func() tea.Msg {
						if err := m.client.DeleteImage(endpointID, imgID, true); err != nil {
							return ErrMsg{err}
						}
						return imageActionDoneMsg{"✓ Force-deleted: " + tag}
					},
				}
			}

		case "P": // prune dangling/unused images
			endpointID := m.endpointID
			return m, func() tea.Msg {
				return ConfirmMsg{
					Prompt: "Remove ALL unused (dangling) images? This frees disk space.",
					OnYes: func() tea.Msg {
						report, err := m.client.PruneImages(endpointID)
						if err != nil {
							return ErrMsg{err}
						}
						freed := ""
						if report != nil {
							freed = fmt.Sprintf(" (freed %s)", formatBytes(report.SpaceReclaimed))
						}
						return imageActionDoneMsg{"✓ Pruned unused images" + freed}
					},
				}
			}
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
			shortID = shortID[7:19] // strip "sha256:" prefix, keep 12 chars
		}
		tag := imageTag(img)
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

	if m.subview == imagesPullView {
		return lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			SubtitleStyle.Render("  Pull Image"),
			"",
			"  Image reference (e.g. nginx:latest, ubuntu:22.04):",
			"  "+m.pullInput.View(),
			"",
			HelpStyle.Render("  [enter] pull  [esc] cancel"),
		)
	}

	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, title, "  Loading images...")
	}
	status := SubtitleStyle.Render("  " + m.status)
	help := HelpStyle.Render("  [p] pull  [d] delete  [D] force-delete  [P] prune unused  [r] refresh  [esc] back")
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		status,
		"",
		m.table.View(),
		"",
		help,
	)
}

func (m *ImagesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 8)
	// Dynamically widen the tag column to fill available space
	tagWidth := w - 14 - 12 - 12 - 10
	if tagWidth < 20 {
		tagWidth = 20
	}
	m.table.SetColumns([]table.Column{
		{Title: "ID", Width: 14},
		{Title: "Repository:Tag", Width: tagWidth},
		{Title: "Size", Width: 12},
		{Title: "Containers", Width: 12},
	})
	m.pullInput.Width = w - 10
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
