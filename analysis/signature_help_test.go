package analysis

import (
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
)

func TestSignatureHelp(t *testing.T) {
	state, uri, _ := openCustomersModel(t)
	state.DbtContext.Dialect = "bigquery"
	state.parseDocument(uri, strings.Join([]string{
		"select date_trunc(",
		"  {{ full_name('a', 'b') }},",
		"  day) as d,",
		"  coalesce(a, b, c, d) as e,",
		"  {{ full_name('x', 'y') }}",
		"from t",
	}, "\n"))

	help := func(line, character int) *lsp.SignatureHelp {
		return state.SignatureHelp(1, uri, lsp.Position{Line: line, Character: character}).Result
	}
	label := func(h *lsp.SignatureHelp, p int) string {
		span := h.Signatures[0].Parameters[p].Label
		return h.Signatures[0].Label[span[0]:span[1]]
	}

	// SQL function, second argument, across a Jinja block.
	got := help(2, 2)
	if got == nil || got.Signatures[0].Label != "DATE_TRUNC(date_value, date_granularity)" || got.ActiveParameter != 1 {
		t.Fatalf("date_trunc: %+v", got)
	}
	if label(got, 1) != "date_granularity" || got.Signatures[0].Documentation == nil || got.Signatures[0].Documentation.Kind != "markdown" {
		t.Fatalf("date_trunc signature detail: %+v", got.Signatures[0])
	}

	// Macro call inside Jinja, second argument.
	got = help(1, 20)
	if got == nil || got.Signatures[0].Label != "full_name(first_name, last_name)" || got.ActiveParameter != 1 || label(got, 1) != "last_name" {
		t.Fatalf("macro: %+v", got)
	}

	// Variadic: beyond the last documented parameter it stays active.
	got = help(3, 20)
	if got == nil || got.Signatures[0].Label != "COALESCE(expr[, ...])" || got.ActiveParameter != 0 {
		t.Fatalf("coalesce: %+v", got)
	}

	// Outside any call.
	if got = help(5, 3); got != nil {
		t.Fatalf("expected null outside a call, got %+v", got)
	}

	// Unknown dialect: no function signatures, macros still work.
	state.DbtContext.Dialect = ""
	if got = help(2, 2); got != nil {
		t.Fatalf("expected null without a dialect, got %+v", got)
	}
	if got = help(1, 20); got == nil {
		t.Fatal("macro signature should not depend on the dialect")
	}
}

func TestHoverFunction(t *testing.T) {
	state, uri, _ := openCustomersModel(t)
	state.DbtContext.Dialect = "bigquery"
	state.parseDocument(uri, "select DATE_TRUNC(t.date, day), net.host(u), safe.substr(s, 1), o.substr from t")

	hover := func(character int) string {
		return state.Hover(1, uri, lsp.Position{Line: 0, Character: character}).Result.Contents
	}
	if got := hover(9); !strings.HasPrefix(got, "```sql\nDATE_TRUNC(") {
		t.Fatalf("upper-case function: %q", got)
	}
	if got := hover(20); got != "" {
		t.Fatalf("alias.column must not resolve to DATE(): %q", got)
	}
	if got := hover(37); !strings.HasPrefix(got, "```sql\nNET.HOST(") {
		t.Fatalf("dotted namespace: %q", got)
	}
	if got := hover(51); !strings.HasPrefix(got, "```sql\nSUBSTR(") {
		t.Fatalf("safe. prefix: %q", got)
	}
	if got := hover(67); got != "" {
		t.Fatalf("alias.substr is a column: %q", got)
	}
}
