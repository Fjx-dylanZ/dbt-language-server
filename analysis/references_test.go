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
