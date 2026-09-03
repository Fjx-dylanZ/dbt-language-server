package parser

import (
	"errors"
	"sort"
	"strings"

	"github.com/j-clemons/dbt-language-server/docs"
)

type Parser struct {
	l       *Lexer
	curTok  Token
	peekTok Token
	tokens  []TokenLL
	ctes    CTE
	inJinja bool      // inside `{{ … }}` or `{% … %}`
	quote   TokenType // the quote that opened the current Jinja string literal, else ""
	jinja   JinjaVars
}

type CTE struct {
	Ind          bool
	ParenCount   int
	Tokens       []Token
	TokenNameMap map[string]Token
}

type TokenLL struct {
	Token     Token
	PrevToken *TokenLL
	Jinja     bool // inside a Jinja block and outside its string literals
}

func NewParser(input string, dialect docs.Dialect) *Parser {
	return &Parser{
		l: New(input, dialect),
		ctes: CTE{
			Ind:        false,
			ParenCount: -1,
			Tokens:     []Token{},
		},
		jinja: JinjaVars{Global: map[string]Token{}},
	}
}

func Parse(input string, dialect docs.Dialect) *Parser {
	p := NewParser(input, dialect)
	p.parseTokens()
	return p
}

func (p *Parser) NextToken() Token {
	if p.curTok.Type != "" {
		prevToken := (*TokenLL)(nil)
		if len(p.tokens) > 0 {
			prevToken = &p.tokens[len(p.tokens)-1]
		}
		// Every token passes through here exactly once, so string parity holds
		// even for quotes consumed by parseRef/parseSource/parseVar.
		if p.inJinja && (p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE) {
			if p.quote == "" {
				p.quote = p.curTok.Type
			} else if p.quote == p.curTok.Type {
				p.quote = ""
			}
		}
		p.tokens = append(p.tokens, TokenLL{
			Token:     p.curTok,
			PrevToken: prevToken,
			Jinja:     p.inJinja && p.quote == "",
		})
	}

	p.curTok = p.peekTok
	p.peekTok = p.l.NextToken()
	return p.curTok
}

func (p *Parser) parseWith() {
	p.NextToken()
	if p.curTok.Type == RECURSIVE {
		p.NextToken()
	}
	p.demoteBareDbtKeyword()
	if p.curTok.Type == IDENT {
		p.parseCteHead()
	}
}

// demoteBareDbtKeyword retypes ref/source/var/config to IDENT when they are
// not a call: `with source as (…)` and `from source` name a CTE, not dbt's source().
func (p *Parser) demoteBareDbtKeyword() {
	switch p.curTok.Type {
	case REF, VAR, SOURCE, CONFIG:
		if p.peekTok.Type != LPAREN {
			p.curTok.Type = IDENT
		}
	}
}

// parseCteHead is called with curTok on a candidate CTE name. It accepts
// `name [(col, …)] [as] (` and, on success, records the name, arms paren
// tracking and leaves curTok on the body's opening paren. Anything else
// (e.g. BigQuery's `unnest(x) with offset as pos`) is not a CTE.
func (p *Parser) parseCteHead() {
	name := p.curTok
	if p.peekTok.Type == LPAREN {
		p.NextToken()
		p.skipParenGroup()
	}
	if p.peekTok.Type == AS {
		p.NextToken()
	}
	if p.peekTok.Type != LPAREN {
		p.ctes.Ind = false
		return
	}
	p.ctes.Ind = true
	p.ctes.ParenCount = 1
	p.ctes.Tokens = append(p.ctes.Tokens, name)
	p.NextToken()
}

// skipParenGroup is called with curTok on LPAREN and advances to the matching RPAREN.
func (p *Parser) skipParenGroup() {
	depth := 1
	for depth > 0 && p.peekTok.Type != EOF {
		p.NextToken()
		switch p.curTok.Type {
		case LPAREN:
			depth++
		case RPAREN:
			depth--
		}
	}
}

func (p *Parser) parseRef() {
	p.NextToken()
	if p.curTok.Type == LPAREN {
		p.incParenCount()
		p.NextToken()
		if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
			p.NextToken()
			if p.curTok.Type == IDENT {
				p.curTok.Type = REF
			}
		}
	}
}

func (p *Parser) parseVar() {
	p.NextToken()
	if p.curTok.Type == LPAREN {
		p.incParenCount()
		p.NextToken()
		if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
			p.NextToken()
			if p.curTok.Type == IDENT {
				p.curTok.Type = VAR
			}
		}
	}
}

// parseMacro is called with curTok on an IDENT inside a Jinja block and marks
// `name(` as MACRO and `pkg.name(` as PACKAGE DOT MACRO. Built-ins such as
// is_incremental() are marked too; they simply resolve to nothing.
func (p *Parser) parseMacro() {
	if p.peekTok.Type == DOT {
		p.curTok.Type = PACKAGE
		p.NextToken()
		p.NextToken()
	}
	if p.curTok.Type == IDENT && p.peekTok.Type == LPAREN {
		p.curTok.Type = MACRO
	}
}

func (p *Parser) parseSource() {
	p.NextToken()
	if p.curTok.Type == LPAREN {
		p.incParenCount()
		p.NextToken()
		if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
			p.NextToken()
			if p.curTok.Type == IDENT {
				p.curTok.Type = SOURCE
				p.NextToken()
				if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
					p.NextToken()
					if p.curTok.Type == COMMA {
						p.NextToken()
						if p.curTok.Type == SINGLE_QUOTE || p.curTok.Type == DOUBLE_QUOTE {
							p.NextToken()
							if p.curTok.Type == IDENT {
								p.curTok.Type = SOURCE_TABLE
							}
						}
					}
				}
			}
		}
	}
}

func (p *Parser) incParenCount() {
	if p.ctes.Ind {
		p.ctes.ParenCount++
	}
}

func (p *Parser) decParenCount() {
	if p.ctes.Ind {
		p.ctes.ParenCount--
	}
}

func (p *Parser) parseTokens() {
	for p.curTok.Type != EOF {
		p.demoteBareDbtKeyword()
		if p.inJinja && p.quote == "" && p.parseJinjaStatement() {
			p.NextToken()
			continue
		}
		switch p.curTok.Type {
		case WITH:
			p.parseWith()
		case LPAREN:
			p.incParenCount()
		case RPAREN:
			p.decParenCount()
			if p.ctes.Ind && p.ctes.ParenCount == 0 {
				p.NextToken()
				if p.curTok.Type == COMMA {
					p.NextToken()
					p.demoteBareDbtKeyword()
					if p.curTok.Type == IDENT {
						p.parseCteHead()
					}
				} else {
					p.ctes.Ind = false
				}
			}
		case SOURCE:
			p.parseSource()
		case REF:
			p.parseRef()
		case VAR:
			p.parseVar()
		case IDENT:
			if p.inJinja && p.quote == "" {
				p.parseMacro()
			}
		case DB_LBRACE, JINJA_LBRACE:
			p.inJinja = true
			p.quote = ""
		case DB_RBRACE, JINJA_RBRACE:
			p.inJinja = false
		}
		p.NextToken()
	}
}

// CreateTokenNameMap maps each CTE name (lower-cased: SQL identifiers are
// case-insensitive) to the token that defines it.
func (p *Parser) CreateTokenNameMap() map[string]Token {
	tokenMap := make(map[string]Token, len(p.ctes.Tokens))
	for _, token := range p.ctes.Tokens {
		tokenMap[strings.ToLower(token.Literal)] = token
	}
	return tokenMap
}

type TokenIndex struct {
	tokens     []TokenLL
	lineTokens map[int][]TokenLL
}

func (p *Parser) CreateTokenIndex() *TokenIndex {
	index := &TokenIndex{
		tokens:     p.tokens,
		lineTokens: make(map[int][]TokenLL),
	}

	for _, t := range p.tokens {
		index.lineTokens[t.Token.Line] = append(index.lineTokens[t.Token.Line], t)
	}

	return index
}

// IdentifierReferences returns, in document order, every IDENT token that
// names `name` (case-insensitive) in a relation position: qualified columns
// (`x.name`), quoted strings (`'name'`) and aliases (`… as name`) are skipped.
func (ti *TokenIndex) IdentifierReferences(name string) []Token {
	var refs []Token
	for i := range ti.tokens {
		t := &ti.tokens[i]
		if t.Token.Type != IDENT || !strings.EqualFold(t.Token.Literal, name) {
			continue
		}
		if prev := t.PrevToken; prev != nil {
			switch prev.Token.Type {
			case DOT, SINGLE_QUOTE, DOUBLE_QUOTE, BACKTICK, AS:
				continue
			}
		}
		refs = append(refs, t.Token)
	}
	return refs
}

func (ti *TokenIndex) FindTokenAtCursor(line, column int) (*TokenLL, error) {
	lineTokens, exists := ti.lineTokens[line]
	if !exists {
		return nil, errors.New("line does not exist")
	}

	// Binary search to find the token
	idx := sort.Search(len(lineTokens), func(i int) bool {
		return lineTokens[i].Token.Column+len(lineTokens[i].Token.Literal) > column
	})

	if idx >= 0 &&
		column >= lineTokens[idx].Token.Column &&
		column < lineTokens[idx].Token.Column+len(lineTokens[idx].Token.Literal) {
		return &lineTokens[idx], nil
	}

	return nil, errors.New("token does not exist")
}

func (tl *TokenLL) TokenLookbackMatch(tokenType TokenType, inc int) (bool, string) {
	if inc <= 0 {
		return false, ""
	}

	prevToken := tl.PrevToken
	for i := 1; i < inc; i++ {
		if prevToken == nil {
			return false, ""
		}
		prevToken = prevToken.PrevToken
	}

	if prevToken != nil && prevToken.Token.Type == tokenType {
		return true, prevToken.Token.Literal
	}
	return false, ""
}
