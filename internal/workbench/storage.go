package workbench

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const workspaceLimit = 12 << 20

var errWorkspaceConflict = errors.New("草稿已被其他页面更新；请先导出本页草稿，再重新加载 / Draft conflict: export this page's drafts, then reload")

type CodeDraft struct {
	ReceivedRun  string `json:"receivedRun,omitempty"`
	Run          string `json:"run,omitempty"`
	Code         string `json:"code"`
	Requirements string `json:"requirements"`
	Name         string `json:"name,omitempty"`
	Original     string `json:"original,omitempty"`
	Proposal     string `json:"proposal,omitempty"`
	Base         string `json:"base,omitempty"`
	Summary      string `json:"summary,omitempty"`
	Undo         string `json:"undo,omitempty"`
}
type CodeArchive struct {
	ID       string    `json:"id"`
	Language string    `json:"language"`
	Draft    CodeDraft `json:"draft"`
}
type SavedAnalysis struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Limited  bool   `json:"limited"`
}
type EDADraft struct {
	Text    string `json:"text"`
	Session string `json:"session"`
}
type WorkspaceData struct {
	Engineering  json.RawMessage      `json:"engineering,omitempty"`
	EDA          map[string]EDADraft  `json:"eda,omitempty"`
	CodeLanguage string               `json:"codeLanguage"`
	CodeDrafts   map[string]CodeDraft `json:"codeDrafts"`
	CodeArchive  []CodeArchive        `json:"codeArchive"`
	Calculator   json.RawMessage      `json:"calculator"`
	Analyses     []SavedAnalysis      `json:"analyses"`
	AnalysisID   string               `json:"analysisID"`
	Chat         struct {
		Text    string `json:"text"`
		Scope   string `json:"scope"`
		Project string `json:"project"`
		Folder  string `json:"folder"`
		Session string `json:"session"`
	} `json:"chat"`
}
type Workspace struct {
	Schema   int           `json:"schema"`
	Revision uint64        `json:"revision"`
	Data     WorkspaceData `json:"data"`
}

func gzipBytes(raw []byte) ([]byte, error) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, e := w.Write(raw); e != nil {
		return nil, e
	}
	if e := w.Close(); e != nil {
		return nil, e
	}
	return b.Bytes(), nil
}
func readGzip(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	r, e := gzip.NewReader(io.LimitReader(f, limit+1))
	if e != nil {
		return nil, e
	}
	defer r.Close()
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errors.New("uncompressed data capacity exceeded")
	}
	return b, nil
}
func (s *Store) loadWorkspace() error {
	s.workspace = Workspace{Schema: 1}
	b, e := readGzip(filepath.Join(s.dir, "workspace.json.gz"), workspaceLimit)
	if os.IsNotExist(e) {
		return nil
	}
	if e == nil {
		e = json.Unmarshal(b, &s.workspace)
	}
	if e == nil && s.workspace.Schema != 1 {
		e = errors.New("unsupported workspace schema")
	}
	if e != nil {
		return fmt.Errorf("workspace unreadable; original preserved: %w", e)
	}
	return validateWorkspace(s.workspace.Data)
}
func validateWorkspace(d WorkspaceData) error {
	if len(d.Engineering) > 0 && string(d.Engineering) != "null" {
		var obj map[string]json.RawMessage
		if json.Unmarshal(d.Engineering, &obj) != nil || obj == nil {
			return errors.New("engineering drafts must be an object")
		}
	}
	if len(d.Engineering) > 2<<20 {
		return errors.New("engineering drafts exceed 2 MiB")
	}
	if len(d.EDA) > 21 {
		return errors.New("too many EDA drafts")
	}
	for _, x := range d.EDA {
		if len(x.Text) > 20000 || len(x.Session) > 100 {
			return errors.New("EDA draft too large")
		}
	}
	if len(d.CodeDrafts) > len(CodeLanguages) || len(d.CodeArchive) > 200 || len(d.Analyses) > 200 {
		return errors.New("draft capacity reached; export before replacing records")
	}
	valid := func(lang string, x CodeDraft) error {
		if hdlExtension(lang) == "" {
			return errors.New("unknown code language")
		}
		for _, v := range []string{x.Code, x.Original, x.Proposal, x.Base, x.Undo} {
			if len(v) > 1<<20 {
				return errors.New("code exceeds 1 MiB")
			}
		}
		if len(x.Requirements) > 20000 || len(x.Name) > 500 || len(x.Summary) > 50000 {
			return errors.New("code metadata capacity exceeded")
		}
		return nil
	}
	for k, v := range d.CodeDrafts {
		if e := valid(k, v); e != nil {
			return e
		}
	}
	for _, v := range d.CodeArchive {
		if e := valid(v.Language, v.Draft); e != nil {
			return e
		}
	}
	for _, v := range d.Analyses {
		if len(v.Text) > 2<<20 || len(v.Answer) > 1<<20 || len(v.Question) > 10000 || len(v.Name) > 500 {
			return errors.New("analysis capacity exceeded")
		}
	}
	if len(d.Calculator) > 100000 || len(d.Chat.Text) > 100000 || len(d.Chat.Folder) > 4096 {
		return errors.New("workspace field too large")
	}
	return nil
}
func (s *Store) Workspace() Workspace { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.workspace) }
func (s *Store) SaveWorkspace(in Workspace) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeFault != nil {
		return Workspace{}, s.writeFault
	}
	if in.Revision != s.workspace.Revision {
		return Workspace{}, errWorkspaceConflict
	}
	if in.Schema != 1 {
		return Workspace{}, errors.New("unsupported workspace schema")
	}
	if e := validateWorkspace(in.Data); e != nil {
		return Workspace{}, e
	}
	old, _ := json.Marshal(s.workspace.Data)
	next, e := json.Marshal(in.Data)
	if e != nil {
		return Workspace{}, e
	}
	if bytes.Equal(old, next) {
		return clone(s.workspace), nil
	}
	in.Revision++
	b, e := json.Marshal(in)
	if e != nil {
		return Workspace{}, e
	}
	if len(b) > workspaceLimit {
		return Workspace{}, errors.New("工作草稿超过 12 MiB，请导出并整理；已有数据未覆盖 / Drafts exceed 12 MiB; existing data preserved")
	}
	compressed, e := gzipBytes(b)
	if e != nil {
		return Workspace{}, e
	}
	if e = s.persist(filepath.Join(s.dir, "workspace.json.gz"), compressed); e != nil {
		var uncertain *durabilityError
		if errors.As(e, &uncertain) {
			s.workspace = clone(in)
			s.writeFault = e
		}
		return Workspace{}, e
	}
	s.workspace = clone(in)
	return clone(in), nil
}
func (a *App) workspaceAPI(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/workspace" {
		return false
	}
	if r.Method == "GET" {
		reply(w, a.Store.Workspace(), nil)
		return true
	}
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, workspaceLimit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var in Workspace
	e := d.Decode(&in)
	if e == nil {
		var extra any
		if d.Decode(&extra) != io.EOF {
			e = errors.New("single JSON object required")
		}
	}
	var out Workspace
	if e == nil {
		out, e = a.Store.SaveWorkspace(in)
	}
	if errors.Is(e, errWorkspaceConflict) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
		return true
	}
	// Return only the new revision; never echo several MiB on every autosave.
	reply(w, map[string]any{"revision": out.Revision}, e)
	return true
}
func (s *Store) StorageUsage() (any, error) {
	categories := map[string]int64{"metadata": 0, "drafts": 0, "backups": 0, "other": 0}
	files := 0
	limited := false
	unreadable := 0
	deadline := time.Now().Add(2 * time.Second)
	e := filepath.WalkDir(s.dir, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			unreadable++
			return nil
		}
		if strings.HasPrefix(d.Name(), "._") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		files++
		if files > 20000 || time.Now().After(deadline) {
			limited = true
			return fs.SkipAll
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			unreadable++
			return nil
		}
		k := "other"
		if d.Name() == "state.json" {
			k = "metadata"
		} else if d.Name() == "workspace.json.gz" {
			k = "drafts"
		} else if strings.HasPrefix(d.Name(), "backup-") || strings.HasPrefix(d.Name(), "pre-migration-") {
			k = "backups"
		}
		categories[k] += info.Size()
		return nil
	})
	if e != nil {
		return nil, e
	}
	total := int64(0)
	for _, v := range categories {
		total += v
	}
	return map[string]any{"directory": s.dir, "bytes": total, "categories": categories, "limited": limited, "unreadable": unreadable, "workspaceLimit": workspaceLimit, "stateLimit": 64 << 20}, nil
}
