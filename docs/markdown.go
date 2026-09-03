package docs

import "strings"

// sectionLabels are the headings the scraped function docs left as bare lines.
var sectionLabels = map[string]bool{
	"description":              true,
	"definitions":              true,
	"details":                  true,
	"arguments":                true,
	"constraints":              true,
	"output":                   true,
	"return type":              true,
	"return data type":         true,
	"return data types":        true,
	"returned data types":      true,
	"supported argument types": true,
	"supported input types":    true,
	"examples":                 true,
	"example":                  true,
	"usage notes":              true,
	"required":                 true,
	"optional":                 true,
	"note":                     true,
}

func isSectionLabel(line string) bool {
	l := strings.ToLower(line)
	return sectionLabels[l] || strings.HasSuffix(l, " definitions")
}

func isReferenceLink(line string) bool {
	return strings.HasPrefix(line, "[") && strings.Contains(line, "](")
}

// Markdown turns a scraped function doc into well-formed markdown: fenced code
// gets a closing fence on its own line with no padding blank lines, bare
// section labels become bold paragraphs, brackets in prose are escaped so
// `expr[, ...]` is not read as a link, and the trailing reference link stands
// on its own.
func Markdown(doc string) string {
	out := make([]string, 0, strings.Count(doc, "\n")+8)
	var code []string
	inCode := false

	blank := func() {
		if n := len(out); n > 0 && out[n-1] != "" {
			out = append(out, "")
		}
	}
	closeCode := func() {
		for len(code) > 0 && strings.TrimSpace(code[len(code)-1]) == "" {
			code = code[:len(code)-1]
		}
		out = append(out, code...)
		out = append(out, "```")
		code = code[:0]
		inCode = false
	}
	// A closing fence is the previous line: prose after it needs a paragraph break.
	afterFence := func() {
		if n := len(out); n > 0 && out[n-1] == "```" {
			out = append(out, "")
		}
	}

	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case inCode && trimmed == "```":
			closeCode()
		case inCode && strings.HasSuffix(trimmed, "```"):
			// Snowflake docs close fences at the end of the last code line.
			code = append(code, strings.TrimSuffix(strings.TrimRight(line, " \t"), "```"))
			closeCode()
		case inCode:
			if len(code) > 0 || trimmed != "" {
				code = append(code, line)
			}
		case strings.HasPrefix(trimmed, "```"):
			blank()
			out = append(out, trimmed)
			inCode = true
		case trimmed == "":
			blank()
		case isSectionLabel(trimmed):
			blank()
			out = append(out, "**"+trimmed+"**")
		case isReferenceLink(trimmed):
			blank()
			out = append(out, trimmed)
		default:
			afterFence()
			out = append(out, escapeBrackets(line))
		}
	}
	if inCode {
		closeCode()
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// escapeBrackets backslash-escapes `[` and `]` that are not escaped already.
func escapeBrackets(line string) string {
	if !strings.ContainsAny(line, "[]") {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + 4)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if (c == '[' || c == ']') && (i == 0 || line[i-1] != '\\') {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}
