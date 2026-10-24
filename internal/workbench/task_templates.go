package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type TaskParameter struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Default string `json:"default"`
}
type TaskTemplate struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Prompt     string          `json:"prompt"`
	Parameters []TaskParameter `json:"parameters"`
}

var templateVariable = regexp.MustCompile(`\{\{([a-zA-Z][a-zA-Z0-9_]{0,31})\}\}`)
var templateName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,31}$`)

func builtinTemplates() []TaskTemplate {
	return []TaskTemplate{
		{"log-review", "日志排查 / Log review", "只读分析 {{input}}，重点关注 {{focus}}。步骤：先读取文件，引用文件及行号，区分事实、假设和待验证步骤。无法读取时停止，不声称已完成。停止条件：{{stop}}。", []TaskParameter{{"input", "输入日志 / Input log", "spectre.out"}, {"focus", "分析重点 / Focus", "收敛错误 / Convergence errors"}, {"stop", "停止条件 / Stop condition", "给出三条有证据的建议 / Three evidenced suggestions"}}},
		{"csv-review", "指标整理 / Metric review", "只读分析 {{input}}。核对指标 {{metrics}} 的单位、corner、电源和温度；不同工况不得直接比较。只输出整理表，不写入文件。缺少工况时标记未核实。", []TaskParameter{{"input", "输入 CSV / Input CSV", "results.csv"}, {"metrics", "目标指标 / Metrics", "gain, bandwidth, power"}}},
		{"script-review", "脚本修改建议 / Script review", "读取 {{input}}，按 {{goal}} 给出修改方案和验证命令。仅在对话中展示候选，不改原文件，不执行检查工具。建议输出位置：{{output}}。实际采用请在 Code 多文件修改集中审阅。", []TaskParameter{{"input", "输入脚本 / Input script", "run.tcl"}, {"goal", "修改目标 / Goal", "移除硬编码路径 / Remove hardcoded paths"}, {"output", "建议输出 / Suggested output", "reviewed/run.tcl"}}},
	}
}
func validateTemplate(t TaskTemplate) error {
	if strings.TrimSpace(t.Name) == "" || len(t.Name) > 160 || len(t.Prompt) > 12000 || len(t.Parameters) > 12 || len(t.Prompt) == 0 {
		return errors.New("template name/prompt required; max 12 parameters / 模板需名称、提示词，最多 12 个参数")
	}
	seen := map[string]bool{}
	for _, p := range t.Parameters {
		if !templateName.MatchString(p.Name) || seen[p.Name] || len(p.Label) > 200 || len(p.Default) > 2000 {
			return errors.New("invalid or duplicate template parameter / 参数名无效或重复")
		}
		seen[p.Name] = true
	}
	used := map[string]bool{}
	for _, m := range templateVariable.FindAllStringSubmatch(t.Prompt, -1) {
		if !seen[m[1]] {
			return fmt.Errorf("undefined parameter: %s", m[1])
		}
		used[m[1]] = true
	}
	if strings.Contains(templateVariable.ReplaceAllString(t.Prompt, ""), "{{") || strings.Contains(templateVariable.ReplaceAllString(t.Prompt, ""), "}}") {
		return errors.New("invalid {{parameter}} syntax")
	}
	for k := range seen {
		if !used[k] {
			return fmt.Errorf("unused parameter: %s", k)
		}
	}
	return nil
}
func (a *App) templates() []TaskTemplate {
	return append(builtinTemplates(), a.Store.Snapshot().TaskTemplates...)
}
func (a *App) templateAction(action string, raw json.RawMessage) (any, error) {
	if action == "task-templates" {
		return a.templates(), nil
	}
	if action == "save-task-template" {
		var t TaskTemplate
		if e := parseInput(raw, &t); e != nil {
			return nil, e
		}
		if e := validateTemplate(t); e != nil {
			return nil, e
		}
		for _, b := range builtinTemplates() {
			if t.ID == b.ID {
				return nil, errors.New("save a built-in template as a new personal copy")
			}
		}
		if t.ID == "" {
			t.ID = ID()
		}
		e := a.Store.Update(func(s *State) error {
			for i, v := range s.TaskTemplates {
				if v.ID == t.ID {
					s.TaskTemplates[i] = t
					return nil
				}
			}
			if len(s.TaskTemplates) >= 50 {
				return errors.New("50 personal templates maximum")
			}
			s.TaskTemplates = append(s.TaskTemplates, t)
			return nil
		})
		return t, e
	}
	var in struct {
		ID         string            `json:"id"`
		Values     map[string]string `json:"values"`
		MaxRounds  int               `json:"maxRounds"`
		MaxSeconds int               `json:"maxSeconds"`
	}
	if e := parseInput(raw, &in); e != nil {
		return nil, e
	}
	if in.MaxRounds < 1 || in.MaxRounds > 12 || in.MaxSeconds < 10 || in.MaxSeconds > 600 {
		return nil, errors.New("budget: 1–12 rounds and 10–600 seconds / 预算：1–12 轮，10–600 秒")
	}
	for _, t := range a.templates() {
		if t.ID != in.ID {
			continue
		}
		if e := validateTemplate(t); e != nil {
			return nil, e
		}
		values := map[string]string{}
		for _, p := range t.Parameters {
			v, ok := in.Values[p.Name]
			if !ok {
				v = p.Default
			}
			if strings.TrimSpace(v) == "" || len(v) > 2000 {
				return nil, fmt.Errorf("parameter required, max 2000 bytes: %s", p.Name)
			}
			values[p.Name] = v
		}
		for k := range in.Values {
			if _, ok := values[k]; !ok {
				return nil, fmt.Errorf("unknown parameter: %s", k)
			}
		}
		// One pass: values containing template syntax are data, never expanded again.
		prompt := templateVariable.ReplaceAllStringFunc(t.Prompt, func(m string) string { return values[m[2:len(m)-2]] })
		if len(prompt) > 20000 {
			return nil, errors.New("expanded task exceeds 20 KB")
		}
		return map[string]any{"text": prompt, "maxRounds": in.MaxRounds, "maxSeconds": in.MaxSeconds, "steps": []string{"核对当前 AI 操作范围 / Review current AI scope", "只读工具获取证据 / Read evidence", "给出结果与待验证事项 / Report evidence and limitations"}}, nil
	}
	return nil, errors.New("template not found")
}

// A template's explicit input is acquired by deterministic code, not guessed by
// the model. Source files are bounded, read-only, and stay in the chosen scope.
func (a *App) prepareTemplate(in *AgentInput, session Session) error {
	if in.TemplateID == "" {
		return nil
	}
	if _, e := a.templateAction("preview-task-template", json.RawMessage(StringJSON(map[string]any{"id": in.TemplateID, "values": in.TemplateValues, "maxRounds": in.MaxRounds, "maxSeconds": in.MaxSeconds}))); e != nil {
		return e
	}
	in.ReadOnly = true
	in.AllowWrite = false
	in.LinuxMode = "off"
	if a.Config.Mode == "demo" {
		in.Context = "DEMO · Synthetic template input; no host file was read."
		return nil
	}
	var input string
	for _, t := range a.templates() {
		if t.ID == in.TemplateID {
			for _, p := range t.Parameters {
				if p.Name == "input" {
					input = p.Default
					if v, ok := in.TemplateValues["input"]; ok {
						input = v
					}
				}
			}
		}
	}
	if input == "" {
		return nil
	}
	if session.Project == "" && session.Folder == "" {
		return errors.New("模板读取文件前，请在 AI 对话选择项目或文件夹 / Select a project or folder before reading template input")
	}
	root, e := a.linuxRoot(session)
	if e != nil {
		return e
	}
	p, e := a.linuxPath(root, input, false)
	if e != nil {
		return e
	}
	st, e := os.Stat(p)
	if e != nil || !st.Mode().IsRegular() {
		return errors.New("template input must be a regular file")
	}
	b, e := readBounded(p, 100000)
	if e != nil {
		return e
	}
	if !validChangeText(string(b)) {
		return errors.New("template input must be UTF-8 text")
	}
	in.Context = "Template input / 模板输入: " + p + "\nSHA256: " + hashContent(b) + "\n<untrusted_file>\n" + string(b) + "\n</untrusted_file>"
	return nil
}

func textualToolRequest(text string) bool {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = strings.TrimSpace(strings.TrimSuffix(text[i+1:], "```"))
		}
	}
	var data map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &data) != nil {
		return false
	}
	var name string
	json.Unmarshal(data["name"], &name)
	if name == "platform_action" || name == "linux_action" {
		return true
	}
	if _, ok := data["tool_calls"]; ok {
		return true
	}
	return false
}
