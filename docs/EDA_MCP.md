> 0.5 新增：在本页直接用 AI 下达任务、接收实时进度，使用方法和边界见 [EDA 任务指南](EDA_TASKS_AND_CODE_CHECKS.md)。

# EDA 控制与 MCP 接入 · 0.3.0-rc1

2026-09-25：工作台现在既保留向外提供工具的 MCP server，也新增作为客户端连接 EDA MCP 服务的独立页面。协议复用已锁定的官方 Go SDK v1.0.0，不捆绑新 Node/Python 运行时。

## GUI 使用

1. 打开“EDA 控制”。内置 Virtuoso / Spectre 桥接可直接“连接并获取工具”。正式模式需先按 EDA_ADAPTER.md 配置 Cadence/OCEAN 脚本与许可证环境；在“Cadence 工程接入配置”逐工程填写配置文件与环境。项目与文件页面不再出现这些执行配置。
2. 工具列表来自 MCP `tools/list`。查看参数 schema，填写 JSON，点击调用后确认具体工具和参数。
3. 内置工具为 `list_projects`、`probe_virtuoso`、`submit_simulation`、`job_status`、`cancel_job`。项目 ID 可从 `list_projects` 回执或工程接入配置复制。探测/仿真返回 Job ID；继续使用 `job_status` 或下方“仿真任务状态与日志”查看实际运行结果。
4. MCP 调用以 Operation 记录排队、运行、成功/失败/取消；回执含工具名、参数与结果。“MCP succeeded”表示协议工具调用成功，不保证其提交的异步 EDA 作业已成功。不会对失败的写操作自动重试，先检查回执及远端任务。
5. 外部服务通过“添加 MCP”登记。保存配置不连接。stdio 填绝对可执行文件、参数数组与工作目录；HTTP 填完整 Streamable HTTP 地址，认证可引用启动进程的环境变量名。工具发现后，将所需工具名加入允许列表才可实际调用。
6. 连接会复用至断开、配置变化或服务关闭；无后台高频探测。一次调用最长 1–600 秒，工具目录最多 128 项；调用并发受工作台已有并发限制。“停止”取消当前 MCP 请求；远端提交的独立任务可能继续运行，需调用对应取消工具。断开会关闭 MCP 会话/stdio 服务，不保证关闭其已经启动的 EDA。

stdio 示例（示意路径；先安装审阅过的服务，不自动下载）：

```json
{"name":"Company EDA bridge","transport":"stdio","command":{"executable":"/opt/company/eda-mcp","args":["--stdio"]},"directory":"/home/user/work","url":"","tokenEnv":"","allowedTools":["list_designs","run_simulation"],"timeoutSeconds":120}
```

工具名必须以服务实际发现结果为准，这里的示例不声明任一厂商已提供这些名称。stdio 服务以当前用户权限运行，不是权限沙箱。标准输出必须只承载 MCP 协议。公司环境加载脚本由设置配置；配置的模型密钥环境变量不会主动传给 stdio 子进程，但本机可执行程序本身具有用户权限。HTTP 不跟随重定向，密钥仅从环境读取，不写入连接清单。

## GitHub 参考与选型

查阅日期：2026-09-25；没有安装或执行这些 EDA 参考仓库。既有 SDK 已 vendor 并保留 MIT 许可证。参考不能替代公司版本的实测。

| 方案 | 已核对的一手信息 | 本版采用方式 |
| --- | --- | --- |
| [官方 MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/quick_start.md) | 官方客户端的连接、tools/list、tools/call 模式；本项目固定 v1.0.0 / Go 1.23 | A：直接使用现有依赖；不升级工具链最低内核要求 |
| [SkillBridge](https://github.com/unihd-cag/skillbridge) | Python ↔ Cadence SKILL 接口，LGPL-3.0；README 要求 Python 3.8+、IC 6.1.7 或 ICADV/M+ | B：作为公司可选桥接层参考；它本身不是 MCP server，不默认安装 |
| [OpenROAD-MCP](https://github.com/The-OpenROAD-Project/OpenROAD-MCP) | 官方 OpenROAD 项目下的 MCP 实现；会话、命令和指标工具；当前 server.json 标记 0.6.1，npm 入口要求 Node 22+，[许可证 BSD-3-Clause](https://github.com/The-OpenROAD-Project/OpenROAD-MCP/blob/main/LICENSE) | B：参考工具发现与会话/回执设计；可手动接入公司已部署的兼容服务，不能据此声称支持商业 EDA |
| [virtuoso-bridge-lite](https://github.com/Arcadia-1/virtuoso-bridge-lite) | Python/SKILL/Spectre 桥接参考；读取 pyproject 为 0.8.0 / Python 3.9+ | B：仅接口思路参考；不捆绑、不执行、不宣称已通过公司版本验收 |

按项目既有十项口径：需求契合、维护主体、成熟证据、许可证、系统兼容、安装/更新行为、权限与数据流、运行资源、验证与升级、替换成本。结论：协议基础采用 A；厂商桥接按 B 保持可替换；C 仅实现本工作台的连接管理、参数确认与回执 UI。最小完整链路是内置 Cadence MCP → 已有适配器 → Job 记录，同时提供 stdio/HTTP 扩展。仓库存在和文档宣称不等于维护/稳定性认证；本次没有深入审计候选的全部安装脚本、遥测或依赖，不自动部署候选。

## CentOS 6.9 与验证边界

核心继续以 Go 1.23.12 构建静态 Linux x86_64。可选 MCP 服务有各自运行时要求，Node 22/Python 3.8+ 不能按 CentOS 6.9 自带环境直接保证运行。可将桥接部署在受支持的内网主机，通过 Streamable HTTP（或用户自建且已验证的 SSH stdio 包装）连接。GUI 不内置远程部署、旧 SSE 传输、OAuth 登录或自动依赖安装。

Demo 也经过官方 SDK 的真实 MCP initialize/list/call，但连接本地模拟桥接；即使登记外部 URL/命令，也不会真正连接或执行它。正式模式已通过自建 HTTP 与 stdio 协议合同、白名单拒绝、超时/恢复检查。商业 Virtuoso/Spectre/其他工具、PDK/许可证及公司网络认证均未实测。
