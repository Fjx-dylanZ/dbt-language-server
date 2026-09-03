package parser

import (
	"reflect"
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
)

func positions(tokens []Token) [][2]int {
	out := make([][2]int, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, [2]int{t.Line, t.Column})
	}
	return out
}

func TestJinjaVariableDefinitions(t *testing.T) {
	input := `{%- set airflow_principals = var('bq_airflow_principals', [
    '694223719049-compute@developer.gserviceaccount.com',
]) -%}
{% set a, b = 1, 2 %}
{% set ns.count = 0 %}
{% set block %}x{% endset %}
{% for k, v in d.items() %}{{ k }}{% endfor %}
{% macro m(x, y=join('(', sep), z) -%}
    {%- set inner = x %}
{%- endmacro %}`

	for _, dialect := range []string{"snowflake", ""} {
		p := Parse(input, docs.Dialect(dialect))
		vars := p.CreateJinjaVars()

		wantGlobal := map[string]Token{
			"airflow_principals": {Type: IDENT, Literal: "airflow_principals", Line: 0, Column: 8},
			"a":                  {Type: IDENT, Literal: "a", Line: 3, Column: 7},
			"b":                  {Type: IDENT, Literal: "b", Line: 3, Column: 10},
			"block":              {Type: IDENT, Literal: "block", Line: 5, Column: 7},
			"k":                  {Type: IDENT, Literal: "k", Line: 6, Column: 7},
			"v":                  {Type: IDENT, Literal: "v", Line: 6, Column: 10},
		}
		if !reflect.DeepEqual(vars.Global, wantGlobal) {
			t.Fatalf("dialect %q global=\n%v\nwant\n%v", dialect, vars.Global, wantGlobal)
		}

		if len(vars.Macros) != 1 {
			t.Fatalf("macros=%v", vars.Macros)
		}
		m := vars.Macros[0]
		if m.Start.Literal != "m" || m.End.Literal != "endmacro" {
			t.Fatalf("scope bounds = %v .. %v", m.Start, m.End)
		}
		wantDefs := map[string]Token{
			"x":     {Type: IDENT, Literal: "x", Line: 7, Column: 11},
			"y":     {Type: IDENT, Literal: "y", Line: 7, Column: 14},
			"z":     {Type: IDENT, Literal: "z", Line: 7, Column: 32},
			"inner": {Type: IDENT, Literal: "inner", Line: 8, Column: 12},
		}
		if !reflect.DeepEqual(m.Defs, wantDefs) {
			t.Fatalf("dialect %q macro defs=\n%v\nwant\n%v", dialect, m.Defs, wantDefs)
		}

		// `y=join(…)` reads like a keyword argument; its definition must still be listed.
		if got := positions(vars.References(p.CreateTokenIndex(), m.Defs["y"])); !reflect.DeepEqual(got, [][2]int{{7, 14}}) {
			t.Fatalf("dialect %q refs of y=%v", dialect, got)
		}
	}
}

func TestJinjaVariableReferences(t *testing.T) {
	input := `{% set p = 'x' %}
{% macro m(p, q) %}
    {{ p }} {{ q.p }} {{ f(p=1) }} {{ f(p == 1) }} {{ 'p' }} "p"
{% endmacro %}
{% for p in items %}{{ p }}{% endfor %}
select p from t where {{ p }}
{% macro n() %}{{ p }}{% endmacro %}`

	p := Parse(input, docs.Dialect("bigquery"))
	index := p.CreateTokenIndex()
	vars := p.CreateJinjaVars()

	global := vars.Global["p"]
	if got := positions(vars.References(index, global)); !reflect.DeepEqual(got, [][2]int{
		{0, 7},  // set p
		{4, 7},  // for p (merged into the top-level scope)
		{4, 23}, // {{ p }} in the loop
		{5, 25}, // {{ p }} in SQL context
		{6, 18}, // {{ p }} inside macro n, which has no p of its own
	}) {
		t.Fatalf("global refs=%v", got)
	}

	param := vars.Macros[0].Defs["p"]
	if got := positions(vars.References(index, param)); !reflect.DeepEqual(got, [][2]int{
		{1, 11}, // parameter
		{2, 7},  // {{ p }}; q.p, p=1, 'p', "p" excluded; p == 1 included
		{2, 40},
	}) {
		t.Fatalf("param refs=%v", got)
	}

	// Resolution from a use site picks the innermost scope.
	use, err := index.FindTokenAtCursor(2, 7)
	if err != nil || !use.Jinja {
		t.Fatalf("token at 2:7 = %v, %v", use, err)
	}
	if def, ok := vars.Resolve(use.Token); !ok || def != param {
		t.Fatalf("resolve(2:7) = %v, %v", def, ok)
	}
	sqlP, _ := index.FindTokenAtCursor(5, 7)
	if sqlP.Jinja {
		t.Fatalf("SQL identifier flagged as Jinja: %v", sqlP)
	}
}
