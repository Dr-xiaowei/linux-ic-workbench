package workbench

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

type AnalysisInput struct {
	Project string `json:"project"`
	Root    string `json:"root"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"`
}
type WavePoint struct {
	T   float64  `json:"t"`
	V   *float64 `json:"v"`
	Raw string   `json:"raw,omitempty"`
}
type WaveSignal struct {
	Name    string      `json:"name"`
	Digital bool        `json:"digital"`
	Points  []WavePoint `json:"points"`
}
type Analysis struct {
	Name        string       `json:"name"`
	Text        string       `json:"text"`
	Format      string       `json:"format"`
	Timescale   string       `json:"timescale"`
	Signals     []WaveSignal `json:"signals"`
	Limited     bool         `json:"limited"`
	Diagnostics any          `json:"diagnostics"`
}

func finiteNumber(s string) (float64, error) {
	v, e := strconv.ParseFloat(s, 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("finite number required")
	}
	return v, nil
}
func (a *App) analyze(raw json.RawMessage) (any, error) {
	var x AnalysisInput
	if e := parseInput(raw, &x); e != nil {
		return nil, e
	}
	limited := false
	if x.Path != "" {
		root, e := a.fileRoot(x.Project, x.Root)
		if e != nil {
			return nil, e
		}
		path, e := SafePath(root, x.Path)
		if e != nil {
			return nil, e
		}
		info, e := os.Stat(path)
		if e != nil {
			return nil, e
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("only regular files can be analyzed")
		}
		b, e := readBounded(path, 2<<20)
		if e != nil {
			return nil, e
		}
		limited = info.Size() > int64(len(b))
		x.Content = string(b)
		x.Name = filepath.Base(path)
	}
	if len(x.Content) > 2<<20 {
		return nil, errors.New("import limit 2 MiB; export a selected interval first")
	}
	if !utf8.ValidString(x.Content) {
		return nil, errors.New("文件不是有效 UTF-8 文本，请按原编码转换后再导入 / Invalid UTF-8; convert using the source encoding first")
	}
	if strings.Contains(x.Content, "\x00") {
		return nil, errors.New("proprietary/binary waveform: export CSV or VCD using Virtuoso/OCEAN, VCS/Verdi or vendor utilities")
	}
	ext := strings.ToLower(filepath.Ext(x.Name))
	if ext == ".fsdb" || ext == ".psf" || ext == ".wlf" || ext == ".shm" {
		return nil, errors.New("vendor binary requires CSV/VCD export; decoding is not claimed")
	}
	out := Analysis{Name: x.Name, Text: x.Content, Format: "text", Timescale: "time", Signals: []WaveSignal{}, Limited: limited, Diagnostics: Diagnose(x.Content)}
	if ext == ".vcd" {
		out.Format = "VCD"
		if e := parseVCD(x.Content, &out); e != nil {
			return nil, e
		}
	} else if ext == ".csv" {
		out.Format = "CSV"
		reader := csv.NewReader(strings.NewReader(x.Content))
		_, headerError := reader.Read()
		row, rowError := reader.Read()
		if headerError != nil {
			return nil, headerError
		}
		if rowError != nil && rowError != io.EOF {
			return nil, rowError
		}
		numeric := false
		if len(row) > 0 {
			_, e := finiteNumber(row[0])
			numeric = e == nil
		}
		if numeric {
			if e := parseWaveCSV(x.Content, &out); e != nil {
				return nil, e
			}
		} else {
			out.Format = "CSV table"
		}
	}
	return out, nil
}
func parseWaveCSV(s string, out *Analysis) error {
	r := csv.NewReader(strings.NewReader(s))
	header, e := r.Read()
	if e != nil {
		return e
	}
	if len(header) < 2 {
		return errors.New("wave CSV needs time and one or more signal columns")
	}
	if len(header) > 65 {
		return errors.New("max 64 signals")
	}
	out.Timescale = header[0]
	for _, h := range header[1:] {
		out.Signals = append(out.Signals, WaveSignal{Name: h, Points: []WavePoint{}})
	}
	prev := -math.MaxFloat64
	for n := 0; ; n++ {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if n >= 50000 || n*len(out.Signals) >= 100000 {
			out.Limited = true
			break
		}
		t, e := finiteNumber(row[0])
		if e != nil || t < prev {
			return errors.New("time must be finite and nondecreasing")
		}
		prev = t
		for i, v := range row[1:] {
			value, e := finiteNumber(v)
			if e != nil {
				return fmt.Errorf("CSV row %d: %w", n+2, e)
			}
			out.Signals[i].Points = append(out.Signals[i].Points, WavePoint{T: t, V: &value})
		}
	}
	return nil
}
func parseVCD(s string, out *Analysis) error {
	tokens := strings.Fields(s)
	ids := map[string][]int{}
	scope := []string{}
	t := 0.0
	count := 0
	header := true
	for i := 0; i < len(tokens); i++ {
		v := tokens[i]
		if v == "$end" {
			continue
		}
		if strings.HasPrefix(v, "$") {
			end := i + 1
			for end < len(tokens) && tokens[end] != "$end" {
				end++
			}
			if end == len(tokens) {
				if v == "$dumpvars" || v == "$dumpon" || v == "$dumpoff" || v == "$end" {
					continue
				}
				return errors.New("unterminated VCD directive")
			}
			args := tokens[i+1 : end]
			switch v {
			case "$scope":
				if len(args) > 1 {
					scope = append(scope, args[1])
				}
			case "$upscope":
				if len(scope) > 0 {
					scope = scope[:len(scope)-1]
				}
			case "$timescale":
				out.Timescale = strings.Join(args, "")
			case "$var":
				if len(args) >= 4 {
					if len(out.Signals) >= 64 {
						out.Limited = true
					} else {
						idx := len(out.Signals)
						ids[args[2]] = append(ids[args[2]], idx)
						out.Signals = append(out.Signals, WaveSignal{Name: strings.Join(append(append([]string{}, scope...), strings.Join(args[3:], "")), "."), Digital: args[0] != "real", Points: []WavePoint{}})
					}
				}
			case "$enddefinitions":
				header = false
			case "$dumpvars", "$dumpon", "$dumpoff":
				continue
			}
			i = end
			continue
		}
		if header {
			continue
		}
		if strings.HasPrefix(v, "#") {
			next, e := finiteNumber(v[1:])
			if e != nil || next < t {
				return errors.New("invalid VCD timestamp")
			}
			t = next
			continue
		}
		if v == "" {
			continue
		}
		value, id := "", ""
		var num *float64
		switch v[0] {
		case 'b', 'B', 'r', 'R':
			if i+1 >= len(tokens) {
				return errors.New("truncated VCD value")
			}
			value = v[1:]
			i++
			id = tokens[i]
			if v[0] == 'r' || v[0] == 'R' {
				if n, e := finiteNumber(value); e == nil {
					num = &n
				}
			} else if n, e := strconv.ParseUint(value, 2, 53); e == nil {
				x := float64(n)
				num = &x
			}
		case '0', '1', 'x', 'X', 'z', 'Z':
			value = v[:1]
			id = v[1:]
			if value == "0" || value == "1" {
				n, _ := strconv.ParseFloat(value, 64)
				num = &n
			}
		default:
			continue
		}
		for _, idx := range ids[id] {
			out.Signals[idx].Points = append(out.Signals[idx].Points, WavePoint{T: t, V: num, Raw: value})
			count++
			if count >= 100000 {
				out.Limited = true
				return nil
			}
		}
	}
	if len(out.Signals) == 0 {
		return errors.New("no VCD signal declarations")
	}
	return nil
}

type HDLInput struct {
	Requirements string `json:"requirements"`
	Language     string `json:"language"`
	Code         string `json:"code"`
	Project      string `json:"project"`
	Root         string `json:"root"`
	Path         string `json:"path"`
}

func hdlExtension(lang string) string {
	for _, v := range CodeLanguages {
		if v.ID == lang {
			return v.Extension
		}
	}
	return ""
}
func (a *App) generateHDL(ctx context.Context, b json.RawMessage) (any, error) {
	var x HDLInput
	if e := parseInput(b, &x); e != nil {
		return nil, e
	}
	if hdlExtension(x.Language) == "" || len(x.Requirements) < 1 || len(x.Requirements) > 20000 {
		return nil, errors.New("Code language and requirements required")
	}
	code := ""
	if a.Config.Mode == "demo" {
		for _, v := range CodeLanguages {
			if v.ID == x.Language {
				code = v.Template
			}
		}
	} else {
		m, e := a.model(ctx, []modelMessage{{Role: "system", Content: "Generate only source code in " + x.Language + ". No markdown fences. Describe assumptions in code comments. Do not claim compilation or simulation passed."}, {Role: "user", Content: x.Requirements}}, false, false)
		if e != nil {
			return nil, e
		}
		code = strings.TrimSpace(m.Content)
		if strings.HasPrefix(code, "```") {
			if i := strings.Index(code, "\n"); i >= 0 {
				code = strings.TrimSuffix(code[i+1:], "```")
			}
		}
	}
	return map[string]any{"code": code, "extension": hdlExtension(x.Language), "demo": a.Config.Mode == "demo", "validated": false}, nil
}
func (a *App) saveHDL(b json.RawMessage) (any, error) {
	var x HDLInput
	if e := parseInput(b, &x); e != nil {
		return nil, e
	}
	root, e := a.fileRoot(x.Project, x.Root)
	if e != nil {
		return nil, e
	}
	return a.saveCodeAtRoot(root, x)
}
func (a *App) saveCodeAtRoot(root string, x HDLInput) (any, error) {
	ext := hdlExtension(x.Language)
	if ext == "" || filepath.Ext(x.Path) != ext || len(x.Code) == 0 || len(x.Code) > 1<<20 {
		return nil, errors.New("valid code and corresponding extension required")
	}
	parent, e := SafePath(root, filepath.Dir(x.Path))
	if e != nil {
		return nil, e
	}
	base := filepath.Base(x.Path)
	if base == "." || base == ".." {
		return nil, errors.New("invalid filename")
	}
	target := filepath.Join(parent, base)
	f, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return nil, e
	}
	_, e = f.WriteString(x.Code)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return map[string]string{"path": target}, e
}
func (a *App) compileHDL(b json.RawMessage) (any, error) {
	var x HDLInput
	if e := parseInput(b, &x); e != nil {
		return nil, e
	}
	if len(x.Code) == 0 || len(x.Code) > 1<<20 || hdlExtension(x.Language) == "" {
		return nil, errors.New("valid code/language required")
	}
	p := a.preferences()
	compiler := p.Compilers[x.Language]
	if compiler.Executable == "" && (x.Language == "verilog" || x.Language == "systemverilog" || x.Language == "vhdl") {
		compiler = p.Compiler
	}
	if compiler.Executable == "" {
		return nil, errors.New("configure a compiler/checker for this language in Code platform first")
	}
	dir := filepath.Join(a.Store.dir, "hdl", ID())
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	file := filepath.Join(dir, "design"+hdlExtension(x.Language))
	if e := os.WriteFile(file, []byte(x.Code), 0600); e != nil {
		return nil, e
	}
	c := compiler
	c.Args = append([]string{}, c.Args...)
	used := false
	for i, arg := range c.Args {
		if strings.Contains(arg, "{file}") {
			used = true
		}
		c.Args[i] = strings.ReplaceAll(arg, "{file}", file)
	}
	if !used {
		return nil, errors.New("compiler args must contain {file}")
	}
	return a.operation("compile", "Code "+x.Language, c, Environment{Setup: p.Setup, Shell: p.Shell}, dir, 120)
}
