package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func (a *App) validateFolder(folder string) (string, error) {
	if !filepath.IsAbs(folder) {
		return "", errors.New("absolute folder path required")
	}
	real, e := filepath.EvalSymlinks(folder)
	if e != nil {
		return "", e
	}
	f, e := os.Open(real)
	if e != nil {
		return "", e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.IsDir() {
		return "", errors.New("readable directory required")
	}
	return real, nil
}
func (a *App) scopedAgentAction(ctx context.Context, s Session, action string, b json.RawMessage) (any, error) {
	// File-set operations must stay in the explicitly selected Agent directory.
	if action == "propose-change-set" || action == "apply-change-file" || action == "undo-change-file" {
		root, e := a.linuxRoot(s)
		if e != nil {
			return nil, e
		}
		root, e = a.changeRoot(root)
		if e != nil {
			return nil, e
		}
		var fields map[string]json.RawMessage
		if e = json.Unmarshal(b, &fields); e != nil {
			return nil, e
		}
		if action == "propose-change-set" {
			var requested string
			json.Unmarshal(fields["root"], &requested)
			if requested != "" && requested != root {
				return nil, errors.New("change set outside selected scope")
			}
			fields["root"] = json.RawMessage(StringJSON(root))
			b = json.RawMessage(StringJSON(fields))
		} else {
			var id string
			json.Unmarshal(fields["id"], &id)
			found := false
			for _, c := range a.Store.Snapshot().ChangeSets {
				if c.ID == id {
					found = true
					if c.Root != root {
						return nil, errors.New("change set outside selected scope")
					}
				}
			}
			if !found {
				return nil, errors.New("change set missing")
			}
		}
	}
	var in map[string]any
	if e := json.Unmarshal(b, &in); e != nil {
		return nil, e
	}
	if in == nil {
		in = map[string]any{}
	}
	// This is an operation context, not an OS sandbox. External executable tools
	// retain the user's permissions and always require a visible approval.
	if s.Project != "" {
		if p, ok := in["project"].(string); ok && p != "" && p != s.Project {
			return nil, errors.New("action outside selected project")
		}
		switch action {
		case "browse", "search", "note", "analysis", "save-code", "save-hdl", "simulate", "optimize", "import-results", "launch-software", "run-plugin":
			in["project"] = s.Project
		}
		if action == "mcp-call" && in["id"] == "builtin" {
			args, _ := in["arguments"].(map[string]any)
			if args == nil {
				args = map[string]any{}
			}
			if p, _ := args["project"].(string); p != "" && p != s.Project {
				return nil, errors.New("MCP request outside selected project")
			}
			if in["tool"] == "probe_virtuoso" || in["tool"] == "submit_simulation" {
				args["project"] = s.Project
			}
			in["arguments"] = args
		}
	}
	if s.Folder != "" {
		get := func(k string) string { v, _ := in[k].(string); return v }
		if get("project") != "" {
			return nil, errors.New("folder scope cannot target a registered project; switch scope first")
		}
		switch action {
		case "workspace":
			return map[string]any{"folder": s.Folder, "projects": []Project{}, "mcpConnections": a.Store.Metadata().MCPConnections}, nil
		case "browse":
			offset, _ := in["offset"].(float64)
			return a.browseRoot("", s.Folder, get("path"), int(offset))
		case "search":
			return a.search(context.WithValue(ctx, agentScopeKey{}, s), "", get("query"), get("cursor"))
		case "analysis":
			target, e := SafePath(s.Folder, get("path"))
			if e != nil {
				return nil, e
			}
			info, e := os.Stat(target)
			if e != nil || !info.Mode().IsRegular() {
				return nil, errors.New("regular file required")
			}
			raw, e := readBounded(target, 2<<20)
			if e != nil {
				return nil, e
			}
			return a.analyze(json.RawMessage(StringJSON(AnalysisInput{Name: filepath.Base(target), Content: string(raw)})))
		case "save-code", "save-hdl":
			var x HDLInput
			if e := parseInput(b, &x); e != nil {
				return nil, e
			}
			return a.saveCodeAtRoot(s.Folder, x)
		case "note", "save-project", "simulate", "optimize", "import-results", "launch-software", "run-plugin":
			return nil, errors.New("this action needs a project scope or its dedicated workspace")
		}
	}
	if action == "workspace" && s.Project != "" {
		v := a.Store.Metadata()
		p, e := FindProject(v, s.Project)
		if e != nil {
			return nil, e
		}
		return map[string]any{"projects": []Project{p}, "mcpConnections": v.MCPConnections}, nil
	}
	return a.action(ctx, action, json.RawMessage(StringJSON(in)))
}
