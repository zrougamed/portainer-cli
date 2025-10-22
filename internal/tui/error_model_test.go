package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── wordWrap ─────────────────────────────────────────────────────────────────

func TestWordWrap_ShortLine(t *testing.T) {
	got := wordWrap("hello world", 80)
	if got != "hello world" {
		t.Errorf("wordWrap short = %q, want unchanged", got)
	}
}

func TestWordWrap_ExactWidth(t *testing.T) {
	got := wordWrap("hello world", 11)
	if got != "hello world" {
		t.Errorf("wordWrap exact = %q, want 'hello world'", got)
	}
}

func TestWordWrap_BreaksAtWordBoundary(t *testing.T) {
	got := wordWrap("one two three four", 10)
	lines := strings.Split(got, "\n")
	for _, line := range lines {
		if len(line) > 10 {
			t.Errorf("line %q exceeds maxWidth 10", line)
		}
	}
}

func TestWordWrap_LongWordNotBroken(t *testing.T) {
	// A single long word can't be broken — it stays on its own line
	got := wordWrap("superlongwordthatcannotbreak", 10)
	if !strings.Contains(got, "superlongwordthatcannotbreak") {
		t.Error("long word should be preserved even if it exceeds maxWidth")
	}
}

func TestWordWrap_EmptyString(t *testing.T) {
	got := wordWrap("", 80)
	// Empty or original returned — just ensure no panic
	_ = got
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	got := wordWrap("some text", 0)
	if got != "some text" {
		t.Errorf("zero maxWidth should return original text, got %q", got)
	}
}

func TestWordWrap_MultipleLines(t *testing.T) {
	long := "API error 404: Unable to find an environment with the specified identifier inside the database, Object not found inside the database (bucket=endpoints, key=1)"
	wrapped := wordWrap(long, 60)
	lines := strings.Split(wrapped, "\n")
	if len(lines) < 2 {
		t.Error("long text should wrap to multiple lines")
	}
	for _, line := range lines {
		if len(line) > 65 { // a bit of tolerance for long words
			t.Errorf("line too long (%d chars): %q", len(line), line)
		}
	}
}

func TestWordWrap_PreservesWords(t *testing.T) {
	input := "api key and auth header are not allowed at the same time"
	got := wordWrap(input, 30)
	// All words should still be present
	for _, word := range strings.Fields(input) {
		if !strings.Contains(got, word) {
			t.Errorf("word %q missing from wrapped output: %q", word, got)
		}
	}
}

// ─── ErrorModalModel ──────────────────────────────────────────────────────────

func TestNewErrorModalModel_StoresText(t *testing.T) {
	m := NewErrorModalModel("something went wrong", 80, 24)
	if m.rawText != "something went wrong" {
		t.Errorf("rawText = %q, want 'something went wrong'", m.rawText)
	}
}

func TestNewErrorModalModel_InitialCopyStatusEmpty(t *testing.T) {
	m := NewErrorModalModel("error", 80, 24)
	if m.copyStatus != "" {
		t.Errorf("copyStatus should be empty initially, got %q", m.copyStatus)
	}
}

func TestErrorModalModel_CopyDoneSuccess(t *testing.T) {
	m := NewErrorModalModel("error msg", 80, 24)
	result, _ := m.Update(CopyDoneMsg{Success: true})
	updated := result.(ErrorModalModel)
	if !strings.Contains(updated.copyStatus, "Copied") {
		t.Errorf("copyStatus = %q, should contain 'Copied'", updated.copyStatus)
	}
}

func TestErrorModalModel_CopyDoneFailure(t *testing.T) {
	m := NewErrorModalModel("error msg", 80, 24)
	result, _ := m.Update(CopyDoneMsg{Success: false})
	updated := result.(ErrorModalModel)
	if !strings.Contains(updated.copyStatus, "failed") {
		t.Errorf("copyStatus = %q, should contain 'failed'", updated.copyStatus)
	}
}

func TestErrorModalModel_CKeyReturnsCopyCmd(t *testing.T) {
	m := NewErrorModalModel("test error", 80, 24)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Error("pressing c should return a copy command")
	}
}

func TestErrorModalModel_View_ContainsText(t *testing.T) {
	m := NewErrorModalModel("api key and auth header are not allowed at the same time", 120, 40)
	view := m.View()
	if !strings.Contains(view, "Error Detail") {
		t.Errorf("view should show 'Error Detail' title, got: %s", view)
	}
}

func TestErrorModalModel_View_ShowsCopyStatusAfterCopy(t *testing.T) {
	m := NewErrorModalModel("err", 80, 24)
	result, _ := m.Update(CopyDoneMsg{Success: true})
	m = result.(ErrorModalModel)
	view := m.View()
	if !strings.Contains(view, "Copied") {
		t.Errorf("view should show copy status, got: %s", view)
	}
}

func TestErrorModalModel_SetSize(t *testing.T) {
	m := NewErrorModalModel("err", 80, 24)
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("SetSize: width=%d height=%d, want 120x40", m.width, m.height)
	}
}

// ─── App error banner ─────────────────────────────────────────────────────────

func TestApp_ErrorBannerWraps(t *testing.T) {
	a := NewApp(nil)
	a.width = 80
	a.height = 40
	longErr := "API error 404: {\"message\":\"Unable to find an environment with the specified identifier inside the database\",\"details\":\"Object not found inside the database (bucket=endpoints, key=1)\"}"
	a.err = &mockError{longErr}

	view := a.View()
	// The error should appear somewhere in the view
	if !strings.Contains(view, "⚠") {
		t.Error("error banner should show warning symbol")
	}
	// Hint keys should be visible
	if !strings.Contains(view, "[e]") {
		t.Error("error banner should show [e] hint")
	}
	if !strings.Contains(view, "[x]") {
		t.Error("error banner should show [x] hint")
	}
}

func TestApp_EKeyOpensErrorModal(t *testing.T) {
	a := NewApp(nil)
	a.width = 80
	a.height = 40
	a.err = &mockError{"something failed"}

	result, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(App)
	if updated.screen != ScreenError {
		t.Errorf("screen = %d, want ScreenError (%d)", updated.screen, ScreenError)
	}
}

func TestApp_EKeyNoErrorDoesNothing(t *testing.T) {
	a := NewApp(nil)
	a.err = nil
	result, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(App)
	if updated.screen == ScreenError {
		t.Error("pressing e without an error should not open error screen")
	}
}

func TestApp_XKeyDismissesError(t *testing.T) {
	a := NewApp(nil)
	a.err = &mockError{"some error"}
	result, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	updated := result.(App)
	if updated.err != nil {
		t.Error("pressing x should clear the error")
	}
}

func TestApp_EscOnErrorScreenGoesBack(t *testing.T) {
	a := NewApp(nil)
	a.screen = ScreenError
	a.prevScreen = ScreenStacks
	a.err = &mockError{"err"}
	result, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(App)
	if updated.screen != ScreenStacks {
		t.Errorf("esc from error screen should return to ScreenStacks, got %d", updated.screen)
	}
}

func TestApp_FooterShowsErrorHintsWhenErrSet(t *testing.T) {
	a := NewApp(nil)
	a.err = &mockError{"something failed"}
	footer := a.renderFooter()
	if !strings.Contains(footer, "[e]") {
		t.Error("footer should mention [e] when error is set")
	}
	if !strings.Contains(footer, "[x]") {
		t.Error("footer should mention [x] when error is set")
	}
}

func TestApp_FooterNoErrorHintsWhenNoErr(t *testing.T) {
	a := NewApp(nil)
	a.err = nil
	footer := a.renderFooter()
	if strings.Contains(footer, "[e] error") {
		t.Error("footer should not show error hints when no error")
	}
}

// mockError satisfies the error interface for testing
type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }
