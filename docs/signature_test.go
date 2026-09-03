package docs

import (
	"reflect"
	"strings"
	"testing"
)

func paramTexts(sig Signature) []string {
	out := make([]string, 0, len(sig.Parameters))
	for _, p := range sig.Parameters {
		out = append(out, sig.Label[p[0]:p[1]])
	}
	return out
}

func TestParseSignatureBigQuery(t *testing.T) {
	tests := []struct {
		fn     string
		label  string
		params []string
		doc    string
	}{
		{"date_trunc", "DATE_TRUNC(date_value, date_granularity)", []string{"date_value", "date_granularity"}, "**Description**\nTruncates"},
		{"count", "COUNT(*)\n[ OVER over_clause ]", []string{"*"}, "**Description**\nGets the number of rows"},
		{"coalesce", "COALESCE(expr[, ...])", []string{"expr[, ...]"}, "**Description**\nReturns the value of the first non-NULL"},
		{"safe_cast", "SAFE_CAST(expression AS typename [format_clause])", []string{"expression AS typename [format_clause]"}, "**Description**\nWhen using CAST"},
		{"appends", "APPENDS(\n  TABLE table,\n  start_timestamp DEFAULT NULL,\n  end_timestamp DEFAULT NULL)", []string{"TABLE table", "start_timestamp DEFAULT NULL", "end_timestamp DEFAULT NULL"}, "**Description**\nThe APPENDS function"},
	}
	for _, tt := range tests {
		sig, ok := ParseSignature(Markdown(BigQueryFunctions[tt.fn]))
		if !ok {
			t.Fatalf("%s: no signature", tt.fn)
		}
		if sig.Label != tt.label {
			t.Fatalf("%s: label=%q want %q", tt.fn, sig.Label, tt.label)
		}
		if got := paramTexts(sig); !reflect.DeepEqual(got, tt.params) {
			t.Fatalf("%s: params=%q want %q", tt.fn, got, tt.params)
		}
		if !strings.HasPrefix(sig.Documentation, tt.doc) {
			t.Fatalf("%s: documentation starts %q, want %q", tt.fn, sig.Documentation[:min(len(sig.Documentation), 60)], tt.doc)
		}
	}

	// Clause definitions after the blank line belong to the documentation.
	sig, _ := ParseSignature(BigQueryFunctions["array_agg"])
	if !strings.HasSuffix(sig.Label, "[ OVER over_clause ]") || strings.Contains(sig.Label, "over_clause:") {
		t.Fatalf("array_agg label not cut at the blank line: %q", sig.Label)
	}
	if len(sig.Parameters) != 1 {
		t.Fatalf("array_agg params=%q", paramTexts(sig))
	}

	if _, ok := ParseSignature("no code block here"); ok {
		t.Fatal("expected no signature without a sql block")
	}
}

func TestSignatureFromLabel(t *testing.T) {
	sig := SignatureFromLabel("bq_consumer(user_email_expr='user_email', sep=', ', labels_expr=fn(a, b), job_labels_expr)")
	want := []string{"user_email_expr='user_email'", "sep=', '", "labels_expr=fn(a, b)", "job_labels_expr"}
	if got := paramTexts(sig); !reflect.DeepEqual(got, want) {
		t.Fatalf("params=%q want %q", got, want)
	}
	if got := SignatureFromLabel("no_args()"); len(got.Parameters) != 0 {
		t.Fatalf("no_args params=%q", paramTexts(got))
	}
	if got := SignatureFromLabel("CASE expr WHEN x THEN y END"); len(got.Parameters) != 0 {
		t.Fatalf("CASE params=%q", paramTexts(got))
	}

	// Offsets are UTF-16 code units.
	sig = SignatureFromLabel("f(π, 𝒳)")
	if !reflect.DeepEqual(sig.Parameters, [][2]int{{2, 3}, {5, 7}}) {
		t.Fatalf("utf16 offsets=%v", sig.Parameters)
	}
}
