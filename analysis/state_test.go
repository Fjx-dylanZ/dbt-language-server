package analysis

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/testutils"
	"github.com/j-clemons/dbt-language-server/util"
)

func expectedTestState() State {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		panic(err)
	}
	docsMd, err := util.ReadFileContents(filepath.Join(testdataRoot, "models/docs.md"))
	if err != nil {
		panic(err)
	}
	ordersStatusDoc := getDocsFileContents(docsMd)[0].Content

	expectedState := State{
		Documents: map[string]Document{},
		DbtContext: DbtContext{
			ProjectRoot: testdataRoot,
			ProjectYaml: DbtProjectYaml{
				ProjectName: AnnotatedField[string]{
					Value: "jaffle_shop",
					Position: lsp.Position{
						Line:      0,
						Character: 6,
					},
				},
				Profile: AnnotatedField[string]{
					Value: "jaffle_shop",
					Position: lsp.Position{
						Line:      5,
						Character: 9,
					},
				},
				ModelPaths: AnnotatedField[[]string]{
					Value: []string{"models"},
					Position: lsp.Position{
						Line:      7,
						Character: 13,
					},
				},
				SeedPaths: AnnotatedField[[]string]{
					Value: []string{"seeds"},
					Position: lsp.Position{
						Line:      8,
						Character: 12,
					},
				},
				MacroPaths: AnnotatedField[[]string]{
					Value: []string{"macros"},
					Position: lsp.Position{
						Line:      11,
						Character: 13,
					},
				},
				PackagesInstallPath: AnnotatedField[string]{
					Value: "dbt_packages",
					Position: lsp.Position{
						Line:      0,
						Character: 0,
					},
				},
				DocsPaths: AnnotatedField[[]string]{
					Value: []string{"models", "macros"},
					Position: lsp.Position{
						Line:      0,
						Character: 0,
					},
				},
				Vars: AnnotatedMap{
					"global_count": AnnotatedField[interface{}]{
						Value: 0,
						Position: lsp.Position{
							Line:      36,
							Character: 16,
						},
					},
					"jaffle_shop": AnnotatedField[interface{}]{
						Value: AnnotatedMap{
							"jaffle_number": AnnotatedField[interface{}]{
								Value: 1,
								Position: lsp.Position{
									Line:      40,
									Character: 19,
								},
							},
							"jaffle_string": AnnotatedField[interface{}]{
								Value: "jaffle",
								Position: lsp.Position{
									Line:      39,
									Character: 19,
								},
							},
						},
						Position: lsp.Position{
							Line:      39,
							Character: 4,
						},
					},
				},
			},
			Dialect: docs.Dialect("duckdb"),
			ModelDetailMap: map[string]ModelDetails{
				"customers": {
					URI:         filepath.Join(testdataRoot, "models/customers.sql"),
					ProjectName: "jaffle_shop",
					Description: "This table has basic information about a customer, as well as some derived facts based on a customer's orders",
					SchemaURI:   filepath.Join(testdataRoot, "models/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 3, Character: 10},
						End:   lsp.Position{Line: 3, Character: 10},
					},
					Columns: []Column{
						{Name: "customer_id", Description: "This is a unique identifier for a customer"},
						{Name: "first_name", Description: "Customer's first name. PII."},
						{Name: "last_name", Description: "Customer's last name. PII."},
						{Name: "first_order", Description: "Date (UTC) of a customer's first order"},
						{Name: "most_recent_order", Description: "Date (UTC) of a customer's most recent order"},
						{Name: "number_of_orders", Description: "Count of the number of orders a customer has placed"},
						{Name: "total_order_amount", Description: "Total value (AUD) of a customer's orders"},
					},
				},
				"orders": {
					URI:         filepath.Join(testdataRoot, "models/orders.sql"),
					ProjectName: "jaffle_shop",
					Description: "This table has basic information about orders, as well as some derived facts based on payments",
					SchemaURI:   filepath.Join(testdataRoot, "models/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 31, Character: 10},
						End:   lsp.Position{Line: 31, Character: 10},
					},
					Columns: []Column{
						{Name: "order_id", Description: "This is a unique identifier for an order"},
						{Name: "customer_id", Description: "Foreign key to the customers table"},
						{Name: "order_date", Description: "Date (UTC) that the order was placed"},
						{Name: "status", Description: ordersStatusDoc},
						{Name: "amount", Description: "Total amount (AUD) of the order"},
						{Name: "credit_card_amount", Description: "Amount of the order (AUD) paid for by credit card"},
						{Name: "coupon_amount", Description: "Amount of the order (AUD) paid for by coupon"},
						{Name: "bank_transfer_amount", Description: "Amount of the order (AUD) paid for by bank transfer"},
						{Name: "gift_card_amount", Description: "Amount of the order (AUD) paid for by gift card"},
					},
				},
				"stg_customer_status": {
					URI:         filepath.Join(testdataRoot, "dbt_packages/jaffle_package/models/stg_customer_status.sql"),
					ProjectName: "jaffle_package",
					Description: "",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
				"stg_customers": {
					URI:         filepath.Join(testdataRoot, "models/staging/stg_customers.sql"),
					ProjectName: "jaffle_shop",
					Description: "",
					SchemaURI:   filepath.Join(testdataRoot, "models/staging/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 3, Character: 10},
						End:   lsp.Position{Line: 3, Character: 10},
					},
					Columns: []Column{{Name: "customer_id"}},
				},
				"stg_orders": {
					URI:         filepath.Join(testdataRoot, "models/staging/stg_orders.sql"),
					ProjectName: "jaffle_shop",
					Description: "",
					SchemaURI:   filepath.Join(testdataRoot, "models/staging/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 10, Character: 10},
						End:   lsp.Position{Line: 10, Character: 10},
					},
					Columns: []Column{{Name: "order_id"}, {Name: "status"}},
				},
				"stg_payments": {
					URI:         filepath.Join(testdataRoot, "models/staging/stg_payments.sql"),
					ProjectName: "jaffle_shop",
					Description: "",
					SchemaURI:   filepath.Join(testdataRoot, "models/staging/schema.yml"),
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 21, Character: 10},
						End:   lsp.Position{Line: 21, Character: 10},
					},
					Columns: []Column{{Name: "payment_id"}, {Name: "payment_method"}},
				},
				"raw_customers": {
					URI:         filepath.Join(testdataRoot, "seeds/raw_customers.csv"),
					ProjectName: "jaffle_shop",
					Description: "Seed File",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
				"raw_orders": {
					URI:         filepath.Join(testdataRoot, "seeds/raw_orders.csv"),
					ProjectName: "jaffle_shop",
					Description: "Seed File",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
				"raw_payments": {
					URI:         filepath.Join(testdataRoot, "seeds/raw_payments.csv"),
					ProjectName: "jaffle_shop",
					Description: "Seed File",
					SchemaURI:   "",
					SchemaRange: lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
				},
			},
			SourceDetailMap: map[string]Source{
				"jaffle_shop": {
					Name:        "jaffle_shop",
					Description: "",
					URI:         filepath.Join(testdataRoot, "models/schema.yml"),
					Range: lsp.Range{
						Start: lsp.Position{Line: 84, Character: 10},
						End:   lsp.Position{Line: 84, Character: 10},
					},
					Tables: map[string]SourceTable{
						"customers": {
							Name:        "customers",
							Description: "",
							Table:       "jaffle_shop",
							URI:         filepath.Join(testdataRoot, "models/schema.yml"),
							Range: lsp.Range{
								Start: lsp.Position{Line: 89, Character: 14},
								End:   lsp.Position{Line: 89, Character: 14},
							},
						},
						"orders": {
							Name:        "orders",
							Description: "",
							Table:       "jaffle_shop",
							URI:         filepath.Join(testdataRoot, "models/schema.yml"),
							Range: lsp.Range{
								Start: lsp.Position{Line: 88, Character: 14},
								End:   lsp.Position{Line: 88, Character: 14},
							},
						},
					},
				},
				"stripe": {
					Name:        "stripe",
					Description: "",
					URI:         filepath.Join(testdataRoot, "models/schema.yml"),
					Range: lsp.Range{
						Start: lsp.Position{Line: 91, Character: 10},
						End:   lsp.Position{Line: 91, Character: 10},
					},
					Tables: map[string]SourceTable{
						"payments": {
							Name:        "payments",
							Description: "",
							Table:       "stripe",
							URI:         filepath.Join(testdataRoot, "models/schema.yml"),
							Range: lsp.Range{
								Start: lsp.Position{Line: 93, Character: 14},
								End:   lsp.Position{Line: 93, Character: 14},
							},
						},
					},
				},
			},
			MacroDetailMap: map[Package]map[string]Macro{
				"jaffle_package": {
					"add_values": {
						Name:        "add_values",
						ProjectName: "jaffle_package",
						Description: "add_values(arg1, arg2)",
						URI:         filepath.Join(testdataRoot, "dbt_packages/jaffle_package/macros/jaffle_package_macros.sql"),
						Range: lsp.Range{
							Start: lsp.Position{
								Line:      0,
								Character: 9,
							},
							End: lsp.Position{
								Line:      0,
								Character: 31,
							},
						},
					},
				},
				"jaffle_shop": {
					"full_name": {
						Name:        "full_name",
						ProjectName: "jaffle_shop",
						Description: "full_name(first_name, last_name)",
						URI:         filepath.Join(testdataRoot, "macros/jaffle_macros.sql"),
						Range: lsp.Range{
							Start: lsp.Position{
								Line:      0,
								Character: 9,
							},
							End: lsp.Position{
								Line:      0,
								Character: 41,
							},
						},
					},
					"times_five": {
						Name:        "times_five",
						ProjectName: "jaffle_shop",
						Description: "times_five(int_value)",
						URI:         filepath.Join(testdataRoot, "macros/jaffle_macros.sql"),
						Range: lsp.Range{
							Start: lsp.Position{
								Line:      6,
								Character: 10,
							},
							End: lsp.Position{
								Line:      6,
								Character: 31,
							},
						},
					},
				},
			},
			VariableDetailMap: map[string]Variable{
				"global_count": {
					Name:  "global_count",
					Value: 0,
					URI:   filepath.Join(testdataRoot, "dbt_project.yml"),
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      36,
							Character: 16,
						},
						End: lsp.Position{
							Line:      36,
							Character: 16,
						},
					},
				},
				"jaffle_number": {
					Name:  "jaffle_number",
					Value: 1,
					URI:   filepath.Join(testdataRoot, "dbt_project.yml"),
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      40,
							Character: 19,
						},
						End: lsp.Position{
							Line:      40,
							Character: 19,
						},
					},
				},
				"jaffle_string": {
					Name:  "jaffle_string",
					Value: "jaffle",
					URI:   filepath.Join(testdataRoot, "dbt_project.yml"),
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      39,
							Character: 19,
						},
						End: lsp.Position{
							Line:      39,
							Character: 19,
						},
					},
				},
			},
			RefNames: map[string]map[string]bool{
				"jaffle_shop": {
					"customers": true, "orders": true,
					"stg_customers": true, "stg_orders": true, "stg_payments": true,
					"raw_customers": true, "raw_orders": true, "raw_payments": true,
				},
				"jaffle_package": {"stg_customer_status": true},
			},
		},
		FusionEnabled: false,
	}

	return expectedState
}

func TestRefreshDbtContext(t *testing.T) {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		t.Fatal(err)
	}

	expectedState := expectedTestState()

	state := NewState()
	state.refreshDbtContext(testdataRoot)

	if !reflect.DeepEqual(state, expectedState) {
		t.Fatalf("expected %#v,\n\ngot %#v", expectedState, state)
	}
}

func BenchmarkRefreshDbtContext(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
		if err != nil {
			b.Fatal(err)
		}

		state := NewState()
		state.refreshDbtContext(testdataRoot)
	}
}

func TestHoverRef(t *testing.T) {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		t.Fatal(err)
	}
	state := NewState()
	state.refreshDbtContext(testdataRoot)

	uri := "file://" + filepath.Join(testdataRoot, "models/customers.sql")
	state.parseDocument(uri, "select * from {{ ref('orders') }}")
	// cursor inside 'orders'
	got := state.Hover(nil, uri, lsp.Position{Line: 0, Character: 24}).Result.Contents

	for _, want := range []string{
		"**orders** · jaffle_shop",
		"This table has basic information about orders",
		// no column declares data_type, so there is no type column; cells are padded
		"| column                 | description                                         |",
		"| ---------------------- | --------------------------------------------------- |",
		"| `order_id`             | This is a unique identifier for an order            |",
		// multi-line doc block collapses to its first line
		"| `status`               | Orders can be one of the following statuses:        |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("hover missing %q in:\n%s", want, got)
		}
	}
	width := 0
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if width == 0 {
			width = len(line)
		} else if len(line) != width {
			t.Fatalf("table rows are not aligned:\n%s", got)
		}
	}
	if strings.Contains(got, "| placed") {
		t.Fatalf("hover leaked doc-block table rows into a cell:\n%s", got)
	}

	// Unknown model: no hover content, and definition must not resolve.
	state.parseDocument(uri, "select * from {{ ref('nope') }}")
	if got := state.Hover(nil, uri, lsp.Position{Line: 0, Character: 24}).Result.Contents; got != "" {
		t.Fatalf("expected empty hover for unknown model, got %q", got)
	}
	if def := state.Definition(nil, uri, lsp.Position{Line: 0, Character: 24}); def.Result.URI != uri {
		t.Fatalf("expected definition to stay on the current document, got %q", def.Result.URI)
	}
}
