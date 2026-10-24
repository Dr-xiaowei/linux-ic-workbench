package workbench

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ProcessIdentity struct {
	PID   int    `json:"pid"`
	Start string `json:"start"`
}
type ProcessUsage struct {
	ProcessIdentity
	Name       string  `json:"name"`
	CPU        float64 `json:"cpu"`
	Memory     uint64  `json:"memoryBytes"`
	Own        bool    `json:"own"`
	Simulation bool    `json:"simulation"`
	ticks      uint64
}
type MonitorSample struct {
	Time        string         `json:"time"`
	Source      string         `json:"source"`
	CPU         *float64       `json:"cpu"`
	Memory      *float64       `json:"memory"`
	Storage     *float64       `json:"storage"`
	StoragePath string         `json:"storagePath"`
	GPUs        []GPU          `json:"gpus"`
	Processes   []ProcessUsage `json:"processes"`
	Alerts      []string       `json:"alerts"`
	Notes       []string       `json:"notes"`
	total       uint64
	idle        uint64
	processes   map[int]ProcessUsage
}
type GPU struct {
	Name   string  `json:"name"`
	Usage  float64 `json:"usage"`
	Memory float64 `json:"memory"`
}

func number(s string) uint64 { n, _ := strconv.ParseUint(s, 10, 64); return n }
func procUsage(base string, pid int) (ProcessUsage, error) {
	p := ProcessUsage{ProcessIdentity: ProcessIdentity{PID: pid}}
	b, e := os.ReadFile(filepath.Join(base, strconv.Itoa(pid), "stat"))
	if e != nil {
		return p, e
	}
	str := string(b)
	l, r := strings.Index(str, "("), strings.LastIndex(str, ")")
	if l < 0 || r <= l {
		return p, errors.New("invalid proc stat")
	}
	f := strings.Fields(str[r+1:])
	if len(f) < 22 {
		return p, errors.New("short proc stat")
	}
	p.Name = str[l+1 : r]
	p.Start = f[19]
	p.ticks = number(f[11]) + number(f[12])
	p.Memory = number(f[21]) * uint64(os.Getpagesize())
	status, e := os.ReadFile(filepath.Join(base, strconv.Itoa(pid), "status"))
	if e == nil {
		for _, line := range strings.Split(string(status), "\n") {
			v := strings.Fields(line)
			if len(v) > 1 && v[0] == "Uid:" {
				p.Own = int(number(v[1])) == os.Getuid()
			}
		}
	}
	name := strings.ToLower(p.Name)
	for _, v := range []string{"spectre", "virtuoso", "ocean", "vcs", "simv", "calibre", "hspice", "alps", "dc_shell", "pt_shell", "xrun", "innovus", "verdi", "vivado"} {
		if strings.Contains(name, v) {
			p.Simulation = true
		}
	}
	return p, nil
}
func (a *App) monitor() MonitorSample {
	a.monitorMu.Lock()
	defer a.monitorMu.Unlock()
	last := a.monitorLast
	if t, e := time.Parse(time.RFC3339, last.Time); e == nil && time.Since(t) < 3*time.Second {
		return last
	}
	p := a.preferences()
	m := MonitorSample{Time: Now(), Source: runtime.GOOS, StoragePath: p.Workspace, GPUs: []GPU{}, Processes: []ProcessUsage{}, Alerts: []string{}, Notes: []string{}, processes: map[int]ProcessUsage{}}
	if a.Config.Mode == "demo" {
		m.Source = "DEMO · synthetic telemetry (not host measurements)"
		cpu := 42 + float64(time.Now().Unix()%7)
		memory := 57.8
		storage := 34.2
		m.CPU = &cpu
		m.Memory = &memory
		m.Storage = &storage
		m.GPUs = []GPU{{Name: "Demo GPU", Usage: 28, Memory: 32}}
		m.Notes = append(m.Notes, "Synthetic preview. Production reads Linux /proc and nvidia-smi.")
		for _, v := range []ProcessUsage{{ProcessIdentity: ProcessIdentity{PID: 999991, Start: "demo-spectre"}, Name: "spectre (DEMO)", CPU: 132.5, Memory: 2411724800, Own: true, Simulation: true}, {ProcessIdentity: ProcessIdentity{PID: 999992, Start: "demo-vcs"}, Name: "simv (DEMO)", CPU: 74.2, Memory: 805306368, Own: true, Simulation: true}, {ProcessIdentity: ProcessIdentity{PID: 999993, Start: "demo-virtuoso"}, Name: "virtuoso (DEMO)", CPU: 3.1, Memory: 486539264, Own: true, Simulation: true}} {
			if !a.demoStopped[v.PID] {
				m.Processes = append(m.Processes, v)
			}
		}
		for k, v := range map[string]*float64{"CPU": m.CPU, "Memory": m.Memory, "Storage": m.Storage} {
			if *v >= p.Threshold {
				m.Alerts = append(m.Alerts, fmt.Sprintf("DEMO %s %.1f%%", k, *v))
			}
		}
		a.monitorLast = m
		return m
	}
	var fs syscall.Statfs_t
	if syscall.Statfs(p.Workspace, &fs) == nil && fs.Blocks > 0 {
		n := 100 * (1 - float64(fs.Bavail)/float64(fs.Blocks))
		m.Storage = &n
	}
	if runtime.GOOS == "linux" {
		b, e := os.ReadFile("/proc/stat")
		if e == nil {
			f := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
			for i := 1; i < len(f) && i <= 8; i++ {
				m.total += number(f[i])
			}
			if len(f) > 5 {
				m.idle = number(f[4]) + number(f[5])
			}
			if last.total > 0 && m.total > last.total && m.idle >= last.idle {
				v := 100 * (1 - float64(m.idle-last.idle)/float64(m.total-last.total))
				m.CPU = &v
			}
		}
		b, e = os.ReadFile("/proc/meminfo")
		if e == nil {
			vals := map[string]uint64{}
			for _, line := range strings.Split(string(b), "\n") {
				f := strings.Fields(line)
				if len(f) > 1 {
					vals[strings.TrimSuffix(f[0], ":")] = number(f[1])
				}
			}
			avail := vals["MemAvailable"]
			if avail == 0 {
				avail = vals["MemFree"] + vals["Buffers"] + vals["Cached"] + vals["SReclaimable"]
				if avail >= vals["Shmem"] {
					avail -= vals["Shmem"]
				}
			}
			if total := vals["MemTotal"]; total > 0 {
				v := 100 * (1 - float64(avail)/float64(total))
				m.Memory = &v
			}
		}
		entries, _ := os.ReadDir("/proc")
		for _, d := range entries {
			pid, e := strconv.Atoi(d.Name())
			if e != nil {
				continue
			}
			v, e := procUsage("/proc", pid)
			if e != nil {
				continue
			}
			if old, ok := last.processes[pid]; ok && old.Start == v.Start && v.ticks >= old.ticks && m.total > last.total {
				v.CPU = 100 * float64(v.ticks-old.ticks) / float64(m.total-last.total) * float64(runtime.NumCPU())
			}
			m.processes[pid] = v
			m.Processes = append(m.Processes, v)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
		raw, e := cmd.Output()
		cancel()
		if e == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(raw)))
			for scanner.Scan() {
				v := strings.Split(scanner.Text(), ",")
				if len(v) == 4 {
					usage, eu := finiteNumber(strings.TrimSpace(v[1]))
					used, em := finiteNumber(strings.TrimSpace(v[2]))
					total, et := finiteNumber(strings.TrimSpace(v[3]))
					if eu == nil && em == nil && et == nil && total > 0 {
						m.GPUs = append(m.GPUs, GPU{strings.TrimSpace(v[0]), usage, 100 * used / total})
					}
				}
			}
		} else {
			m.Notes = append(m.Notes, "GPU unavailable: NVIDIA driver/nvidia-smi missing or unsupported GPU")
		}
	} else {
		m.Notes = append(m.Notes, "Linux /proc monitoring requires a Linux server; unavailable fields are not zero")
	}
	sort.Slice(m.Processes, func(i, j int) bool {
		if m.Processes[i].CPU == m.Processes[j].CPU {
			return m.Processes[i].Memory > m.Processes[j].Memory
		}
		return m.Processes[i].CPU > m.Processes[j].CPU
	})
	if len(m.Processes) > 200 {
		m.Processes = m.Processes[:200]
	}
	for k, v := range map[string]*float64{"CPU": m.CPU, "Memory": m.Memory, "Storage": m.Storage} {
		if v != nil && *v >= p.Threshold {
			m.Alerts = append(m.Alerts, fmt.Sprintf("%s %.1f%% ≥ %.1f%%", k, *v, p.Threshold))
		}
	}
	for _, g := range m.GPUs {
		if g.Usage >= p.Threshold || g.Memory >= p.Threshold {
			m.Alerts = append(m.Alerts, "GPU "+g.Name+" exceeds threshold")
		}
	}
	a.monitorLast = m
	return m
}
func (a *App) terminate(x ProcessIdentity) error {
	if a.Config.Mode == "demo" {
		a.monitorMu.Lock()
		defer a.monitorMu.Unlock()
		known := map[int]string{999991: "demo-spectre", 999992: "demo-vcs", 999993: "demo-virtuoso"}
		if known[x.PID] != x.Start || x.Start == "" {
			return errors.New("demo cannot terminate host processes")
		}
		a.demoStopped[x.PID] = true
		a.monitorLast.Time = ""
		return nil
	}
	if runtime.GOOS != "linux" {
		return errors.New("Linux process control only")
	}
	if x.PID <= 1 || x.PID == os.Getpid() || x.PID == os.Getppid() || x.Start == "" {
		return errors.New("protected/invalid process")
	}
	p, e := procUsage("/proc", x.PID)
	if e != nil {
		return e
	}
	if !p.Own || p.Start != x.Start {
		return errors.New("process owner/identity changed; refresh list")
	}
	return syscall.Kill(x.PID, syscall.SIGTERM)
}
