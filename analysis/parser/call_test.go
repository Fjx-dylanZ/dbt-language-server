package parser

import (
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
)

// cursorCall parses input with `|` marking the cursor and returns the callee
// literal and argument index found there ("" when there is no call).
func cursorCall(t *testing.T, input string, known ...string) (string, int) {
	t.Helper()
	line, column := 0, 0
	for i := range len(input) {
		if input[i] == '|' {
			break
		}
		if input[i] == '\n' {
			line++
			column = 0
		} else {
			column++
		}
	}
	text := strings.Replace(input, "|", "", 1)
	if len(text) == len(input) {
		t.Fatalf("no cursor in %q", input)
	}
	names := map[string]bool{}
	for _, k := range known {
		names[k] = true
	}
	index := Parse(text, docs.Dialect("bigquery")).CreateTokenIndex()
	call, ok := index.CallAtCursor(line, column, func(tl TokenLL) bool {
		return names[strings.ToLower(tl.Token.Literal)]
	})
	if !ok {
		return "", -1
	}
	return call.Callee.Token.Literal, call.Argument
}

func TestCallAtCursor(t *testing.T) {
	known := []string{"date_trunc", "coalesce", "concat", "to_ny_date", "generate_surrogate_key", "sum", "count"}
	tests := []struct {
		name   string
		input  string
		callee string
		arg    int
	}{
		{"first argument", "select date_trunc(|created_at, day)", "date_trunc", 0},
		{"second argument", "select date_trunc(created_at, |day)", "date_trunc", 1},
		{"right after the paren", "select DATE_TRUNC(|", "DATE_TRUNC", 0},
		{"before the paren", "select date_trunc|(a, b)", "", -1},
		{"after the closing paren", "select date_trunc(a, b)| from t", "", -1},
		{"nested known call", "coalesce(a, date_trunc(x, |y), c)", "date_trunc", 1},
		{"subquery and in-list are transparent", "coalesce(a, (select x from t where y in (1, 2|)), c)", "coalesce", 1},
		{"comma inside a string literal", "concat(a, ', ', |b)", "concat", 2},
		{"cursor inside a string literal", "concat(a, 'x|y')", "concat", 1},
		{"string spanning tokens with a paren", "concat('(', a, |b)", "concat", 2},
		{"multi-line with an opaque jinja block", "select date_trunc(\n  {{ ts }},\n  |day)", "date_trunc", 1},
		{"jinja macro call", "{{ to_ny_date(|x) }}", "to_ny_date", 0},
		{"jinja list literal is transparent", "{{ generate_surrogate_key(['a', |'b']) }}", "generate_surrogate_key", 0},
		{"jinja package-qualified macro", "{{ dbt_utils.generate_surrogate_key(['a'], |2) }}", "generate_surrogate_key", 1},
		{"inside jinja without a call", "{{ x |}}", "", -1},
		{"jinja expression as a sql argument", "coalesce({{ x }}, |y)", "coalesce", 1},
		{"unknown callee is transparent", "coalesce(a, my_udf(b, |c))", "coalesce", 1},
		{"over clause is not a call", "sum(x) over (partition by |y)", "", -1},
		{"count star then more", "count(*), coalesce(|", "coalesce", 0},
		{"no parens at all", "select a, b |from t", "", -1},
	}
	for _, tt := range tests {
		callee, arg := cursorCall(t, tt.input, known...)
		if callee != tt.callee || arg != tt.arg {
			t.Errorf("%s: %q -> (%q, %d), want (%q, %d)", tt.name, tt.input, callee, arg, tt.callee, tt.arg)
		}
	}
}
