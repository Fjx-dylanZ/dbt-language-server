package analysis

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/testutils"
	"github.com/j-clemons/dbt-language-server/util"
)

func openCustomersModel(t *testing.T) (*State, string, string) {
	t.Helper()
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		t.Fatal(err)
	}
	state := NewState()
	state.refreshDbtContext(testdataRoot)

	path := filepath.Join(testdataRoot, "models/customers.sql")
	text, err := util.ReadFileContents(path)
	if err != nil {
		t.Fatal(err)
	}
	uri := "file://" + path
	state.parseDocument(uri, text)
	return &state, uri, testdataRoot
}

func span(uri string, line, start, end int) lsp.Location {
	return lsp.Location{
		URI: uri,
		Range: lsp.Range{
			Start: lsp.Position{Line: line, Character: start},
			End:   lsp.Position{Line: line, Character: end},
		},
	}
}

// customers.sql: `orders as (` on line 6; used on lines 32, 41, 46, 47, 49.
var ordersDef = [3]int{6, 0, 6}
var ordersUses = [][3]int{{32, 9, 15}, {41, 8, 14}, {46, 14, 20}, {47, 29, 35}, {49, 13, 19}}

func TestDefinitionCTE(t *testing.T) {
	state, uri, _ := openCustomersModel(t)

	for _, use := range ordersUses {
		got := state.Definition(1, uri, lsp.Position{Line: use[0], Character: use[1] + 2}).Result
		want := span(uri, ordersDef[0], ordersDef[1], ordersDef[2])
		if got != want {
			t.Fatalf("definition from %v = %+v, want %+v", use, got, want)
		}
	}

	// Not a CTE: definition stays at the cursor.
	got := state.Definition(2, uri, lsp.Position{Line: 29, Character: 12}).Result // order_date
	if got != span(uri, 29, 12, 12) {
		t.Fatalf("expected non-CTE identifier to stay at the cursor, got %+v", got)
	}
}

func TestReferencesCTE(t *testing.T) {
	state, uri, _ := openCustomersModel(t)

	var uses []lsp.Location
	for _, u := range ordersUses {
		uses = append(uses, span(uri, u[0], u[1], u[2]))
	}
	withDecl := append([]lsp.Location{span(uri, ordersDef[0], ordersDef[1], ordersDef[2])}, uses...)

	// From a use site and from the definition itself.
	for _, pos := range []lsp.Position{{Line: 32, Character: 10}, {Line: 6, Character: 2}} {
		got := state.References(1, uri, pos, true).Result
		if !reflect.DeepEqual(got, withDecl) {
			t.Fatalf("references from %+v =\n%+v\nwant\n%+v", pos, got, withDecl)
		}
		got = state.References(2, uri, pos, false).Result
		if !reflect.DeepEqual(got, uses) {
			t.Fatalf("references (no decl) from %+v =\n%+v\nwant\n%+v", pos, got, uses)
		}
	}

	// Not a CTE: nothing.
	if got := state.References(3, uri, lsp.Position{Line: 29, Character: 12}, true).Result; len(got) != 0 {
		t.Fatalf("expected no references for a plain column, got %+v", got)
	}
}

func TestReferencesRef(t *testing.T) {
	state, uri, root := openCustomersModel(t)

	// cursor inside ref('stg_orders') on line 8
	got := state.References(1, uri, lsp.Position{Line: 8, Character: 28}, true).Result

	stgOrders := "file://" + filepath.Join(root, "models/staging/stg_orders.sql")
	ordersModel := "file://" + filepath.Join(root, "models/orders.sql")
	want := []lsp.Location{
		{URI: stgOrders},
		span(uri, 8, 26, 36),
		span(ordersModel, 4, 26, 36),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("references =\n%+v\nwant\n%+v", got, want)
	}

	// Unsaved edits in an open document are honoured.
	state.parseDocument(ordersModel, "select * from {{ ref(\"stg_orders\") }}, {{ ref('jaffle_shop', 'stg_orders') }}")
	got = state.References(2, uri, lsp.Position{Line: 8, Character: 28}, false).Result
	want = []lsp.Location{
		span(uri, 8, 26, 36),
		span(ordersModel, 0, 22, 32),
		span(ordersModel, 0, 62, 72),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("references (open doc) =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDefinitionMacroInStatementBlock(t *testing.T) {
	state, uri, root := openCustomersModel(t)
	macros := "file://" + filepath.Join(root, "macros/jaffle_macros.sql")

	// Nested call inside a {% set %} block: both calls resolve.
	state.parseDocument(uri, "{%- set v = times_five(full_name('a', 'b')) %}")
	if got := state.Definition(1, uri, lsp.Position{Line: 0, Character: 14}).Result; got.URI != macros || got.Range.Start.Line != 6 {
		t.Fatalf("times_five definition = %+v", got)
	}
	if got := state.Definition(2, uri, lsp.Position{Line: 0, Character: 26}).Result; got.URI != macros || got.Range.Start.Line != 0 {
		t.Fatalf("full_name definition = %+v", got)
	}
	if got := state.Hover(3, uri, lsp.Position{Line: 0, Character: 26}).Result.Contents; got != "full_name(first_name, last_name)" {
		t.Fatalf("full_name hover = %q", got)
	}
}

func TestReferencesMacro(t *testing.T) {
	state, uri, root := openCustomersModel(t)
	macros := "file://" + filepath.Join(root, "macros/jaffle_macros.sql")

	// From a call site in customers.sql (line 61) and from the definition itself.
	macroDoc, err := util.ReadFileContents(filepath.Join(root, "macros/jaffle_macros.sql"))
	if err != nil {
		t.Fatal(err)
	}
	state.parseDocument(macros, macroDoc)

	want := []lsp.Location{
		span(macros, 0, 9, 18),
		span(uri, 60, 11, 20),
	}
	for _, at := range []struct {
		uri string
		pos lsp.Position
	}{
		{uri, lsp.Position{Line: 60, Character: 14}},
		{macros, lsp.Position{Line: 0, Character: 12}},
	} {
		got := state.References(1, at.uri, at.pos, true).Result
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("references from %s %+v =\n%+v\nwant\n%+v", at.uri, at.pos, got, want)
		}
		got = state.References(2, at.uri, at.pos, false).Result
		if !reflect.DeepEqual(got, want[1:]) {
			t.Fatalf("references (no decl) from %s %+v =\n%+v\nwant\n%+v", at.uri, at.pos, got, want[1:])
		}
	}

	// Package macro called with its qualifier: jaffle_package.add_values on line 67.
	pkgMacros := "file://" + filepath.Join(root, "dbt_packages/jaffle_package/macros/jaffle_package_macros.sql")
	got := state.References(3, uri, lsp.Position{Line: 66, Character: 30}, true).Result
	want = []lsp.Location{
		span(pkgMacros, 0, 9, 19),
		span(uri, 66, 26, 36),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("package macro references =\n%+v\nwant\n%+v", got, want)
	}

	// Built-ins are not project macros: nothing.
	state.parseDocument(uri, "{% if is_incremental() %} select 1 {% endif %}")
	if got := state.References(4, uri, lsp.Position{Line: 0, Character: 8}, true).Result; len(got) != 0 {
		t.Fatalf("expected no references for a built-in, got %+v", got)
	}
}

func TestReferencesIgnoreJinjaComments(t *testing.T) {
	state, uri, root := openCustomersModel(t)
	ordersModel := "file://" + filepath.Join(root, "models/orders.sql")

	// Examples inside `{# … #}` are not rendered by dbt and must not count.
	state.parseDocument(uri, "{# usage: {{ full_name('a', 'b') }} from {{ ref('stg_orders') }} #}\n{{ full_name('c', 'd') }} {{ ref('stg_orders') }}")

	got := state.References(1, uri, lsp.Position{Line: 1, Character: 5}, false).Result
	if want := []lsp.Location{span(uri, 1, 3, 12)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("macro references =\n%+v\nwant\n%+v", got, want)
	}

	got = state.References(2, uri, lsp.Position{Line: 1, Character: 36}, false).Result
	if want := []lsp.Location{span(uri, 1, 34, 44), span(ordersModel, 4, 26, 36)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ref references =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDefinitionAndReferencesJinjaVariable(t *testing.T) {
	state, uri, _ := openCustomersModel(t)
	state.parseDocument(uri, `{% macro bq_consumer(user_email_expr) -%}
    {%- set airflow_principals = var('bq_airflow_principals', [
        '694223719049-compute@developer.gserviceaccount.com',
    ]) -%}
    case
        when {{ bq_principal_in(user_email_expr, airflow_principals) }} then 'airflow'
    end
{%- endmacro %}
{{ airflow_principals }}`)

	def := span(uri, 1, 12, 30)
	use := span(uri, 5, 49, 67)

	// gd from the use and from the definition itself.
	for _, pos := range []lsp.Position{{Line: 5, Character: 55}, {Line: 1, Character: 15}} {
		if got := state.Definition(1, uri, pos).Result; got != def {
			t.Fatalf("definition from %+v = %+v, want %+v", pos, got, def)
		}
	}
	// The macro parameter resolves to the signature.
	if got := state.Definition(2, uri, lsp.Position{Line: 5, Character: 35}).Result; got != span(uri, 0, 21, 36) {
		t.Fatalf("parameter definition = %+v", got)
	}

	got := state.References(3, uri, lsp.Position{Line: 5, Character: 55}, true).Result
	if want := []lsp.Location{def, use}; !reflect.DeepEqual(got, want) {
		t.Fatalf("references =\n%+v\nwant\n%+v", got, want)
	}
	got = state.References(4, uri, lsp.Position{Line: 5, Character: 55}, false).Result
	if want := []lsp.Location{use}; !reflect.DeepEqual(got, want) {
		t.Fatalf("references (no decl) =\n%+v\nwant\n%+v", got, want)
	}

	// Outside the macro the name is undefined: no definition, no references.
	if got := state.Definition(5, uri, lsp.Position{Line: 8, Character: 5}).Result; got != span(uri, 8, 5, 5) {
		t.Fatalf("expected the cursor to stay put outside the scope, got %+v", got)
	}
	if got := state.References(6, uri, lsp.Position{Line: 8, Character: 5}, true).Result; len(got) != 0 {
		t.Fatalf("expected no references outside the scope, got %+v", got)
	}
}
