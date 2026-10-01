package lsp

import "encoding/json"

// ID is a JSON-RPC request id kept as raw JSON: LSP ids may be integers or
// strings, and a response must echo the id exactly as it was sent.
type ID = json.RawMessage

type Request struct {
	RPC    string `json:"jsonrpc"`
	ID     ID     `json:"id"`
	Method string `json:"method"`

	// Specify params for all request types
	// Params
}

type Response struct {
	RPC string `json:"jsonrpc"`
	ID  ID     `json:"id"`

	// Result
	// Error
}

// ShutdownResponse carries an explicit `"result": null`; clients (e.g. Neovim)
// reject a response that has neither `result` nor `error`.
type ShutdownResponse struct {
	Response
	Result any `json:"result"`
}

// JSON-RPC 2.0 error codes.
const (
	MethodNotFound = -32601
	InvalidParams  = -32602
)

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	RPC   string        `json:"jsonrpc"`
	ID    ID            `json:"id"`
	Error ResponseError `json:"error"`
}

func NewErrorResponse(id ID, code int, message string) ErrorResponse {
	return ErrorResponse{
		RPC: "2.0",
		ID:  id,
		Error: ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

type Notification struct {
	RPC    string `json:"jsonrpc"`
	Method string `json:"method"`
}
