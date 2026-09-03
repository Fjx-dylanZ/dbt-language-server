package analysis

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/testutils"
)

func TestParsePropertiesYamlFile(t *testing.T) {
	testdataRoot, err := testutils.GetTestdataPath("jaffle_shop_duckdb")
	if err != nil {
		panic(err)
	}

	actualProperties := parsePropertiesYamlFile(
		filepath.Join(testdataRoot, "models/schema.yml"),
	)

	// Column entries in models/schema.yml: `- name:` value at column 14,
	// `description:` value at column 21 (0-based). Lines are 0-based.
	column := func(name string, nameLine int, description string, descriptionLine int) ColumnProperties {
		return ColumnProperties{
			Name: AnnotatedField[string]{Value: name, Position: lsp.Position{Line: nameLine, Character: 14}},
			Description: AnnotatedField[string]{
				Value: description, Position: lsp.Position{Line: descriptionLine, Character: 21},
			},
		}
	}

	expectedProperties := PropertiesYaml{
		Models: []ModelProperties{
			{
				Name: AnnotatedField[string]{
					Value: "customers", Position: lsp.Position{Line: 3, Character: 10},
				},
				Description: AnnotatedField[string]{
					Value:    "This table has basic information about a customer, as well as some derived facts based on a customer's orders",
					Position: lsp.Position{Line: 4, Character: 17},
				},
				ModelConfig: AnnotatedMap(nil),
				Columns: []ColumnProperties{
					column("customer_id", 7, "This is a unique identifier for a customer", 8),
					column("first_name", 13, "Customer's first name. PII.", 14),
					column("last_name", 16, "Customer's last name. PII.", 17),
					column("first_order", 19, "Date (UTC) of a customer's first order", 20),
					column("most_recent_order", 22, "Date (UTC) of a customer's most recent order", 23),
					column("number_of_orders", 25, "Count of the number of orders a customer has placed", 26),
					column("total_order_amount", 28, "Total value (AUD) of a customer's orders", 29),
				},
			},
			{
				Name: AnnotatedField[string]{
					Value:    "orders",
					Position: lsp.Position{Line: 31, Character: 10},
				},
				Description: AnnotatedField[string]{
					Value:    "This table has basic information about orders, as well as some derived facts based on payments",
					Position: lsp.Position{Line: 32, Character: 17},
				},
				ModelConfig: AnnotatedMap(nil),
				Columns: []ColumnProperties{
					column("order_id", 35, "This is a unique identifier for an order", 39),
					column("customer_id", 41, "Foreign key to the customers table", 42),
					column("order_date", 49, "Date (UTC) that the order was placed", 50),
					column("status", 52, `{{ doc("orders_status") }}`, 53),
					column("amount", 58, "Total amount (AUD) of the order", 59),
					column("credit_card_amount", 63, "Amount of the order (AUD) paid for by credit card", 64),
					column("coupon_amount", 68, "Amount of the order (AUD) paid for by coupon", 69),
					column("bank_transfer_amount", 73, "Amount of the order (AUD) paid for by bank transfer", 74),
					column("gift_card_amount", 78, "Amount of the order (AUD) paid for by gift card", 79),
				},
			},
		},
		Sources: []SourceProperties{
			{
				Name: AnnotatedField[string]{
					Value:    "jaffle_shop",
					Position: lsp.Position{Line: 84, Character: 10},
				},
				Database: AnnotatedField[string]{
					Value:    "raw",
					Position: lsp.Position{Line: 85, Character: 14},
				},
				Schema: AnnotatedField[string]{
					Value:    "jaffle_shop",
					Position: lsp.Position{Line: 86, Character: 12},
				},
				Description: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Tables: []SourceTableProperties{
					{
						Name: AnnotatedField[string]{
							Value:    "orders",
							Position: lsp.Position{Line: 88, Character: 14},
						},

						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
						},
					},
					{
						Name: AnnotatedField[string]{
							Value:    "customers",
							Position: lsp.Position{Line: 89, Character: 14},
						},
						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
						},
					},
				},
			},
			{
				Name: AnnotatedField[string]{
					Value:    "stripe",
					Position: lsp.Position{Line: 91, Character: 10},
				},
				Database: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Schema: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Description: AnnotatedField[string]{
					Value:    "",
					Position: lsp.Position{Line: 0, Character: 0},
				},
				Tables: []SourceTableProperties{
					{
						Name: AnnotatedField[string]{
							Value:    "payments",
							Position: lsp.Position{Line: 93, Character: 14},
						},
						Description: AnnotatedField[string]{
							Value:    "",
							Position: lsp.Position{Line: 0, Character: 0},
						},
					},
				},
			},
		},
	}

	if fmt.Sprintf("%#v", actualProperties) != fmt.Sprintf("%#v", expectedProperties) {
		t.Errorf("expected %#v but got %#v", expectedProperties, actualProperties)
	}
}
