package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspacePersistenceUpgradeConflictAndBackupDisabled(t *testing.T) {
	dir := privateTestDir(t)
	old := State{Schema: 4, Mode: "production", Language: "en", Projects: []Project{{ID: "p", Notes: "keep", Tags: []string{"tag"}}}, Preferences: Preferences{Configured: true, Model: "intranet", ModelURL: "http://localhost/glm", KeyEnv: "KEY"}, FileNotes: []FileNote{{Project: "p", Path: "a.v", Notes: "keep"}}, Software: []Software{{Name: "Spectre"}}, Sessions: []Session{{ID: "chat", Messages: []Message{{Role: "user", Content: "retain"}}}}, MCPConnections: []EDAConnection{{ID: "mcp", Name: "retain"}}}
	raw, _ := json.MarshalIndent(old, "", "  ")
	os.WriteFile(filepath.Join(dir, "state.json"), raw, 0600)
	s, e := OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	want := old
	want.Schema = Schema
	if StringJSON(s.Snapshot()) != StringJSON(want) {
		t.Fatal("migration lost data")
	}
	w := s.Workspace()
	w.Data.CodeLanguage = "verilog"
	source := strings.Repeat("wire sample; // fixture\n", 10000)
	w.Data.CodeDrafts = map[string]CodeDraft{"verilog": {Code: source, Original: source, Name: "original.v", Proposal: source + "// proposal\n", Requirements: "Optimize"}}
	w.Data.Calculator = json.RawMessage(`{"expression":"R*C","variables":"R=1k;C=1n","angle":"rad","result":{"formatted":"0.000001","expression":"R*C","variables":{"R":1000,"C":1e-9}}}`)
	w.Data.Analyses = []SavedAnalysis{{ID: "a", Name: "log", Text: "ERROR fixture", Answer: "Evidence and hypothesis"}}
	w.Data.Chat.Text = "unsent question"
	w.Data.Chat.Scope = "folder"
	w.Data.Chat.Folder = "/tmp"
	saved, e := s.SaveWorkspace(w)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveWorkspace(w); e != errWorkspaceConflict {
		t.Fatal("stale revision accepted", e)
	}
	same, e := s.SaveWorkspace(saved)
	if e != nil || same.Revision != saved.Revision {
		t.Fatal("unchanged save created revision")
	}
	original, _ := os.ReadFile(filepath.Join(dir, "workspace.json.gz"))
	uncompressed, _ := json.Marshal(saved)
	if len(original) >= len(uncompressed)/10 {
		t.Fatal("compression ineffective")
	}
	info, _ := os.Stat(filepath.Join(dir, "workspace.json.gz"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("permissions", info.Mode())
	}
	if _, e := s.Backup(); e == nil {
		t.Fatal("backup feature remains enabled")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "backup-*"))
	if len(files) != 0 {
		t.Fatal("backup created", files)
	}
	// Older JSON backups are untouched; no history garbage collection during upgrade.
	os.WriteFile(filepath.Join(dir, "backup-old.json"), []byte("legacy"), 0600)
	bad := saved
	bad.Data.CodeDrafts = map[string]CodeDraft{"verilog": {Code: strings.Repeat("x", (1<<20)+1)}}
	if _, e = s.SaveWorkspace(bad); e == nil {
		t.Fatal("oversized code accepted")
	}
	current, _ := os.ReadFile(filepath.Join(dir, "workspace.json.gz"))
	if !bytes.Equal(current, original) {
		t.Fatal("failed write damaged data")
	}
	s.Close()
	s, e = OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if StringJSON(s.Workspace()) != StringJSON(saved) || StringJSON(s.Snapshot()) != StringJSON(want) {
		t.Fatal("restart data lost")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "backup-old.json"))
	if string(b) != "legacy" {
		t.Fatal("legacy backup removed")
	}
	usage, e := s.StorageUsage()
	if e != nil || usage.(map[string]any)["bytes"].(int64) <= 0 {
		t.Fatal(usage, e)
	}
	t.Logf("workspace fixture raw=%d gzip=%d reduction=%.2f%%", len(uncompressed), len(original), 100*(1-float64(len(original))/float64(len(uncompressed))))
}
func TestWorkspaceCorruptionCapacityAndAtomicFailure(t *testing.T) {
	dir := privateTestDir(t)
	s, e := OpenStore(dir, "demo")
	if e != nil {
		t.Fatal(e)
	}
	w := s.Workspace()
	w.Data.CodeDrafts = map[string]CodeDraft{"verilog": {Code: "module demo; endmodule\n"}}
	saved, e := s.SaveWorkspace(w)
	if e != nil {
		t.Fatal(e)
	}
	huge := clone(saved)
	huge.Data.Analyses = make([]SavedAnalysis, 7)
	for i := range huge.Data.Analyses {
		huge.Data.Analyses[i].Text = strings.Repeat("a", 2<<20)
	}
	if _, e = s.SaveWorkspace(huge); e == nil {
		t.Fatal("total size limit ignored")
	}
	p := filepath.Join(dir, "workspace.json.gz")
	b, _ := os.ReadFile(p)
	os.Rename(p, p+".kept")
	os.Mkdir(p, 0700)
	changed := clone(saved)
	changed.Data.Chat.Text = "new"
	if _, e = s.SaveWorkspace(changed); e == nil {
		t.Fatal("directory write succeeded")
	}
	if StringJSON(s.Workspace()) != StringJSON(saved) {
		t.Fatal("failed save changed memory")
	}
	os.Remove(p)
	os.WriteFile(p, []byte("corrupt"), 0600)
	s.Close()
	if other, e := OpenStore(dir, "demo"); e == nil {
		other.Close()
		t.Fatal("corrupt store accepted")
	}
	raw, _ := os.ReadFile(p)
	if string(raw) != "corrupt" {
		t.Fatal("corrupt source overwritten")
	}
	os.WriteFile(p, b, 0600)
	s, e = OpenStore(dir, "demo")
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
}
func TestWorkspaceHTTPConflictAndAuthorization(t *testing.T) {
	a := appForTest(t, "demo")
	h := a.Handler()
	request := func(method, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1/api/workspace", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := a.Store.Workspace()
	w.Data.Chat.Text = "persist"
	raw := StringJSON(w)
	if r := request("POST", a.Token, raw); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := request("POST", a.Token, raw); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := request("POST", a.Token, raw+" {}"); r.Code != 400 {
		t.Fatal("trailing JSON accepted")
	}
	if r := request("GET", a.Token, ""); r.Code != 200 || !strings.Contains(r.Body.String(), "persist") {
		t.Fatal(r.Body.String())
	}
}
func TestImproveCodeModelContractAndOriginalPreservation(t *testing.T) {
	a := appForTest(t, "production")
	original := "module original(input clk); endmodule\n"
	before := StringJSON(a.Store.Snapshot())
	calls := 0
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []modelMessage `json:"messages"`
			Tools    any            `json:"tools"`
		}
		if json.Unmarshal(body, &req) != nil || req.Tools != nil || len(req.Messages) != 2 {
			t.Error("unexpected model request")
		}
		var input map[string]string
		json.Unmarshal([]byte(req.Messages[1].Content), &input)
		if input["source"] != original || input["requirements"] != "Add reset input" {
			t.Error("source or requirements lost")
		}
		content := `{"code":"module original(input clk, input rst_n); endmodule\n","summary":"Reset input added; logic still needs design review."}`
		if calls == 2 {
			content = "invalid JSON"
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}}})
	}))
	defer model.Close()
	a.Config.ModelURL = model.URL
	a.Config.Model = "contract-fixture"
	input := json.RawMessage(StringJSON(HDLInput{Language: "verilog", Code: original, Requirements: "Add reset input"}))
	got, e := a.improveCode(context.Background(), input)
	if e != nil || !strings.Contains(StringJSON(got), "rst_n") || got.(map[string]any)["validated"] != false {
		t.Fatal(got, e)
	}
	if _, e = a.improveCode(context.Background(), input); e == nil {
		t.Fatal("malformed model output accepted")
	}
	if StringJSON(a.Store.Snapshot()) != before {
		t.Fatal("AI modified personal state or source")
	}
	d := appForTest(t, "demo")
	got, e = d.improveCode(context.Background(), json.RawMessage(StringJSON(HDLInput{Language: "python", Code: "print(1)  \r\n", Requirements: "Simplify"})))
	if e != nil || got.(map[string]any)["code"] != "print(1)\n" || got.(map[string]any)["demo"] != true {
		t.Fatal(got, e)
	}
	for _, in := range []HDLInput{{Language: "bad", Code: "code", Requirements: "req"}, {Language: "verilog", Code: strings.Repeat("a", 120001), Requirements: "req"}, {Language: "verilog", Code: "code"}} {
		if _, e = d.improveCode(context.Background(), json.RawMessage(StringJSON(in))); e == nil {
			t.Fatal("invalid input accepted")
		}
	}
}
