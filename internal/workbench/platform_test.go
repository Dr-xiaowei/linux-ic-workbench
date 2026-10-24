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
)

func actionTest(t *testing.T, a *App, name string, input any) any {
	t.Helper()
	v, e := a.action(context.Background(), name, json.RawMessage(StringJSON(input)))
	if e != nil {
		t.Fatal(name, e)
	}
	return v
}
func awaitOperation(t *testing.T, a *App, id string) Operation {
	t.Helper()
	for i := 0; i < 300; i++ {
		for _, o := range a.Store.Snapshot().Operations {
			if o.ID == id && o.Status != "queued" && o.Status != "running" {
				return o
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("operation timeout")
	return Operation{}
}
func awaitAgent(t *testing.T, a *App, id, status string) AgentRun {
	t.Helper()
	for i := 0; i < 500; i++ {
		v, e := a.agentProgress(id)
		if e != nil {
			t.Fatal(e)
		}
		r := v.(AgentRun)
		if r.Status == status {
			return r
		}
		if r.Status == "failed" && status != "failed" {
			t.Fatal(r)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("agent timeout", status)
	return AgentRun{}
}
func TestV2ProjectsAnnotationsSearchPagination(t *testing.T) {
	a := appForTest(t, "demo")
	p := a.Store.Snapshot().Projects[0]
	second := privateTestDir(t)
	p.Paths = []string{second}
	actionTest(t, a, "save-project", p)
	for i := 0; i < 205; i++ {
		os.WriteFile(filepath.Join(p.Root, fmt.Sprintf("gain-%03d.txt", i)), []byte("gain=61 dB\n"), 0600)
	}
	os.WriteFile(filepath.Join(second, "extra.log"), []byte("gain=62"), 0600)
	outside := filepath.Join(privateTestDir(t), "secret")
	os.WriteFile(outside, []byte("gain secret"), 0600)
	os.Symlink(outside, filepath.Join(p.Root, "escape"))
	actionTest(t, a, "note", map[string]any{"project": "p", "path": "gain-001.txt", "notes": "已复核", "description": "TT"})
	v := actionTest(t, a, "browse", map[string]any{"project": "p", "path": "", "offset": 200}).(map[string]any)
	if len(v["entries"].([]any)) < 5 {
		t.Fatal(v)
	}
	cursor := ""
	hits := 0
	for i := 0; i < 10; i++ {
		v := actionTest(t, a, "search", map[string]any{"query": "gain", "cursor": cursor}).(map[string]any)
		hits += len(v["hits"].([]any))
		if !v["more"].(bool) {
			break
		}
		cursor = v["cursor"].(string)
	}
	if hits != 206 {
		t.Fatalf("search hit count: %d", hits)
	}
	if _, e := a.browse("p", p.Root, "escape", 0); e == nil {
		t.Fatal("symlink escape allowed")
	}
	if _, e := a.browse("p", second, "../", 0); e == nil {
		t.Fatal("path escape allowed")
	}
	state := a.Store.Snapshot()
	if len(state.FileNotes) != 1 || len(state.Projects[0].Paths) != 1 {
		t.Fatal(state)
	}
	rec := apiCall(a, "/api/v2/action", StringJSON(map[string]any{"action": "workspace", "input": map[string]any{}}), false)
	if rec.Code != 401 {
		t.Fatal("unauthenticated access")
	}
}
func TestV2WaveformsAndHDLFiles(t *testing.T) {
	a := appForTest(t, "demo")
	v := actionTest(t, a, "analysis", AnalysisInput{Name: "s.csv", Content: "time_s,voltage\n0,0\n1e-9,0.5\n2e-9,1\n"}).(Analysis)
	if len(v.Signals) != 1 || *v.Signals[0].Points[1].V != .5 {
		t.Fatal(v)
	}
	table := actionTest(t, a, "analysis", AnalysisInput{Name: "metrics.csv", Content: "corner,gain\nTT,60\n"}).(Analysis)
	if table.Format != "CSV table" || len(table.Signals) != 0 {
		t.Fatal(table)
	}
	vcd := "$timescale 1ns $end\n$scope module tb $end\n$var wire 1 ! clk $end\n$var wire 1 ! alias $end\n$var wire 8 # q [7:0] $end\n$upscope $end\n$enddefinitions $end\n$dumpvars\n0!\nb00000000 #\n$end\n#5\n1!\nb00000011 #\n#10\nx!\n"
	v = actionTest(t, a, "analysis", AnalysisInput{Name: "s.vcd", Content: vcd}).(Analysis)
	if len(v.Signals) != 3 || len(v.Signals[0].Points) != 3 || len(v.Signals[1].Points) != 3 || v.Signals[0].Points[2].V != nil || *v.Signals[2].Points[1].V != 3 {
		t.Fatal(StringJSON(v))
	}
	for _, x := range []AnalysisInput{{Name: "bad.csv", Content: "t,x\n1,0\n0,1"}, {Name: "nan.csv", Content: "t,x\n0,NaN"}, {Name: "x.fsdb", Content: "binary"}, {Name: "bad.vcd", Content: "$timescale"}} {
		if _, e := a.analyze(json.RawMessage(StringJSON(x))); e == nil {
			t.Fatal("accepted malformed file", x.Name)
		}
	}
	g := actionTest(t, a, "generate-hdl", HDLInput{Language: "systemverilog", Requirements: "counter"}).(map[string]any)
	code := g["code"].(string)
	if !g["demo"].(bool) {
		t.Fatal(g)
	}
	actionTest(t, a, "save-hdl", HDLInput{Project: "p", Language: "systemverilog", Path: "counter.sv", Code: code})
	if _, e := a.saveHDL(json.RawMessage(StringJSON(HDLInput{Project: "p", Language: "systemverilog", Path: "counter.sv", Code: "overwrite"}))); e == nil {
		t.Fatal("overwrote existing file")
	}
	if _, e := a.saveHDL(json.RawMessage(StringJSON(HDLInput{Project: "p", Language: "verilog", Path: "../escape.v", Code: "escape"}))); e == nil {
		t.Fatal("path escape")
	}
}
func TestV2SoftwarePluginAndCompilerExecution(t *testing.T) {
	a := appForTest(t, "production")
	setup := filepath.Join(privateTestDir(t), "env.sh")
	os.WriteFile(setup, []byte("export ICW_TEST_VERSION=version_two\n"), 0600)
	sw := Software{ID: "sw", Name: "test", Command: "printf '%s' \"$ICW_TEST_VERSION\"", Shell: "/bin/sh", Active: "two", Versions: []SoftwareVersion{{Name: "one"}, {Name: "two", Setup: setup}}}
	actionTest(t, a, "save-software", sw)
	op := actionTest(t, a, "launch-software", map[string]any{"id": "sw", "version": "two", "project": "p"}).(Operation)
	done := awaitOperation(t, a, op.ID)
	if done.Status != "succeeded" || done.Log != "version_two" {
		t.Fatal(done)
	}
	plugin := Plugin{ID: "echo", Name: "echo", Command: Command{"/bin/echo", []string{"{project}"}}, Enabled: true, Timeout: 3}
	actionTest(t, a, "install-plugins", map[string]any{"plugins": []Plugin{plugin}})
	done = awaitOperation(t, a, actionTest(t, a, "run-plugin", map[string]any{"id": "echo", "project": "p"}).(Operation).ID)
	if !strings.Contains(done.Log, a.Store.Snapshot().Projects[0].Root) {
		t.Fatal(done)
	}
	plugin.Timeout = 0
	if _, e := a.action(context.Background(), "install-plugins", json.RawMessage(StringJSON(map[string]any{"plugins": []Plugin{plugin}}))); e == nil {
		t.Fatal("invalid manifest accepted")
	}
	p := a.preferences()
	p.Compiler = Command{"/bin/cat", []string{"{file}"}}
	actionTest(t, a, "save-preferences", p)
	op = actionTest(t, a, "compile-hdl", HDLInput{Language: "verilog", Code: "module m; endmodule"}).(Operation)
	done = awaitOperation(t, a, op.ID)
	if done.Log != "module m; endmodule" {
		t.Fatal(done)
	} // Contract only: /bin/cat is deliberately not a real compiler.
}
func TestV2SettingsLoadAndUpgradePreservesData(t *testing.T) {
	dir := privateTestDir(t)
	old := State{Schema: 2, Mode: "production", Language: "en", Projects: []Project{{ID: "p", Root: privateTestDir(t), Notes: "keep"}}, Sessions: []Session{{ID: "s", Messages: []Message{{Role: "user", Content: "history"}}}}}
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(StringJSON(old)), 0600)
	cfg := Defaults()
	cfg.DataDir = dir
	a, e := NewApp(cfg)
	if e != nil {
		t.Fatal(e)
	}
	setup := filepath.Join(privateTestDir(t), "env.sh")
	os.WriteFile(setup, []byte("export ICW_GLOBAL_TEST=loaded\n"), 0600)
	p := a.preferences()
	p.Setup = setup
	p.Shell = "/bin/sh"
	p.Workspace = old.Projects[0].Root
	p.Threshold = 81
	actionTest(t, a, "save-preferences", p)
	for i := 0; i < 200; i++ {
		if a.validationSnapshot().Environment != "checking" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.validationSnapshot().Environment != "ready" {
		t.Fatal(a.validationSnapshot())
	}
	log, e := a.command(context.Background(), Command{"/bin/sh", []string{"-c", "printf '%s' \"$ICW_GLOBAL_TEST\""}}, Environment{}, p.Workspace, nil)
	if e != nil || log != "loaded" {
		t.Fatal(log, e)
	}
	a.Close()
	a, e = NewApp(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	a.StartValidation()
	s := a.Store.Snapshot()
	if s.Schema != Schema || s.Projects[0].Notes != "keep" || s.Sessions[0].Messages[0].Content != "history" || s.Preferences.Threshold != 81 {
		t.Fatal(s)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "pre-migration-*.json"))
	if len(backups) != 1 {
		t.Fatal("missing migration backup")
	}
}
func TestV2AgentToolsProgressApprovalAndDenial(t *testing.T) {
	a := appForTest(t, "production")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []modelMessage `json:"messages"`
			Tools    []any          `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		calls.Add(1)
		if len(req.Tools) == 0 {
			t.Error("missing Agent tools")
		}
		if req.Messages[len(req.Messages)-1].Role == "tool" {
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": modelMessage{Role: "assistant", Content: "Reviewed tool receipt"}}}})
			return
		}
		tc := toolCall{ID: "call-1", Type: "function"}
		tc.Function.Name = "platform_action"
		tc.Function.Arguments = StringJSON(AgentAction{Action: "save-hdl", Input: StringJSON(HDLInput{Project: "p", Language: "verilog", Path: "agent.v", Code: "module m; endmodule"})})
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": modelMessage{Role: "assistant", ToolCalls: []toolCall{tc}}}}})
	}))
	defer server.Close()
	a.Config.ModelURL = server.URL
	a.Config.Model = "test"
	a.Config.ModelTools = true
	v, e := a.startAgent(AgentInput{Session: "project", Text: "write HDL", AllowWrite: true})
	if e != nil {
		t.Fatal(e)
	}
	id := v.(map[string]string)["id"]
	r := awaitAgent(t, a, id, "awaiting_approval")
	target := filepath.Join(a.Store.Snapshot().Projects[0].Root, "agent.v")
	if _, e := os.Stat(target); !os.IsNotExist(e) {
		t.Fatal("executed before approval")
	}
	if r.Pending == nil {
		t.Fatal(r)
	}
	if e := a.approveAgent(id, true); e != nil {
		t.Fatal(e)
	}
	r = awaitAgent(t, a, id, "completed")
	if len(r.Events) < 5 || !strings.Contains(r.Content, "Tool receipts") {
		t.Fatal(r)
	}
	if _, e := os.Stat(target); e != nil {
		t.Fatal(e)
	}
	v, e = a.startAgent(AgentInput{Session: "general", Text: "write HDL", AllowWrite: false})
	if e != nil {
		t.Fatal(e)
	}
	r = awaitAgent(t, a, v.(map[string]string)["id"], "completed")
	if r.Pending != nil {
		t.Fatal("readonly turn requested mutation approval")
	}
	if calls.Load() != 4 {
		t.Fatal(calls.Load())
	}
}
func TestV2ProcessParserAndDemoMonitor(t *testing.T) {
	base := privateTestDir(t)
	os.Mkdir(filepath.Join(base, "123"), 0700)
	f := make([]string, 22)
	for i := range f {
		f[i] = "0"
	}
	f[0] = "S"
	f[11] = "40"
	f[12] = "2"
	f[19] = "999"
	f[21] = "100"
	os.WriteFile(filepath.Join(base, "123/stat"), []byte("123 (spectre worker) "+strings.Join(f, " ")), 0600)
	os.WriteFile(filepath.Join(base, "123/status"), []byte(fmt.Sprintf("Uid:\t%d\t%d\n", os.Getuid(), os.Getuid())), 0600)
	p, e := procUsage(base, 123)
	if e != nil || p.Start != "999" || p.ticks != 42 || !p.Own || !p.Simulation {
		t.Fatal(p, e)
	}
	a := appForTest(t, "demo")
	m := a.monitor()
	if len(m.Processes) != 3 || !strings.Contains(m.Source, "synthetic") {
		t.Fatal(m)
	}
	if e := a.terminate(ProcessIdentity{PID: os.Getpid(), Start: "fake"}); e == nil {
		t.Fatal("demo allowed real process")
	}
	if e := a.terminate(m.Processes[0].ProcessIdentity); e != nil {
		t.Fatal(e)
	}
	if len(a.monitor().Processes) != 2 {
		t.Fatal("demo task not removed")
	}
}

func TestV2LargeSearchResumeAndNestedOrder(t *testing.T) {
	a := appForTest(t, "demo")
	p := a.Store.Snapshot().Projects[0]
	os.Mkdir(filepath.Join(p.Root, "d"), 0700)
	for i := 0; i < 100; i++ {
		os.WriteFile(filepath.Join(p.Root, "d", fmt.Sprintf("%03d", i)), []byte("needle"), 0600)
	}
	os.WriteFile(filepath.Join(p.Root, "d.after"), []byte("needle"), 0600)
	f, e := os.Create(filepath.Join(p.Root, "huge.txt"))
	if e != nil {
		t.Fatal(e)
	}
	block := strings.Repeat("a", 65536)
	for i := 0; i < 520; i++ {
		f.WriteString(block)
	}
	f.WriteString("\nneedle beyond 32 MiB\n")
	f.Close()
	cursor := ""
	total := 0
	pages := 0
	for pages = 0; pages < 10; pages++ {
		v := actionTest(t, a, "search", map[string]any{"query": "needle", "cursor": cursor}).(map[string]any)
		total += len(v["hits"].([]any))
		if !v["more"].(bool) {
			break
		}
		cursor = v["cursor"].(string)
	}
	if total != 102 || pages < 2 {
		t.Fatal("search resume mismatch", total, pages)
	}
}

func TestV2DemoNeverContactsConfiguredModel(t *testing.T) {
	a := appForTest(t, "demo")
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		http.Error(w, "unexpected external call", 500)
	}))
	defer server.Close()
	a.Config.ModelURL = server.URL
	a.Config.Model = "must-not-call"
	a.Config.ModelTools = true
	a.StartValidation()
	if _, e := a.Chat(context.Background(), ChatInput{Session: "general", Text: "question"}); e != nil {
		t.Fatal(e)
	}
	actionTest(t, a, "generate-hdl", HDLInput{Language: "verilog", Requirements: "counter"})
	v, e := a.startAgent(AgentInput{Session: "project", Text: "files"})
	if e != nil {
		t.Fatal(e)
	}
	awaitAgent(t, a, v.(map[string]string)["id"], "completed")
	for i := 0; i < 100 && a.validationSnapshot().Model == "checking"; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if count.Load() != 0 {
		t.Fatal("demo contacted configured model")
	}
}
