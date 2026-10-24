# 0.7.0-rc1 原生 Linux 客户端

2026-09-26。正式版为 Python/Tk 系统控件应用，网页只用于 Demo。窗口无 HTML、WebView 或浏览器；原生后台关闭静态网页路由，保留鉴权本机 API。源代码位于 `desktop/`，业务和 Schema 6 / 草稿 Schema 1 保持兼容。

## 运行和升级

在 X11/VNC 桌面的终端中：

```sh
tar -xzf ic-workbench-0.7.0-rc1-linux-amd64-production.tar.gz
cd ic-workbench-0.7.0-rc1-linux-amd64-production
sha256sum -c SHA256SUMS
./install.sh
~/.local/bin/ic-workbench
```

需要 Python 2.6+ / 3 与系统 Tk；CentOS 6.9 的依赖名是 `tkinter`，由管理员通过公司仓库安装。无需联网下载运行时、Node、WebKit、浏览器。没有 DISPLAY 的纯 SSH 终端不能显示原生窗口，需要桌面/VNC或 X 转发。

`--open` 同样打开原生窗口，`--status` 查询当前实例。再次执行默认命令聚焦原窗口。退出关闭自己启动的后台；如果启动前已有同版本独立 API，关闭窗口保留该服务。`--service` 只启动本机 API，不提供网页。安装器只切换版本链接，保留旧版本、个人配置、记录和压缩草稿；项目仅登记路径，不复制工程。

## 十页功能

AI 对话选择项目/文件夹、历史、Linux 本轮关闭/逐项确认/自主执行、进度与取消；项目文件的路径/标签/注释、目录浏览、跨项目搜索；软件共用命令与版本环境；插件表单/JSON/脚本导入与执行；实际 Linux 监测与同用户进程终止；原页文件分析和 CSV/VCD 原生 Canvas 波形；16 种 Code 生成/导入/AI 修改、候选审阅/采用/撤销/导出、检查/修复/格式化；变量计算器；MCP 连接/工具/AI 任务与进度；模型及环境配置自动加载验证。

原生界面与网页 Demo 共享业务与数据模型。界面像素不完全相同：原生版采用系统控件和轻量绘图，保持浅绿色侧栏/会话/工作区布局。窄窗口可滚动访问控件。

## 验证范围和解释

使用保留的 QEMU CentOS 6.9 x86_64，内核 2.6.32、glibc 2.12、Python 2.6.6、Tk 8.5.7，普通 UID 505，在 Xvfb 的真实 X11 窗口中测试。测试通过实际 Tk 按钮、输入、表格和文件导入操作生产类；截图从 X11 窗口采集。Xvfb 只代替显示设备，不代替界面实现。不同 UID 与实装升级另有独立证据。

AI 推理由宿主本地 llama.cpp b11146 提供，通过 SSH 回环隧道供真实 CentOS 后台调用；不是在 CentOS 内执行模型推理，也不是调用公司 GLM。模型与程序分开记录。测试日志/小电路均为明确构造的确定性测试数据，不冒充真实 EDA 仿真数据；AND 真值表、欧姆定律与 Python 数值测试有独立预期值。

小模型官方文件、固定 revision、SHA256 和下载来源保留于验证环境 manifest。Qwen3-0.6B 未达标；追加 Qwen3-1.7B Q8_0。失败报告保留，不以接口返回 200 或模型自称完成代替任务检查。Linux 工具拆分为有明确必填字段的操作，兼容旧调用格式；检查环境不会静默吞掉命令参数。

## 最终验收结果

| 验证 | 结果 | 证据 |
| --- | --- | --- |
| 宿主后端 race | 72 项通过：全量 71 项 + 指定官方 Verible 后补验 1 项；无最终跳过 | [全量日志](validation/desktop-rc1/go-race-final-v2.txt)、[Verible 实测](validation/desktop-rc1/verible-final.txt) |
| CentOS 后端 | 70 项通过，2 项因缺少 clang/Verible 跳过 | [Linux 日志](validation/desktop-rc1/linux-tests-final-v2.txt) |
| 原生功能与生命周期 | 11 组通过；十页/项目搜索备注/软件/插件/波形/Code/计算器/双语/持久化/重复启动/退出 | [原生结果](validation/desktop-rc1/native-ui-final/native-ui.json) |
| 原生真实 AI 交互 | 7 组通过；问答、原页分析、代码修改与实际检查、Linux 审批、取消、MCP fixture 进度、窄窗口 | [AI GUI 结果](validation/desktop-rc1/native-ai-final/native-ai.json) |
| 双 UID 实装升级 | UID 500/505 两个真实窗口、不同数据/端口/令牌；交叉访问拒绝；重复命令聚焦；复用独立后台不会误杀 | [安装结果](validation/desktop-rc1/installed-native/installed-native.json) |
| 原有数据 | 两用户的 state、workspace.gz、config 安装前后逐字节 SHA 一致；老账户 4 项目/12 对话保留 | 同上 |
| 网页 Demo | 0.7.0-rc1 十页可打开，无脚本错误；更新前后原数据 SHA 一致 | [页面](validation/desktop-rc1/web-demo.json)、[保留数据](validation/desktop-rc1/demo-upgrade.json) |

界面截图：[原生 Code 与真实编译结果](validation/desktop-rc1/native-ai-final/native-ai-code.png)、[原生 AI 命令回执](validation/desktop-rc1/native-ai-final/native-ai-linux.png)、[原生 MCP 进度](validation/desktop-rc1/native-ai-final/native-ai-mcp.png)、[安装后窗口](validation/desktop-rc1/installed-native/installed-native.png)。

两个账户的客户端与后台合计 RSS 分别为 32,304 / 32,444 KiB（约 31.5 / 31.7 MiB）；5 秒空闲采样各约单核 1.6%。这是 QEMU 测试环境的短时测量，不是生产负载保证。Xvfb 另约 23 MiB，模型服务位于宿主且不计入客户端；内网推理资源不能视为零。生产包不包含模型或虚拟机。

## 小模型任务能力：保留未通过项

最终 Qwen3-1.7B Q8_0、llama.cpp b11146、思考上限 1024 token、temperature 0 的独立结果矩阵为 **8/9 通过**。通过项：配置自动加载、欧姆定律数值问答、合成 EDA 错误日志分析、Verilog 生成+实际 Verible/Icarus 编译及 4 组 AND 真值表仿真、导入 Python 修改+独立数值断言、真实 CentOS 编译失败→AI 修复→复检、计算器工具结果、Linux 实际命令与 UID。

**未通过：自主创建并核验新文件。** 模型错误地把目标文件路径先创建为目录，随后写入被冲突检查拒绝，未形成目标文件；因此该小模型不满足可靠的多步骤自主 Linux 操作要求。模型最终文本/任务会话结束并不代表用户目标通过，必须看工具回执和独立产物检查。此项不能包装成全部 AI 通过，也不能推断公司 GLM 的能力。

[最终矩阵与原始响应](validation/desktop-rc1/small-model-17-thinking.json)；[0.6B 失败基线](validation/desktop-rc1/small-model-object-tools.json)；[1.7B 关闭思考 7/9](validation/desktop-rc1/small-model-17-final.json)。一次名为 dedicated-tools 的中间报告实际仍使用旧部署（部署脚本使用了 Python 2.6 不支持的 check_output）；之后通过二进制 SHA 对齐才运行 final/thinking 矩阵，不把中间名称当作修复生效证据。早期 fixture 缺失、BooleanVar 类型、插件 ID、MCP fixture stdin 读法等错误均保留初始日志，最终通过记录对应修正后的程序/测试。

MCP 原生 AI 测试使用显式的无 EDA echo fixture，包含 SDK 协议、允许列表、真实通知与回执；没有调用仿真软件。真实测试还暴露静默 MCP 初始化的超时问题，已让 subprocess 随连接 lifetime 取消，并增加专门截止时间回归。公司 GLM 5.3、Virtuoso/Spectre/Calibre 等真实商业 EDA、GPU/NFS/公司桌面条件、复杂电路设计和长期负载不在本地通过结论内。专有 PSF/FSDB 仍需厂商工具导出，MCP fixture 通过仅代表协议与进度路径通过。
