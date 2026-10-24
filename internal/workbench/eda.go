package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EDAConnection struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Transport    string   `json:"transport"`
	Command      Command  `json:"command"`
	Directory    string   `json:"directory"`
	URL          string   `json:"url"`
	TokenEnv     string   `json:"tokenEnv"`
	AllowedTools []string `json:"allowedTools"`
	Timeout      int      `json:"timeoutSeconds"`
}
type edaProgressKey struct{}

type edaSession struct {
	progressMu     sync.Mutex
	progressToken  string
	progress       func(*mcp.ProgressNotificationParams)
	mu             sync.Mutex
	client         *mcp.ClientSession
	cleanup        func()
	signature      string
	lifetimeCancel context.CancelFunc
}

func (e *edaSession) lock(ctx context.Context) error {
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		if e.mu.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
func (e *edaSession) close() {
	if e.lifetimeCancel != nil {
		timer := time.AfterFunc(2*time.Second, e.lifetimeCancel)
		defer timer.Stop()
		defer e.lifetimeCancel()
	}
	if e.client != nil {
		e.client.Close()
		e.client = nil
	}
	if e.cleanup != nil {
		e.cleanup()
		e.cleanup = nil
	}
}
func (a *App) edaEntry(id string) *edaSession {
	a.edaMu.Lock()
	defer a.edaMu.Unlock()
	if a.edaSessions == nil {
		a.edaSessions = map[string]*edaSession{}
	}
	if a.edaSessions[id] == nil {
		a.edaSessions[id] = &edaSession{}
	}
	return a.edaSessions[id]
}
func (a *App) closeEDA() {
	a.edaMu.Lock()
	defer a.edaMu.Unlock()
	for _, e := range a.edaSessions {
		e.mu.Lock()
		e.close()
		e.mu.Unlock()
	}
}
func (a *App) edaConfig(id string) (EDAConnection, error) {
	if id == "builtin" {
		return EDAConnection{ID: "builtin", Name: "Virtuoso / Spectre", Transport: "builtin", Timeout: 30}, nil
	}
	for _, c := range a.Store.Metadata().MCPConnections {
		if c.ID == id {
			return c, nil
		}
	}
	return EDAConnection{}, errors.New("MCP connection not found")
}
func validateEDA(c EDAConnection) error {
	if c.Name == "" || len(c.Name) > 120 || len(c.ID) > 100 || c.ID == "builtin" || c.Timeout < 1 || c.Timeout > 600 || len(c.AllowedTools) > 128 {
		return errors.New("name, timeout 1..600 and max 128 allowed tools required")
	}
	for _, tool := range c.AllowedTools {
		if tool == "" || len(tool) > 200 {
			return errors.New("invalid allowed tool")
		}
	}
	if c.TokenEnv != "" && !calcName.MatchString(c.TokenEnv) {
		return errors.New("invalid token environment variable")
	}
	switch c.Transport {
	case "stdio":
		if !filepath.IsAbs(c.Command.Executable) || len(c.Command.Args) > 64 || !filepath.IsAbs(c.Directory) {
			return errors.New("absolute executable/work directory and max 64 arguments required")
		}
	case "http":
		u, e := url.Parse(c.URL)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("HTTP(S) MCP URL required; keep credentials in token environment variable")
		}
	default:
		return errors.New("transport must be stdio or http")
	}
	return nil
}

type edaAuthTransport struct {
	base  http.RoundTripper
	token string
}

func (t edaAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if t.token != "" {
		r.Header.Set("Authorization", "Bearer "+t.token)
	}
	res, e := t.base.RoundTrip(r)
	if e == nil {
		res.Body = &edaLimitedBody{ReadCloser: res.Body, remaining: 4 << 20}
	}
	return res, e
}

type edaLimitedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *edaLimitedBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, errors.New("MCP response exceeds 4 MiB")
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, e := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, e
}
func (a *App) connectEDA(ctx context.Context, c EDAConnection, e *edaSession) error {
	signature := StringJSON(c)
	if e.client != nil && e.signature == signature {
		return nil
	}
	e.close()
	life, cancelLife := context.WithCancel(context.Background())
	e.lifetimeCancel = cancelLife
	stopInit := context.AfterFunc(ctx, cancelLife)
	defer stopInit()
	connected := false
	defer func() {
		if !connected {
			e.close()
		}
	}()
	var transport mcp.Transport
	if c.Transport == "builtin" || a.Config.Mode == "demo" {
		server := a.localEDAServer()
		ct, st := mcp.NewInMemoryTransports()
		ss, err := server.Connect(life, st, nil)
		if err != nil {
			return err
		}
		e.cleanup = func() { ss.Close() }
		transport = ct
	} else if c.Transport == "stdio" {
		// Tie an unresponsive initialize handshake to the connection lifetime.
		// The SDK cannot finish closing a session while a silent child holds its pipe.
		cmd := exec.CommandContext(life, c.Command.Executable, c.Command.Args...)
		cmd.WaitDelay = 2 * time.Second
		cmd.Dir = c.Directory
		cmd.Stderr = io.Discard
		a.mu.Lock()
		env := append([]string{}, a.loadedEnv...)
		a.mu.Unlock()
		if len(env) == 0 {
			env = os.Environ()
		}
		prefs := a.preferences()
		for _, v := range env {
			k := strings.SplitN(v, "=", 2)[0]
			if k != prefs.KeyEnv && k != a.Config.KeyEnv && k != c.TokenEnv {
				cmd.Env = append(cmd.Env, v)
			}
		}
		transport = &mcp.CommandTransport{Command: cmd, TerminateDuration: 500 * time.Millisecond}
	} else {
		token := ""
		if c.TokenEnv != "" {
			token = os.Getenv(c.TokenEnv)
			if token == "" {
				return errors.New("MCP token environment variable is unset")
			}
		}
		client := &http.Client{Transport: edaAuthTransport{http.DefaultTransport, token}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("MCP redirect disabled") }}
		transport = &mcp.StreamableClientTransport{Endpoint: c.URL, HTTPClient: client, MaxRetries: -1}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "ic-workbench-eda", Version: Version}, &mcp.ClientOptions{ProgressNotificationHandler: func(ctx context.Context, r *mcp.ProgressNotificationClientRequest) {
		e.progressMu.Lock()
		fn := e.progress
		match := e.progressToken != "" && fmt.Sprint(r.Params.ProgressToken) == e.progressToken
		e.progressMu.Unlock()
		if match && fn != nil {
			fn(r.Params)
		}
	}})
	cs, err := client.Connect(life, transport, nil)
	if err != nil {
		e.close()
		return err
	}
	e.client = cs
	e.signature = signature
	connected = true
	return nil
}

// The built-in bridge also traverses the official MCP protocol. Production
// handlers submit to the existing audited Cadence adapter, while Demo uses its
// explicitly labelled analytical RC engine.
func (a *App) localEDAServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "ic-workbench-cadence-bridge", Version: Version}, nil)
	for _, name := range []string{"list_projects", "probe_virtuoso", "submit_simulation", "job_status", "cancel_job"} {
		name := name
		mcp.AddTool(s, &mcp.Tool{Name: name, Description: map[string]string{"list_projects": "List project IDs and available Cadence configuration. Does not launch EDA.", "probe_virtuoso": "Verify configured Virtuoso/Spectre adapter. Returns a job ID; poll job_status.", "submit_simulation": "Submit a simulation through configured Cadence adapter. Returns a job ID; poll job_status.", "job_status": "Read job state/log; queued is not completed.", "cancel_job": "Request cancellation of the specified job."}[name]}, func(ctx context.Context, req *mcp.CallToolRequest, in MCPInput) (*mcp.CallToolResult, MCPOutput, error) {
			var out any
			var err error
			switch name {
			case "list_projects":
				out = a.Store.Metadata().Projects
			case "probe_virtuoso", "submit_simulation":
				kind := "probe"
				if name == "submit_simulation" {
					kind = "simulate"
				}
				out, err = a.Submit(Job{Project: in.Project, Kind: kind, Params: in.Params, Key: in.Key})
			case "job_status":
				out, err = a.Job(in.Job)
			case "cancel_job":
				err = a.Cancel(in.Job)
				out = map[string]bool{"requested": err == nil}
			}
			return nil, MCPOutput{Data: out}, err
		})
	}
	return s
}
func (a *App) edaTools(ctx context.Context, c EDAConnection) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	entry := a.edaEntry(c.ID)
	if err := entry.lock(ctx); err != nil {
		return nil, err
	}
	defer entry.mu.Unlock()
	if e := a.connectEDA(ctx, c, entry); e != nil {
		return nil, e
	}
	list := []*mcp.Tool{}
	cursor := ""
	for i := 0; i < 8; i++ {
		r, e := entry.client.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if e != nil {
			entry.close()
			return nil, e
		}
		list = append(list, r.Tools...)
		if len(list) > 128 || len(StringJSON(list)) > 500000 {
			return nil, errors.New("MCP tool catalog limit")
		}
		if r.NextCursor == "" {
			return map[string]any{"tools": list, "demo": a.Config.Mode == "demo", "connected": true}, nil
		}
		cursor = r.NextCursor
	}
	return nil, errors.New("MCP tool pagination limit")
}
func (a *App) edaCall(ctx context.Context, c EDAConnection, tool string, args map[string]any) (string, error) {
	if c.ID != "builtin" {
		allowed := false
		for _, n := range c.AllowedTools {
			if tool == n {
				allowed = true
			}
		}
		if !allowed {
			return "", errors.New("tool is not in this connection's allowedTools")
		}
	}
	entry := a.edaEntry(c.ID)
	if err := entry.lock(ctx); err != nil {
		return "", err
	}
	defer entry.mu.Unlock()
	if e := a.connectEDA(ctx, c, entry); e != nil {
		return "", e
	}
	// SDK v1.0.0 SetProgressToken needs an initialized metadata map.
	params := &mcp.CallToolParams{Meta: mcp.Meta{}, Name: tool, Arguments: args}
	if fn, ok := ctx.Value(edaProgressKey{}).(func(*mcp.ProgressNotificationParams)); ok {
		t := ID()
		params.SetProgressToken(t)
		entry.progressMu.Lock()
		entry.progressToken = t
		entry.progress = fn
		entry.progressMu.Unlock()
		defer func() {
			entry.progressMu.Lock()
			entry.progressToken = ""
			entry.progress = nil
			entry.progressMu.Unlock()
		}()
	}
	r, e := entry.client.CallTool(ctx, params)
	if e != nil {
		entry.close()
		return "", e
	}
	raw := StringJSON(r)
	if len(raw) > 1<<20 {
		raw = raw[:1<<20] + "\n[OUTPUT TRUNCATED]"
	}
	if r.IsError {
		return raw, errors.New("MCP tool reported an error; inspect its output")
	}
	return raw, nil
}
func (a *App) edaAction(ctx context.Context, action string, b json.RawMessage) (any, error) {
	if action == "save-mcp" {
		var c EDAConnection
		if e := parseInput(b, &c); e != nil {
			return nil, e
		}
		if c.ID == "" {
			c.ID = ID()
		}
		if e := validateEDA(c); e != nil {
			return nil, e
		}
		e := a.Store.Update(func(s *State) error {
			for i, x := range s.MCPConnections {
				if x.ID == c.ID {
					s.MCPConnections[i] = c
					return nil
				}
			}
			if len(s.MCPConnections) >= 20 {
				return errors.New("max 20 MCP connections")
			}
			s.MCPConnections = append(s.MCPConnections, c)
			return nil
		})
		return c, e
	}
	var in struct {
		ID        string         `json:"id"`
		Tool      string         `json:"tool"`
		Arguments map[string]any `json:"arguments"`
	}
	if e := parseInput(b, &in); e != nil {
		return nil, e
	}
	c, e := a.edaConfig(in.ID)
	if e != nil {
		return nil, e
	}
	if action == "disconnect-mcp" || action == "remove-mcp" {
		entry := a.edaEntry(c.ID)
		entry.mu.Lock()
		entry.close()
		entry.mu.Unlock()
		if action == "remove-mcp" {
			if c.ID == "builtin" {
				return nil, errors.New("built-in bridge cannot be removed")
			}
			e = a.Store.Update(func(s *State) error {
				out := []EDAConnection{}
				for _, x := range s.MCPConnections {
					if x.ID != c.ID {
						out = append(out, x)
					}
				}
				s.MCPConnections = out
				return nil
			})
		}
		return map[string]bool{"disconnected": true}, e
	}
	if action == "mcp-tools" {
		return a.edaTools(ctx, c)
	}
	if in.Tool == "" || len(StringJSON(in.Arguments)) > 100000 {
		return nil, errors.New("tool required; arguments max 100 KB")
	}
	return a.recordMCP(c, in.Tool, in.Arguments)
}
func (a *App) recordMCP(c EDAConnection, tool string, args map[string]any) (Operation, error) {
	return a.recordMCPContext(context.Background(), c, tool, args)
}
func (a *App) recordMCPContext(parent context.Context, c EDAConnection, tool string, args map[string]any) (Operation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return Operation{}, errors.New("shutting down")
	}
	active := 0
	for _, o := range a.Store.Metadata().Operations {
		if o.Status == "queued" || o.Status == "running" {
			active++
		}
	}
	if active >= 8 {
		return Operation{}, errors.New("operation queue full")
	}
	op := Operation{Connection: c.ID, ID: ID(), Kind: "mcp", Name: c.Name + " · " + tool, Created: Now(), Status: "queued"}
	if e := a.Store.Update(func(s *State) error {
		if len(s.Operations) >= 10000 {
			return errors.New("operation history limit")
		}
		s.Operations = append(s.Operations, op)
		return nil
	}); e != nil {
		return op, e
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(c.Timeout)*time.Second)
	a.cancels["op:"+op.ID] = cancel
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer cancel()
		defer func() { a.mu.Lock(); delete(a.cancels, "op:"+op.ID); a.mu.Unlock() }()
		update := func(status, log string, err error) {
			a.Store.Update(func(s *State) error {
				for i := range s.Operations {
					if s.Operations[i].ID == op.ID {
						o := &s.Operations[i]
						o.Status = status
						o.Log = log
						if err != nil {
							o.Error = err.Error()
						}
						if status != "running" {
							o.Finished = Now()
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
		update("running", "MCP tools/call · "+tool, nil)
		log, err := a.edaCall(ctx, c, tool, args)
		status := "succeeded"
		if err != nil {
			status = "failed"
		}
		if ctx.Err() != nil {
			status = "cancelled"
			err = ctx.Err()
		}
		label := ""
		if a.Config.Mode == "demo" {
			label = "DEMO: local MCP protocol + simulated EDA; no external connection.\n"
		}
		update(status, label+fmt.Sprintf("tools/call %s\nArguments: %s\n", tool, StringJSON(args))+log, err)
	}()
	return op, nil
}
