package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type FileNote struct {
	Project     string `json:"project"`
	Root        string `json:"root"`
	Path        string `json:"path"`
	Notes       string `json:"notes"`
	Description string `json:"description"`
}
type SoftwareVersion struct {
	Name     string `json:"name"`
	Setup    string `json:"setup"`
	ToolPath string `json:"toolPath"`
}
type Software struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Command  string            `json:"command"`
	Shell    string            `json:"shell"`
	Versions []SoftwareVersion `json:"versions"`
	Active   string            `json:"active"`
}
type Plugin struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Description string  `json:"description"`
	Command     Command `json:"command"`
	Enabled     bool    `json:"enabled"`
	Timeout     int     `json:"timeoutSeconds"`
}
type Preferences struct {
	Formatters  map[string]Command `json:"formatters,omitempty"`
	Compilers   map[string]Command `json:"compilers,omitempty"`
	Configured  bool               `json:"configured"`
	ModelURL    string             `json:"modelUrl"`
	Model       string             `json:"model"`
	KeyEnv      string             `json:"keyEnv"`
	ModelTools  bool               `json:"modelTools"`
	Shell       string             `json:"shell"`
	Setup       string             `json:"setup"`
	Workspace   string             `json:"workspace"`
	Threshold   float64            `json:"threshold"`
	PollSeconds int                `json:"pollSeconds"`
	Compiler    Command            `json:"compiler"`
}
type Operation struct {
	Code        *CodeResult `json:"codeResult,omitempty"`
	Connection  string      `json:"connection,omitempty"`
	Command     Command     `json:"command"`
	Environment Environment `json:"environment"`
	Directory   string      `json:"directory"`
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	Status      string      `json:"status"`
	Created     string      `json:"created"`
	Finished    string      `json:"finished"`
	Log         string      `json:"log"`
	Error       string      `json:"error"`
}
type Validation struct {
	Time        string   `json:"time"`
	Model       string   `json:"model"`
	Environment string   `json:"environment"`
	Details     []string `json:"details"`
}

func (a *App) preferences() Preferences {
	a.Store.mu.Lock()
	p := clone(a.Store.data.Preferences)
	a.Store.mu.Unlock()
	if !p.Configured {
		p.ModelURL = a.Config.ModelURL
		p.Model = a.Config.Model
		p.KeyEnv = a.Config.KeyEnv
		p.ModelTools = a.Config.ModelTools
	}
	if p.Shell == "" {
		p.Shell = "/bin/bash"
	}
	if p.Workspace == "" {
		p.Workspace = a.Store.dir
	}
	if p.Threshold == 0 {
		p.Threshold = 90
	}
	if p.PollSeconds == 0 {
		p.PollSeconds = 5
	}
	return p
}
func validShell(s string) bool {
	return s == "/bin/sh" || s == "/bin/bash" || s == "/bin/csh" || s == "/bin/tcsh"
}
func projectRoots(p Project) []string {
	out := []string{p.Root}
	for _, r := range p.Paths {
		if r != "" && r != p.Root {
			out = append(out, r)
		}
	}
	return out
}
func (a *App) fileRoot(project, root string) (string, error) {
	p, e := FindProject(a.Store.Snapshot(), project)
	if e != nil {
		return "", e
	}
	if root == "" {
		root = p.Root
	}
	for _, v := range projectRoots(p) {
		if root == v {
			return root, nil
		}
	}
	return "", errors.New("path is not registered with this project")
}
func (a *App) browse(project, root, path string, offset int) (any, error) {
	root, e := a.fileRoot(project, root)
	if e != nil {
		return nil, e
	}
	return a.browseRoot(project, root, path, offset)
}
func (a *App) browseRoot(project, root, path string, offset int) (any, error) {
	real, e := SafePath(root, path)
	if e != nil {
		return nil, e
	}
	info, e := os.Stat(real)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, errors.New("only regular files can be read")
		}
		b, e := readBounded(real, a.Config.MaxPreview)
		if !utf8.Valid(b) {
			return nil, errors.New("文件不是有效 UTF-8 文本，请按原编码转换后再预览 / Invalid UTF-8; convert using the source encoding first")
		}
		if strings.Contains(string(b), "\x00") {
			return nil, errors.New("binary file: export text/CSV/VCD using the vendor tool")
		}
		return map[string]any{"content": string(b), "size": info.Size(), "truncated": info.Size() > int64(len(b)), "path": path, "root": root}, e
	}
	if offset < 0 {
		return nil, errors.New("invalid offset")
	}
	f, e := os.Open(real)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	for skip := 0; skip < offset; {
		n := offset - skip
		if n > 200 {
			n = 200
		}
		batch, er := f.ReadDir(n)
		skip += len(batch)
		if er != nil {
			return map[string]any{"entries": []any{}, "more": false}, nil
		}
	}
	entries, e := f.ReadDir(201)
	if e != nil && e != io.EOF {
		return nil, e
	}
	notes := a.Store.Snapshot().FileNotes
	out := []any{}
	for i, x := range entries {
		if i == 200 {
			break
		}
		rel := filepath.Join(path, x.Name())
		note := FileNote{}
		for _, v := range notes {
			if v.Project == project && v.Root == root && v.Path == rel {
				note = v
			}
		}
		out = append(out, map[string]any{"name": x.Name(), "path": rel, "directory": x.IsDir(), "symlink": x.Type()&os.ModeSymlink != 0, "notes": note.Notes, "description": note.Description})
	}
	return map[string]any{"entries": out, "more": len(entries) > 200, "offset": offset, "root": root, "path": path}, nil
}

// Search streams arbitrary-sized regular files. A cursor resumes inside a large
// file, so resource limits never silently exclude its remaining text.
func (a *App) search(ctx context.Context, project, query, after string) (any, error) {
	select {
	case a.searchSlots <- struct{}{}:
		defer func() { <-a.searchSlots }()
	default:
		return nil, errors.New("another search is active; retry after it finishes")
	}
	if strings.TrimSpace(query) == "" || len(query) > 200 {
		return nil, errors.New("keyword required, max 200 bytes")
	}
	type cursor struct {
		InFile   bool   `json:"inFile"`
		Key      string `json:"key"`
		Offset   int64  `json:"offset"`
		Line     int    `json:"line"`
		Size     int64  `json:"size"`
		Modified int64  `json:"modified"`
	}
	old := cursor{}
	if after != "" {
		if e := json.Unmarshal([]byte(after), &old); e != nil {
			return nil, errors.New("invalid search cursor")
		}
	}
	state := a.Store.Metadata()
	projects := state.Projects
	if project != "" {
		p, e := FindProject(state, project)
		if e != nil {
			return nil, e
		}
		projects = []Project{p}
	}
	if scope, ok := ctx.Value(agentScopeKey{}).(Session); ok && scope.Folder != "" {
		projects = []Project{{ID: "folder", Root: scope.Folder}}
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	hits := []any{}
	scanned, skipped := 0, 0
	var readBytes int64
	last := old
	reached := old.Key == ""
	more := false
	q := strings.ToLower(query)
	stop := errors.New("page complete")
	for _, p := range projects {
		roots := projectRoots(p)
		sort.Strings(roots)
		for _, root := range roots {
			err := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
				if ctx.Err() != nil {
					more = true
					return stop
				}
				if e != nil {
					skipped++
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 {
					skipped++
					return nil
				}
				if d.IsDir() {
					if d.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}
				rel, _ := filepath.Rel(root, path)
				key := p.ID + "|" + root + "|" + rel
				resume := false
				if !reached {
					if key != old.Key {
						return nil
					}
					reached = true
					if !old.InFile {
						return nil
					}
					resume = true
				}
				if scanned >= 20000 || readBytes >= 32<<20 {
					more = true
					return stop
				}
				scanned++
				info, e := d.Info()
				if e != nil || !info.Mode().IsRegular() {
					skipped++
					last = cursor{Key: key}
					return nil
				}
				if resume && (info.Size() != old.Size || info.ModTime().UnixNano() != old.Modified) {
					return errors.New("file changed during search; restart search")
				}
				match := strings.Contains(strings.ToLower(rel), q)
				snippet := ""
				line := 0
				for _, note := range state.FileNotes {
					if note.Project == p.ID && note.Root == root && note.Path == rel && strings.Contains(strings.ToLower(note.Notes+" "+note.Description), q) {
						match = true
						snippet = note.Notes + " " + note.Description
						break
					}
				}
				f, e := os.Open(path)
				if e != nil {
					skipped++
					last = cursor{Key: key}
					return nil
				}
				defer f.Close()
				position := int64(0)
				lines := 0
				if resume {
					position = old.Offset
					lines = old.Line
					if _, e = f.Seek(position, io.SeekStart); e != nil {
						return e
					}
				}
				buf := make([]byte, 65536)
				carry := ""
				for !match {
					if ctx.Err() != nil || readBytes >= 32<<20 {
						last = cursor{Key: key, InFile: true, Offset: position, Line: lines, Size: info.Size(), Modified: info.ModTime().UnixNano()}
						more = true
						return stop
					}
					n, err := f.Read(buf)
					if n > 0 {
						part := string(buf[:n])
						if strings.Contains(part, "\x00") {
							skipped++
							break
						}
						combined := carry + part
						idx := strings.Index(strings.ToLower(combined), q)
						if idx >= 0 {
							match = true
							line = lines - strings.Count(carry, "\n") + strings.Count(combined[:idx], "\n") + 1
							lo := strings.LastIndex(combined[:idx], "\n") + 1
							hi := strings.Index(combined[idx:], "\n")
							if hi < 0 {
								hi = len(combined)
							} else {
								hi += idx
							}
							if hi-lo > 400 {
								lo = idx
								if hi-lo > 400 {
									hi = lo + 400
								}
							}
							snippet = combined[lo:hi]
						}
						lines += strings.Count(part, "\n")
						position += int64(n)
						readBytes += int64(n)
						keep := len(query) * 4
						if keep > len(combined) {
							keep = len(combined)
						}
						carry = combined[len(combined)-keep:]
					}
					if err == io.EOF {
						break
					}
					if err != nil {
						skipped++
						break
					}
					if n == 0 {
						break
					}
					// Re-read the small overlap on resume to include a keyword crossing the boundary.
					if !match && (ctx.Err() != nil || readBytes >= 32<<20) {
						position -= int64(len(carry))
						lines -= strings.Count(carry, "\n")
						if position < 0 {
							position = 0
						}
					}
				}
				last = cursor{Key: key}
				if match {
					hits = append(hits, map[string]any{"project": p.ID, "root": root, "path": rel, "line": line, "snippet": snippet})
					if len(hits) >= 100 {
						more = true
						return stop
					}
				}
				return nil
			})
			if err != nil && err != stop {
				return nil, err
			}
			if err == stop {
				more = true
				break
			}
		}
		if more {
			break
		}
	}
	if !reached && old.Key != "" {
		return nil, errors.New("search cursor path disappeared; restart search")
	}
	return map[string]any{"hits": hits, "scanned": scanned, "skipped": skipped, "more": more, "cursor": StringJSON(last), "cancelled": ctx.Err() != nil, "bytesRead": readBytes}, nil
}

func (a *App) platformAPI(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/v2/") {
		return false
	}
	var out any
	var e error
	switch r.URL.Path {
	case "/api/v2/action":
		if r.Method != "POST" {
			e = errors.New("POST required")
			break
		}
		var in struct {
			Action string          `json:"action"`
			Input  json.RawMessage `json:"input"`
		}
		if e = decode(r, &in); e == nil {
			out, e = a.action(r.Context(), in.Action, in.Input)
		}
	case "/api/v2/agent":
		if r.Method != "POST" {
			e = errors.New("POST required")
			break
		}
		var in AgentInput
		if e = decode(r, &in); e == nil {
			out, e = a.startAgent(in)
		}
	case "/api/v2/progress":
		if r.Method != "GET" {
			e = errors.New("GET required")
			break
		}
		out, e = a.agentProgress(r.URL.Query().Get("id"))
	case "/api/v2/cancel-agent":
		if r.Method != "POST" {
			e = errors.New("POST required")
			break
		}
		var in struct {
			ID string `json:"id"`
		}
		if e = decode(r, &in); e == nil {
			a.mu.Lock()
			if c := a.cancels["agent:"+in.ID]; c != nil {
				c()
			}
			a.mu.Unlock()
			out = map[string]bool{"requested": true}
		}
	case "/api/v2/approve":
		if r.Method != "POST" {
			e = errors.New("POST required")
			break
		}
		var in struct {
			ID      string `json:"id"`
			Approve bool   `json:"approve"`
		}
		if e = decode(r, &in); e == nil {
			e = a.approveAgent(in.ID, in.Approve)
			out = map[string]bool{"accepted": e == nil}
		}
	default:
		e = errors.New("unknown endpoint")
	}
	reply(w, out, e)
	return true
}
func parseInput(b json.RawMessage, v any) error {
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func (a *App) action(ctx context.Context, action string, b json.RawMessage) (any, error) {
	switch action {
	case "task-templates", "save-task-template", "preview-task-template":
		return a.templateAction(action, b)
	case "compare-metrics":
		return compareUnitMetrics(b)
	case "propose-change-set", "change-sets", "change-set", "apply-change-file", "undo-change-file", "archive-change-set":
		return a.changeSetAction(ctx, action, b)
	}
	var s State
	switch action {
	case "batch":
		s = a.Store.Snapshot()
	case "launch-software", "run-plugin":
		s = a.Store.Metadata()
	}
	switch action {
	case "wait-job", "wait-operation":
		var x struct {
			ID string `json:"id"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		var last any
		for {
			if action == "wait-job" {
				j, e := a.Job(x.ID)
				if e != nil {
					return nil, e
				}
				last = j
				if j.Status != "queued" && j.Status != "running" {
					return j, nil
				}
			} else {
				found := false
				for _, o := range a.Store.Snapshot().Operations {
					if o.ID == x.ID {
						found = true
						last = o
						if o.Status != "queued" && o.Status != "running" {
							return o, nil
						}
					}
				}
				if !found {
					return nil, errors.New("operation missing")
				}
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timer.C:
				return last, nil
			case <-tick.C:
			}
		}
	case "job", "batch":
		var x struct {
			ID string `json:"id"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		if action == "job" {
			return a.Job(x.ID)
		}
		for _, v := range s.Batches {
			if v.ID == x.ID {
				return v, nil
			}
		}
		return nil, errors.New("batch missing")
	case "workspace":
		v := a.Store.Metadata()
		v.Preferences = Preferences{}
		return v, nil
	case "browse", "search", "note":
		var x struct {
			Project     string `json:"project"`
			Root        string `json:"root"`
			Path        string `json:"path"`
			Query       string `json:"query"`
			Cursor      string `json:"cursor"`
			Offset      int    `json:"offset"`
			Notes       string `json:"notes"`
			Description string `json:"description"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		if action == "browse" {
			return a.browse(x.Project, x.Root, x.Path, x.Offset)
		}
		if action == "search" {
			return a.search(ctx, x.Project, x.Query, x.Cursor)
		}
		root, e := a.fileRoot(x.Project, x.Root)
		if e != nil {
			return nil, e
		}
		if _, e = SafePath(root, x.Path); e != nil {
			return nil, e
		}
		n := FileNote{x.Project, root, filepath.Clean(x.Path), x.Notes, x.Description}
		if len(n.Notes)+len(n.Description) > 20000 {
			return nil, errors.New("note too long")
		}
		e = a.Store.Update(func(s *State) error {
			for i, v := range s.FileNotes {
				if v.Project == n.Project && v.Root == n.Root && v.Path == n.Path {
					s.FileNotes[i] = n
					return nil
				}
			}
			if len(s.FileNotes) >= 50000 {
				return errors.New("note limit reached")
			}
			s.FileNotes = append(s.FileNotes, n)
			return nil
		})
		return n, e
	case "save-project":
		var p Project
		if e := parseInput(b, &p); e != nil {
			return nil, e
		}
		if p.Name == "" || len(p.Name) > 200 || len(p.Paths) > 32 {
			return nil, errors.New("name required; max 32 paths")
		}
		if p.ID == "" {
			p.ID = ID()
		}
		roots := projectRoots(p)
		for i, r := range roots {
			if !filepath.IsAbs(r) {
				return nil, errors.New("absolute existing project paths required")
			}
			real, e := filepath.EvalSymlinks(r)
			if e != nil {
				return nil, e
			}
			info, e := os.Stat(real)
			if e != nil || !info.IsDir() {
				return nil, errors.New("project directory unavailable")
			}
			roots[i] = real
		}
		p.Root = roots[0]
		p.Paths = roots[1:]
		e := a.Store.Update(func(s *State) error {
			for i, x := range s.Projects {
				if x.ID == p.ID {
					s.Projects[i] = p
					return nil
				}
			}
			if len(s.Projects) >= 1000 {
				return errors.New("project limit")
			}
			s.Projects = append(s.Projects, p)
			return nil
		})
		return p, e
	case "save-software":
		var x Software
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		if x.Name == "" || x.Command == "" || len(x.Command) > 10000 || !validShell(x.Shell) || len(x.Versions) == 0 || len(x.Versions) > 100 {
			return nil, errors.New("name, command, supported shell and versions required")
		}
		seen := map[string]bool{}
		active := false
		for _, v := range x.Versions {
			if v.Name == "" || seen[v.Name] {
				return nil, errors.New("unique version names required")
			}
			seen[v.Name] = true
			if v.Name == x.Active {
				active = true
			}
			if v.Setup != "" && !filepath.IsAbs(v.Setup) {
				return nil, errors.New("absolute environment script required")
			}
		}
		if !active {
			return nil, errors.New("active version not found")
		}
		if x.ID == "" {
			x.ID = ID()
		}
		e := a.Store.Update(func(s *State) error {
			for _, version := range x.Versions {
				eid := "software:" + x.ID + ":" + version.Name
				env := Environment{ID: eid, Name: x.Name, Version: version.Name, Executable: x.Shell, Args: []string{"-c", x.Command}, Shell: x.Shell, Setup: version.Setup, ToolPath: version.ToolPath}
				found := false
				for i, old := range s.Environments {
					if old.ID == eid {
						s.Environments[i] = env
						found = true
					}
				}
				if !found {
					s.Environments = append(s.Environments, env)
				}
			}
			for i, v := range s.Software {
				if v.ID == x.ID {
					s.Software[i] = x
					return nil
				}
			}
			if len(s.Software) >= 1000 {
				return errors.New("software limit")
			}
			s.Software = append(s.Software, x)
			return nil
		})
		return x, e
	case "launch-software":
		var x struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			Project string `json:"project"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		for _, v := range s.Software {
			if v.ID == x.ID {
				if x.Version == "" {
					x.Version = v.Active
				}
				for _, ver := range v.Versions {
					if ver.Name == x.Version {
						dir := a.preferences().Workspace
						if x.Project != "" {
							p, e := FindProject(s, x.Project)
							if e != nil {
								return nil, e
							}
							dir = p.Root
						}
						statement := v.Command
						flag := "-c"
						prefix := ". "
						if v.Shell == "/bin/csh" || v.Shell == "/bin/tcsh" {
							flag = "-fc"
							prefix = "source "
						}
						if ver.Setup != "" {
							statement = prefix + quote(ver.Setup) + " && " + statement
						}
						return a.operation("software", v.Name+" "+ver.Name, Command{v.Shell, []string{flag, statement}}, Environment{Shell: v.Shell, ToolPath: ver.ToolPath}, dir, 86400)
					}
				}
			}
		}
		return nil, errors.New("software/version missing")
	case "install-plugins":
		var x struct {
			Plugins []Plugin `json:"plugins"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		if len(x.Plugins) < 1 || len(x.Plugins) > 100 {
			return nil, errors.New("supply 1..100 manifests")
		}
		ids := map[string]bool{}
		for _, p := range x.Plugins {
			if p.ID == "" || p.Name == "" || ids[p.ID] || !filepath.IsAbs(p.Command.Executable) || len(p.Command.Args) > 64 || p.Timeout < 1 || p.Timeout > 3600 {
				return nil, errors.New("invalid plugin manifest: unique id, name, absolute executable and timeout 1..3600 required")
			}
			ids[p.ID] = true
		}
		e := a.Store.Update(func(s *State) error {
			for _, p := range x.Plugins {
				found := false
				for i, v := range s.Plugins {
					if v.ID == p.ID {
						s.Plugins[i] = p
						found = true
					}
				}
				if !found {
					s.Plugins = append(s.Plugins, p)
				}
			}
			if len(s.Plugins) > 1000 {
				return errors.New("plugin limit")
			}
			return nil
		})
		return x, e
	case "run-plugin":
		var x struct {
			ID      string `json:"id"`
			Project string `json:"project"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		for _, p := range s.Plugins {
			if p.ID == x.ID && p.Enabled {
				dir := a.preferences().Workspace
				if x.Project != "" {
					pr, e := FindProject(s, x.Project)
					if e != nil {
						return nil, e
					}
					dir = pr.Root
				}
				return a.operation("plugin", p.Name, p.Command, Environment{}, dir, p.Timeout)
			}
		}
		return nil, errors.New("plugin missing or disabled")
	case "monitor":
		return a.monitor(), nil
	case "terminate":
		var x ProcessIdentity
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		return map[string]bool{"requested": true}, a.terminate(x)
	case "preferences":
		return map[string]any{"preferences": a.preferences(), "validation": a.validationSnapshot()}, nil
	case "save-preferences":
		var x Preferences
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		if x.Threshold < 1 || x.Threshold > 100 || x.PollSeconds < 3 || x.PollSeconds > 60 || !validShell(x.Shell) {
			return nil, errors.New("threshold 1..100, interval 3..60 and supported shell required")
		}
		if x.ModelURL != "" {
			u, e := url.Parse(x.ModelURL)
			if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
				return nil, errors.New("valid HTTP(S) endpoint without credentials required")
			}
		}
		if !filepath.IsAbs(x.Workspace) || (x.Setup != "" && !filepath.IsAbs(x.Setup)) {
			return nil, errors.New("absolute workspace/setup paths required")
		}
		if x.Compiler.Executable != "" && !filepath.IsAbs(x.Compiler.Executable) {
			return nil, errors.New("absolute compiler executable required")
		}
		for language, command := range x.Formatters {
			if hdlExtension(language) == "" || (command.Executable != "" && !filepath.IsAbs(command.Executable)) || len(command.Args) > 64 {
				return nil, errors.New("invalid formatter")
			}
		}
		for language, command := range x.Compilers {
			if hdlExtension(language) == "" || (command.Executable != "" && !filepath.IsAbs(command.Executable)) || len(command.Args) > 64 {
				return nil, errors.New("invalid per-language compiler")
			}
		}
		x.Configured = true
		e := a.Store.Update(func(s *State) error { s.Preferences = x; return nil })
		if e != nil {
			return nil, e
		}
		a.startValidation()
		return map[string]any{"saved": true, "validation": a.validationSnapshot()}, nil
	case "validate":
		a.startValidation()
		return a.validationSnapshot(), nil
	case "analysis":
		return a.analyze(b)
	case "calculate":
		return calculate(b)
	case "storage-usage":
		return a.Store.StorageUsage()
	case "eda-agent":
		return a.startEDAAgent(b)
	case "code-workflow":
		return a.startCodeWorkflow(b)
	case "improve-code":
		return a.improveCode(ctx, b)
	case "analyze-ai":
		return a.analyzeAI(ctx, b)
	case "save-mcp", "mcp-tools", "mcp-call", "remove-mcp", "disconnect-mcp":
		return a.edaAction(ctx, action, b)
	case "generate-code", "generate-hdl":
		return a.generateHDL(ctx, b)
	case "save-code", "save-hdl":
		return a.saveHDL(b)
	case "compile-code", "compile-hdl":
		return a.compileHDL(b)
	case "operation":
		var x struct {
			ID string `json:"id"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		return a.Store.Operation(x.ID)
	case "cancel-operation":
		var x struct {
			ID string `json:"id"`
		}
		if e := parseInput(b, &x); e != nil {
			return nil, e
		}
		a.mu.Lock()
		c := a.cancels["op:"+x.ID]
		if c != nil {
			c()
		}
		a.mu.Unlock()
		return map[string]bool{"requested": c != nil}, nil
	}
	// Existing simulation, result, optimizer and metadata APIs remain available to the Agent.
	paths := map[string]string{"simulate": "jobs", "cancel-job": "cancel", "retry-job": "retry", "optimize": "experiments", "control-optimization": "experiment-control", "import-results": "import", "compare-results": "compare", "export-results": "export", "save-environment": "environments", "new-session": "sessions", "language": "settings"}
	if path, ok := paths[action]; ok {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/"+path, strings.NewReader(string(b))).WithContext(ctx)
		a.api(rec, req)
		var v any
		e := json.Unmarshal(rec.Body.Bytes(), &v)
		if rec.Code != 200 {
			return nil, fmt.Errorf("%s", rec.Body.String())
		}
		return v, e
	}
	return nil, errors.New("unknown platform action")
}
func (a *App) operation(kind, name string, c Command, env Environment, dir string, seconds int) (Operation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return Operation{}, errors.New("shutting down")
	}
	active := 0
	for _, v := range a.Store.Snapshot().Operations {
		if v.Status == "running" || v.Status == "queued" {
			active++
		}
	}
	if active >= 8 {
		return Operation{}, errors.New("operation queue full")
	}
	op := Operation{ID: ID(), Kind: kind, Name: name, Created: Now(), Status: "queued", Command: clone(c), Environment: env, Directory: dir}
	e := a.Store.Update(func(s *State) error {
		if len(s.Operations) >= 10000 {
			return errors.New("operation history limit")
		}
		s.Operations = append(s.Operations, op)
		return nil
	})
	if e != nil {
		return op, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	a.cancels["op:"+op.ID] = cancel
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer cancel()
		defer func() { a.mu.Lock(); delete(a.cancels, "op:"+op.ID); a.mu.Unlock() }()
		update := func(status, log string, err error) error {
			return a.Store.Update(func(s *State) error {
				for i := range s.Operations {
					v := &s.Operations[i]
					if v.ID == op.ID {
						v.Status = status
						v.Log = log
						if err != nil {
							v.Error = err.Error()
						}
						if status != "running" {
							v.Finished = Now()
						}
					}
				}
				return nil
			})
		}
		select {
		case a.slots <- struct{}{}:
			defer func() { <-a.slots }()
		case <-ctx.Done():
			update("cancelled", "", ctx.Err())
			return
		}
		if e := update("running", "", nil); e != nil {
			return
		}
		var log string
		var err error
		if a.Config.Mode == "demo" {
			log = "DEMO: command preview only; no process executed.\n" + StringJSON(c) + "\n" + StringJSON(env)
		} else {
			prefs := a.preferences()
			if env.Setup == "" {
				env.Setup = prefs.Setup
				env.Shell = prefs.Shell
			}
			log, err = a.command(ctx, c, env, dir, map[string]string{"project": dir})
		}
		status := "succeeded"
		if err != nil {
			status = "failed"
		}
		if ctx.Err() != nil {
			status = "cancelled"
		}
		update(status, log, err)
	}()
	return op, nil
}
func (a *App) validationSnapshot() Validation {
	a.mu.Lock()
	defer a.mu.Unlock()
	return clone(a.validation)
}
func (a *App) startValidation() {
	a.mu.Lock()
	if a.validating || a.closing {
		if a.validating && !a.closing {
			a.validationAgain = true
		}
		a.mu.Unlock()
		return
	}
	a.validating = true
	a.validation = Validation{Time: Now(), Model: "checking", Environment: "checking", Details: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	validationKey := "validation:" + ID()
	a.cancels[validationKey] = cancel
	a.wg.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.wg.Done()
		defer func() { a.mu.Lock(); delete(a.cancels, validationKey); a.mu.Unlock() }()
		defer cancel()
		p := a.preferences()
		a.mu.Lock()
		a.loadedEnv = nil
		a.mu.Unlock()
		v := Validation{Time: Now(), Model: "unconfigured", Environment: "ready", Details: []string{}}
		if info, e := os.Stat(p.Workspace); e != nil || !info.IsDir() {
			v.Environment = "failed"
			v.Details = append(v.Details, "Workspace directory unavailable")
		}
		if _, e := os.Stat(p.Shell); e != nil {
			v.Environment = "failed"
			v.Details = append(v.Details, e.Error())
		}
		if p.Setup != "" {
			if a.Config.Mode == "demo" {
				v.Environment = "demo-not-executed"
			} else {
				env, e := a.loadEnvironment(ctx, p)
				if e == nil {
					a.mu.Lock()
					a.loadedEnv = env
					a.mu.Unlock()
				}
				if e != nil {
					v.Environment = "failed"
					v.Details = append(v.Details, "Environment load failed: "+e.Error())
				}
			}
		}
		if p.ModelURL != "" {
			if a.Config.Mode == "demo" {
				v.Model = "demo-offline"
			} else {
				_, e := a.model(ctx, []modelMessage{{Role: "user", Content: "Reply OK. Connection validation only."}}, false, false)
				if e != nil {
					v.Model = "failed"
					v.Details = append(v.Details, e.Error())
				} else {
					v.Model = "connected"
				}
			}
		}
		a.mu.Lock()
		a.validation = v
		a.validating = false
		again := a.validationAgain
		a.validationAgain = false
		a.mu.Unlock()
		if again {
			a.startValidation()
		}
	}()
}

func (a *App) StartValidation() { a.startValidation() }

// A private transient file separates environment data from setup script chatter.
func (a *App) loadEnvironment(ctx context.Context, p Preferences) ([]string, error) {
	dir, e := os.MkdirTemp("", "icw-environment-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(dir)
	target := filepath.Join(dir, "environment")
	_, e = a.command(ctx, Command{"/bin/sh", []string{"-c", "umask 077; /usr/bin/env -0 > " + quote(target)}}, Environment{Setup: p.Setup, Shell: p.Shell}, p.Workspace, nil)
	if e != nil {
		return nil, e
	}
	b, e := readBounded(target, 2<<20)
	if e != nil {
		return nil, e
	}
	if len(b) == 0 || b[len(b)-1] != 0 {
		return nil, errors.New("environment capture truncated or invalid")
	}
	return strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00"), nil
}
