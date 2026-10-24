# 0.6.0-rc2：终端安装与打开 GUI

2026-09-26。目标 CentOS 6.9 x86_64。本版新增 `--open`，启动服务后通过 Linux 默认浏览器打开界面；再次执行复用同一账户、同一数据目录的后台。界面仍为轻量网页，不安装 Electron、桌面运行时或浏览器。

## 安装与打开

在自己的 Linux 账号下执行，无需 root：

```sh
tar -xzf ic-workbench-0.6.0-rc2-linux-amd64-production.tar.gz
cd ic-workbench-0.6.0-rc2-linux-amd64-production
./install.sh
~/.local/bin/ic-workbench --open
```

安装器自动校验全部文件，添加个人终端入口，初次运行创建个人配置。安装新版本仅切换程序链接，沿用已有记录和草稿。每个 Linux 用户使用自己的 HOME，不能共用一个 Linux 账号来隔离个人数据。

保持启动终端打开，Ctrl+C 停止后台。关闭网页不会停止后台。`~/.local/bin` 已加入 PATH 时可简写 `ic-workbench --open`；不修改 `.cshrc` / `.bashrc`。

```sh
~/.local/bin/ic-workbench --status
~/.local/bin/ic-workbench --status --open
~/.local/bin/ic-workbench --open --browser /usr/bin/firefox
```

`--browser` 或环境变量 `ICW_BROWSER` 只接受一个可执行文件路径，不接受带参数的 shell 字符串。需要特殊参数时可指定自己的可执行包装脚本。默认先使用 `xdg-open`，没有该命令时再寻找常见浏览器。正常 Linux 桌面/VNC 会话会提供 DISPLAY；不要为了绕过提示随意填写 DISPLAY。

首次启动时浏览器缺失、没有图形会话或打开器失败，后台仍保持可访问，终端给出错误和网页/SSH 地址。已有后台的 `--open` 请求失败会返回非零状态。打开器成功启动仅显示“已请求打开”；不能据此推定任意浏览器都成功渲染。

## 只有 SSH 终端时

服务器没有桌面不影响使用：

```sh
# 服务器终端
~/.local/bin/ic-workbench
# 自己电脑上的另一个终端，替换 PORT / USER / SERVER
ssh -N -L 18765:127.0.0.1:PORT USER@SERVER
```

PORT 使用服务器启动输出中的实际端口。电脑浏览器打开 `http://127.0.0.1:18765/`，输入启动令牌。终端会打印可替换账户/主机的具体命令。后台仅监听回环地址；不需要开放公网端口。浏览器渲染在个人电脑，减少服务器开销。

## 实际验收

后端与原生 GUI 均运行在保留的 QEMU CentOS 6.9 虚拟机：内核 `2.6.32-696.el6.x86_64`、glibc 2.12、普通用户 UID 505、2 vCPU / 1 GiB 内存。使用真实 Firefox 52.8.0 图形进程，在最小 Xvfb 显示器上运行；没有使用模拟打开器代替 GUI。另以宿主 Chrome 经真实 SSH 转发测试远程使用。

| 检查 | 结果 |
| --- | --- |
| 宿主完整 Go race | 68 项通过、0 失败 |
| CentOS 完整测试二进制 | 66 项通过、2 项跳过（缺少 clang / Verible）、0 失败 |
| 原生 Firefox | 7 组通过：终端打开和自动登录、十页、公式计算、Code 文件导入、分析文件导入、双语/窄窗口、重载恢复 |
| 终端预验 | sh/bash/csh/tcsh、重复打开、指定 Firefox、缺失/失败打开器、无桌面降级、SIGINT 正常退出通过 |
| 重复打开 | 当前后台 PID、端口与令牌保持不变 |
| 现代 Chrome | Linux SSH 转发及用户网页 Demo，各十页与脚本错误检查通过 |
| Demo 更新 | 原 state.json 与 workspace.json.gz 的 SHA256 均保持不变 |
| 静态检查 | go vet、JavaScript / Shell / Python 语法及 Linux 静态 ELF 检查通过 |

证据在 `docs/validation/terminal-gui-rc2/`。最终交付包实装、旧用户升级与服务收尾结果另见发布目录的 `ic-workbench-0.6.0-rc2-validation.json`，避免把预验包哈希当作最终归档哈希。

本轮修复旧浏览器缺失的 Promise.finally、File.text / File.arrayBuffer、剪贴板接口和 CSS inset / gap 的回退；原生 GUI 截图已经目视检查。实测瞬时 RSS：后台 13,712 KiB、Firefox 185,740 KiB、Xvfb 25,168 KiB；这是一次会话快照，不是多人长期负载承诺。平台不会捆绑这些浏览器测试依赖。

## 验证依赖来源与边界

浏览器与最小 X 依赖取自 [CentOS 6.9 官方归档](https://vault.centos.org/6.9/)，启用 GPG 校验，76 个 RPM 签名验证通过。包版本包括 `firefox-52.8.0-1.el6.centos`、`xorg-x11-server-Xvfb-1.17.4-16.el6.centos.1`、`xdg-utils-1.0.2-17.20091016cvs.el6`。下载 URL、SHA256 与签名输出保留在验证证据和独立环境目录，不进入产品运行依赖。

CentOS 6 和 Firefox 52 已停止维护；该浏览器仅用于隔离环境的兼容验证，不建议作为公司通用上网浏览器。推荐通过 SSH 使用个人电脑的受维护浏览器。当前验证覆盖 Firefox 52.8.0，不能推断所有更早浏览器均可使用。

本轮没有公司服务器桌面/VNC、NFS/ACL、真实 GPU、公司 GLM、EDA/PDK/许可证；不将 GUI 验收当作这些现场能力已经通过。上一轮小模型多步自主规划的失败仍有效，详见 [Linux Agent 报告](LINUX_AGENT_REPORT.md)。专用服务完成后停止，QEMU、虚拟磁盘、浏览器缓存和模型等文件保留，须经用户确认后才清理。
