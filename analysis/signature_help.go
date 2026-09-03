package analysis

import (
	"strings"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/docs"
	"github.com/j-clemons/dbt-language-server/lsp"
)

// SignatureHelp answers textDocument/signatureHelp for the innermost call
// around the cursor: a dialect function (from the docs table) or a project
// macro (its `{% macro name(params) %}` head), with the argument under the
// cursor marked active.
func (s *State) SignatureHelp(id int, uri string, position lsp.Position) lsp.SignatureHelpResponse {
	response := lsp.SignatureHelpResponse{
		Response: lsp.Response{
			RPC: "2.0",
			ID:  &id,
		},
	}

	doc, ok := s.Documents[uri]
	if !ok {
		return response
	}

	var sig docs.Signature
	call, ok := doc.Tokens.CallAtCursor(position.Line, position.Character, func(t parser.TokenLL) bool {
		sig, ok = s.calleeSignature(t)
		return ok
	})
	if !ok {
		return response
	}

	info := lsp.SignatureInformation{
		Label:      sig.Label,
		Parameters: make([]lsp.ParameterInformation, 0, len(sig.Parameters)),
	}
	for _, p := range sig.Parameters {
		info.Parameters = append(info.Parameters, lsp.ParameterInformation{Label: p})
	}
	if sig.Documentation != "" {
		info.Documentation = &lsp.MarkupContent{Kind: "markdown", Value: sig.Documentation}
	}

	// Past the last documented parameter (variadic `expr[, ...]`) keep it active.
	active := call.Argument
	if n := len(sig.Parameters); n > 0 && active >= n {
		active = n - 1
	}

	response.Result = &lsp.SignatureHelp{
		Signatures:      []lsp.SignatureInformation{info},
		ActiveParameter: active,
	}
	return response
}

// calleeSignature resolves the token before a `(` to a macro or dialect function.
func (s *State) calleeSignature(t parser.TokenLL) (docs.Signature, bool) {
	if t.Token.Type == parser.MACRO {
		packageName := Package(s.DbtContext.ProjectYaml.ProjectName.Value)
		if match, literal := t.TokenLookbackMatch(parser.PACKAGE, 2); match {
			packageName = Package(literal)
		}
		macro, ok := s.DbtContext.MacroDetailMap[packageName][t.Token.Literal]
		if !ok {
			return docs.Signature{}, false
		}
		return docs.SignatureFromLabel(macro.Description), true
	}

	doc, ok := s.functionDoc(t)
	if !ok {
		return docs.Signature{}, false
	}
	return docs.ParseSignature(doc)
}

// functionDoc looks the token up in the dialect's function table, case-
// insensitively. A dotted prefix qualifies the name (`net.host`, `aead.encrypt`);
// BigQuery's `safe.` prefix is transparent (`safe.substr` → `substr`); any other
// unknown prefix is a table alias, so `t.date` is a column, not DATE().
func (s *State) functionDoc(t parser.TokenLL) (string, bool) {
	dialect := s.DbtContext.Dialect
	name := strings.ToLower(t.Token.Literal)
	if match, _ := t.TokenLookbackMatch(parser.DOT, 1); match {
		prefix := ""
		if p := t.PrevToken.PrevToken; p != nil {
			prefix = strings.ToLower(p.Token.Literal)
		}
		if doc, ok := dialect.FunctionDoc(prefix + "." + name); ok {
			return doc, true
		}
		if prefix != "safe" {
			return "", false
		}
	}
	return dialect.FunctionDoc(name)
}
