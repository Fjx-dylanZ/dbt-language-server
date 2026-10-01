package lsp

import "encoding/json"

type ExecuteCommandRequest struct {
	Request
	Params ExecuteCommandParams `json:"params"`
}

// ExecuteCommandParams keeps arguments raw; each command decodes its own.
type ExecuteCommandParams struct {
	Command   string            `json:"command"`
	Arguments []json.RawMessage `json:"arguments"`
}

type ExecuteCommandResponse struct {
	Response
	Result any `json:"result"`
}

// GoToSchemaParams is the single argument of the dbt.goToSchema command.
type GoToSchemaParams struct {
	URI      string   `json:"uri"`
	Position Position `json:"position"`
}
