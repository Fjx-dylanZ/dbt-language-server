package analysis

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/lsp"
)

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDiagnosticsReportUnresolvedRefsAndSources(t *testing.T) {
	root := writeProject(t, map[string]string{
		"dbt_project.yml":             "name: shop\nprofile: shop\n",
		"models/orders.sql":           "select 1",
		"models/dim_customers_v1.sql": "select 1",
		// .yaml files count, a versioned model is ref'd by its YAML name, and
		// tables of one source may be spread over several files.
		"models/schema.yaml": "models:\n  - name: dim_customers\n    versions:\n      - v: 1\n" +
			"sources:\n  - name: raw\n    tables:\n      - name: orders\n",
		"models/more_sources.yml":                  "sources:\n  - name: raw\n    tables:\n      - name: payments\n",
		"seeds/countries.csv":                      "id\n1\n",
		"snapshots/orders_snapshot.sql":            "{% snapshot orders_snapshot %}\nselect 1\n{% endsnapshot %}\n",
		"snapshots/snapshots.yml":                  "snapshots:\n  - name: customers_snapshot\n",
		"dbt_packages/utils/dbt_project.yml":       "name: utils\n",
		"dbt_packages/utils/models/util_model.sql": "select 1",
	})

	state := NewState()
	state.refreshDbtContext(root)

	lines := []string{
		"select * from {{ ref('orders') }}",
		"join {{ ref('countries') }} join {{ ref('orders_snapshot') }} join {{ ref(\"customers_snapshot\") }}",
		"join {{ ref('dim_customers', v=1) }} join {{ ref('utils', 'util_model') }}",
		"join {{ ref('missing') }}",
		"join {{ ref('utils', 'missing_util') }}",
		"join {{ ref('other_project', 'thing') }} join {{ ref('stg_' ~ name) }} join {{ ref(' orders') }}",
		"join {{ source('raw', 'orders') }} join {{ source('raw', 'payments') }}",
		"join {{ source('raw', 'refunds') }}",
		"join {{ source('nope', 'x') }}",
		"{# {{ ref('in_jinja_comment') }} #}",
		"select ref('outside_jinja')",
	}
	uri := "file://" + filepath.Join(root, "models/report.sql")
	state.parseDocument(uri, strings.Join(lines, "\n"))

	at := func(line, start, end int) lsp.Range {
		return lsp.Range{Start: lsp.Position{Line: line, Character: start}, End: lsp.Position{Line: line, Character: end}}
	}
	diagnostic := func(r lsp.Range, message string) lsp.Diagnostic {
		return lsp.Diagnostic{Range: r, Message: message, Severity: 1, Source: "dbt-language-server"}
	}
	want := []lsp.Diagnostic{
		diagnostic(at(3, 13, 20), "No model, seed or snapshot named 'missing'"),
		diagnostic(at(4, 22, 34), "No model, seed or snapshot named 'missing_util' in package 'utils'"),
		diagnostic(at(7, 23, 30), "Source 'raw' has no table 'refunds'"),
		diagnostic(at(8, 16, 20), "No source named 'nope'"),
	}
	if got := state.Diagnostics(uri); !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics =\n%+v\nwant\n%+v", got, want)
	}

	fusion := lsp.Diagnostic{Range: at(0, 0, 0), Message: "compile error", Severity: 1, Source: "dbt Fusion"}
	state.SetFusionDiagnostics(uri, []lsp.Diagnostic{fusion})
	if got := state.Diagnostics(uri); !reflect.DeepEqual(got, append(want, fusion)) {
		t.Fatalf("diagnostics with Fusion results =\n%+v", got)
	}

	state.CloseDocument(uri)
	if got := state.Diagnostics(uri); len(got) != 0 {
		t.Fatalf("closed document still has diagnostics: %+v", got)
	}
}

func TestDiagnosticsOutsideDbtProject(t *testing.T) {
	state := NewState()
	uri := "file:///tmp/query.sql"
	state.parseDocument(uri, "select * from {{ ref('anything') }} join {{ source('a', 'b') }}")

	if got := state.Diagnostics(uri); got == nil || len(got) != 0 {
		t.Fatalf("expected an empty, non-nil list without a dbt project, got %#v", got)
	}
}
