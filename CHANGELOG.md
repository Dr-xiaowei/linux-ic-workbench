# 变更记录

## 2026-10-13 · 0.8.0-rc3 候选

- 精简安装包至约 3.6 MB：保留全部运行功能、当前使用说明和许可证，历史验证附件留在源码项目。
- 开发工具保留官方压缩包，临时解压离线编译；清理已验证可恢复的工具展开副本、编译/模块缓存及重复打包目录，保留 QEMU/镜像/模型和个人数据。
- 修正两条权限测试对 umask 的依赖，在 umask 077 下验证；交付增加体积和完整性检查，详见 [报告](docs/ENGINEERING_RC1.md)。

## 2026-10-12 · 0.8.0-rc2 候选

- 修复单位换算后数值/阈值溢出和下溢误比较，拒绝非法文本 CSV。
- 路径错误及外部修改冲突不再让修改集卡在 checking；保留原文件和此前状态。
- 保留最终申报书与所有发布候选，清理本地过程稿和经逐文件确认的重复打包目录。

## 2026-09-30 · 0.8.0-rc1 候选

- 参数模板、单位与工况 CSV 比较、AI 多文件候选/采用/撤销，Schema 7 保留旧记录。
- AI 对话折叠结构化工具回执、保留原始证据；拒绝伪工具文本误报完成。
- 排版明确整份/选区、预览与差异；浅绿双语布局与原生异步保存修复。
- CentOS 与真实小模型验证及限制见 [本轮报告](docs/ENGINEERING_RC1.md)。

## 2026-09-29 · 0.7.0-rc2 候选

- 修复原生后台队列满后保存请求无法恢复的问题，保留未保存草稿并允许重试。
- 原生客户端与网页 Demo 的文件浏览、搜索、预览只采用最新请求结果，换目录/查询不混用旧结果；搜索词变化后不复用旧游标。
- 不新增依赖、持久化格式或权限；历史安装包保留。验证与改进建议见 docs/DESKTOP_RC2.md 和 docs/OPTIMIZATION_REVIEW.md。

## 2026-09-26 · 0.6.0-rc2

- 增加 `--open` / `--browser`，后台就绪后打开默认浏览器；重复打开复用本人实例，无桌面和打开失败时提供明确反馈。
- 补齐 Firefox 52 的文件读取、Promise.finally、剪贴板和 CSS 布局回退，不引入额外运行依赖。
- CentOS 原生 Firefox GUI 7 组与 Chrome SSH/Demo 十页检查通过；宿主 race 68 项、CentOS 66 项通过/2 项缺工具跳过。详细证据见 docs/TERMINAL_GUI.md。

## 2026-09-26 · 0.6.0-rc1

- AI 对话新增 Linux Agent：每轮关闭/确认/自主选择，系统查询、目录/文本文件与非交互命令；真实 stdout/stderr、退出码、PID、超时和进程组取消。
- 文件原子发布和 SHA256 冲突检查；原生 JSON 对象工具参数，拒绝把模型普通文本当作工具执行；没有实际调用时明确显示未执行。
- 最近任务回执本地持久化与重启中断恢复；输出/轮数/调用次数有界，不新增常驻依赖，Schema 6 / 草稿 Schema 1 不变。
- 网页 Demo 仅模拟 Linux 操作，浅绿双语界面优先显示终端回执；补充模拟 IC 研发适用性评估及建议。

## 2026-09-26 · 0.5.0-rc3

- 新增个人终端命令 `~/.local/bin/ic-workbench`、`--status`、重复启动复用当前实例；默认动态端口，旧固定配置可在占用时回退，明确 `--listen` 严格保持指定地址。MCP 入口自动定位本人实例。
- 生产配置/数据校验真实目录、UID、0700/0600、链接与独占锁；初始配置并发创建及安装发布采用原子流程，升级不覆盖个人文件。
- 数据写入失败回滚、目录同步失败后停止后续写入、异常退出临时文件清理；关闭时取消启动验证并等待请求和任务结束。过期网页令牌可重新登录。
- 实际 CentOS 双 UID、跨用户访问拒绝、620 请求短时负载、16 请求草稿冲突、SIGKILL 写入恢复、真实 ENOSPC 恢复与 MCP CLI 自动发现通过。详见可靠性复验报告；环境文件继续保留，等待用户确认清理。

## 2026-09-26 · 0.5.0-rc2

- QEMU 实际 CentOS 6.9 x86_64 正式模式非 EDA 验收：文件权限、sh/csh、系统 Python 2.6 插件、资源监测、模型/Agent、Code、存储/升级及浏览器。
- 修复 Python 2.6 py_compile 元组错误的诊断行号/列号；修正插件安装说明路径和部署文档。
- 实测本地 Qwen3 0.6B 与 Qwen2.5-Coder 1.5B；记录生成不完整和复位语义错误，保留修正过程，不把代码输出视为芯片验证。
- 环境独立、验证后等用户确认清理；报告和交付包保留。原 Demo/个人数据不重置。

## 2026-09-25 · 0.5.0-rc1

- 取消手动项目/个人备份入口与 Agent 创建动作，旧 API 返回错误；项目只登记路径、备注和配置，旧数据/备份保留。
- EDA 控制增加绑定当前 MCP 连接的 AI 任务对话、工具白名单、实时进度、取消、持久回执与重启中断状态。
- 纳入 IDEA-09/10：可配置检查器/格式化器、诊断跳行、最多 1–3 次 AI 修复与复检、全文件和选中行格式化；候选审阅后采用。
- Schema 6 无损升级，EDA 草稿和最新任务恢复；继续浅绿双语与网页 Demo / Linux 数据隔离。
- 真实 Verible/clang、本机模型与 MCP 协议、Go race 和浏览器验证；公司 CentOS/GLM/商业 EDA 仍待现场验收。

## 2026-09-25 · 0.4.0-rc1

- 主 Schema 5，独立压缩草稿 Schema 1：Code 原文/候选/历史、分析历史/答案、计算器、未发送对话；约 1 秒防抖自动保存、状态提示、冲突保护、恢复导出。
- 主记录紧凑 JSON；备份压缩并按内容去重；设置页显示分类占用；保留全部旧数据和备份，不复制项目目录。
- Code 支持 16 类格式与常见扩展名导入，AI 基于实际输入提出修改、并排审阅、采用/撤销、导出原文/建议；过期建议不能覆盖新编辑。Demo 明确仅格式整理。
- Linux 候选与网页 Demo 同步；GitHub 新建议单独列出；测试与目标环境未验收说明见 COMPLETION_REPORT.md。

## 0.3.0-rc1 · 2026-09-25 · 页面职责调整候选

- HDL 扩展为 Code 平台：16 种电路代码类型、各语言草稿/导出及独立检查工具。
- 文件 AI 分析结果直接显示在当前页；新增支持公式、变量、工程后缀与科学函数的计算器。
- 仅 AI 对话选择项目/文件夹范围；项目页只管理目录、文件、备注与标签，历史执行/结果/优化数据保留。
- 新增 EDA 控制页面，使用官方 MCP SDK 接入内置 Cadence 或外部 stdio/Streamable HTTP 服务，支持工具发现、白名单、确认、进度、回执与取消。
- Schema 4 迁移前备份，保留旧数据；继续使用浅绿双语界面和静态 Linux 构建。商业 EDA、公司 GLM 与 CentOS 6.9 实机仍待验收。

## 0.2.0-rc1 · 2026-09-23 · 第一版重构候选

- 按用户偏好恢复浅绿色主题，统一侧栏、按钮、对话区、资源图表和波形配色；布局与功能保持不变。
- 八个独立工作区与 Harness 风格双语界面；生产 Linux 和网页 Demo 共用业务代码。
- 多目录项目、文件注释和流式关键词搜索；统一 Agent 工具、进度、回执和审批。
- 软件版本环境、可安装插件、资源监测与任务关闭、CSV/VCD 波形、HDL 双栏工作台、配置自动加载验证。
- Schema 3 无损迁移；原仿真结果、任务、优化和 MCP 能力保留，Demo 不生成安装包。
- 真实 CentOS 6.9、公司 GLM、商业 EDA、GPU 和 HDL 编译器仍待目标环境验收。

# 重要变化

## 2026-09-23 · 网页 Demo 预览

- 按用户要求移除独立 Linux Demo 安装包及其暂存副本，打包流程仅生成生产候选包。
- 增加本机网页预览入口，自动载入解析 RC 演示数据，继续复用完整双语界面与模拟后端；生产交付和用户历史数据保留。

## 0.1.0-rc1 · 2026-09-21 开发候选（未正式发布）

- 实现首版八组功能：独立/项目 AI 问答、带来源的结果与日志分析、Virtuoso/OCEAN/Spectre 适配与官方 MCP、受限优化、项目文件、环境卡片及双语持久化底座。
- 针对 CentOS 6.9 x86_64 构建 Go 1.23.12 静态程序，轻量网页，无运行时依赖安装；生产/Demo 配置、数据、入口和安装包分离。
- 升级保留用户目录，schema 迁移前备份，写入失败回滚；任务幂等/取消/超时、参数预算和候选复验有回归检查。
- 本地 race/接口/浏览器与公开 RC 参考数据验证见 COMPLETION_REPORT.md；真实 CentOS、内网 GLM、商业 EDA/PDK 和公司服务器负载尚未验收。

## 2026-09-21 · 需求补充与确认（未发布）

- 用户确认 IDEA-01～08 全部纳入：新增变更影响提醒、参数重要性/敏感度、分级仿真、评审证据/例外及远端会话能力，细化多目标面板、实验分支/重跑和 AI 回归评测。
- 将八项映射到稳定需求编号，补充验收、跨模块场景及升级数据保留范围；依赖不明、未验证、例外接受与实际通过分别记录。
- 新增中文/英文界面切换、偏好持久化，以及 Linux UI/后台低资源占用要求；补充启动、空闲、大数据与共享服务器负载验收，量化资源预算待目标环境确定。
- 按用户要求进入首版范围讨论，提出“模拟仿真实验助手”范围草案（待确认）；正式版与 Demo 的开发配置、数据和交付隔离成为明确要求。
- 明确首版增加独立内网 AI 对话页，优先验证 Virtuoso/Spectre，并要求 MCP 控制先核查 GitHub 成熟方案再适配；协议/桥接/仿真结果分别验证。
- 开源组件、公司基础设施接入及开发阶段仍未确定，该记录为实现前的需求确认；后续实现状态见上方开发候选记录。

## 2026-09-20 · 项目建立（未发布）

- 建立 Linux IC 工作台独立目录与需求管理入口。
- 确认模拟 IC 为主要场景，同时面向数字 DE 和版图工程师，记录用户常用软件清单。
- 确认 AI 赋能研发为核心，记录公司 Linux 内网 GLM 5.3 部署事实；模型接口与能力尚未验证。
- 完整梳理 AI、环境、项目标注、执行、数据、debug、迁移、数字/版图、MCP/工作流、知识/文件、UI、部署与自动优化需求，补充跨模块场景与验收依据。
- 新增项目备注/标签、MCP 控制 EDA、模拟设计自动调参的明确需求和资料级可行性判断；区分 MCP 协议、EDA 适配器及实际执行。
- 将升级保留既有数据设为硬性要求：程序与持久数据分离，必要迁移保留原信息，失败可恢复，禁止通过清库或重置完成升级。
- 按用户要求先审阅需求全貌，撤下未确认的首版与阶段排序；旧 MVP 编号在需求稿保留追溯映射，技术路线仍待评估。
- 当前仅有规划文档，未确认技术栈，未开始产品实现或正式发布。

## 0.7.0-rc1 · 2026-09-26 · native Linux desktop

- Replace production browser launch with Python/Tk native widgets; retain a separate web Demo and the same local record/draft schema.
- Add native ten-page flows, waveform canvas, reviewed Code proposals, live task/approval/cancellation, bounded background workers and bilingual light-green navigation.
- Add per-user desktop lock/focus socket, owned-backend shutdown, headless diagnostics and terminal installation entry. Production desktop API serves no web UI.
- Make platform tool inputs native JSON objects (legacy string accepted); expose Linux operations as separate tools with required fields and reject action-inappropriate arguments.
- Tie MCP subprocess initialization to a cancellable lifetime so an unresponsive server cannot indefinitely block connection shutdown.
- Record real small-model successes and failures, including independent RTL compilation/truth-table simulation. Do not equate API/model completion with engineering correctness.
