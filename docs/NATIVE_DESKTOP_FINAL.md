# 原生客户端最终交付与验证服务收尾

2026-09-26。交付 `ic-workbench-0.7.0-rc1-linux-amd64-production.tar.gz`，12,708,517 字节，253 个文件校验项。

SHA256：`bbc980ea8a2903b1b55a3b286dd3c41969b2b51b9763a7b2629ce2b70ec38b61`。

最终归档在真实 CentOS 6.9 普通 UID 505 下再次命令安装，253 项 SHA256 全部通过；二进制、原生客户端、启动器、安装器与初始配置逐字节等同此前 UID 500/505 双窗口实测版本。最终包补充了验收文档和证据。见 [最终实装](validation/desktop-rc1/final-artifact.json) 和 [双用户原生验收](validation/desktop-rc1/installed-native/installed-native.json)。

两个 GUI 测试账户共用的是专用 Xvfb 显示夹具，以便截图；数据/API 隔离用不同真实 UID 验证。公司部署仍应使用各自桌面/VNC 会话，不共享 Xauthority。主机原有其他模型服务未被改动。

QEMU 已正常关机，专用模型、SSH 转发及临时归档代理已停止；测试端口 18822/18823/18824/18865/18880/18883 已关闭。用户网页 Demo 18766 保持运行。虚拟磁盘、模型、工具、日志和下载证据保留，未执行验证环境卸载；等待用户确认后再清理。见 [服务收尾](validation/desktop-rc1/service-shutdown.json)。

小模型任务仍为 8/9 通过，自主创建并核验文件未通过。完整功能、依赖、资源、模型能力边界与截图见 [原生客户端报告](NATIVE_DESKTOP.md)。早期打包脚本变量覆盖导致一次中间归档名称错误，已修复并添加根目录/归档名称一致性检查；该中间产物移到 build 下，不列入正式 release 校验清单。最终以上述哈希为准。
