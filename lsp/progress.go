package lsp

type WorkDoneProgressCreateRequest struct {
	Request
	Params WorkDoneProgressCreateParams `json:"params"`
}

type WorkDoneProgressCreateParams struct {
	Token string `json:"token"`
}

// NewWorkDoneProgressCreateRequest asks the client to create a progress token.
func NewWorkDoneProgressCreateRequest(id ID, token string) WorkDoneProgressCreateRequest {
	return WorkDoneProgressCreateRequest{
		Request: Request{
			RPC:    "2.0",
			ID:     id,
			Method: "window/workDoneProgress/create",
		},
		Params: WorkDoneProgressCreateParams{Token: token},
	}
}

type ProgressNotification struct {
	Notification
	Params ProgressParams `json:"params"`
}

type ProgressParams struct {
	Token string                `json:"token"`
	Value WorkDoneProgressValue `json:"value"`
}

// WorkDoneProgressValue is a begin or end report; Title is set on begin.
type WorkDoneProgressValue struct {
	Kind  string `json:"kind"`
	Title string `json:"title,omitempty"`
}

// NewProgressNotification reports $/progress of kind "begin" (with a title) or "end".
func NewProgressNotification(token, kind, title string) ProgressNotification {
	return ProgressNotification{
		Notification: Notification{
			RPC:    "2.0",
			Method: "$/progress",
		},
		Params: ProgressParams{
			Token: token,
			Value: WorkDoneProgressValue{Kind: kind, Title: title},
		},
	}
}
