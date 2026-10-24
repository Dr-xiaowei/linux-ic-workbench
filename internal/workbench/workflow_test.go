package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCodeDiagnosticsAndBackupDisabled(t *testing.T) {
	log := "/tmp/design.sv:2:3: syntax error\n%Warning-WIDTH: /tmp/design.sv:4:5: width mismatch\nError-[SE] Syntax error\n\"/tmp/design.sv\", 6: token missing\n  File \"/tmp/design.sv\", line 8\n/else/dep.sv:9:1: error: dependency\n"
	ds := parseCodeDiagnostics(log, "/tmp/design.sv")
	if len(ds) != 5 || ds[1].Severity != "warning" || ds[2].Line != 6 || !ds[3].CurrentFile || ds[4].CurrentFile {
		t.Fatal(ds)
	}
	a := appForTest(t, "demo")
	legacy := filepath.Join(a.Store.dir, "backup-old.json")
	os.WriteFile(legacy, []byte("retain"), 0600)
	if r := apiCall(a, "/api/backup", "{}", true); r.Code != 400 {
		t.Fatal(r.Code, r.Body.String())
	}
	if _, e := a.action(context.Background(), "backup", json.RawMessage(`{}`)); e == nil {
		t.Fatal("Agent backup enabled")
	}
	files, _ := filepath.Glob(filepath.Join(a.Store.dir, "backup-*"))
	b, _ := os.ReadFile(legacy)
	if len(files) != 1 || string(b) != "retain" {
		t.Fatal("backup changed", files)
	}
}

func TestPython26TupleDiagnostics(t *testing.T) {
	log := "SyntaxError: ('invalid syntax', ('/tmp/code space/design.py', 1, 14, 'def add(a, b)\\n'))\n" +
		"IndentationError: ('unexpected indent', ('/tmp/dependency.py', 7, 4, '    pass\\n'))\n" +
		"  File \"/tmp/code space/design.py\", line 9\n"
	ds := parseCodeDiagnostics(log, "/tmp/code space/design.py")
	if len(ds) != 3 || ds[0].Line != 1 || ds[0].Column != 14 || !ds[0].CurrentFile || ds[0].Message != "SyntaxError: invalid syntax" || ds[1].CurrentFile || ds[1].Line != 7 || ds[2].Line != 9 {
		t.Fatal(ds)
	}
}
func setCodeTools(t *testing.T, a *App, lang string, checker, formatter Command) {
	t.Helper()
	if e := a.Store.Update(func(s *State) error {
		s.Preferences.Compilers = map[string]Command{lang: checker}
		s.Preferences.Formatters = map[string]Command{lang: formatter}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
func codeRun(t *testing.T, a *App, in CodeWorkflowInput) Operation {
	t.Helper()
	op := actionTest(t, a, "code-workflow", in).(Operation)
	return awaitOperation(t, a, op.ID)
}
func fakeCodeModel(t *testing.T, a *App, answer func(int) string) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []modelMessage `json:"messages"`
			Tools    any            `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Tools != nil || len(req.Messages) != 2 || !strings.Contains(req.Messages[1].Content, "Diagnostic log") {
			t.Error("repair evidence missing / unexpected tools")
		}
		n := int(calls.Add(1))
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": modelMessage{Role: "assistant", Content: StringJSON(map[string]string{"code": answer(n), "summary": "Fixture model candidate"})}}}})
	}))
	t.Cleanup(srv.Close)
	a.Config.ModelURL = srv.URL
	a.Config.Model = "fixture"
	return &calls
}
func TestCodeRepairRealCheckerAndOriginalPreservation(t *testing.T) {
	if _, e := os.Stat("/usr/bin/clang"); e != nil {
		t.Skip("clang unavailable")
	}
	a := appForTest(t, "production")
	source := "int main(void) { return 0 }\n"
	fixed := "int main(void) { return 0; }\n"
	path := filepath.Join(a.Store.Snapshot().Projects[0].Root, "original.c")
	os.WriteFile(path, []byte(source), 0600)
	setCodeTools(t, a, "c", Command{Executable: "/usr/bin/clang", Args: []string{"-fsyntax-only", "{file}"}}, Command{})
	calls := fakeCodeModel(t, a, func(int) string { return fixed })
	done := codeRun(t, a, CodeWorkflowInput{Language: "c", Code: source, Mode: "repair", MaxRepairs: 2})
	if done.Status != "succeeded" || len(done.Code.Checks) != 2 || done.Code.Checks[0].Passed || !done.Code.Checks[1].Passed || calls.Load() != 1 || done.Code.Base != source || done.Code.Code != fixed || len(done.Code.Checks[0].Diagnostics) == 0 {
		t.Fatal(StringJSON(done))
	}
	b, _ := os.ReadFile(path)
	if string(b) != source {
		t.Fatal("original changed")
	}
	files, _ := filepath.Glob(filepath.Join(a.Store.dir, ".code-*"))
	if len(files) != 0 {
		t.Fatal("temporary copies retained", files)
	}
	for _, op := range a.Store.Metadata().Operations {
		if op.Code != nil {
			t.Fatal("poll payload contains code")
		}
	}
	t.Logf("real clang: first failed, repair candidate passed; original retained; checks=%d", len(done.Code.Checks))
}
func TestCodeRepairBudgetValidationAndCancellation(t *testing.T) {
	a := appForTest(t, "production")
	setCodeTools(t, a, "verilog", Command{Executable: "/bin/sh", Args: []string{"-c", "echo \"$1:1:1: error: fixture\"; exit 1", "checker", "{file}"}}, Command{})
	calls := fakeCodeModel(t, a, func(n int) string { return fmt.Sprintf("module m; // candidate %d\nendmodule\n", n) })
	done := codeRun(t, a, CodeWorkflowInput{Language: "verilog", Code: "module m; endmodule", Mode: "repair", MaxRepairs: 2})
	if done.Status != "failed" || len(done.Code.Checks) != 3 || done.Code.Repairs != 2 || calls.Load() != 2 {
		t.Fatal(StringJSON(done))
	}
	for _, n := range []int{-1, 0, 4} {
		if _, e := a.startCodeWorkflow(json.RawMessage(StringJSON(CodeWorkflowInput{Language: "verilog", Code: "x", Mode: "repair", MaxRepairs: n}))); e == nil {
			t.Fatal("invalid budget accepted", n)
		}
	}
	done = codeRun(t, a, CodeWorkflowInput{Language: "verilog", Code: "x", Mode: "check", MaxRepairs: -1})
	if done.Status != "failed" || len(done.Code.Checks) != 1 {
		t.Fatal("nonrepair budget not normalized", done)
	}
	setCodeTools(t, a, "verilog", Command{Executable: "/does/not/exist", Args: []string{"{file}"}}, Command{})
	done = codeRun(t, a, CodeWorkflowInput{Language: "verilog", Code: "x", Mode: "repair", MaxRepairs: 2})
	if done.Code.Repairs != 0 || done.Code.Checks[0].Executed || calls.Load() != 2 {
		t.Fatal("AI invoked despite unavailable checker")
	}
	setCodeTools(t, a, "verilog", Command{Executable: "/bin/sh", Args: []string{"-c", "sleep 30", "checker", "{file}"}}, Command{})
	op := actionTest(t, a, "code-workflow", CodeWorkflowInput{Language: "verilog", Code: "x", Mode: "check"}).(Operation)
	actionTest(t, a, "cancel-operation", map[string]string{"id": op.ID})
	if done = awaitOperation(t, a, op.ID); done.Status != "cancelled" {
		t.Fatal(done)
	}
}
func TestCodeFormattingBoundariesOutputAndFailure(t *testing.T) {
	a := appForTest(t, "production")
	source := "module m;  \n  wire a;  \nendmodule  \n"
	good := Command{Executable: "/bin/sh", Args: []string{"-c", `cat "$1"; echo diagnostic-only >&2`, "formatter", "{file}", "{startLine}", "{endLine}"}}
	setCodeTools(t, a, "verilog", Command{}, good)
	done := codeRun(t, a, CodeWorkflowInput{Language: "verilog", Code: source, Mode: "format", StartLine: 2, EndLine: 2})
	if done.Status != "succeeded" || done.Code.Code != source || strings.Contains(done.Code.Code, "diagnostic-only") {
		t.Fatal(done)
	}
	bad := good
	bad.Args = append([]string{}, good.Args...)
	bad.Args[1] = `echo changed; cat "$1"`
	setCodeTools(t, a, "verilog", Command{}, bad)
	done = codeRun(t, a, CodeWorkflowInput{Language: "verilog", Code: source, Mode: "format", StartLine: 2, EndLine: 2})
	if done.Status != "failed" || !strings.Contains(done.Error, "outside selected") {
		t.Fatal(done)
	}
	bad.Args[1] = `head -c 1100000 /dev/zero`
	setCodeTools(t, a, "verilog", Command{}, bad)
	done = codeRun(t, a, CodeWorkflowInput{Language: "verilog", Code: source, Mode: "format"})
	if done.Status != "failed" || !strings.Contains(done.Error, "exceeds") {
		t.Fatal(done)
	}
	bad.Args[1] = `echo "syntax error" >&2; exit 1`
	setCodeTools(t, a, "verilog", Command{}, bad)
	done = codeRun(t, a, CodeWorkflowInput{Language: "verilog", Code: source, Mode: "format"})
	if done.Status != "failed" || done.Code.Code != source {
		t.Fatal(done)
	}
}
func TestCodeVeribleOfficialBinary(t *testing.T) {
	bin := os.Getenv("ICW_VERIBLE_BIN")
	if bin == "" {
		t.Skip("set ICW_VERIBLE_BIN to official Verible bin directory")
	}
	a := appForTest(t, "production")
	checker := Command{Executable: filepath.Join(bin, "verible-verilog-lint"), Args: []string{"--ruleset=none", "{file}"}}
	formatter := Command{Executable: filepath.Join(bin, "verible-verilog-format"), Args: []string{"--failsafe_success=false", "--lines={startLine}-{endLine}", "{file}"}}
	setCodeTools(t, a, "systemverilog", checker, formatter)
	original := "module dut;\nwire a\nassign a = 1'b0;\nendmodule\n"
	fixed := "module dut;\n  wire a;\n  assign a = 1'b0;\nendmodule\n"
	fakeCodeModel(t, a, func(int) string { return fixed })
	done := codeRun(t, a, CodeWorkflowInput{Language: "systemverilog", Code: original, Mode: "repair", MaxRepairs: 1})
	if done.Status != "succeeded" || len(done.Code.Checks) != 2 || done.Code.Checks[0].Passed || len(done.Code.Checks[0].Diagnostics) == 0 {
		t.Fatal(StringJSON(done))
	}
	t.Log("Verible actual first diagnostic:", done.Code.Checks[0].Log)
	done = codeRun(t, a, CodeWorkflowInput{Language: "systemverilog", Code: "module dut;\nwire   a;\nendmodule\n", Mode: "format", StartLine: 2, EndLine: 2})
	if done.Status != "succeeded" || done.Code.Code == done.Code.Base || !strings.HasPrefix(done.Code.Code, "module dut;\n") || !strings.HasSuffix(done.Code.Code, "endmodule\n") {
		t.Fatal(StringJSON(done))
	}
	done = codeRun(t, a, CodeWorkflowInput{Language: "systemverilog", Code: original, Mode: "format"})
	if done.Status != "failed" {
		t.Fatal("malformed format reported success", done)
	}
	t.Log("Official Verible: syntax repair recheck, selected-line formatting, malformed source rejection passed")
}
func TestEDAAgentSelectedToolsLiveProgressAndPersistence(t *testing.T) {
	a := appForTest(t, "production")
	var toolCalls atomic.Int32
	progress := make(chan struct{}, 1)
	release := make(chan struct{}, 1)
	server := mcp.NewServer(&mcp.Implementation{Name: "EDA contract", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "run_eda", Description: "Run contract fixture"}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Value string `json:"value"`
	}) (*mcp.CallToolResult, map[string]string, error) {
		toolCalls.Add(1)
		if in.Value != "design-A" {
			t.Error("wrong tool arguments")
		}
		if e := req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: req.Params.GetProgressToken(), Progress: 1, Total: 2, Message: "netlist loaded"}); e != nil {
			t.Error(e)
		}
		progress <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
		return nil, map[string]string{"result": "simulation complete fixture"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "forbidden"}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, map[string]string, error) {
		t.Error("forbidden invoked")
		return nil, nil, nil
	})
	host := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	defer host.Close()
	defer a.Close()
	c := EDAConnection{ID: "eda-a", Name: "Selected software", Transport: "http", URL: host.URL, Timeout: 10, AllowedTools: []string{"run_eda"}}
	actionTest(t, a, "save-mcp", c)
	var modelCalls atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []modelMessage `json:"messages"`
			Tools    []struct {
				Function struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"function"`
			} `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		modelCalls.Add(1)
		if len(req.Tools) != 1 || !strings.HasPrefix(req.Tools[0].Function.Description, "run_eda:") {
			t.Error("unexpected advertised tools", StringJSON(req.Tools))
			http.Error(w, "bad tools", 500)
			return
		}
		m := modelMessage{Role: "assistant", Content: "Completed from MCP receipt"}
		if req.Messages[len(req.Messages)-1].Role != "tool" {
			tc := toolCall{ID: "eda-call", Type: "function"}
			tc.Function.Name = req.Tools[0].Function.Name
			tc.Function.Arguments = `{"value":"design-A"}`
			m.Content = ""
			m.ToolCalls = []toolCall{tc}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": m}}})
	}))
	defer model.Close()
	a.Config.ModelURL = model.URL
	a.Config.Model = "fixture"
	a.Config.ModelTools = true
	task := actionTest(t, a, "eda-agent", map[string]string{"id": c.ID, "text": "run design-A"}).(map[string]string)
	select {
	case <-progress:
	case <-time.After(3 * time.Second):
		t.Fatal("no MCP call")
	}
	found := false
	for i := 0; i < 200; i++ {
		v, _ := a.agentProgress(task["id"])
		run := v.(AgentRun)
		for _, ev := range run.Events {
			if ev.Stage == "progress" && strings.Contains(ev.Text, "netlist loaded") {
				found = true
			}
		}
		if found {
			if run.Status != "running" {
				t.Fatal("progress arrived only after completion")
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !found {
		t.Fatal("MCP progress not forwarded")
	}
	release <- struct{}{}
	done := awaitAgent(t, a, task["id"], "completed")
	if toolCalls.Load() != 1 || modelCalls.Load() != 2 || done.Connection != c.ID {
		t.Fatal(done)
	}
	if _, e := a.startEDAAgent(json.RawMessage(StringJSON(map[string]string{"id": "builtin", "session": task["session"], "text": "wrong connection"}))); e == nil {
		t.Fatal("session scope changed")
	}
	state := a.Store.Snapshot()
	var saved Session
	for _, s := range state.Sessions {
		if s.ID == task["session"] {
			saved = s
		}
	}
	if saved.LastTask == nil || saved.LastTask.Status != "completed" || len(saved.Messages) != 2 {
		t.Fatal(saved)
	}
	a.mu.Lock()
	delete(a.agentRuns, task["id"])
	a.mu.Unlock()
	if v, e := a.agentProgress(task["id"]); e != nil || v.(AgentRun).Content != done.Content {
		t.Fatal("persisted progress lost", e)
	}
	for _, s := range a.Store.Metadata().Sessions {
		if s.LastTask != nil && (s.LastTask.Content != "" || len(s.LastTask.Events) > 0) {
			t.Fatal("poll payload contains full trace")
		}
	}
	saved.LastTask.Status = "running"
	state.Sessions = []Session{saved}
	dir := privateTestDir(t)
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(StringJSON(state)), 0600)
	store, e := OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	if store.Snapshot().Sessions[0].LastTask.Status != "interrupted" {
		t.Fatal("restart resumes unsafe EDA execution")
	}
}
func TestEDAAgentCancelAndUnknownTool(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprint(unknown), func(t *testing.T) {
			a := appForTest(t, "production")
			started := make(chan struct{}, 1)
			stopped := make(chan struct{}, 1)
			server := mcp.NewServer(&mcp.Implementation{Name: "cancel fixture", Version: "1"}, nil)
			mcp.AddTool(server, &mcp.Tool{Name: "slow"}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, map[string]string, error) {
				started <- struct{}{}
				<-ctx.Done()
				stopped <- struct{}{}
				return nil, nil, ctx.Err()
			})
			host := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
			defer host.Close()
			defer a.Close()
			c := EDAConnection{ID: "fixture", Name: "Cancel fixture", Transport: "http", URL: host.URL, Timeout: 5, AllowedTools: []string{"slow"}}
			actionTest(t, a, "save-mcp", c)
			model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc := toolCall{ID: "c", Type: "function"}
				tc.Function.Name = "eda_0"
				if unknown {
					tc.Function.Name = "platform_action"
				}
				tc.Function.Arguments = "{}"
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": modelMessage{Role: "assistant", ToolCalls: []toolCall{tc}}}}})
			}))
			defer model.Close()
			a.Config.ModelURL = model.URL
			a.Config.Model = "fixture"
			a.Config.ModelTools = true
			task := actionTest(t, a, "eda-agent", map[string]string{"id": c.ID, "text": "run"}).(map[string]string)
			if unknown {
				r := awaitAgent(t, a, task["id"], "failed")
				if !strings.Contains(r.Error, "not advertised") {
					t.Fatal(r)
				}
				if len(a.Store.Snapshot().Operations) != 0 {
					t.Fatal("unknown tool executed")
				}
				return
			}
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("not started")
			}
			if r := apiCall(a, "/api/v2/cancel-agent", StringJSON(map[string]string{"id": task["id"]}), true); r.Code != 200 {
				t.Fatal(r.Body.String())
			}
			awaitAgent(t, a, task["id"], "cancelled")
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("MCP cancellation not propagated")
			}
		})
	}
}
func TestEDAAgentDemoTracksSimulation(t *testing.T) {
	a := appForTest(t, "demo")
	task := actionTest(t, a, "eda-agent", map[string]string{"id": "builtin", "text": "simulate project"}).(map[string]string)
	run := awaitAgent(t, a, task["id"], "completed")
	if !strings.Contains(run.Content, "completed") || !strings.Contains(run.Content, "DEMO") {
		t.Fatal(run)
	}
	jobs := a.Store.Snapshot().Jobs
	if len(jobs) != 1 || jobs[0].Status != "completed" {
		t.Fatal(jobs)
	}
}
