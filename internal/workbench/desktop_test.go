package workbench

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSilentMCPHandshakeDeadline(t *testing.T) {
	a := appForTest(t, "production")
	c := EDAConnection{ID: "silent", Name: "Silent test subprocess", Transport: "stdio", Command: Command{Executable: "/bin/sleep", Args: []string{"30"}}, Directory: t.TempDir(), Timeout: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if e := a.connectEDA(ctx, c, a.edaEntry(c.ID)); e == nil {
		t.Fatal("silent MCP initialized")
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("unbounded MCP handshake")
	}
}

func TestPlatformNativeObjectAndLegacyInput(t *testing.T) {
	a := appForTest(t, "production")
	for _, raw := range []string{`{"action":"calculate","input":{"expression":"R*C","variables":"R=2k\nC=5n","angle":"rad"}}`, `{"action":"calculate","input":"{\"expression\":\"R*C\",\"variables\":\"R=2k\\nC=5n\",\"angle\":\"rad\"}"}`} {
		var call AgentAction
		if err := decodePlatformAction(raw, &call); err != nil {
			t.Fatal(err)
		}
		v, err := a.action(nil, call.Action, json.RawMessage(call.Input))
		if err != nil {
			t.Fatal(err)
		}
		if v.(map[string]any)["value"] != float64(1e-5) {
			t.Fatal(v)
		}
	}
	for _, raw := range []string{`{"action":"calculate","input":null}`, `{"action":"calculate","input":[]}`, `{"action":"calculate","input":2}`, `{"action":"calculate"}`} {
		if decodePlatformAction(raw, &AgentAction{}) == nil {
			t.Fatal("accepted invalid input", raw)
		}
	}
}

func TestDesktopAPIHasNoWebInterface(t *testing.T) {
	a := appForTest(t, "production")
	a.Config.DesktopAPI = true
	for _, path := range []string{"/", "/index.html", "/app.js", "/style.css"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1"+path, nil)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatalf("desktop serves %s: %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1/api/state", nil)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("desktop API lost authentication")
	}
	r.Header.Set("Authorization", "Bearer "+a.Token)
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	a.Config.DesktopAPI = false
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if w.Code != 200 {
		t.Fatal("web Demo renderer unavailable")
	}
}

func TestLinuxDedicatedFields(t *testing.T) {
	for _, action := range []string{"inspect", "command", "read-file", "write-file", "make-directory"} {
		if validateLinuxFields(action, json.RawMessage(`{"unknown":"x"}`)) == nil {
			t.Fatal(action)
		}
	}
	if validateLinuxFields("inspect", json.RawMessage(`{"command":"pwd"}`)) == nil {
		t.Fatal("inspect silently discarded command")
	}
	if validateLinuxFields("command", json.RawMessage(`{"command":"pwd"}`)) != nil {
		t.Fatal("command rejected")
	}
	if len(linuxToolDefinitions()) != 5 {
		t.Fatal("missing dedicated tools")
	}
	for _, raw := range []string{`null`, `[]`, `"text"`} {
		if validateLinuxFields("inspect", json.RawMessage(raw)) == nil {
			t.Fatal("non-object accepted", raw)
		}
	}
}
