# Linux 安装、内网配置与升级

当前候选为 **0.8.0-rc3 原生客户端**，目标 CentOS 6.9 x86_64。实际验证记录见 [Linux 验收报告](LINUX_QEMU_VALIDATION.md) 和 [多用户可靠性复验](RELIABILITY_RC3.md)。公司 GLM、真实 EDA、GPU/NFS 和特定服务器条件仍需现场验证；本地小模型用于接口与工作流程验收。

## 安装和启动

仅交付 production 安装包。解压后在包目录执行：

```sh
sha256sum -c SHA256SUMS
./install.sh
~/.local/bin/ic-workbench --open
```

也可直接执行包目录中的 `./start.sh`。默认启动原生桌面窗口，需要 Linux X11 桌面/VNC（或配置正确的 SSH X 转发）、Python 2.6+ / 3 和系统 Tk。CentOS 6.9 由管理员从公司仓库安装 `tkinter`；Ubuntu/Debian 对应 `python3-tk`。不捆绑浏览器，不依赖 Node/Electron/WebKit。原生窗口通过鉴权回环 API 访问静态 Go 后台；该模式的网页路由返回 404。关闭窗口或 Ctrl+C 停止本客户端启动的后台，复用已有 API 服务时不会误杀它。

```sh
# CentOS 6.9：仅缺少 Tk 时由管理员执行
sudo yum install tkinter
# 安装完成后的个人终端入口
~/.local/bin/ic-workbench
```

平台不会自动执行上述系统安装命令；离线公司仓库需由管理员提供。`ICW_PYTHON=/absolute/path/to/python` 可指定带 Tk 的解释器。
安装器新增个人终端命令，不修改 `.cshrc` / `.bashrc`。若 `~/.local/bin` 已在 PATH 中，可直接运行 `ic-workbench`；否则使用上面的完整路径。sh/bash/csh/tcsh 都可执行这个入口。

```sh
~/.local/bin/ic-workbench --status
~/.local/bin/ic-workbench --listen auto
# 明确固定端口；被占用会报错，不会偷偷换端口。
~/.local/bin/ic-workbench --listen 127.0.0.1:18770
```

同一账户、同一数据目录再次启动会聚焦已打开的原生客户端，不产生第二个后台。`--status` 验证私有令牌与实际服务响应。旧个人配置中保存的固定端口被占用时可自动选择空闲端口；命令行明确指定的端口则严格遵守。每个 Linux UID 具有独立的实例记录、桌面锁和本地聚焦 socket。
Demo 为网页预览：在开发工作区运行 `python3 scripts/preview_demo.py`，参见 [Demo 说明](../demo/README.md)。没有独立 Demo 安装包。

| 内容 | 生产默认位置 |
| --- | --- |
| 安装版本 | `~/.local/opt/ic-workbench/releases/<version>-production` |
| 当前入口 | `~/.local/opt/ic-workbench/current-production/start.sh` |
| 终端命令 | `~/.local/bin/ic-workbench`，链接到当前入口 |
| 初始配置 | `~/.config/ic-workbench/production/config.json` |
| 平台记录 | `~/.local/share/ic-workbench/production/state.json` |
| 压缩草稿 | 同目录 `workspace.json.gz` |

可设置 `ICW_INSTALL_ROOT`、`ICW_BIN_DIR`、`ICW_CONFIG_DIR`，启动时用 `--data /absolute/path` 指定数据目录。升级沿用原目录。程序可以安装在所有用户只读的共享目录；每个 Linux 用户在自己的终端启动，各自使用 HOME 下的配置、数据、端口与令牌。

生产数据与配置目录要求归当前 UID 所有、权限 0700；平台文件要求 0600，拒绝软链接/硬链接的数据库或配置。已有不安全目录会明确报错，不擅自改共享目录权限、不清空记录。若旧数据路径本身是软链接，使用其真实私有目标路径；若权限错误，仅在确认是自己的专用目录后修正权限。生产/Demo 的目录锁与模式标记继续拒绝混用。不要让多个账户共用一份平台数据库，也不要以共享 Linux 账号代表不同个人。

`.instance.json` 是同目录下的私有启动记录，包含当前令牌；正常停止时移除。状态查询不信任失效记录。账户隔离针对不同普通 UID，不针对 Linux 管理员/root。公司 NFS、ACL、目录服务与重连条件需现场复验；本轮实际验证使用 CentOS 本地文件系统。

## 原生桌面与轻量运行

正式入口不打开浏览器。公司有图形桌面或 VNC 时，在该会话终端运行命令即可。纯 SSH 没有 DISPLAY 时明确报错；需要先配置 X 转发或 VNC，再启动原生客户端。`--service` 是可选的 API 自动化入口，保持回环鉴权且不提供网页。

窗口使用 Tk 系统控件，三个有界后台工作线程处理请求，GUI 线程独立更新进度。普通页面每 15 秒检查资源，监测页按个人设置刷新，窗口最小化暂停周期监测。窄窗口提供横向/纵向滚动，避免控件不可达。详见 [原生客户端验收](NATIVE_DESKTOP.md)。历史浏览器验收仅证明旧版本能力，不能代表本版原生 GUI。

预览默认 256 KiB，文件分析导入最多 2 MiB，Code 导入 1 MiB、AI 修改输入最多 120 KB。资源阈值默认 90%，可改为 1–100%；刷新间隔 3–60 秒。GPU 依赖 Linux NVIDIA 驱动与 `nvidia-smi`，不可用时显示不可用，不伪造 0%。关闭进程只对同用户且启动身份未变化的 PID 发送 SIGTERM。

## 模型与环境配置

在“设置与验证”中填写完整 Chat Completions URL、模型标识、密钥环境变量名；例如完整路径 `http://intranet-host:port/v1/chat/completions`。地址不会自动补路径。支持 HTTP/HTTPS；不跟随重定向，不连接外部模型兜底。HTTPS 使用系统受信 CA。

初始配置也可在 `config.json` 中设置 `modelUrl`、`model`、`keyEnv`、`modelTools`、`modelTimeoutSeconds`。界面保存过的个人 Preferences 优先于初始配置；后续调整请在设置页完成。真实密钥通过启动进程的环境变量提供，不写入工程、工作草稿或安装包。

Agent 需要模型支持 `tools` / `tool_calls`。普通问答、Code 生成和原页文件分析不要求工具调用。原有平台 Agent 修改受平台数据修改开关和具体操作确认控制。新增 Linux Agent 使用独立的本轮关闭/确认/自主开关；命令使用账号原有权限，工作目录不是沙箱，详见 [Linux Agent](LINUX_AGENT.md)；EDA 页仅使用选中 MCP 连接允许的工具。模型答复不替代执行回执，排队不代表外部任务完成。非流式模型回答仍支持取消与阶段进度。

Linux 环境脚本使用 `/bin/sh`、`/bin/bash`、`/bin/csh` 或 `/bin/tcsh`，在启动及保存设置后自动加载、验证。各软件版本可有自己的脚本/PATH，共用启动语句；不会改写用户全局 `.cshrc`。环境文件需适合非交互执行。

插件批量导入示例：

```sh
export ICW_TOKEN=your-current-local-token
python ~/.local/opt/ic-workbench/current-production/install_plugins.py \
  --url http://127.0.0.1:实际服务端口 plugins.json
```

清单只登记脚本路径和参数，导入不会执行脚本。脚本实际以当前 Linux 用户运行，有超时、取消和日志。插件页面运行默认使用设置中的工作目录，Agent/API 可明确传入项目。

## 升级与数据保留

先结束活动任务，再用 Ctrl+C 或 SIGTERM 停止后台。安装新包后重新运行 `~/.local/bin/ic-workbench`。安装器先在临时目录复制并校验完整版本，再发布目录和切换链接，旧版本保留；同版本目录存在时拒绝覆盖，也不会覆盖不属于本平台的同名终端命令。初始配置通过锁和原子写入创建，仅首次缺失时初始化。

主 Schema 为 **6**，草稿 Schema 为 **1**。项目只保存路径、标签、备注和配置，不复制工程；公司负责工程备份。手动备份入口已取消，旧 `backup-*` 文件保留。旧格式迁移前只生成一次 metadata 恢复文件；不含工程副本。保存采用临时文件、fsync、原子 rename，多页面草稿冲突返回 409 并允许导出未保存草稿。

升级不会删除对话、Code 原文/候选/历史、文件分析记录、计算器或配置。不要把旧快照覆盖到升级后的数据目录。回退前先核对 Schema 支持，在独立恢复目录验证。损坏或未知新格式会拒绝启动并保留原文件；异常退出时活动任务标记中断，不自动重跑。磁盘写满时保存报错且保留原数据；若重命名已完成但目录同步失败，会明确报错并暂停后续写入，检查存储后重启，避免内存与磁盘分歧。持有独占锁并通过数据验证后，才清理本程序异常退出遗留的私有 `.atomic-*` 临时文件。

详见 [本地数据与 Code](LOCAL_DATA_AND_CODE.md)、[EDA 与 Code 闭环](EDA_TASKS_AND_CODE_CHECKS.md)。

## 验证和源码构建

包内 `./acceptance-linux.sh /path/to/report` 是基础冒烟脚本，使用独立临时 Demo 数据，只证明启动、API 和解析样例；不能代替真实模型、权限、浏览器和公司接入验收。完整 QEMU 方法、结果和边界见 [验收报告](LINUX_QEMU_VALIDATION.md)。

源码固定官方 Go 1.23.12，`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1`。执行 `GO=/path/to/go1.23.12/bin/go scripts/build.sh` 生成一个生产候选包。打包需要开发机 Python 3；目标运行不需要 Go，但原生客户端需要系统 Python/Tk。更换工具链需重新验证旧内核兼容性。当前不自动联网升级。

本轮新增功能、Schema 7 升级和验证边界见 [0.8.0-rc3](ENGINEERING_RC1.md)。
