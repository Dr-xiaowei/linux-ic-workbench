# GitHub 成熟项目调研与功能候选

调研日期：2026-09-20（Asia/Shanghai）；GitHub API 快照时间为 2026-09-20 13:52:37 UTC，补充采样时间为 13:56:32 UTC。以下提交、发布和回应时间统一使用 UTC。

**结论：六个项目具备持续维护、持续发布和成体系文档，值得作为各自领域的成熟参考；本轮没有找到同时满足“成熟、持续维护、开箱即用、覆盖全部商业 EDA 与模拟 AI 研发需求”的完整替代平台。** SiliconCompiler 最接近 EDA 流程，Optuna 最贴近自动调参，MLflow/AiiDA 值得学习数据与任务组织，Lmod 直接对应环境切换，Open OnDemand 提供远程工作入口经验。

这是公开资料调研，不是安装验收。未安装或运行候选，未连接公司模型、商业 EDA、PDK 或授权服务。**决策更新（2026-09-21）：用户已确认将 IDEA-01～08 全部纳入正式需求，映射与验收以 [PROJECT.md](../PROJECT.md) 1.1 及对应功能章节为准。** 该确认不等于选定开源组件或制定阶段计划。以下维护数据仍为 2026-09-20 的历史快照，Harness 仍是已有 UI 风格参考。

## 1. 筛选方法与维护证据

检索围绕 analog IC automation、Virtuoso MCP、EDA workflow、simulation provenance、experiment tracking、parameter optimization、HPC portal、environment modules。先看相关性，再核查 GitHub 默认分支提交、非预发布 Release、文档、测试/持续集成和维护者活动。Star 只作社区规模参考；最近一次机器人更新或图片上传不能独立证明活跃开发。

[公开 API 摘录](research/github-metadata-2026-09-20.json)保存仓库元数据、近期提交、发布、Issue/PR 样本、维护者回应样本与部分许可原文。查询不是原子快照，统计可能短时间变化；Issue 数由 API 的 Issue+PR 总数减单独 PR 数得到。未取得的指标明确留空，不凭缓存补成实时数据。

### 1.1 六个主要参考项目

下表版本是本轮返回的近期发布中选出的正式产品版本，不把 `model-catalog/latest` 等附件发布或 prerelease 视为稳定产品版本。链接分别指向仓库、具体 Release 和近期有意义的提交；不声称列出的提交就是最后一次代码修改。

| 项目与用途 | 正式版本 / 发布时间（UTC） | 近期有意义提交（UTC） | Star / Fork | 开放 Issue / PR | 主项目许可 |
| --- | --- | --- | --- | --- | --- |
| [Optuna：参数优化](https://github.com/optuna/optuna) | [v5.0.0](https://github.com/optuna/optuna/releases/tag/v5.0.0) / 2026-09-07 05:15:45 | [f194136](https://github.com/optuna/optuna/commit/f19413647e0e50d106f96e1b341a2a6d4e21274b) / 2026-09-18 02:15:54 | 14,817 / 1,391 | 9 / 12 | MIT |
| [MLflow：实验与 AI 评测](https://github.com/mlflow/mlflow) | [v3.16.1](https://github.com/mlflow/mlflow/releases/tag/v3.16.1) / 2026-09-17 02:40:08 | [1d34d99](https://github.com/mlflow/mlflow/commit/1d34d997fc4876886128c113c9bfd61f172cf042) / 2026-09-19 15:18:15 | 28,053 / 6,329 | 1484 / 636 | Apache-2.0 |
| [AiiDA：计算追溯与工作流](https://github.com/aiidateam/aiida-core) | [v2.9.2](https://github.com/aiidateam/aiida-core/releases/tag/v2.9.2) / 2026-09-03 08:10:13 | [33f53d4](https://github.com/aiidateam/aiida-core/commit/33f53d42cd868685ca0c15af3c347219033a63c3) / 2026-09-18 13:19:26 | 590 / 269 | 598 / 115 | MIT |
| [SiliconCompiler：EDA 流程](https://github.com/siliconcompiler/siliconcompiler) | [v0.38.8](https://github.com/siliconcompiler/siliconcompiler/releases/tag/v0.38.8) / 2026-09-14 12:10:44 | [ac6bcbf](https://github.com/siliconcompiler/siliconcompiler/commit/ac6bcbfbc887599728bf16d83c6883f6ca14c02b) / 2026-09-16 18:46:41 | 1,214 / 138 | 32 / 2 | Apache-2.0 |
| [Open OnDemand：远程科研门户](https://github.com/OSC/ondemand) | [v4.2.4](https://github.com/OSC/ondemand/releases/tag/v4.2.4) / 2026-08-25 21:57:44 | [79cecc1](https://github.com/OSC/ondemand/commit/79cecc106b2303f2a8fa8ea6dada66ccd4d19afe) / 2026-09-18 19:32:58 | 513 / 195 | 合计 412；拆分未核实 | MIT |
| [Lmod：软件环境切换](https://github.com/TACC/Lmod) | [9.4.2](https://github.com/TACC/Lmod/releases/tag/9.4.2) / 2026-09-09 17:39:13 | [4cd077c](https://github.com/TACC/Lmod/commit/4cd077c6e82dbe352b334b9270e5039a9dcef903) / 2026-09-09 17:27:11 | 610 / 147 | 21 / 4 | MIT |

六个仓库的 API 均返回 `archived=false`、`disabled=false`。成熟依据是发布历史、维护活动和文档体系的组合，**不代表现版本无缺陷，也不代表在公司的 Linux/EDA 组合上验证过**。SiliconCompiler 仍为 0.x，接口迁移成本需单独关注；Open OnDemand 是机构级门户，不能按个人桌面软件估计部署成本。

### 1.2 维护者回应与成熟度补充

| 项目 | 抽样回应（UTC） | 成熟度判断及限制 |
| --- | --- | --- |
| Optuna | COLLABORATOR 于 [2026-09-11 01:25:45](https://github.com/optuna/optuna/pull/6832#issuecomment-5627986045)回应代码评审；相关修复随后合并 | 多代版本、完整 API/教程与测试；算法框架成熟，商业 EDA 接口不随框架提供 |
| MLflow | 本轮 PR #25258 评论样本未取得带 MEMBER/COLLABORATOR 标记的回应；响应时效未核实 | 持续产品发布、实验跟踪与 AI 评测文档；近期有安全修复，采用版本必须另行核对修复覆盖 |
| AiiDA | MEMBER 于 [2026-09-20 11:33:19](https://github.com/aiidateam/aiida-core/issues/7028#issuecomment-5749533010)参与讨论 | 2.x 科学计算框架，追溯、持久工作流、插件文档完整；该样本为开发讨论，不能推导支持 SLA |
| SiliconCompiler | PR #5410 评论样本未取得对应身份回应；近期仍有非依赖类修复与合并 | 连续版本、EDA 教程和测试；本轮代码修复涉及重跑状态，较贴近实际运行问题 |
| Open OnDemand | PR #5804 评论样本未取得对应身份回应；支持时效未核实 | OSC 维护，有完整运维与集群集成文档；仓库自述部署于 2,100 多家组织，仅作项目自报证据 |
| Lmod | MEMBER 于 [2026-09-10 19:40:53](https://github.com/TACC/Lmod/issues/857#issuecomment-5624426654)跟进并关闭问题，前序回应说明修复版本 | 9.x 软件环境管理工具，问题—修复—发布链条可追溯；公司是否已有部署尚未知 |

未在小样本中找到回应不表示没有维护者；本轮没有统计全仓库平均响应时间。原始样本与查询错误保留在 JSON 中。

### 1.3 相关但未进入成熟候选的项目

| 项目 | 核查结果 | 本轮用途 |
| --- | --- | --- |
| [AutoCkt](https://github.com/ksettaluri6/AutoCkt) | 136 Star / 45 Fork；默认分支最后提交为 2022-03-17 的 README 更新，前序代码提交为 2020 年；无 Release，许可未核实 | 模拟电路学习式优化的研究参考，不作为持续维护的平台底座 |
| [Virtuoso-Flow-Plugin](https://github.com/VlHash/Virtuoso-Flow-Plugin) | 6 Star / 0 Fork；最近发布 `ETV-Alpha-0.1.0` 为 2026-06-17 09:35:03 UTC，标记 prerelease；最近提交 2026-06-23 为图片调整；仓库声明 Apache-2.0 | 与 Virtuoso/MCP 直接相关，但 Alpha 和有限工程证据不足以满足成熟条件；可学习桥接思路，不承诺直接使用 |
| [ucb-art/bag](https://github.com/ucb-art/bag)、[bluecheetah/bag](https://github.com/bluecheetah/bag)、[BAG_framework](https://github.com/ucb-art/BAG_framework) | 分别查到默认分支最近提交为 2024-12-27、2020-08-22、2022-12-04；另核查 BAG3 文档仓库最近提交为 2023-10-05 | 有模拟设计领域参考价值，但此次检查的具体仓库不满足近期持续维护筛选；不能据此断言整个 BAG 生态停止维护 |

## 2. 六个项目的十项适配检查

检查项 1「维护状态」、2「社区与弃用」见上表；下列各表覆盖其余八项。所有安装、升级、卸载、硬件架构和运行兼容性均为文档评估，未实测；不能以开发机 macOS 验证代替公司 Linux 验证。

### 2.1 Optuna

| 检查项 | 结论 |
| --- | --- |
| 3. 普通用户部署 | Python 库，适合嵌入优化进程；个人使用比服务平台轻。并行数据库和可视化属于额外配置；卸载库不应触碰实验存储 |
| 4. 环境与服务 | 需要兼容 Python 与数值依赖；不要求云模型或 GPU，仿真器/授权/队列另配。具体 wheel 与公司发行版兼容性未核实 |
| 5. 直接能力 | Ask-and-Tell 可把候选参数生成与外部评估分开；多目标搜索、参数重要性分析可复用。对应 OPT-02/05/06/10 |
| 6. 可改造 | 封装优化会话、约束、暂停和持久化，将 EDA 实测指标交回算法；可先作为单一组件评估 |
| 7. 新开发 | Virtuoso/Spectre/ALPS 适配、合法器件参数、测量表达式、PVT/MC 复验、设计写回保护；框架不会自动理解这些含义 |
| 8. 安全与外联 | objective 回调可执行任意业务代码，应由平台提供受控适配；运行可使用内网资源。外部 OptunaHub 扩展和依赖下载不自动启用，未完成全代码外联审计 |
| 9. 许可 | 仓库声明 MIT；复用需保留版权与许可，算法扩展及依赖独立核对 |
| 10. 综合 | 自动调参的优先组件候选；适配收益明确，但不保证全局最优，也不替代电路知识与仿真验证 |

依据：[Ask-and-Tell](https://optuna.readthedocs.io/en/stable/tutorial/20_recipes/009_ask_and_tell.html)、[多目标教程](https://optuna.readthedocs.io/en/stable/tutorial/20_recipes/002_multi_objective.html)、[参数重要性 API](https://optuna.readthedocs.io/en/stable/reference/importance.html)。

### 2.2 MLflow

| 检查项 | 结论 |
| --- | --- |
| 3. 普通用户部署 | Python 客户端与跟踪服务；个人试验和多人服务的部署复杂度不同。团队使用需要持久存储、认证和备份，升级须单独检查数据库迁移 |
| 4. 环境与服务 | Python 运行时，服务端数据库与产物存储按规模配置；可自托管，无需为基础跟踪购买云账号。模型评价所需模型调用另计，GLM 接口仍待核实 |
| 5. 直接能力 | 参数、指标、产物、实验对比，以及 AI traces、评测数据集；对应 SIM/OPT 和 AI 质量基线 |
| 6. 可改造 | 可接入实验跟踪，也可仅借鉴 Run/Artifact 与评测数据模型；不必把所有 EDA 波形复制进它的存储 |
| 7. 新开发 | PVT/MC、单位、工艺/模型版本、测量定义、波形读取与共享盘引用等 EDA 语义；现有 UI 不等于用户所需工作台 |
| 8. 安全与外联 | 官方说明 3.2.0 起默认使用遥测，目的为使用统计，端点 `mlflow-telemetry.io`；内网部署需设置 `MLFLOW_DISABLE_TELEMETRY=true` 或 `DO_NOT_TRACK=true` 并检查客户端/服务端进程继承。AI traces 可能含设计上下文，存储权限和模型端点需限制。9 月 19 日默认分支有 artifact host 安全修复，不能把 9 月 17 日 Release 自动视为已包含修复 |
| 9. 许可 | 仓库声明 Apache-2.0；复用保留许可/适用 NOTICE、注明改动，依赖另核 |
| 10. 综合 | 实验记录与 AI 质量管理的重要参考；是否完整部署取决于团队规模和服务维护能力，不建议现在就作为强制依赖 |

依据：[Tracking](https://mlflow.org/docs/latest/ml/tracking/)、[AI 评测数据集](https://mlflow.org/docs/latest/genai/datasets/)、[自托管架构](https://mlflow.org/docs/latest/self-hosting/architecture/overview/)、[遥测与关闭方式](https://mlflow.org/docs/latest/community/usage-tracking/)、[artifact host 修复提交](https://github.com/mlflow/mlflow/commit/53e212391904411c9360f87b263b84ffbfe55346)。

### 2.3 AiiDA

| 检查项 | 结论 |
| --- | --- |
| 3. 普通用户部署 | Python 科学工作流框架；生产形态涉及 profile、守护进程、存储和远程计算机配置，部署与运维成本高于单库嵌入 |
| 4. 环境与服务 | 数据库、消息/任务服务依所选 profile 配置，常见部署涉及 PostgreSQL/RabbitMQ；本轮未锁定生产服务版本。远端任务另需 SSH/调度凭据及计算资源，不依赖商业 AI 账号 |
| 5. 直接能力 | 输入—计算—输出追溯图、任务状态持久化、工作流、缓存；对应 RUN-04、AUT-04、PRJ-03 和结果来源 |
| 6. 可改造 | 插件与远程作业机制可接新计算器；也可只借鉴追溯图和缓存键设计 |
| 7. 新开发 | 商业 EDA 作业/解析插件、既有工程接入、结果读取、UI 与 GLM 工具层；不能将现有材料计算插件当作 EDA 驱动 |
| 8. 安全与外联 | 可提交远程代码和访问 SSH/作业资源；凭据隔离在执行层。资料归档/发布属于显式动作，不自动上传公司数据；默认外联和全部插件行为未完整审计 |
| 9. 许可 | 已读取 v2.9.2 LICENSE.txt，MIT；API 自动识别 NOASSERTION 不代表无许可证。插件独立核对 |
| 10. 综合 | 对“结果可信、长任务可恢复”很有价值；完整引入成本偏高，优先借鉴数据关系，是否采用运行引擎留待验证 |

依据：[追溯图](https://aiida.readthedocs.io/projects/aiida-core/en/stable/topics/provenance/index.html)、[缓存规则](https://aiida.readthedocs.io/projects/aiida-core/en/stable/topics/provenance/caching.html)、[持久化进程](https://aiida.readthedocs.io/projects/aiida-core/en/stable/topics/processes/usage.html)、[v2.9.2 许可](https://github.com/aiidateam/aiida-core/blob/v2.9.2/LICENSE.txt)。

### 2.4 SiliconCompiler

| 检查项 | 结论 |
| --- | --- |
| 3. 普通用户部署 | Python 工程流程工具，文档要求 Python 3.10+；还需准备 EDA 后端、工具配置与工程清单，不能仅安装 Python 包即完成商业流程 |
| 4. 环境与服务 | 本地/远程执行依配置，商业工具安装与授权独立；容器并非解决所有商业 EDA 兼容问题。硬件取决于具体设计 |
| 5. 直接能力 | 工程 manifest、步骤依赖、工具适配和带证据的 checklist；数字 DE/实现流程相关性较强 |
| 6. 可改造 | 自定义流程和适配器，可学习统一输入/指标/产物/状态契约；流程定义可以逐项接入 |
| 7. 新开发 | 模拟 ADE/OCEAN、工艺迁移、个人项目管理、AI 解释与公司工具版本适配；现有商业驱动覆盖需逐项核查，不推定全软件清单可用 |
| 8. 安全与外联 | 流程可运行脚本及选择远程服务；内网采用须明确本地/公司队列目标，示例远程端点和资源下载不得直接沿用。静态资料不构成安全审计 |
| 9. 许可 | 仓库声明 Apache-2.0，第三方工具、PDK 与示例设计分别受各自许可约束 |
| 10. 综合 | 最贴近 EDA 的流程参考，尤其适合数字能力与评审证据设计；0.x API 变动成本需评估，不作为成熟模拟控制桥的证据 |

依据：[安装](https://docs.siliconcompiler.com/en/latest/user_guide/installation.html)、[Checklist](https://docs.siliconcompiler.com/en/latest/user_guide/checklist.html)。文档明确提醒：自动检查项若没有绑定任务，可出现“未检查却返回通过”。本平台若借鉴，缺失任务/证据必须显示“未验证”，不能显示达标。

### 2.5 Open OnDemand

| 检查项 | 结论 |
| --- | --- |
| 3. 普通用户部署 | 用户通过浏览器进入；后台部署涉及机构认证、Web 服务和集群配置，通常需管理员，不是个人用户装包即用 |
| 4. 环境与服务 | 要求支持的 Linux 与服务组件；交互桌面还需远程图形服务。当前支持矩阵不能直接套用到旧版公司 Linux；是否已有门户/VNC/调度系统需先确认 |
| 5. 直接能力 | 文件、终端、作业、交互应用入口；对应项目到远端 EDA 会话的统一访问 |
| 6. 可改造 | 通过应用模板和调度适配接入 EDA 启动；可只借鉴会话卡片、资源显示和重连交互 |
| 7. 新开发 | GLM、仿真指标语义、自动优化、MCP 与专用 EDA 数据面板；不替代仿真/验证业务层 |
| 8. 安全与外联 | 涉及登录会话、文件与终端执行，权限必须保持每用户隔离；远端桌面入口需遵守现有网络认证。未审计完整外联，不能把浏览器可用等同于数据可外发 |
| 9. 许可 | 仓库声明 MIT，基础设施和第三方远程桌面组件许可独立 |
| 10. 综合 | 机构门户领域成熟，个人平台完整复用过重；公司已有对应基础设施时，集成现有入口更有价值 |

依据：[官方仓库](https://github.com/OSC/ondemand)、[系统要求](https://osc.github.io/ood-documentation/latest/requirements.html)。

### 2.6 Lmod

| 检查项 | 结论 |
| --- | --- |
| 3. 普通用户部署 | 已有站点安装时用户可直接加载 module；新建全公司模块体系涉及管理员。平台可兼容现有脚本，不强制迁移 |
| 4. 环境与服务 | Lua 与 shell 集成，支持包括 csh/tcsh 的 shell；无需模型账号或持续云连接。工具软件与许可证仍由现有环境提供 |
| 5. 直接能力 | module 加载/切换/卸载、环境集合保存恢复、family 互斥与层次关系；直接对应 ENV-03/04/06 |
| 6. 可改造 | 平台在独立启动进程加载指定模块集合，记录工具环境；读取既有模块描述辅助诊断 |
| 7. 新开发 | 项目绑定、图形界面、PDK/版本兼容矩阵与差异解释；加载成功不能证明 EDA 实际可用 |
| 8. 安全与外联 | modulefile 是可执行脚本，可改环境甚至执行其他行为；只运行已认可的模块，不把脚本全文和授权敏感值自动交给模型。框架不要求云服务，站点脚本外联需逐项核查 |
| 9. 许可 | 已读取 9.4.2 License 原文，MIT，支持相应许可条件下的商用/修改/分发；原参考记录中的“许可未核实”现已补证 |
| 10. 综合 | 优先评估的环境组件；已有 Lmod 则接入，没有则与当前公司脚本并列比较，不擅自改全局 .cshrc |

依据：[用户指南](https://lmod.readthedocs.io/en/latest/010_user.html)、[9.4.2 许可](https://github.com/TACC/Lmod/blob/9.4.2/License)。

## 3. 可以新增或细化的八个点

**以下是结合项目能力推导的产品建议，不是这些项目开箱即用的 EDA 功能。** 成本为相对复杂度，不是工期。表内“建议纳入/细化/保留”为 2026-09-20 调研时的评审意见；2026-09-21 用户已确认八项全部纳入，包括 IDEA-08，现行需求与验收统一维护在 PROJECT.md。

| 调研项 | 实际使用场景与价值 | 与调研时需求的关系 | 成本 / 原始建议 |
| --- | --- | --- | --- |
| IDEA-01 变更影响与结果有效性提醒 | 换 PDK 模型或测量表达式后，显示哪些指标、报告和优化候选需要重验；列出受影响链条，避免旧结果用于新设计 | 新增行为，建立在 PRJ-03、ENV-06、结果追溯之上 | 中高；建议纳入 |
| IDEA-02 参数重要性与局部敏感度分析 | 根据已有扫描找出最影响带宽/功耗的参数，再组织小幅扰动实验核查，让 AI 有依据地建议先调哪里 | 新增分析能力，补充 OPT-06/10 与 AI debug | 中；建议纳入 |
| IDEA-03 多目标候选对比面板 | 并列展示低功耗、低噪声、高带宽的可行候选，以及每个候选的角落覆盖和规格余量，支持选定设计 | 细化 OPT-02/08，已有多指标取舍不另立重复需求 | 中；建议细化 |
| IDEA-04 分级仿真与提前淘汰 | 先跑工作点/基本功能，再做代表性工况，最后对入围候选执行完整 PVT/MC/后仿；减少明显无效设计占用资源 | 新增执行策略，补充 OPT-07/08 与 AUT-04 | 中高；建议纳入，但筛选规则须可解释 |
| IDEA-05 实验分支与选择性重跑 | 从历史实验生成新分支，只改补偿电容并保留对照；一批仿真中仅重跑失败点，明确哪些成功点是复用结果 | 细化 RUN-05、AUT-04、SIM-05 和既有缓存有效性约束 | 中；建议细化 |
| IDEA-06 评审证据包与例外记录 | 每条规格绑定版本、工况、结果和报告；未满足项记录原因、负责人、批准人与有效范围，交接时一并导出 | 新增正式评审对象，扩展 PRJ-02/04、OUT-01；不是 AI 自动签核 | 中；建议纳入 |
| IDEA-07 AI 回归样本库与版本对比 | 每次 GLM 接入方式、提示词或 MCP 工具升级，用固定案例比较错误定位、取数准确率、错误操作率和实际耗时 | 细化现有 AI 质量基线、AUT-05、升级兼容验收 | 中；建议细化 |
| IDEA-08 远端 EDA 会话卡片 | 项目内看到服务器、软件版本、图形会话、作业和资源到期状态；复用公司已有入口恢复工作上下文 | 新增交互范围；与 RUN-04 长任务恢复关联但不等同 | 接已有入口为中，自建桌面服务为高；先保留候选 |

### IDEA-01：变更影响与结果有效性提醒

借鉴 AiiDA 的追溯图与缓存输入匹配。建议记录“设计/模型/测试定义 → 运行 → 指标 → 报告”的依赖；变更后显示“对当前基线需要重验”，保留旧基线结果的历史有效性。无法确认依赖时标记未知，不擅自判断无影响，也不自动重跑全部任务。

验收例：更换模型版本后，关联结果出现重验提示，旧模型结果仍可查；只改项目备注不使仿真结果失效。重验清单能力已随本项纳入，实际执行仍沿用授权与预算。来源：[AiiDA provenance](https://aiida.readthedocs.io/projects/aiida-core/en/stable/topics/provenance/index.html)、[缓存](https://aiida.readthedocs.io/projects/aiida-core/en/stable/topics/provenance/caching.html)。

### IDEA-02：参数重要性与局部敏感度分析

借鉴 Optuna 的已完成试验重要性分析，再增加本平台的局部扰动测试。比如解释“在当前采样范围内，补偿电容与带宽变化关联最大”，而不是宣称已证明物理因果。重要性估计与真正的灵敏度导数分别展示；注明参数范围、工况、样本数及失败样本比例，样本不足时不强行排名。

验收例：相同数据和分析版本得到可追溯结果；给定参数 ±扰动的仿真可用于核查建议，并计入仿真预算。来源：[Optuna importance](https://optuna.readthedocs.io/en/stable/reference/importance.html)。

### IDEA-03：多目标候选对比面板

借鉴 Optuna 多目标试验和 Pareto 前沿：显示无法在所有目标上同时被另一候选超过的取舍集合。优先列出满足硬约束的候选；未跑角落标“未验证”，失败或不收敛不被优化分数掩盖。前沿只针对当前已评估候选，不宣称全局最优。

验收例：用户选择偏低功耗方案后，能看到与偏低噪声方案的具体差异、适用工况和尚未覆盖测试。来源：[多目标优化](https://optuna.readthedocs.io/en/stable/tutorial/20_recipes/002_multi_objective.html)。

### IDEA-04：分级仿真与提前淘汰

借鉴优化中的早停思路和可编排工作流，但 EDA 分级准则需我们新建。区分硬性淘汰、暂缓昂贵测试和完整验证；DC、AC、瞬态以及 MC 并非可任意替换的精度层级。不能拿瞬态尚未稳定的一小段直接判设计失败，也不能把优化训练中的 pruning 原样套到所有多目标 EDA 任务。

验收例：工作点明确违背设定约束时跳过后续昂贵步骤并记录原因；未完成完整复验的候选始终不能显示“规格已达标”。来源：[Ask-and-Tell 与提前终止示例](https://optuna.readthedocs.io/en/stable/tutorial/20_recipes/009_ask_and_tell.html)、[AiiDA 工作流](https://aiida.readthedocs.io/projects/aiida-core/en/stable/topics/processes/usage.html)。

### IDEA-05：实验分支与选择性重跑

借鉴 MLflow 的实验记录、AiiDA 的缓存/恢复和 SiliconCompiler 的步骤组织。实验分支是元数据与参数分支，不意味着复制整个项目或必须强制 Git 管理大型波形。只有输入、工具/模型、随机种子或样本标识、测量定义一致时才复用对应结果；历史原件缺失或完整性不可验证时不能假装可重现。

验收例：失败点重跑生成新运行记录，关联原失败证据；界面能区分本轮新算与沿用结果，重连不重复提交。来源：[MLflow Tracking](https://mlflow.org/docs/latest/ml/tracking/)、[AiiDA 缓存](https://aiida.readthedocs.io/projects/aiida-core/en/stable/topics/provenance/caching.html)。

### IDEA-06：评审证据包与例外记录

借鉴 SiliconCompiler Checklist。将通过、未通过、未验证、例外接受区分开；例外不能改写原始指标，并应随规格/设计基线变化重新核查。负责人与批准人字段在个人模式也可简化，不因此默认引入全公司审批系统。

验收例：缺失结果/报告不显示通过；打包后可以定位版本、工况、原始证据和例外适用范围。AI 可整理证据和起草说明，工程师决定是否接受。来源：[Checklist 审计模型](https://docs.siliconcompiler.com/en/latest/user_guide/checklist.html)。

### IDEA-07：AI 回归样本库与版本对比

借鉴 MLflow 评测数据集，把正确导出、日志诊断、失败恢复、错误工具参数、越界路径等案例保存在公司内网。冻结输入与预期事实，记录模型端点/版本、提示词、工具 schema 和解析器版本；不能完全依靠同一个模型自评正确性。统计重复运行的波动，禁止测试触及真实设计写回目标。

验收例：升级前后对相同样本比较，某类日志定位回退能被发现；更换模型服务不丢失旧基线。来源：[MLflow AI 评测数据集](https://mlflow.org/docs/latest/genai/datasets/)。

### IDEA-08：远端 EDA 会话卡片

借鉴 Open OnDemand 的交互入口。优先链接现有 SSH/VNC/门户与队列服务，显示会话和作业分别的生命周期：浏览器重连、图形会话恢复、仿真作业恢复是三件事。并不意味着 MCP 可以控制所有 GUI 控件。

验收例：关闭工作台再进入后能定位原会话和作业，不启动重复 Virtuoso 或额外占用授权；实际重连依公司基础设施支持。来源：[Open OnDemand](https://github.com/OSC/ondemand)、[运行环境要求](https://osc.github.io/ood-documentation/latest/requirements.html)。

## 4. 怎样复用，而不是把六套平台全部装进来

| 路线 | 实际含义 | 优点 | 主要缺口与判断 |
| --- | --- | --- | --- |
| A 直接使用 | 分别使用现有平台/组件，例如既有 Lmod、MLflow 跟踪页或公司 Open OnDemand | 单项成熟能力可直接获得，减少重复开发 | 各工具分散，没有统一模拟研发上下文；找不到一个直接满足全量需求的产品 |
| B 基于成熟组件适配 | 平台提供统一工作流与 UI，优化/环境层按条件复用 Optuna/Lmod；其他项目按必要性接入 | 数值算法、环境管理等不必重写，能保留公司现有 EDA 流程 | 仍需 EDA 适配、统一数据关系、内网模型与安全执行；本轮推荐继续评估，但未选定技术栈 |
| C 独立开发业务平台 | 借鉴上述交互和数据模型，自己实现业务存储/运行管理/界面；仍可使用通用库 | 业务契合度与部署形态可控 | 长任务、缓存、恢复、迁移都要承担工程成本；不应从零写优化算法或 EDA 引擎 |

判断：**可以先借鉴所有六个项目的经验，再只选确实需要的组件。** Optuna、Lmod 是较明确的组件候选；MLflow/AiiDA/SiliconCompiler 的数据与流程经验可先进入设计讨论；完整 Open OnDemand 仅在机构基础设施确有需求时评估。它们与 MCP 不冲突：MCP 描述给 AI 的动作接口，实际动作由受控脚本/官方 API/作业系统完成。

可验证的最小切片建议（仅供后续选型，不是首版范围或阶段计划）：对一个已获授权的测试平台，固定环境与基线，执行一组有预算的参数试验，记录结果并比较候选，再验证断线恢复与一次数据保留升级。该切片只用于证明所选组合可行，不能替代完整需求评审。

## 5. 用户决策与后续状态（2026-09-21）

- 用户确认“纳入”，对应 IDEA-01～08 全部八项；已合并正式需求，不再等待逐项重复确认。
- IDEA-01/02/04/06 分别新增 PRJ-10、OPT-11、OPT-12、OUT-02；IDEA-08 新增 RUN-07，其实现依赖公司既有远端环境。
- IDEA-03/05 分别细化 OPT-02/08 与 RUN-05/AUT-04/SIM-05；IDEA-07 为已有质量要求增加可追溯编号 AIC-08，并关联工作流及升级验收。
- 正式业务范围与验收只在 PROJECT.md 维护；本报告保留原始调研依据和相对成本判断。功能确认不构成组件选型、产品实现或阶段计划。
- 未安装运行候选，公开项目的维护/许可/版本证据未在本次文档合并中重新联网核查。

后续选型需要的关键事实：公司发行版与 Python/Lua 条件、现有调度/远程桌面、GLM 的实际工具调用接口、首个仿真器及版本、工艺数据可读范围。它们不妨碍先判断功能价值，本轮未以这些未知项阻止调研。
