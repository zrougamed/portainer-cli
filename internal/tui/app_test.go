package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zrougamed/portainer-cli/internal/api"
)

func TestNewApp_DefaultScreen(t *testing.T) {
	a := NewApp(nil)
	if a.screen != ScreenDashboard {
		t.Errorf("initial screen = %d, want ScreenDashboard (%d)", a.screen, ScreenDashboard)
	}
}

func TestApp_Init_ReturnsCmd(t *testing.T) {
	a := NewApp(nil)
	// Init may return nil (dashboard has no init cmd), that's fine
	_ = a.Init()
}

func TestApp_WindowSizeMsg(t *testing.T) {
	a := NewApp(nil)
	result, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := result.(App)
	if updated.width != 120 {
		t.Errorf("width = %d, want 120", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height = %d, want 40", updated.height)
	}
}

func TestApp_ErrMsg(t *testing.T) {
	a := NewApp(nil)
	testErr := errors.New("something went wrong")
	result, _ := a.Update(ErrMsg{Err: testErr})
	updated := result.(App)
	if updated.err == nil {
		t.Error("expected err to be set")
	}
	if updated.err.Error() != "something went wrong" {
		t.Errorf("err = %q, want 'something went wrong'", updated.err.Error())
	}
}

func TestApp_NavigateToEndpoints(t *testing.T) {
	a := NewApp(nil)
	result, _ := a.Update(NavigateMsg{Screen: ScreenEndpoints})
	updated := result.(App)
	if updated.screen != ScreenEndpoints {
		t.Errorf("screen = %d, want ScreenEndpoints (%d)", updated.screen, ScreenEndpoints)
	}
}

func TestApp_NavigateToStacks(t *testing.T) {
	a := NewApp(nil)
	result, _ := a.Update(NavigateMsg{Screen: ScreenStacks})
	updated := result.(App)
	if updated.screen != ScreenStacks {
		t.Errorf("screen = %d, want ScreenStacks (%d)", updated.screen, ScreenStacks)
	}
}

func TestApp_EndpointSelected(t *testing.T) {
	a := NewApp(nil)
	ep := api.Endpoint{ID: 1, Name: "local"}
	result, _ := a.Update(EndpointSelectedMsg{Endpoint: ep})
	updated := result.(App)
	if updated.screen != ScreenContainers {
		t.Errorf("screen = %d, want ScreenContainers (%d)", updated.screen, ScreenContainers)
	}
	if updated.activeEndpoint == nil {
		t.Fatal("activeEndpoint should be set")
	}
	if updated.activeEndpoint.ID != 1 {
		t.Errorf("activeEndpoint.ID = %d, want 1", updated.activeEndpoint.ID)
	}
}

func TestApp_ShowLogsMsg(t *testing.T) {
	a := NewApp(nil)
	result, _ := a.Update(ShowLogsMsg{EndpointID: 1, ContainerID: "abc123", Name: "web"})
	updated := result.(App)
	if updated.screen != ScreenLogs {
		t.Errorf("screen = %d, want ScreenLogs (%d)", updated.screen, ScreenLogs)
	}
}

func TestApp_ConfirmMsg(t *testing.T) {
	a := NewApp(nil)
	result, _ := a.Update(ConfirmMsg{Prompt: "Really?"})
	updated := result.(App)
	if updated.screen != ScreenConfirm {
		t.Errorf("screen = %d, want ScreenConfirm (%d)", updated.screen, ScreenConfirm)
	}
}

func TestApp_ConfirmResultConfirmed(t *testing.T) {
	a := NewApp(nil)
	a.screen = ScreenConfirm
	a.prevScreen = ScreenContainers
	a.confirm = NewConfirmModel("test", func() tea.Msg { return nil })

	result, _ := a.Update(ConfirmResultMsg{Confirmed: true})
	updated := result.(App)
	if updated.screen != ScreenContainers {
		t.Errorf("screen = %d, want ScreenContainers (%d)", updated.screen, ScreenContainers)
	}
}

func TestApp_ConfirmResultCancelled(t *testing.T) {
	a := NewApp(nil)
	a.screen = ScreenConfirm
	a.prevScreen = ScreenStacks
	result, _ := a.Update(ConfirmResultMsg{Confirmed: false})
	updated := result.(App)
	if updated.screen != ScreenStacks {
		t.Errorf("screen = %d, want ScreenStacks (%d)", updated.screen, ScreenStacks)
	}
}

func TestApp_EscKeyNavigatesBack(t *testing.T) {
	a := NewApp(nil)
	a.screen = ScreenContainers
	a.prevScreen = ScreenDashboard
	result, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(App)
	if updated.screen != ScreenDashboard {
		t.Errorf("screen after esc = %d, want ScreenDashboard (%d)", updated.screen, ScreenDashboard)
	}
}

func TestApp_QuitOnDashboard(t *testing.T) {
	a := NewApp(nil)
	a.screen = ScreenDashboard
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command when pressing q on dashboard")
	}
}

func TestApp_ViewDashboard(t *testing.T) {
	a := NewApp(nil)
	a.screen = ScreenDashboard
	a.width = 80
	a.height = 24
	view := a.View()
	if view == "" {
		t.Error("View() should not return empty string")
	}
}

func TestApp_ViewWithError(t *testing.T) {
	a := NewApp(nil)
	a.screen = ScreenDashboard
	a.width = 80
	a.height = 30
	a.err = errors.New("test error message")
	view := a.View()
	if !strings.Contains(view, "⚠") {
		t.Errorf("view should show error warning symbol, got: %s", view)
	}
	if !strings.Contains(view, "[e]") {
		t.Errorf("view should show [e] hint to expand error, got: %s", view)
	}
}

func TestApp_ViewWithActiveEndpoint(t *testing.T) {
	a := NewApp(nil)
	a.width = 80
	ep := api.Endpoint{ID: 1, Name: "production"}
	a.activeEndpoint = &ep
	view := a.View()
	if !strings.Contains(view, "production") {
		t.Errorf("view should show active endpoint name, got: %s", view)
	}
}

func TestApp_ScreenName(t *testing.T) {
	tests := []struct {
		screen Screen
		want   string
	}{
		{ScreenDashboard, "Dashboard"},
		{ScreenEndpoints, "Environments"},
		{ScreenContainers, "Containers"},
		{ScreenStacks, "Stacks"},
		{ScreenImages, "Images"},
		{ScreenVolumes, "Volumes"},
		{ScreenLogs, "Logs"},
		{ScreenConfirm, "Confirm"},
		{ScreenError, "Error Detail"},
	}
	a := NewApp(nil)
	for _, tc := range tests {
		a.screen = tc.screen
		got := a.screenName()
		if got != tc.want {
			t.Errorf("screenName() for screen %d = %q, want %q", tc.screen, got, tc.want)
		}
	}
}

func TestApp_NavigateToImagesWithoutEndpoint(t *testing.T) {
	a := NewApp(nil)
	a.activeEndpoint = nil
	// Should not panic when navigating to Images without active endpoint
	result, _ := a.Update(NavigateMsg{Screen: ScreenImages})
	updated := result.(App)
	if updated.screen != ScreenImages {
		t.Errorf("screen = %d, want ScreenImages (%d)", updated.screen, ScreenImages)
	}
}

func TestApp_NavigateToImagesWithEndpoint(t *testing.T) {
	a := NewApp(nil)
	ep := api.Endpoint{ID: 2, Name: "remote"}
	a.activeEndpoint = &ep
	result, _ := a.Update(NavigateMsg{Screen: ScreenImages})
	updated := result.(App)
	if updated.screen != ScreenImages {
		t.Errorf("screen = %d, want ScreenImages", updated.screen)
	}
}

func TestApp_NavigateToContainersWithoutEndpoint(t *testing.T) {
	a := NewApp(nil)
	a.activeEndpoint = nil
	// Should not panic
	result, _ := a.Update(NavigateMsg{Screen: ScreenContainers})
	updated := result.(App)
	_ = updated
}

func TestApp_PropagateSize(t *testing.T) {
	a := NewApp(nil)
	// SetSize via WindowSizeMsg — should not panic
	result, _ := a.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	updated := result.(App)
	if updated.width != 200 || updated.height != 50 {
		t.Errorf("size not propagated: %dx%d", updated.width, updated.height)
	}
}
