package workbench

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"time"
)

func ValidateExperiment(x Experiment) error {
	if len(x.Variables) < 1 || len(x.Variables) > 5 {
		return errors.New("use 1..5 variables")
	}
	if x.Budget < 2 || x.Budget > 100 {
		return errors.New("budget must be 2..100 including baseline and revalidation")
	}
	if x.MaxSeconds < 1 || x.MaxSeconds > 86400 {
		return errors.New("maxSeconds must be 1..86400")
	}
	if x.Objective == "" || x.Unit == "" || (x.Direction != "min" && x.Direction != "max") {
		return errors.New("objective, unit, min/max direction required")
	}
	seen := map[string]bool{}
	for _, v := range x.Variables {
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`).MatchString(v.Name) || seen[v.Name] {
			return errors.New("unique safe parameter names required")
		}
		seen[v.Name] = true
		if math.IsNaN(v.Min+v.Max+v.Step+v.Initial) || math.IsInf(v.Min+v.Max+v.Step+v.Initial, 0) || v.Min > v.Max || v.Step < 0 || v.Initial < v.Min || v.Initial > v.Max {
			return errors.New("invalid parameter bounds")
		}
	}
	for _, c := range x.Constraints {
		if c.Metric == "" || c.Unit == "" || c.Min == nil && c.Max == nil {
			return errors.New("constraint requires metric, unit and bound")
		}
		if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
			return errors.New("constraint min exceeds max")
		}
	}
	return nil
}
func halton(n, base int) float64 {
	f, r := 1.0, 0.0
	for n > 0 {
		f /= float64(base)
		r += f * float64(n%base)
		n /= base
	}
	return r
}
func Candidate(x Experiment, index int) map[string]float64 {
	out := map[string]float64{}
	for i, v := range x.Variables {
		value := v.Initial
		if index > 0 {
			value = v.Min + (v.Max-v.Min)*halton(index, []int{2, 3, 5, 7, 11}[i])
			if v.Step > 0 {
				value = v.Min + math.Min(math.Floor((v.Max-v.Min)/v.Step+1e-9), math.Round((value-v.Min)/v.Step))*v.Step
			}
			value = math.Max(v.Min, math.Min(v.Max, value))
		}
		out[v.Name] = value
	}
	return out
}
func Score(x Experiment, b Batch) (*float64, bool) {
	var value *float64
	found := map[string]bool{}
	feasible := true
	for _, m := range b.Metrics {
		if m.Status != "ok" || m.Value == nil {
			feasible = false
			continue
		}
		if m.Name == x.Objective {
			if m.Unit != x.Unit {
				feasible = false
				continue
			}
			v := *m.Value
			if value == nil || (x.Direction == "max" && v < *value) || (x.Direction == "min" && v > *value) {
				value = &v
			}
		}
		for _, c := range x.Constraints {
			if m.Name == c.Metric {
				found[c.Metric] = true
				if m.Unit != c.Unit || c.Min != nil && *m.Value < *c.Min || c.Max != nil && *m.Value > *c.Max {
					feasible = false
				}
			}
		}
	}
	for _, c := range x.Constraints {
		if !found[c.Metric] {
			feasible = false
		}
	}
	return value, feasible && value != nil
}
func (a *App) NewExperiment(x Experiment) (Experiment, error) {
	if e := ValidateExperiment(x); e != nil {
		return x, e
	}
	if _, e := FindProject(a.Store.Snapshot(), x.Project); e != nil {
		return x, e
	}
	x.ID = ID()
	x.Status = "paused"
	x.Created = Now()
	x.Trials = []Trial{}
	x.Best = ""
	x.CurrentJob = ""
	x.Error = ""
	x.ElapsedSeconds = 0
	x.Revalidated = false
	x.MeasurementKeys = nil
	x.Design = ""
	x.Definition = ""
	e := a.Store.Update(func(s *State) error {
		if len(s.Experiments) >= a.Config.MaxRecords {
			return errors.New("experiment capacity reached")
		}
		s.Experiments = append(s.Experiments, x)
		return nil
	})
	return x, e
}
func (a *App) experiment(id string) (Experiment, error) {
	for _, x := range a.Store.Snapshot().Experiments {
		if x.ID == id {
			return x, nil
		}
	}
	return Experiment{}, errors.New("experiment not found")
}
func (a *App) updateExperiment(id string, fn func(*Experiment)) {
	a.Store.Update(func(s *State) error {
		for i := range s.Experiments {
			if s.Experiments[i].ID == id {
				fn(&s.Experiments[i])
				return nil
			}
		}
		return errors.New("experiment not found")
	})
}
func (a *App) ControlExperiment(id, action string) error {
	x, e := a.experiment(id)
	if e != nil {
		return e
	}
	if x.Status == "completed" || x.Status == "stopped" {
		return errors.New("experiment already finished")
	}
	if action == "pause" || action == "stop" {
		status := "paused"
		if action == "stop" {
			status = "stopped"
		}
		a.updateExperiment(id, func(x *Experiment) { x.Status = status })
		if action == "stop" && x.CurrentJob != "" {
			a.Cancel(x.CurrentJob)
		}
		return nil
	}
	if action != "resume" {
		return errors.New("unknown action")
	}
	if x.Status == "completed" || x.Status == "stopped" {
		return errors.New("experiment already finished")
	}
	a.mu.Lock()
	if a.experiments[id] || a.closing {
		a.mu.Unlock()
		return errors.New("current trial still active; resume after it finishes")
	}
	a.experiments[id] = true
	a.wg.Add(1)
	a.mu.Unlock()
	a.updateExperiment(id, func(x *Experiment) { x.Status = "running"; x.Error = "" })
	go a.optimize(id)
	return nil
}
func measurementKeys(b Batch) []string {
	out := []string{}
	for _, m := range b.Metrics {
		out = append(out, m.Point+"/"+m.Corner+"/"+m.Name+"/"+m.Unit)
	}
	sort.Strings(out)
	return out
}
func (a *App) optimize(id string) {
	defer a.wg.Done()
	defer func() { a.mu.Lock(); delete(a.experiments, id); a.mu.Unlock() }()
	started := time.Now()
	initial, _ := a.experiment(id)
	prior := initial.ElapsedSeconds
	elapsed := func() float64 { return prior + time.Since(started).Seconds() }
	defer func() { a.updateExperiment(id, func(x *Experiment) { x.ElapsedSeconds = elapsed() }) }()
	for {
		x, e := a.experiment(id)
		if e != nil || x.Status != "running" {
			return
		}
		if elapsed() >= float64(x.MaxSeconds) {
			a.updateExperiment(id, func(x *Experiment) { x.Status = "stopped"; x.Error = "wall-time budget exhausted" })
			return
		}
		if len(x.Trials) >= x.Budget {
			a.updateExperiment(id, func(x *Experiment) {
				x.Status = "completed"
				if !x.Revalidated {
					x.Error = "Budget consumed; no revalidated feasible candidate"
				}
			})
			return
		}
		params := Candidate(x, len(x.Trials))
		final := len(x.Trials) == x.Budget-1
		if final && x.Best == "" {
			params = Candidate(x, 0)
		}
		if final && x.Best != "" {
			for _, tr := range x.Trials {
				if tr.Job == x.Best {
					params = tr.Params
				}
			}
		}
		var j Job
		if x.CurrentJob != "" {
			j, e = a.Job(x.CurrentJob)
		} else {
			j, e = a.Submit(Job{Project: x.Project, Kind: "simulate", Params: params, Environment: x.Environment, Key: fmt.Sprintf("opt:%s:%d", id, len(x.Trials))})
		}
		if e != nil {
			a.updateExperiment(id, func(x *Experiment) { x.Status = "paused"; x.Error = e.Error() })
			return
		}
		a.updateExperiment(id, func(x *Experiment) { x.CurrentJob = j.ID })
		for {
			j, _ = a.Job(j.ID)
			if j.Status != "running" && j.Status != "queued" {
				break
			}
			time.Sleep(100 * time.Millisecond)
			current, _ := a.experiment(id)
			if current.Status == "stopped" || elapsed() >= float64(x.MaxSeconds) {
				a.Cancel(j.ID)
			}
		}
		tr := Trial{Job: j.ID, Params: j.Params, Status: j.Status}
		var batch Batch
		if j.Status == "completed" {
			for _, b := range a.Store.Snapshot().Batches {
				if b.ID == j.Batch {
					batch = b
					tr.Value, tr.Feasible = Score(x, b)
				}
			}
		}
		compatible := len(x.Trials) == 0 || (batch.Design == x.Design && batch.Definition == x.Definition && StringJSON(measurementKeys(batch)) == StringJSON(x.MeasurementKeys))
		if !compatible {
			tr.Feasible = false
			tr.Status = "incompatible"
		}
		a.updateExperiment(id, func(x *Experiment) {
			if len(x.Trials) == 0 {
				x.Design = batch.Design
				x.Definition = batch.Definition
				x.MeasurementKeys = measurementKeys(batch)
			}
			x.Trials = append(x.Trials, tr)
			x.CurrentJob = ""
			x.ElapsedSeconds = elapsed()
			if len(x.Trials) == 1 && (j.Status != "completed" || tr.Value == nil) {
				x.Status = "stopped"
				x.Error = "Baseline failed or objective absent; inspect evidence and create a revised experiment"
			}
			hadCandidate := x.Best != ""
			if tr.Feasible {
				var best *float64
				for _, old := range x.Trials {
					if old.Job == x.Best {
						best = old.Value
					}
				}
				if best == nil || (x.Direction == "max" && *tr.Value > *best) || (x.Direction == "min" && *tr.Value < *best) {
					x.Best = tr.Job
				}
			}
			if final {
				x.Revalidated = tr.Feasible && compatible && hadCandidate
				if x.Revalidated {
					x.Best = tr.Job
				}
			}
		})
	}
}
