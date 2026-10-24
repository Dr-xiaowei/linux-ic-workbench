package workbench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) seedWorkspace() error {
	if a.Config.Mode != "demo" {
		return nil
	}
	dir := filepath.Join(a.Store.dir, "sample-project")
	if e := os.MkdirAll(filepath.Join(dir, "rtl"), 0700); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Join(dir, "results"), 0700); e != nil {
		return e
	}
	csv := "time_s,vin_V,vout_V\n"
	for i := 0; i <= 100; i++ {
		csv += fmt.Sprintf("%g,1,%g\n", float64(i)*1e-8, 1-float64(100-i)/100)
	}
	files := map[string]string{"README.md": "# RC filter demonstration\nSynthetic fixture, no PDK or commercial EDA.\nTarget: review gain and settling time.\n", "results/spectre-demo.log": "DEMO FIXTURE - not actual Spectre execution\nwarning: gain below example target at SS corner\nTransient completed (synthetic)\n", "results/transient.csv": csv, "rtl/counter.v": "// DEMO fixture\nmodule counter(input clk, input rst_n, output reg [7:0] q);\nalways @(posedge clk or negedge rst_n) if (!rst_n) q <= 0; else q <= q + 1'b1;\nendmodule\n", "results/digital.vcd": "$timescale 1ns $end\n$scope module counter $end\n$var wire 1 ! clk $end\n$var wire 8 # q [7:0] $end\n$upscope $end\n$enddefinitions $end\n$dumpvars\n0!\nb00000000 #\n$end\n#5\n1!\nb00000001 #\n#10\n0!\n#15\n1!\nb00000010 #\n#20\n0!\n"}
	for name, body := range files {
		f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if os.IsExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		_, e = f.WriteString(body)
		f.Close()
		if e != nil {
			return e
		}
	}
	return a.Store.Update(func(s *State) error {
		for i := range s.Projects {
			p := &s.Projects[i]
			if p.ID == "demo-rc" && p.Root == a.Store.dir {
				p.Root = dir
				p.Description = "RC 滤波器 · 模拟与数字资料示例 / synthetic design workspace"
			}
		}
		if len(s.FileNotes) == 0 {
			s.FileNotes = []FileNote{{"demo-rc", dir, "results", "仿真输出 / Simulation outputs", "CSV 与 VCD 均为合成演示数据 / synthetic fixtures"}, {"demo-rc", dir, "rtl/counter.v", "8 位计数器 / 8-bit counter", "Demo RTL example"}}
		}
		if len(s.Software) == 0 {
			s.Software = []Software{{ID: "demo-virtuoso", Name: "Virtuoso", Command: "virtuoso", Shell: "/bin/tcsh", Active: "IC618", Versions: []SoftwareVersion{{Name: "IC618", Setup: "/eda/env/ic618.csh"}, {Name: "ICADVM20.1", Setup: "/eda/env/icadvm201.csh"}}}, {ID: "demo-spectre", Name: "Spectre", Command: "spectre -W", Shell: "/bin/tcsh", Active: "21.1", Versions: []SoftwareVersion{{Name: "21.1", Setup: "/eda/env/spectre211.csh"}}}, {ID: "demo-vcs", Name: "VCS", Command: "vcs -ID", Shell: "/bin/bash", Active: "T-2022.06", Versions: []SoftwareVersion{{Name: "T-2022.06", Setup: "/eda/env/vcs.sh"}}}}
		}
		if len(s.Plugins) == 0 {
			s.Plugins = []Plugin{{ID: "demo-report", Name: "Report helper", Version: "1.0.0", Description: "演示：项目目录清单 / Demo project listing", Command: Command{Executable: "/bin/ls", Args: []string{"-la", "{project}"}}, Enabled: true, Timeout: 30}}
		}
		return nil
	})
}

// Keep protocol payloads in receipts rather than displaying raw JSON as the answer.
func demoActionSummary(action string, value any) string {
	var data map[string]any
	json.Unmarshal([]byte(StringJSON(value)), &data)
	switch action {
	case "browse":
		if rows, ok := data["entries"].([]any); ok {
			names := []string{}
			for _, v := range rows {
				r, ok := v.(map[string]any)
				if !ok {
					continue
				}
				name, _ := r["name"].(string)
				if strings.HasPrefix(name, "._") {
					continue
				}
				if r["directory"] == true {
					name += "/"
				}
				names = append(names, "• "+name)
			}
			if len(names) > 20 {
				names = append(names[:20], "…")
			}
			return fmt.Sprintf("已读取目录，本页 %d 项。 / Directory read: %d entries on this page.\n%s", len(rows), len(rows), strings.Join(names, "\n"))
		}
	case "search":
		if rows, ok := data["hits"].([]any); ok {
			return fmt.Sprintf("本页找到 %d 处匹配。可在项目页继续检索并查看原文。 / Found %d matches on this page; inspect source in Projects.", len(rows), len(rows))
		}
	case "monitor":
		return fmt.Sprintf("已读取资源样本：CPU %v%%，内存 %v%%，存储 %v%%。详细任务请查看资源监测页。 / Resource sample loaded; inspect tasks in System monitor.", data["cpu"], data["memory"], data["storage"])
	}
	return "演示工具调用已返回：" + action + "。请展开执行轨迹核对结果。 / Demo tool returned: " + action + "; inspect the execution trace."
}
