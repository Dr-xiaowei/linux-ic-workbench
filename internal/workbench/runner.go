package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type App struct {
	changeSetMu sync.Mutex
	linuxFileMu sync.Mutex
	edaMu       sync.Mutex
	edaSessions map[string]*edaSession

	searchSlots chan struct{}

	demoStopped map[int]bool

	validationAgain bool
	loadedEnv       []string

	validation  Validation
	validating  bool
	agentRuns   map[string]*AgentRun
	monitorMu   sync.Mutex
	monitorLast MonitorSample

	Config      Config
	Store       *Store
	Token       string
	slots       chan struct{}
	mu          sync.Mutex
	cancels     map[string]context.CancelFunc
	experiments map[string]bool
	chatBusy    map[string]bool
	wg          sync.WaitGroup
	closing     bool
}

func NewApp(c Config) (*App, error) {
	if c.MaxWorkers < 1 || c.MaxWorkers > 8 {
		return nil, errors.New("maxWorkers must be 1..8")
	}
	if c.ModelTimeout < 1 || c.ModelTimeout > 3600 {
		return nil, errors.New("modelTimeoutSeconds must be 1..3600")
	}
	if c.MaxPreview < 1 || c.MaxPreview > 1<<20 || c.MaxRecords < 1 || c.MaxRecords > 10000 {
		return nil, errors.New("preview/record limits out of bounds")
	}
	if c.Timeout < 1 || c.Timeout > 86400 {
		return nil, errors.New("timeout out of range")
	}
	s, e := OpenStore(c.DataDir, c.Mode)
	if e != nil {
		return nil, e
	}
	a := &App{Config: c, Store: s, Token: ID() + ID(), slots: make(chan struct{}, c.MaxWorkers), cancels: map[string]context.CancelFunc{}, experiments: map[string]bool{}, chatBusy: map[string]bool{}}
	a.searchSlots = make(chan struct{}, 1)
	a.agentRuns = map[string]*AgentRun{}
	a.demoStopped = map[int]bool{}
	return a, nil
}
func (a *App) BeginShutdown() {
	a.mu.Lock()
	a.closing = true
	for _, cancel := range a.cancels {
		cancel()
	}
	a.mu.Unlock()
}
func (a *App) Close() {
	a.BeginShutdown()
	a.wg.Wait()
	a.Store.Update(func(s *State) error {
		for i := range s.Experiments {
			if s.Experiments[i].Status == "running" {
				s.Experiments[i].Status = "paused"
			}
		}
		return nil
	})
	a.closeEDA()
	a.Store.Close()
}
func (a *App) Submit(j Job) (Job, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return j, errors.New("server shutting down")
	}
	state := a.Store.Snapshot()
	p, e := FindProject(state, j.Project)
	if e != nil {
		return j, e
	}
	j.ProjectSnapshot = &p
	if j.Environment == "" {
		j.Environment = p.Environment
	}
	env := Environment{}
	if j.Environment != "" {
		found := false
		for _, v := range state.Environments {
			if v.ID == j.Environment {
				env = v
				found = true
			}
		}
		if !found {
			return j, errors.New("environment not found")
		}
	}
	j.EnvironmentSnapshot = &env
	if a.Config.Mode == "demo" {
		for k, v := range j.Params {
			if (k != "R" && k != "C") || v <= 0 || v > 1e9 || v < 1e-18 {
				return j, errors.New("demo accepts positive R/C only, 1e-18..1e9")
			}
		}
	}
	if j.Kind != "simulate" && j.Kind != "probe" && j.Kind != "launch" {
		return j, errors.New("unknown job kind")
	}
	for k, v := range j.Params {
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`).MatchString(k) || math.IsNaN(v) || math.IsInf(v, 0) {
			return j, errors.New("invalid parameter")
		}
	}
	j.ID = ID()
	j.Created = Now()
	j.Status = "queued"
	j.Error = ""
	j.Log = ""
	j.Batch = ""
	j.Finished = ""
	j.Evidence = ""
	existing := false
	e = a.Store.Update(func(s *State) error {
		if j.Key != "" {
			for _, old := range s.Jobs {
				if old.Key == j.Key && old.Project == j.Project {
					if old.Kind != j.Kind || StringJSON(old.Params) != StringJSON(j.Params) || old.Environment != j.Environment {
						return errors.New("idempotency key used for different request")
					}
					j = old
					existing = true
					return nil
				}
			}
		}
		if len(s.Jobs) >= a.Config.MaxRecords {
			return errors.New("job capacity reached")
		}
		pending := 0
		for _, x := range s.Jobs {
			if x.Status == "queued" || x.Status == "running" {
				pending++
			}
		}
		if pending >= 32 {
			return errors.New("queue full")
		}
		if a.Config.Mode == "production" && p.CadenceConfig != "" && j.Kind != "launch" {
			c, _, err := loadCadence(p.CadenceConfig)
			if err != nil {
				return err
			}
			if _, err = cadenceParameters(c, j.Params); err != nil {
				return err
			}
			frozen, err := snapshotCadence(p.CadenceConfig, filepath.Join(a.Store.dir, "jobs", j.ID))
			if err != nil {
				return err
			}
			frozenProject := p
			frozenProject.CadenceConfig = frozen
			j.ProjectSnapshot = &frozenProject
		}
		s.Jobs = append(s.Jobs, j)
		return nil
	})
	if e != nil {
		return j, e
	}
	if !existing {
		ctx, cancel := context.WithCancel(context.Background())
		a.cancels[j.ID] = cancel
		a.wg.Add(1)
		go a.run(ctx, j)
	}
	return j, nil
}
func (a *App) setJob(id string, fn func(*Job)) error {
	return a.Store.Update(func(s *State) error {
		for i := range s.Jobs {
			if s.Jobs[i].ID == id {
				fn(&s.Jobs[i])
				return nil
			}
		}
		return errors.New("job missing")
	})
}
func (a *App) Cancel(id string) error {
	a.mu.Lock()
	cancel, ok := a.cancels[id]
	a.mu.Unlock()
	if !ok {
		return errors.New("job is not active; outcome must be inspected")
	}
	cancel()
	return nil
}
func (a *App) Job(id string) (Job, error) {
	for _, j := range a.Store.Snapshot().Jobs {
		if j.ID == id {
			return j, nil
		}
	}
	return Job{}, errors.New("job not found")
}

type limitedBuffer struct {
	mu  sync.Mutex
	b   bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	left := b.max - b.b.Len()
	if left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		b.b.Write(p)
	}
	return n, nil
}
func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.b.String()
	if b.b.Len() >= b.max {
		s += "\n[output capped]"
	}
	return s
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func (a *App) command(ctx context.Context, c Command, env Environment, dir string, repl map[string]string) (string, error) {
	return a.commandOutput(ctx, c, env, dir, repl, nil)
}

// A separate bounded stdout sink keeps formatter diagnostics out of source text.
func (a *App) commandOutput(ctx context.Context, c Command, env Environment, dir string, repl map[string]string, stdout io.Writer) (string, error) {
	if !filepath.IsAbs(c.Executable) {
		return "", errors.New("configure absolute executable path")
	}
	if strings.ContainsAny(c.Executable+env.Setup, "\n\r\x00") {
		return "", errors.New("invalid executable/setup path")
	}
	args := append([]string{}, c.Args...)
	for i, arg := range args {
		for k, v := range repl {
			arg = strings.ReplaceAll(arg, "{"+k+"}", v)
		}
		if strings.ContainsAny(arg, "\n\r\x00") {
			return "", errors.New("multiline command arguments unsupported")
		}
		args[i] = arg
	}
	var cmd *exec.Cmd
	if env.Setup != "" {
		if !filepath.IsAbs(env.Setup) {
			return "", errors.New("setup path must be absolute")
		}
		shell := env.Shell
		if shell != "/bin/csh" && shell != "/bin/tcsh" && shell != "/bin/sh" && shell != "/bin/bash" {
			return "", errors.New("unsupported shell")
		}
		parts := []string{quote(c.Executable)}
		for _, v := range args {
			parts = append(parts, quote(v))
		}
		prefix := ". "
		flag := "-c"
		if shell == "/bin/csh" || shell == "/bin/tcsh" {
			prefix = "source "
			flag = "-fc"
		}
		cmd = exec.Command(shell, flag, prefix+quote(env.Setup)+" && exec "+strings.Join(parts, " "))
	} else {
		cmd = exec.Command(c.Executable, args...)
	}
	cmd.Dir = dir
	cmd.Env = os.Environ()
	a.mu.Lock()
	cmd.Env = append(cmd.Env, a.loadedEnv...)
	a.mu.Unlock()
	if env.ToolPath != "" {
		basePath := os.Getenv("PATH")
		for _, v := range cmd.Env {
			if strings.HasPrefix(v, "PATH=") {
				basePath = strings.TrimPrefix(v, "PATH=")
			}
		}
		cmd.Env = append(cmd.Env, "PATH="+env.ToolPath+":"+basePath)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	buf := &limitedBuffer{max: 65536}
	cmd.Stdout = buf
	if stdout != nil {
		cmd.Stdout = stdout
	}
	cmd.Stderr = buf
	if e := ctx.Err(); e != nil {
		return "", e
	}
	if e := cmd.Start(); e != nil {
		return "", e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case e := <-done:
		return buf.String(), e
	case <-ctx.Done():
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(time.Second):
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return buf.String(), ctx.Err()
	}
}
func (a *App) run(ctx context.Context, j Job) {
	defer a.wg.Done()
	defer func() { a.mu.Lock(); delete(a.cancels, j.ID); a.mu.Unlock() }()
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	case <-ctx.Done():
		a.setJob(j.ID, func(x *Job) { x.Status = "cancelled"; x.Finished = Now() })
		return
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(a.Config.Timeout)*time.Second)
	defer cancel()
	if err := a.setJob(j.ID, func(x *Job) { x.Status = "running" }); err != nil {
		stdlog.Printf("job %s not executed: persistence error: %v", j.ID, err)
		return
	}
	batch, evidence, log, e := a.execute(ctx, j)
	status := "completed"
	if e != nil {
		status = "failed"
		if errors.Is(e, context.Canceled) {
			status = "cancelled"
		}
		if errors.Is(e, context.DeadlineExceeded) {
			status = "timed_out"
		}
	}
	if e == nil && len(batch.Metrics) > 0 {
		batch.ID = ID()
		batch.Project = j.Project
		batch.Created = Now()
		batch.Parent = j.Parent
		e = a.Store.Update(func(s *State) error {
			if len(s.Batches) >= a.Config.MaxRecords {
				return errors.New("batch capacity reached")
			}
			s.Batches = append(s.Batches, batch)
			return nil
		})
		if e != nil {
			status = "failed"
		}
	}
	saveErr := a.setJob(j.ID, func(x *Job) {
		x.Status = status
		x.Finished = Now()
		x.Evidence = evidence
		x.Log = log
		if e != nil {
			x.Error = e.Error()
		} else {
			x.Batch = batch.ID
		}
	})
	if saveErr != nil {
		stdlog.Printf("job %s final state not persisted: %v; inspect artifacts", j.ID, saveErr)
	}
}
func (a *App) execute(ctx context.Context, j Job) (Batch, string, string, error) {
	var batch Batch
	p, e := FindProject(a.Store.Snapshot(), j.Project)
	if e != nil {
		return batch, "", "", e
	}
	if j.ProjectSnapshot != nil {
		p = *j.ProjectSnapshot
	}
	env := Environment{}
	if j.EnvironmentSnapshot != nil {
		env = *j.EnvironmentSnapshot
	}
	if a.Config.Mode == "demo" {
		select {
		case <-ctx.Done():
			return batch, "", "", ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		if j.Kind == "probe" {
			return batch, `{"demo":true,"tool":"Virtuoso","version":"SIMULATED","library":"demoLib","cell":"rc_filter","view":"schematic"}`, "[DEMO] Simulated bridge identity; no EDA connected", nil
		}
		if j.Kind == "launch" {
			return batch, "", "[DEMO] launch simulated", nil
		}
		return demoBatch(j.Params), "", "[DEMO] Analytical RC response, not Spectre", nil
	}
	dir := filepath.Join(a.Store.dir, "jobs", j.ID)
	if e = os.MkdirAll(dir, 0700); e != nil {
		return batch, "", "", e
	}
	request := map[string]any{"job": j, "project": p, "workDir": dir}
	b, _ := json.MarshalIndent(request, "", "  ")
	if e = atomicFile(filepath.Join(dir, "request.json"), b); e != nil {
		return batch, "", "", e
	}
	repl := map[string]string{"request": filepath.Join(dir, "request.json"), "workdir": dir, "project": p.Root}
	log := ""
	if p.CadenceConfig != "" && j.Kind != "launch" {
		return a.cadence(ctx, j, p, env, dir)
	}
	if j.Kind == "launch" {
		log, e = a.command(ctx, Command{Executable: env.Executable, Args: env.Args}, env, p.Root, repl)
		return batch, "", log, e
	}
	if j.Kind == "probe" {
		log, e = a.command(ctx, a.Config.Probe, env, dir, repl)
		if e != nil {
			return batch, "", log, e
		}
		raw, e := readBounded(filepath.Join(dir, "identity.json"), 65536)
		if e != nil {
			return batch, "", log, e
		}
		var identity map[string]any
		if e = json.Unmarshal(raw, &identity); e != nil {
			return batch, "", log, e
		}
		for _, key := range []string{"tool", "version", "library", "cell", "view"} {
			if v, ok := identity[key].(string); !ok || v == "" {
				return batch, "", log, fmt.Errorf("probe identity missing %s", key)
			}
		}
		if identity["tool"] != "Virtuoso" {
			return batch, "", log, errors.New("identity is not Virtuoso")
		}
		return batch, string(raw), log, nil
	}
	for _, step := range []struct {
		name string
		c    Command
	}{{"prepare", a.Config.Prepare}, {"simulate", a.Config.Simulate}, {"export", a.Config.Export}} {
		part, err := a.command(ctx, step.c, env, dir, repl)
		log += step.name + ":\n" + part + "\n"
		if err != nil {
			return batch, "", log, fmt.Errorf("%s: %w", step.name, err)
		}
	}
	raw, e := readBounded(filepath.Join(dir, "metrics.csv"), 4<<20)
	if e != nil {
		return batch, "", log, e
	}
	ms, e := ParseCSV(string(raw), "job:"+j.ID+"/metrics.csv")
	if e != nil {
		return batch, "", log, e
	}
	meta, e := readBounded(filepath.Join(dir, "result.json"), 65536)
	if e != nil {
		return batch, "", log, errors.New("export must write result.json with design and definition")
	}
	if e = json.Unmarshal(meta, &batch); e != nil {
		return batch, "", log, e
	}
	if batch.Design == "" || batch.Definition == "" {
		return batch, "", log, errors.New("result missing design/measurement version")
	}
	batch.Metrics = ms
	batch.Name = "Run " + j.ID[:8]
	batch.Provenance = "Configured EDA adapter; verify recorded tool/model versions: " + string(meta)
	return batch, "", log, nil
}
func readBounded(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("file exceeds size limit")
	}
	return b, e
}
func demoBatch(params map[string]float64) Batch {
	r, c := 1000.0, 1e-9
	if x, ok := params["R"]; ok {
		r = x
	}
	if x, ok := params["C"]; ok {
		c = x
	}
	ms := []Metric{}
	for _, corner := range []struct {
		name   string
		factor float64
	}{{"nominal", 1}, {"R+5%", 1.05}} {
		v := 1 / (2 * math.Pi * r * corner.factor * c)
		min, max := 150000.0, 170000.0
		ms = append(ms, Metric{Point: "ac", Corner: corner.name, Name: "bandwidth", Value: &v, Unit: "Hz", Status: "ok", Min: &min, Max: &max, Source: "DEMO analytic: 1/(2*pi*R*C)"})
	}
	return Batch{Name: "DEMO RC low-pass", Design: "analytical-rc-v1", Definition: "rc-bandwidth-v1", Provenance: "DEMO/SYNTHETIC — independently checkable RC formula, not commercial EDA", Metrics: ms}
}
