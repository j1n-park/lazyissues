package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazyissues/internal/issues"
)

type focusPane int

const (
	focusList focusPane = iota
	focusDetail
)

const (
	minListWidth = 28
	maxListWidth = 52
)

var (
	appStyle = lipgloss.NewStyle()

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	focusedPaneStyle = paneStyle.Copy().BorderForeground(lipgloss.Color("39"))

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("238")).Bold(true)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
)

// Model is the read-only lazyissues Bubble Tea model.
type Model struct {
	issues            []issues.Issue
	dbPath            string
	selected          int
	detailScroll      int
	width             int
	height            int
	showHelp          bool
	focus             focusPane
	err               error
	collapsedSections map[issueSectionKey]bool
}

type issueSectionKey struct {
	issueID   int64
	sectionID issueBodySectionID
}

// NewModel creates a read-only TUI model for browsing issues.
func NewModel(issueList []issues.Issue, dbPath string) Model {
	return Model{
		issues: issueList,
		dbPath: dbPath,
		focus:  focusList,
	}
}

// NewErrorModel creates a model that renders a startup error state.
func NewErrorModel(err error, dbPath string) Model {
	return Model{
		dbPath: dbPath,
		err:    err,
	}
}

func (m Model) selectedIssue() (issues.Issue, bool) {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return issues.Issue{}, false
	}
	return m.issues[m.selected], true
}

func (m Model) sectionCollapsed(issueID int64, sectionID issueBodySectionID) bool {
	return m.collapsedSections[issueSectionKey{issueID: issueID, sectionID: sectionID}]
}

func (m *Model) setSelectedSectionCollapsed(sectionID issueBodySectionID, collapsed bool) {
	issue, ok := m.selectedIssue()
	if !ok {
		m.detailScroll = 0
		return
	}
	m.setSectionCollapsed(issue.ID, sectionID, collapsed)
}

func (m *Model) setSectionCollapsed(issueID int64, sectionID issueBodySectionID, collapsed bool) {
	if sectionID == "" {
		return
	}
	key := issueSectionKey{issueID: issueID, sectionID: sectionID}
	if collapsed {
		if m.collapsedSections == nil {
			m.collapsedSections = make(map[issueSectionKey]bool)
		}
		m.collapsedSections[key] = true
	} else if m.collapsedSections != nil {
		delete(m.collapsedSections, key)
		if len(m.collapsedSections) == 0 {
			m.collapsedSections = nil
		}
	}
	m.clampDetailScroll()
}

// WithSize returns a copy of the model sized for static rendering or tests.
func (m Model) WithSize(width, height int) Model {
	m.width = width
	m.height = height
	m.clampDetailScroll()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampDetailScroll()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "tab":
			m.toggleFocus()
			return m, nil
		case "left", "h":
			m.focus = focusList
			return m, nil
		case "right", "l":
			m.focus = focusDetail
			return m, nil
		}

		if m.err != nil || len(m.issues) == 0 {
			return m, nil
		}

		if m.focus == focusDetail {
			m.updateDetailNavigation(msg.String())
		} else {
			m.updateListNavigation(msg.String())
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) toggleFocus() {
	if m.focus == focusList {
		m.focus = focusDetail
		return
	}
	m.focus = focusList
}

func (m *Model) updateListNavigation(key string) {
	page := max(1, m.visibleListItems()-1)
	switch key {
	case "up", "k":
		m.moveSelection(-1)
	case "down", "j":
		m.moveSelection(1)
	case "pgup":
		m.moveSelection(-page)
	case "pgdown":
		m.moveSelection(page)
	case "home":
		m.setSelection(0)
	case "end":
		m.setSelection(len(m.issues) - 1)
	}
}

func (m *Model) updateDetailNavigation(key string) {
	page := max(1, m.detailViewportHeight()-1)
	switch key {
	case "up", "k":
		m.detailScroll--
	case "down", "j":
		m.detailScroll++
	case "pgup":
		m.detailScroll -= page
	case "pgdown":
		m.detailScroll += page
	case "home":
		m.detailScroll = 0
	case "end":
		m.detailScroll = m.maxDetailScroll()
	}
	m.clampDetailScroll()
}

func (m *Model) moveSelection(delta int) {
	m.setSelection(m.selected + delta)
}

func (m *Model) setSelection(index int) {
	if len(m.issues) == 0 {
		m.selected = 0
		m.detailScroll = 0
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.issues) {
		index = len(m.issues) - 1
	}
	if index != m.selected {
		m.selected = index
		m.detailScroll = 0
		return
	}
	m.clampDetailScroll()
}

func (m *Model) clampDetailScroll() {
	maxScroll := m.maxDetailScroll()
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "Loading lazyissues…\n"
	}

	bodyWidth := max(20, m.width)
	footerLines := m.footerLines(bodyWidth)
	paneHeight := max(6, m.height-len(footerLines))

	if m.err != nil {
		content := errorStyle.Render("Unable to load issues") + "\n\n" + wrapJoin(m.err.Error(), bodyWidth-4) + "\n\n" + subtleStyle.Render("Database: "+m.dbPath)
		return appStyle.Width(bodyWidth).Render(content + "\n" + strings.Join(footerLines, "\n"))
	}

	listWidth, detailWidth := m.paneWidths(bodyWidth)
	list := m.renderList(listWidth, paneHeight)
	detail := m.renderDetail(detailWidth, paneHeight)
	main := lipgloss.JoinHorizontal(lipgloss.Top, list, " ", detail)
	return appStyle.Width(bodyWidth).Render(main + "\n" + strings.Join(footerLines, "\n"))
}

func (m Model) paneWidths(totalWidth int) (int, int) {
	gap := 1
	available := max(20, totalWidth-gap)
	if available < minListWidth+30 {
		listWidth := max(8, available/2)
		detailWidth := available - listWidth
		if detailWidth < 12 {
			detailWidth = 12
			listWidth = max(8, available-detailWidth)
		}
		return listWidth, detailWidth
	}

	listWidth := available / 3
	if available >= 100 {
		listWidth = available * 2 / 5
	}
	listWidth = clamp(listWidth, minListWidth, maxListWidth)
	if available-listWidth < 30 {
		listWidth = max(minListWidth, available-30)
	}
	detailWidth := available - listWidth
	return listWidth, detailWidth
}

func (m Model) renderList(width, height int) string {
	style := paneStyle
	if m.focus == focusList {
		style = focusedPaneStyle
	}
	innerWidth := max(1, width-style.GetHorizontalFrameSize())
	innerHeight := max(1, height-style.GetVerticalFrameSize())

	lines := []string{titleStyle.Render("Issues") + " " + subtleStyle.Render(fmt.Sprintf("%d", len(m.issues)))}
	if len(m.issues) == 0 {
		lines = append(lines, "", "No issues found.", subtleStyle.Render("Try a different --db path or create issues first."))
		return style.Width(innerWidth).Height(innerHeight).Render(fitLines(lines, innerHeight))
	}

	visible := max(1, (innerHeight-2)/2)
	start := listWindowStart(m.selected, len(m.issues), visible)
	end := min(len(m.issues), start+visible)
	if start > 0 {
		lines = append(lines, subtleStyle.Render(fmt.Sprintf("↑ %d more", start)))
	}
	for i := start; i < end; i++ {
		issue := m.issues[i]
		selected := i == m.selected
		cursor := " "
		if selected {
			cursor = "›"
		}
		first := fmt.Sprintf("%s #%d %s", cursor, issue.ID, truncate(issue.Title, max(8, innerWidth-lipgloss.Width(cursor)-lipgloss.Width(fmt.Sprintf(" #%d ", issue.ID)))))
		second := fmt.Sprintf("  %s %s · %s", badge(issue.State, stateColor(issue.State)), badge(blankDefault(issue.Status, "no-status"), statusColor(issue.Status)), readableTime(issue.UpdatedAt))
		second = truncate(second, innerWidth)
		if selected {
			first = selectedStyle.Render(padRight(truncate(first, innerWidth), innerWidth))
			second = selectedStyle.Render(padRight(second, innerWidth))
		} else {
			first = truncate(first, innerWidth)
			second = subtleStyle.Render(second)
		}
		lines = append(lines, first, second)
	}
	if end < len(m.issues) {
		lines = append(lines, subtleStyle.Render(fmt.Sprintf("↓ %d more", len(m.issues)-end)))
	}

	return style.Width(innerWidth).Height(innerHeight).Render(fitLines(lines, innerHeight))
}

func (m Model) renderDetail(width, height int) string {
	style := paneStyle
	if m.focus == focusDetail {
		style = focusedPaneStyle
	}
	innerWidth := max(1, width-style.GetHorizontalFrameSize())
	innerHeight := max(1, height-style.GetVerticalFrameSize())

	if len(m.issues) == 0 {
		lines := []string{titleStyle.Render("No issue selected"), "", "The issue database loaded successfully, but it contains no rows."}
		return style.Width(innerWidth).Height(innerHeight).Render(fitLines(lines, innerHeight))
	}

	lines := m.detailLines(innerWidth)
	maxScroll := max(0, len(lines)-innerHeight)
	scroll := clamp(m.detailScroll, 0, maxScroll)
	end := min(len(lines), scroll+innerHeight)
	visible := append([]string{}, lines[scroll:end]...)
	if maxScroll > 0 {
		scrollInfo := subtleStyle.Render(fmt.Sprintf(" lines %d-%d/%d ", scroll+1, end, len(lines)))
		if len(visible) == innerHeight {
			visible[len(visible)-1] = scrollInfo
		} else {
			visible = append(visible, scrollInfo)
		}
	}
	return style.Width(innerWidth).Height(innerHeight).Render(fitLines(visible, innerHeight))
}

func (m Model) detailLines(width int) []string {
	issue, ok := m.selectedIssue()
	if !ok {
		return nil
	}
	lines := []string{
		titleStyle.Render(truncate(fmt.Sprintf("#%d %s", issue.ID, issue.Title), width)),
		fmt.Sprintf("%s %s", badge(issue.State, stateColor(issue.State)), badge(blankDefault(issue.Status, "no-status"), statusColor(issue.Status))),
		"",
	}
	meta := []string{
		fmt.Sprintf("State:   %s", blankDefault(issue.State, "unknown")),
		fmt.Sprintf("Status:  %s", blankDefault(issue.Status, "no-status")),
		fmt.Sprintf("Created: %s", readableTime(issue.CreatedAt)),
		fmt.Sprintf("Updated: %s", readableTime(issue.UpdatedAt)),
	}
	if issue.ParentID != nil {
		meta = append(meta, fmt.Sprintf("Parent:  #%d", *issue.ParentID))
	}
	if issue.Owner != nil && strings.TrimSpace(*issue.Owner) != "" {
		meta = append(meta, fmt.Sprintf("Owner:   %s", *issue.Owner))
	}
	if issue.BlockedReason != nil && strings.TrimSpace(*issue.BlockedReason) != "" {
		meta = append(meta, "Blocked:")
		meta = append(meta, indentLines(wrapText(*issue.BlockedReason, max(8, width-2)), "  ")...)
	}
	if issue.ClosedAt != nil && strings.TrimSpace(*issue.ClosedAt) != "" {
		meta = append(meta, fmt.Sprintf("Closed:  %s", readableTime(*issue.ClosedAt)))
	}
	for _, line := range meta {
		lines = append(lines, labelStyle.Render(truncate(line, width)))
	}
	lines = append(lines, "", titleStyle.Render("Body"), "")
	body := strings.TrimSpace(issue.Body)
	if body == "" {
		lines = append(lines, subtleStyle.Render("(empty body)"))
	} else {
		lines = append(lines, renderIssueBodyLinesWithCollapse(body, width, func(sectionID issueBodySectionID) bool {
			return m.sectionCollapsed(issue.ID, sectionID)
		})...)
	}
	return lines
}

func (m Model) footerLines(width int) []string {
	focus := "list"
	if m.focus == focusDetail {
		focus = "detail"
	}
	base := fmt.Sprintf("tab/h/l switch focus • j/k/↑/↓ navigate • pgup/pgdn/home/end • ? help • q quit • focus: %s", focus)
	lines := []string{helpStyle.Render(truncate(base, width))}
	if m.showHelp {
		lines = append(lines,
			helpStyle.Render(truncate("List focus: move between issues. Detail focus: scroll selected issue body.", width)),
			helpStyle.Render(truncate("Read-only browser: no issue actions mutate the database.", width)),
		)
	}
	return lines
}

func (m Model) visibleListItems() int {
	_, height := m.contentMetrics()
	return max(1, (height-2)/2)
}

func (m Model) detailViewportHeight() int {
	_, height := m.contentMetrics()
	return max(1, height)
}

func (m Model) contentMetrics() (int, int) {
	bodyWidth := max(20, m.width)
	footerLines := m.footerLines(bodyWidth)
	paneHeight := max(6, m.height-len(footerLines))
	_, detailWidth := m.paneWidths(bodyWidth)
	innerWidth := max(1, detailWidth-paneStyle.GetHorizontalFrameSize())
	innerHeight := max(1, paneHeight-paneStyle.GetVerticalFrameSize())
	return innerWidth, innerHeight
}

func (m Model) maxDetailScroll() int {
	if _, ok := m.selectedIssue(); !ok {
		return 0
	}
	innerWidth, innerHeight := m.contentMetrics()
	return max(0, len(m.detailLines(innerWidth))-innerHeight)
}

func listWindowStart(selected, total, visible int) int {
	if total <= visible {
		return 0
	}
	start := selected - visible/2
	if start < 0 {
		return 0
	}
	if start+visible > total {
		return total - visible
	}
	return start
}

func badge(text, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color(color)).Padding(0, 1).Render(strings.ToUpper(text))
}

func stateColor(state string) string {
	switch strings.ToLower(state) {
	case "open":
		return "36"
	case "closed":
		return "240"
	default:
		return "99"
	}
}

func statusColor(status string) string {
	switch strings.ToLower(status) {
	case "todo":
		return "63"
	case "in_progress":
		return "33"
	case "blocked":
		return "196"
	case "done":
		return "35"
	default:
		return "244"
	}
}

func readableTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Local().Format("2006-01-02 15:04")
		}
	}
	return value
}

func wrapJoin(value string, width int) string {
	return strings.Join(wrapText(value, max(8, width)), "\n")
}

func wrapText(value string, width int) []string {
	width = max(1, width)
	value = strings.ReplaceAll(value, "\t", "    ")
	paragraphs := strings.Split(value, "\n")
	var lines []string
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimRight(paragraph, " \r")
		if strings.TrimSpace(paragraph) == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		line := ""
		for _, word := range words {
			for lipgloss.Width(word) > width {
				part, rest := splitWidth(word, width)
				if line != "" {
					lines = append(lines, line)
					line = ""
				}
				lines = append(lines, part)
				word = rest
			}
			if line == "" {
				line = word
				continue
			}
			candidate := line + " " + word
			if lipgloss.Width(candidate) <= width {
				line = candidate
			} else {
				lines = append(lines, line)
				line = word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func splitWidth(value string, width int) (string, string) {
	if width <= 0 || value == "" {
		return "", value
	}
	count := 0
	for index, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if count+runeWidth > width {
			return value[:index], value[index:]
		}
		count += runeWidth
	}
	return value, ""
}

func indentLines(lines []string, prefix string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			result = append(result, prefix)
		} else {
			result = append(result, prefix+line)
		}
	}
	return result
}

func fitLines(lines []string, height int) string {
	if height <= 0 {
		return ""
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func padRight(value string, width int) string {
	for lipgloss.Width(value) < width {
		value += " "
	}
	return value
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	limit := width - 1
	result := ""
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		candidate := result + string(r)
		if lipgloss.Width(candidate) > limit {
			break
		}
		result = candidate
		value = value[size:]
	}
	return result + "…"
}

func blankDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func clamp(value, low, high int) int {
	if high < low {
		return low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
