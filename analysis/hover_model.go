package analysis

import (
	"fmt"
	"strings"
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

	b.WriteString("\n| column | type | description |\n|---|---|---|\n")
	for _, c := range model.Columns {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", c.Name, c.DataType, hoverCell(c.Description))
	}
	return b.String()
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
