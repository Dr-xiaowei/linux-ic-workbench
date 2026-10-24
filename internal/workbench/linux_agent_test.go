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
	"syscall"
	"testing"
	"time"
)

func linuxTestCall(t *testing.T, a *App, root, action string, x LinuxInput) (any, error) {
	t.Helper()
	return a.linuxAction(context.Background(), root, action, json.RawMessage(StringJSON(x)), &AgentRun{})
}
func TestLinuxFilesConflictAndBoundaries(t *testing.T) {
	a := appForTest(t, "production")
	root := a.Store.Snapshot().Projects[0].Root
	if _, e := linuxTestCall(t, a, root, "make-directory", LinuxInput{Path: "notes"}); e != nil {
		t.Fatal(e)
	}
	x := LinuxInput{Path: "notes/a.v", Content: "module a; endmodule\n", ExpectedSHA256: "new"}
	if _, e := linuxTestCall(t, a, root, "write-file", x); e != nil {
		t.Fatal(e)
	}
	if _, e := linuxTestCall(t, a, root, "write-file", x); e == nil {
		t.Fatal("replaced new file")
	}
	v, e := linuxTestCall(t, a, root, "read-file", LinuxInput{Path: x.Path})
	if e != nil {
		t.Fatal(e)
	}
	x.ExpectedSHA256 = v.(map[string]any)["sha256"].(string)
	x.Content = "module b; endmodule\n"
	if _, e = linuxTestCall(t, a, root, "write-file", x); e != nil {
		t.Fatal(e)
	}
	if _, e = linuxTestCall(t, a, root, "write-file", x); e == nil {
		t.Fatal("stale overwrite")
	}
	outside := privateTestDir(t)
	os.WriteFile(filepath.Join(outside, "private"), []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(root, "link"))
	for _, p := range []string{"../private", "/etc/passwd", "link/private", ".ssh/id_rsa"} {
		if _, e = linuxTestCall(t, a, root, "read-file", LinuxInput{Path: p}); e == nil {
			t.Fatal("allowed path", p)
		}
	}
	if _, e = linuxTestCall(t, a, root, "write-file", LinuxInput{Path: "huge", ExpectedSHA256: "new", Content: strings.Repeat("x", 120001)}); e == nil {
		t.Fatal("large write")
	}
	matches, _ := filepath.Glob(filepath.Join(root, "notes/.icw-edit-*"))
	if len(matches) != 0 {
		t.Fatal(matches)
	}
}
func TestLinuxCommandReceiptsEnvironmentAndBounds(t *testing.T) {
	a := appForTest(t, "production")
	root := a.Store.Snapshot().Projects[0].Root
	t.Setenv("ICW_TEST_SECRET", "credential-test-long-value")
	t.Setenv("DESIGN_PROCESS", "analog")
	v, e := linuxTestCall(t, a, root, "command", LinuxInput{Command: "printf 'stage-one\\n'; printf '%s' \"$DESIGN_PROCESS\"; test -z \"$ICW_TEST_SECRET\" || exit 99; printf 'credential-test-long-value'; exit 7"})
	r := v.(LinuxExecution)
	if e == nil || r.ExitCode == nil || *r.ExitCode != 7 || !strings.Contains(r.Output, "stage-one") || !strings.Contains(r.Output, "analog") || strings.Contains(r.Output, "credential-test-long-value") {
		t.Fatal(r, e)
	}
	v, e = linuxTestCall(t, a, root, "command", LinuxInput{Command: "yes x | head -c 80000"})
	r = v.(LinuxExecution)
	if e != nil || !r.Truncated || len(r.Output) > 65536 {
		t.Fatal(r, e)
	}
	for _, x := range []LinuxInput{{Command: "id", Timeout: 301}, {Command: "id", Directory: "../"}, {Command: ""}} {
		if _, e = linuxTestCall(t, a, root, "command", x); e == nil {
			t.Fatal(x)
		}
	}
}
func TestLinuxOutputSecretChunks(t *testing.T) {
	var output string
	b := linuxOutput{secrets: []string{"super-secret", ""}, update: func(s string, _ bool) { output = s }}
	b.Write([]byte("hello super-"))
	if strings.Contains(output, "super-") {
		t.Fatal(output)
	}
	b.Write([]byte("secret world"))
	if output != "hello [REDACTED] world" {
		t.Fatal(output)
	}
}
func TestLinuxCommandLiveCancellationAndTimeout(t *testing.T) {
	a := appForTest(t, "production")
	root := a.Store.Snapshot().Projects[0].Root
	r := &AgentRun{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, e := a.linuxCommand(ctx, root, LinuxInput{Command: "echo running; sleep 30 & echo $! > child.pid; wait"}, r)
		done <- e
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		a.mu.Lock()
		ready := r.Terminal != nil && strings.Contains(r.Terminal.Output, "running")
		a.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no live output")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Wait for child pid file so this verifies cancellation of a real child.
	for {
		if b, e := os.ReadFile(filepath.Join(root, "child.pid")); e == nil && len(b) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no child")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancel missing")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel stalled")
	}
	a.mu.Lock()
	result := clone(*r.Terminal)
	a.mu.Unlock()
	if result.Status != "cancelled" {
		t.Fatal(result)
	}
	if e := syscall.Kill(-result.PID, 0); e == nil { // zombies may await init reap; check no live process via shell
		v, _ := linuxTestCall(t, a, root, "command", LinuxInput{Command: fmt.Sprintf("ps -o stat= -g %d", result.PID)})
		for _, line := range strings.Fields(v.(LinuxExecution).Output) {
			if !strings.HasPrefix(line, "Z") {
				t.Fatal("live group", line)
			}
		}
	}
	start := time.Now()
	v, e := linuxTestCall(t, a, root, "command", LinuxInput{Command: "sleep 30", Timeout: 1})
	if e == nil || v.(LinuxExecution).Status != "cancelled" || time.Since(start) > 4*time.Second {
		t.Fatal(v, e)
	}
}

func linuxContractModel(t *testing.T, a *App, action, input string, advertised bool, repeat bool) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []modelMessage `json:"messages"`
			Tools    []any          `json:"tools"`
		}
		if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
			t.Error(e)
			return
		}
		if strings.Contains(StringJSON(req.Tools), `"name":"linux_command"`) != advertised {
			t.Error("tool advertisement mismatch")
		}
		n := calls.Add(1)
		msg := modelMessage{Role: "assistant", Content: "Reviewed the actual tool receipt."}
		if req.Messages[len(req.Messages)-1].Role != "tool" || repeat {
			tc := toolCall{ID: fmt.Sprint(n), Type: "function"}
			tc.Function.Name = "linux_" + strings.ReplaceAll(action, "-", "_")
			tc.Function.Arguments = input
			msg.ToolCalls = []toolCall{tc}
			msg.Content = "Inspect or execute the requested Linux operation."
			if repeat {
				for i := 1; i < 8; i++ {
					c := tc
					c.ID = fmt.Sprintf("%d-%d", n, i)
					msg.ToolCalls = append(msg.ToolCalls, c)
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": msg}}})
	}))
	t.Cleanup(srv.Close)
	a.Config.ModelURL = srv.URL
	a.Config.Model = "contract"
	a.Config.ModelTools = true
	return &calls
}
func startLinuxTest(t *testing.T, a *App, mode string) string {
	t.Helper()
	v, e := a.startAgent(AgentInput{Session: "project", Text: "perform Linux task", LinuxMode: mode})
	if e != nil {
		t.Fatal(e)
	}
	return v.(map[string]string)["id"]
}
func TestLinuxAgentModesApprovalPersistenceAndNoCarryover(t *testing.T) {
	for _, mode := range []string{"off", "confirm", "auto"} {
		t.Run(mode, func(t *testing.T) {
			a := appForTest(t, "production")
			root := a.Store.Snapshot().Projects[0].Root
			linuxContractModel(t, a, "command", `{"command":"printf verified > result.txt; cat result.txt"}`, mode != "off", false)
			id := startLinuxTest(t, a, mode)
			if mode == "confirm" {
				r := awaitAgent(t, a, id, "awaiting_approval")
				if r.Pending.Action != "linux:command" {
					t.Fatal(r)
				}
				if _, e := os.Stat(filepath.Join(root, "result.txt")); !os.IsNotExist(e) {
					t.Fatal("executed before approval")
				}
				if e := a.approveAgent(id, true); e != nil {
					t.Fatal(e)
				}
			}
			r := awaitAgent(t, a, id, "completed")
			b, e := os.ReadFile(filepath.Join(root, "result.txt"))
			if mode == "off" {
				if !os.IsNotExist(e) || r.Terminal != nil {
					t.Fatal("Linux tools executed when off")
				}
				return
			}
			if e != nil || string(b) != "verified" || r.Terminal == nil || *r.Terminal.ExitCode != 0 {
				t.Fatal(r, e)
			}
			s := a.Store.Snapshot()
			if s.Sessions[1].LastTask.ID != id || s.Sessions[1].LastTask.Terminal.Output != "verified" {
				t.Fatal("receipt not persisted")
			}
			if a.Store.Metadata().Sessions[1].LastTask.Terminal != nil {
				t.Fatal("metadata contains terminal bulk")
			}
			a.mu.Lock()
			delete(a.agentRuns, id)
			a.mu.Unlock()
			v, e := a.agentProgress(id)
			if e != nil || v.(AgentRun).Terminal.Output != "verified" {
				t.Fatal(v, e)
			}
		})
	}
}
func TestLinuxAgentDeclineCancelAndDefaultOff(t *testing.T) {
	a := appForTest(t, "production")
	linuxContractModel(t, a, "command", `{"command":"touch refused.txt"}`, true, false)
	id := startLinuxTest(t, a, "confirm")
	awaitAgent(t, a, id, "awaiting_approval")
	a.approveAgent(id, false)
	r := awaitAgent(t, a, id, "completed")
	if r.Terminal != nil {
		t.Fatal("declined executed")
	}
	id = startLinuxTest(t, a, "confirm")
	awaitAgent(t, a, id, "awaiting_approval")
	a.mu.Lock()
	cancel := a.cancels["agent:"+id]
	a.mu.Unlock()
	cancel()
	awaitAgent(t, a, id, "cancelled")
	if _, e := os.Stat(filepath.Join(a.Store.Snapshot().Projects[0].Root, "refused.txt")); !os.IsNotExist(e) {
		t.Fatal("file exists")
	}
	// Disable advertisement for a subsequent omitted mode; previous authorization must expire.
	a.Config.ModelTools = false
	if _, e := a.startAgent(AgentInput{Session: "general", Text: "hello", LinuxMode: "auto"}); e == nil {
		t.Fatal("enabled without tool support")
	}
	if _, e := a.startAgent(AgentInput{Session: "general", Text: "hello", LinuxMode: "invalid"}); e == nil {
		t.Fatal("invalid mode")
	}
}
func TestLinuxAgentToolBudgetAndRestart(t *testing.T) {
	a := appForTest(t, "production")
	calls := linuxContractModel(t, a, "inspect", "{}", true, true)
	id := startLinuxTest(t, a, "auto")
	r := awaitAgent(t, a, id, "failed")
	if calls.Load() != 3 || !strings.Contains(r.Error, "24-tool-call") {
		t.Fatal(r, calls.Load())
	}
	a.Store.Update(func(s *State) error {
		s.Sessions[1].LastTask = &AgentRun{ID: "interrupted-linux", Status: "running", LinuxMode: "auto", Terminal: &LinuxExecution{Status: "running"}}
		return nil
	})
	dir := a.Store.dir
	a.Close()
	st, e := OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	last := st.Snapshot().Sessions[1].LastTask
	if last.Status != "interrupted" || last.Terminal.Status != "interrupted" {
		t.Fatal(last)
	}
}
func TestLinuxDemoDoesNotExecuteHostCommandsOrWriteFiles(t *testing.T) {
	a := appForTest(t, "demo")
	root := privateTestDir(t)
	target := filepath.Join(root, "never-created")
	for action, input := range map[string]LinuxInput{"command": {Command: "touch " + target}, "write-file": {Path: "never-created", Content: "unsafe", ExpectedSHA256: "new"}, "make-directory": {Path: "never-created"}} {
		_, e := linuxTestCall(t, a, root, action, input)
		if e != nil {
			t.Fatal(e)
		}
	}
	if _, e := os.Stat(target); !os.IsNotExist(e) {
		t.Fatal("demo changed host")
	}
	id := startLinuxTest(t, a, "auto")
	r := awaitAgent(t, a, id, "completed")
	if r.Terminal == nil || !r.Terminal.Demo || r.Terminal.Status != "simulated" {
		t.Fatal(r)
	}
}

func TestLinuxPlainTextToolIsNotExecutedOrReportedAsSuccess(t *testing.T) {
	a := appForTest(t, "production")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": modelMessage{Role: "assistant", Content: "```json\n{\"name\":\"linux_action\",\"arguments\":{\"action\":\"command\",\"input\":\"touch never\"}}\n```"}}}})
	}))
	defer srv.Close()
	a.Config.ModelURL = srv.URL
	a.Config.Model = "text-only"
	a.Config.ModelTools = true
	r := awaitAgent(t, a, startLinuxTest(t, a, "auto"), "failed")
	if r.Terminal != nil || !strings.Contains(r.Error, "plain text") || !strings.Contains(r.Content, "No Linux operations executed") {
		t.Fatal(r)
	}
	if _, e := os.Stat(filepath.Join(a.Store.Snapshot().Projects[0].Root, "never")); !os.IsNotExist(e) {
		t.Fatal("text executed")
	}
}

func TestLinuxNativeObjectToolInput(t *testing.T) {
	var call AgentAction
	if e := decodeLinuxAction(`{"action":"write-file","input":{"path":"native.txt","content":"native\n","expectedSHA256":"new"}}`, &call); e != nil {
		t.Fatal(e)
	}
	a := appForTest(t, "production")
	root := a.Store.Snapshot().Projects[0].Root
	if _, e := a.linuxAction(context.Background(), root, call.Action, json.RawMessage(call.Input), &AgentRun{}); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(root, "native.txt"))
	if e != nil || string(b) != "native\n" {
		t.Fatal(string(b), e)
	}
	if e := decodeLinuxAction(`{"action":"command","input":{"command":"id","unknown":true}}`, &call); e == nil {
		t.Fatal("accepted unknown Linux input")
	}
}
