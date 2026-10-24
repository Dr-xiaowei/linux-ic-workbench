# GitHub 优化建议与确认状态

核对日期：2026-09-25。原始公开 API 快照见 [v4-github.json](validation/v4-github.json)。以下是基于维护状态、文档和既有发布记录推导的产品建议，在 0.4 调研时尚未纳入。最新决定：IDEA-09/10 已确认并在 0.5 实施，IDEA-11 仍待判断，IDEA-12 备份取消不纳入。Verible 官方 macOS 二进制仅用于本轮开发验证，不随生产包分发。之前已经确认的 IDEA-01～08 保持不变。

## 候选与维护证据

| 项目 | 本轮可核对维护证据 | 适合借鉴的能力与限制 |
| --- | --- | --- |
| [Verible](https://github.com/chipsalliance/verible) | 未归档；默认分支提交 2026-09-23，发布 v0.0-4296-g0f262651 同日，约 1,944 stars；有解析器、规则文档、测试与持续构建 | SystemVerilog 格式/风格检查、语言服务器；不是综合或模拟仿真器。滚动构建需固定版本，C++20 构建要求与旧 Linux 运行要求分开核实。 |
| [Verilator](https://github.com/verilator/verilator) | 未归档；默认分支提交 2026-09-25，约 3,955 stars；官方用户手册与长期测试/版本历史 | Verilog/SystemVerilog lint 与仿真检查；不能代替 VCS 的全部语言支持或公司签核。GitHub latest-release API 返回 404，不据此判断停更。 |
| [restic](https://github.com/restic/restic) | 未归档；默认分支提交 2026-09-20，v0.19.1 发布于 2026-07-05，约 36,249 stars；稳定手册与维护记录 | 快照、内容去重、完整性检查和恢复；空间整理应由用户选择保留策略，不自动删工程数据。 |
| [Aider](https://github.com/Aider-AI/aider) | 未归档；默认分支提交 2026-05-22，最新正式 release v0.86.0 为 2025-08-09，约 49,179 stars | 借鉴 AI 修改与检查/测试反馈、Git 撤销体验；近期发布较慢，只作为成熟交互参考，不建议把整套 Python 应用直接装到 CentOS 6.9。 |

stars 只是快照，不作为质量或可靠性证明。许可证：restic 为 BSD-2-Clause，Aider 为 Apache-2.0；Verible/Verilator 的 GitHub license API 均返回 NOASSERTION，若实际分发必须逐项核对项目与第三方许可证。本次没有复制其源代码，也没有引入新依赖。

## 建议新增点

| 编号 | 建议 | 对你的实际价值 | 范围、成本与优先级 |
| --- | --- | --- | --- |
| IDEA-09 | **AI 修改 → 检查 → 带证据修复** | AI 修改 RTL 后，自动收集指定检查工具的错误行号并交给 AI 修复，减少来回复制报错；设定最多迭代次数，成功与否以工具返回为依据 | 在现有“手动检查”上增加受控闭环；中等成本，优先。可先 Verible / Verilator，再用同一入口接公司 VCS；Analog/SKILL 等仍需对应厂商工具。 |
| IDEA-10 | **轻量代码诊断与定位** | Code 里高亮错误、点击跳到行、格式化选定代码；后续可接工程内符号导航 | 参考 Verible LSP；先做诊断/格式化，按需启动并空闲退出，不全盘索引。中等成本，优先；不承诺 LSP 已覆盖全部 16 种语言。 |
| IDEA-11 | **AI 修改集与版本对照** | 一次需求改多个 RTL/Tcl/SDC 文件时，逐文件选择接受，查看相对 Git 基线的差异，能够回退该次 AI 修改 | 参考 Aider 的 Git/diff/undo。当前版本已经支持单文件原文/审阅/撤销，本建议新增多文件修改集和 Git 关联；中高成本，后续。不能自动提交用户此前未提交的修改。 |
| IDEA-12 | **按需增量备份与恢复预览** | 在更换工作机或服务器前，只备份变更部分；恢复前比较项目备注、代码和配置差异，选择恢复到新目录 | 参考 restic 的快照与去重；现版本只做整份个人元数据快照压缩及完全相同内容去重。本建议增加跨快照增量、加密和选择性恢复；中等成本，建议在数据量增大时做。源工程可按需选择，巨型波形默认排除但清晰列出未备份范围。 |

以上与既有 IDEA-07“AI 回归样本库”不同：IDEA-09 针对每次用户的代码编辑，当场用工具检查，不是版本升级前的模型评测。

## 一手依据与接入判断

- Verible 工具范围、lint/formatter/LSP 及构建要求：[官方 README](https://github.com/chipsalliance/verible/blob/master/README.md)。语法/风格诊断不证明电路等价。
- Verilator 的 `--lint-only` 与执行参数：[官方手册源文件](https://github.com/verilator/verilator/blob/master/docs/guide/exe_verilator.rst)。可复用工作台当前语言检查命令接口；仿真闭环仍需测试平台和明确预期。
- Aider 的检查/测试反馈：[官方使用文档](https://aider.chat/docs/usage/lint-test.html)；Git 操作参考：[Git integration](https://aider.chat/docs/git.html)。本次仅借鉴方法，不采用其外部模型默认配置。
- restic 的快照、内容标识和原子存储：[官方设计参考](https://restic.readthedocs.io/en/stable/100_references.html)。未来若集成，备份目标可以是本机或内网，不需要公有云。

IDEA-09 与 IDEA-10 的本轮实现见 [新功能指南](EDA_TASKS_AND_CODE_CHECKS.md)。新工具的二进制/运行时不默认兼容 CentOS 6.9：优先核对可用的兼容静态版本，或运行在内网较新执行节点上，经受限接口调用。未做组件安装、目标机运行、模型效果或商业 EDA 验证。
