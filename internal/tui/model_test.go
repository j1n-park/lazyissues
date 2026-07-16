package tui

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazyissues/internal/issues"
)

func TestTokyoNightPaletteBadgeColors(t *testing.T) {
	for _, tt := range []struct {
		name string
		got  string
		want string
	}{
		{"open state", stateColor("open"), "#24606f"},
		{"closed state", stateColor("closed"), "#414868"},
		{"unknown state", stateColor("unknown"), "#60458c"},
		{"todo status", statusColor("todo"), "#304f8a"},
		{"in-progress status", statusColor("in_progress"), "#695a32"},
		{"blocked status", statusColor("blocked"), "#713b50"},
		{"done status", statusColor("done"), "#42643d"},
		{"default status", statusColor(""), "#414868"},
		{"low thinking", thinkingColor("low"), "#414868"},
		{"medium thinking", thinkingColor("medium"), "#60458c"},
		{"high thinking", thinkingColor("high"), "#713b50"},
		{"default thinking", thinkingColor(""), "#414868"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("badge color = %q, want %q", tt.got, tt.want)
			}
		})
	}

	if got := appStyle.GetBackground(); got != lipgloss.Color("#1a1b26") {
		t.Errorf("app background = %v, want #1a1b26", got)
	}
	if got := paneStyle.GetForeground(); got != lipgloss.Color("#c0caf5") {
		t.Errorf("pane foreground = %v, want #c0caf5", got)
	}
	if got := paneStyle.GetBackground(); got != lipgloss.Color("#1a1b26") {
		t.Errorf("pane background = %v, want #1a1b26", got)
	}
}

func TestViewRendersReadableStatusesAndHelp(t *testing.T) {
	model := NewModel(renderFixtureIssues(), "./example_issues.db").WithSize(120, 28)
	model.selected = 2
	model.showHelp = true

	view := stripANSI(model.View())

	for _, want := range []string{
		"Issues 4",
		"OPEN",
		"CLOSED",
		"TODO",
		"IN_PROGRESS",
		"BLOCKED",
		"DONE",
		"LOW",
		"MEDIUM",
		"HIGH",
		"Thinking: high",
		"Blocked:",
		"waiting for review from release owner",
		"[/] previous/next heading",
		"automatically refresh every second",
		"Read-only browser: no issue actions mutate the database.",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
}

func TestListModelBadgeOmitsProviderPrefix(t *testing.T) {
	withModel := issues.Issue{ID: 1, Title: "delegated", State: "open", Model: "openai/gpt-5", CreatedAt: "2026-01-01", UpdatedAt: "2026-01-02"}
	list := stripANSI(strings.Join(NewModel(nil, "test.db").renderListIssue(withModel, false, 100), "\n"))
	if !strings.Contains(list, "GPT-5") {
		t.Fatalf("list model badge missing model name:\n%s", list)
	}
	if strings.Contains(list, "OPENAI/GPT-5") {
		t.Fatalf("list model badge rendered provider prefix:\n%s", list)
	}

	detail := stripANSI(strings.Join(NewModel(nil, "test.db").detailPrefixLines(withModel, 100), "\n"))
	if !strings.Contains(detail, "Model:    openai/gpt-5") {
		t.Fatalf("detail model metadata changed:\n%s", detail)
	}

	withoutModel := withModel
	withoutModel.Model = ""
	view := stripANSI(NewModel([]issues.Issue{withoutModel}, "test.db").WithSize(120, 20).View())
	if strings.Contains(view, "Model:") {
		t.Fatalf("View() rendered empty model metadata:\n%s", view)
	}
}

func TestViewRendersEmptyAndErrorStates(t *testing.T) {
	empty := stripANSI(NewModel(nil, "./empty.db").WithSize(90, 18).View())
	for _, want := range []string{"No issues found.", "No issue selected", "database loaded successfully"} {
		if !strings.Contains(empty, want) {
			t.Fatalf("empty View() missing %q:\n%s", want, empty)
		}
	}

	errView := stripANSI(NewErrorModel(errors.New("database is missing"), "./missing.db").WithSize(80, 12).View())
	for _, want := range []string{"Unable to load issues", "database is missing", "Database: ./missing.db"} {
		if !strings.Contains(errView, want) {
			t.Fatalf("error View() missing %q:\n%s", want, errView)
		}
	}
}

func TestWrapTextAndTruncateHelpers(t *testing.T) {
	wrapped := wrapText("alpha beta gamma", 10)
	if got, want := strings.Join(wrapped, "|"), "alpha beta|gamma"; got != want {
		t.Fatalf("wrapText() = %q, want %q", got, want)
	}

	longWord := wrapText("abcdefghij", 4)
	if got, want := strings.Join(longWord, "|"), "abcd|efgh|ij"; got != want {
		t.Fatalf("wrapText(long word) = %q, want %q", got, want)
	}

	if got, want := truncate("abcdef", 4), "abc…"; got != want {
		t.Fatalf("truncate() = %q, want %q", got, want)
	}
}

func TestListWindowStartKeepsSelectionVisible(t *testing.T) {
	tests := []struct {
		name     string
		selected int
		total    int
		visible  int
		want     int
	}{
		{name: "all visible", selected: 4, total: 5, visible: 5, want: 0},
		{name: "near top", selected: 1, total: 10, visible: 4, want: 0},
		{name: "middle", selected: 6, total: 10, visible: 4, want: 4},
		{name: "near bottom", selected: 9, total: 10, visible: 4, want: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listWindowStart(tt.selected, tt.total, tt.visible); got != tt.want {
				t.Fatalf("listWindowStart(%d, %d, %d) = %d, want %d", tt.selected, tt.total, tt.visible, got, tt.want)
			}
		})
	}
}

func TestLongTitlesWrapInListAndDetailWithoutEllipsis(t *testing.T) {
	title := "Implement complete title rendering without ellipsis across both panes"
	issue := issues.Issue{ID: 2, Title: title, State: "open", Status: "todo", Thinking: "medium"}
	model := NewModel([]issues.Issue{
		{ID: 1, Title: "before", State: "open"},
		issue,
		{ID: 3, Title: "after", State: "open"},
	}, "./issues.db").WithSize(70, 10)
	model.selected = 1

	bodyWidth := max(20, model.width)
	paneHeight := max(6, model.height-len(model.footerLines(bodyWidth)))
	listWidth, detailWidth := model.paneWidths(bodyWidth)
	listInnerWidth := listWidth - paneStyle.GetHorizontalFrameSize()
	list := stripANSI(model.renderList(listWidth, paneHeight))
	for _, line := range wrapText("› #2 "+title, listInnerWidth) {
		if !strings.Contains(list, line) {
			t.Fatalf("list omitted wrapped title line %q:\n%s", line, list)
		}
		if lipgloss.Width(line) > listInnerWidth {
			t.Fatalf("list title line width = %d, want <= %d", lipgloss.Width(line), listInnerWidth)
		}
	}
	if strings.Index(list, "OPEN") < strings.Index(list, "panes") {
		t.Fatalf("list metadata appeared before the complete title:\n%s", list)
	}

	detailInnerWidth := detailWidth - paneStyle.GetHorizontalFrameSize()
	detail := stripANSI(strings.Join(model.detailPrefixLines(issue, detailInnerWidth), "\n"))
	for _, line := range wrapText("#2 "+title, detailInnerWidth) {
		if !strings.Contains(detail, line) {
			t.Fatalf("detail omitted wrapped title line %q:\n%s", line, detail)
		}
		if lipgloss.Width(line) > detailInnerWidth {
			t.Fatalf("detail title line width = %d, want <= %d", lipgloss.Width(line), detailInnerWidth)
		}
	}
}

func TestListWindowKeepsSelectedVariableHeightRowVisible(t *testing.T) {
	start, end := listWindow(1, []int{2, 5, 2}, 7)
	if start != 1 || end != 2 {
		t.Fatalf("listWindow() = (%d, %d), want selected row only", start, end)
	}
}

func TestDetailAlwaysRendersAllBodyContentWithoutDisclosurePrefixes(t *testing.T) {
	body := strings.TrimSpace(`# Goal
Goal details
## Nested
Nested details
# Next
Next details`)
	model := NewModel([]issues.Issue{{ID: 11, Title: "one", Body: body, State: "open"}}, "./issues.db").WithSize(90, 18)
	text := stripANSI(strings.Join(model.detailLines(80), "\n"))
	for _, want := range []string{"Goal", "Goal details", "Nested", "Nested details", "Next", "Next details"} {
		if !strings.Contains(text, want) {
			t.Fatalf("detail missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "▾") || strings.Contains(text, "▸") {
		t.Fatalf("detail included a disclosure prefix:\n%s", text)
	}
}

func TestDetailFocusSectionNavigationKeys(t *testing.T) {
	body := strings.TrimSpace(`# First
First details
## Nested
Nested details
# Second
Second details
line 01
line 02
line 03
line 04
line 05
line 06`)
	model := NewModel([]issues.Issue{{ID: 42, Title: "sections", Body: body, State: "open"}}, "./issues.db").WithSize(90, 10)
	model.focus = focusDetail
	sections := model.selectedIssueSectionLines()
	model.detailScroll = sections[0].StartLine
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if got, want := model.detailScroll, 0; got != want {
		t.Fatalf("[ detailScroll = %d, want detail top %d", got, want)
	}
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if got, want := model.detailScroll, 0; got != want {
		t.Fatalf("[ at detail top moved to %d, want %d", got, want)
	}
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if got, want := model.detailScroll, sections[0].StartLine; got != want {
		t.Fatalf("] detailScroll = %d, want first section line %d", got, want)
	}
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if got, want := model.detailScroll, sections[1].StartLine; got != want {
		t.Fatalf("] detailScroll = %d, want next section line %d", got, want)
	}
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if got, want := model.detailScroll, sections[2].StartLine; got != want {
		t.Fatalf("] did not reach final heading: detailScroll = %d, want %d", got, want)
	}
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if got, want := model.detailScroll, sections[2].StartLine; got != want {
		t.Fatalf("] at final heading moved to %d, want it to stay at %d", got, want)
	}
}

func TestInitSchedulesAutoRefreshWhenLoaderIsConfigured(t *testing.T) {
	model := NewModel(nil, "./issues.db").WithIssueLoader(func(ctx context.Context) ([]issues.Issue, error) {
		return nil, nil
	})
	if cmd := model.Init(); cmd == nil {
		t.Fatal("Init should schedule automatic refresh when an issue loader is configured")
	}

	model = NewModel(nil, "./issues.db")
	if cmd := model.Init(); cmd != nil {
		t.Fatal("Init should not schedule automatic refresh without an issue loader")
	}
}

func TestAutoRefreshTickReloadsIssuesAndPreservesSelectionByID(t *testing.T) {
	model := NewModel([]issues.Issue{
		{ID: 1, Title: "one", Body: "old", State: "open"},
		{ID: 2, Title: "two", Body: "old", State: "open"},
	}, "./issues.db").WithIssueLoader(func(ctx context.Context) ([]issues.Issue, error) {
		return []issues.Issue{
			{ID: 3, Title: "three", Body: "new", State: "open"},
			{ID: 2, Title: "two updated", Body: "new", State: "open"},
		}, nil
	}).WithSize(90, 18)
	model.selected = 1

	updated, cmd := model.Update(autoRefreshTickMsg{})
	if cmd == nil {
		t.Fatal("automatic refresh tick should return a refresh command")
	}
	model = updated.(Model)
	updated, cmd = model.Update(cmd())
	if cmd == nil {
		t.Fatal("refresh completion should schedule the next automatic refresh")
	}
	model = updated.(Model)

	if got, want := len(model.issues), 2; got != want {
		t.Fatalf("issues length after refresh = %d, want %d", got, want)
	}
	if got, want := model.issues[model.selected].ID, int64(2); got != want {
		t.Fatalf("selected issue ID after refresh = %d, want %d", got, want)
	}
	if got, want := model.issues[model.selected].Title, "two updated"; got != want {
		t.Fatalf("selected issue title after refresh = %q, want %q", got, want)
	}
}

func TestRKeyNoLongerRefreshesIssues(t *testing.T) {
	called := false
	model := NewModel([]issues.Issue{{ID: 1, Title: "one", Body: "old", State: "open"}}, "./issues.db").WithIssueLoader(func(ctx context.Context) ([]issues.Issue, error) {
		called = true
		return []issues.Issue{{ID: 2, Title: "refreshed", Body: "new", State: "open"}}, nil
	}).WithSize(90, 18)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("r should no longer return a refresh command")
	}
	if called {
		t.Fatal("r should not call the issue loader")
	}
	if got, want := model.issues[model.selected].Title, "one"; got != want {
		t.Fatalf("selected issue title after r = %q, want %q", got, want)
	}
}

func TestRefreshErrorIsDisplayedAndCanRecover(t *testing.T) {
	refreshErr := errors.New("database is locked")
	model := NewModel([]issues.Issue{{ID: 1, Title: "one", Body: "old", State: "open"}}, "./issues.db").WithSize(90, 18)

	updated, _ := model.Update(refreshIssuesMsg{err: refreshErr})
	model = updated.(Model)
	if model.err == nil || model.err.Error() != refreshErr.Error() {
		t.Fatalf("refresh error = %v, want %v", model.err, refreshErr)
	}
	if view := stripANSI(model.View()); !strings.Contains(view, "database is locked") {
		t.Fatalf("error view missing refresh error:\n%s", view)
	}

	updated, _ = model.Update(refreshIssuesMsg{issues: []issues.Issue{{ID: 2, Title: "recovered", Body: "new", State: "open"}}})
	model = updated.(Model)
	if model.err != nil {
		t.Fatalf("refresh success should clear error, got %v", model.err)
	}
	if got, want := model.issues[model.selected].Title, "recovered"; got != want {
		t.Fatalf("selected issue title after recovery = %q, want %q", got, want)
	}
}

func updateModelWithKey(t *testing.T, model Model, key tea.KeyMsg) Model {
	t.Helper()
	updated, _ := model.Update(key)
	updatedModel, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	return updatedModel
}

func renderFixtureIssues() []issues.Issue {
	closedAt := "2026-07-03T10:00:00Z"
	blockedReason := "waiting for review from release owner"
	return []issues.Issue{
		{
			ID:        1,
			Title:     "Build read-only browser",
			Body:      "Initial TUI browsing support.",
			State:     "open",
			Status:    "in_progress",
			Thinking:  "medium",
			CreatedAt: "2026-07-01T10:00:00Z",
			UpdatedAt: "2026-07-03T09:00:00Z",
		},
		{
			ID:        2,
			Title:     "Document command line usage",
			Body:      "Add install and run notes.",
			State:     "open",
			Status:    "todo",
			Thinking:  "low",
			CreatedAt: "2026-07-01T10:00:00Z",
			UpdatedAt: "2026-07-03T08:00:00Z",
		},
		{
			ID:            3,
			Title:         "Polish release validation",
			Body:          "Check rendering and release smoke output.",
			State:         "open",
			Status:        "blocked",
			Thinking:      "high",
			BlockedReason: &blockedReason,
			CreatedAt:     "2026-07-01T10:00:00Z",
			UpdatedAt:     "2026-07-03T07:00:00Z",
		},
		{
			ID:        4,
			Title:     "Ship first iteration",
			Body:      "Done.",
			State:     "closed",
			Status:    "done",
			Thinking:  "medium",
			CreatedAt: "2026-07-01T10:00:00Z",
			UpdatedAt: "2026-07-03T06:00:00Z",
			ClosedAt:  &closedAt,
		},
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}
