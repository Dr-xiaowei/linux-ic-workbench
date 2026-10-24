# 终端与 GUI 最终交付验收

2026-09-26。**0.6.0-rc2 的命令安装、命令打开 GUI 和本轮验证收尾已完成。** 操作与完整检查范围见 [终端 GUI 指南](TERMINAL_GUI.md)。

- 生产包 12,263,982 字节；210 个文件逐一校验。SHA256：`3bd56c6a8d4b83cbe4e42077883e5c80303f9e9eb585309f15f024c5b9729a57`。
- CentOS 6.9 普通用户 UID 505 通过 `./install.sh` 全新安装，个人终端链接执行 `--open`；实际运行路径为该用户安装目录中的最终二进制。真实 Firefox 52.8.0 GUI 自动登录，十页/导入/计算/双语/重载均已验证。
- 原用户 UID 500 从 rc1 升级到 rc2，4 个项目、12 个会话及原配置、state.json、workspace.json.gz SHA256 保持不变。GUI 测试用户的数据也保留，私有目录 0700、文件 0600；旧发布目录保留。
- 宿主 race 68 项通过；CentOS 66 项通过、2 项因缺少 clang/Verible 跳过。原生 GUI 7 组通过，现代 Chrome 的 Linux SSH 及网页 Demo 各十页通过，无页面脚本异常。没有把缺少公司 GLM/EDA/GPU/NFS 条件的项目标为通过。
- 四种 shell、重复 `--open`、显式浏览器、无桌面/缺失浏览器/失败打开器、Ctrl+C 通过。重复打开复用当前 PID/端口/令牌。
- QEMU、Firefox、Xvfb、SSH 验证转发与依赖代理已停止；18822/18823/18824/18865/18868/18880 均无监听。网页 Demo 18766 继续运行，更新前后记录和草稿哈希相同。
- QEMU 虚拟磁盘、安装的验证浏览器、下载缓存与之前模型文件均保留，未卸载；等待用户明确确认再清理。浏览器不打入生产包。

[最终机器可读验收摘要](../release/ic-workbench-0.6.0-rc2-validation.json) · [最终包实装证据](validation/terminal-gui-rc2/final-installation.json) · [GUI 实测截图](validation/terminal-gui-rc2/installed-linux-gui.png) · [服务停止记录](validation/terminal-gui-rc2/services-finished.json)

证据包内包含打包前的程序/GUI 测试。最终压缩包实装和停止服务的后续证据保留在工作区与发布摘要中，归档包实测后未再修改。此前小模型多步自主规划失败仍保留于 [Agent 报告](LINUX_AGENT_REPORT.md)，本轮 GUI 通过不改变该结论。
