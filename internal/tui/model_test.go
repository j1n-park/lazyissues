package tui

import (
	"errors"
	"regexp"
	"strings"
	"testing"

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
