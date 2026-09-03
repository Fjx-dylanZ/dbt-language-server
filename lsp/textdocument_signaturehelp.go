package lsp

type SignatureHelpRequest struct {
	Request
	Params SignatureHelpParams `json:"params"`
}

type SignatureHelpParams struct {
	TextDocumentPositionParams
}

// SignatureHelpResponse carries a null result when the cursor is not inside a
// known call.
type SignatureHelpResponse struct {
	Response
	Result *SignatureHelp `json:"result"`
}

type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type SignatureInformation struct {
	Label         string                 `json:"label"`
	Documentation *MarkupContent         `json:"documentation,omitempty"`
	Parameters    []ParameterInformation `json:"parameters"`
}

type ParameterInformation struct {
	// Label is the [start, end) UTF-16 span of the parameter inside the signature label.
	Label [2]int `json:"label"`
}

type MarkupContent struct {
	Kind  string `json:"kind"` // "plaintext" | "markdown"
	Value string `json:"value"`
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}
