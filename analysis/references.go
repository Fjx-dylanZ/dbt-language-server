package analysis

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/util"
)

func tokenRange(t parser.Token) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{Line: t.Line, Character: t.Column},
		End:   lsp.Position{Line: t.Line, Character: t.Column + len(t.Literal)},
	}
}

// References answers textDocument/references.
//   - On a CTE name: every use of that CTE within the current document.
//   - On a ref('model') argument: every ref() of that model across the project's models.
//
// Other tokens yield an empty list.
func (s *State) References(id int, uri string, position lsp.Position, includeDeclaration bool) lsp.ReferencesResponse {
	response := lsp.ReferencesResponse{
		Response: lsp.Response{
			RPC: "2.0",
			ID:  &id,
		},
		Result: []lsp.Location{},
	}

	doc, exists := s.Documents[uri]
	if !exists || doc.Tokens == nil {
		return response
	}
	cursorTokenLL, err := doc.Tokens.FindTokenAtCursor(position.Line, position.Character)
	if err != nil {
		return response
	}
	cursorToken := cursorTokenLL.Token

	switch cursorToken.Type {
	case parser.REF:
		response.Result = s.modelReferences(cursorToken.Literal, includeDeclaration)
	case parser.IDENT:
		defToken, ok := doc.DefTokens[strings.ToLower(cursorToken.Literal)]
		if !ok {
			return response
		}
		for _, t := range doc.Tokens.IdentifierReferences(defToken.Literal) {
			if !includeDeclaration && t == defToken {
				continue
			}
			response.Result = append(response.Result, lsp.Location{URI: uri, Range: tokenRange(t)})
		}
	}

	return response
}

// modelReferences scans every .sql/.py model of the current project for
// `ref('<name>')` / `ref('<package>', '<name>')`. Open documents are read from
// memory so unsaved edits are reflected; everything else is read from disk.
func (s *State) modelReferences(modelName string, includeDeclaration bool) []lsp.Location {
	locations := []lsp.Location{}

	projectName := s.DbtContext.ProjectYaml.ProjectName.Value
	paths := make([]string, 0, len(s.DbtContext.ModelDetailMap))
	for _, model := range s.DbtContext.ModelDetailMap {
		if model.ProjectName != projectName {
			continue
		}
		switch filepath.Ext(model.URI) {
		case ".sql", ".py":
			paths = append(paths, model.URI)
		}
	}
	sort.Strings(paths)

	if model, ok := s.DbtContext.ModelDetailMap[modelName]; ok && includeDeclaration && model.URI != "" {
		locations = append(locations, lsp.Location{URI: "file://" + model.URI})
	}

	// Group 1 captures the model-name argument so the location lands on the name, not on `ref(`.
	pattern := regexp.MustCompile(`\bref\(\s*(?:['"][^'"]*['"]\s*,\s*)?['"](` + regexp.QuoteMeta(modelName) + `)['"]`)

	for _, path := range paths {
		uri := "file://" + path
		text := ""
		if doc, open := s.Documents[uri]; open {
			text = doc.Text
		} else {
			contents, err := util.ReadFileContents(path)
			if err != nil {
				continue
			}
			text = contents
		}

		matches := pattern.FindAllStringSubmatchIndex(text, -1)
		if len(matches) == 0 {
			continue
		}

		// Matches are ascending, so line/column can be advanced incrementally.
		line, lineStart, pos := 0, 0, 0
		for _, m := range matches {
			start, end := m[2], m[3]
			for ; pos < start; pos++ {
				if text[pos] == '\n' {
					line++
					lineStart = pos + 1
				}
			}
			locations = append(locations, lsp.Location{
				URI: uri,
				Range: lsp.Range{
					Start: lsp.Position{Line: line, Character: start - lineStart},
					End:   lsp.Position{Line: line, Character: end - lineStart},
				},
			})
		}
	}

	return locations
}
