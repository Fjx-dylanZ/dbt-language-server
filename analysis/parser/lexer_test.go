package parser

import (
	"testing"

	"github.com/j-clemons/dbt-language-server/docs"
)

func TestNextToken(t *testing.T) {
	input := `select *
from {{ ref('users') }}`

	tests := []Token{
		{Type: SELECT, Literal: "select", Line: 0, Column: 0},
		{Type: ASTERISK, Literal: "*", Line: 0, Column: 7},
		{Type: FROM, Literal: "from", Line: 1, Column: 0},
		{Type: DB_LBRACE, Literal: "{{", Line: 1, Column: 5},
		{Type: REF, Literal: "ref", Line: 1, Column: 8},
		{Type: LPAREN, Literal: "(", Line: 1, Column: 11},
		{Type: SINGLE_QUOTE, Literal: "'", Line: 1, Column: 12},
		{Type: IDENT, Literal: "users", Line: 1, Column: 13},
		{Type: SINGLE_QUOTE, Literal: "'", Line: 1, Column: 18},
		{Type: RPAREN, Literal: ")", Line: 1, Column: 19},
		{Type: DB_RBRACE, Literal: "}}", Line: 1, Column: 21},
	}

	l := New(input, docs.Dialect("snowflake"))

	for i, tt := range tests {
		tok := l.NextToken()

		if tok != tt {
			t.Fatalf("tests[%d] - expected=%v, got=%v",
				i, tt, tok)
		}
	}
}

func TestSkipComments(t *testing.T) {
	input := "select a, -- ) trailing\r\n" +
		"/* multi\n" +
		"line ( */ b {# jinja\n" +
		"comment ) #} from t"

	tests := []Token{
		{Type: SELECT, Literal: "select", Line: 0, Column: 0},
		{Type: IDENT, Literal: "a", Line: 0, Column: 7},
		{Type: COMMA, Literal: ",", Line: 0, Column: 8},
		{Type: IDENT, Literal: "b", Line: 2, Column: 10},
		{Type: FROM, Literal: "from", Line: 3, Column: 13},
		{Type: IDENT, Literal: "t", Line: 3, Column: 18},
		{Type: EOF, Literal: "", Line: 0, Column: 0},
	}

	l := New(input, docs.Dialect("snowflake"))

	for i, tt := range tests {
		tok := l.NextToken()

		if tok != tt {
			t.Fatalf("tests[%d] - expected=%v, got=%v",
				i, tt, tok)
		}
	}
}

func TestKeywordsCaseInsensitive(t *testing.T) {
	l := New("WITH Ref Select", docs.Dialect("snowflake"))

	tests := []TokenType{WITH, IDENT, SELECT}
	for i, want := range tests {
		if tok := l.NextToken(); tok.Type != want {
			t.Fatalf("tests[%d] - expected=%v, got=%v", i, want, tok.Type)
		}
	}
}
