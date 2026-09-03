package docs

import (
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/lsp/completionKind"
)

type Dialect string

func (d Dialect) FunctionDocs() map[string]string {
	switch d {
	case "snowflake":
		return SnowflakeFunctions
	case "bigquery":
		return BigQueryFunctions
	default:
		return map[string]string{}
	}
}

// FunctionDoc returns the markdown documentation of a dialect function.
func (d Dialect) FunctionDoc(name string) (string, bool) {
	doc, ok := d.FunctionDocs()[name]
	if !ok {
		return "", false
	}
	return Markdown(doc), true
}

func (d Dialect) FunctionCompletionItems() []lsp.CompletionItem {
	items := []lsp.CompletionItem{}

	functions := d.FunctionDocs()

	for k, v := range functions {
		items = append(items, lsp.CompletionItem{
			Label:         k,
			Detail:        "",
			Documentation: Markdown(v),
			Kind:          completionKind.Function,
			InsertText:    k,
			SortText:      k,
		})
	}
	return items
}
