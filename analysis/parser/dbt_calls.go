package parser

// DbtCall is a ref() or source() call inside Jinja whose positional arguments
// are all plain string literals, e.g. ref('pkg', 'model') or
// source('src', 'table'). Keyword arguments (ref('m', v=2)) are ignored.
type DbtCall struct {
	Callee Token   // the ref/source keyword
	Args   []Token // one token per positional argument, holding the string's contents
}

// DbtCalls returns the calls to kind (REF or SOURCE) whose targets are known
// without rendering the template. Calls with any computed positional argument
// (ref(name), ref('stg_' ~ name)) are left out.
func (ti *TokenIndex) DbtCalls(kind TokenType) []DbtCall {
	var calls []DbtCall
	tokens := ti.tokens
	for i := 0; i+1 < len(tokens); i++ {
		callee := tokens[i]
		if callee.Token.Type != kind || !callee.Jinja || tokens[i+1].Token.Type != LPAREN {
			continue
		}
		if args, ok := literalArgs(tokens[i+2:]); ok {
			calls = append(calls, DbtCall{Callee: callee.Token, Args: args})
		}
	}
	return calls
}

// literalArgs reads the positional arguments that follow a call's `(`. Each
// must be a quoted single word, e.g. 'orders'; the list ends at `)` or at the
// first keyword argument.
func literalArgs(tokens []TokenLL) ([]Token, bool) {
	var args []Token
	for i := 0; ; i += 4 {
		if i < len(tokens) && tokens[i].Token.Type == RPAREN && len(args) > 0 {
			return args, true // trailing comma
		}
		if i+1 < len(tokens) && tokens[i].Token.Type == IDENT && tokens[i+1].Token.Type == EQUAL && len(args) > 0 {
			return args, true
		}
		if i+3 >= len(tokens) {
			return nil, false
		}
		open, word, close, next := tokens[i].Token, tokens[i+1].Token, tokens[i+2].Token, tokens[i+3].Token
		if (open.Type != SINGLE_QUOTE && open.Type != DOUBLE_QUOTE) || close.Type != open.Type {
			return nil, false
		}
		// The word must fill the whole string: the lexer drops whitespace, so
		// ' orders' would otherwise read as 'orders'.
		if !isWord(word.Literal) || word.Line != open.Line || close.Line != open.Line ||
			word.Column != open.Column+1 || close.Column != word.Column+len(word.Literal) {
			return nil, false
		}
		args = append(args, word)

		switch next.Type {
		case RPAREN:
			return args, true
		case COMMA:
		default:
			return nil, false
		}
	}
}

func isWord(s string) bool {
	if s == "" || !isLetter(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isLetter(s[i]) && !isDigit(s[i]) {
			return false
		}
	}
	return true
}
