# CentOS 6.9 / QEMU 验证记录

日期：2026-09-26。目标：Linux x86_64，CentOS 6.9；当前修复候选 **0.5.0-rc2 / Schema 6**。

本轮在真实 CentOS 6.9 客体内执行生产二进制、实际文件/进程操作和模型 HTTP 请求。浏览器在宿主机运行，通过 SSH 访问 Linux 后台。公司 EDA、PDK、许可证与 GLM 按用户要求不作为本轮前置条件；没有把 Demo 或模拟 EDA 执行计为真实 EDA 通过。

## 环境和边界

| 项目 | 实际配置 |
| --- | --- |
| 虚拟机 | QEMU 11.1.1，TCG x86_64，2 vCPU，1024 MiB RAM，无桌面 |
| 客体系统 | CentOS 6.9 Final，2.6.32-696.el6.x86_64，glibc 2.12 |
| 系统脚本 | Python 2.6.6；官方签名 tcsh 6.17-38.el6 |
| 磁盘 | qcow2，虚拟容量 4 GiB，实际约 1 GiB，按需增长 |
| 平台 | CGO=0 / GOAMD64=v1 / Go 1.23.12 静态 Linux 程序 |
| 浏览器 | Chrome 153.0.8010.53，1440px 与 760px 页面 |
| 模型 | 宿主 llama.cpp b11146；Qwen3 0.6B Q4_0（409 MiB）做流程验证；Qwen2.5-Coder 1.5B Q4_K_M（1.04 GiB）补测电路代码，单模型运行 |
| 网络 | 平台和模型仅监听回环地址，经专用 SSH 正向/反向端口转发 |
| 验证数据 | 独立 `icw` 用户与 `/home/icw/validation`，仅自建公开结构测试样例 |

选最小 ISO，避免网络安装大量软件；不安装 Linux 桌面、开发工具链或全局 Homebrew。模型在宿主原生运行，避免跨架构模拟推理。原有网页 Demo、用户工程及个人配置未重置。

CentOS ISO 对照官方发布说明 SHA256；模型按固定提交和 LFS SHA256 校验；QEMU 瓶包匹配 Homebrew 官方 SHA256，部分通过清华镜像下载。QEMU 动态库仅重定位到本项目，并进行本机签名，没有修改系统目录。[固定下载来源](validation/linux-qemu/sources.json)、[客体平台记录](validation/linux-qemu/platform.txt)可追溯。

## 功能覆盖

| 功能 | 实际验证 | 结论与证据 |
| --- | --- | --- |
| 项目与文件 | 手动添加/编辑，多根目录，标签/备注/说明，文件注释，浏览、关键词搜索，权限拒绝、符号链接/路径边界、二进制预览拒绝 | 通过；[API](validation/linux-qemu/api.json)、[页面](validation/linux-qemu/browser.json) |
| AI 对话 / Agent | 普通问答，计算器工具，项目/文件夹读取范围，调用进度，修改开关、批准/拒绝、取消，重命名和回执 | 通过；真实 0.6B 模型；模型文字与实际工具回执分别记录 |
| 软件管理 | 图形添加，统一启动语句，两个 sh 版本与 csh 版本，环境变量确实改变，全局配置文件不变 | 通过；实际无害 shell 命令，未调用商业 EDA |
| 插件 | 单独表单、JSON 批量、系统 Python 2.6 批量脚本，默认禁用、启用、实际运行、超时与取消 | 通过；导入仅登记，不隐式执行 |
| 资源监测 | 读取真实 `/proc` 与文件系统，CPU/内存/存储，任务过滤，提醒阈值，关闭当前用户测试进程，拒绝错误启动身份和其他用户 PID | 通过；[API](validation/linux-qemu/api.json)、[实际负载排行](validation/linux-qemu/process-load.json)、[资源采样](validation/linux-qemu/resources-server.json) |
| GPU | 无驱动时明确不可用；独立 CLI 样例验证 95% 使用率/显存、阈值告警与无效值拒绝 | 逻辑通过；[GPU 合同](validation/linux-qemu/gpu-contract.json)；无真实 NVIDIA 硬件，未验收硬件数据准确性 |
| 文件分析 | Virtuoso/DC/Calibre/VCS 类文本、网表，CSV、VCD、未知数字值、缩放/平移；模型结果原页显示；分析重点和答案刷新保留 | 通过；[重点转发与持久化](validation/linux-qemu/analysis-focus.json)；专有二进制拒绝并提示导出 |
| Code | 导入保留原文，真实 AI 修改候选，采用/撤销，真实 Python 2.6 检查→AI 修复→复检，诊断跳行，选中行/全文件格式化，16 类扩展名导出、独占另存 | 通过；专业 EDA 编译/仿真排除。格式化执行接口使用确定性空白整理脚本验证，不宣称已测所有厂商格式化器 |
| 计算器 | 常规运算、工程后缀、科学函数、公式依赖变量、角度制、错误输入和循环依赖 | 通过 |
| 设置 | 保存后自动加载，实际环境继承，模型/脚本错误状态，恢复后重验证，启动自动加载 | 通过；[重启与错误恢复](validation/linux-qemu/restart.json) |
| 本地存储 | 压缩草稿、0600 权限、多标签页冲突、恢复导出、备份入口关闭、只保存工程路径 | 通过；非 EDA 文件无隐式副本 |
| 升级/重启 | 真正安装 0.4→0.5；Schema 5→6；同版本拒绝覆盖；项目、注释、语言、会话/草稿、原文件和配置保留；rc2 重启校验 | 通过；[升级记录](validation/linux-qemu/upgrade.json)、[重启记录](validation/linux-qemu/restart.json) |
| 双语/轻量 UI | 中文/英文，十个页面，760px 无横向溢出，非预期脚本错误为零，普通页与监测页刷新频率 | 通过；[页面](validation/linux-qemu/browser.json)、[网络采样](validation/linux-qemu/resources-browser.json) |
| 真实 EDA / 公司 MCP | Virtuoso/Spectre、VCS、Calibre 等实际调用、许可证、PDK、仿真结果正确性 | 用户明确排除，本轮未执行 |

29 项非 EDA 顶层 Go 测试在 CentOS 内通过，另有 4 项 rc2 针对性 Linux 回归（含 1 项新增 Python 2.6 诊断测试）通过，不能把两批重复项相加为独立测试数。[原始 Linux 测试日志](validation/linux-qemu/go-tests.txt)、[修复回归](validation/linux-qemu/fix-tests-linux.txt)。生产 API 12 组、浏览器 12 组以及分析重点、错误恢复、升级和 GPU 逻辑补验均有独立记录。

## 本轮发现与修正

1. **Python 2.6 错误行号缺失。** 旧 `py_compile` 输出元组形式的 SyntaxError，原解析器只能识别较新格式。rc2 增加文件/行/列解析及回归，真实浏览器点击定位和 AI 修复复检已通过。
2. **插件安装说明路径不匹配。** 包中的脚本位于安装根目录；页面原示例写为源码目录下 `scripts/`。已修正命令，并说明默认运行工作目录。
3. **部署文档过时。** 清除独立 Demo 包、ES5 浏览器、Schema 5 等旧描述，明确当前入口、现代浏览器和实际验证边界。
4. **小模型质量限制。** 0.6B 通用模型的 Verilog 输出出现不完整；失败样例见 [模型质量记录](validation/linux-qemu/model-quality.json)。页面测试还遇到一次候选未出现，缺少当时原始模型响应，不能仅凭该现象断定是模型 JSON 问题。后续测试增加导入完成等待，并使用固定采样复核。不能把 HTTP 成功当作代码正确。固定采样改善重复性，但不构成复杂电路设计能力验收。补充代码模型后，Verilog、SystemVerilog、VHDL 的生成和 AI 修改流程通过；但首次 SystemVerilog 使用了异步复位，与同步复位要求不符，已记录并通过明确修改要求纠正敏感列表。未经 EDA 编译/仿真，不能标为电路正确。见 [生成与修正证据](validation/linux-qemu/circuit-code-generation.json)。

验证脚本自身的 NUL 字节构造、Python 2.6 采样 API、页面异步等待也已校正并复测；这些不作为产品缺陷计数。所有未通过尝试留在隔离环境日志中，最终证据明确区分产品修复、测试修正和模型质量。

## 资源与实际限制

40 秒轻量页面/监测采样中，后台 RSS 约 12.6–13.9 MiB，5 秒 CPU 窗口约 0–1.8%；首次长会话验证后的此前单次驻留约 16.5 MiB。浏览器 JS 堆约 2.1 MiB，属于宿主浏览器而非 Linux 服务器占用。普通页面每 15 秒轻量检查资源告警；监测页按默认 5 秒刷新；离开后停止其高频定时器。

这些是两核 TCG 虚拟机上的小样例结果，**不能保证任意公司共享服务器、海量目录或真实 EDA 负载下的性能**。不含 QEMU/模型进程的内存；两者仅为验证工具，不随候选包部署。

- 公司 GLM 5.3 未连接；小模型证明接口和操作流程可用，不证明与 GLM 的工具调用细节或复杂设计质量一致。
- QEMU 无 NVIDIA GPU，实际硬件驱动/数据仍需现场测试。
- ext4 本地文件系统已测；公司 NFS、ACL、网络断连与物理断电条件未具备，不能声称验收。
- CentOS 历史桌面浏览器未安装；使用现代电脑浏览器加 SSH 的推荐方式已测。
- EDA 检查 PASS/芯片签核、PSF/FSDB 私有格式解码不在本轮结论中。

## 复用与保留

验证环境全部放在项目的 `reference-sandbox/linux-validation/`。下载镜像/模型、QEMU/llama 运行时、qcow2 磁盘、独立 SSH 密钥、测试载荷和原始日志均在此；持久验收报告位于 `docs/validation/linux-qemu/`。未创建系统级服务或修改主机全局 shell 文件。

环境内保留 `vm_control.py`、`vm/ssh_config`、固定来源清单及安装配置；可重新启动虚拟机和模型继续复测。仓库中的 `tests/linux_qemu_api.py`、`tests/linux_qemu_browser.cjs` 等记录了请求、断言和操作路径。它们仅针对隔离测试用户，不可直接向真实个人数据运行。

**用户明确要求验证完成后经确认再卸载。当前保留环境文件；确认后只移除该验证目录，保留候选包、源码、截图、测试与报告。** 最终空间和停止状态另记录在清理清单中。

最终保留环境实际占用 **4.075 GiB**。虚拟机、模型、SSH 转发和安装 HTTP 服务均已停止，18822/18823/18824/18865/18880/18881 无监听。原网页 Demo 18766 返回 HTTP 200，未被停止。[清理范围与保留清单](validation/linux-qemu/cleanup-manifest.json)明确限定后续操作范围。

最终安装包额外完成一次 CentOS 真安装与数据字节复核，见工作区 [最终包安装回执](validation/linux-qemu/final-package.json)。该回执产生于打包之后，工作区保存；已校验安装包本身不因回执生成而重新打包。
