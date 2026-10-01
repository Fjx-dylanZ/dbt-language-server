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
//   - On a Jinja variable (`{% set %}`, `{% for %}` target, macro parameter):
//     every use within its scope in the current document.
//   - On a ref('model') argument: every ref() of that model across the project.
//   - On a macro call: every call of that macro across the project.
//
// Other tokens yield an empty list.
func (s *State) References(id lsp.ID, uri string, position lsp.Position, includeDeclaration bool) lsp.ReferencesResponse {
	response := lsp.ReferencesResponse{
		Response: lsp.Response{
			RPC: "2.0",
			ID:  id,
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
	case parser.MACRO:
		packageName := Package(s.DbtContext.ProjectYaml.ProjectName.Value)
		if match, literal := cursorTokenLL.TokenLookbackMatch(parser.PACKAGE, 2); match {
			packageName = Package(literal)
		}
		macro, ok := s.DbtContext.MacroDetailMap[packageName][cursorToken.Literal]
		if !ok {
			return response
		}
		response.Result = s.macroReferences(macro, includeDeclaration)
	case parser.IDENT:
		var def parser.Token
		var refs []parser.Token
		if cursorTokenLL.Jinja {
			d, ok := doc.Jinja.Resolve(cursorToken)
			if !ok {
				return response
			}
			def, refs = d, doc.Jinja.References(doc.Tokens, d)
		} else {
			d, ok := doc.DefTokens[strings.ToLower(cursorToken.Literal)]
			if !ok {
				return response
			}
			def, refs = d, doc.Tokens.IdentifierReferences(d.Literal)
		}
		for _, t := range refs {
			if !includeDeclaration && t == def {
				continue
			}
			response.Result = append(response.Result, lsp.Location{URI: uri, Range: tokenRange(t)})
		}
	}

	return response
}

// modelReferences finds `ref('<name>')` / `ref('<package>', '<name>')` across the project.
func (s *State) modelReferences(modelName string, includeDeclaration bool) []lsp.Location {
	locations := []lsp.Location{}

	if model, ok := s.DbtContext.ModelDetailMap[modelName]; ok && includeDeclaration && model.URI != "" {
		locations = append(locations, lsp.Location{URI: "file://" + model.URI})
	}

	// Group 1 captures the model-name argument so the location lands on the name, not on `ref(`.
	pattern := regexp.MustCompile(`\bref\(\s*(?:['"][^'"]*['"]\s*,\s*)?['"](` + regexp.QuoteMeta(modelName) + `)['"]`)

	return append(locations, s.scanProject(func(text string) [][2]int {
		var spans [][2]int
		for _, block := range jinjaBlocks(text) {
			for _, m := range pattern.FindAllStringSubmatchIndex(text[block[0]:block[1]], -1) {
				spans = append(spans, [2]int{block[0] + m[2], block[0] + m[3]})
			}
		}
		return spans
	})...)
}

// A `{# … #}` comment is matched as a whole so the `{{ … }}` examples it may
// contain are never reported as blocks of their own.
var jinjaBlockPattern = regexp.MustCompile(`(?s)\{#.*?#\}|\{[{%].*?[}%]\}`)

// jinjaBlocks returns the byte spans of every rendered `{{ … }}` / `{% … %}` block.
func jinjaBlocks(text string) [][2]int {
	var blocks [][2]int
	for _, b := range jinjaBlockPattern.FindAllStringIndex(text, -1) {
		if text[b[0]+1] != '#' {
			blocks = append(blocks, [2]int{b[0], b[1]})
		}
	}
	return blocks
}

// macroReferences finds calls of `macro` inside Jinja blocks across the project:
// `name(` (unqualified) and `pkg.name(` (qualified). An unqualified call binds to
// the project's own macro when one of that name exists, otherwise to the package's.
func (s *State) macroReferences(macro Macro, includeDeclaration bool) []lsp.Location {
	locations := []lsp.Location{}

	if includeDeclaration {
		locations = append(locations, lsp.Location{
			URI: "file://" + macro.URI,
			Range: lsp.Range{
				Start: macro.Range.Start,
				End:   lsp.Position{Line: macro.Range.Start.Line, Character: macro.Range.Start.Character + len(macro.Name)},
			},
		})
	}

	projectName := Package(s.DbtContext.ProjectYaml.ProjectName.Value)
	_, shadowedByProject := s.DbtContext.MacroDetailMap[projectName][macro.Name]
	unqualifiedBinds := macro.ProjectName == projectName || !shadowedByProject

	// Group 1: qualifier package; group 2: `macro` keyword of a definition; group 3: the name.
	pattern := regexp.MustCompile(`(?:\b(\w+)\s*\.\s*)?(?:\b(macro)\s+)?\b(` + regexp.QuoteMeta(macro.Name) + `)\s*\(`)

	return append(locations, s.scanProject(func(text string) [][2]int {
		var spans [][2]int
		for _, block := range jinjaBlocks(text) {
			for _, m := range pattern.FindAllStringSubmatchIndex(text[block[0]:block[1]], -1) {
				if m[4] != -1 { // definition, reported via includeDeclaration
					continue
				}
				if m[2] != -1 {
					if Package(text[block[0]+m[2]:block[0]+m[3]]) != macro.ProjectName {
						continue
					}
				} else if !unqualifiedBinds {
					continue
				}
				spans = append(spans, [2]int{block[0] + m[6], block[0] + m[7]})
			}
		}
		return spans
	})...)
}

// projectSQLFiles lists the project's own model and macro files, sorted.
func (s *State) projectSQLFiles() []string {
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
	for _, p := range s.DbtContext.ProjectYaml.MacroPaths.Value {
		macroFiles, err := util.WalkFilepath(filepath.Join(s.DbtContext.ProjectRoot, p), ".sql")
		if err != nil {
			continue
		}
		paths = append(paths, macroFiles...)
	}
	sort.Strings(paths)
	return paths
}

// scanProject applies match to every project file and converts the returned
// byte spans to locations. Open documents are read from memory so unsaved
// edits are reflected; everything else is read from disk.
func (s *State) scanProject(match func(text string) [][2]int) []lsp.Location {
	var locations []lsp.Location

	for _, path := range s.projectSQLFiles() {
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

		spans := match(text)
		if len(spans) == 0 {
			continue
		}

		// Spans are ascending, so line/column can be advanced incrementally.
		line, lineStart, pos := 0, 0, 0
		for _, span := range spans {
			start, end := span[0], span[1]
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
