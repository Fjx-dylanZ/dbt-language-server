package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strconv"

	flag "github.com/spf13/pflag"

	"github.com/j-clemons/dbt-language-server/analysis"
	"github.com/j-clemons/dbt-language-server/analysis/fusion"
	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/rpc"
	"github.com/j-clemons/dbt-language-server/util"
	"github.com/j-clemons/dbt-language-server/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "upgrade" {
		cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/j-clemons/dbt-language-server/main/install | bash")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal("Upgrade failed:", err)
		}
		os.Exit(0)
	}

	showVersion := flag.BoolP("version", "v", false, "Print version")
	debug := flag.BoolP("debug", "d", false, "Enable debug logging to log.txt")

	fusion := flag.StringP("fusion", "f", "", "Enable dbt fusion features. Provide an absolute path if default value is not dbt")
	flag.Lookup("fusion").NoOptDefVal = "dbt"

	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	var logger *log.Logger
	if *debug {
		logger = util.GetLogger("log.txt")
	} else {
		logger = log.New(io.Discard, "", 0)
	}

	state := analysis.NewState()
	state.FusionEnabled = false
	state.FusionPath = *fusion

	if *fusion != "" {
		go func() {
			fusionValidation, err := util.ValidateFusion(*fusion)
			if err != nil {
				logger.Println(err)
			}
			state.SetFusionEnabled(fusionValidation)
		}()
	}

	logger.Println("dbt Language Server Started!")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(rpc.Split)
	writer := os.Stdout

	for scanner.Scan() {
		msg := scanner.Bytes()
		method, contents, err := rpc.DecodeMessage(msg)
		if err != nil {
			logger.Printf("Got an error: %s", err)
		}
		handleMessage(logger, writer, &state, method, contents)
	}
}

func handleMessage(logger *log.Logger, writer io.Writer, state *analysis.State, method string, contents []byte) {
	logger.Printf("Received msg with method: %s", method)

	switch method {
	case "initialize":
		var request lsp.InitializeRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		logger.Printf("Connected to: %s %s %s",
			request.Params.ClientInfo.Name,
			request.Params.ClientInfo.Version,
			request.Params.RootPath,
		)

		capabilities := request.Params.Capabilities
		state.LspClientRootPath = request.Params.RootPath
		state.PullDiagnostics = capabilities.TextDocument.Diagnostic != nil
		state.DiagnosticRefresh = capabilities.Workspace.Diagnostics.RefreshSupport
		state.WorkDoneProgress = capabilities.Window.WorkDoneProgress

		util.WriteResponse(writer, lsp.NewInitializeResponse(request.ID, state.PullDiagnostics))
		logger.Print("Sent the reply")
	case "initialized":
		// Read the project up front. A client that accepts server-initiated
		// progress is first asked for a token, and the reading is reported on it
		// once the client replies: some clients (e.g. omp) hold project-wide
		// requests such as diagnostics until that progress ends.
		if state.WorkDoneProgress {
			util.WriteResponse(writer, lsp.NewWorkDoneProgressCreateRequest(loadProjectRequestID, loadProjectToken))
			return
		}
		state.LoadProject()
	case "":
		// A client's reply to a server request. Only the progress token request
		// needs handling; refresh replies carry nothing.
		var reply struct {
			ID    lsp.ID             `json:"id"`
			Error *lsp.ResponseError `json:"error"`
		}
		if err := json.Unmarshal(contents, &reply); err != nil || !bytes.Equal(reply.ID, loadProjectRequestID) {
			return
		}
		if reply.Error != nil {
			logger.Printf("No progress token: %s", reply.Error.Message)
			state.LoadProject()
			return
		}
		util.WriteResponse(writer, lsp.NewProgressNotification(loadProjectToken, "begin", "Reading dbt project"))
		state.LoadProject()
		util.WriteResponse(writer, lsp.NewProgressNotification(loadProjectToken, "end", ""))
	case "textDocument/didOpen":
		var request lsp.DidOpenTextDocumentNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didOpen: %s", err)
			return
		}

		uri := request.Params.TextDocument.URI
		state.OpenDocument(uri, request.Params.TextDocument.Text, request.Params.TextDocument.Version)
		logger.Printf("Opened: %s", uri)

		state.SetFusionDiagnostics(uri, fusion.FusionCompile(state, uri, logger))
		// Opening re-reads the project, which can change any open document's results.
		diagnosticsChanged(writer, state)
	case "textDocument/didSave":
		logger.Print("textDocument/didSave")
		var request lsp.DidSaveTextDocumentNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didSave: %s", err)
			return
		}

		uri := request.Params.TextDocument.URI
		logger.Printf("Saved: %s", uri)
		state.SaveDocument(uri)

		state.SetFusionDiagnostics(uri, fusion.FusionCompile(state, uri, logger))
		diagnosticsChanged(writer, state)
	case "textDocument/didChange":
		var request lsp.TextDocumentDidChangeNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didChange: %s", err)
			return
		}

		uri := request.Params.TextDocument.URI
		logger.Printf("Changed: %s", uri)
		state.UpdateDocumentIncremental(uri, request.Params.TextDocument.Version, request.Params.ContentChanges)
		publishDiagnostics(writer, state, uri)
	case "textDocument/didClose":
		var request lsp.DidCloseTextDocumentNotification
		if err := json.Unmarshal(contents, &request); err != nil {
			logger.Printf("textDocument/didClose: %s", err)
			return
		}

		uri := request.Params.TextDocument.URI
		logger.Printf("Closed: %s", uri)
		state.CloseDocument(uri)
		if !state.PullDiagnostics {
			util.WriteResponse(writer, lsp.NewDiagnosticsNotification(uri, nil, nil))
		}
	case "textDocument/diagnostic":
		var request lsp.DocumentDiagnosticRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		diagnostics := state.Diagnostics(request.Params.TextDocument.URI)
		util.WriteResponse(writer, lsp.NewDocumentDiagnosticResponse(request.ID, diagnostics))
	case "textDocument/hover":
		var request lsp.HoverRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		response := state.Hover(request.ID, request.Params.TextDocument.URI, request.Params.Position)

		util.WriteResponse(writer, response)
	case "textDocument/definition":
		logger.Print("textDocument/definition")
		var request lsp.DefinitionRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		response := state.Definition(request.ID, request.Params.TextDocument.URI, request.Params.Position)

		util.WriteResponse(writer, response)
	case "textDocument/references":
		logger.Print("textDocument/references")
		var request lsp.ReferencesRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		response := state.References(
			request.ID,
			request.Params.TextDocument.URI,
			request.Params.Position,
			request.Params.Context.IncludeDeclaration,
		)

		util.WriteResponse(writer, response)
	case "textDocument/signatureHelp":
		logger.Print("textDocument/signatureHelp")
		var request lsp.SignatureHelpRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		response := state.SignatureHelp(request.ID, request.Params.TextDocument.URI, request.Params.Position)

		util.WriteResponse(writer, response)
	case "textDocument/completion":
		logger.Print("textDocument/completion")
		var request lsp.CompletionRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		response := state.TextDocumentCompletion(request.ID, request.Params.TextDocument.URI, request.Params.Position)

		util.WriteResponse(writer, response)
	case "shutdown":
		var request lsp.Request
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		logger.Print("Received shutdown request")
		response := lsp.ShutdownResponse{
			Response: lsp.Response{
				RPC: "2.0",
				ID:  request.ID,
			},
		}
		util.WriteResponse(writer, response)
	case "exit":
		logger.Print("Received exit notification")
		os.Exit(0)
	case "workspace/executeCommand":
		logger.Print("workspace/executeCommand")
		var request lsp.ExecuteCommandRequest
		if !decodeRequest(logger, writer, method, contents, &request) {
			return
		}

		switch command := request.Params.Command; command {
		case "dbt.goToSchema":
			var params lsp.GoToSchemaParams
			if len(request.Params.Arguments) == 0 ||
				json.Unmarshal(request.Params.Arguments[0], &params) != nil ||
				params.URI == "" {
				logger.Printf("%s: bad arguments %s", command, request.Params.Arguments)
				util.WriteResponse(writer, lsp.NewErrorResponse(request.ID, lsp.InvalidParams,
					"dbt.goToSchema expects an argument {uri, position}"))
				return
			}

			response := state.GoToSchema(request.ID, params.URI, params.Position)
			util.WriteResponse(writer, response)
		default:
			logger.Printf("Unknown command: %s", command)
			util.WriteResponse(writer, lsp.NewErrorResponse(request.ID, lsp.InvalidParams,
				fmt.Sprintf("Unknown command: %s", command)))
		}
	default:
		// JSON-RPC requires a reply to every request; notifications (no id)
		// must not be answered.
		id := requestID(contents)
		if id == nil {
			return
		}
		logger.Printf("Method not found: %s", method)
		util.WriteResponse(writer, lsp.NewErrorResponse(id, lsp.MethodNotFound, "Method not found: "+method))
	}
}

// decodeRequest unmarshals a request into v. When that fails it answers with
// InvalidParams, so the client is not left waiting, and returns false.
func decodeRequest(logger *log.Logger, writer io.Writer, method string, contents []byte, v any) bool {
	err := json.Unmarshal(contents, v)
	if err == nil {
		return true
	}

	logger.Printf("%s: %s", method, err)
	if id := requestID(contents); id != nil {
		util.WriteResponse(writer, lsp.NewErrorResponse(id, lsp.InvalidParams,
			fmt.Sprintf("Invalid params for %s: %s", method, err)))
	}
	return false
}

// requestID returns a message's id exactly as sent, or nil when it has none.
func requestID(contents []byte) lsp.ID {
	var message struct {
		ID lsp.ID `json:"id"`
	}
	if err := json.Unmarshal(contents, &message); err != nil {
		return nil
	}
	return message.ID
}

// loadProjectRequestID is the id of the window/workDoneProgress/create request
// sent on `initialized`; the client's reply starts the project reading.
var loadProjectRequestID = lsp.ID(`"load-project"`)

const loadProjectToken = "dbt-language-server/load-project"

// serverRequestID numbers the other requests the server sends to the client.
var serverRequestID int

// diagnosticsChanged reports that every open document's diagnostics may have
// changed: pull clients are asked to pull again, push clients get them.
func diagnosticsChanged(writer io.Writer, state *analysis.State) {
	if state.PullDiagnostics {
		if state.DiagnosticRefresh {
			serverRequestID++
			util.WriteResponse(writer, lsp.Request{
				RPC:    "2.0",
				ID:     lsp.ID(strconv.Itoa(serverRequestID)),
				Method: "workspace/diagnostic/refresh",
			})
		}
		return
	}

	for _, uri := range slices.Sorted(maps.Keys(state.Documents)) {
		publishDiagnostics(writer, state, uri)
	}
}

// publishDiagnostics pushes an open document's diagnostics, tagged with the
// version they were computed for. Pull clients ask for them instead.
func publishDiagnostics(writer io.Writer, state *analysis.State, uri string) {
	doc, ok := state.Documents[uri]
	if state.PullDiagnostics || !ok {
		return
	}

	version := doc.Version
	util.WriteResponse(writer, lsp.NewDiagnosticsNotification(uri, &version, state.Diagnostics(uri)))
}
