package docs

import (
	"strings"
	"testing"
)

func TestMarkdownBigQueryDoc(t *testing.T) {
	got, ok := Dialect("bigquery").FunctionDoc("coalesce")
	if !ok {
		t.Fatal("coalesce missing")
	}
	want := strings.Join([]string{
		"```sql",
		"COALESCE(expr[, ...])",
		"```",
		"",
		"**Description**",
		"Returns the value of the first non-NULL expression, if any, otherwise NULL.",
		"The remaining expressions aren't evaluated. An input expression can be any",
		"type. There may be multiple input expression types. All input expressions must",
		"be implicitly coercible to a common supertype.",
		"",
		"**Return Data Type**",
		`Supertype of expr\[, ...\].`,
		"",
		"[BigQuery Documentation](https://cloud.google.com/bigquery/docs/reference/standard-sql/conditional_expressions#coalesce)",
	}, "\n")
	if got != want {
		t.Fatalf("coalesce markdown:\n%s\nwant:\n%s", got, want)
	}

	// Multi-part sql blocks keep their internal blank lines (the signature parser cuts there).
	got, _ = Dialect("bigquery").FunctionDoc("array_agg")
	if !strings.Contains(got, "[ OVER over_clause ]\n\nover_clause:") || strings.Contains(got, "\n\n```\n") {
		t.Fatalf("array_agg block:\n%s", got[:min(len(got), 500)])
	}
	if !strings.Contains(got, "**Supported Argument Types**\nAll data types except ARRAY.\n\n**Returned Data Types**\nARRAY") {
		t.Fatalf("array_agg sections:\n%s", got)
	}

	// Signature parsing runs on the formatted text.
	sig, ok := ParseSignature(got)
	if !ok || !strings.HasSuffix(sig.Label, "[ OVER over_clause ]") || !strings.HasPrefix(sig.Documentation, "**Description**\nReturns an ARRAY") {
		t.Fatalf("array_agg signature from markdown: %+v", sig)
	}
}

func TestMarkdownSnowflakeFences(t *testing.T) {
	// Snowflake docs close argument fences at the end of the code line.
	got := Markdown("```sql\nDATE_TRUNC( <part>, <expr> )\n```\n```sql\n<part>```\nThis argument must be one of the values.\nNote\nSomething.\n[Snowflake Documentation](https://x)")
	want := strings.Join([]string{
		"```sql",
		"DATE_TRUNC( <part>, <expr> )",
		"```",
		"",
		"```sql",
		"<part>",
		"```",
		"",
		"This argument must be one of the values.",
		"",
		"**Note**",
		"Something.",
		"",
		"[Snowflake Documentation](https://x)",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMarkdownIsIdempotent(t *testing.T) {
	for _, name := range []string{"coalesce", "array_agg", "date_trunc", "count"} {
		once, _ := Dialect("bigquery").FunctionDoc(name)
		if twice := Markdown(once); twice != once {
			t.Fatalf("%s: formatting twice changed the text:\n%s\n---\n%s", name, once, twice)
		}
	}
}

func TestMarkdownWholeTables(t *testing.T) {
	for _, dialect := range []Dialect{"bigquery", "snowflake"} {
		for name := range dialect.FunctionDocs() {
			doc, _ := dialect.FunctionDoc(name)
			fences := 0
			for _, line := range strings.Split(doc, "\n") {
				if strings.HasPrefix(line, "```") {
					fences++
				}
			}
			if fences%2 != 0 {
				t.Fatalf("%s %s: unbalanced fences:\n%s", dialect, name, doc)
			}
			if strings.Contains(doc, "\n\n\n") || strings.HasSuffix(doc, "\n") {
				t.Fatalf("%s %s: stray blank lines:\n%s", dialect, name, doc)
			}
			if strings.Contains(doc, "\n\n```\n") {
				t.Fatalf("%s %s: blank line before a closing fence:\n%s", dialect, name, doc)
			}
		}
	}
}
