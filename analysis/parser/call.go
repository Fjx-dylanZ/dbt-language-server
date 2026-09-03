package parser

import "sort"

// Call is the innermost call expression whose argument list contains a cursor.
type Call struct {
	Callee   TokenLL // the name token before the opening `(`
	Argument int     // zero-based index of the argument under the cursor
}

// CallAtCursor walks back from the token before the cursor, matching brackets,
// until it meets an unmatched `(` preceded by a token isCallee accepts. Other
// unmatched groups — subqueries, `over (…)`, `in (…)`, list literals — are
// transparent, so the enclosing call is still found. Rendered Jinja blocks
// passed on the way are opaque; reaching a `{{`/`{%` opener directly means the
// cursor is inside Jinja with no call around it. String literals are skipped.
func (ti *TokenIndex) CallAtCursor(line, column int, isCallee func(TokenLL) bool) (Call, bool) {
	tokens := ti.tokens
	start := sort.Search(len(tokens), func(i int) bool {
		t := tokens[i].Token
		return t.Line > line || (t.Line == line && t.Column >= column)
	}) - 1
	if start < 0 {
		return Call{}, false
	}

	skip := stringMask(tokens[:start+1])

	depth, commas := 0, 0
	for i := start; i >= 0; i-- {
		if skip[i] {
			continue
		}
		t := tokens[i]
		switch t.Token.Type {
		case RPAREN, RBRACE:
			depth++
		case LBRACE:
			if depth > 0 {
				depth--
			} else {
				commas = 0
			}
		case LPAREN:
			if depth > 0 {
				depth--
			} else if i > 0 && isCallee(tokens[i-1]) {
				return Call{Callee: tokens[i-1], Argument: commas}, true
			} else {
				commas = 0
			}
		case COMMA:
			if depth == 0 {
				commas++
			}
		case DB_RBRACE, JINJA_RBRACE:
			opener := TokenType(DB_LBRACE)
			if t.Token.Type == JINJA_RBRACE {
				opener = JINJA_LBRACE
			}
			for i > 0 && tokens[i].Token.Type != opener {
				i--
			}
		case DB_LBRACE, JINJA_LBRACE:
			return Call{}, false
		case ILLEGAL:
			// `[` / `]` are not lexed as tokens of their own.
			switch t.Token.Literal {
			case "]":
				depth++
			case "[":
				if depth > 0 {
					depth--
				} else {
					commas = 0
				}
			}
		}
	}
	return Call{}, false
}

// stringMask marks the tokens that are string delimiters or string contents.
func stringMask(tokens []TokenLL) []bool {
	mask := make([]bool, len(tokens))
	var quote TokenType
	for i, t := range tokens {
		switch {
		case quote != "":
			mask[i] = true
			if t.Token.Type == quote {
				quote = ""
			}
		case t.Token.Type == SINGLE_QUOTE || t.Token.Type == DOUBLE_QUOTE || t.Token.Type == BACKTICK:
			mask[i] = true
			quote = t.Token.Type
		}
	}
	return mask
}
