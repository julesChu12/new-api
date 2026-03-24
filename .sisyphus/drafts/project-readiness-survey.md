# Draft: Project Readiness Survey

## Requirements (confirmed)

- 阅读当前项目，为后续的二次开发做准备
- 深挖方向聚焦：后端网关核心
- 新目标浮现：准备新增一个渠道，用户手头已有外部接口文档
- 用户确认：新增渠道按“独立 Tencent VOD”归类，不与现有腾讯混元复用
- 首版范围收敛：仅图片能力
- 用户当前最关心：腾讯这套新异步流程的开发量级，以及当前项目整体架构草图
- 新偏好：首版希望按较完整、可商用、稳定优先的方式规划，后续倾向全量接入腾讯 VOD 能力
- 新增产出需求：给出当前项目 ER 图草图，帮助建立整体数据模型概念
- 新增产出需求：为每个 ER 实体补一张集中释义表，说明各表含义与角色
- 用户确认：Tencent VOD 配置采用“强类型配置”方案，不走纯散装 JSON
- 用户确认：首版结果资产仅保存腾讯返回的 URL / FileId，不做自有存储同步；后续保留切换到自有存储的演进空间

## Technical Decisions

- 先做只读式项目摸底，不进行任何源码修改
- 输出目标先定为“项目结构/风险/后续切入建议”的研究摘要；是否进一步生成执行计划待确认
- 下一轮重点转向 relay/service/model/middleware 的后端主链路与高风险约束
- 初步渠道策略：将腾讯 VOD AIGC 作为独立 provider/channel type 规划接入
- 产品定位修正：Tencent VOD AIGC 应视为“腾讯托管的异步媒体生成平台/聚合层”，而不是简单的海外模型透明代理
- 配置初判：若按当前项目架构接入，Tencent VOD 凭据更适合存入 `Channel.Key` / `Channel.settings`，而非写死 `.env`
- 配置方向确认：扩展现有 `Channel` 体系，优先在 `dto.ChannelOtherSettings`/后台表单中引入 VOD 强类型字段
- 任务 ID 方向确认：延续现有双 ID 模式，系统生成公开 `TaskID`，并在任务私有数据中映射腾讯上游 `TaskId`
- 资产策略确认：首版不引入自有对象存储/媒资同步链路，任务仅沉淀腾讯结果 URL / FileId；后续如需切换，再通过任务完成后的资产后处理层演进
- 计费策略确认：首版采用“按模型/分辨率分档预扣 + 失败退款”，不做复杂完成后二次结算
- 首版能力范围确认：实施范围锁定为“仅图片”，视频能力放到后续阶段
- 技术默认值确认：`upstream_task_id` 升级为 `Task` 显式可索引字段；异步完成机制采用“回调主触发 + 轮询补偿 + 终态不可回退”

## Research Findings

- `main.go` + `router/main.go`: 后端启动入口，负责资源初始化、路由注册、前端静态资源承载
- `router/` + `controller/` + `service/` + `model/`: 项目采用明确的 Router -> Controller -> Service -> Model 分层
- `relay/channel/`: 40+ 上游模型通道适配层，是二次开发最核心、最敏感的扩展点
- `setting/` + `.env.example`: 配置面广，覆盖模型、运营、系统、OAuth、缓存、数据库等多个维度
- `web/`: React 18 + Vite + Semi Design；前端由 Go embed 承载，也可独立部署
- Go 测试能力存在（testify + 20+ `*_test.go`），但 Go lint、前端测试、完整测试 CI 仍有明显缺口
- 部署形态以 Docker 为主，存在 GitHub Actions、docker-compose、Electron 构建链路
- 后端请求主链路已确认：`router/relay-router.go` -> `middleware/auth.go` / `middleware/distributor.go` -> `controller/relay.go` -> `service/billing.go` / `service/channel_select.go` -> `relay/relay_adaptor.go` -> `relay/channel/{provider}/`
- 后端高风险约束已确认：JSON 必须走 `common/json.go`，数据库必须保持 SQLite/MySQL/PostgreSQL 兼容，请求 DTO 必须保留显式零值，认证/计费链路强耦合不可随意改动
- 已确认外部渠道文档文件存在：`【通用】VOD AIGC服务接入指南.docx`，文档解析任务已启动
- 文档确认该接口是 **腾讯云 VOD AIGC 异步任务型 API**：提交 `CreateAigcVideoTask` / `CreateAigcImageTask` 返回 `TaskId`，再通过 `DescribeTaskDetail` 轮询或回调取结果
- 鉴权不是普通 Bearer Key，而是 **腾讯云 CAM 签名（SecretId/SecretKey）**，这与现有混元类渠道语义明显不同，更支持“独立 Tencent VOD 渠道”方案
- 文档覆盖图片 + 视频两类生成，支持多模型、分辨率、ExtInfo 扩展参数，存在并发限制和内容安全失败码
- 仓内已存在通用异步任务基础设施：`model/task.go`（Task 表，含 `UserId`、`TaskID`、`Platform`、`Status`、`PrivateData`、`Data`）、`controller/task.go`（`/api/task/self` / `/api/task`）、`service/task_polling.go`（15s 轮询、超时清扫、按平台分发）
- 当前 Task 能力已经支持“按用户隔离查询”，但尚未体现 workspace 维度；若目标是商用级工作台隔离，扩表/扩查询条件很可能是核心工作之一
- Oracle 结论：对于商用级 Tencent VOD 首版，真正的核心是“异步任务生命周期 + 隔离 + 可恢复任务管理”，而不仅仅是接通腾讯 API
- 腾讯官方任务细节已确认：状态只有 `WAITING` / `PROCESSING` / `FINISH`，`DescribeTaskDetail` 只保证最近 3 天任务可查；`SessionId` 可做 3 天内去重，`SubAppId` 是重要的任务归属/资源边界
- 新渠道注册面已确认：`constant/channel.go`、`relay/relay_adaptor.go`、`relay/channel/task/{provider}/adaptor.go`、`web/src/constants/channel.constants.js` 为必改入口
- 当前 `Task` 表足以承载“按用户隔离”的异步任务；但若“工作台”指多 workspace/团队空间，而非单用户任务页，则需要新增归属维度与查询索引，不能只依赖 `UserId`
- 当前项目核心 ER 主干已确认：`User` 为中心，向外关联 `Token`、`Task`、`Log`、`TopUp`、`UserSubscription`；`Channel` 通过 `Ability(group+model+channel)` 参与模型路由，并与 `Task`/`Log` 形成执行链路
- 对 Tencent VOD 而言，`Task` 是最合适的落点实体；在“仅用户隔离”前提下，首选方案是复用现有 `Task` 体系并补齐 VOD 专属字段，而不是新建一套任务库
- ER 草图文档已生成：`.sisyphus/drafts/current-project-full-er-tencent-vod.md`
- ER 文档已增强“ER 实体释义总表”，可作为阅读源码前的表级词典
- 渠道配置面核对完成：`model/channel.go` 的 `Key`、`BaseURL`、`Setting`、`settings` 理论上足以承载 Tencent VOD 凭据与附加配置；真正缺的是 **VOD 专属类型注册、前端表单提示/校验、`ChannelOtherSettings` 的强类型字段、后端解析消费逻辑**
- 配置策略已收敛：Tencent VOD 走“按渠道后台配置 + 强类型配置 + 基础项与轮询策略首版开放”，不走纯 `.env` 也不走纯散装 JSON
- Task 字段级策略已收敛：Tencent VOD 首版优先复用现有 `Task` 显式列，VOD 特有字段分层落入 `Properties / PrivateData / Data`，不急于新增 Task DB 列
- 已补充 Tencent VOD 图片任务的“表级读写时序”草案，并识别出一个核心实现风险：若回调按上游 `TaskId` 反查本地任务时跨数据库查询不稳，则上游 `TaskId` 可能需要升级为显式可索引字段
- 当前默认推荐已形成：将腾讯上游 `TaskId` 升级为显式可索引字段；回调作为主触发器，轮询作为补偿机制，二者应汇入同一套幂等状态推进逻辑，禁止轮询覆盖已写入的更新终态

## Open Questions

- 首版异步完成机制是否仅采用后台轮询，还是同时纳入腾讯回调

## Scope Boundaries

- INCLUDE: 项目结构、模块职责、测试与构建、配置与部署面、潜在风险点
- EXCLUDE: 任何业务代码修改、配置变更、执行实现任务

