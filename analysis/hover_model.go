package analysis

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// modelHoverMarkdown renders a ref() hover: model name, description, and the
// column schema declared in the model's properties YAML.
func modelHoverMarkdown(name string, model ModelDetails) string {
	if model.URI == "" {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s**", name)
	if model.ProjectName != "" {
		fmt.Fprintf(&b, " · %s", model.ProjectName)
	}
	b.WriteString("\n")

	if desc := strings.TrimSpace(model.Description); desc != "" {
		b.WriteString("\n")
		b.WriteString(desc)
		b.WriteString("\n")
	}

	if len(model.Columns) == 0 {
		return b.String()
	}

	// The type column only earns its space when some column declares data_type.
	typed := false
	for _, c := range model.Columns {
		if strings.TrimSpace(c.DataType) != "" {
			typed = true
			break
		}
	}
	header := []string{"column", "description"}
	if typed {
		header = []string{"column", "type", "description"}
	}
	rows := make([][]string, 0, len(model.Columns))
	for _, c := range model.Columns {
		row := []string{"`" + c.Name + "`", hoverCell(c.Description)}
		if typed {
			row = []string{"`" + c.Name + "`", strings.TrimSpace(c.DataType), hoverCell(c.Description)}
		}
		rows = append(rows, row)
	}

	b.WriteString("\n")
	writeMarkdownTable(&b, header, rows)
	return b.String()
}

// writeMarkdownTable writes a pipe table with cells padded to their column
// width, so it reads as a table even where markdown is only highlighted, not
// rendered (Neovim hover windows).
func writeMarkdownTable(b *strings.Builder, header []string, rows [][]string) {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = max(3, utf8.RuneCountInString(h))
	}
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}

	writeRow := func(cells []string) {
		b.WriteString("|")
		for i, cell := range cells {
			b.WriteString(" ")
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell)))
			b.WriteString(" |")
		}
		b.WriteString("\n")
	}

	writeRow(header)
	separators := make([]string, len(header))
	for i, w := range widths {
		separators[i] = strings.Repeat("-", w)
	}
	writeRow(separators)
	for _, row := range rows {
		writeRow(row)
	}
}

// hoverCell flattens a description into one table cell: first line only,
// pipes escaped so they cannot break the row.
func hoverCell(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return strings.ReplaceAll(s, "|", `\|`)
}
