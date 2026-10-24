package workbench

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Version = "0.8.0-rc3"
const Schema = 7

type Project struct {
	Description   string   `json:"description"`
	Paths         []string `json:"paths"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Root          string   `json:"root"`
	Notes         string   `json:"notes"`
	Tags          []string `json:"tags"`
	Environment   string   `json:"environment"`
	CadenceConfig string   `json:"cadenceConfig"`
}
type Environment struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
	Setup      string   `json:"setup"`
	Shell      string   `json:"shell"`
	ToolPath   string   `json:"toolPath,omitempty"`
}
type Message struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Time     string `json:"time"`
	Evidence string `json:"evidence,omitempty"`
}
type Session struct {
	EDAConnection string    `json:"edaConnection,omitempty"`
	LastTask      *AgentRun `json:"lastTask,omitempty"`
	Folder        string    `json:"folder,omitempty"`
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Project       string    `json:"project"`
	Messages      []Message `json:"messages"`
}
type Metric struct {
	Point  string   `json:"point"`
	Corner string   `json:"corner"`
	Name   string   `json:"name"`
	Value  *float64 `json:"value"`
	Unit   string   `json:"unit"`
	Status string   `json:"status"`
	Min    *float64 `json:"min,omitempty"`
	Max    *float64 `json:"max,omitempty"`
	Source string   `json:"source"`
}
type Batch struct {
	ID         string   `json:"id"`
	Project    string   `json:"project"`
	Name       string   `json:"name"`
	Design     string   `json:"design"`
	Definition string   `json:"definition"`
	Created    string   `json:"created"`
	Metrics    []Metric `json:"metrics"`
	Provenance string   `json:"provenance"`
	Parent     string   `json:"parent"`
}
type Job struct {
	ID                  string             `json:"id"`
	Project             string             `json:"project"`
	Kind                string             `json:"kind"`
	Status              string             `json:"status"`
	Created             string             `json:"created"`
	Finished            string             `json:"finished"`
	Params              map[string]float64 `json:"params"`
	Parent              string             `json:"parent"`
	Key                 string             `json:"key"`
	Batch               string             `json:"batch"`
	Error               string             `json:"error"`
	Log                 string             `json:"log"`
	Environment         string             `json:"environment"`
	Evidence            string             `json:"evidence"`
	ProjectSnapshot     *Project           `json:"projectSnapshot,omitempty"`
	EnvironmentSnapshot *Environment       `json:"environmentSnapshot,omitempty"`
}
type Variable struct {
	Name    string  `json:"name"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Step    float64 `json:"step"`
	Initial float64 `json:"initial"`
}
type Constraint struct {
	Metric string   `json:"metric"`
	Unit   string   `json:"unit"`
	Min    *float64 `json:"min,omitempty"`
	Max    *float64 `json:"max,omitempty"`
}
type Trial struct {
	Job      string             `json:"job"`
	Params   map[string]float64 `json:"params"`
	Value    *float64           `json:"value"`
	Feasible bool               `json:"feasible"`
	Status   string             `json:"status"`
}
type Experiment struct {
	ID              string       `json:"id"`
	Project         string       `json:"project"`
	Name            string       `json:"name"`
	Variables       []Variable   `json:"variables"`
	Objective       string       `json:"objective"`
	Unit            string       `json:"unit"`
	Direction       string       `json:"direction"`
	Constraints     []Constraint `json:"constraints"`
	Budget          int          `json:"budget"`
	Status          string       `json:"status"`
	Trials          []Trial      `json:"trials"`
	Best            string       `json:"best"`
	Error           string       `json:"error"`
	Environment     string       `json:"environment"`
	CurrentJob      string       `json:"currentJob"`
	MaxSeconds      int          `json:"maxSeconds"`
	Created         string       `json:"created"`
	ElapsedSeconds  float64      `json:"elapsedSeconds"`
	Revalidated     bool         `json:"revalidated"`
	MeasurementKeys []string     `json:"measurementKeys"`
	Design          string       `json:"design"`
	Definition      string       `json:"definition"`
}
type State struct {
	TaskTemplates  []TaskTemplate  `json:"taskTemplates,omitempty"`
	ChangeSets     []ChangeSet     `json:"changeSets,omitempty"`
	MCPConnections []EDAConnection `json:"mcpConnections"`
	FileNotes      []FileNote      `json:"fileNotes"`
	Software       []Software      `json:"software"`
	Plugins        []Plugin        `json:"plugins"`
	Preferences    Preferences     `json:"preferences"`
	Operations     []Operation     `json:"operations"`
	Schema         int             `json:"schema"`
	Mode           string          `json:"mode"`
	Language       string          `json:"language"`
	Projects       []Project       `json:"projects"`
	Environments   []Environment   `json:"environments"`
	Sessions       []Session       `json:"sessions"`
	Batches        []Batch         `json:"batches"`
	Jobs           []Job           `json:"jobs"`
	Experiments    []Experiment    `json:"experiments"`
}
type Command struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}
type Config struct {
	DesktopAPI   bool              `json:"-"`
	Listen       string            `json:"listen"`
	Mode         string            `json:"mode"`
	DataDir      string            `json:"dataDir"`
	ModelURL     string            `json:"modelUrl"`
	Model        string            `json:"model"`
	KeyEnv       string            `json:"keyEnv"`
	Timeout      int               `json:"timeoutSeconds"`
	MaxWorkers   int               `json:"maxWorkers"`
	MaxPreview   int64             `json:"maxPreviewBytes"`
	MaxRecords   int               `json:"maxRecords"`
	Probe        Command           `json:"probe"`
	Prepare      Command           `json:"prepare"`
	Simulate     Command           `json:"simulate"`
	Export       Command           `json:"export"`
	ModelHeaders map[string]string `json:"modelHeaders"`
	ModelTools   bool              `json:"modelTools"`
	ModelTimeout int               `json:"modelTimeoutSeconds"`
}

func Defaults() Config {
	return Config{Listen: "127.0.0.1:0", Mode: "production", Timeout: 120, ModelTimeout: 120, MaxWorkers: 1, MaxPreview: 262144, MaxRecords: 10000, KeyEnv: "ICW_MODEL_KEY"}
}
func ID() string {
	b := make([]byte, 12)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Now() string             { return time.Now().UTC().Format(time.RFC3339) }
func StringJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func clone[T any](v T) T      { var out T; b, _ := json.Marshal(v); json.Unmarshal(b, &out); return out }

type Store struct {
	mu         sync.Mutex
	dir        string
	data       State
	lock       *os.File
	workspace  Workspace
	persist    func(string, []byte) error
	writeFault error
}

var ErrStoreInUse = errors.New("data directory already in use")

func OpenStore(dir, mode string) (*Store, error) {
	if mode != "demo" && mode != "production" {
		return nil, errors.New("mode must be demo or production")
	}
	abs, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	if mode == "production" {
		abs, e = PrivateDir(abs, true)
	} else {
		e = os.MkdirAll(abs, 0700)
		if e == nil {
			abs, e = filepath.EvalSymlinks(abs)
		}
	}
	if e != nil {
		return nil, e
	}
	var lock *os.File
	if mode == "production" {
		lock, e = privateLock(filepath.Join(abs, ".lock"))
	} else {
		lock, e = os.OpenFile(filepath.Join(abs, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	}
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		lock.Close()
		if errors.Is(e, syscall.EWOULDBLOCK) {
			return nil, ErrStoreInUse
		}
		return nil, fmt.Errorf("cannot lock data directory: %w", e)
	}
	s := &Store{dir: abs, lock: lock, persist: atomicFile}
	fail := func(e error) (*Store, error) { s.Close(); return nil, e }
	if mode == "production" {
		for _, name := range []string{"MODE", "state.json", "workspace.json.gz", ".instance.json"} {
			if e = PrivateFile(filepath.Join(abs, name)); e != nil && !os.IsNotExist(e) {
				return fail(e)
			}
		}
	}
	marker := filepath.Join(abs, "MODE")
	b, e := os.ReadFile(marker)
	if e == nil && strings.TrimSpace(string(b)) != mode {
		return fail(errors.New("demo/production data directory mismatch"))
	}
	if e != nil && !os.IsNotExist(e) {
		return fail(e)
	}
	raw, e := readBounded(filepath.Join(abs, "state.json"), 64<<20)
	if e == nil {
		if len(raw) > 64<<20 {
			return fail(errors.New("state exceeds 64 MiB limit"))
		}
		if e = json.Unmarshal(raw, &s.data); e != nil {
			return fail(fmt.Errorf("data unreadable; original preserved: %w", e))
		}
		if s.data.Mode != mode {
			return fail(errors.New("state mode mismatch"))
		}
		if s.data.Schema > Schema || s.data.Schema < 1 {
			return fail(errors.New("unsupported schema; data left intact"))
		}
		if s.data.Schema < Schema {
			if e = atomicFile(filepath.Join(abs, "pre-migration-"+ID()+".json"), raw); e != nil {
				return fail(e)
			}
			s.data.Schema = Schema
			if s.data.Language == "" {
				s.data.Language = "zh"
			}
		}
	} else if os.IsNotExist(e) {
		s.data = State{Schema: Schema, Mode: mode, Language: "zh"}
	} else {
		return fail(e)
	}
	if e = s.loadWorkspace(); e != nil {
		return fail(e)
	}
	// Incomplete writes left by SIGKILL are never source data. Clean only our
	// own private temporary files, after validation and while holding the lock.
	if mode == "production" {
		orphans, err := filepath.Glob(filepath.Join(abs, ".atomic-*"))
		if err != nil {
			return fail(err)
		}
		for _, p := range orphans {
			if PrivateFile(p) == nil {
				if e = os.Remove(p); e != nil {
					return fail(e)
				}
			}
		}
	}
	if e = atomicFile(marker, []byte(mode+"\n")); e != nil {
		return fail(e)
	}
	for i := range s.data.Sessions {
		t := s.data.Sessions[i].LastTask
		if t != nil && (t.Status == "running" || t.Status == "awaiting_approval") {
			t.Status = "interrupted"
			t.Error = "Service restarted; inspect task receipts and processes before retry"
			t.Pending = nil
			if t.Terminal != nil && t.Terminal.Status == "running" {
				t.Terminal.Status = "interrupted"
			}
		}
	}
	for i := range s.data.Operations {
		if s.data.Operations[i].Status == "running" || s.data.Operations[i].Status == "queued" {
			s.data.Operations[i].Status = "interrupted"
			s.data.Operations[i].Error = "Server restarted; inspect process before retry"
		}
	}
	for i := range s.data.Jobs {
		j := &s.data.Jobs[i]
		if j.Status == "running" || j.Status == "queued" {
			j.Status = "interrupted"
			j.Error = "Process restarted: previous outcome unknown; inspect before retry"
		}
	}
	for i := range s.data.Experiments {
		x := &s.data.Experiments[i]
		if x.Status == "running" {
			x.Status = "paused"
			x.Error = "Restarted; review last job before explicit resume"
		}
	}
	if e = s.save(); e != nil {
		return fail(e)
	}
	return s, nil
}

type durabilityError struct{ err error }

func (e *durabilityError) Error() string {
	return "file replaced but directory sync failed; further writes disabled, check storage and restart: " + e.err.Error()
}
func (e *durabilityError) Unwrap() error { return e.err }

func syncDirectory(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func atomicFile(path string, b []byte) error { return atomicFileSync(path, b, syncDirectory) }
func atomicFileSync(path string, b []byte, syncDir func(string) error) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".atomic-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(tmp, path); e != nil {
		return e
	}
	if e = syncDir(filepath.Dir(path)); e != nil {
		return &durabilityError{e}
	}
	return nil
}
func (s *Store) save() error {
	b, e := json.Marshal(s.data)
	if e != nil {
		return e
	}
	if len(b) > 64<<20 {
		return errors.New("state capacity reached; export/archive before adding data")
	}
	return s.persist(filepath.Join(s.dir, "state.json"), b)
}
func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeFault != nil {
		return s.writeFault
	}
	old := clone(s.data)
	if e := fn(&s.data); e != nil {
		s.data = old
		return e
	}
	if e := s.save(); e != nil {
		var uncertain *durabilityError
		if errors.As(e, &uncertain) {
			s.writeFault = e
		} else {
			s.data = old
		}
		return e
	}
	return nil
}
func (s *Store) Snapshot() State { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.data) }

// Progress polling copies only the requested receipt, not all stored code and files.
func (s *Store) Operation(id string) (Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, op := range s.data.Operations {
		if op.ID == id {
			return clone(op), nil
		}
	}
	return Operation{}, errors.New("operation missing")
}

// Metadata polling never serializes waveform/metric rows or full conversations.
func (s *Store) Metadata() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.data
	v.ChangeSets = nil
	v.TaskTemplates = nil
	v.Operations = append([]Operation(nil), s.data.Operations...)
	for i := range v.Operations {
		v.Operations[i].Log = ""
		v.Operations[i].Code = nil
	}
	v.Batches = append([]Batch(nil), s.data.Batches...)
	for i := range v.Batches {
		v.Batches[i].Metrics = nil
	}
	v.Sessions = append([]Session(nil), s.data.Sessions...)
	for i := range v.Sessions {
		v.Sessions[i].Messages = nil
		if v.Sessions[i].LastTask != nil {
			t := *v.Sessions[i].LastTask
			t.Events = nil
			t.Content = ""
			t.Terminal = nil
			v.Sessions[i].LastTask = &t
		}
	}
	v.Jobs = append([]Job(nil), s.data.Jobs...)
	for i := range v.Jobs {
		v.Jobs[i].Log = ""
		v.Jobs[i].Evidence = ""
		v.Jobs[i].ProjectSnapshot = nil
		v.Jobs[i].EnvironmentSnapshot = nil
	}
	return clone(v)
}
func (s *Store) Close() {
	if s.lock != nil {
		syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		s.lock.Close()
		s.lock = nil
	}
}

// Kept as a rejecting compatibility boundary for older clients.
func (s *Store) Backup() (string, error) {
	return "", errors.New("Project backup disabled; company backup manages source data")
}
func FindProject(s State, id string) (Project, error) {
	for _, p := range s.Projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, errors.New("project not found")
}
func SafePath(root, relative string) (string, error) {
	r, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	p, e := filepath.EvalSymlinks(filepath.Join(r, relative))
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(r, p)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path outside project")
	}
	return p, nil
}
