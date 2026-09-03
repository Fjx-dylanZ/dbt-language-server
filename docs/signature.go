package docs

import (
	"strings"
	"unicode/utf16"
)

// Signature is a call form as shown by textDocument/signatureHelp.
type Signature struct {
	Label string
	// Parameters are [start, end) UTF-16 offsets into Label, one per argument.
	Parameters [][2]int
	// Documentation is markdown describing the call; empty when unknown.
	Documentation string
}

// ParseSignature splits a function's docs into its call form and description.
// The call form is the first ```sql block, cut at its first blank line so
// clause definitions that follow (`over_clause: …`) stay in the documentation.
func ParseSignature(doc string) (Signature, bool) {
	const fence = "```sql\n"
	start := strings.Index(doc, fence)
	if start == -1 {
		return Signature{}, false
	}
	start += len(fence)
	end := strings.Index(doc[start:], "```")
	if end == -1 {
		return Signature{}, false
	}
	block := doc[start : start+end]
	if cut := strings.Index(block, "\n\n"); cut != -1 {
		block = block[:cut]
	}
	sig := SignatureFromLabel(strings.TrimSpace(block))
	sig.Documentation = strings.TrimSpace(doc[start+end+3:])
	return sig, true
}

// SignatureFromLabel derives the parameter spans of a call form such as
// `DATE_TRUNC(date_value, date_granularity)`: the text inside the outermost
// parentheses split on top-level commas (brackets and quotes are respected).
func SignatureFromLabel(label string) Signature {
	sig := Signature{Label: label}
	open := strings.IndexByte(label, '(')
	if open == -1 {
		return sig
	}

	depth := 0
	var quote byte
	argStart := open + 1
	flush := func(end int) {
		if s, e := trimSpan(label, argStart, end); s < e {
			sig.Parameters = append(sig.Parameters, [2]int{utf16Len(label[:s]), utf16Len(label[:e])})
		}
	}
	for i := open; i < len(label); i++ {
		c := label[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 {
				flush(i)
				return sig
			}
		case c == ',' && depth == 1:
			flush(i)
			argStart = i + 1
		}
	}
	flush(len(label))
	return sig
}

func trimSpan(s string, start, end int) (int, int) {
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return start, end
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
