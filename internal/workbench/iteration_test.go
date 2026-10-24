package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskTemplateValidationAndBudget(t *testing.T) {
	a := appForTest(t, "demo")
	for _, v := range builtinTemplates() {
		if e := validateTemplate(v); e != nil {
			t.Fatal(e)
		}
	}
	for _, tcase := range []TaskTemplate{{Name: "bad", Prompt: "{{missing}}"}, {Name: "bad", Prompt: "{{x", Parameters: []TaskParameter{{Name: "x"}}}} {
		if validateTemplate(tcase) == nil {
			t.Fatal("invalid accepted")
		}
	}
	v, e := a.templateAction("preview-task-template", json.RawMessage(`{"id":"log-review","values":{"input":"{{focus}}"},"maxRounds":2,"maxSeconds":20}`))
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(v.(map[string]any)["text"].(string), "{{focus}}") {
		t.Fatal("recursive expansion")
	}
	if _, e = a.startAgent(AgentInput{Session: "general", Text: "hi", MaxRounds: 13}); e == nil {
		t.Fatal("budget accepted")
	}
	a.mu.Lock()
	a.mu.Unlock()
}
func TestMetricUnitsConditionsAndSpecifications(t *testing.T) {
	h := "metric,value,unit,corner,voltage,temperature,condition,min,max\n"
	in := map[string]string{"baseline": h + "bw,1,MHz,tt,1.8,25,ac,0.9,1.2\npower,2,mW,tt,1.8,25,dc,,\ngain,60,dB,tt,1.8,25,ac,55,\n", "candidate": h + "bw,1100,kHz,tt,1.8,25,ac,,\npower,1000,uW,ff,1.8,25,dc,,\ngain,62,dB,tt,1.8,25,ac,,\n"}
	v, e := compareUnitMetrics(json.RawMessage(StringJSON(in)))
	if e != nil {
		t.Fatal(e)
	}
	rows := v.(map[string]any)["rows"].([]MetricComparison)
	if rows[0].Status != "pass" || *rows[0].Delta != 100000 || rows[1].Status != "not-comparable" || rows[2].Percent != nil {
		t.Fatalf("bad comparison %+v", rows)
	}
	for _, bad := range []string{"x,NaN,V,tt,1,25,dc,,\n", "x,1,V,tt,,25,dc,,\n", "x,1,V,tt,1,25,dc,2,1\n", "x,1,V,tt,1,25,dc,,\nx,2,V,tt,1,25,dc,,\n"} {
		if _, e := parseUnitMetrics(h+bad, "x"); e == nil {
			t.Fatal("bad CSV accepted", bad)
		}
	}
}
func TestChangeSetProductionConflictUndoAndRestart(t *testing.T) {
	a := appForTest(t, "production")
	root, _ := filepath.EvalSymlinks(a.Store.Snapshot().Projects[0].Root)
	for _, p := range []string{"a.sv", "run.tcl"} {
		if e := os.WriteFile(filepath.Join(root, p), []byte("original\n"), 0640); e != nil {
			t.Fatal(e)
		}
		// Explicit fixture permissions: creation modes are filtered by the caller's umask.
		if e := os.Chmod(filepath.Join(root, p), 0640); e != nil {
			t.Fatal(e)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": `{"summary":"proposal only","files":[{"path":"a.sv","code":"revised\n"},{"path":"run.tcl","code":"updated\n"}]}`}}}})
	}))
	defer srv.Close()
	a.Config.ModelURL = srv.URL
	a.Config.Model = "fixture"
	v, e := a.proposeChangeSet(context.Background(), json.RawMessage(StringJSON(map[string]any{"root": root, "paths": []string{"a.sv", "run.tcl"}, "requirements": "test"})))
	if e != nil {
		t.Fatal(e)
	}
	c := v.(ChangeSet)
	if len(a.Store.Metadata().ChangeSets) != 0 {
		t.Fatal("polling includes source")
	}
	raw := json.RawMessage(StringJSON(map[string]string{"id": c.ID, "path": "a.sv"}))
	if _, e = a.changeSetAction(context.Background(), "apply-change-file", raw); e != nil {
		t.Fatal(e)
	}
	st, _ := os.Stat(filepath.Join(root, "a.sv"))
	if st.Mode().Perm() != 0640 {
		t.Fatal("mode changed")
	}
	b, _ := os.ReadFile(filepath.Join(root, "run.tcl"))
	if string(b) != "original\n" {
		t.Fatal("unselected file modified")
	}
	os.WriteFile(filepath.Join(root, "a.sv"), []byte("manual\n"), 0640)
	if _, e = a.changeSetAction(context.Background(), "undo-change-file", raw); e == nil {
		t.Fatal("overwrote manual change")
	}
	if got := a.Store.Snapshot().ChangeSets[0].Files[0].State; got != "applied" {
		t.Fatalf("conflict corrupted last known state: %s", got)
	}
	os.WriteFile(filepath.Join(root, "a.sv"), []byte("revised\n"), 0640)
	if _, e = a.changeSetAction(context.Background(), "undo-change-file", raw); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(filepath.Join(root, "a.sv"))
	if string(b) != "original\n" {
		t.Fatal("undo failed")
	}
	for _, path := range []string{"../outside", "/etc/passwd", "a.sv/../run.tcl"} {
		_, e = a.proposeChangeSet(context.Background(), json.RawMessage(StringJSON(map[string]any{"root": root, "paths": []string{path}, "requirements": "test"})))
		if e == nil {
			t.Fatal("bad path accepted")
		}
	}
	a.Store.mu.Lock()
	rawState, _ := json.Marshal(a.Store.data)
	a.Store.mu.Unlock()
	var s State
	if e = json.Unmarshal(rawState, &s); e != nil || len(s.ChangeSets) != 1 || s.ChangeSets[0].Files[0].Before != "original\n" {
		t.Fatal("persistence")
	}
}
func TestChangeSetDemoDoesNotTouchHost(t *testing.T) {
	a := appForTest(t, "demo")
	v, e := a.proposeChangeSet(context.Background(), json.RawMessage(`{"root":"/does-not-exist","paths":["a.sv","b.sv"],"requirements":"test"}`))
	if e != nil {
		t.Fatal(e)
	}
	c := v.(ChangeSet)
	_, e = a.changeSetAction(context.Background(), "apply-change-file", json.RawMessage(StringJSON(map[string]string{"id": c.ID, "path": "a.sv"})))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat("/does-not-exist/a.sv"); !os.IsNotExist(e) {
		t.Fatal("demo accessed host")
	}
}

func TestInvalidUTF8PreviewAndReadableDemo(t *testing.T) {
	a := appForTest(t, "production")
	p := a.Store.Snapshot().Projects[0]
	os.WriteFile(filepath.Join(p.Root, "legacy.log"), []byte{0xff, 0xfe, 0x41}, 0600)
	if _, e := a.browse("p", "", "legacy.log", 0); e == nil {
		t.Fatal("invalid UTF8 preview accepted")
	}
	if _, e := a.analyze(json.RawMessage(StringJSON(AnalysisInput{Project: "p", Path: "legacy.log"}))); e == nil {
		t.Fatal("invalid UTF8 analysis accepted")
	}
	text := demoActionSummary("browse", map[string]any{"entries": []any{map[string]any{"name": "amp.scs", "directory": false}}})
	if strings.Contains(text, `"entries"`) || !strings.Contains(text, "amp.scs") {
		t.Fatal(text)
	}
}

func TestChangeSetWriteFaultAndSymlinkPreserveFiles(t *testing.T) {
	a := appForTest(t, "production")
	root, _ := filepath.EvalSymlinks(a.Store.Snapshot().Projects[0].Root)
	path := filepath.Join(root, "a.py")
	os.WriteFile(path, []byte("old\n"), 0600)
	c := ChangeSet{ID: "review", Root: root, Files: []ChangeFile{{Path: "a.py", Before: "old\n", After: "new\n", SHA256: hashContent([]byte("old\n")), State: "pending"}}}
	a.Store.Update(func(s *State) error { s.ChangeSets = []ChangeSet{c}; return nil })
	original := a.Store.persist
	a.Store.persist = func(string, []byte) error { return errors.New("disk full fixture") }
	raw := json.RawMessage(`{"id":"review","path":"a.py"}`)
	if _, e := a.changeSetAction(context.Background(), "apply-change-file", raw); e == nil {
		t.Fatal("write proceeded despite failed intent persistence")
	}
	a.Store.persist = original
	b, _ := os.ReadFile(path)
	if string(b) != "old\n" {
		t.Fatal("file changed on failed persistence")
	}
	os.Rename(path, filepath.Join(root, "target.py"))
	os.Symlink("target.py", path)
	if _, e := a.changeSetAction(context.Background(), "apply-change-file", raw); e == nil {
		t.Fatal("symlink accepted")
	}
	if got := a.Store.Snapshot().ChangeSets[0].Files[0].State; got != "pending" {
		t.Fatalf("path error corrupted review state: %s", got)
	}
	b, _ = os.ReadFile(filepath.Join(root, "target.py"))
	if string(b) != "old\n" {
		t.Fatal("symlink target modified")
	}
}
func TestSchemaSevenUpgradePreservesMetadataAndDrafts(t *testing.T) {
	dir := privateTestDir(t)
	s, e := OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	e = s.Update(func(v *State) error {
		v.Projects = []Project{{ID: "keep", Name: "KEEP", Root: "/projects/keep", Notes: "用户备注"}}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	original := s.Snapshot()
	s.Close()
	original.Schema = 6
	raw := []byte(StringJSON(original))
	if e = os.WriteFile(filepath.Join(dir, "state.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	s, e = OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	got := s.Snapshot()
	if got.Schema != 7 || got.Projects[0].Notes != "用户备注" || got.Projects[0].Root != "/projects/keep" {
		t.Fatal("metadata lost")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "pre-migration-*.json"))
	if len(files) != 1 {
		t.Fatal("migration original missing")
	}
	before, _ := os.ReadFile(files[0])
	if string(before) != string(raw) {
		t.Fatal("migration original changed")
	}
}

func TestTemplateReadsOnlyExplicitInputAndTextCallsAreNotSuccess(t *testing.T) {
	a := appForTest(t, "production")
	root := a.Store.Snapshot().Projects[0].Root
	os.WriteFile(filepath.Join(root, "input.log"), []byte("missing nmos_missing\n"), 0600)
	in := AgentInput{TemplateID: "log-review", TemplateValues: map[string]string{"input": "input.log"}, MaxRounds: 4, MaxSeconds: 120, LinuxMode: "auto", AllowWrite: true}
	if e := a.prepareTemplate(&in, Session{Project: "p"}); e != nil {
		t.Fatal(e)
	}
	if !in.ReadOnly || in.AllowWrite || in.LinuxMode != "off" || !strings.Contains(in.Context, "nmos_missing") || !strings.Contains(in.Context, "SHA256") {
		t.Fatal("template evidence/policy", in)
	}
	in.TemplateValues["input"] = "../outside"
	if a.prepareTemplate(&in, Session{Project: "p"}) == nil {
		t.Fatal("traversal allowed")
	}
	if !textualToolRequest(`{"name":"platform_action","arguments":{"action":"browse"}}`) || textualToolRequest("A normal answer about platform_action") || textualToolRequest(`{"gain":60}`) {
		t.Fatal("tool-text classification")
	}
}

func TestMetricConversionRejectsUnrepresentableInputs(t *testing.T) {
	h := "metric,value,unit,corner,voltage,temperature,condition,min,max\n"
	for _, row := range []string{
		"bw,1,MHz,tt,1.8,25,ac,,1e308\n",
		"bw,1,MHz,tt,1.8,25,ac,-1e308,\n",
		"delay,5e-324,ps,tt,1.8,25,tran,,\n",
		"delay,1,ps,tt,1.8,25,tran,5e-324,\n",
	} {
		_, err := compareUnitMetrics(json.RawMessage(StringJSON(map[string]string{"baseline": h + row, "candidate": h + row})))
		if err == nil {
			t.Errorf("unrepresentable unit conversion accepted: %s", row)
		}
	}
	for _, bad := range []string{h + "x,1,V,tt,1,25,dc\x00,,\n", h + "x,1,V,tt,1,25,\xff,,\n"} {
		if _, err := parseUnitMetrics(bad, "bad.csv"); err == nil {
			t.Error("invalid text accepted")
		}
	}
}
