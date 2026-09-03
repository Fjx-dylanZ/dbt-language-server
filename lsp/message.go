package lsp

type Request struct {
	RPC    string `json:"jsonrpc"`
	ID     int    `json:"id"`
	Method string `json:"method"`

	// Specify params for all request types
	// Params
}

type Response struct {
	RPC string `json:"jsonrpc"`
	ID  *int   `json:"id"`

	// Result
	// Error
}

// ShutdownResponse carries an explicit `"result": null`; clients (e.g. Neovim)
// reject a response that has neither `result` nor `error`.
type ShutdownResponse struct {
	Response
	Result any `json:"result"`
}

type Notification struct {
	RPC    string `json:"jsonrpc"`
	Method string `json:"method"`
}
