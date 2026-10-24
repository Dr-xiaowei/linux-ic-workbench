package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MCPInput struct {
	Project string             `json:"project,omitempty"`
	Job     string             `json:"job,omitempty"`
	Batch   string             `json:"batch,omitempty"`
	Params  map[string]float64 `json:"params,omitempty"`
	Key     string             `json:"key,omitempty"`
}
type MCPOutput struct {
	Data any `json:"data"`
}

func MCPServer(endpoint, token string, allowRun bool) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "ic-workbench", Version: Version}, nil)
	for _, name := range []string{"workspace_state", "job_status", "read_batch", "probe_virtuoso", "submit_simulation", "cancel_job"} {
		name := name
		if !allowRun && name != "workspace_state" && name != "job_status" && name != "read_batch" {
			continue
		}
		mcp.AddTool(s, &mcp.Tool{Name: name, Description: "IC workbench " + name + ". Uses configured project scope and audited adapter; no arbitrary shell or SKILL."}, func(ctx context.Context, req *mcp.CallToolRequest, in MCPInput) (*mcp.CallToolResult, MCPOutput, error) {
			out := MCPOutput{}
			u, e := url.Parse(endpoint)
			if e != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
				return nil, out, errors.New("MCP proxy requires a loopback workbench endpoint")
			}
			method, path := http.MethodGet, "/api/state"
			var body any
			switch name {
			case "read_batch":
				path = "/api/batch?id=" + url.QueryEscape(in.Batch)
			case "job_status":
				path = "/api/job?id=" + url.QueryEscape(in.Job)
			case "probe_virtuoso", "submit_simulation":
				method = "POST"
				path = "/api/jobs"
				kind := "probe"
				if name == "submit_simulation" {
					kind = "simulate"
				}
				body = map[string]any{"project": in.Project, "kind": kind, "params": in.Params, "key": in.Key}
			case "cancel_job":
				method = "POST"
				path = "/api/cancel"
				body = map[string]string{"id": in.Job}
			}
			b, _ := json.Marshal(body)
			request, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(endpoint, "/")+path, bytes.NewReader(b))
			if e != nil {
				return nil, out, e
			}
			request.Header.Set("Authorization", "Bearer "+token)
			request.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect disabled") }}
			res, e := client.Do(request)
			if e != nil {
				return nil, out, e
			}
			defer res.Body.Close()
			data, e := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
			if e != nil {
				return nil, out, e
			}
			if len(data) > 4<<20 {
				return nil, out, errors.New("response limit")
			}
			if res.StatusCode != 200 {
				return nil, out, fmt.Errorf("workbench HTTP %d: %s", res.StatusCode, string(data))
			}
			if e = json.Unmarshal(data, &out.Data); e != nil {
				return nil, out, e
			}
			return nil, out, nil
		})
	}
	return s
}
