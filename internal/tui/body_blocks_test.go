package tui

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseIssueBodyBlocksIdentifiesHeadingsAndPreservesOrder(t *testing.T) {
	blocks := parseIssueBodyBlocks("Intro line\n\n# Goal\ntext\n## Details\n- item")

	want := []issueBodyBlock{
		{Kind: issueBodyTextBlock, Lines: []string{"Intro line", ""}},
		{Kind: issueBodyHeadingBlock, Level: 1, Text: "Goal", RawLine: "# Goal"},
		{Kind: issueBodyTextBlock, Lines: []string{"text"}},
		{Kind: issueBodyHeadingBlock, Level: 2, Text: "Details", RawLine: "## Details"},
		{Kind: issueBodyTextBlock, Lines: []string{"- item"}},
	}
	if !reflect.DeepEqual(blocks, want) {
		t.Fatalf("parseIssueBodyBlocks() = %#v, want %#v", blocks, want)
	}
}

func TestParseIssueBodyBlocksIgnoresHeadingsInsideFencedCode(t *testing.T) {
	blocks := parseIssueBodyBlocks("Before\n```go\n# not a heading\n```\n### Real heading")

	want := []issueBodyBlock{
		{Kind: issueBodyTextBlock, Lines: []string{"Before", "```go", "# not a heading", "```"}},
		{Kind: issueBodyHeadingBlock, Level: 3, Text: "Real heading", RawLine: "### Real heading"},
	}
	if !reflect.DeepEqual(blocks, want) {
		t.Fatalf("parseIssueBodyBlocks() = %#v, want %#v", blocks, want)
	}
}

func TestParseIssueBodyBlocksHandlesMarkdownLikeHeadingEdges(t *testing.T) {
	blocks := parseIssueBodyBlocks(strings.Join([]string{
		"### Trim closing markers ###",
		"#NoSpace",
		"####### Too many markers",
		"    # indented code, not a heading",
		"   ###### Six is ok",
	}, "\n"))

	want := []issueBodyBlock{
		{Kind: issueBodyHeadingBlock, Level: 3, Text: "Trim closing markers", RawLine: "### Trim closing markers ###"},
		{Kind: issueBodyTextBlock, Lines: []string{"#NoSpace", "####### Too many markers", "    # indented code, not a heading"}},
		{Kind: issueBodyHeadingBlock, Level: 6, Text: "Six is ok", RawLine: "   ###### Six is ok"},
	}
	if !reflect.DeepEqual(blocks, want) {
		t.Fatalf("parseIssueBodyBlocks() = %#v, want %#v", blocks, want)
	}
}

func TestRenderIssueBodyLinesMatchesLegacyWrappedBody(t *testing.T) {
	body := strings.TrimSpace(`Intro words wrap normally
# Heading
Details line
~~~md
## code heading stays text
~~~
After`)

	got := renderIssueBodyLines(body, 80)
	want := wrapText(body, 80)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renderIssueBodyLines() = %#v, want %#v", got, want)
	}
}
