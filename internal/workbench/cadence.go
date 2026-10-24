package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// This adapter uses user-reviewed OCEAN setup/measurement definitions, never AI-generated SKILL.
type CadenceConfig struct {
	Fingerprint string        `json:"fingerprint,omitempty"`
	Virtuoso    string        `json:"virtuoso"`
	Ocean       string        `json:"ocean"`
	Spectre     string        `json:"spectre"`
	Library     string        `json:"library"`
	Cell        string        `json:"cell"`
	View        string        `json:"view"`
	Design      string        `json:"design"`
	Definition  string        `json:"definition"`
	Variables   []Variable    `json:"variables"`
	Cases       []CadenceCase `json:"cases"`
}
type CadenceCase struct {
	Name         string               `json:"name"`
	Setup        string               `json:"setup"`
	Measurements []CadenceMeasurement `json:"measurements"`
}
type CadenceMeasurement struct {
	Point      string   `json:"point"`
	Name       string   `json:"name"`
	Unit       string   `json:"unit"`
	Expression string   `json:"expression"`
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
}

func hashBytes(b []byte) string   { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func skillString(s string) string { return strconv.Quote(s) }
func loadCadence(path string) (CadenceConfig, []byte, error) {
	var c CadenceConfig
	raw, e := readBounded(path, 1<<20)
	if e != nil {
		return c, nil, e
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, nil, e
	}
	for _, p := range []string{c.Virtuoso, c.Ocean, c.Spectre} {
		if !filepath.IsAbs(p) {
			return c, nil, errors.New("Cadence executable paths must be absolute")
		}
	}
	if filepath.Base(c.Spectre) != "spectre" {
		return c, nil, errors.New("spectre executable must be named spectre (OCEAN PATH selection)")
	}
	safe := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,127}$`)
	if !safe.MatchString(c.Library) || !safe.MatchString(c.Cell) || !safe.MatchString(c.View) || c.Design == "" || c.Definition == "" {
		return c, nil, errors.New("Cadence library/cell/view and design/measurement versions required")
	}
	if len(c.Cases) < 1 || len(c.Cases) > 32 {
		return c, nil, errors.New("use 1..32 fixed cases")
	}
	names := map[string]bool{}
	for _, cc := range c.Cases {
		if cc.Name == "" || names[cc.Name] || !filepath.IsAbs(cc.Setup) || len(cc.Measurements) == 0 {
			return c, nil, errors.New("unique cases, absolute setup scripts and scalar measurements required")
		}
		names[cc.Name] = true
		for _, m := range cc.Measurements {
			if m.Point == "" || m.Name == "" || m.Unit == "" || m.Expression == "" {
				return c, nil, errors.New("measurement point/name/unit/expression required")
			}
			if m.Min != nil && m.Max != nil && *m.Min > *m.Max {
				return c, nil, errors.New("measurement minimum exceeds maximum")
			}
		}
	}
	names = map[string]bool{}
	for _, v := range c.Variables {
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`).MatchString(v.Name) || names[v.Name] || math.IsNaN(v.Min+v.Max+v.Initial) || math.IsInf(v.Min+v.Max+v.Initial, 0) || v.Min > v.Max || v.Initial < v.Min || v.Initial > v.Max {
			return c, nil, errors.New("invalid adapter parameter bounds")
		}
		names[v.Name] = true
	}
	return c, raw, nil
}
func cadenceParameters(c CadenceConfig, overrides map[string]float64) (map[string]float64, error) {
	out := map[string]float64{}
	bounds := map[string]Variable{}
	for _, v := range c.Variables {
		out[v.Name] = v.Initial
		bounds[v.Name] = v
	}
	for k, v := range overrides {
		b, ok := bounds[k]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < b.Min || v > b.Max {
			return nil, fmt.Errorf("parameter %s outside approved Cadence manifest", k)
		}
		out[k] = v
	}
	return out, nil
}
func probeScript(c CadenceConfig, dir string) string {
	return fmt.Sprintf(`; Read-only cellview probe. No dbSave call.
icwCV = dbOpenCellViewByType(%s %s %s "" "r")
unless(icwCV error("ICW cellview not readable"))
icwOut = outfile(%s)
fprintf(icwOut "%%s\n%%s\n%%s\n%%s\n" getVersion() icwCV~>libName icwCV~>cellName icwCV~>viewName)
close(icwOut)
dbClose(icwCV)
exit(0)
`, skillString(c.Library), skillString(c.Cell), skillString(c.View), skillString(filepath.Join(dir, "identity.txt")))
}
func oceanScript(c CadenceConfig, cc CadenceCase, params map[string]float64, dir, setup string) string {
	var s strings.Builder
	fmt.Fprintf(&s, "simulator('spectre)\ndesign(%s %s %s)\nresultsDir(%s)\nload(%s)\n", skillString(c.Library), skillString(c.Cell), skillString(c.View), skillString(filepath.Join(dir, "psf")), skillString(setup))
	// Enforce the selected design/output after the reviewed analysis/model setup.
	fmt.Fprintf(&s, "simulator('spectre)\ndesign(%s %s %s)\nresultsDir(%s)\n", skillString(c.Library), skillString(c.Cell), skillString(c.View), skillString(filepath.Join(dir, "psf")))
	keys := []string{}
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&s, "desVar(%s %.17g)\n", skillString(k), params[k])
	}
	fmt.Fprintf(&s, "icwNet = createNetlist(?recreateAll t ?display nil)\nunless(icwNet error(\"ICW netlist failed\"))\nicwN = outfile(%s)\nfprintf(icwN \"%%s\\n\" icwNet)\nclose(icwN)\nrun()\n", skillString(filepath.Join(dir, "netlist.path")))
	// Each expression must yield one scalar. Failed evaluations are emitted explicitly.
	fmt.Fprintf(&s, "icwOut = outfile(%s)\n", skillString(filepath.Join(dir, "values.tsv")))
	for i, m := range cc.Measurements {
		fmt.Fprintf(&s, "icwValue = errset(%s t)\nif(icwValue && numberp(car(icwValue)) then\n fprintf(icwOut \"%d\\t%%.17g\\n\" car(icwValue))\nelse\n fprintf(icwOut \"%d\\tFAILED\\n\")\n)\n", m.Expression, i, i)
	}
	s.WriteString("close(icwOut)\nexit(0)\n")
	return s.String()
}
func (a *App) cadence(ctx context.Context, j Job, p Project, env Environment, dir string) (Batch, string, string, error) {
	b := Batch{}
	c, raw, e := loadCadence(p.CadenceConfig)
	if e != nil {
		return b, "", "", e
	}
	params, e := cadenceParameters(c, j.Params)
	if e != nil {
		return b, "", "", e
	}
	if e = atomicFile(filepath.Join(dir, "cadence.snapshot.json"), raw); e != nil {
		return b, "", "", e
	}
	env.ToolPath = filepath.Dir(c.Spectre)
	script := filepath.Join(dir, "probe.il")
	if e = atomicFile(script, []byte(probeScript(c, dir))); e != nil {
		return b, "", "", e
	}
	log, e := a.command(ctx, Command{Executable: c.Virtuoso, Args: []string{"-nograph", "-restore", script, "-log", filepath.Join(dir, "virtuoso.log")}}, env, p.Root, nil)
	if e != nil {
		return b, "", log, fmt.Errorf("Virtuoso probe: %w", e)
	}
	identity, e := readBounded(filepath.Join(dir, "identity.txt"), 65536)
	if e != nil {
		return b, "", log, fmt.Errorf("Virtuoso probe produced no identity: %w", e)
	}
	lines := strings.Split(strings.TrimSpace(string(identity)), "\n")
	if len(lines) != 4 || lines[0] == "" || lines[1] != c.Library || lines[2] != c.Cell || lines[3] != c.View {
		return b, "", log, errors.New("Virtuoso identity does not match requested cellview")
	}
	evidence := StringJSON(map[string]any{"tool": "Virtuoso", "version": lines[0], "library": lines[1], "cell": lines[2], "view": lines[3], "configSHA256": hashBytes(raw), "access": "read-only", "process": "new headless session"})
	if e = atomicFile(filepath.Join(dir, "identity.json"), []byte(evidence)); e != nil {
		return b, evidence, log, e
	}
	if j.Kind == "probe" {
		return b, evidence, log, nil
	}
	version, err := a.command(ctx, Command{Executable: c.Spectre, Args: []string{"-W"}}, env, p.Root, nil)
	log += "\nSpectre version:\n" + version
	if err != nil {
		return b, evidence, log, fmt.Errorf("Spectre version probe: %w", err)
	}
	provenance := map[string]any{"virtuoso": lines[0], "spectre": strings.TrimSpace(version), "configSHA256": hashBytes(raw), "parameters": params, "setupHashes": map[string]string{}}
	metrics := []Metric{}
	for i, cc := range c.Cases {
		if e = ctx.Err(); e != nil {
			return b, evidence, log, e
		}
		caseDir := filepath.Join(dir, fmt.Sprintf("case-%02d", i))
		if e = os.MkdirAll(caseDir, 0700); e != nil {
			return b, evidence, log, e
		}
		setup, e := readBounded(cc.Setup, 1<<20)
		if e != nil {
			return b, evidence, log, e
		}
		setupPath := filepath.Join(caseDir, "setup.snapshot.ocn")
		if e = atomicFile(setupPath, setup); e != nil {
			return b, evidence, log, e
		}
		provenance["setupHashes"].(map[string]string)[cc.Name] = hashBytes(setup)
		ocn := filepath.Join(caseDir, "run.ocn")
		if e = atomicFile(ocn, []byte(oceanScript(c, cc, params, caseDir, setupPath))); e != nil {
			return b, evidence, log, e
		}
		part, e := a.command(ctx, Command{Executable: c.Ocean, Args: []string{"-nograph", "-restore", ocn, "-log", filepath.Join(caseDir, "ocean.log")}}, env, p.Root, nil)
		log += "\nCase " + cc.Name + ":\n" + part
		if e != nil {
			return b, evidence, log, fmt.Errorf("OCEAN case %s: %w", cc.Name, e)
		}
		net, e := readBounded(filepath.Join(caseDir, "netlist.path"), 65536)
		if e != nil {
			return b, evidence, log, errors.New("OCEAN netlist evidence absent")
		}
		// Preserve netlist evidence when available; model/PDK trees are referenced, never copied.
		netPath := strings.TrimSpace(string(net))
		if !filepath.IsAbs(netPath) {
			netPath = filepath.Join(p.Root, netPath)
		}
		netBytes, e := readBounded(netPath, 16<<20)
		if e != nil {
			return b, evidence, log, fmt.Errorf("cannot snapshot generated netlist: %w", e)
		}
		if e = atomicFile(filepath.Join(caseDir, "input.snapshot.scs"), netBytes); e != nil {
			return b, evidence, log, e
		}
		values, e := readBounded(filepath.Join(caseDir, "values.tsv"), 1<<20)
		if e != nil {
			return b, evidence, log, errors.New("OCEAN measurement evidence absent")
		}
		rows := strings.Split(strings.TrimSpace(string(values)), "\n")
		if len(rows) != len(cc.Measurements) {
			return b, evidence, log, errors.New("measurement count mismatch")
		}
		for k, m := range cc.Measurements {
			fields := strings.Split(rows[k], "\t")
			if len(fields) != 2 || fields[0] != strconv.Itoa(k) {
				return b, evidence, log, errors.New("measurement index mismatch")
			}
			metric := Metric{Point: m.Point, Corner: cc.Name, Name: m.Name, Unit: m.Unit, Min: m.Min, Max: m.Max, Status: "failed", Source: fmt.Sprintf("job:%s/case-%02d/values.tsv:%d", j.ID, i, k+1)}
			if fields[1] != "FAILED" {
				v, e := strconv.ParseFloat(fields[1], 64)
				if e != nil || math.IsInf(v, 0) || math.IsNaN(v) {
					return b, evidence, log, errors.New("non-finite EDA measurement")
				}
				metric.Value = &v
				metric.Status = "ok"
			}
			metrics = append(metrics, metric)
		}
	}
	// Validate uniqueness/units with the same parser used for imported data.
	if _, e = ParseCSV(ExportCSV(metrics), "adapter"); e != nil {
		return b, evidence, log, e
	}
	b = Batch{Name: "Cadence " + j.ID[:8], Design: c.Design, Definition: c.Definition + "@" + c.Fingerprint, Metrics: metrics, Provenance: StringJSON(provenance)}
	if e = atomicFile(filepath.Join(dir, "metrics.csv"), []byte(ExportCSV(metrics))); e != nil {
		return b, evidence, log, e
	}
	e = atomicFile(filepath.Join(dir, "result.json"), []byte(StringJSON(b)))
	return b, evidence, log, e
}

// Freeze manifest and setup scripts before a queued run; external PDK references remain explicit.
func snapshotCadence(path, dir string) (string, error) {
	c, raw, e := loadCadence(path)
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	fingerprint := append([]byte{}, raw...)
	for i, cc := range c.Cases {
		setup, e := readBounded(cc.Setup, 1<<20)
		if e != nil {
			return "", e
		}
		fingerprint = append(fingerprint, setup...)
		target := filepath.Join(dir, fmt.Sprintf("approved-setup-%02d.ocn", i))
		if e = atomicFile(target, setup); e != nil {
			return "", e
		}
		c.Cases[i].Setup = target
	}
	c.Fingerprint = hashBytes(fingerprint)
	target := filepath.Join(dir, "approved-cadence.json")
	return target, atomicFile(target, []byte(StringJSON(c)))
}
