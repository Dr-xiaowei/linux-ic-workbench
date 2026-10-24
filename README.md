# Linux IC AI Workbench

面向模拟 IC 研发的 AI 仿真实验工作台，同时为数字 DE、版图工程师提供项目、文件、软件环境和内网问答入口。界面采用 Harness 风格的侧栏、会话与证据区，支持中文/English。

**当前为 0.8.0-rc3 原生 Linux 桌面候选。** 正式客户端使用系统 Tk 控件，不使用浏览器、HTML 或 WebView；Demo 继续网页。保留浅绿双语十个工作区、现有 Go 业务与个人数据格式。实际 CentOS 6.9 的原生窗口、命令安装与小模型验收见 [原生客户端报告](docs/NATIVE_DESKTOP.md)。公司 GLM 和商业 EDA 仍需现场验证。
本轮精简客户端安装包至约 3.6 MB；只保留运行文件、当前操作说明和许可证，完整测试证据留在源码目录。参数模板、单位/工况比较、多文件修改与验收见 [0.8.0-rc3 操作及验收](docs/ENGINEERING_RC1.md)。

## 下载与运行

- [生产配置候选包（Linux x86_64）](release/ic-workbench-0.8.0-rc3-linux-amd64-production.tar.gz)
- [网页 Demo 预览](demo/README.md)：运行 `python3 scripts/preview_demo.py`，打开终端打印的网页链接，无需安装 Demo 软件包。
- [当前原生客户端验收结果](docs/NATIVE_DESKTOP.md) · [最终包与服务收尾](docs/NATIVE_DESKTOP_FINAL.md)
- [安装、内网配置、升级与目标机验收](docs/DEPLOYMENT.md)
- [Demo 操作路径](docs/DEMO.md)

解压后运行 `sha256sum -c SHA256SUMS`、`./install.sh`，在 Linux 桌面/VNC 的终端执行 `~/.local/bin/ic-workbench`，直接打开原生窗口；`--open` 为同义入口。需要系统 Python 2.6+ / 3 与 Tk（CentOS 6.9 的 `tkinter` 包）。纯 SSH 无 DISPLAY 时会明确提示，不能显示窗口。无需 Node、Electron 或 WebKit，不修改 `.cshrc`；重复命令聚焦已有客户端，退出关闭本客户端启动的后台。`--status` 查询实例，`--service` 可单独运行仅本机 API。升级复用个人目录，不复制项目工程。

新增使用说明：[本地数据与 Code 修改](docs/LOCAL_DATA_AND_CODE.md)；[GitHub 优化建议](docs/GITHUB_RESEARCH_V4.md)中的 IDEA-09/10 已纳入；IDEA-11 多文件修改已纳入（逐文件审阅/采用/撤销，不自动 Git 提交），IDEA-12 按取消备份的决定不纳入。

## 当前十个工作区

| 入口 | 功能 |
| --- | --- |
| AI 对话 | 独立选择项目/文件夹范围、通用问答、统一平台工具与 Linux 自主终端/文件操作、逐步执行进度、审批、取消、上下文与回执持久化 |
| 项目与文件 | 多路径、项目备注/标签/说明、逐层浏览、每个文件/目录注释、跨项目文件名与文本关键词检索；专注项目文件管理 |
| 软件管理 | 一个软件统一启动语句，各版本独立环境脚本/PATH，可手动增改与启动 |
| 插件 | 单独填写表单、JSON 批量导入、批量脚本、启用/禁用、运行/取消与记录 |
| 资源监测 | Linux CPU/内存/存储、NVIDIA GPU、进程与仿真任务排行、可配置阈值提醒、同用户任务关闭 |
| 文件分析 | 通用 EDA 文本报告/日志/网表、CSV/VCD 波形、缩放平移、在当前页面显示 AI 分析结果，并保留本地分析历史 |
| Code 平台 | UTF-8 代码导入、AI 修改并排审阅、采用/撤销、本地历史； 16 种 RTL/模拟模型/网表/约束/脚本类型，左右对照、编辑导出、另存、分语言检查工具；诊断定位、限次 AI 修复与复检、全文件/选中行格式化 |
| 计算器 | 常规与公式计算、依赖变量、科学函数、工程后缀、RC/热噪声/增益示例 |
| EDA 控制 | 官方 MCP 客户端、内置 Cadence 桥接、外部 stdio/HTTP 连接、工具发现/调用/记录；绑定连接的 AI 任务对话、实时进度及取消 |
| 设置与验证 | 内网模型与工具调用、全局 Linux 环境、阈值/轮询、编译器；启动和保存后自动加载验证 |

新增 EDA 任务与 Code 闭环见 [操作指南](docs/EDA_TASKS_AND_CODE_CHECKS.md)。项目备份功能已取消，项目只登记路径与说明，旧记录和配置升级保留。

详细操作、插件开发和格式限制见 [使用说明](docs/V2_GUIDE.md)。官方 MCP SDK 与 Virtuoso/OCEAN/Spectre 桥接、结果比较和有预算的自动调参继续保留。

网页 Demo 的 Agent 路由、Code 模板和资源数据明确标为演示；不连接真实模型，不执行软件/插件宿主命令，不关闭宿主进程。真实 Linux/GLM/商业 EDA 验收没有被 Demo 替代。专有二进制 PSF/FSDB 等需厂商导出。

## 开发与验证入口

```sh
scripts/go.sh test -mod=vendor -race ./cmd/... ./internal/...
scripts/go.sh vet -mod=vendor ./cmd/... ./internal/...
scripts/build.sh
```

开发机 macOS ARM64 默认使用 `scripts/go.sh`：校验项目保留的官方 Go 压缩包，在系统临时目录解压和编译，命令结束自动清理工具与缓存，不在项目保留展开副本。其他开发主机请在构建/Demo 前设置 `GO=/path/to/go1.23.12/bin/go`；测试直接使用对应 Go 命令。显式设置 GOCACHE/GOPATH 时由开发者管理其空间。客户端不需要 Go。

Go 模块固定版本，vendor 支持离线构建；Linux 2.6.32 目标固定 Go 1.23.12，禁止未经复核换成提高内核门槛的工具链。运行测试需要本机回环网络权限。新版界面测试在 `tests/browser_v5.cjs`、`tests/browser_v5_regression.cjs`、`tests/browser_v5_data.cjs`，需要开发机 Playwright/Chrome，不是目标机运行依赖。

| 文档 | 用途 |
| --- | --- |
| [完整需求](PROJECT.md) / [首版范围](docs/V1_SCOPE.md) | 长期业务清单、当前八组范围与暂缓项 |
| [EDA 适配](docs/EDA_ADAPTER.md) / [MCP](docs/MCP.md) / [EDA 控制接入](docs/EDA_MCP.md) | 公司接入、证据与执行边界 |
| [验证矩阵与限制](COMPLETION_REPORT.md) | 本地证据、未验收原因与正式版本门槛 |
| [参考与选型](REFERENCES.md) / [GitHub 调研](docs/GITHUB_RESEARCH.md) | 固定版本、原始来源、复用判断 |
| [任务](TASKS.md) / [更新](CHANGELOG.md) | 状态与重要变化 |
| [第三方声明](THIRD_PARTY_NOTICES.md) | 依赖版本与许可证 |

[GitHub 仓库](https://github.com/Dr-xiaowei/linux-ic-workbench)按私有可见性保存当前候选源码和安装包，尚未选定公开分发许可证，未标记正式版本。完整本地验证环境、历史安装包、运行数据和公司申报材料不进入仓库。验证日志中的临时令牌及开发机路径已脱敏，原始证据保留在本地；历史校验值对应当时原件。

从 GitHub 克隆后，请自行准备 Go 1.23.12，并用 `GO=/absolute/path/to/go scripts/build.sh` 或 `GO=/absolute/path/to/go python3 scripts/preview_demo.py` 构建；本机保留的压缩工具链不上传。当前安装包校验文件为 `release/CURRENT_SHA256SUMS`。
