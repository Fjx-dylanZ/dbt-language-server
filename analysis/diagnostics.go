package analysis

import (
	"fmt"

	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
	diagnosticseverity "github.com/j-clemons/dbt-language-server/lsp/diagnosticSeverity"
)

const diagnosticSource = "dbt-language-server"

// Diagnostics returns the problems in an open document: ref() and source()
// targets the project does not define, then the latest dbt Fusion compile
// results. The result is never nil.
func (s *State) Diagnostics(uri string) []lsp.Diagnostic {
	diagnostics := []lsp.Diagnostic{}
	if doc, ok := s.Documents[uri]; ok {
		diagnostics = append(diagnostics, s.unresolvedReferences(doc)...)
	}
	return append(diagnostics, s.FusionDiagnostics[uri]...)
}

// unresolvedReferences checks every ref() and source() call with literal
// arguments against the project index. It reports nothing when no dbt project
// is loaded, and skips two-argument refs into projects that are not installed
// packages (e.g. dbt Mesh dependencies), which it cannot see.
func (s *State) unresolvedReferences(doc Document) []lsp.Diagnostic {
	if s.DbtContext.ProjectYaml.ProjectName.Value == "" || doc.Tokens == nil {
		return nil
	}

	var diagnostics []lsp.Diagnostic
	report := func(at parser.Token, format string, args ...any) {
		diagnostics = append(diagnostics, lsp.Diagnostic{
			Range:    tokenRange(at),
			Message:  fmt.Sprintf(format, args...),
			Severity: diagnosticseverity.Error,
			Source:   diagnosticSource,
		})
	}

	for _, call := range doc.Tokens.DbtCalls(parser.REF) {
		switch len(call.Args) {
		case 1:
			name := call.Args[0]
			if !s.refResolves(name.Literal) {
				report(name, "No model, seed or snapshot named '%s'", name.Literal)
			}
		case 2:
			pkg, name := call.Args[0], call.Args[1]
			if names, installed := s.DbtContext.RefNames[pkg.Literal]; installed && !names[name.Literal] {
				report(name, "No model, seed or snapshot named '%s' in package '%s'", name.Literal, pkg.Literal)
			}
		}
	}

	for _, call := range doc.Tokens.DbtCalls(parser.SOURCE) {
		if len(call.Args) != 2 {
			continue
		}
		sourceName, tableName := call.Args[0], call.Args[1]
		source, ok := s.DbtContext.SourceDetailMap[sourceName.Literal]
		if !ok {
			report(sourceName, "No source named '%s'", sourceName.Literal)
			continue
		}
		if _, ok := source.Tables[tableName.Literal]; !ok {
			report(tableName, "Source '%s' has no table '%s'", sourceName.Literal, tableName.Literal)
		}
	}

	return diagnostics
}

// refResolves reports whether a one-argument ref() finds name in the root
// project or any installed package.
func (s *State) refResolves(name string) bool {
	for _, names := range s.DbtContext.RefNames {
		if names[name] {
			return true
		}
	}
	return false
}
