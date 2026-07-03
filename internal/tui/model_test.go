package tui

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazyissues/internal/issues"
)

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
		"Blocked:",
		"waiting for review from release owner",
		"enter/space toggle",
		"a expand all",
		"z collapse all",
		"Read-only browser: no issue actions mutate the database.",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
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

func TestSectionCollapseStateDefaultsExpandedAndIsScopedByIssue(t *testing.T) {
	body1 := strings.TrimSpace(`# Goal
Issue one details
## Nested
Nested details
# Next
Next details`)
	body2 := strings.Replace(body1, "Issue one details", "Issue two details", 1)
	model := NewModel([]issues.Issue{
		{ID: 11, Title: "one", Body: body1, State: "open"},
		{ID: 22, Title: "two", Body: body2, State: "open"},
	}, "./issues.db").WithSize(90, 18)
	sectionID := headingSectionIDAt(t, body1, 1)

	if model.sectionCollapsed(11, sectionID) {
		t.Fatal("sections should be expanded by default")
	}
	text := stripANSI(strings.Join(model.detailLines(80), "\n"))
	for _, want := range []string{"▾ Goal", "Issue one details", "Nested details", "▾ Next", "Next details"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expanded detail missing %q:\n%s", want, text)
		}
	}

	model.setSelectedSectionCollapsed(sectionID, true)
	text = stripANSI(strings.Join(model.detailLines(80), "\n"))
	for _, want := range []string{"▸ Goal", "▾ Next", "Next details"} {
		if !strings.Contains(text, want) {
			t.Fatalf("collapsed detail missing %q:\n%s", want, text)
		}
	}
	for _, hidden := range []string{"Issue one details", "Nested details"} {
		if strings.Contains(text, hidden) {
			t.Fatalf("collapsed detail unexpectedly included %q:\n%s", hidden, text)
		}
	}

	model.setSelection(1)
	text = stripANSI(strings.Join(model.detailLines(80), "\n"))
	for _, want := range []string{"▾ Goal", "Issue two details", "Nested details"} {
		if !strings.Contains(text, want) {
			t.Fatalf("second issue should remain expanded and include %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "▸ Goal") {
		t.Fatalf("collapse state leaked to second issue:\n%s", text)
	}

	model.setSelection(0)
	model.setSelectedSectionCollapsed(sectionID, false)
	text = stripANSI(strings.Join(model.detailLines(80), "\n"))
	if !strings.Contains(text, "Issue one details") || strings.Contains(text, "▸ Goal") {
		t.Fatalf("expanded section did not restore issue one body:\n%s", text)
	}
}

func TestCollapseStateClampsDetailScrollAndSelectionReset(t *testing.T) {
	longBody := "# Long\n" + strings.Join([]string{
		"line 01", "line 02", "line 03", "line 04", "line 05",
		"line 06", "line 07", "line 08", "line 09", "line 10",
		"line 11", "line 12", "line 13", "line 14", "line 15",
	}, "\n")
	model := NewModel([]issues.Issue{
		{ID: 1, Title: "long", Body: longBody, State: "open"},
		{ID: 2, Title: "short", Body: "short body", State: "open"},
	}, "./issues.db").WithSize(80, 8)
	sectionID := headingSectionIDAt(t, longBody, 1)

	model.detailScroll = model.maxDetailScroll()
	if model.detailScroll == 0 {
		t.Fatal("test fixture should produce scrollable detail content")
	}
	model.setSelectedSectionCollapsed(sectionID, true)
	if maxScroll := model.maxDetailScroll(); model.detailScroll > maxScroll {
		t.Fatalf("detailScroll = %d after collapse, want <= %d", model.detailScroll, maxScroll)
	}

	model.detailScroll = 999
	model.setSelection(1)
	if model.detailScroll != 0 {
		t.Fatalf("detailScroll = %d after selected issue changed, want 0", model.detailScroll)
	}
}

func TestDetailFocusTogglesSectionAtScrollWithEnterAndSpace(t *testing.T) {
	body := strings.TrimSpace(`# Goal
Goal details
## Nested
Nested details
# Next
Next details`)
	model := NewModel([]issues.Issue{{ID: 11, Title: "one", Body: body, State: "open"}}, "./issues.db").WithSize(90, 18)
	model.focus = focusDetail
	goalSectionID := headingSectionIDAt(t, body, 1)
	sections := model.selectedIssueSectionLines()
	if len(sections) == 0 {
		t.Fatal("test fixture should produce section lines")
	}
	model.detailScroll = sections[0].StartLine + 1

	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if !model.sectionCollapsed(11, goalSectionID) {
		t.Fatal("enter should collapse the section at or before the detail scroll position")
	}
	text := stripANSI(strings.Join(model.detailLines(80), "\n"))
	if strings.Contains(text, "Goal details") || strings.Contains(text, "Nested details") {
		t.Fatalf("collapsed detail unexpectedly included hidden section content:\n%s", text)
	}

	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if model.sectionCollapsed(11, goalSectionID) {
		t.Fatal("space should expand the section at or before the detail scroll position")
	}
	text = stripANSI(strings.Join(model.detailLines(80), "\n"))
	if !strings.Contains(text, "Goal details") || !strings.Contains(text, "Nested details") {
		t.Fatalf("expanded detail missing restored section content:\n%s", text)
	}
}

func TestDetailFocusExpandAllCollapseAllAndSectionNavigationKeys(t *testing.T) {
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
	sectionIDs := issueBodySectionIDs(body)
	if len(sectionIDs) != 3 {
		t.Fatalf("sectionIDs length = %d, want 3", len(sectionIDs))
	}

	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	for _, sectionID := range sectionIDs {
		if !model.sectionCollapsed(42, sectionID) {
			t.Fatalf("z should collapse all sections; %q remained expanded", sectionID)
		}
	}
	text := stripANSI(strings.Join(model.detailLines(80), "\n"))
	if strings.Contains(text, "First details") || strings.Contains(text, "Nested details") || strings.Contains(text, "Second details") {
		t.Fatalf("collapse-all unexpectedly included hidden section content:\n%s", text)
	}

	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	for _, sectionID := range sectionIDs {
		if model.sectionCollapsed(42, sectionID) {
			t.Fatalf("a should expand all sections; %q remained collapsed", sectionID)
		}
	}

	sections := model.selectedIssueSectionLines()
	model.detailScroll = sections[0].StartLine
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if got, want := model.detailScroll, sections[1].StartLine; got != want {
		t.Fatalf("] detailScroll = %d, want next section line %d", got, want)
	}
	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if got, want := model.detailScroll, sections[0].StartLine; got != want {
		t.Fatalf("[ detailScroll = %d, want previous section line %d", got, want)
	}
}

func TestSectionKeysOnlyApplyWhenDetailPaneIsFocused(t *testing.T) {
	body := "# Goal\nDetails"
	model := NewModel([]issues.Issue{{ID: 7, Title: "one", Body: body, State: "open"}}, "./issues.db").WithSize(90, 18)
	model.focus = focusList
	model.detailScroll = model.selectedIssueSectionLines()[0].StartLine

	model = updateModelWithKey(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.sectionCollapsed(7, headingSectionIDAt(t, body, 1)) {
		t.Fatal("enter should not toggle sections while list pane is focused")
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

func headingSectionIDAt(t *testing.T, body string, ordinal int) issueBodySectionID {
	t.Helper()
	headingOrdinal := 0
	for _, block := range parseIssueBodyBlocks(body) {
		if block.Kind != issueBodyHeadingBlock {
			continue
		}
		headingOrdinal++
		if headingOrdinal == ordinal {
			return issueBodyHeadingSectionID(block, headingOrdinal)
		}
	}
	t.Fatalf("heading ordinal %d not found in body", ordinal)
	return ""
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
			CreatedAt: "2026-07-01T10:00:00Z",
			UpdatedAt: "2026-07-03T09:00:00Z",
		},
		{
			ID:        2,
			Title:     "Document command line usage",
			Body:      "Add install and run notes.",
			State:     "open",
			Status:    "todo",
			CreatedAt: "2026-07-01T10:00:00Z",
			UpdatedAt: "2026-07-03T08:00:00Z",
		},
		{
			ID:            3,
			Title:         "Polish release validation",
			Body:          "Check rendering and release smoke output.",
			State:         "open",
			Status:        "blocked",
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
