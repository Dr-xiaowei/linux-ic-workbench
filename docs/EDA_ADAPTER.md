# Virtuoso / Spectre 首条适配通路

本版实现了原生 Cadence 批处理适配器和自定义命令适配接口。实际 Cadence、PDK 和许可证在本机不可用；以下接口代码已做生成、边界、冻结输入和进程合同测试，**商业软件执行未验收**。不能用 Demo 的 SIMULATED 身份当作联通结果。

## 原生适配器配置

1. 先建立单个获准测试平台，保留其原理图、cds.lib、模型设置及 ADE 导出的参考 OCEAN。第一版面向普通 OCEAN 和 schematic；ADE XL/Explorer/Assembler、config view 等需另做版本适配。
2. 复制 `examples/cadence/project.example.json`，改成公司的 Virtuoso/OCEAN/Spectre 绝对路径、真实 library/cell/view、设计版本、测量定义、批准参数范围和固定工况。
3. 每个工况的 setup 指向工程师审核的 OCEAN 文件，仅配置模型/分析/保存节点/温度/固定变量；示例不包含 PDK。setup/测量 expression 是**受信任可执行代码**，由工程师管理，不能把 AI 输出直接填入并执行。setup 不应包含 run/plot/exit/dbSave/后台调度或写工程原件。
4. 在“软件环境”登记版本、工具路径和公司 setup `.csh`/`.sh`，选择相应 shell；在“项目与文件”绑定此环境，把 Cadence JSON 的绝对路径填入“Cadence 配置文件”。工程根目录用于读取 cds.lib。
5. 运行“验证 Virtuoso 身份”。程序新建无图形会话，调用 `dbOpenCellViewByType(... "r")`，读取实际版本/library/cell/view，验证其与配置一致。不会连接用户已有 CIW/GUI 会话，不保存 cellview。
6. 身份成功后提交仿真：冻结配置与各 setup 文件 → 只读探测 → 记录 `spectre -W` → 各工况用 OCEAN 设置 Spectre、设计、结果目录与 desVar → createNetlist → run → 对批准的标量测量表达式求值 → 导出指标。

OCEAN 通过 PATH 找 Spectre，适配器将配置的 Spectre 所在目录放到子进程 PATH 前部；不改全局 `.cshrc`。必须在实际仿真日志中核对使用的可执行文件和版本，避免公司启动包装脚本覆盖 PATH。`spectre` 路径文件名必须为 spectre。

每次运行独立目录包含 request.json、approved-cadence.json、setup 快照、probe.il、identity.json、日志、每工况 run.ocn、生成网表快照、原始标量值、metrics.csv/result.json。记录配置/脚本 SHA-256；外部 PDK 和依赖 include 只引用、不自动复制，工程师需固定版本。生成网表仍可能受公司 OCEAN netlist 设置影响，必须核对首个真实运行输出路径，禁止写原工程。

标量求值失败输出 failed；缺少身份/网表/导出文件、数量错误、非有限数均失败，不编造指标。仿真进程退出成功与“规格达标”分开显示。测量返回 waveform/family 的表达式需先做正确标量约简，不能直接作为 value。

优化仅覆盖明确 desVar、固定工况和定义，1～5 参数、2～100 次总预算，第一轮基线、最后一轮最佳候选复验，剩余轮采用确定性 Halton 空间采样。目标对固定工况取最差值，所有硬约束需满足。暂停会完成当前一轮后停止提交，可继续恢复且累计时间/次数不清零；停止立即请求取消。参数不写回原理图；候选由工程师采纳。中途改变配置、测量定义、指标键/单位会使结果不可比；外部 PDK/工程改动须靠用户固定版本，当前不提供整个设计数据库快照。

## 自定义受控命令适配器

如果公司已有成熟批处理脚本，在全局 config.json 设置 `probe`、`prepare`、`simulate`、`export`，每项为 `{"executable":"/absolute/path","args":["{request}","{workdir}","{project}"]}`。仅当项目没有 Cadence 配置路径时使用这组命令。argv 不经过任意命令拼接，参数写入 request.json；受信环境脚本单独加载。

| 动作 | 要求的证据 |
| --- | --- |
| probe | 写 workdir/identity.json，tool=Virtuoso，version/library/cell/view 均为真实读取的非空字符串 |
| prepare | 根据 request.json 生成试验副本和网表，不覆盖原设计 |
| simulate | 前台执行仿真，非零退出表示失败，不得自行脱离进程组或提交远程作业 |
| export | 写 metrics.csv 与 result.json，后者至少含 design 和 definition 版本 |

CSV 必需列：point,corner,metric,value,unit,status；可选 min,max。status 为 ok/failed/missing/not_run；无量纲单位为 `1`，失败点 value 可空。脚本需自行执行批准参数边界校验；平台还会限制有限数字、安全参数名、并发、超时和提交预算。

通用 probe 身份来自已配置脚本，本身不能证明脚本真实读了 Virtuoso；必须现场核对。原生适配器做了读取及身份比对，但版本兼容仍待公司实测。

## 首条真实链路验收记录

记录：CentOS/kernel/架构、浏览器、Virtuoso/ADE/Spectre 版本、PDK 修订、testbench 版本、身份证据、实际网表/日志、固定 PVT、手工测量和导出值差异、取消后许可证释放、一次失败/超时、优化总次数和最终候选复验。完成前只能称开发候选。
