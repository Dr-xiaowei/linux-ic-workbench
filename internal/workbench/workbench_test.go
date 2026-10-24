package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ptr(v float64) *float64 { return &v }
func appForTest(t *testing.T, mode string) *App {
	t.Helper()
	c := Defaults()
	c.Mode = mode
	c.DataDir = privateTestDir(t)
	a, e := NewApp(c)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Close)
	if e = a.Store.Update(func(s *State) error {
		s.Projects = []Project{{ID: "p", Name: "Test", Root: privateTestDir(t)}}
		s.Sessions = []Session{{ID: "general", Title: "General"}, {ID: "project", Project: "p"}}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	return a
}
func awaitJob(t *testing.T, a *App, id string) Job {
	t.Helper()
	// Race-instrumented Cadence fixtures launch several child test processes.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		j, e := a.Job(id)
		if e != nil {
			t.Fatal(e)
		}
		if j.Status != "queued" && j.Status != "running" {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job timed out in test")
	return Job{}
}
func apiCall(a *App, path, body string, token bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "http://127.0.0.1"+path, strings.NewReader(body))
	if token {
		r.Header.Set("Authorization", "Bearer "+a.Token)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}
func TestCSVValidationAndEvidence(t *testing.T) {
	good := "point,corner,metric,value,unit,status,min,max\nac,TT,gain,65,dB,ok,60,80\nac,SS,gain,,dB,failed,60,80\nac,FF,gain,81,dB,ok,60,80\n"
	ms, e := ParseCSV(good, "source.csv")
	if e != nil {
		t.Fatal(e)
	}
	if Verdict(ms[0]) != "pass" || Verdict(ms[1]) != "unverified" || Verdict(ms[2]) != "fail" || ms[1].Source != "source.csv:3" {
		t.Fatal(ms)
	}
	for name, bad := range map[string]string{"missing-unit": strings.Replace(good, "65,dB", "65,", 1), "NaN": strings.Replace(good, "65,dB", "NaN,dB", 1), "missing-value": strings.Replace(good, "65,dB", ",dB", 1), "duplicate": good + "ac,TT,gain,65,dB,ok,60,80\n", "inverted": strings.Replace(good, "60,80", "90,80", 1), "unknown-status": strings.Replace(good, "ok", "done", 1)} {
		t.Run(name, func(t *testing.T) {
			if _, e := ParseCSV(bad, "file"); e == nil {
				t.Fatal("accepted invalid CSV")
			}
		})
	}
	ms[0].Name = "=HYPERLINK(1)"
	if !strings.Contains(ExportCSV(ms), "'=HYPERLINK") {
		t.Fatal("unsafe spreadsheet cell")
	}
	b := Batch{Project: "p", Definition: "v1", Metrics: ms}
	c := clone(b)
	c.Metrics[0].Unit = "V"
	diff, e := Compare(b, c)
	if e != nil || diff[2].State != "unit_mismatch" {
		t.Fatal(diff, e)
	}
	c.Definition = "v2"
	if _, e = Compare(b, c); e == nil {
		t.Fatal("definition mismatch accepted")
	}
	if !strings.Contains(Report(b), "Unverified: 1") {
		t.Fatal(Report(b))
	}
}
func TestStoreMigrationIsolationRecovery(t *testing.T) {
	dir := privateTestDir(t)
	old := State{Schema: 1, Mode: "production", Language: "en", Projects: []Project{{ID: "p", Notes: "保留", Tags: []string{"tag"}}}, Sessions: []Session{{ID: "s", Messages: []Message{{Role: "user", Content: "history"}}}}, Batches: []Batch{{ID: "b", Metrics: []Metric{{Name: "gain", Value: ptr(42)}}}}, Jobs: []Job{{ID: "j", Status: "running"}}, Experiments: []Experiment{{ID: "x", Status: "running"}}}
	raw := []byte(StringJSON(old))
	os.WriteFile(filepath.Join(dir, "state.json"), raw, 0600)
	s, e := OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	v := s.Snapshot()
	if v.Schema != Schema || v.Language != "en" || v.Projects[0].Notes != "保留" || v.Sessions[0].Messages[0].Content != "history" || v.Jobs[0].Status != "interrupted" || v.Experiments[0].Status != "paused" {
		t.Fatal(v)
	}
	copies, _ := filepath.Glob(filepath.Join(dir, "pre-migration-*.json"))
	b, _ := os.ReadFile(copies[0])
	if string(b) != string(raw) {
		t.Fatal("migration snapshot changed")
	}
	if x, e := OpenStore(dir, "production"); e == nil {
		x.Close()
		t.Fatal("concurrent store allowed")
	}
	if _, e := s.Backup(); e == nil {
		t.Fatal("cancelled backup feature remains enabled")
	}
	if e = s.Update(func(v *State) error { v.Projects[0].Notes = "wrong"; return errors.New("rollback") }); e == nil || s.Snapshot().Projects[0].Notes != "保留" {
		t.Fatal("transaction rollback failed")
	}
	s.Close()
	alias := filepath.Join(privateTestDir(t), "alias")
	os.Symlink(dir, alias)
	if x, e := OpenStore(alias, "demo"); e == nil {
		x.Close()
		t.Fatal("mode alias bypass")
	}
	s, e = OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	future := []byte(`{"schema":999,"mode":"production"}`)
	os.WriteFile(filepath.Join(dir, "state.json"), future, 0600)
	if _, e = OpenStore(dir, "production"); e == nil {
		t.Fatal("future schema accepted")
	}
	b, _ = os.ReadFile(filepath.Join(dir, "state.json"))
	if string(b) != string(future) {
		t.Fatal("future data overwritten")
	}
}
func TestPathsAndHTTPBoundary(t *testing.T) {
	a := appForTest(t, "production")
	p, _ := FindProject(a.Store.Snapshot(), "p")
	out := privateTestDir(t)
	os.WriteFile(filepath.Join(out, "secret"), []byte("private"), 0600)
	os.Symlink(out, filepath.Join(p.Root, "escape"))
	for _, path := range []string{"../../secret", "escape/secret"} {
		if _, e := SafePath(p.Root, path); e == nil {
			t.Fatal("escape accepted")
		}
	}
	for name, modify := range map[string]func(*http.Request){"auth": func(r *http.Request) { r.Header.Del("Authorization") }, "origin": func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }, "dns": func(r *http.Request) { r.Host = "evil.example" }} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://127.0.0.1/api/state", nil)
			r.Header.Set("Authorization", "Bearer "+a.Token)
			modify(r)
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, r)
			if w.Code == 200 {
				t.Fatal("accepted")
			}
		})
	}
	if w := apiCall(a, "/api/demo-seed", "{}", true); w.Code == 200 {
		t.Fatal("production seeded")
	}
	if w := apiCall(a, "/api/settings", `{"language":"en"} {}`, true); w.Code == 200 {
		t.Fatal("trailing JSON accepted")
	}
	if w := apiCall(a, "/api/settings", `{"language":"en"}`, true); w.Code != 200 || a.Store.Snapshot().Language != "en" {
		t.Fatal(w.Body.String())
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1/api/files?project=p", nil)
	r.Header.Set("Authorization", "Bearer "+a.Token)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}
func TestQueueIdempotencyCancellationAndSnapshot(t *testing.T) {
	a := appForTest(t, "demo")
	j, e := a.Submit(Job{Project: "p", Kind: "simulate", Key: "one", Params: map[string]float64{"R": 1000}})
	if e != nil {
		t.Fatal(e)
	}
	again, e := a.Submit(Job{Project: "p", Kind: "simulate", Key: "one", Params: map[string]float64{"R": 1000}})
	if e != nil || again.ID != j.ID {
		t.Fatal("duplicate submission")
	}
	if _, e = a.Submit(Job{Project: "p", Kind: "simulate", Key: "one", Params: map[string]float64{"R": 900}}); e == nil {
		t.Fatal("key conflict accepted")
	}
	if _, e = a.Submit(Job{Project: "p", Kind: "simulate", Params: map[string]float64{"R": 0}}); e == nil {
		t.Fatal("zero RC accepted")
	}
	j2, e := a.Submit(Job{Project: "p", Kind: "simulate"})
	if e != nil {
		t.Fatal(e)
	}
	a.Cancel(j2.ID)
	if got := awaitJob(t, a, j2.ID); got.Status != "cancelled" {
		t.Fatal(got)
	}
	a.Store.Update(func(s *State) error { s.Projects[0].Name = "changed"; return nil })
	if j.ProjectSnapshot.Name != "Test" {
		t.Fatal("project snapshot mutated")
	}
	done := awaitJob(t, a, j.ID)
	if done.Status != "completed" || done.Batch == "" {
		t.Fatal(done)
	}
	if len(a.Store.Snapshot().Batches) != 1 {
		t.Fatal("extra batches")
	}
	b := a.Store.Snapshot().Batches[0]
	if math.Abs(*b.Metrics[0].Value-159154.94309189534) > 1e-7 {
		t.Fatal("RC reference differs")
	}
}
func TestProcessTimeoutAndEnvironment(t *testing.T) {
	a := appForTest(t, "production")
	dir := privateTestDir(t)
	setup := filepath.Join(dir, "setup file.sh")
	os.WriteFile(setup, []byte("export ICW_TEST_SENTINEL=isolated\n"), 0600)
	output, e := a.command(context.Background(), Command{Executable: "/bin/sh", Args: []string{"-c", "printf '%s' \"$ICW_TEST_SENTINEL\""}}, Environment{Setup: setup, Shell: "/bin/sh"}, dir, nil)
	if e != nil || output != "isolated" || os.Getenv("ICW_TEST_SENTINEL") != "" {
		t.Fatal(output, e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, e = a.command(ctx, Command{Executable: "/bin/sh", Args: []string{"-c", "sleep 20 & wait"}}, Environment{}, dir, nil)
	if !errors.Is(e, context.DeadlineExceeded) || time.Since(started) > 3*time.Second {
		t.Fatal("process group timeout", e)
	}
}
func TestGenericAdapterContract(t *testing.T) {
	a := appForTest(t, "production")
	exe, _ := os.Executable()
	cmd := func(mode string) Command {
		return Command{Executable: exe, Args: []string{"-test.run=TestCommandHelper", "--", mode, "{workdir}"}}
	}
	a.Config.Probe = cmd("probe")
	a.Config.Prepare = cmd("prepare")
	a.Config.Simulate = cmd("simulate")
	a.Config.Export = cmd("export")
	j, e := a.Submit(Job{Project: "p", Kind: "probe"})
	if e != nil {
		t.Fatal(e)
	}
	done := awaitJob(t, a, j.ID)
	if done.Status != "completed" || !strings.Contains(done.Evidence, "Virtuoso") {
		t.Fatal(done)
	}
	j, e = a.Submit(Job{Project: "p", Kind: "simulate"})
	if e != nil {
		t.Fatal(e)
	}
	done = awaitJob(t, a, j.ID)
	if done.Status != "completed" || done.Batch == "" {
		t.Fatal(done)
	}
	a.Config.Export = cmd("badexport")
	j, _ = a.Submit(Job{Project: "p", Kind: "simulate"})
	if done = awaitJob(t, a, j.ID); done.Status != "failed" {
		t.Fatal("malformed metrics not rejected", done)
	}
}

// This subprocess fixture exercises orchestration contracts, not Cadence itself.
func TestCommandHelper(t *testing.T) {
	args := os.Args
	index := -1
	for i, v := range args {
		if v == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	mode, dir := args[index+1], args[index+2]
	write := func(name, text string) {
		if e := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); e != nil {
			os.Exit(3)
		}
	}
	switch mode {
	case "probe":
		write("identity.json", `{"tool":"Virtuoso","version":"CONTRACT-MOCK","library":"lib","cell":"cell","view":"schematic"}`)
	case "prepare":
		write("input.scs", "CONTRACT MOCK ONLY")
	case "simulate":
		if _, e := os.Stat(filepath.Join(dir, "input.scs")); e != nil {
			os.Exit(2)
		}
	case "export":
		write("metrics.csv", "point,corner,metric,value,unit,status\nac,TT,gain,65,dB,ok\n")
		write("result.json", `{"design":"fixture-v1","definition":"gain-v1"}`)
	case "badexport":
		write("metrics.csv", "point,corner,metric,value,unit,status\nac,TT,gain,NaN,dB,ok\n")
	}
	fmt.Println("CONTRACT-MOCK", mode)
	os.Exit(0)
}
func TestBoundedOptimization(t *testing.T) {
	a := appForTest(t, "demo")
	x, e := a.NewExperiment(Experiment{Project: "p", Name: "RC", Variables: []Variable{{Name: "R", Min: 900, Max: 1205, Step: 10, Initial: 1000}}, Objective: "bandwidth", Unit: "Hz", Direction: "max", Budget: 5, MaxSeconds: 10, Constraints: []Constraint{{Metric: "bandwidth", Unit: "Hz", Min: ptr(140000), Max: ptr(180000)}}})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 100; i++ {
		v := Candidate(x, i)["R"]
		if v < 900 || v > 1205 || math.Abs((v-900)/10-math.Round((v-900)/10)) > 1e-8 {
			t.Fatal("off bounds/grid", v)
		}
	}
	if e = a.ControlExperiment(x.ID, "resume"); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		x, _ = a.experiment(x.ID)
		if x.Status == "completed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if x.Status != "completed" || len(x.Trials) != 5 || !x.Revalidated || x.Best != x.Trials[4].Job || x.ElapsedSeconds <= 0 {
		t.Fatal(x)
	}
	if e = a.ControlExperiment(x.ID, "resume"); e == nil {
		t.Fatal("budget replay allowed")
	}
	if _, ok := Score(x, Batch{Metrics: []Metric{{Name: "bandwidth", Unit: "kHz", Status: "ok", Value: ptr(160)}}}); ok {
		t.Fatal("unit mismatch feasible")
	}
}
func TestOptimizationPauseResumeAndTimeBudget(t *testing.T) {
	a := appForTest(t, "demo")
	x, _ := a.NewExperiment(Experiment{Project: "p", Variables: []Variable{{Name: "R", Min: 900, Max: 1100, Initial: 1000}}, Objective: "bandwidth", Unit: "Hz", Direction: "max", Budget: 20, MaxSeconds: 1})
	a.ControlExperiment(x.ID, "resume")
	time.Sleep(100 * time.Millisecond)
	a.ControlExperiment(x.ID, "pause")
	time.Sleep(300 * time.Millisecond)
	x, _ = a.experiment(x.ID)
	// Pause finishes the current trial, then resume preserves spent budget and baseline.
	if x.Status != "paused" || len(x.Trials) != 1 {
		t.Fatal(x.Status)
	}
	if e := a.ControlExperiment(x.ID, "resume"); e != nil {
		t.Fatal(e)
	}
	time.Sleep(100 * time.Millisecond)
	a.ControlExperiment(x.ID, "stop")
	y, _ := a.NewExperiment(Experiment{Project: "p", Variables: x.Variables, Objective: x.Objective, Unit: x.Unit, Direction: "max", Budget: 20, MaxSeconds: 1})
	a.ControlExperiment(y.ID, "resume")
	time.Sleep(1300 * time.Millisecond)
	y, _ = a.experiment(y.ID)
	if y.Status != "stopped" || len(y.Trials) > 5 || y.ElapsedSeconds < 1 {
		t.Fatal(y)
	}
}
func TestModelChatEvidenceToolsAndErrors(t *testing.T) {
	a := appForTest(t, "production")
	calls := 0
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var p map[string]json.RawMessage
		json.NewDecoder(r.Body).Decode(&p)
		if calls == 1 {
			if _, ok := p["tools"]; ok {
				t.Error("plain question sent tools")
			}
			io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"A grounded answer"}}]}`)
		} else {
			w.WriteHeader(503)
		}
	}))
	defer model.Close()
	a.Config.ModelURL = model.URL
	a.Config.Model = "configured-glm"
	out, e := a.Chat(context.Background(), ChatInput{Session: "general", Text: "What is gain?"})
	if e != nil || out.Content != "A grounded answer" || out.Demo {
		t.Fatal(out, e)
	}
	if _, e = a.Chat(context.Background(), ChatInput{Session: "general", Text: "x", Batches: []string{"foreign"}}); e == nil {
		t.Fatal("plain chat attached project")
	}
	if _, e = a.Chat(context.Background(), ChatInput{Session: "general", Text: "next"}); e == nil {
		t.Fatal("model failure hidden")
	}
	if len(a.Store.Snapshot().Sessions[0].Messages) != 4 {
		t.Fatal("history not persisted")
	}
}
func TestModelCannotRunWithoutPermission(t *testing.T) {
	a := appForTest(t, "production")
	a.Config.ModelTools = true
	calls := 0
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			io.WriteString(w, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"a","type":"function","function":{"name":"submit_simulation","arguments":"{\"params\":{\"R\":1000}}"}}]}}]}`)
		} else {
			io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Recorded tool response"}}]}`)
		}
	}))
	defer model.Close()
	a.Config.ModelURL = model.URL
	a.Config.Model = "mock"
	_, e := a.Chat(context.Background(), ChatInput{Session: "project", Text: "analyze"})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Store.Snapshot().Jobs) != 0 {
		t.Fatal("unauthorized job")
	}
	calls = 0
	out, e := a.Chat(context.Background(), ChatInput{Session: "project", Text: "submit", AllowRun: true})
	if e != nil || len(out.Actions) != 1 || len(a.Store.Snapshot().Jobs) != 1 {
		t.Fatal(out, e)
	}
}
func TestMCPProtocolAndReadOnlyGate(t *testing.T) {
	a := appForTest(t, "demo")
	httpServer := httptest.NewServer(a.Handler())
	defer httpServer.Close()
	for _, allow := range []bool{false, true} {
		t.Run(fmt.Sprint(allow), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			server := MCPServer(httpServer.URL, a.Token, allow)
			client := mcp.NewClient(&mcp.Implementation{Name: "integration-test", Version: "1"}, nil)
			aTransport, bTransport := mcp.NewInMemoryTransports()
			ss, e := server.Connect(ctx, aTransport, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer ss.Close()
			cs, e := client.Connect(ctx, bTransport, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer cs.Close()
			list, e := cs.ListTools(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			expected := 3
			if allow {
				expected = 6
			}
			if len(list.Tools) != expected {
				t.Fatal(len(list.Tools))
			}
			res, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "workspace_state", Arguments: map[string]any{}})
			if e != nil || res.IsError {
				t.Fatal(res, e)
			}
			res, e = cs.CallTool(ctx, &mcp.CallToolParams{Name: "probe_virtuoso", Arguments: map[string]any{"project": "p"}})
			if allow && (e != nil || res.IsError) {
				t.Fatal(res, e)
			}
			if !allow && e == nil && !res.IsError {
				t.Fatal("readonly tool gate bypass")
			}
		})
	}
}
func TestCadenceGeneratedScriptsAndBounds(t *testing.T) {
	c := CadenceConfig{Library: "lib", Cell: "tb", View: "schematic", Variables: []Variable{{Name: "R", Min: 900, Max: 1100, Initial: 1000}}}
	p, e := cadenceParameters(c, map[string]float64{"R": 1050})
	if e != nil || p["R"] != 1050 {
		t.Fatal(e)
	}
	for _, v := range []map[string]float64{{"R": 800}, {"unknown": 1}} {
		if _, e = cadenceParameters(c, v); e == nil {
			t.Fatal("adapter accepted unapproved parameter")
		}
	}
	script := probeScript(c, "/tmp/space dir")
	if !strings.Contains(script, `"r")`) || strings.Contains(script, "dbSave(") {
		t.Fatal(script)
	}
	run := oceanScript(c, CadenceCase{Measurements: []CadenceMeasurement{{Expression: `value(v("/out" ?result 'ac) 1000)`}}}, p, "/tmp/results", "/tmp/setup.ocn")
	for _, needle := range []string{"createNetlist(", "run()", "desVar(\"R\" 1050)", "errset(", "FAILED"} {
		if !strings.Contains(run, needle) {
			t.Fatal("missing", needle)
		}
	}
}
func TestExportImportAPIWorkflow(t *testing.T) {
	a := appForTest(t, "demo")
	payload := map[string]any{"project": "p", "name": "PVT", "source": "fixture.csv", "design": "v1", "definition": "gain", "csv": "point,corner,metric,value,unit,status,min\nac,TT,gain,65,dB,ok,60\n"}
	w := apiCall(a, "/api/import", StringJSON(payload), true)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var b Batch
	json.Unmarshal(w.Body.Bytes(), &b)
	w = apiCall(a, "/api/export", StringJSON(map[string]string{"id": b.ID, "format": "csv"}), true)
	if !strings.Contains(w.Body.String(), "DEMO:") {
		t.Fatal("demo watermark absent")
	}
	w = apiCall(a, "/api/compare", StringJSON(map[string]string{"a": b.ID, "b": b.ID}), true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"delta":0`) {
		t.Fatal(w.Body.String())
	}
}
func TestDiagnosisEvidence(t *testing.T) {
	d := Diagnose("header\nERROR license checkout failed\nconvergence failed")
	if len(d) != 2 || !strings.Contains(d[0]["evidence"], "line 2") || d[1]["hypothesis"] == "" {
		t.Fatal(d)
	}
}
func TestStaticUIAssets(t *testing.T) {
	a := appForTest(t, "demo")
	for _, path := range []string{"/", "/app.js", "/style.css"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1"+path, nil)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatal(path, w.Code)
		}
	}
}

func TestNativeCadenceContractAndFrozenInputs(t *testing.T) {
	a := appForTest(t, "production")
	dir := privateTestDir(t)
	exe, _ := os.Executable()
	wrapper := func(name, mode string) string {
		p := filepath.Join(dir, name)
		s := "#!/bin/sh\nexec " + quote(exe) + " -test.run=TestNativeHelper -- " + mode + " \"$@\"\n"
		if e := os.WriteFile(p, []byte(s), 0700); e != nil {
			t.Fatal(e)
		}
		return p
	}
	setup := filepath.Join(dir, "setup.ocn")
	os.WriteFile(setup, []byte("; CONTRACT MOCK\nanalysis('ac ?start \"1k\" ?stop \"1M\" ?dec 10)\n"), 0600)
	c := CadenceConfig{Virtuoso: wrapper("virtuoso", "virt"), Ocean: wrapper("ocean", "ocean"), Spectre: wrapper("spectre", "version"), Library: "testLib", Cell: "testCell", View: "schematic", Design: "frozen-topology", Definition: "gain-v1", Variables: []Variable{{Name: "R", Min: 900, Max: 1100, Initial: 1000}}, Cases: []CadenceCase{{Name: "TT", Setup: setup, Measurements: []CadenceMeasurement{{Point: "ac", Name: "gain", Unit: "dB", Expression: "123.5", Min: ptr(100)}}}}}
	manifest := filepath.Join(dir, "cadence.json")
	os.WriteFile(manifest, []byte(StringJSON(c)), 0600)
	a.Store.Update(func(s *State) error { s.Projects[0].CadenceConfig = manifest; return nil })
	j, e := a.Submit(Job{Project: "p", Kind: "simulate", Params: map[string]float64{"R": 1000}})
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(setup, []byte("CHANGED AFTER SUBMISSION"), 0600)
	frozen, _, e := loadCadence(j.ProjectSnapshot.CadenceConfig)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(frozen.Cases[0].Setup)
	if strings.Contains(string(raw), "CHANGED") {
		t.Fatal("setup not frozen")
	}
	done := awaitJob(t, a, j.ID)
	if done.Status != "completed" || done.Batch == "" || !strings.Contains(done.Evidence, "CONTRACT-MOCK") {
		t.Fatal(done)
	}
	b := a.Store.Snapshot().Batches[0]
	if len(b.Metrics) != 1 || *b.Metrics[0].Value != 123.5 || Verdict(b.Metrics[0]) != "pass" || !strings.Contains(b.Definition, "@") {
		t.Fatal(b)
	}
	if _, e = a.Submit(Job{Project: "p", Kind: "simulate", Params: map[string]float64{"R": 2000}}); e == nil {
		t.Fatal("parameter envelope bypassed")
	}
}
func TestNativeHelper(t *testing.T) {
	args := os.Args
	at := -1
	for i, v := range args {
		if v == "--" {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	mode := args[at+1]
	if mode == "version" {
		fmt.Println("Spectre CONTRACT-MOCK 1.0")
		os.Exit(0)
	}
	script := ""
	for i, v := range args {
		if v == "-restore" && i+1 < len(args) {
			script = args[i+1]
		}
	}
	dir := filepath.Dir(script)
	if mode == "virt" {
		os.WriteFile(filepath.Join(dir, "identity.txt"), []byte("Virtuoso CONTRACT-MOCK\ntestLib\ntestCell\nschematic\n"), 0600)
	} else {
		net := filepath.Join(dir, "generated.scs")
		os.WriteFile(net, []byte("// MOCK generated netlist\n"), 0600)
		os.WriteFile(filepath.Join(dir, "netlist.path"), []byte(net+"\n"), 0600)
		os.WriteFile(filepath.Join(dir, "values.tsv"), []byte("0\t123.5\n"), 0600)
	}
	os.Exit(0)
}
func TestModelCancellationAndSelectedEvidence(t *testing.T) {
	a := appForTest(t, "production")
	seen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen <- string(b)
		<-r.Context().Done()
	}))
	defer server.Close()
	a.Config.ModelURL = server.URL
	a.Config.Model = "fake"
	b := demoBatch(nil)
	b.ID = "b"
	b.Project = "p"
	a.Store.Update(func(s *State) error { s.Batches = append(s.Batches, b); return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, e := a.Chat(ctx, ChatInput{Session: "project", Text: "inspect", Batches: []string{"b"}, Log: "LICENSE line"})
	if e == nil {
		t.Fatal("cancel ignored")
	}
	payload := <-seen
	if !strings.Contains(payload, "DEMO analytic") || !strings.Contains(payload, "LICENSE line") {
		t.Fatal("missing evidence")
	}
	s := a.Store.Snapshot().Sessions[1]
	if !strings.Contains(s.Messages[0].Evidence, "LICENSE line") {
		t.Fatal("evidence history missing")
	}
}
func TestPublicReferenceCSV(t *testing.T) {
	b, e := os.ReadFile("../../tests/fixtures/ngspice/analytic-reference.csv")
	if e != nil {
		t.Fatal(e)
	}
	ms, e := ParseCSV(string(b), "ngspice-topology / analytical reference")
	if e != nil || len(ms) != 82 {
		t.Fatal(len(ms), e)
	}
	for _, m := range ms {
		if Verdict(m) != "no_spec" {
			t.Fatal("reference invents specification")
		}
	}
}

func TestCSHEnvironmentIsolation(t *testing.T) {
	if _, e := os.Stat("/bin/csh"); e != nil {
		t.Skip("csh unavailable on test host")
	}
	a := appForTest(t, "production")
	dir := privateTestDir(t)
	setup := filepath.Join(dir, "company setup.csh")
	os.WriteFile(setup, []byte("setenv ICW_CSH_TEST isolated_csh\n"), 0600)
	out, e := a.command(context.Background(), Command{Executable: "/bin/sh", Args: []string{"-c", "printf '%s' \"$ICW_CSH_TEST\""}}, Environment{Setup: setup, Shell: "/bin/csh"}, dir, nil)
	if e != nil || out != "isolated_csh" || os.Getenv("ICW_CSH_TEST") != "" {
		t.Fatal(out, e)
	}
}
func TestMetadataBoundedAndRollbackOnIOFailure(t *testing.T) {
	a := appForTest(t, "demo")
	b := demoBatch(nil)
	b.ID = "large"
	b.Project = "p"
	b.Metrics = nil
	for i := 0; i < 10000; i++ {
		b.Metrics = append(b.Metrics, Metric{Point: fmt.Sprint(i), Corner: "TT", Name: "gain", Unit: "dB", Value: ptr(60), Status: "ok"})
	}
	if e := a.Store.Update(func(s *State) error { s.Batches = append(s.Batches, b); return nil }); e != nil {
		t.Fatal(e)
	}
	full := a.Store.Snapshot()
	meta := a.Store.Metadata()
	if len(meta.Batches[0].Metrics) != 0 || len(full.Batches[0].Metrics) != 10000 || len(StringJSON(meta)) > 10000 {
		t.Fatal("polling copied large metrics")
	}
	// Simulate an atomic rename failure without destroying the existing saved file.
	statePath := filepath.Join(a.Store.dir, "state.json")
	savedPath := statePath + ".saved"
	os.Rename(statePath, savedPath)
	os.Mkdir(statePath, 0700)
	e := a.Store.Update(func(s *State) error { s.Language = "en"; return nil })
	if e == nil || a.Store.Snapshot().Language != "zh" {
		t.Fatal("failed save changed in-memory state")
	}
	os.Remove(statePath)
	os.Rename(savedPath, statePath)
}

func TestInfeasibleOptimizationDoesNotClaimRevalidation(t *testing.T) {
	a := appForTest(t, "demo")
	x, e := a.NewExperiment(Experiment{Project: "p", Variables: []Variable{{Name: "R", Min: 1000, Max: 1100, Initial: 1000}}, Objective: "bandwidth", Unit: "Hz", Direction: "max", Constraints: []Constraint{{Metric: "bandwidth", Unit: "Hz", Min: ptr(1e9)}}, Budget: 2, MaxSeconds: 10})
	if e != nil {
		t.Fatal(e)
	}
	a.ControlExperiment(x.ID, "resume")
	time.Sleep(900 * time.Millisecond)
	x, _ = a.experiment(x.ID)
	if x.Status != "completed" || x.Revalidated || x.Best != "" || x.Error == "" {
		t.Fatal(x)
	}
}
