package tui

import "strings"

type issueBodyBlockKind int

const (
	issueBodyTextBlock issueBodyBlockKind = iota
	issueBodyHeadingBlock
)

type issueBodyBlock struct {
	Kind    issueBodyBlockKind
	Level   int
	Text    string
	RawLine string
	Lines   []string
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
	blocks := parseIssueBodyBlocks(body)
	if len(blocks) == 0 {
		return nil
	}

	var lines []string
	for _, block := range blocks {
		switch block.Kind {
		case issueBodyHeadingBlock:
			lines = append(lines, wrapText(block.RawLine, width)...)
		case issueBodyTextBlock:
			lines = append(lines, wrapText(strings.Join(block.Lines, "\n"), width)...)
		}
	}
	return lines
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
