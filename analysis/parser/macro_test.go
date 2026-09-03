package parser

import (
	"reflect"
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
)

func tokensOfType(p *Parser, tt TokenType) []string {
	var out []string
	for _, t := range p.tokens {
		if t.Token.Type == tt {
			out = append(out, t.Token.Literal)
		}
	}
	return out
}

func TestParseMacroCallsInsideJinja(t *testing.T) {
	input := `{%- set watermark = get_watermark('x', var('days', 7)) %}
{% if is_incremental() %}
select
    {{ outer(inner() + [1], 'y') }} as a,
    {{ pkg.helper(z) }} as b,
    {{ this }},
    coalesce(sum(amount), 0) as c
from {{ ref('t') }}
{% endif %}`

	p := Parse(input, docs.Dialect("bigquery"))

	wantMacros := []string{"get_watermark", "is_incremental", "outer", "inner", "helper"}
	if got := tokensOfType(p, MACRO); !reflect.DeepEqual(got, wantMacros) {
		t.Fatalf("MACRO tokens=%v, want %v", got, wantMacros)
	}
	if got := tokensOfType(p, PACKAGE); !reflect.DeepEqual(got, []string{"pkg"}) {
		t.Fatalf("PACKAGE tokens=%v, want [pkg]", got)
	}
	// SQL function calls outside Jinja stay identifiers.
	for _, ident := range []string{"coalesce", "sum", "amount"} {
		found := false
		for _, lit := range tokensOfType(p, IDENT) {
			if lit == ident {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected %q to remain IDENT", ident)
		}
	}
}

func TestParseMacroDefinitionName(t *testing.T) {
	p := Parse(`{% macro to_ny_date(ts_expr) -%}
    date({{ ts_expr }}, 'America/New_York')
{%- endmacro %}`, docs.Dialect("bigquery"))

	if got := tokensOfType(p, MACRO); !reflect.DeepEqual(got, []string{"to_ny_date"}) {
		t.Fatalf("MACRO tokens=%v, want [to_ny_date]", got)
	}
}
