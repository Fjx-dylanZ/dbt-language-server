package parser

import (
	"slices"
	"sort"
)

// JinjaScope is a `{% macro %} … {% endmacro %}` body: its parameters and the
// variables set inside it are visible only there.
type JinjaScope struct {
	Start Token // macro name
	End   Token // `endmacro`; zero when unterminated (open-ended)
	Defs  map[string]Token
}

func (s *JinjaScope) contains(t Token) bool {
	if before(t, s.Start) {
		return false
	}
	return s.End == (Token{}) || !before(s.End, t)
}

func before(a, b Token) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Column < b.Column)
}

// JinjaVars holds the Jinja variables a document defines: top-level `{% set %}`
// and `{% for %}` targets in Global, macro parameters and locals per macro.
// Jinja is case-sensitive, so names are kept verbatim.
type JinjaVars struct {
	Global map[string]Token
	Macros []JinjaScope
}

func (j *JinjaVars) scopeAt(t Token) *JinjaScope {
	for i := range j.Macros {
		if j.Macros[i].contains(t) {
			return &j.Macros[i]
		}
	}
	return nil
}

// Resolve returns the definition of the variable named by t: the enclosing
// macro's parameter or local when there is one, else the top-level definition.
func (j *JinjaVars) Resolve(t Token) (Token, bool) {
	if scope := j.scopeAt(t); scope != nil {
		if def, ok := scope.Defs[t.Literal]; ok {
			return def, true
		}
	}
	def, ok := j.Global[t.Literal]
	return def, ok
}

// References returns, in document order, every Jinja identifier that resolves
// to def, the definition itself included.
func (j *JinjaVars) References(index *TokenIndex, def Token) []Token {
	var refs []Token
	found := false
	for _, t := range index.jinjaIdentifiers(def.Literal) {
		if d, ok := j.Resolve(t); ok && d == def {
			refs = append(refs, t)
			found = found || t == def
		}
	}
	if !found {
		// A parameter with a default (`macro(x=1)`) or a tuple target (`set a, b = …`)
		// reads like a keyword argument and is filtered by jinjaIdentifiers.
		i := sort.Search(len(refs), func(i int) bool { return before(def, refs[i]) })
		refs = slices.Insert(refs, i, def)
	}
	return refs
}

// jinjaIdentifiers returns the Jinja-context IDENT tokens named `name` that can
// denote a variable: attributes (`x.name`) and keyword-argument names
// (`f(name=…)`) are skipped.
func (ti *TokenIndex) jinjaIdentifiers(name string) []Token {
	var out []Token
	for i := range ti.tokens {
		t := &ti.tokens[i]
		if !t.Jinja || t.Token.Type != IDENT || t.Token.Literal != name {
			continue
		}
		if prev := t.PrevToken; prev != nil {
			switch prev.Token.Type {
			case DOT:
				continue
			case LPAREN, COMMA:
				if i+2 < len(ti.tokens) && ti.tokens[i+1].Token.Type == EQUAL && ti.tokens[i+2].Token.Type != EQUAL {
					continue
				}
			}
		}
		out = append(out, t.Token)
	}
	return out
}

func (p *Parser) CreateJinjaVars() JinjaVars {
	return p.jinja
}

// prevToken is the token before curTok.
func (p *Parser) prevToken(n int) Token {
	if len(p.tokens) < n {
		return Token{}
	}
	return p.tokens[len(p.tokens)-n].Token
}

// atStatementStart reports whether curTok directly follows `{%`, `{%-` or `{%+`.
func (p *Parser) atStatementStart() bool {
	switch prev := p.prevToken(1); prev.Type {
	case JINJA_LBRACE:
		return true
	case MINUS, PLUS:
		return p.prevToken(2).Type == JINJA_LBRACE
	}
	return false
}

// parseJinjaStatement handles the statements that bind variables — `set`, `for`,
// `macro` — and `endmacro`. It reports whether curTok began one of them.
func (p *Parser) parseJinjaStatement() bool {
	if !p.atStatementStart() {
		return false
	}
	switch p.curTok.Literal {
	case "set":
		p.parseJinjaSet()
	case "for":
		p.parseJinjaFor()
	case "macro":
		p.parseJinjaMacro()
	case "endmacro":
		if n := len(p.jinja.Macros); n > 0 && p.jinja.Macros[n-1].End == (Token{}) {
			p.jinja.Macros[n-1].End = p.curTok
		}
	default:
		return false
	}
	return true
}

func (p *Parser) defineJinjaVar(t Token) {
	defs := p.jinja.Global
	if n := len(p.jinja.Macros); n > 0 && p.jinja.Macros[n-1].End == (Token{}) {
		defs = p.jinja.Macros[n-1].Defs
	}
	if _, exists := defs[t.Literal]; !exists {
		defs[t.Literal] = t
	}
}

// nextIdent advances when the next token is an identifier (demoting a bare
// ref/source/var/config name) and reports whether it did.
func (p *Parser) nextIdent() bool {
	switch p.peekTok.Type {
	case IDENT, REF, VAR, SOURCE, CONFIG:
		p.NextToken()
		p.demoteBareDbtKeyword()
		return p.curTok.Type == IDENT
	}
	return false
}

// `{% set a[, b] = … %}` and `{% set a %}…{% endset %}`. `{% set ns.x = … %}`
// assigns an attribute, which defines nothing.
func (p *Parser) parseJinjaSet() {
	for p.nextIdent() {
		if p.peekTok.Type == DOT {
			return
		}
		p.defineJinjaVar(p.curTok)
		if p.peekTok.Type != COMMA {
			return
		}
		p.NextToken()
	}
}

// `{% for a[, b] in … %}`
func (p *Parser) parseJinjaFor() {
	for p.peekTok.Literal != "in" && p.nextIdent() {
		p.defineJinjaVar(p.curTok)
		if p.peekTok.Type != COMMA {
			return
		}
		p.NextToken()
	}
}

// `{% macro name(a, b=default, …) %}` opens a scope holding the parameters.
func (p *Parser) parseJinjaMacro() {
	if !p.nextIdent() {
		return
	}
	p.parseMacro()
	p.jinja.Macros = append(p.jinja.Macros, JinjaScope{Start: p.curTok, Defs: map[string]Token{}})
	if p.peekTok.Type != LPAREN {
		return
	}
	p.NextToken()
	depth := 1
	for depth > 0 && p.peekTok.Type != EOF {
		p.NextToken()
		if p.quote != "" { // inside a string default such as sep='('
			continue
		}
		switch p.curTok.Type {
		case LPAREN:
			depth++
		case RPAREN:
			depth--
		case IDENT:
			if prev := p.prevToken(1).Type; depth == 1 && (prev == LPAREN || prev == COMMA) {
				p.defineJinjaVar(p.curTok)
			}
		}
	}
}
