package workbench

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

func ParseCSV(text, source string) ([]Metric, error) {
	r := csv.NewReader(strings.NewReader(text))
	head, e := r.Read()
	if e != nil {
		return nil, e
	}
	cols := map[string]int{}
	for i, k := range head {
		cols[strings.TrimSpace(strings.TrimPrefix(k, "\ufeff"))] = i
	}
	for _, k := range []string{"point", "corner", "metric", "value", "unit", "status"} {
		if _, ok := cols[k]; !ok {
			return nil, fmt.Errorf("missing CSV column %s", k)
		}
	}
	out := []Metric{}
	seen := map[string]bool{}
	line := 1
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		line++
		if len(out) >= 10000 {
			return nil, errors.New("maximum 10000 metrics per import")
		}
		get := func(k string) string {
			if i, ok := cols[k]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		m := Metric{Point: get("point"), Corner: get("corner"), Name: get("metric"), Unit: get("unit"), Status: get("status"), Source: fmt.Sprintf("%s:%d", source, line)}
		if m.Point == "" || m.Corner == "" || m.Name == "" || m.Unit == "" {
			return nil, fmt.Errorf("line %d: point, corner, metric and unit required (use '1' for dimensionless)", line)
		}
		switch m.Status {
		case "ok", "failed", "missing", "not_run":
		default:
			return nil, fmt.Errorf("line %d: invalid status", line)
		}
		key := m.Point + "\x00" + m.Corner + "\x00" + m.Name
		if seen[key] {
			return nil, fmt.Errorf("line %d: duplicate measurement", line)
		}
		seen[key] = true
		for _, pair := range []struct {
			k string
			p **float64
		}{{"value", &m.Value}, {"min", &m.Min}, {"max", &m.Max}} {
			v := get(pair.k)
			if v != "" {
				f, e := strconv.ParseFloat(v, 64)
				if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
					return nil, fmt.Errorf("line %d: invalid finite number for %s", line, pair.k)
				}
				*pair.p = &f
			}
		}
		if m.Status == "ok" && m.Value == nil {
			return nil, fmt.Errorf("line %d: ok result needs value", line)
		}
		if m.Min != nil && m.Max != nil && *m.Min > *m.Max {
			return nil, errors.New("min exceeds max")
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, errors.New("empty metric file")
	}
	return out, nil
}
func Verdict(m Metric) string {
	if m.Status != "ok" || m.Value == nil {
		return "unverified"
	}
	if (m.Min != nil && *m.Value < *m.Min) || (m.Max != nil && *m.Value > *m.Max) {
		return "fail"
	}
	if m.Min == nil && m.Max == nil {
		return "no_spec"
	}
	return "pass"
}
func ExportCSV(ms []Metric) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Write([]string{"point", "corner", "metric", "value", "unit", "status", "min", "max", "verdict", "source"})
	f := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'g', -1, 64)
	}
	safe := func(v string) string {
		if strings.HasPrefix(v, "=") || strings.HasPrefix(v, "+") || strings.HasPrefix(v, "-") || strings.HasPrefix(v, "@") || strings.HasPrefix(v, "\t") {
			return "'" + v
		}
		return v
	}
	for _, m := range ms {
		w.Write([]string{safe(m.Point), safe(m.Corner), safe(m.Name), f(m.Value), safe(m.Unit), m.Status, f(m.Min), f(m.Max), Verdict(m), safe(m.Source)})
	}
	w.Flush()
	return b.String()
}
func Report(b Batch) string {
	var s strings.Builder
	fmt.Fprintf(&s, "# %s\n\nProject: %s · Design: %s · Measurement: %s\n\nProvenance: %s\n\n", b.Name, b.Project, b.Design, b.Definition, b.Provenance)
	counts := map[string]int{}
	for _, m := range b.Metrics {
		counts[Verdict(m)]++
	}
	fmt.Fprintf(&s, "Pass: %d · Fail: %d · Unverified: %d · No specification: %d\n\n", counts["pass"], counts["fail"], counts["unverified"], counts["no_spec"])
	for _, m := range b.Metrics {
		v := "—"
		if m.Value != nil {
			v = strconv.FormatFloat(*m.Value, 'g', -1, 64)
		}
		fmt.Fprintf(&s, "- %s / %s / %s: %s %s [%s] — %s\n", m.Point, m.Corner, m.Name, v, m.Unit, Verdict(m), m.Source)
	}
	return s.String()
}

type Difference struct {
	Point  string   `json:"point"`
	Metric string   `json:"metric"`
	Corner string   `json:"corner"`
	Before *float64 `json:"before"`
	After  *float64 `json:"after"`
	Delta  *float64 `json:"delta"`
	Unit   string   `json:"unit"`
	State  string   `json:"state"`
}

func Compare(a, b Batch) ([]Difference, error) {
	if a.Project != b.Project || a.Definition == "" || a.Definition != b.Definition {
		return nil, errors.New("project/measurement definitions incompatible")
	}
	key := func(m Metric) string { return m.Point + "\x00" + m.Corner + "\x00" + m.Name }
	ma := map[string]Metric{}
	mb := map[string]Metric{}
	keys := map[string]bool{}
	for _, m := range a.Metrics {
		ma[key(m)] = m
		keys[key(m)] = true
	}
	for _, m := range b.Metrics {
		mb[key(m)] = m
		keys[key(m)] = true
	}
	ks := []string{}
	for k := range keys {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	out := []Difference{}
	for _, k := range ks {
		x, oka := ma[k]
		y, okb := mb[k]
		base := x
		if !oka {
			base = y
		}
		d := Difference{Point: base.Point, Metric: base.Name, Corner: base.Corner, Before: x.Value, After: y.Value, Unit: base.Unit, State: "unverified"}
		if oka && okb && x.Unit != y.Unit {
			d.State = "unit_mismatch"
		} else if oka && okb && x.Status == "ok" && y.Status == "ok" && x.Value != nil && y.Value != nil {
			delta := *y.Value - *x.Value
			d.Delta = &delta
			d.State = "comparable"
		}
		out = append(out, d)
	}
	return out, nil
}
func Diagnose(log string) []map[string]string {
	rules := []struct{ needle, fact, hypothesis, next string }{{"license", "License-related log entry", "License unavailable or configuration mismatch", "Check selected environment and license service; do not change server settings automatically"}, {"converg", "Convergence-related log entry", "Operating point, model or solver settings may be involved", "Inspect operating point and model references before changing tolerances"}, {"not found", "Missing resource reported", "Path or selected tool/PDK version may be wrong", "Verify actual file and tool version in the recorded environment"}, {"error", "Error reported", "Cause not established", "Inspect preceding log lines and reproduce with frozen inputs"}}
	out := []map[string]string{}
	for i, line := range strings.Split(log, "\n") {
		for _, r := range rules {
			if strings.Contains(strings.ToLower(line), r.needle) {
				out = append(out, map[string]string{"fact": r.fact, "evidence": fmt.Sprintf("line %d: %s", i+1, line), "hypothesis": r.hypothesis, "next": r.next})
				break
			}
		}
	}
	return out
}
