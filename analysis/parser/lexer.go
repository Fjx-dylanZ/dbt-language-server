package parser

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/j-clemons/dbt-language-server/docs"
)

type Lexer struct {
	reader  *bufio.Reader
	ch      byte // current char under examination
	line    int
	column  int
	dialect docs.Dialect
	jinja   bool // inside `{{ … }}` or `{% … %}`, where SQL keywords are plain identifiers
}

func New(input string, dialect docs.Dialect) *Lexer {
	l := &Lexer{
		reader:  bufio.NewReader(strings.NewReader(input)),
		line:    0,
		column:  -1,
		dialect: dialect,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() error {
	ch, err := l.reader.ReadByte()
	if err != nil {
		if err == io.EOF {
			l.ch = 0
			return nil
		}
		return err
	}
	l.ch = ch
	l.column++
	return nil
}

func (l *Lexer) peekChar() byte {
	ch, err := l.reader.Peek(1)
	if err != nil {
		return 0
	}
	return ch[0]
}

func newToken(tokenType TokenType, l Lexer) Token {
	return Token{Type: tokenType, Literal: string(l.ch), Line: l.line, Column: l.column}
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	switch l.ch {
	case '=':
		tok = newToken(EQUAL, *l)
	case ';':
		tok = newToken(SEMICOLON, *l)
	case '(':
		tok = newToken(LPAREN, *l)
	case ')':
		tok = newToken(RPAREN, *l)
	case ',':
		tok = newToken(COMMA, *l)
	case '.':
		tok = newToken(DOT, *l)
	case '+':
		tok = newToken(PLUS, *l)
	case '-':
		tok = newToken(MINUS, *l)
	case '!':
		tok = l.twoCharToken('=', NOT_EQ, BANG)
	case '/':
		tok = newToken(SLASH, *l)
	case '*':
		tok = newToken(ASTERISK, *l)
	case '<':
		tok = l.twoCharToken('=', LT_EQ, LT)
	case '>':
		tok = l.twoCharToken('=', GT_EQ, GT)
	case '{':
		tok = l.handleLeftBrace()
		if tok.Type == DB_LBRACE || tok.Type == JINJA_LBRACE {
			l.jinja = true
		}
	case '}':
		tok = l.twoCharToken('}', DB_RBRACE, RBRACE)
		if tok.Type == DB_RBRACE {
			l.jinja = false
		}
	case '\'':
		tok = newToken(SINGLE_QUOTE, *l)
	case '"':
		tok = newToken(DOUBLE_QUOTE, *l)
	case '`':
		tok = newToken(BACKTICK, *l)
	case '%':
		tok = l.twoCharToken('}', JINJA_RBRACE, PERCENT)
		if tok.Type == JINJA_RBRACE {
			l.jinja = false
		}
	case 0:
		tok.Literal = ""
		tok.Type = EOF
	default:
		if isLetter(l.ch) {
			tok.Line = l.line
			tok.Column = l.column // record column at start of token
			tok.Literal = l.readIdentifier()
			if l.jinja {
				tok.Type = LookupJinjaIdent(tok.Literal)
			} else {
				tok.Type = LookupIdent(tok.Literal, l.dialect)
			}
			return tok
		} else if isDigit(l.ch) {
			tok.Line = l.line
			tok.Column = l.column // record column at start of token
			tok.Type = INT
			tok.Literal = l.readNumber()
			return tok
		} else {
			tok = newToken(ILLEGAL, *l)
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) readIdentifier() string {
	var buf bytes.Buffer

	for isLetter(l.ch) || isDigit(l.ch) {
		buf.WriteByte(l.ch)
		l.readChar()
	}

	return buf.String()
}

func (l *Lexer) readNumber() string {
	var buf bytes.Buffer

	for isDigit(l.ch) {
		buf.WriteByte(l.ch)
		l.readChar()
	}

	return buf.String()
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// skipWhitespace skips whitespace and comments (`-- …`, `/* … */`, `{# … #}`).
// Comments are dropped so that parentheses inside them cannot corrupt CTE
// paren tracking in the parser.
func (l *Lexer) skipWhitespace() {
	for {
		switch {
		case l.ch == ' ' || l.ch == '\t':
			l.readChar()
		case l.ch == '\n' || l.ch == '\r':
			l.newline()
		case l.ch == '-' && l.peekChar() == '-':
			for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
				l.readChar()
			}
		case l.ch == '/' && l.peekChar() == '*':
			l.skipBlockComment('*', '/')
		case l.ch == '{' && l.peekChar() == '#':
			l.skipBlockComment('#', '}')
		default:
			return
		}
	}
}

// newline consumes a line terminator (`\n`, `\r`, or `\r\n`) and advances the line counter once.
func (l *Lexer) newline() {
	if l.ch == '\r' && l.peekChar() == '\n' {
		l.readChar()
	}
	l.line++
	l.column = -1
	l.readChar()
}

// skipBlockComment consumes from the current two-byte opener through the
// two-byte closer `end0 end1`, or to EOF if unterminated.
func (l *Lexer) skipBlockComment(end0, end1 byte) {
	l.readChar()
	l.readChar()
	for l.ch != 0 {
		switch {
		case l.ch == end0 && l.peekChar() == end1:
			l.readChar()
			l.readChar()
			return
		case l.ch == '\n' || l.ch == '\r':
			l.newline()
		default:
			l.readChar()
		}
	}
}

func (l *Lexer) twoCharToken(nextCh byte, trueToken, defaultToken TokenType) Token {
	if l.peekChar() == nextCh {
		ch := l.ch
		l.readChar()
		return Token{
			Type:    trueToken,
			Literal: string(ch) + string(l.ch),
			Line:    l.line,
			Column:  l.column - 1,
		}
	}
	return newToken(defaultToken, *l)
}

func (l *Lexer) handleLeftBrace() Token {
	switch l.peekChar() {
	case '%':
		return l.twoCharToken('%', JINJA_LBRACE, LBRACE)
	case '{':
		return l.twoCharToken('{', DB_LBRACE, LBRACE)
	default:
		return newToken(LBRACE, *l)
	}
}
