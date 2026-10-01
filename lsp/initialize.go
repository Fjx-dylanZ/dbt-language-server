package lsp

import "github.com/j-clemons/dbt-language-server/version"

type InitializeRequest struct {
	Request
	Params InitializeRequestParams `json:"params"`
}

type InitializeRequestParams struct {
	ClientInfo   ClientInfo         `json:"clientInfo"`
	RootPath     string             `json:"rootPath"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities holds the client features the server adapts to.
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument"`
	Workspace    WorkspaceClientCapabilities    `json:"workspace"`
	Window       WindowClientCapabilities       `json:"window"`
}

type WindowClientCapabilities struct {
	// WorkDoneProgress means the client accepts server-initiated progress.
	WorkDoneProgress bool `json:"workDoneProgress"`
}

type TextDocumentClientCapabilities struct {
	// Diagnostic is set when the client can pull diagnostics with textDocument/diagnostic.
	Diagnostic *DiagnosticClientCapabilities `json:"diagnostic"`
}

type DiagnosticClientCapabilities struct{}

type WorkspaceClientCapabilities struct {
	Diagnostics DiagnosticWorkspaceClientCapabilities `json:"diagnostics"`
}

type DiagnosticWorkspaceClientCapabilities struct {
	// RefreshSupport means the client accepts workspace/diagnostic/refresh.
	RefreshSupport bool `json:"refreshSupport"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResponse struct {
	Response
	Result InitializeResult `json:"result"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	TextDocumentSync int `json:"textDocumentSync"`

	HoverProvider          bool                  `json:"hoverProvider"`
	DefinitionProvider     bool                  `json:"definitionProvider"`
	ReferencesProvider     bool                  `json:"referencesProvider"`
	SignatureHelpProvider  SignatureHelpOptions  `json:"signatureHelpProvider"`
	CompletionProvider     map[string]any        `json:"completionProvider"`
	ExecuteCommandProvider ExecuteCommandOptions `json:"executeCommandProvider"`
	DiagnosticProvider     *DiagnosticOptions    `json:"diagnosticProvider,omitempty"`
}

// DiagnosticOptions advertises pull diagnostics. A document's results depend on
// other files: the models, seeds, snapshots and sources it refers to.
type DiagnosticOptions struct {
	InterFileDependencies bool `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool `json:"workspaceDiagnostics"`
}

type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewInitializeResponse advertises pull diagnostics only to clients that
// support them; the others get diagnostics pushed.
func NewInitializeResponse(id ID, pullDiagnostics bool) InitializeResponse {
	capabilities := ServerCapabilities{
		TextDocumentSync:   2,
		HoverProvider:      true,
		DefinitionProvider: true,
		ReferencesProvider: true,
		SignatureHelpProvider: SignatureHelpOptions{
			TriggerCharacters: []string{"(", ","},
		},
		CompletionProvider: map[string]any{},
		ExecuteCommandProvider: ExecuteCommandOptions{
			Commands: []string{"dbt.goToSchema"},
		},
	}
	if pullDiagnostics {
		capabilities.DiagnosticProvider = &DiagnosticOptions{InterFileDependencies: true}
	}

	return InitializeResponse{
		Response: Response{
			RPC: "2.0",
			ID:  id,
		},
		Result: InitializeResult{
			Capabilities: capabilities,
			ServerInfo: ServerInfo{
				Name:    "dbt-language-server",
				Version: version.Version,
			},
		},
	}
}
