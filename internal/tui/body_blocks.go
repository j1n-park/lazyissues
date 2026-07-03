package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type issueBodyBlockKind int

const (
	issueBodyTextBlock issueBodyBlockKind = iota
	issueBodyHeadingBlock
)

const (
	issueBodyHeadingMarker          = "▾"
	issueBodyCollapsedHeadingMarker = "▸"
)

var issueBodyHeadingStyles = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("57")).Bold(true),
	lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("24")).Bold(true),
	lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("30")).Bold(true),
	lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("59")).Bold(true),
	lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("60")).Bold(true),
	lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("238")).Bold(true),
}

type issueBodyBlock struct {
	Kind    issueBodyBlockKind
	Level   int
	Text    string
	RawLine string
	Lines   []string
}

type issueBodySectionID string

func issueBodyHeadingSectionID(block issueBodyBlock, ordinal int) issueBodySectionID {
	return issueBodySectionID(fmt.Sprintf("heading:%d:%d:%s", ordinal, block.Level, block.RawLine))
}

func parseIssueBodyBlocks(body string) []issueBodyBlock {
	if body == "" {
		return nil
	}

	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	blocks := make([]issueBodyBlock, 0, len(lines))
	textLines := make([]string, 0, len(lines))
	inFence := false
	var fenceChar byte
	var fenceLength int

	flushText := func() {
		if len(textLines) == 0 {
			return
		}
		blockLines := append([]string(nil), textLines...)
		blocks = append(blocks, issueBodyBlock{Kind: issueBodyTextBlock, Lines: blockLines})
		textLines = textLines[:0]
	}

	for _, rawLine := range lines {
		line := strings.TrimSuffix(rawLine, "\r")
		if inFence {
			textLines = append(textLines, line)
			if isClosingFence(line, fenceChar, fenceLength) {
				inFence = false
			}
			continue
		}

		if char, length, ok := openingFence(line); ok {
			textLines = append(textLines, line)
			inFence = true
			fenceChar = char
			fenceLength = length
			continue
		}

		if level, text, ok := parseATXHeading(line); ok {
			flushText()
			blocks = append(blocks, issueBodyBlock{
				Kind:    issueBodyHeadingBlock,
				Level:   level,
				Text:    text,
				RawLine: line,
			})
			continue
		}

		textLines = append(textLines, line)
	}

	flushText()
	return blocks
}

func renderIssueBodyLines(body string, width int) []string {
	return renderIssueBodyLinesWithCollapse(body, width, nil)
}

func renderIssueBodyLinesWithCollapse(body string, width int, isCollapsed func(issueBodySectionID) bool) []string {
	blocks := parseIssueBodyBlocks(body)
	if len(blocks) == 0 {
		return nil
	}

	var lines []string
	headingOrdinal := 0
	collapsedLevel := 0
	for _, block := range blocks {
		switch block.Kind {
		case issueBodyHeadingBlock:
			headingOrdinal++
			if collapsedLevel > 0 {
				if block.Level > collapsedLevel {
					continue
				}
				collapsedLevel = 0
			}

			sectionID := issueBodyHeadingSectionID(block, headingOrdinal)
			collapsed := isCollapsed != nil && isCollapsed(sectionID)
			lines = append(lines, renderIssueBodyHeadingLinesWithMarker(block, width, headingMarker(collapsed))...)
			if collapsed {
				collapsedLevel = block.Level
			}
		case issueBodyTextBlock:
			if collapsedLevel > 0 {
				continue
			}
			lines = append(lines, wrapText(strings.Join(block.Lines, "\n"), width)...)
		}
	}
	return lines
}

func renderIssueBodyHeadingLines(block issueBodyBlock, width int) []string {
	return renderIssueBodyHeadingLinesWithMarker(block, width, issueBodyHeadingMarker)
}

func renderIssueBodyHeadingLinesWithMarker(block issueBodyBlock, width int, marker string) []string {
	width = max(1, width)
	prefix := marker + " "
	continuationPrefix := strings.Repeat(" ", lipgloss.Width(prefix))
	text := strings.TrimSpace(block.Text)
	contentWidth := max(1, width-lipgloss.Width(prefix))
	wrapped := wrapText(text, contentWidth)
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}

	style := issueBodyHeadingStyle(block.Level).Width(width)
	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		linePrefix := prefix
		if i > 0 {
			linePrefix = continuationPrefix
		}
		lines = append(lines, style.Render(truncate(linePrefix+line, width)))
	}
	return lines
}

func headingMarker(collapsed bool) string {
	if collapsed {
		return issueBodyCollapsedHeadingMarker
	}
	return issueBodyHeadingMarker
}

func issueBodyHeadingStyle(level int) lipgloss.Style {
	if len(issueBodyHeadingStyles) == 0 {
		return lipgloss.NewStyle()
	}
	return issueBodyHeadingStyles[clamp(level, 1, len(issueBodyHeadingStyles))-1]
}

func parseATXHeading(line string) (level int, text string, ok bool) {
	start := 0
	for start < len(line) && line[start] == ' ' {
		start++
	}
	if start > 3 || start >= len(line) || line[start] != '#' {
		return 0, "", false
	}

	end := start
	for end < len(line) && line[end] == '#' {
		end++
	}
	level = end - start
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if end < len(line) && line[end] != ' ' && line[end] != '\t' {
		return 0, "", false
	}

	text = strings.TrimSpace(line[end:])
	if text != "" {
		trimmedEnd := len(text)
		for trimmedEnd > 0 && text[trimmedEnd-1] == '#' {
			trimmedEnd--
		}
		if trimmedEnd < len(text) && (trimmedEnd == 0 || text[trimmedEnd-1] == ' ' || text[trimmedEnd-1] == '\t') {
			text = strings.TrimSpace(text[:trimmedEnd])
		}
	}

	return level, text, true
}

func openingFence(line string) (char byte, length int, ok bool) {
	char, length, _, ok = fencePrefix(line)
	return char, length, ok
}

func isClosingFence(line string, char byte, length int) bool {
	closingChar, closingLength, rest, ok := fencePrefix(line)
	if !ok || closingChar != char || closingLength < length {
		return false
	}
	return strings.Trim(rest, " \t") == ""
}

func fencePrefix(line string) (char byte, length int, rest string, ok bool) {
	start := 0
	for start < len(line) && line[start] == ' ' {
		start++
	}
	if start > 3 || start >= len(line) {
		return 0, 0, "", false
	}

	char = line[start]
	if char != '`' && char != '~' {
		return 0, 0, "", false
	}

	end := start
	for end < len(line) && line[end] == char {
		end++
	}
	length = end - start
	if length < 3 {
		return 0, 0, "", false
	}

	return char, length, line[end:], true
}
