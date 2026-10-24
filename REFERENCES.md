# 候选方案与参考记录

## 2026-09-21 · Virtuoso/Spectre 控制方案补充初筛

用户已明确优先验证 Virtuoso/Spectre，MCP 控制先参考 GitHub 成熟方案再执行。本次检索原仓库、README、标签/发布页，**未安装、运行或完整审计候选**。以下用于缩小后续评估范围，不是成熟度终审或采用决定；具体 tag 的源码、维护者响应、兼容性和运行成本需在实现前补证。

| 候选与一手来源 | 本次实际取得的证据 | 当前判断 |
| --- | --- | --- |
| [官方 MCP Python SDK](https://github.com/modelcontextprotocol/python-sdk)、[发布记录](https://github.com/modelcontextprotocol/python-sdk/releases) | 官方协议客户端/服务端 SDK；发布页展示 v2.2.0 / 9972c21，时间文字为 07 Sep 15:53；正文有 2.x 稳定线与迁移说明，Python 3.10+，MIT | 作为成熟协议层的优先参考，仍需锁定兼容版本；它本身不提供 Virtuoso/Spectre 控制 |
| [SkillBridge](https://github.com/unihd-cag/skillbridge)、[标签](https://github.com/unihd-cag/skillbridge/tags) | Python 调用 Virtuoso SKILL，有 docs/tests；标签页展示 releases/1.8.0 / ca6105e，2026-03-23，变更为 UNIX TCP sockets；有跨年标签历史；README 要求 Python 3.8+、IC 6.1.7 或 ICADV/M 以上，仓库标识 LGPL-3.0 | 值得优先深入核查的基础桥接；不是 MCP 服务或完整仿真平台，不能把声明版本范围当作公司版本验收 |
| [virtuoso-bridge-lite](https://github.com/Arcadia-1/virtuoso-bridge-lite)、[发布记录](https://github.com/Arcadia-1/virtuoso-bridge-lite/releases) | README 描述 Virtuoso/SKILL、Maestro、Spectre 和远程连接；发布页 v0.8.0 / 1978a10，文字时间 19 Aug 18:36，说明包括 csh/tcsh 传输修复和作者实验室的五个 Spectre 作业验证；仓库标识 MIT | 相关性强，有发布和修复证据；0.x 接口、实验室自报验证不足以证明本公司适用。README 指出两条控制路径可独立，集成时须防止误报已联通 |
| [virtuoso-cli](https://github.com/deanyou/virtuoso-cli)、[发布记录](https://github.com/deanyou/virtuoso-cli/releases) | Rust CLI/守护进程，README 说明基于上述桥接重写，覆盖会话、SKILL、Spectre；发布页 v1.3.5 / 0743fb1，文字时间 11 Sep 08:49，记录会话身份/配置及传输修复；仓库标识 MIT | 对会话管理有参考价值，1.x 不自动代表生产成熟；需核对目标 Linux 二进制兼容性、资源占用及真实工具测试 |

所有时间按网页原文记录，发布页未展开完整年份/时区的项不换算 UTC，也不冒充实时 API 数据；SkillBridge 标签页为缓存结果，不声称已核实当前最新 tag。

### 十项检查摘要与尚缺证据

| 检查项 | 协议 SDK | EDA 桥接候选 |
| --- | --- | --- |
| 1. 维护 | 有正式发布和迁移说明；最新有意义提交、维护者回应完整时间待核 | 标签/发布见上表；实际最新代码、长期维护与回应日期待核，不能只看 Release 名称 |
| 2. 社区与弃用 | 页面约 24.4k Star / 4k Fork，Issue 243 / PR 192，均为页面快照 | SkillBridge 页面 343 Star / 77 Fork、Issue 0 / PR 1；vcli 33 / 11、Issue 5 / PR 1；bridge-lite 页面统计不一致，本轮不取实时数值。归档标志与平均响应未核实 |
| 3. 部署 | Python 库与协议服务，内网离线依赖包需准备，升级兼容待验 | SkillBridge 需在 Virtuoso 加载 SKILL 桥；bridge-lite/vcli 涉及本地或 SSH/守护进程配置，安装、升级、卸载与普通用户权限未实测 |
| 4. 环境服务 | Python 3.10+；不要求云模型账号，GLM 是独立服务 | 需要真实 Virtuoso/Spectre 和授权；远端方式另需 SSH/共享路径，厂商费用独立，运行时/二进制架构按固定版本核对 |
| 5. 直接能力 | MCP 客户端/服务端协议能力 | 分别为 SKILL 桥、综合 Python 桥和 Rust CLI；都不能仅凭文档认定全首版动作已经支持 |
| 6. 改造 | 封装有明确参数/返回结构的工具，避免自写协议 | 复用会话绑定、参数读取和作业接口；对选定测试平台包装最小动作，精简暂不需要的 GUI/多主机功能 |
| 7. 新开发 | GLM 调用编排、业务权限和任务模型不由 SDK 自动提供 | 工程/会话身份校验、指标语义、幂等提交、预算与写回保护需平台补齐；单纯 SKILL bridge 还需仿真适配 |
| 8. 风险/外联 | SDK 不是执行隔离边界；服务地址、认证、跟踪/日志出口及依赖行为待固定版本审计 | 有任意 SKILL/命令能力、SSH 和工程写入面；需检查监听范围、凭据、目标主机与环境文件加载，生产仅暴露受控动作，不把任意 eval 直接交给模型 |
| 9. 许可 | 仓库声明 MIT；采用时保留版权/许可，依赖单独核查 | SkillBridge 标识 LGPL-3.0，复制/修改/分发方案需核对许可原文义务；其他两项标识 MIT，仍需检查固定版本及上游来源声明，不默认为公司自有代码 |
| 10. 综合 | 优先参考的协议层，未确定语言栈或固定版本 | SkillBridge 适合基础接口评估，另外两项补充会话/仿真经验；当前没有完成“成熟且适配公司版本的完整 MCP 控制器”证明 |

### 采用顺序与最小验证

A 直接采用完整桥接：仅当维护、版本、许可、权限和全部最小动作证据满足时考虑；当前未达到采用结论。B 复用协议 SDK 与已验证桥接/厂商脚本，补齐受控 EDA 动作，作为优先比较方向。C 自建小型适配器只在复用不满足时评估，说明缺口，不重写 MCP 协议。

先固定候选版本并完成隔离检查，再按确定路线验证：Virtuoso 会话/工程身份读取 → 工程副本的测试配置与导出/网表 → Spectre 提交、状态和结果 → 人工基线数值核对 → 超时/断线/重试/取消与资源行为 → 最后接入 AI。无成熟完整桥接时如实说明，不把旧 Virtuoso-Flow-Plugin Alpha 或新检索到的示例当作成熟替代品。

以上记录补充了 2026-09-20 调研未覆盖的桥接仓库；旧报告的“本轮未找到”应按当时检索范围理解，不是断言不存在其他项目。独立问答页面无需等待 EDA 适配即可单独验证内网模型连接，但不代表仿真闭环已验收。

## 最新补充：GitHub 成熟项目调研（2026-09-20）

[完整报告](docs/GITHUB_RESEARCH.md)核查 Optuna、MLflow、AiiDA、SiliconCompiler、Open OnDemand、Lmod，按十项标准比较维护、部署、适配与风险。2026-09-21 用户已确认纳入全部八项功能建议，正式映射与验收见 [PROJECT.md](PROJECT.md) 1.1 及对应功能章节；组件选型仍未确认。维护时间、版本、许可和调研结论保留 2026-09-20 快照含义，以下旧记录仅保留历史初筛背景。

[公开 API 摘录](docs/research/github-metadata-2026-09-20.json)保存查询时间、版本/提交、社区指标与回应样本。补证：Lmod 9.4.2 许可原文已确认为 MIT；Virtuoso-Flow-Plugin 仍缺乏成熟度证据，不因 MCP 相关性而推荐为生产底座。所有候选均未安装运行，没有公司 Linux/EDA 实测。

## 历史初筛与接口参考

调研日期：2026-09-20（Asia/Shanghai）。本轮为**文档级初筛**：只读本地已有 Harness 包和公开项目资料，未下载、安装或运行候选；未做完整代码安全审计或 Linux 实测。

**最新适用说明：** 用户要求先完成全部需求，再制定开发计划，并补充公司内网已部署 GLM 5.3。历史初筛不构成已确认路线，原 B 路线倾向与 MVP 建议均未生效。新增 MCP/自动优化需求的公开资料核对见下方补充；用户内网模型和 EDA 接口仍未连接或实测。

## A / B / C 比较

| 路线 | 可以复用什么 | 缺口与成本 | 本轮判断 |
| --- | --- | --- | --- |
| A 直接使用现成工具组合 | Harness 提供 AI 会话与工作区，现有环境脚本/Lmod 管理环境，文件管理器打开资料 | 配置成本较低，但软件、项目和运行记录仍分散；不形成统一 IC 平台 | 可用于验证流程和需求 |
| B 在 Harness 上扩展 | UI 布局、会话、文件工具和插件机制；环境层按现状接入脚本或 Lmod | 新增软件目录、项目配置、运行记录、EDA 日志适配；需要跟踪预览版接口变动 | 历史初筛曾倾向此路线；现保留为候选，需按完整需求重新评估 |
| C 独立开发平台 | 借鉴 Harness 的交互思路，按需连接外部 agent 接口 | 自行维护 UI、会话、进程管理、权限和持久化，初期工作量最大 | 当 B 的接口或部署条件不能满足需求时再评估 |

“参考风格”不构成代码复用授权或已确定技术栈。完整需求与验收只在 [PROJECT.md](PROJECT.md) 维护；当前未划定 MVP。

## REF-01 · DeepSeek Harness

来源：[官方仓库](https://github.com/deepseek-ai/deepseek-harness)、[发布记录](https://github.com/deepseek-ai/deepseek-harness/releases)、[官方介绍](https://deepseek.com/harness/en/)。

本地只读证据：`../../deployment/package.json`、`../../deployment/node_modules/@deepseek-ai/dsh/package.json`，以及该作用域下 `dsh-client-ui-layout`、`dsh-client-ui-sidebar`、`dsh-client-ui-theme` 的 README.zh.md 和 lib/client.js。这些路径仅供当前工作区溯源，不作为新平台运行依赖。

| 检查项 | 结果与限制 |
| --- | --- |
| 1. 维护状态 | 本地 CLI 为 0.1.5-rc.1，layout 包为 0.1.5-rc.2。公开发布页可见 dsh-v0.1.6-alpha.2 / ddefc45，显示 09-17 13:30；页面未给出时区与完整年份，未换算。最新有意义提交和维护者回应时间未核实。官方注明开发者预览及兼容性变化 |
| 2. 社区与弃用 | 搜索快照显示约 229.6k Star / 27.5k Fork；不是当日 API 精确统计。Issue/PR 数、回应速度与归档标志未完整核实；不据热度判断成熟度 |
| 3. 普通用户部署 | 官方提供 Node.js 下运行 Web UI 的路径；本地已有 macOS 部署。目标 Linux 安装、升级/卸载和普通用户权限未实测 |
| 4. 环境与服务 | 需要兼容 Node.js 与相关本地依赖；模型服务另行配置，联网条件与费用取决于所选服务；离线部署方式待验证 |
| 5. 直接能力 | 插件化 agent、工作区/会话、文件与 shell 工具；本地包确认三栏和主题机制，不等于已具备 IC 业务功能 |
| 6. 可改造 | 优先评估新增面板与工具适配；固定版本接口、升级成本和发行组合需进一步验证 |
| 7. 新开发 | 软件/环境配置、项目元数据、运行记录关联、日志来源定位和 EDA 适配；分别属于新增业务与工具适配 |
| 8. 安全风险 | shell/文件工具可操作用户数据，模型请求可能发送上下文；凭据、外联、遥测与插件安装路径尚未完整审计。拟限定目录和数据去向；实际关闭配置待选型验证 |
| 9. 许可证 | 仓库及本地包声明 MIT；拟复用时保留许可和版权声明，逐项核对依赖/素材与 THIRD_PARTY_NOTICES；未做完整分发审查 |
| 10. 综合适配 | UI 和 agent 层适配度较高，适合作为 B 路线候选；预览版变化与 Linux 兼容性是待验证项，不自动升级现有部署 |

## REF-02 · Lmod

来源：[官方仓库](https://github.com/TACC/Lmod)、[官方文档](https://lmod.readthedocs.io/en/latest/)、[发布记录](https://github.com/TACC/Lmod/releases)、[许可文件](https://github.com/TACC/Lmod/blob/main/License)。

| 检查项 | 结果与限制 |
| --- | --- |
| 1. 维护状态 | 文档页面显示 9.4.2；发布页已读到 9.4 / c3f11ad，显示 09-08 09:39，页面未给出时区与完整年份。未确认最新 tag、最新提交或维护者回应时间，不把这条记录当最新版本 |
| 2. 社区与弃用 | 搜索/页面快照约 606 Star，Fork 为 143–144，缓存不一致；许可页侧栏快照 Issue 26 / PR 7。全部不是当日实时值，响应与归档情况未核实 |
| 3. 普通用户部署 | Unix 环境管理工具，是否已有系统安装需询问；安装权限、升级/卸载和发行版包需实测 |
| 4. 环境与服务 | Lua 环境与 shell 集成；具体依赖随版本/安装方式确认，不需要 AI 账号；EDA 授权服务独立 |
| 5. 直接能力 | 通过 modulefile 动态调整环境和切换软件版本；不是 EDA 安装器、项目管理器或 GUI 工作台 |
| 6. 可改造 | 可作为可选环境适配器，优先复用现有 modulefile；已有脚本也应可用 |
| 7. 新开发 | 图形配置与项目绑定、进程环境隔离、失败诊断；属于平台适配层 |
| 8. 安全风险 | modulefile/环境脚本是可执行配置，不应当作无副作用的数据。9.4 发布说明含安全修复；实际采用前核对修复版本。遥测、外联与脚本执行未审计 |
| 9. 许可证 | 已定位 License 文件，但网页正文未成功提取，原始文件请求失败；具体条件未核实，分发前必须读取拟采用 tag 原文 |
| 10. 综合适配 | 软件环境管理的候选组件，不作为完整平台底座；是否需要引入取决于用户现有环境 |

## REF-03 · FuseSoC

来源：[官方仓库](https://github.com/olofk/fusesoc)、[发布记录](https://github.com/olofk/fusesoc/releases)、[官方文档](https://fusesoc.readthedocs.io/)。

| 检查项 | 结果与限制 |
| --- | --- |
| 1. 维护状态 | 本轮读取 main README 和发布页，其中包含 2.4.5 / a8730bd；页面显示 11-13 13:58，年份与时区未展开，不能判为当年或最新版；最新提交与维护者回应日期未核实 |
| 2. 社区与弃用 | 页面快照约 1.5k Star / 281 Fork、Issue 137 / PR 14；不是实时 API 统计；归档状态与回应速度未核实 |
| 3. 普通用户部署 | Python 命令行工具，通过项目描述组织构建；普通设计用户仍需准备后端工具与工程配置，未实测安装/升级/卸载 |
| 4. 环境与服务 | Python 与具体 EDA 后端；依赖下载需要网络或预置缓存；具体版本和商业工具授权条件另核 |
| 5. 直接能力 | 面向 FPGA/ASIC 的包管理与构建抽象，适合评估数字设计 IP/构建组织 |
| 6. 可改造 | 可借鉴工程清单与构建目标，或作为数字设计流程的可选适配器 |
| 7. 新开发 | 统一 UI、个人文件、AI 和模拟/版图等领域的集成仍需新增；不预设用户需要转换现有工程 |
| 8. 安全风险 | 库获取、生成器和构建步骤可能下载内容或执行代码；必须核对来源/版本与命令，实际网络和权限行为未审计 |
| 9. 许可证 | 官方 README 声明 BSD-2-Clause；复用时保留相应声明，所获取 IP 核和后端另有许可；拟采用 tag 原文待核对 |
| 10. 综合适配 | 用户现已明确模拟优先并兼顾数字 DE；保留为数字流程可选参考，不列为模拟首版必选依赖 |

## 2026-09-20 · 场景细化补充证据

本节为已有商业工具能力和环境接口的资料核对，不是新增开源底座的采用决定。只阅读公开网页，不调用工具、不下载脚本；版本与许可可用性必须结合公司实际安装进一步确认。

| 参考 | 核实范围 | 对需求的影响与限制 |
| --- | --- | --- |
| [Lmod 安装文档](https://lmod.readthedocs.io/en/latest/030_installing.html) 与 [Tcl modulefile 文档](https://lmod.readthedocs.io/en/latest/051_tcl_modulefiles.html) | 文档提供 csh/tcsh 初始化方式与 source-sh 的 shell 支持说明 | 可评估复用现有脚本的环境适配；不必强制改用 bash。未验证公司环境脚本、alias 和图形会话 |
| [Cadence Custom Design Migration](https://www.cadence.com/en_US/home/tools/custom-ic-analog-rf-design/virtuoso-studio/custom-design-migration.html) | 官方介绍器件/参数映射、设计优化和版图迁移流程 | 优先核对已有产品能力，平台组织映射与回归；不假定当前许可证包含这些功能，不承诺自动迁移后达标 |
| [Cadence ADE Assembler 数据手册](https://login.cadence.com/content/dam/cadence-www/global/en_US/documents/tools/custom-ic-analog-rf-design/virtuoso-ade-assembler-ds.pdf) | 官方资料将多测试与运行/工艺/环境条件验证列为能力 | 仿真结果模型需要保留测试与工况维度；具体结果导出接口、命令与版本待本地文档确认 |
| [Cadence ADE XL 导出说明](https://support1.cadence.com/public/docs/content/20469912.html) | 官方页面说明 Outputs Setup 定义可导出 CSV | 输出定义与实际测量结果是不同对象，不能凭该文档声称已验证仿真结果/波形导出 |
| [Cadence Innovus 数据手册](https://www.cadence.com/zh_TW/home/resources/datasheets/innovus-implementation-system-ds.html) | 官方产品归属与数字实现定位 | 将 Innovus 与 Synopsys 软件分开登记，尚未确认任何自动化接口 |
| [华大九天官网](https://www.empyrean.com.cn/) 与 [官方英文站](https://www.empyrean-tech.com/) | 产品范围；英文站列出 Aether 设计环境与 ALPS 仿真器 | 保留 ALPS、Aether、Demeter 用户工具范围，Demeter 具体模块及各产品本机接口未核实，不推定与 Cadence 脚本兼容 |

调研也发现 Cadence 社区的 OCEAN/CSV 导出讨论，但本轮不采用社区示例脚本；下一步以用户实际 ADE 版本文档和脱敏样本验证指标、测试点与波形的导出路径。

## 下一阶段选型必须补齐

上述记录足以支持需求讨论，不足以批准生产依赖。用户确认运行环境和范围后，补齐候选准确 tag/commit、UTC 时间、当日社区统计、维护回应、许可原文、安装步骤和依赖/外联审查，再在隔离的 `reference-sandbox/` 中按授权验证。当前没有候选代码下载，也没有宣称审计通过。

## 2026-09-20 · MCP 控制与自动调参可行性

结论来自官方协议/产品资料和候选 README 的只读评估。没有安装 MCP 服务、下载候选代码、运行优化或连接公司模型/EDA。MCP 控制方向和自动调参需求已确认，但具体实现路线未确认。

### 协议与厂商依据

| 来源 | 本次核实的事实 | 应用判断与待验证项 |
| --- | --- | --- |
| [MCP 架构](https://modelcontextprotocol.io/docs/2026-07-28/learn/architecture) | host 管理 client 与 server 连接，服务可在本地或远端运行 | 平台可作为 host，EDA 能力由 server 提供；不要求 GLM 模型端原生连接 MCP |
| [MCP Tools 规范 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/server/tools) | 工具具有参数 schema，调用有协议及执行错误反馈 | 可设计明确的 EDA 动作；不能从 MCP 协议推导 EDA 自带接口、权限隔离或长任务恢复。此固定版本用于概念核实，不声称是最新协议或已选版本 |
| [Cadence ViVA 官方数据手册](https://www.cadence.com/content/dam/cadence-www/global/en_US/documents/tools/custom-ic-analog-rf-design/virtuoso-visualization-analysis-ds.pdf) | 描述 OCEAN 命令行批量仿真与波形后处理，以及 SKILL/OCEAN 脚本能力 | 支持评估脚本封装；资料较早，具体 ADE/Spectre 接口必须按安装版本核对 |
| [华大九天 PyAether 官方介绍](https://www.empyrean.com.cn/solutions/index-129.html) | 描述基于 Aether 的 Python API、电路/版图自动化和第三方集成 | 可评估 Python 适配；不推定当前 Aether 已包含相应功能，也不扩展为 ALPS/Demeter 的接口证明 |
| [MATLAB Engine API 官方文档](https://www.mathworks.com/help/matlab/matlab-engine-for-python.html) | Python 可通过 Engine 调用 MATLAB，要求实际安装 MATLAB，不能仅凭 Runtime 使用 | 后处理可考虑直接 Engine 或再包装 MCP；Python/MATLAB 版本和许可证需验证 |
| [Cadence Virtuoso ADE Suite](https://www.cadence.com/en_US/home/tools/custom-ic-analog-rf-design/circuit-design/virtuoso-ade-suite.html) 与 [IC23.1 ISR3 官方说明](https://community.cadence.com/cadence_blogs_8/b/cic/posts/virtuoso-ic23-1-isr-3-now-available) | ADE 产品资料有优化分析；ISR3 说明提及 Assembler/Variation Option 的 Advanced Optimization Method | 自动优化可先比较已有厂商能力；不默认公司已有许可证，不能据此保证任意电路可收敛 |

[AOP 官方 RAK 页面](https://support1.cadence.com/public/docs/content/20509363.html) 的搜索摘要提到 ADE Assembler 与 AOP，直接打开返回错误，未读取完整内容；不作为具体 API/安装操作依据。未使用第三方转载商业手册作为实施规范。

### 复用路径 A / B / C（对 EDA 控制和优化，不替代平台底座选型）

| 路线 | 能复用的部分 | 缺口与判断 |
| --- | --- | --- |
| A 直接使用公司现有脚本/厂商优化功能 | 已验证的 EDA 操作、测试配置和优化器 | 最适合先核查是否已有可用能力；统一项目/结果、MCP 暴露和跨工具流程仍需平台整合 |
| B 复用脚本/API，适配为 MCP 与平台公共执行能力 | 厂商接口及经审查的桥接实现 | 本轮建议方向；需完成会话/权限、参数约束、任务状态和结果来源管理；无成熟候选证据时不能直接安装 |
| C 自建 EDA 适配与优化编排 | 按实际版本建立明确动作与实验记录 | 可控但适配与维护成本高；数值算法/EDA 引擎仍可复用，不建议无理由从零重写 |

需求验证用例（不构成首版范围）：针对一个真实测试平台，读取允许的变量，修改一个候选参数并运行仿真，查回指标与任务状态；对一个有界优化任务验证停止/恢复与候选复验。技术路线在完整需求评审后确定。

### REF-04 / REF-05 · 社区候选初筛（未采用）

来源：[Virtuoso-Flow-Plugin](https://github.com/VlHash/Virtuoso-Flow-Plugin)（VFP）；[mcp-skill-server](https://github.com/Olderdriver/mcp-skill-server)（SKILL 文档检索）。读取 main 页面，未锁定 commit/tag；不是成熟度或安全审计结论。

| 十项检查 | VFP 控制桥接候选 | mcp-skill-server 文档检索候选 |
| --- | --- | --- |
| 1. 维护状态 | 页面可读；最近有效提交、Release、维护者回应日期及时区未核实 | 最近提交、Release、回应与时区未核实 |
| 2. 社区与弃用 | 页面快照 Fork 0、Issue 0、PR 0，Star 数未返回；实时统计/归档状态未核实 | 当日 Star/Fork/Issue/PR、归档状态未核实 |
| 3. 普通用户部署 | README 描述 Virtuoso 内插件 + Python 桥接进程；未安装，权限/升级/卸载未测试 | 未安装，部署及卸载条件未核实 |
| 4. 环境与服务 | README 以 IC23.1 SKILL API 为开发目标，Python 桥接；公司环境和许可证需核对 | 模型/索引依赖、离线能力和费用未核实 |
| 5. 直接能力 | README 描述设计上下文、参数修改、仿真和结果；全部为项目方声明，未独立验证 | 定位为 SKILL 文档 RAG MCP；不能据名称认定可控制 Virtuoso |
| 6. 可改造 | 可研究会话桥接和任务/结果设计，实际 MCP 端点覆盖仍需审查 | 如需要知识检索，可研究；与执行适配器职责不同 |
| 7. 需新开发 | GLM/平台整合、其他工具与版本、公司权限和升级兼容仍有缺口 | 无法代替本需求的 EDA 执行层 |
| 8. 安全风险 | 持久桥接可改设计/运行程序；鉴权、监听地址、脚本、外联与依赖未审计 | 设计手册导入、索引和模型数据去向未审计 |
| 9. 许可证 | 仓库标注 Apache-2.0；拟用 commit 的原文、依赖和素材未核对；采用时保留许可/必要 NOTICE 并标记修改 | 许可条件未核实，不默认可复制或分发 |
| 10. 综合适配 | 仅保留为 Virtuoso 控制参考；不宣称其成熟或适合生产，不采用其自述作为验收证据 | 排除为 EDA 控制方案，保留为可选知识检索参考 |

自动调参建议由 AI 理解目标与解释结果，优化器产生合法候选，EDA 给出实际性能。可比较厂商优化、已有内部算法与外部搜索方法；本轮未挑选数值库，也不承诺最优解、固定提速或无需工程师复验。

## 2026-09-21 · 首版实际采用与兼容性证据

用户已授权按首版方案开发，并确认 x86_64、模型接口可配置。前文“未下载/未采用”描述的是当时调研阶段；本节与交付报告描述当前实际使用情况。

| 来源/固定版本 | 本次判断与复用 | 验证/限制 |
| --- | --- | --- |
| [Go Linux 支持表](https://go.dev/wiki/Linux)、[最低要求](https://go.dev/wiki/MinimumRequirements)、[Go 1.24](https://go.dev/doc/go1.24) | Linux 2.6.32 对应 Go 1.23.12；Go 1.24 提高到 3.2。采用静态 Linux amd64/v1 构建，规避系统 libc/Node/Python 依赖 | 官方 Go 1.23.12 下载 SHA-256 已核对，ELF 无 PT_INTERP/PT_DYNAMIC；未在 CentOS 内核启动；历史工具链不等于当前维护分支 |
| [CentOS 6.9 发布说明](https://wiki.centos.org/Manuals%282f%29ReleaseNotes%282f%29CentOS6%282e%299.html) | 将旧内核/用户空间与浏览器作为独立目标验收 | 本地为 macOS ARM64，无公司 CentOS 虚拟机或访问凭据，不能宣称实机兼容已保证 |
| [官方 MCP Go SDK v1.0.0](https://github.com/modelcontextprotocol/go-sdk/tree/v1.0.0)、[go.mod](https://github.com/modelcontextprotocol/go-sdk/blob/v1.0.0/go.mod) | 官方稳定 v1 协议 SDK，Go 1.23；MIT。实际编译依赖 v1.0.0，复用协议/传输/JSON Schema，EDA 业务适配自建 | SDK client/server 和真实 stdio 子进程 initialize/tools/list/call、只读门控通过；不是选择未经核验的“全栈 EDA MCP”。较新版内核/Go 门槛需另行复核 |
| [jsonschema-go v0.3.0](https://github.com/google/jsonschema-go/tree/v0.3.0)、[uritemplate v3.0.2](https://github.com/yosida95/uritemplate/tree/v3.0.2) | SDK 的间接运行依赖，分别 MIT/BSD-2-Clause，保留许可证 | go.sum 锁定，go mod verify 通过；AppleDouble 元数据导致的目录校验偏差已清理后复核，无修改上游源文件 |
| [Cadence 社区：只读 cellview 参数](https://community.cadence.com/cadence_technology_forums/f/custom-ic-skill/43826/usage-of-commands-dbopencellviewbytype-and-deopencellview-and-difference-between-gegetwindowcellview-and-gegeteditcellview/1366435) | 原生 probe 使用正确位置的 access mode，显式 r | 自己编写适配代码；生成与身份匹配合同已测，实际安装版本 SKILL 支持未验收 |
| [Cadence 社区：desVar 与 OCEAN](https://community.cadence.com/cadence_technology_forums/f/custom-ic-skill/38920/how-to-pass-desvar-using-include-file-in-ocean-runs/1355539)、[OCEAN/Spectre 示例说明](https://community.cadence.com/cadence_technology_forums/f/custom-ic-design/24495/spectre-sweep-statement-in-ocean/1317845) | 正确区分 OCEAN/SKILL 与 Spectre 网表语法，设计变量通过 desVar，在公司审核的固定脚本环境中执行 | 不复制商业手册或直接运行社区脚本；公司模型/测量表达式仍需工程师确认 |
| [ngspice 官方 RC 样例](https://github.com/ngspice/ngspice/blob/032b1c32c4dbad45ff132bcfac1dbecadbd8abb0/examples/measure/rc-meas-ac.sp) | 固定历史提交的公开电路与 COPYING 放在测试/Demo；用独立节点方程与闭式传递函数生成解析数据 | 41 频点、82 指标，最大复数绝对差 5.93e-17；不声称执行过 ngspice 或商业 EDA |

### A/B/C 选择补充

A：直接部署现成 MCP/Virtuoso 项目不能证明本公司的 CentOS 6.9、ADE、GLM 和持久数据需求得到满足。SkillBridge/Python SDK 等常需较新 Python；virtuoso-cli 的目标 libc/内核尚未验证，不嵌入生产候选。

B（本版）：复用官方成熟协议 SDK，围绕 Cadence 原生 OCEAN/SKILL 及公司已有脚本做受控动作适配。自有业务层承担项目、证据、预算、版本隔离与双语界面，明确实际 EDA 适配尚待实机验证。

C：不从零实现 MCP、通用波形解析、EDA 引擎或复杂优化库。首版数值候选采用小型确定性 Halton 搜索，避免额外现代运行时；性能取舍/最优性需实测，不声称其优于厂商优化器。

### 部署、权限、许可与维护

运行依赖：用户已有 Linux x86_64/浏览器、可选 GLM、已授权的 EDA/PDK。核心服务无遥测、无默认公网调用、不自动下载更新；MCP stdio 与应用 API 只连接回环地址。用户主动选取的证据只送往显式配置的模型 URL。环境/测量脚本是受信任执行面，普通问答与未授权模型工具不能执行任意 shell/SKILL。

复用依赖许可证原文保存在 licenses/，公开样例随 COPYING。本次完成固定版本接口检查、模块校验和本地运行测试；不声称全面安全审计或上游所有 issue/维护回应均已复核。此前候选维护/社区证据见原调研表；本版 SDK 选择依据官方稳定接口与目标工具链约束，不以 Star 数替代成熟度。目标旧内核与现代依赖安全更新存在长期维护成本，后续升级需重新验证。

## 2026-09-23 重构参考

- 本地 `@deepseek-ai/dsh` / `dsh-web-frontend` 0.1.5-rc.1，MIT；读取随包 README、CSS 设计变量与导航/对话资源，参考白色画布、浅灰侧栏、蓝色强调、居中输入和工具进度。本次界面独立实现，没有复制 Harness 代码或素材，没有引入其 Node 运行时。
- [Linux /proc 官方文档](https://docs.kernel.org/filesystems/proc.html)：CPU 累积计数、进程身份与内存字段；老内核缺少 MemAvailable 时使用兼容估算。在线阅读，不代表 CentOS 实测。
- [NVIDIA SMI 官方说明](https://docs.nvidia.com/deploy/nvidia-smi/)：GPU 使用率/显存查询；不支持项保留未知，未连接 GPU。
- [Icarus Verilog 官方文档](https://steveicarus.github.io/iverilog/) 与 [VCD 写出实现](https://github.com/steveicarus/iverilog/blob/master/vpi/sys_vcd.c)：核对通用 VCD 概念与导出路径；没有复制源码，没有安装编译器，不能宣称实际 HDL 编译通过。
- 继续使用已固定、已审查的官方 MCP Go SDK v1.0.0；本次没有引入新的第三方运行依赖。新增测试夹具是有明确预期值的自建 CSV/VCD/proc/模型合同数据，不冒充厂商仿真输出；原公开 RC 数学交叉验证保留。

## 2026-09-25 EDA MCP 客户端与页面调整

已查询官方 MCP Go SDK、SkillBridge、OpenROAD-MCP 与 virtuoso-bridge-lite 原仓库，具体来源、版本观察、许可证、兼容性和 A/B/C 结论见 [EDA MCP 调研与接入](docs/EDA_MCP.md)。复用已 vendor 的官方 SDK v1.0.0，未引入或安装上述 EDA 候选作为依赖。计算器为受限 Go 表达式解析器，不引入浏览器数学运行时；标准算式与错误/边界有自动化验收。

## 2026-09-25 补充调研

[Code 检查与本地数据优化候选](docs/GITHUB_RESEARCH_V4.md)：维护元数据见 docs/validation/v4-github.json。仅新增建议待评审，不改变已确认 IDEA-01～08。

## 2026-09-26 终端打开 GUI 与 CentOS 原生浏览器验证

- [xdg-open 官方说明](https://portland.freedesktop.org/doc/xdg-open.html)：复用桌面默认打开器，仅桌面会话可用；不创建新的浏览器/桌面框架。
- [CentOS 6.9 官方归档](https://vault.centos.org/6.9/)：隔离兼容环境使用官方 RPM，76 个 RPM 的签名和下载 SHA256 已保留；历史系统不再获得维护，不作为公司部署浏览器推荐。
- [Mozilla ESR52 Marionette 原始实现](https://hg.mozilla.org/releases/mozilla-esr52/raw-file/tip/testing/marionette/driver.js)：测试客户端参考公开消息协议，无 Mozilla 代码复制到产品，不依赖新 Selenium/geckodriver。
- 取舍：A 复用已有 xdg-open/浏览器；B 用现有 Go 单进程增加打开请求与错误反馈；C 不重写桌面框架。没有新增产品依赖，仅验证环境增加 Firefox/Xvfb。

## 2026-09-26 原生桌面选择与真实小模型验收

本轮交付覆盖旧浏览器方式，选择系统 Tk 原生控件；并未复制其他项目界面代码。A/B/C：A 优先复用 Linux 自带成熟 Tk；B 复用当前 Go API、权限与本地数据，增加薄客户端；C 不自研窗口系统、不把浏览器套壳当原生客户端。

| 评估项 | Tk / Python（选用） | Qt / PyQt（未选） | Tauri / WebKit（未选） |
| --- | --- | --- | --- |
| 来源 | [Tcl/Tk 官方组织](https://github.com/tcltk) | [PyQt 官方](https://www.riverbankcomputing.com/software/pyqt/) | [Tauri 前置依赖](https://v2.tauri.app/start/prerequisites/) |
| 维护 | 当前上游仍发布；本次旧 Tk 8.5.7 已停止维护 | 现行 PyQt 维护，旧 PyQt4 不受支持 | 现行 Tauri 维护 |
| 验证版本 | CentOS 官方 tkinter 2.6.6 / Tk 8.5.7 | 未安装/未实测 | 未安装/未实测 |
| 成熟度 | 长期系统 GUI 工具，实际原生控件可用 | 成熟完整桌面框架 | 成熟 WebView 桌面框架 |
| 许可证 | [Tk BSD 类许可](https://github.com/tcltk/tk/blob/main/license.terms)，Python 系统解释器许可 | PyQt GPL / 商业；PySide 另有许可边界 | Tauri 双许可；WebKit 多组件许可 |
| CentOS 6.9 兼容 | 使用官方归档签名 RPM 实测，不带新 glibc | 现代版本兼容旧系统未证明 | WebKitGTK 4.1 等现代依赖不适合当前基线 |
| 权限/数据 | 当前 Linux UID，本地 Tk + 鉴权回环 API | 未接入，无数据外传 | 未接入，无数据外传 |
| 资源 | 原生 Python/Tk 进程+Go 后台，实际测量另见报告 | 无本轮测量，不推断 | [官方发行基线说明](https://tauri.app/distribute/appimage/)，无本轮测量 |
| 复用/风险 | 不捆绑 toolkit，公司维护系统旧依赖；少量 Python 2.6 兼容层 | 会增大发布/许可评估范围 | 不符合本轮禁止网页版正式客户端的要求 |
| 采用结论 | 原生轻客户端，测试通过范围详见报告 | 未来公司更新 Linux 基线后再评估 | 仅作为技术比较，不用于正式版 |

- [CentOS 6.9 官方归档](https://vault.centos.org/6.9/os/x86_64/Packages/)：本轮安装 Tk/Tcl/Tix/tkinter 四个系统 RPM；签名、路径和哈希保留在专用验证环境，无公司数据参与。
- [Qwen3-1.7B 官方量化仓库](https://huggingface.co/Qwen/Qwen3-1.7B-GGUF)：固定 revision `90862c4b9d2787eaed51d12237eafdfe7c5f6077`，`Qwen3-1.7B-Q8_0.gguf`，1,834,426,016 字节；SHA256 `061b54daade076b5d3362dac252678d17da8c68f07560be70818cace6590cb1a`。通过官方 Qwen ModelScope 镜像下载，逐字节大小与固定 HF LFS SHA 对比通过。模型独立于产品交付，保留 Apache-2.0 来源信息，不捆绑权重。
- [Icarus Verilog 官方项目](https://github.com/steveicarus/iverilog)：仅验收环境使用 13.0（GPL-2.0+）；Homebrew 官方 bottle 的 SHA256 与下载元数据核验。真实编译/执行四输入组合 AND 测试；不作为厂商 EDA 或 CentOS 内 Icarus 运行证据。
- llama.cpp b11146、0.6B 及 1.5B 的原验证 manifest 继续保留。无外部商业模型兜底、无真实工程上传；0.6B 和 1.7B 的失败任务均保留。
