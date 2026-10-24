package workbench

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type UnitMetric struct {
	Metric      string   `json:"metric"`
	Value       float64  `json:"value"`
	Unit        string   `json:"unit"`
	Corner      string   `json:"corner"`
	Voltage     float64  `json:"voltage"`
	Temperature float64  `json:"temperature"`
	Condition   string   `json:"condition"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Source      string   `json:"source"`
}
type MetricComparison struct {
	Metric    string   `json:"metric"`
	Condition string   `json:"condition"`
	Unit      string   `json:"unit"`
	Baseline  *float64 `json:"baseline,omitempty"`
	Candidate *float64 `json:"candidate,omitempty"`
	Delta     *float64 `json:"delta,omitempty"`
	Percent   *float64 `json:"percent,omitempty"`
	Status    string   `json:"status"`
	Reason    string   `json:"reason"`
	Source    string   `json:"source"`
}
type metricUnit struct {
	dimension string
	scale     float64
}

var metricUnits = map[string]metricUnit{
	"V": {"V", 1}, "mV": {"V", 1e-3}, "uV": {"V", 1e-6}, "µV": {"V", 1e-6}, "A": {"A", 1}, "mA": {"A", 1e-3}, "uA": {"A", 1e-6}, "nA": {"A", 1e-9},
	"W": {"W", 1}, "mW": {"W", 1e-3}, "uW": {"W", 1e-6}, "nW": {"W", 1e-9}, "Hz": {"Hz", 1}, "kHz": {"Hz", 1e3}, "MHz": {"Hz", 1e6}, "GHz": {"Hz", 1e9},
	"s": {"s", 1}, "ms": {"s", 1e-3}, "us": {"s", 1e-6}, "ns": {"s", 1e-9}, "ps": {"s", 1e-12}, "F": {"F", 1}, "pF": {"F", 1e-12}, "nF": {"F", 1e-9},
	"ohm": {"ohm", 1}, "kohm": {"ohm", 1e3}, "Mohm": {"ohm", 1e6}, "dB": {"dB", 1}, "deg": {"deg", 1}, "%": {"%", 1}, "1": {"1", 1},
}

func metricNumber(s string) (float64, error) {
	n, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, errors.New("finite numeric value required")
	}
	return n, nil
}
func parseUnitMetrics(text, source string) ([]UnitMetric, error) {
	if len(text) > 256000 || len(source) > 250 {
		return nil, errors.New("CSV max 256 KB; source label max 250 bytes")
	}
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, errors.New("CSV must be UTF-8 text without NUL / CSV 必须为不含空字符的 UTF-8 文本")
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	rows, e := r.ReadAll()
	if e != nil {
		return nil, e
	}
	if len(rows) < 2 || len(rows) > 1001 {
		return nil, errors.New("CSV requires 1–1000 data rows")
	}
	header := map[string]int{}
	for i, v := range rows[0] {
		v = strings.TrimSpace(v)
		if _, exists := header[v]; exists {
			return nil, fmt.Errorf("duplicate header: %s", v)
		}
		header[v] = i
	}
	for _, v := range []string{"metric", "value", "unit", "corner", "voltage", "temperature", "condition"} {
		if _, ok := header[v]; !ok {
			return nil, fmt.Errorf("missing CSV column: %s", v)
		}
	}
	out := []UnitMetric{}
	seen := map[string]bool{}
	for i, row := range rows[1:] {
		get := func(k string) string {
			n, ok := header[k]
			if !ok {
				return ""
			}
			return strings.TrimSpace(row[n])
		}
		v, e := metricNumber(get("value"))
		if e != nil {
			return nil, fmt.Errorf("row %d value: %w", i+2, e)
		}
		vol, e := metricNumber(get("voltage"))
		if e != nil {
			return nil, fmt.Errorf("row %d voltage (V): %w", i+2, e)
		}
		temp, e := metricNumber(get("temperature"))
		if e != nil {
			return nil, fmt.Errorf("row %d temperature (C): %w", i+2, e)
		}
		m := UnitMetric{Metric: get("metric"), Value: v, Unit: get("unit"), Corner: get("corner"), Voltage: vol, Temperature: temp, Condition: get("condition"), Source: fmt.Sprintf("%s:%d", source, i+2)}
		if m.Metric == "" || m.Unit == "" || m.Corner == "" || m.Condition == "" || vol <= 0 || temp < -273.15 {
			return nil, fmt.Errorf("row %d needs metric, unit and complete PVT/condition; voltage>0, temperature>=-273.15C", i+2)
		}
		for _, k := range []string{"min", "max"} {
			if get(k) != "" {
				n, e := metricNumber(get(k))
				if e != nil {
					return nil, e
				}
				if k == "min" {
					m.Min = &n
				} else {
					m.Max = &n
				}
			}
		}
		if m.Min != nil && m.Max != nil && *m.Min > *m.Max {
			return nil, errors.New("minimum exceeds maximum")
		}
		// Check each known-unit input, including limits, before matching conditions.
		// Otherwise an overflowed maximum can silently become an unlimited spec.
		if unit, ok := metricUnits[m.Unit]; ok {
			values := []float64{m.Value}
			if m.Min != nil {
				values = append(values, *m.Min)
			}
			if m.Max != nil {
				values = append(values, *m.Max)
			}
			for _, original := range values {
				converted := original * unit.scale
				if math.IsInf(converted, 0) || math.IsNaN(converted) || (original != 0 && converted == 0) {
					return nil, fmt.Errorf("row %d unit conversion overflows or underflows / 单位换算超出数值范围", i+2)
				}
			}
		}
		key := metricKey(m)
		if seen[key] {
			return nil, fmt.Errorf("duplicate metric and operating condition at row %d", i+2)
		}
		seen[key] = true
		out = append(out, m)
	}
	return out, nil
}
func metricKey(m UnitMetric) string {
	return StringJSON([]any{m.Metric, m.Corner, m.Voltage, m.Temperature, m.Condition})
}
func compareUnitMetrics(raw json.RawMessage) (any, error) {
	var x struct {
		Baseline      string `json:"baseline"`
		Candidate     string `json:"candidate"`
		BaselineName  string `json:"baselineName"`
		CandidateName string `json:"candidateName"`
	}
	if e := parseInput(raw, &x); e != nil {
		return nil, e
	}
	if x.BaselineName == "" {
		x.BaselineName = "baseline.csv"
	}
	if x.CandidateName == "" {
		x.CandidateName = "candidate.csv"
	}
	base, e := parseUnitMetrics(x.Baseline, x.BaselineName)
	if e != nil {
		return nil, e
	}
	next, e := parseUnitMetrics(x.Candidate, x.CandidateName)
	if e != nil {
		return nil, e
	}
	index := map[string]UnitMetric{}
	names := map[string]bool{}
	for _, m := range next {
		index[metricKey(m)] = m
		names[m.Metric] = true
	}
	out := []MetricComparison{}
	for _, b := range base {
		key := metricKey(b)
		c, ok := index[key]
		delete(index, key)
		condition := fmt.Sprintf("%s · %.8g V · %.8g °C · %s", b.Corner, b.Voltage, b.Temperature, b.Condition)
		row := MetricComparison{Metric: b.Metric, Condition: condition, Status: "not-comparable", Source: b.Source}
		if !ok {
			row.Reason = "候选缺少相同工况 / No matching candidate condition"
			if names[b.Metric] {
				row.Reason = "工况不同，不比较 / Operating conditions differ"
			}
			out = append(out, row)
			continue
		}
		row.Source += " ↔ " + c.Source
		u, bu := metricUnits[b.Unit]
		v, cu := metricUnits[c.Unit]
		if !bu || !cu || u.dimension != v.dimension {
			row.Reason = "单位未知或不兼容 / Unknown or incompatible units"
			out = append(out, row)
			continue
		}
		bv, cv := b.Value*u.scale, c.Value*v.scale
		d := cv - bv
		if math.IsInf(bv, 0) || math.IsInf(cv, 0) || math.IsInf(d, 0) {
			return nil, errors.New("converted metric overflows")
		}
		row.Unit = u.dimension
		row.Baseline = &bv
		row.Candidate = &cv
		row.Delta = &d
		row.Status = "compared"
		row.Reason = "仅比较数值，不自动判断优劣 / Numeric comparison only"
		if bv != 0 && u.dimension != "dB" {
			p := d / math.Abs(bv) * 100
			if !math.IsInf(p, 0) {
				row.Percent = &p
			}
		}
		// Baseline specification is the single reference; candidate limits never relax it.
		if b.Min != nil || b.Max != nil {
			row.Status = "pass"
			row.Reason = "满足基线阈值 / Meets baseline limits"
			if b.Min != nil && cv < *b.Min*u.scale || b.Max != nil && cv > *b.Max*u.scale {
				row.Status = "fail"
				row.Reason = "未满足基线阈值 / Outside baseline limits"
			}
		}
		out = append(out, row)
	}
	for _, c := range next {
		if _, ok := index[metricKey(c)]; ok {
			out = append(out, MetricComparison{Metric: c.Metric, Condition: fmt.Sprintf("%s · %g V · %g °C · %s", c.Corner, c.Voltage, c.Temperature, c.Condition), Status: "not-comparable", Reason: "基线缺少相同工况 / No matching baseline", Source: c.Source})
		}
	}
	return map[string]any{"rows": out, "note": "电源列用 V，温度列用 °C；阈值取基线单位。dB 仅计算差值。无自动性能签核。 / Voltage in V, temperature in °C; limits use baseline units. dB delta only. No performance sign-off."}, nil
}
