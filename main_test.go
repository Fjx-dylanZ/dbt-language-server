package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/analysis/parser"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/rpc"
)

// message is any frame the server writes: response or notification/request.
type message struct {
	ID     json.RawMessage    `json:"id"`
	Method string             `json:"method"`
	Params json.RawMessage    `json:"params"`
	Result json.RawMessage    `json:"result"`
	Error  *lsp.ResponseError `json:"error"`
}

// send runs one client message through handleMessage and returns every frame written.
func send(t *testing.T, state *analysis.State, method, contents string) []message {
	t.Helper()
	var out bytes.Buffer
	handleMessage(log.New(io.Discard, "", 0), &out, state, method, []byte(contents))

	scanner := bufio.NewScanner(&out)
	scanner.Split(rpc.Split)
	var messages []message
	for scanner.Scan() {
		_, body, err := rpc.DecodeMessage(scanner.Bytes())
		if err != nil {
			t.Fatalf("failed to decode frame: %v", err)
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("frame is not valid JSON: %v: %s", err, body)
		}
		messages = append(messages, m)
	}
	return messages
}

// sendOne is send for a request that must get exactly one response.
func sendOne(t *testing.T, state *analysis.State, method, contents string) message {
	t.Helper()
	messages := send(t, state, method, contents)
	if len(messages) != 1 {
		t.Fatalf("%s: expected exactly 1 response, got %d: %+v", method, len(messages), messages)
	}
	return messages[0]
}

func testdataRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("testdata/jaffle_shop_duckdb")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRequestErrorsAreAnswered(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		id      string
		params  string
		code    int
		message string
	}{
		{
			name:    "unknown method, integer id",
			method:  "textDocument/documentSymbol",
			id:      `7`,
			params:  `{"textDocument":{"uri":"file:///x.sql"}}`,
			code:    lsp.MethodNotFound,
			message: "Method not found: textDocument/documentSymbol",
		},
		{
			name:    "unknown method, string id",
			method:  "textDocument/documentSymbol",
			id:      `"req-7"`,
			params:  `{"textDocument":{"uri":"file:///x.sql"}}`,
			code:    lsp.MethodNotFound,
			message: "Method not found: textDocument/documentSymbol",
		},
		{
			name:   "handled method with malformed params, integer id",
			method: "textDocument/hover",
			id:     `8`,
			params: `{"textDocument":{"uri":"file:///x.sql"},"position":"start"}`,
			code:   lsp.InvalidParams,
		},
		{
			name:   "handled method with malformed params, string id",
			method: "textDocument/definition",
			id:     `"req-8"`,
			params: `{"textDocument":{"uri":42},"position":{"line":0,"character":0}}`,
			code:   lsp.InvalidParams,
		},
		{
			name:    "unknown command",
			method:  "workspace/executeCommand",
			id:      `9`,
			params:  `{"command":"dbt.nope","arguments":[]}`,
			code:    lsp.InvalidParams,
			message: "Unknown command: dbt.nope",
		},
		{
			name:    "goToSchema without arguments",
			method:  "workspace/executeCommand",
			id:      `10`,
			params:  `{"command":"dbt.goToSchema"}`,
			code:    lsp.InvalidParams,
			message: "dbt.goToSchema expects an argument {uri, position}",
		},
		{
			name:    "goToSchema with a malformed argument",
			method:  "workspace/executeCommand",
			id:      `11`,
			params:  `{"command":"dbt.goToSchema","arguments":["file:///x.sql"]}`,
			code:    lsp.InvalidParams,
			message: "dbt.goToSchema expects an argument {uri, position}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := analysis.NewState()
			contents := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"%s","params":%s}`, tt.id, tt.method, tt.params)
			response := sendOne(t, &state, tt.method, contents)

			if string(response.ID) != tt.id {
				t.Errorf("expected id %s echoed verbatim, got %s", tt.id, response.ID)
			}
			if response.Result != nil {
				t.Errorf("error response must not carry a result, got %s", response.Result)
			}
			if response.Error == nil {
				t.Fatalf("expected an error object, got %+v", response)
			}
			if response.Error.Code != tt.code {
				t.Errorf("expected error code %d, got %d", tt.code, response.Error.Code)
			}
			if tt.message != "" && response.Error.Message != tt.message {
				t.Errorf("unexpected error message %q", response.Error.Message)
			}
		})
	}
}

func TestStringIDRequestGetsResult(t *testing.T) {
	state := analysis.NewState()
	response := sendOne(t, &state, "textDocument/hover",
		`{"jsonrpc":"2.0","id":"hover-1","method":"textDocument/hover","params":{"textDocument":{"uri":"file:///x.sql"},"position":{"line":0,"character":0}}}`)

	if string(response.ID) != `"hover-1"` {
		t.Errorf("expected the string id echoed, got %s", response.ID)
	}
	if response.Error != nil || response.Result == nil {
		t.Errorf("expected a result, got %+v", response)
	}
}

// Positions the document has no token for used to crash the server.
func TestRequestsOutsideTheTextAreAnswered(t *testing.T) {
	tests := []struct {
		name   string
		method string
		uri    string
		line   int
		char   int
	}{
		{name: "hover after the last token on a line", method: "textDocument/hover", uri: "file:///open.sql", line: 0, char: 12},
		{name: "definition in a document never opened", method: "textDocument/definition", uri: "file:///closed.sql", line: 0, char: 0},
		{name: "completion past the end of a line", method: "textDocument/completion", uri: "file:///open.sql", line: 0, char: 40},
		{name: "completion below the last line", method: "textDocument/completion", uri: "file:///open.sql", line: 5, char: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := analysis.NewState()
			send(t, &state, "textDocument/didOpen", didOpen("file:///open.sql", 1, "select a   -- note"))

			response := sendOne(t, &state, tt.method, fmt.Sprintf(
				`{"jsonrpc":"2.0","id":1,"method":%q,"params":{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}}`,
				tt.method, tt.uri, tt.line, tt.char))
			if response.Error != nil || response.Result == nil {
				t.Errorf("expected a result, got %+v", response)
			}
		})
	}
}

func TestNotificationsProduceNoOutput(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		contents string
	}{
		{
			name:     "cancel request",
			method:   "$/cancelRequest",
			contents: `{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":2}}`,
		},
		{
			name:     "initialized",
			method:   "initialized",
			contents: `{"jsonrpc":"2.0","method":"initialized","params":{}}`,
		},
		{
			name:     "did change configuration",
			method:   "workspace/didChangeConfiguration",
			contents: `{"jsonrpc":"2.0","method":"workspace/didChangeConfiguration","params":{"settings":{}}}`,
		},
		{
			// A client's response to a server request has an id but no method.
			name:     "client response",
			method:   "",
			contents: `{"jsonrpc":"2.0","id":4,"result":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := analysis.NewState()
			if messages := send(t, &state, tt.method, tt.contents); len(messages) != 0 {
				t.Errorf("expected no output, got %+v", messages)
			}
		})
	}
}

func TestExecuteCommandGoToSchema(t *testing.T) {
	state := analysis.NewState()
	state.DbtContext.ModelDetailMap = map[string]analysis.ModelDetails{
		"customers": {
			URI:       "/test/models/customers.sql",
			SchemaURI: "/test/models/schema.yml",
			SchemaRange: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 10},
				End:   lsp.Position{Line: 5, Character: 10},
			},
		},
	}
	testSQL := `SELECT * FROM {{ ref('customers') }}`
	parserIns := parser.Parse(testSQL, "duckdb")
	state.Documents["file:///test/models/orders.sql"] = analysis.Document{
		Text:      testSQL,
		Tokens:    parserIns.CreateTokenIndex(),
		DefTokens: parserIns.CreateTokenNameMap(),
	}

	response := sendOne(t, &state, "workspace/executeCommand", `{"jsonrpc":"2.0","id":1,"method":"workspace/executeCommand",
		"params":{"command":"dbt.goToSchema","arguments":[{"uri":"file:///test/models/orders.sql","position":{"line":0,"character":25}}]}}`)

	var location lsp.Location
	if err := json.Unmarshal(response.Result, &location); err != nil {
		t.Fatalf("expected a location result, got %+v", response)
	}
	if location.URI != "file:///test/models/schema.yml" || location.Range.Start.Line != 5 {
		t.Errorf("unexpected location %+v", location)
	}
}

func TestRPCMessageHandling(t *testing.T) {
	// Test the full RPC message flow
	executeCommandJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "workspace/executeCommand",
		"params": {
			"command": "dbt.goToSchema",
			"arguments": [
				{
					"uri": "file:///test/models/orders.sql",
					"position": {
						"line": 0,
						"character": 20
					}
				}
			]
		}
	}`

	// Test RPC message decoding
	rpcMessage := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(executeCommandJSON), executeCommandJSON)
	method, contents, err := rpc.DecodeMessage([]byte(rpcMessage))
	if err != nil {
		t.Fatalf("Failed to decode RPC message: %v", err)
	}

	if method != "workspace/executeCommand" {
		t.Errorf("Expected method 'workspace/executeCommand', got '%s'", method)
	}

	// Test JSON unmarshaling
	var request lsp.ExecuteCommandRequest
	if err := json.Unmarshal(contents, &request); err != nil {
		t.Fatalf("Failed to unmarshal execute command request: %v", err)
	}

	if request.Params.Command != "dbt.goToSchema" {
		t.Errorf("Expected command 'dbt.goToSchema', got '%s'", request.Params.Command)
	}
}

func TestResponseEncoding(t *testing.T) {
	// Test that responses are properly encoded
	response := lsp.ExecuteCommandResponse{
		Response: lsp.Response{
			RPC: "2.0",
			ID:  lsp.ID("1"),
		},
		Result: lsp.Location{
			URI: "file:///test/schema.yml",
			Range: lsp.Range{
				Start: lsp.Position{Line: 5, Character: 10},
				End:   lsp.Position{Line: 5, Character: 10},
			},
		},
	}

	// Test encoding
	encoded := rpc.EncodeMessage(response)

	// Should contain proper headers and JSON
	if !strings.Contains(encoded, "Content-Length:") {
		t.Error("Expected Content-Length header in encoded message")
	}

	if !strings.Contains(encoded, `"jsonrpc":"2.0"`) {
		t.Error("Expected jsonrpc field in encoded message")
	}

	if !strings.Contains(encoded, `"file:///test/schema.yml"`) {
		t.Error("Expected URI in encoded message")
	}
}

// initialize starts a session on the testdata project with the given client capabilities.
func initialize(t *testing.T, state *analysis.State, capabilities string) lsp.InitializeResult {
	t.Helper()
	params, _ := json.Marshal(map[string]any{"rootPath": testdataRoot(t), "capabilities": json.RawMessage(capabilities)})
	response := sendOne(t, state, "initialize", fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":%s}`, params))
	var result lsp.InitializeResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("bad initialize result: %+v", response)
	}
	return result
}

func didOpen(uri string, version int, text string) string {
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "sql", "version": version, "text": text}})
	return fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":%s}`, params)
}

func TestDiagnosticsArePushedWithTheDocumentVersion(t *testing.T) {
	state := analysis.NewState()
	if result := initialize(t, &state, `{}`); result.Capabilities.DiagnosticProvider != nil {
		t.Fatalf("a client without pull support must not be offered diagnosticProvider")
	}
	uri := "file://" + filepath.Join(testdataRoot(t), "models/report.sql")

	published := func(messages []message) lsp.PublishDiagnosticsParams {
		t.Helper()
		if len(messages) != 1 || messages[0].Method != "textDocument/publishDiagnostics" {
			t.Fatalf("expected one publishDiagnostics, got %+v", messages)
		}
		var params lsp.PublishDiagnosticsParams
		if err := json.Unmarshal(messages[0].Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.URI != uri {
			t.Fatalf("published for %s, want %s", params.URI, uri)
		}
		return params
	}

	params := published(send(t, &state, "textDocument/didOpen", didOpen(uri, 3, "select * from {{ ref('nope') }}")))
	if params.Version == nil || *params.Version != 3 {
		t.Errorf("expected version 3, got %v", params.Version)
	}
	if len(params.Diagnostics) != 1 || params.Diagnostics[0].Message != "No model, seed or snapshot named 'nope'" {
		t.Errorf("unexpected diagnostics %+v", params.Diagnostics)
	}

	params = published(send(t, &state, "textDocument/didChange", fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":4},"contentChanges":[{"text":"select * from {{ ref('orders') }}"}]}}`, uri)))
	if params.Version == nil || *params.Version != 4 || len(params.Diagnostics) != 0 {
		t.Errorf("expected no diagnostics at version 4, got version %v: %+v", params.Version, params.Diagnostics)
	}

	params = published(send(t, &state, "textDocument/didClose", fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":%q}}}`, uri)))
	if params.Version != nil || params.Diagnostics == nil || len(params.Diagnostics) != 0 {
		t.Errorf("closing must clear diagnostics without a version, got version %v: %+v", params.Version, params.Diagnostics)
	}
}

func TestDiagnosticsArePulled(t *testing.T) {
	state := analysis.NewState()
	result := initialize(t, &state, `{"textDocument":{"diagnostic":{"dynamicRegistration":true}}}`)
	if result.Capabilities.DiagnosticProvider == nil || !result.Capabilities.DiagnosticProvider.InterFileDependencies {
		t.Fatalf("expected diagnosticProvider for a pull client, got %+v", result.Capabilities.DiagnosticProvider)
	}
	uri := "file://" + filepath.Join(testdataRoot(t), "models/report.sql")

	if messages := send(t, &state, "textDocument/didOpen", didOpen(uri, 1, "select * from {{ source('stripe', 'refunds') }}")); len(messages) != 0 {
		t.Fatalf("a pull client must not get pushed diagnostics, got %+v", messages)
	}

	response := sendOne(t, &state, "textDocument/diagnostic", fmt.Sprintf(
		`{"jsonrpc":"2.0","id":"d-1","method":"textDocument/diagnostic","params":{"textDocument":{"uri":%q}}}`, uri))
	var report lsp.FullDocumentDiagnosticReport
	if err := json.Unmarshal(response.Result, &report); err != nil {
		t.Fatalf("bad report: %+v", response)
	}
	if string(response.ID) != `"d-1"` || report.Kind != "full" || len(report.Items) != 1 ||
		report.Items[0].Message != "Source 'stripe' has no table 'refunds'" {
		t.Errorf("unexpected report %s: %+v", response.ID, report)
	}
}

func TestPullClientIsAskedToRefreshAfterSave(t *testing.T) {
	state := analysis.NewState()
	initialize(t, &state, `{"textDocument":{"diagnostic":{}},"workspace":{"diagnostics":{"refreshSupport":true}}}`)
	uri := "file://" + filepath.Join(testdataRoot(t), "models/orders.sql")
	send(t, &state, "textDocument/didOpen", didOpen(uri, 1, "select 1"))

	messages := send(t, &state, "textDocument/didSave", fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":%q}}}`, uri))
	if len(messages) != 1 || messages[0].Method != "workspace/diagnostic/refresh" || messages[0].ID == nil {
		t.Fatalf("expected one workspace/diagnostic/refresh request, got %+v", messages)
	}
}

func TestProjectIsReadWithProgressAfterInitialized(t *testing.T) {
	state := analysis.NewState()
	initialize(t, &state, `{"window":{"workDoneProgress":true}}`)

	messages := send(t, &state, "initialized", `{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	if len(messages) != 1 || messages[0].Method != "window/workDoneProgress/create" || messages[0].ID == nil {
		t.Fatalf("expected a progress token request, got %+v", messages)
	}
	var create lsp.WorkDoneProgressCreateParams
	if err := json.Unmarshal(messages[0].Params, &create); err != nil {
		t.Fatal(err)
	}

	// Progress is reported only once the client has created the token.
	messages = send(t, &state, "", fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":null}`, messages[0].ID))
	var kinds []string
	for _, m := range messages {
		var progress lsp.ProgressParams
		if m.Method != "$/progress" || json.Unmarshal(m.Params, &progress) != nil || progress.Token != create.Token {
			t.Fatalf("expected $/progress on token %q, got %+v", create.Token, m)
		}
		kinds = append(kinds, progress.Value.Kind)
	}
	if strings.Join(kinds, ",") != "begin,end" {
		t.Fatalf("expected begin then end, got %v", kinds)
	}
	if state.DbtContext.ProjectYaml.ProjectName.Value != "jaffle_shop" {
		t.Errorf("project not read: %+v", state.DbtContext.ProjectYaml.ProjectName)
	}
}
