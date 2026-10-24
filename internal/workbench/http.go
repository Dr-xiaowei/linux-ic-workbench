package workbench

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed web/*
var web embed.FS

func (a *App) Handler() http.Handler {
	files, _ := fs.Sub(web, "web")
	static := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'")
		host, _, e := net.SplitHostPort(r.Host)
		if e != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			http.Error(w, "loopback host required; use SSH port forwarding", 403)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://"+r.Host {
			http.Error(w, "origin rejected", 403)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(a.Token)) != 1 {
				http.Error(w, "unauthorized", 401)
				return
			}
			a.mu.Lock()
			if a.closing {
				a.mu.Unlock()
				http.Error(w, "server shutting down", http.StatusServiceUnavailable)
				return
			}
			a.wg.Add(1)
			a.mu.Unlock()
			defer a.wg.Done()
			a.api(w, r)
			return
		}
		if a.Config.DesktopAPI {
			http.Error(w, "Native desktop service: no web interface", http.StatusNotFound)
			return
		}
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		static.ServeHTTP(w, r)
	})
}
func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 4<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return e
	}
	var extra any
	if e := dec.Decode(&extra); e != io.EOF {
		return errors.New("single JSON object required")
	}
	return nil
}
func reply(w http.ResponseWriter, v any, e error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if e != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
		return
	}
	json.NewEncoder(w).Encode(v)
}
func (a *App) api(w http.ResponseWriter, r *http.Request) {
	if a.workspaceAPI(w, r) || a.platformAPI(w, r) {
		return
	}
	var result any
	var err error
	path := r.URL.Path
	var state State
	if path == "/api/state" {
		state = a.Store.Metadata()
	} else {
		state = a.Store.Snapshot()
	}
	if r.Method == "GET" {
		switch path {
		case "/api/state":
			for i := range state.Sessions {
				state.Sessions[i].Messages = nil
			}
			for i := range state.Batches {
				state.Batches[i].Metrics = nil
			}
			for i := range state.Jobs {
				state.Jobs[i].Log = ""
			}
			result = map[string]any{"state": state, "codeLanguages": CodeLanguages, "version": Version, "model": a.Config.Model, "modelConfigured": a.Config.ModelURL != "", "limits": map[string]any{"workers": a.Config.MaxWorkers, "previewBytes": a.Config.MaxPreview}, "adapters": map[string]bool{"probe": a.Config.Probe.Executable != "", "prepare": a.Config.Prepare.Executable != "", "simulate": a.Config.Simulate.Executable != "", "export": a.Config.Export.Executable != ""}}
		case "/api/session":
			for _, x := range state.Sessions {
				if x.ID == r.URL.Query().Get("id") {
					result = x
				}
			}
			if result == nil {
				err = errors.New("session not found")
			}
		case "/api/batch":
			for _, x := range state.Batches {
				if x.ID == r.URL.Query().Get("id") {
					result = x
				}
			}
			if result == nil {
				err = errors.New("batch not found")
			}
		case "/api/job":
			result, err = a.Job(r.URL.Query().Get("id"))
		case "/api/files":
			var p Project
			p, err = FindProject(state, r.URL.Query().Get("project"))
			if err == nil {
				var real string
				real, err = SafePath(p.Root, r.URL.Query().Get("path"))
				if err == nil {
					info, e := os.Stat(real)
					err = e
					if e == nil {
						if info.IsDir() {
							var f *os.File
							f, err = os.Open(real)
							if err == nil {
								entries, e := f.ReadDir(201)
								f.Close()
								if e != nil && e != io.EOF && len(entries) == 0 {
									err = e
								}
								list := []map[string]any{}
								for i, x := range entries {
									if i >= 200 {
										break
									}
									list = append(list, map[string]any{"name": x.Name(), "directory": x.IsDir(), "symlink": x.Type()&os.ModeSymlink != 0})
								}
								result = map[string]any{"entries": list, "limited": len(entries) > 200}
							}
						} else {
							var b []byte
							b, err = readBounded(real, a.Config.MaxPreview)
							if err == nil {
								if strings.Contains(string(b), "\x00") {
									err = errors.New("binary preview unsupported; use original EDA viewer")
								} else {
									result = map[string]any{"content": string(b), "size": info.Size(), "path": r.URL.Query().Get("path")}
								}
							}
						}
					}
				}
			}
		default:
			err = errors.New("unknown endpoint")
		}
		reply(w, result, err)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	switch path {
	case "/api/projects":
		var p Project
		if err = decode(r, &p); err == nil {
			p.Name = strings.TrimSpace(p.Name)
			if p.Name == "" || len(p.Name) > 200 {
				err = errors.New("project name required, max 200 characters")
				break
			}
			if p.Root == "" {
				err = errors.New("project root required")
				break
			}
			p.Root, err = filepath.Abs(p.Root)
			if err != nil {
				break
			}
			p.Root, err = filepath.EvalSymlinks(p.Root)
			if err != nil {
				break
			}
			var info os.FileInfo
			info, err = os.Stat(p.Root)
			if err != nil || !info.IsDir() {
				err = errors.New("project root must be an existing directory")
				break
			}
			if p.ID == "" {
				p.ID = ID()
			}
			err = a.Store.Update(func(s *State) error {
				for i, x := range s.Projects {
					if x.ID == p.ID {
						s.Projects[i] = p
						return nil
					}
				}
				if len(s.Projects) >= 1000 {
					return errors.New("project limit reached")
				}
				s.Projects = append(s.Projects, p)
				return nil
			})
			result = p
		}
	case "/api/environments":
		var x Environment
		if err = decode(r, &x); err == nil {
			if x.Name == "" {
				err = errors.New("environment name required")
				break
			}
			if x.Executable != "" && !filepath.IsAbs(x.Executable) {
				err = errors.New("absolute executable required")
				break
			}
			if x.Setup != "" && !filepath.IsAbs(x.Setup) {
				err = errors.New("absolute setup script required")
				break
			}
			if x.ID == "" {
				x.ID = ID()
			}
			err = a.Store.Update(func(s *State) error {
				for i, v := range s.Environments {
					if v.ID == x.ID {
						s.Environments[i] = x
						return nil
					}
				}
				if len(s.Environments) >= 200 {
					return errors.New("environment limit reached")
				}
				s.Environments = append(s.Environments, x)
				return nil
			})
			result = x
		}
	case "/api/sessions":
		var x struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Project string `json:"project"`
			Folder  string `json:"folder"`
		}
		if err = decode(r, &x); err == nil {
			if x.Folder != "" {
				x.Folder, err = a.validateFolder(x.Folder)
				if err != nil {
					break
				}
				if x.Project != "" {
					err = errors.New("choose either project or folder")
					break
				}
			}
			if x.Project != "" {
				_, err = FindProject(state, x.Project)
				if err != nil {
					break
				}
			}
			if x.ID == "" {
				x.ID = ID()
			}
			if x.Title == "" {
				x.Title = "New conversation"
			}
			err = a.Store.Update(func(s *State) error {
				for i, v := range s.Sessions {
					if v.ID == x.ID {
						s.Sessions[i].Title = x.Title
						return nil
					}
				}
				if len(s.Sessions) >= 1000 {
					return errors.New("session limit reached")
				}
				s.Sessions = append(s.Sessions, Session{ID: x.ID, Title: x.Title, Project: x.Project, Folder: x.Folder, Messages: []Message{}})
				return nil
			})
			result = x
		}
	case "/api/chat":
		var in ChatInput
		if err = decode(r, &in); err == nil {
			result, err = a.Chat(r.Context(), in)
		}
	case "/api/import":
		var in struct {
			Project    string `json:"project"`
			Name       string `json:"name"`
			Design     string `json:"design"`
			Definition string `json:"definition"`
			CSV        string `json:"csv"`
			Source     string `json:"source"`
			Parent     string `json:"parent"`
		}
		if err = decode(r, &in); err == nil {
			if _, err = FindProject(state, in.Project); err != nil {
				break
			}
			if in.Design == "" || in.Definition == "" || in.Name == "" || in.Source == "" {
				err = errors.New("name, source, design and measurement definition required")
				break
			}
			var ms []Metric
			ms, err = ParseCSV(in.CSV, in.Source)
			if err != nil {
				break
			}
			b := Batch{ID: ID(), Project: in.Project, Name: in.Name, Design: in.Design, Definition: in.Definition, Created: Now(), Metrics: ms, Provenance: "User import: " + in.Source, Parent: in.Parent}
			err = a.Store.Update(func(s *State) error {
				if len(s.Batches) >= a.Config.MaxRecords {
					return errors.New("batch limit reached")
				}
				s.Batches = append(s.Batches, b)
				return nil
			})
			result = b
		}
	case "/api/compare":
		var in struct {
			A string `json:"a"`
			B string `json:"b"`
		}
		if err = decode(r, &in); err == nil {
			var x, y Batch
			for _, b := range state.Batches {
				if b.ID == in.A {
					x = b
				}
				if b.ID == in.B {
					y = b
				}
			}
			if x.ID == "" || y.ID == "" {
				err = errors.New("select two batches")
			} else {
				result, err = Compare(x, y)
			}
		}
	case "/api/export":
		var in struct {
			ID     string `json:"id"`
			Format string `json:"format"`
		}
		if err = decode(r, &in); err == nil {
			err = errors.New("batch missing")
			for _, b := range state.Batches {
				if b.ID == in.ID {
					err = nil
					content := Report(b)
					if in.Format == "csv" {
						content = ExportCSV(b.Metrics)
					}
					if a.Config.Mode == "demo" {
						if in.Format == "csv" {
							for i := range b.Metrics {
								b.Metrics[i].Source = "DEMO: " + b.Metrics[i].Source
							}
							content = ExportCSV(b.Metrics)
						} else {
							content = "DEMO / SYNTHETIC\n\n" + content
						}
					}
					result = map[string]string{"content": content}
				}
			}
		}
	case "/api/diagnose":
		var in struct {
			Log string `json:"log"`
		}
		if err = decode(r, &in); err == nil {
			if len(in.Log) > 262144 {
				err = errors.New("log too large")
			} else {
				result = Diagnose(in.Log)
			}
		}
	case "/api/jobs":
		var in struct {
			Project     string             `json:"project"`
			Kind        string             `json:"kind"`
			Params      map[string]float64 `json:"params"`
			Environment string             `json:"environment"`
			Key         string             `json:"key"`
			Parent      string             `json:"parent"`
		}
		if err = decode(r, &in); err == nil {
			result, err = a.Submit(Job{Project: in.Project, Kind: in.Kind, Params: in.Params, Environment: in.Environment, Key: in.Key, Parent: in.Parent})
		}
	case "/api/cancel":
		var in struct {
			ID string `json:"id"`
		}
		if err = decode(r, &in); err == nil {
			err = a.Cancel(in.ID)
			result = map[string]bool{"requested": err == nil}
		}
	case "/api/retry":
		var in struct {
			ID string `json:"id"`
		}
		if err = decode(r, &in); err == nil {
			var j Job
			j, err = a.Job(in.ID)
			if err == nil {
				if j.Status == "running" || j.Status == "queued" {
					err = errors.New("active job cannot be retried")
				} else {
					j.Parent = j.ID
					j.Key = ""
					result, err = a.Submit(j)
				}
			}
		}
	case "/api/experiments":
		var x Experiment
		if err = decode(r, &x); err == nil {
			result, err = a.NewExperiment(x)
		}
	case "/api/experiment-control":
		var x struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		if err = decode(r, &x); err == nil {
			err = a.ControlExperiment(x.ID, x.Action)
			result = map[string]bool{"accepted": err == nil}
		}
	case "/api/experiment-export":
		var x struct {
			ID string `json:"id"`
		}
		if err = decode(r, &x); err == nil {
			result, err = a.experiment(x.ID)
		}
	case "/api/settings":
		var x struct {
			Language string `json:"language"`
		}
		if err = decode(r, &x); err == nil {
			if x.Language != "zh" && x.Language != "en" {
				err = errors.New("language must be zh or en")
			} else {
				err = a.Store.Update(func(s *State) error { s.Language = x.Language; return nil })
				result = x
			}
		}
	case "/api/backup":
		var name string
		name, err = a.Store.Backup()
		result = map[string]string{"name": name, "directory": a.Store.dir}
	case "/api/demo-seed":
		if a.Config.Mode != "demo" {
			err = errors.New("demo seed disabled in production")
			break
		}
		err = a.Store.Update(func(s *State) error {
			for _, p := range s.Projects {
				if p.ID == "demo-rc" {
					return nil
				}
			}
			s.Projects = append(s.Projects, Project{ID: "demo-rc", Name: "RC Filter · Demo", Root: a.Store.dir, Notes: "Analytical reference: f₋₃dB = 1/(2πRC). Synthetic; no PDK or EDA.", Tags: []string{"Demo", "RC"}})
			b := demoBatch(nil)
			b.ID = ID()
			b.Project = "demo-rc"
			b.Created = Now()
			s.Batches = append(s.Batches, b)
			return nil
		})
		if err == nil {
			err = a.seedWorkspace()
		}
		result = map[string]bool{"seeded": err == nil}
	default:
		err = fmt.Errorf("unknown endpoint %s", path)
	}
	reply(w, result, err)
}
func (a *App) Server() *http.Server {
	return &http.Server{Addr: a.Config.Listen, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
}
