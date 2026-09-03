package parser

import (
	"reflect"
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
)

func cteLiterals(p *Parser) []string {
	names := make([]string, 0, len(p.ctes.Tokens))
	for _, t := range p.ctes.Tokens {
		names = append(names, t.Literal)
	}
	return names
}

// CTE tracking must not depend on the dialect having a keyword table
// (bigquery has none; an unresolved profile yields "").
func TestParseCTEsAnyDialect(t *testing.T) {
	input := `WITH accounts AS (
    select * from {{ ref('stg_accounts') }}
),
final as (
    select * from accounts
)
select * from final`

	for _, dialect := range []string{"snowflake", "duckdb", "bigquery", ""} {
		p := Parse(input, docs.Dialect(dialect))
		got := cteLiterals(p)
		want := []string{"accounts", "final"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("dialect %q: ctes=%v, want %v", dialect, got, want)
		}
	}
}

func TestParseCTEForms(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name: "recursive",
			input: `with recursive tree as (
    select 1 as n union all select n + 1 from tree where n < 3
)
select * from tree`,
			want: []string{"tree"},
		},
		{
			name: "column list",
			input: `with pairs (a, b) as (
    select 1, 2
),
next as (
    select * from pairs
)
select * from next`,
			want: []string{"pairs", "next"},
		},
		{
			name: "chained column list",
			input: `with a as (select 1), b (x) as (select 2), c as (select 3)
select * from c`,
			want: []string{"a", "b", "c"},
		},
		{
			name: "unnest with offset is not a cte",
			input: `with base as (
    select arr from t
)
select pos from base, unnest(arr) with offset as pos
where f(pos), other`,
			want: []string{"base"},
		},
		{
			name: "comments with unbalanced parens inside body",
			input: `with a as (
    -- trailing paren ) here
    select 1 /* ( */ as x {# ( #}
),
b as (
    select * from a
)
select * from b`,
			want: []string{"a", "b"},
		},
		{
			name: "jinja call with nested parens",
			input: `with spine as (
    {{ dbt_utils.date_spine(datepart="day", start_date="cast('2020-01-01' as date)", end_date="current_date()") }}
),
final as (select * from spine)
select * from final`,
			want: []string{"spine", "final"},
		},
		{
			name: "dbt keyword as cte name",
			input: `with source as (
    select * from {{ source('app', 'users') }}
),
staged as (
    select source.id, ref from source
)
select * from staged`,
			want: []string{"source", "staged"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cteLiterals(Parse(tt.input, docs.Dialect("bigquery")))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ctes=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenNameMapIsCaseInsensitive(t *testing.T) {
	p := Parse(`with Orders as (select 1) select * from ORDERS`, docs.Dialect(""))
	m := p.CreateTokenNameMap()
	if _, ok := m["orders"]; !ok {
		t.Fatalf("expected lower-cased key, got %v", m)
	}
	if _, ok := m["Orders"]; ok {
		t.Fatalf("expected only lower-cased keys, got %v", m)
	}
}

func TestIdentifierReferences(t *testing.T) {
	input := `with orders as (
    select * from {{ ref('orders') }}
),
final as (
    select
        orders.id,
        x.orders,
        'orders' as orders
    from orders
    left join ORDERS as o on o.id = orders.id
)
select * from final`

	p := Parse(input, docs.Dialect("bigquery"))
	got := p.CreateTokenIndex().IdentifierReferences("orders")

	want := []Token{
		{Type: IDENT, Literal: "orders", Line: 0, Column: 5},  // definition
		{Type: IDENT, Literal: "orders", Line: 5, Column: 8},  // orders.id
		{Type: IDENT, Literal: "orders", Line: 8, Column: 9},  // from orders
		{Type: IDENT, Literal: "ORDERS", Line: 9, Column: 14}, // join ORDERS
		{Type: IDENT, Literal: "orders", Line: 9, Column: 36}, // = orders.id
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("references=\n%v\nwant\n%v", got, want)
	}
}
