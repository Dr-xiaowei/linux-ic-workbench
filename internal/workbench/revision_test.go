package workbench

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCalculatorEngineeringAndErrors(t *testing.T) {
	for _, tt := range []struct {
		expr, vars, angle string
		want              float64
	}{{"1/(2*pi*R*C)", "C=1n\nR=1k", "rad", 159154.94309189535}, {"-2^2 + 2^3^2", "", "rad", 508}, {"2^-2", "", "rad", .25}, {"sin(30)+cos(60)", "", "deg", 1}, {"gain", "gain=20*log10(b/a)\na=10m\nb=1", "rad", 40}, {"sqrt(4*kB*T*R*BW)", "T=300;R=10k;BW=20k", "rad", math.Sqrt(4 * 1.380649e-23 * 300 * 1e4 * 2e4)}, {"1M + 2meg + 1e6", "", "rad", 4e6}, {"5%2 + min(2,3) + abs(-2)", "", "rad", 5}} {
		v, e := calculate(json.RawMessage(StringJSON(CalculatorInput{tt.expr, tt.vars, tt.angle})))
		if e != nil {
			t.Fatal(tt, e)
		}
		got := v.(map[string]any)["value"].(float64)
		if math.Abs(got-tt.want) > 1e-10*math.Max(1, math.Abs(tt.want)) {
			t.Fatalf("%s = %g, want %g", tt.expr, got, tt.want)
		}
	}
	for _, tt := range []CalculatorInput{{Expression: "1/0"}, {Expression: "sqrt(-1)"}, {Expression: "process.exit()"}, {Expression: "2pi"}, {Expression: "pow(2)"}, {Expression: "1e999"}, {Expression: "1", Variables: "a=b\nb=a"}, {Expression: "1", Variables: "pi=3"}, {Expression: "missing"}, {Expression: strings.Repeat("(", 100) + "1" + strings.Repeat(")", 100)}} {
		if _, e := calculate(json.RawMessage(StringJSON(tt))); e == nil {
			t.Fatalf("accepted invalid input %#v", tt)
		}
	}
}
func TestCodeLanguagesAnalysisAndScope(t *testing.T) {
	a := appForTest(t, "demo")
	p := a.Store.Metadata().Projects[0]
	for _, l := range CodeLanguages {
		v := actionTest(t, a, "generate-code", map[string]any{"language": l.ID, "requirements": "RC filter or digital test"}).(map[string]any)
		if v["extension"] != l.Extension || !strings.Contains(v["code"].(string), "DEMO") {
			t.Fatal(l.ID, v)
		}
		name := l.ID + l.Extension
		actionTest(t, a, "save-code", HDLInput{Language: l.ID, Code: v["code"].(string), Project: p.ID, Path: name})
		if _, e := a.saveHDL(json.RawMessage(StringJSON(HDLInput{Language: l.ID, Code: "x", Project: p.ID, Path: name}))); e == nil {
			t.Fatal("overwrote existing code")
		}
	}
	before := len(a.Store.Metadata().Sessions)
	v := actionTest(t, a, "analyze-ai", map[string]any{"name": "spectre.log", "content": "hello\nERROR: convergence failure\n", "question": "分析", "limited": false}).(map[string]any)
	if !strings.Contains(v["content"].(string), "convergence") || len(a.Store.Metadata().Sessions) != before {
		t.Fatal(v)
	}
	folder := privateTestDir(t)
	os.WriteFile(filepath.Join(folder, "only.txt"), []byte("folder-needle"), 0600)
	outside := filepath.Join(privateTestDir(t), "escape.txt")
	os.WriteFile(outside, []byte("secret"), 0600)
	os.Symlink(outside, filepath.Join(folder, "escape"))
	scope := Session{Folder: folder}
	v2, e := a.scopedAgentAction(context.Background(), scope, "browse", json.RawMessage(`{"path":"only.txt"}`))
	if e != nil || !strings.Contains(StringJSON(v2), "folder-needle") {
		t.Fatal(v2, e)
	}
	for _, path := range []string{"../escape", "escape"} {
		if _, e := a.scopedAgentAction(context.Background(), scope, "browse", json.RawMessage(StringJSON(map[string]string{"path": path}))); e == nil {
			t.Fatal("scope escape", path)
		}
	}
	v2, e = a.scopedAgentAction(context.Background(), scope, "search", json.RawMessage(`{"query":"folder-needle"}`))
	if e != nil || len(v2.(map[string]any)["hits"].([]any)) != 1 {
		t.Fatal(v2, e)
	}
	if _, e := a.scopedAgentAction(context.Background(), Session{Project: p.ID}, "browse", json.RawMessage(`{"project":"another"}`)); e == nil {
		t.Fatal("project scope ignored")
	}
	actionTest(t, a, "save-preferences", func() Preferences {
		x := a.preferences()
		x.Compilers = map[string]Command{"python": {Executable: "/usr/bin/false", Args: []string{"{file}"}}}
		return x
	}())
	if _, e := a.compileHDL(json.RawMessage(`{"language":"shell","code":"echo hi"}`)); e == nil {
		t.Fatal("wrong language compiler used")
	}
}
func TestEDAMCPBuiltinAndHTTP(t *testing.T) {
	for _, mode := range []string{"demo", "production"} {
		t.Run(mode, func(t *testing.T) {
			a := appForTest(t, mode)
			list := actionTest(t, a, "mcp-tools", map[string]any{"id": "builtin"}).(map[string]any)
			if len(list["tools"].([]*mcp.Tool)) != 5 {
				t.Fatal(list)
			}
			// A second request must reuse a live session after the first request's context ends.
			list = actionTest(t, a, "mcp-tools", map[string]any{"id": "builtin"}).(map[string]any)
			if !list["connected"].(bool) {
				t.Fatal(list)
			}
			op := actionTest(t, a, "mcp-call", map[string]any{"id": "builtin", "tool": "list_projects", "arguments": map[string]any{}}).(Operation)
			done := awaitOperation(t, a, op.ID)
			if done.Status != "succeeded" || !strings.Contains(done.Log, "tools/call") {
				t.Fatal(done)
			}
			actionTest(t, a, "disconnect-mcp", map[string]any{"id": "builtin"})
		})
	}
	a := appForTest(t, "production")
	server := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "protocol contract fixture"}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Value string `json:"value"`
	}) (*mcp.CallToolResult, map[string]string, error) {
		return nil, map[string]string{"echo": in.Value}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "slow", Description: "timeout fixture"}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, map[string]string, error) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return nil, map[string]string{"done": "true"}, nil
		}
	})
	host := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil))
	defer host.Close()
	c := EDAConnection{ID: "fixture", Name: "HTTP fixture", Transport: "http", URL: host.URL, Timeout: 1, AllowedTools: []string{"echo", "slow"}}
	actionTest(t, a, "save-mcp", c)
	actionTest(t, a, "mcp-tools", map[string]any{"id": c.ID})
	op := actionTest(t, a, "mcp-call", map[string]any{"id": c.ID, "tool": "echo", "arguments": map[string]any{"value": "contract-result"}}).(Operation)
	done := awaitOperation(t, a, op.ID)
	if done.Status != "succeeded" || !strings.Contains(done.Log, "contract-result") {
		t.Fatal(done)
	}
	op = actionTest(t, a, "mcp-call", map[string]any{"id": c.ID, "tool": "disallowed", "arguments": map[string]any{}}).(Operation)
	if done = awaitOperation(t, a, op.ID); done.Status != "failed" {
		t.Fatal(done)
	}
	op = actionTest(t, a, "mcp-call", map[string]any{"id": c.ID, "tool": "slow", "arguments": map[string]any{}}).(Operation)
	if done = awaitOperation(t, a, op.ID); done.Status != "cancelled" {
		t.Fatal(done)
	}
	op = actionTest(t, a, "mcp-call", map[string]any{"id": c.ID, "tool": "echo", "arguments": map[string]any{"value": "reconnected"}}).(Operation)
	if done = awaitOperation(t, a, op.ID); done.Status != "succeeded" {
		t.Fatal(done)
	}
	actionTest(t, a, "remove-mcp", map[string]any{"id": c.ID})
}

func TestEDAStdioFixtureProcess(t *testing.T) {
	if os.Getenv("ICW_MCP_FIXTURE") != "1" {
		return
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fixture-stdio", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo"}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Value string `json:"value"`
	}) (*mcp.CallToolResult, map[string]string, error) {
		return nil, map[string]string{"value": in.Value}, nil
	})
	if e := server.Run(context.Background(), &mcp.StdioTransport{}); e != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
func TestEDAStdioAndSchema4Persistence(t *testing.T) {
	t.Setenv("ICW_MCP_FIXTURE", "1")
	a := appForTest(t, "production")
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	c := EDAConnection{ID: "stdio-test", Name: "stdio fixture", Transport: "stdio", Command: Command{Executable: exe, Args: []string{"-test.run=^TestEDAStdioFixtureProcess$"}}, Directory: privateTestDir(t), Timeout: 10, AllowedTools: []string{"echo"}}
	actionTest(t, a, "save-mcp", c)
	actionTest(t, a, "mcp-tools", map[string]any{"id": c.ID})
	op := actionTest(t, a, "mcp-call", map[string]any{"id": c.ID, "tool": "echo", "arguments": map[string]any{"value": "stdio-real-protocol"}}).(Operation)
	if done := awaitOperation(t, a, op.ID); done.Status != "succeeded" || !strings.Contains(done.Log, "stdio-real-protocol") {
		t.Fatal(done)
	}
	folder := privateTestDir(t)
	res := apiCall(a, "/api/sessions", StringJSON(map[string]string{"title": "folder chat", "folder": folder}), true)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	state := a.Store.Snapshot()
	state.Schema = 3
	dir := privateTestDir(t)
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(StringJSON(state)), 0600)
	store, e := OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	upgraded := store.Snapshot()
	if upgraded.Schema != Schema || upgraded.MCPConnections[0].ID != c.ID || upgraded.Sessions[len(upgraded.Sessions)-1].Folder != state.Sessions[len(state.Sessions)-1].Folder || len(upgraded.Operations) != len(state.Operations) {
		t.Fatal(upgraded)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "pre-migration-*.json"))
	if len(backups) != 1 {
		t.Fatal("migration backup missing")
	}
}

func TestInlineAnalysisAndCodeUseConfiguredModel(t *testing.T) {
	a := appForTest(t, "production")
	requests := []string{}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, string(body))
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Fixture model answer"}}]}`)
	}))
	defer model.Close()
	a.Config.ModelURL = model.URL
	a.Config.Model = "contract-fixture"
	got := actionTest(t, a, "analyze-ai", map[string]any{"name": "log", "content": "Warning: test fixture", "question": "Explain", "limited": true})
	if !strings.Contains(StringJSON(got), "Fixture model answer") || !strings.Contains(requests[0], "untrusted_file") || strings.Contains(requests[0], `"tools"`) {
		t.Fatal(got, requests)
	}
	got = actionTest(t, a, "generate-code", map[string]any{"language": "verilog-a", "requirements": "unity-gain buffer"})
	if !strings.Contains(StringJSON(got), "Fixture model answer") || !strings.Contains(requests[1], "verilog-a") {
		t.Fatal(got, requests)
	}
	if len(a.Store.Metadata().Sessions) != 2 {
		t.Fatal("inline analysis created a conversation")
	}
}
