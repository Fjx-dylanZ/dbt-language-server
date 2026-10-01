package lsp

type DiagnosticsNotification struct {
	Notification
	Params PublishDiagnosticsParams `json:"params"`
}

type PublishDiagnosticsParams struct {
	URI string `json:"uri"`
	// Version is the document version the diagnostics were computed for.
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Message  string `json:"message"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source"`
}

// NewDiagnosticsNotification builds textDocument/publishDiagnostics. A nil
// version publishes without one, e.g. to clear a closed document.
func NewDiagnosticsNotification(uri string, version *int, diagnostics []Diagnostic) DiagnosticsNotification {
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	return DiagnosticsNotification{
		Notification: Notification{
			RPC:    "2.0",
			Method: "textDocument/publishDiagnostics",
		},
		Params: PublishDiagnosticsParams{
			URI:         uri,
			Version:     version,
			Diagnostics: diagnostics,
		},
	}
}

type DocumentDiagnosticRequest struct {
	Request
	Params DocumentDiagnosticParams `json:"params"`
}

type DocumentDiagnosticParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentDiagnosticResponse struct {
	Response
	Result FullDocumentDiagnosticReport `json:"result"`
}

type FullDocumentDiagnosticReport struct {
	Kind  string       `json:"kind"`
	Items []Diagnostic `json:"items"`
}

func NewDocumentDiagnosticResponse(id ID, diagnostics []Diagnostic) DocumentDiagnosticResponse {
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	return DocumentDiagnosticResponse{
		Response: Response{
			RPC: "2.0",
			ID:  id,
		},
		Result: FullDocumentDiagnosticReport{
			Kind:  "full",
			Items: diagnostics,
		},
	}
}
