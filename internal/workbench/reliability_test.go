package workbench

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func privateTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPrivateStorageBoundaries(t *testing.T) {
	for _, kind := range []string{"directory-permissions", "directory-symlink", "state-permissions", "state-symlink", "state-hardlink", "lock-symlink", "workspace-symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := privateTestDir(t)
			outside := filepath.Join(privateTestDir(t), "original")
			original := []byte(`{"schema":6,"mode":"production","language":"en"}`)
			if err := os.WriteFile(outside, original, 0600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "directory-permissions":
				os.Chmod(dir, 0777)
			case "directory-symlink":
				alias := filepath.Join(privateTestDir(t), "alias")
				os.Symlink(dir, alias)
				dir = alias
			case "state-permissions":
				path := filepath.Join(dir, "state.json")
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
				// Deliberately insecure input, independent of the process umask.
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "state-symlink":
				os.Symlink(outside, filepath.Join(dir, "state.json"))
			case "state-hardlink":
				os.Link(outside, filepath.Join(dir, "state.json"))
			case "lock-symlink":
				os.Symlink(outside, filepath.Join(dir, ".lock"))
			case "workspace-symlink":
				os.Symlink(outside, filepath.Join(dir, "workspace.json.gz"))
			}
			if s, err := OpenStore(dir, "production"); err == nil {
				s.Close()
				t.Fatal("unsafe storage accepted")
			}
			got, _ := os.ReadFile(outside)
			if !bytes.Equal(got, original) {
				t.Fatal("outside file changed")
			}
		})
	}
}

func TestConfigConcurrentInitializationAndPreservation(t *testing.T) {
	dir := privateTestDir(t)
	path := filepath.Join(dir, "config.json")
	template := filepath.Join(privateTestDir(t), "template.json")
	original := []byte(`{"mode":"production","model":"personal-model","listen":"127.0.0.1:0"}`)
	os.WriteFile(template, original, 0644)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := LoadConfig(path, template)
			if e != nil || c.Model != "personal-model" {
				t.Errorf("concurrent init: %v %v", c, e)
			}
		}()
	}
	wg.Wait()
	os.WriteFile(template, []byte(`{"model":"new-default"}`), 0644)
	if _, e := LoadConfig(path, template); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("upgrade replaced personal config")
	}
	if e := PrivateFile(path); e != nil {
		t.Fatal(e)
	}
	bad := []byte(`{"model":"keep"} {"model":"discard"}`)
	os.WriteFile(path, bad, 0600)
	if _, e := LoadConfig(path, template); e == nil {
		t.Fatal("trailing configuration accepted")
	}
	got, _ = os.ReadFile(path)
	if !bytes.Equal(got, bad) {
		t.Fatal("corrupt config replaced")
	}
}

func TestStoreFailedWriteRollbackAndConcurrentEdits(t *testing.T) {
	dir := privateTestDir(t)
	s, e := OpenStore(dir, "production")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	w := s.Workspace()
	w.Data.Chat.Text = "saved"
	w, e = s.SaveWorkspace(w)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	wb, _ := os.ReadFile(filepath.Join(dir, "workspace.json.gz"))
	// Make the persistence parent unavailable while keeping the original files intact.
	away := dir + "-unavailable"
	if e = os.Rename(dir, away); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Rename(away, dir) })
	if e = s.Update(func(v *State) error { v.Language = "lost"; return nil }); e == nil {
		t.Fatal("write failure hidden")
	}
	w.Data.Chat.Text = "unsaved"
	if _, e = s.SaveWorkspace(w); e == nil {
		t.Fatal("workspace write failure hidden")
	}
	if s.Snapshot().Language != "zh" || s.Workspace().Data.Chat.Text != "saved" {
		t.Fatal("memory changed after failed write")
	}
	a, _ := os.ReadFile(filepath.Join(away, "state.json"))
	b, _ := os.ReadFile(filepath.Join(away, "workspace.json.gz"))
	if !bytes.Equal(a, before) || !bytes.Equal(b, wb) {
		t.Fatal("previous disk data changed")
	}
	if e = os.Rename(away, dir); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if e := s.Update(func(v *State) error { v.Projects = append(v.Projects, Project{ID: fmt.Sprint(n)}); return nil }); e != nil {
				t.Error(e)
			}
		}(i)
	}
	wg.Wait()
	if len(s.Snapshot().Projects) != 40 {
		t.Fatal("concurrent edit lost")
	}
	w = s.Workspace()
	w.Data.Chat.Text = "one-winner"
	var wins int32
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.SaveWorkspace(w)
			if e == nil {
				atomic.AddInt32(&wins, 1)
			} else if !errors.Is(e, errWorkspaceConflict) {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("workspace winners %d", wins)
	}
}

func TestCorruptWorkspacePreserved(t *testing.T) {
	for _, b := range [][]byte{[]byte("truncated-gzip"), {31, 139, 8, 0}} {
		dir := privateTestDir(t)
		p := filepath.Join(dir, "workspace.json.gz")
		os.WriteFile(p, b, 0600)
		if s, e := OpenStore(dir, "production"); e == nil {
			s.Close()
			t.Fatal("corrupt workspace accepted")
		}
		got, _ := os.ReadFile(p)
		if !bytes.Equal(got, b) {
			t.Fatal("corrupt file reset")
		}
	}
}

func TestDurabilityFailureStopsFurtherWrites(t *testing.T) {
	for _, workspace := range []bool{false, true} {
		t.Run(fmt.Sprint(workspace), func(t *testing.T) {
			dir := privateTestDir(t)
			s, e := OpenStore(dir, "production")
			if e != nil {
				t.Fatal(e)
			}
			s.persist = func(path string, b []byte) error {
				return atomicFileSync(path, b, func(string) error { return errors.New("injected directory fsync failure") })
			}
			if workspace {
				w := s.Workspace()
				w.Data.Chat.Text = "committed"
				_, e = s.SaveWorkspace(w)
			} else {
				e = s.Update(func(v *State) error { v.Language = "en"; return nil })
			}
			var uncertain *durabilityError
			if !errors.As(e, &uncertain) {
				t.Fatal("uncertain commit hidden", e)
			}
			if e = s.Update(func(v *State) error { v.Language = "lost"; return nil }); e == nil {
				t.Fatal("write continued after storage fault")
			}
			if _, e = s.SaveWorkspace(s.Workspace()); e == nil {
				t.Fatal("workspace write continued")
			}
			s.Close()
			s, e = OpenStore(dir, "production")
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			if workspace && s.Workspace().Data.Chat.Text != "committed" || !workspace && s.Snapshot().Language != "en" {
				t.Fatal("committed data disagrees after restart")
			}
		})
	}
}

func TestDynamicListenerAndAuthenticatedStatus(t *testing.T) {
	ln, e := ListenLoopback("auto", false)
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	if other, e := ListenLoopback(ln.Addr().String(), false); e == nil {
		other.Close()
		t.Fatal("explicit occupied port accepted")
	}
	other, e := ListenLoopback(ln.Addr().String(), true)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	if other.Addr().String() == ln.Addr().String() {
		t.Fatal("port collision")
	}
	if bad, e := ListenLoopback("0.0.0.0:0", true); e == nil {
		bad.Close()
		t.Fatal("public listener allowed")
	}
	a := appForTest(t, "production")
	server := httptest.NewServer(a.Handler())
	defer server.Close()
	if _, e := a.PublishInstance(strings.TrimPrefix(server.URL, "http://")); e != nil {
		t.Fatal(e)
	}
	if _, e := RunningInstance(a.Store.dir, "production"); e != nil {
		t.Fatal(e)
	}
	otherApp := appForTest(t, "production")
	otherServer := httptest.NewServer(otherApp.Handler())
	defer otherServer.Close()
	if _, e := a.PublishInstance(strings.TrimPrefix(otherServer.URL, "http://")); e != nil {
		t.Fatal(e)
	}
	if _, e := RunningInstance(a.Store.dir, "production"); e == nil {
		t.Fatal("other instance token accepted")
	}
	a.RemoveInstance()
	if _, e := RunningInstance(a.Store.dir, "production"); e == nil {
		t.Fatal("missing instance reported running")
	}
}

func TestShutdownCancelsStartupValidation(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan struct{})
	release := make(chan struct{})
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
			close(finished)
		case <-release:
		}
	}))
	defer model.Close()
	defer close(release)
	c := Defaults()
	c.DataDir = privateTestDir(t)
	c.ModelURL = model.URL
	c.Model = "fixture"
	a, e := NewApp(c)
	if e != nil {
		t.Fatal(e)
	}
	a.StartValidation()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("validation did not start")
	}
	t0 := time.Now()
	a.Close()
	if time.Since(t0) > 2*time.Second {
		t.Fatal("shutdown blocked on validation")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("model request not cancelled")
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1/api/state", nil).WithContext(context.Background())
	r.Header.Set("Authorization", "Bearer "+a.Token)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal("request admitted after shutdown", w.Code)
	}
}
