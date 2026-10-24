# MCP 接入与执行边界

0.3.0 新增独立 EDA 控制页面，工作台可作为 MCP 客户端连接外部服务，见 [EDA 控制接入](EDA_MCP.md)。下文是原有“工作台作为 MCP server”的接口，二者共用官方 SDK。

协议层使用官方 `github.com/modelcontextprotocol/go-sdk v1.0.0`，Go 1.23 可构建。未自行实现 JSON-RPC/MCP 协议；EDA 动作复用同一受控任务服务。调研与选择理由见 REFERENCES.md。

0.5.0-rc3 先在终端启动自己的工作台，然后让 MCP 客户端使用下列命令，自动读取同一用户的私有启动记录，连接正确的动态端口：

```sh
~/.local/bin/ic-workbench --mcp
```

若自定义过配置/数据目录，在 MCP 入口沿用 `ICW_CONFIG_DIR` 和 `--data`。连接明确地址时使用 `--mcp --url http://127.0.0.1:实际端口`，并提供其当前 `ICW_TOKEN` 环境变量。自动发现只读取当前 UID 的私有记录，不搜索其他用户。

使用官方 stdio 传输，stdout 仅协议数据。默认只提供 `workspace_state`（元数据）、`job_status`（job ID）、`read_batch`（batch ID）。只有显式增加 `--allow-run` 才提供 `probe_virtuoso`、`submit_simulation`、`cancel_job`。这是给 MCP 客户端的执行授权；客户端还应按自身方式在提交前展示项目/参数。普通聊天不因 MCP 接入而获得执行权限。

提交输入：project、params 数字映射、可选 key 幂等标识；同 project/key 的不同参数会拒绝，同一提交不会重复启动。`job_status`/`cancel_job` 用 job；`read_batch` 用 batch。结果为结构化 JSON，运行后通过 job.batch 读取测量证据。当前仅本机/SSH 转发的单用户工作台，不是组织级远程 MCP 服务。

应用内 GLM 使用兼容 Chat Completions 的工具调用桥接到同一服务，并非要求模型服务本身实现 MCP。默认 modelTools=false；实际接口能力确认后启用。不能把 MCP 握手通过等同于 Virtuoso/Spectre 已联通。

本地测试通过官方 SDK client/server 真实 initialize、tools/list、tools/call、结构化输出验证与只读工具门控；商业 EDA 和用户实际 MCP 客户端仍需现场验收。
