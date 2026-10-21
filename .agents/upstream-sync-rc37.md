# new-api 上游同步与部署记录

## 2026-10-22 rc.46 安全升级

用户确认跟进完整 rc.46，随后明确要求生产镜像替换由用户自行择时执行；本轮仅整合、验证、推送和发布镜像，禁止修改生产 `.env`、重启或切换生产容器。合并提交 `5984793047b7dedd346396d284cb9f8a5a7f154f`，上游标签 `v1.0.0-rc.46`。保留现有定制；冲突处理保留构建提交号、独立 count_tokens 的免计费行为、条件错误日志、USDT 配置过滤和本站翻译，同时采纳上游 Claude 保活与 Resend 配置。

Docker 构建固定使用上游 Go **1.26.10** 镜像及摘要，`go.mod` 和发布工作流同步提高最低版本。仅重新构建旧 Dockerfile 无法修复 CVE-2026-111943。验证也显式使用 `GOTOOLCHAIN=go1.26.10`，主模块完整构建、Linux 候选构建及独立 relaykit/tokenkit 构建通过，候选二进制的 Go 构建信息为 1.26.10；线上旧 new-api 二进制实测为 1.26.1，claudeRelay 实测已为 1.26.10。

验证结果：

- `go test ./common/... ./controller ./model ./relay/... ./service ./setting/model_status_setting -count=1 -timeout=180s` 中，除既有代理域名夹具的本机网络差异外全部通过；同一夹具编译为 Linux 测试二进制后在服务器独立容器复核通过。没有为此改业务代码。独立 relaykit 的 `GOWORK=off go test ./... -count=1 -timeout=120s` 全部通过。
- Bun **1.4.0** 按上游锁文件安装。前端 typecheck、生产构建、70 个相关测试文件 / **995** 项测试、涉及文件的 oxlint 与 oxfmt 检查通过；oxlint 只有上游 warning。可灵插件的定向 lint 无 error、格式检查通过。
- SQLite **3.50.4**、MySQL **8.0.46**、PostgreSQL **18.4**：审计排序索引、schema 稳定性与本站拒绝日志筛选矩阵通过。六个完整应用场景覆盖三个引擎的新装和当前 r45 镜像升级；每个候选启动两次，列与索引保持稳定，合成用户余额、管理员价格、旧统计数据和审计记录保留，新增缓存统计可以累计。MySQL/PostgreSQL 分设主库和日志库，日志库历史审计记录保留。测试只使用独立容器、数据库与合成数据。
- 未调用真实上游或真实支付，未实际发送邮件。没有宣称全仓库测试全绿或全站 ASVS 合规。邮件相关沿用上游凭据不回显和失败回归，参考 OWASP Authentication / Session Management Cheat Sheets。

生产主库、日志库、部署配置和应用数据已备份至服务器 `/opt/new-api-backups/20261021T171346Z-before-rc46/`，包含 SHA256 清单，SQL gzip 完整性检查通过。备份含敏感内容，只留服务器，不输出或下载。

镜像发布沿用 **Publish personal Docker image**，输入固定完整提交号；短提交号会被 checkout 当成分支名而失败。发布运行：https://github.com/paopaoandlingyia/new-api/actions/runs/39642327065 。生产保留 r45 / `sha-f0a89b16`；镜像发布成功后交付用户手动切换。

本机验证日志为 `/tmp/newapi-rc46-*.log`；隔离数据库容器及匿名数据卷随验证结束清理。一次性脚本不作为日常发布依赖。

## 历史生产边界（2026-09-30）

用户于 2026-09-30 明确要求：生产容器正在提供服务，暂时不得重启或切换镜像。代码整合、Git 推送、隔离验证不代表生产切换授权。下述部署步骤仅在用户后续明确允许切换时执行。

该日记录的生产镜像为 `ghcr.io/paopaoandlingyia/new-api:sha-0575582b`，对应 `codex/group-user-concurrency-limit` 的 rc.37 代码。后续部署见各日期记录，当前生产版本须以服务器 `.env` 和运行容器为准。删除源码分支不会改变正在运行的镜像。

## 2026-10-19 rc.45 本地整合

用户确认先整合固定 `v1.0.0-rc.45`，再实现客户分组展示隔离；仅限本地代码、验证和提交，未授权本轮生产发布。整合前提交 `59280f332`，上游提交 `6cf0d90b5ff27f901697dddd61e0c28aba8efefc`。本机已添加 `upstream = https://github.com/QuantumNous/new-api.git`。继续使用现有 `main` 和个人镜像工作流，没有推送、构建远程镜像或接触生产服务。

- 保留 Claude 独立 count_tokens、拒绝类别及筛选、管理员上游账号日志、条件错误日志、模型状态页、公开指标隐私、USDT 支付、HTML no-store 和构建元信息。
- 日志列表采用上游 `LogFilter`，把本站 `UpstreamAccount`、`RefusedOnly` 接入共用筛选，使新增导出与列表条件一致；个人导出清除上游账号和管理员元数据，并忽略管理员筛选参数。沿用上游错误脱敏更新。
- 游乐场清理的是浏览器未区分账号的旧本地记录，随后按账号保存。普通用户日志范围默认 365 天。新增令牌额度查询开关：旧令牌升级后没有获得该权限，创建/更新沿用上游规则。
- 前端测试移除一个旧测试对未声明 happy-dom 的依赖，改用现有 Vitest/Testing Library 环境；修正导出夹具的本站字段和性能徽标测试，保留模型广场不请求公开性能指标的本站行为。没有额外增加依赖或 UI 封装。

验证通过：主模块 `GOWORK=off go build ./...`；独立 `relaykit` 的同命令及 `go test ./dto ./relayconvert/...`；控制器、模型、服务层、中间件、common、perf_metrics、relay、Claude 及其他相关渠道、插件测试。前端 `bun run typecheck`、`bun run build`，12 个含本站合并改动的前端文件定向 oxlint，相关 54 个前端测试文件合计 757 项通过（首次 4 文件因 react-icons 下载未完成无法加载，安装后重跑全部通过）。Bun 通过临时 npm exec 使用，安装遵循 `bun install --frozen-lockfile`，没有改写锁文件到上游以外的版本。

真实数据库：SQLite **3.50.4**、MySQL **8.0.46**、PostgreSQL **18.6**，各覆盖新装和本站 rc.43 升级，候选迁移分别执行两次；MySQL/PostgreSQL 分设主库和日志库。合成用户余额、令牌额度、分组、活跃订阅、禁用套餐、六位小数价格、USDT 钱包检查点/订单和拒绝日志均保留；令牌及 USDT 金额预留唯一性保留，两次候选启动后的列和索引一致。SQLite 升级结构仅新增令牌额度查询列。另用真实 ClickHouse **25.3.14.14** 验证拒绝筛选及新增日志导出，包括个人视图隐私。

```sh
# 所有 TEST_/AUDIT_ DSN 均仅指向本机一次性 Docker 测试库
go test ./model -run 'Test(ChannelPriorityDatabaseMatrix|ClaudeRefusalLogFilterDatabaseMatrix|MigrationSchemaStability)/(mysql|sqlite)' -count=1 -v
go test ./model -run 'Test(ChannelPriorityDatabaseMatrix|ClaudeRefusalLogFilterDatabaseMatrix|MigrationSchemaStability)/postgres' -count=1 -v
go test ./controller -run 'Test(LogExportDatabaseMatrix|CryptoPaymentDatabaseMatrix)/mysql|TestTokenMigrationFromChar48ToVarchar128MySQL' -count=1 -v
go test ./controller -run 'Test(LogExportDatabaseMatrix|CryptoPaymentDatabaseMatrix)/postgres|TestTokenMigrationFromChar48ToVarchar128Postgres' -count=1 -v
go test ./model -run 'TestClaudeRefusalLogFilterDatabaseMatrix/clickhouse' -count=1 -v
go test ./controller -run 'TestLogExportDatabaseMatrix/clickhouse' -count=1 -v
```

验证限制：扩展 `go test ./controller ./model ./service ./middleware ./common ./pkg/perf_metrics ./relay/... ./plugins/... ./router/...` 中，路由全覆盖检查缺少本站模型状态路由声明、两个缓存测试仍期待上游缓存策略；三类失败已在整合前 `59280f332` 源码复现，没有修改这些既有问题。上游新增 `TestAdaptorMasksDialedHostsInDialFailure/channel_proxy_host` 在当前网络环境返回 EOF，未出现夹具预期的代理主机名；对应实现/测试与 rc.45 一致，没有为此改业务代码。因此不宣称全仓库测试全绿。未调用真实上游或真实支付，不宣称全站 ASVS 合规；认证整合参考 [OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html) 和 [ASVS](https://owasp.org/projects/asvs)，沿用上游认证、会话和令牌权限回归。

一次性迁移验证源码及结果在 `/tmp/newapi-rc45-validation/`，其他检查日志为 `/tmp/newapi-rc45-*.log`；不作为日常发布工具。

### 客户分组展示隔离

在 rc.45 合并提交 `0a48140ab` 之后单独实施。仅修改生产文件 `controller/pricing.go`：接口的可用分组排除未配置倍率的普通分组；每个模型的 `enable_groups` 只包含当前客户可选的具体分组，无可见分组的模型不返回；原 `all` 标记展开为该客户的具体分组，`auto` 仍是选择器的虚拟选项，不作为模型标签。继续沿用现有全局可用分组、所属分组和特殊可用分组配置，没有改写客户权限配置。

过滤时分配新的分组切片，禁止修改全局价格缓存。模型卡片、表格、详情和接口同步收敛，不需要前端改动。新增一个控制器测试文件覆盖客户 A → 客户 B → 客户 A → 匿名访问的共享缓存场景、仅其他客户可见的模型、未配置分组、`all`、仅 `auto` 和空输入。先用 Go overlay 在修改前复现失败，候选及实际修改后通过；最终 `go test ./controller -count=1`、`GOWORK=off go build ./...` 通过。此阶段没有数据库结构或查询变更。

本次创建的 MySQL/PostgreSQL/ClickHouse 容器及匿名数据卷已清理，Docker 虚拟机的临时代理配置已移除，Colima 恢复为运行前的关闭状态。没有推送或生产发布。

## 固定使用现有发布流程

仓库：[paopaoandlingyia/new-api](https://github.com/paopaoandlingyia/new-api)。上游：[QuantumNous/new-api](https://github.com/QuantumNous/new-api)。

1. 合并代码到 `main`，推送 GitHub。
2. 在 Actions 运行 **Publish personal Docker image**，`ref` 选择 `main`（或指定提交）。沿用 `.github/workflows/publish-personal-image.yml`，不在服务器编译应用。
3. 从工作流结果取得 `ghcr.io/paopaoandlingyia/new-api:sha-<8位提交号>`。
4. 服务器 `/opt/new-api/.env` 只修改 `NEW_API_IMAGE`。部署配置继续使用 `image: ${NEW_API_IMAGE:?...}` 和 `pull_policy: always`，然后执行：

   ```sh
   cd /opt/new-api
   docker compose pull new-api
   docker compose up -d --no-deps --timeout 150 new-api
   ```

5. 检查容器健康及站点 `/api/status` 中的版本、构建提交。涉及数据库迁移时先备份，只为具体风险补充验证。

用户明确要求保留这套简单流程，清理本次临时脚本和隔离资源，不再把一次性运维脚本当作日常部署依赖。当前生产镜像以服务器 `.env` 为准，构建来源以 GitHub Actions 记录为准。

## 2026-09-30 rc.41 整合

用户确认只保留 `main` 作为长期应用分支，包含上游发布版和本站必要定制，不设 `deploy`。本次基线为 `v1.0.0-rc.41` / `2035a82aeb5414253a728bd937d4b8f97aa99b9b`。继续使用现有 GitHub Actions 发布流程。

- 订阅控制器、支付、模型及界面恢复为 rc.41 上游实现，移除本站“余额专用”和“同时生效上限”。保留上游原生订阅功能及已有订阅、余额数据；旧自定义字段保留在数据库中但不再参与业务判断。
- 不整合分组并发分支；本站自定义并发规则不进入新 `main`。
- Claude 单次请求 effort 修复直接采用上游代码，移除重复修复及旧测试。统计归因也采用上游新的拒绝、流式失败和客户端取消处理。
- 保留独立 Claude `count_tokens`、模型状态页及来源管理、管理员上游账号和 Claude 拒绝类别日志、条件错误日志、统计隐私处理、HTML no-store、版本与构建号展示、1Panel 部署配置及个人镜像工作流。既有管理员定价配置不改写。
- 上游新增认证、请求策略、任务插件等功能随发布版整体同步，不另行重构。旧的 Custom OAuth 布尔字段迁移补丁经三数据库验证后移除，恢复上游迁移行为。

用户随后确认移除自定义参数条件 `exists` 及 `**.` 递归路径，后端和参数编辑器恢复为 rc.41 上游实现。缓存规则使用原生通配符统一已有 TTL 为 `1h`，再用 `@dig:cache_control|#` / `lt` / `4` 条件在有空余名额时补充顶层一小时自动缓存；递归计数也会统计非协议位置的同名字段。原来使用 `exists` 的渠道规则须在部署新代码前替换，未自动修改生产配置。后端 `go test ./relay/common`、前端类型检查、定向 lint/格式检查通过；完整规则的无标记、一/三/四个显式标记、顶层加三个显式标记共五类输入验证通过，未调用上游验证真实缓存命中。当前 shell 无 Bun，前端检查使用现有 npm 脚本和本地工具，未改依赖。

前端 typecheck、生产构建、57 个测试文件 / 825 项测试通过；Go 主模块和独立 `relaykit` 构建通过。Linux 控制器 406 项顶层测试、服务层和统计测试通过，其余相关后端包通过。Windows 认证测试遇到 SQLite 文件仍占用的清理失败，Linux 复核通过；渠道亲和性统计测试也在 Windows 失败、Linux 通过，未为此修改上游生产实现。

真实数据库验证通过：SQLite **3.50.4**、MySQL **8.0.46**、PostgreSQL **18.4**。三个引擎分别验证 rc.41 新装、当前生产 rc.37 并发版升级、官方 rc.40 升级，共九个场景；每个场景启动候选版两次，核对用户余额、令牌额度、活跃订阅、禁用套餐、六位小数价格、日志和配置，并验证唯一性及两次启动的列、索引结构一致。MySQL/PostgreSQL 使用独立主库和日志库，SQLite 使用上游默认的同库配置。旧订阅自定义列及原上游账号日志保留，未接触生产库。

验证命令及结果：

```sh
# 主模块及独立协议模块：均通过
GOWORK=off go build ./...
cd relaykit && GOWORK=off go build ./...

# 相关后端包：通过；控制器、服务层和统计包另在 Linux 运行测试二进制通过
go test ./model ./relay ./relay/channel/claude ./relay/common ./relay/helper ./setting/model_status_setting ./middleware

# 前端：通过
cd web && bun run typecheck && bun run build
# 本次前端相关测试共 57 文件 / 825 项通过，定向 oxlint 通过

# 隔离服务器：新装和升级启动、数据与 schema 对比通过
bash db-prep.sh
bash db-test.sh
# 最后一个 PostgreSQL 场景在调整脚本等待条件后续跑通过
bash db-resume.sh
bash db-final-check.sh

# 使用上述真实 MySQL/PostgreSQL 测试库运行，SQLite 也实际运行：通过
TEST_MYSQL_DSN='<isolated mysql DSN>' TEST_POSTGRES_DSN='<isolated postgres DSN>' \
  ./model-rc41.test -test.v -test.timeout=180s \
  -test.run='Test(MigrationSchemaStability|RequestPolicyDatabaseMatrix|MigrateTokenKeyUniqueness|MigratePrefillGroupUniqueness|UserSessionPreviousRefreshHashMigration)'
```

隔离验证使用合成数据和既有镜像中的旧二进制，无生产数据库副本或真实凭据。一处脚本生命周期问题已定位：HTTP 监听先于上游退出信号注册约 100ms，验证脚本改为同时等待 `/api/status` 成功和启动完成日志，不修改应用实现。服务器测试容器、网络及验证目录已清理，本机临时 Git 工作树已归档并删除。这些一次性工具不作为日常部署依赖。ClickHouse 未做真实实例验证，本次其驱动和既有本站日志扩展没有变更。

本机临时工作树旁的构建、验证文件仍在 `F:/Relocated/Users/Administrator/.codex/worktrees/rc41-integration/`，另有 `F:/github-fork-pr/main-rc41-build.log`；删除操作被自动审批拒绝（`blocked by policy`），未绕过限制。这些文件不在仓库中、不参与发布，可后续手动清理。

全仓库 lint 仍有既有错误（本次检查 181 个，涉及的 100 个文件均与 rc.41 上游完全一致）；格式检查也发现上游文件的既有差异。冲突处理中修改的前端文件通过定向 lint，未扩大范围处理其他文件。没有进行物理 Passkey 或第三方 SSO 实际账号验证。认证参考 [OWASP Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html) 和 [ASVS 5.0](https://owasp.org/www-project-application-security-verification-standard/)；使用上游认证回归测试验证权限、过期、重放及敏感操作，不宣称全站 ASVS 合规。

整合前 `main`、生产并发版、旧 Claude effort 和 count_tokens 分支提交通过 `archive/2026-09-30/*` 标签保留，便于恢复源码。数据库升级后的生产回退必须结合当时的备份及账务变化评估，不能仅靠切换旧镜像或恢复旧库。

## Claude 拒绝日志标记

用户要在大量日志中直接辨认 HTTP 200 的 Claude 拒绝请求，并确认同时记录类别。上游和生产 `0575582b` 已记录 `other.admin_info.reject_reason = "claude_stop_reason=refusal"`，原界面仅在管理员详情显示；现在桌面表格时间列和手机列表增加共用 `RefusalBadge`，与原消费标签并列显示红色“拒绝”或“拒绝 · cyber”等标记。复用 `StatusBadge`，普通用户和管理员个人日志视图不显示管理员信息。

Claude 非流式响应和 SSE `message_delta.delta` 解析 `stop_details.category`，Claude 转 Responses 的流式路径也传递类别；结算写入现有 `other.admin_info.refusal_category`，详情同步展示。保留原始类别代码（包括以后新增的类别），不记录自由文本 explanation，不推断缺失/null 类别。历史日志有拒绝原因即可显示标记，原来未保存的类别无法回填。无数据库结构、SQL 查询、计费或拒绝重试行为变化，未修改 claudeRelay、生产渠道配置或生产容器。

已通过：`go test ./relay/channel/claude -count=1`（流式/非流式类别及 null/缺失/普通完成）；服务层定向结算和 SQLite 固定价格日志回归；主模块 `GOWORK=off go build ./...`；`cd relaykit && GOWORK=off go build ./... && go test ./dto ./relayconvert/...`。前端类型检查、定向 lint/格式检查、生产构建通过；拒绝原因、手机列表和详情预览三个测试文件共 53 项通过，覆盖管理员边界、旧日志、已知及未来类别、语言切换。七种语言标签已同步；当前 shell 无 Bun，运行现有 npm 脚本与本地 Node 工具，未改依赖。尚未调用真实上游验证新类别，未部署。

协议依据：[Anthropic Refusals and fallback](https://platform.claude.com/docs/en/build-with-claude/refusals-and-fallback)。类别可为 null，拒绝仍以 `stop_reason = "refusal"` 判定。

## 2026-10-01 Claude 拒绝筛选

用户已自行部署 `12c2ed65`，随后指出仅显示标记无法在大量日志中查找拒绝请求，并确认本轮范围：管理员普通请求日志增加“全部请求 / 仅拒绝”筛选；已有历史标记可查询；分页、总数和用量统计同步筛选；不改数据库结构、不增加类别筛选。

界面复用现有 `LogsFilterToolbar`、`LogsFilterField` 和 `Select`，桌面与手机均可使用，搜索重置到第一页，URL 保留筛选状态，重置或选择全部请求可清除条件。所有七种语言已同步。管理员个人日志视图及普通用户不展示、不发送拒绝条件；个人接口忽略该管理员参数，继续移除 `admin_info`。

管理员 `/api/log/` 与 `/api/log/stat` 接收 `refused_only=true`，非法布尔值返回 400。共用数据库条件精确匹配现有 `other.admin_info.reject_reason = "claude_stop_reason=refusal"`，列表计数、分页、用量及最近一分钟 RPM/TPM 均应用该条件。SQLite/MySQL/PostgreSQL 使用各自 JSON 提取语法，ClickHouse 使用 `JSONExtractString`；历史 `RecordLog` 可能保存空 `other`，前三者用 `NULLIF` 表达空记录没有拒绝元数据，非空损坏 JSON 不做静默降级。无新增列、索引、回填或启动迁移；大时间跨度查询仍需要解析范围内的日志，应结合时间、模型、渠道等条件使用，尚未对完整生产日志量做性能测量。

真实数据库验证通过：SQLite **3.50.4**、MySQL **5.7.8-rc / 8.0.46**、PostgreSQL **9.6.24 / 18.4**、ClickHouse **25.3.14.14**。六个引擎/版本场景覆盖历史无类别标记、带空白的 JSON、空记录、普通请求、其他拦截、同名文本/前缀/大小写误匹配、null 原因、分页总数、时间与账号条件叠加、用量及速率。只使用独立 Docker 内部网络和合成数据。旧 MySQL 镜像的 schema v1 格式不能被当前 Docker 直接加载，使用临时 Skopeo 容器转换成现代格式后运行；未修改应用或生产 Docker 配置。

```sh
# 本机：通过（外部数据库在隔离 Linux 容器中执行同一个测试二进制）
GOWORK=off go build ./...
go test ./model -run 'TestClaudeRefusalLogFilterDatabaseMatrix|TestConsumeLogsFilterAndSumByUpstreamAccount' -count=1 -timeout=120s
go test ./controller -run '^$' -count=1
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOWORK=off go test -c -p 4 -ldflags '-s -w' -o filter.test ./model
# 隔离验证：六个真实数据库场景通过，实际命令含仅指向测试容器的 TEST_*_DSN
sh /opt/new-api-refusal-filter-test/verify.sh
# 临时接口验证程序：七个管理员列表/统计、参数校验、个人视图契约通过
GOWORK=off go run /path/to/api-check.go
# 前端（当前 shell 无 Bun，未改变依赖）：类型检查、定向 oxlint/oxfmt、生产构建通过
cd web && npm run typecheck && npm run build
# 日志类型、手机筛选、分组、API 参数与拒绝标记五个相关测试文件共 44 项通过
npm run test -- src/features/usage-logs/components/__tests__/log-type-filter.test.tsx src/features/usage-logs/components/__tests__/mobile-filter.test.tsx src/features/usage-logs/components/__tests__/group-filter.test.tsx src/features/usage-logs/lib/__tests__/upstream-account-filter.test.ts src/features/usage-logs/components/__tests__/reject-reason.test.tsx
```

额外运行 `go test ./controller -run '^TestAdmin' -count=1 -timeout=120s` 时，Windows 中既有管理员验证测试因 `audit.db` 仍被占用而在 `TempDir` 清理阶段失败；没有修改这些无关夹具，也未宣称控制器完整回归通过。本次接口契约已单独验证通过。数据库日志及小型验证程序保留在 `F:/Relocated/Users/Administrator/.codex/tmp/new-api-refusal-filter-20261001/`。本轮仅提交代码，未部署或修改生产配置。

## 2026-10-01 OAuth 历史迁移补丁验证

用户说明兼容补丁源于 TiDB 迁出，并授权验证是否可以恢复上游；本轮仅验证，没有删除正式代码中的补丁或部署。候选版本基于 `2b7cc0ab0` 的隔离源码副本，将 `CustomOAuthProvider.Enabled` 恢复为上游 `gorm:"default:false"`，移除 `ensureUnmanagedColumns` 及对应测试调用；主分支业务代码保持原状。

只读确认生产主库为 MySQL **8.0.46** / `new_api`，OAuth 表无记录，`enabled` 为 `tinyint(1)`、允许 NULL、默认 NULL。仅复制该表的 CREATE TABLE，使用合成数据，不复制真实凭据或生产业务数据。生产结构复现的首次迁移只执行 `ALTER TABLE custom_oauth_providers MODIFY COLUMN enabled boolean DEFAULT false`；后续两次迁移均无 DDL。启用、禁用、SQL NULL 状态、启用筛选、默认禁用和 slug 唯一约束均保留。

真实 SQLite **3.50.4**、MySQL **8.0.46**、PostgreSQL **18.4** 上的定向测试通过：每种引擎覆盖新建、旧无默认值字段、缺失字段，并额外覆盖生产 MySQL 表结构，共十个场景；同时运行上游结构、索引、约束与小数默认值回归。候选应用分别验证三个引擎的新建、生产 rc.37 镜像二进制升级、校验和通过的官方 rc.41 二进制升级，共九个完整场景；候选版本每个场景启动两次。用户和令牌额度、订阅额度、禁用套餐、六位小数价格、日志、配置、OAuth 三种状态和唯一性保留，两次候选启动后的列与索引快照一致。MySQL/PostgreSQL 使用独立日志库。

```sh
# 隔离源码副本构建，未修改正式分支源码
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOWORK=off go test -c -p 4 -ldflags '-s -w' -o oauth-migration.test ./model
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOWORK=off go build -p 4 -ldflags '-s -w' -o new-api-candidate .
# 测试二进制在内部 Docker 网络运行，DSN 仅指向新建的独立数据库
./oauth-migration.test -test.v -test.timeout=180s -test.run='Test(OAuthMigrationUpstream|MigrationSchemaStability)$'
# 临时启动/数据/结构验证脚本；首轮 SQLite 完成后，其余引擎在同一内部网络续跑
bash validate.sh
bash validate-container.sh
```

首轮 MySQL/PostgreSQL 因内部网络的宿主端口不可达而未执行迁移；将测试程序和数据检查程序放入同一内部网络后完成验证，未修改应用实现。结论：当前这项 OAuth 历史迁移规避措施可以撤除。验证依据为当前生产表结构和合成样本，不是完整生产数据库副本。

用户随后确认移除补丁。正式 `main` 恢复 `CustomOAuthProvider.Enabled` 的上游默认禁用标签，删除 `ensureUnmanagedColumns`、专用测试及测试夹具中的调用；保留上游账号日志的 ClickHouse 迁移。修改后的两份生产源码与上述已验证候选源码核对一致（统一换行符后逐字相同），两份测试夹具文件与 rc.41 一致。`GOWORK=off go build ./...` 和 `go test ./model -run 'TestMigrationSchemaStability/sqlite|TestSubscriptionGroupTransitionsPreserveAuthVersionAndSessions|TestSubscriptionGroupCacheRefreshFailureDoesNotChangeCommittedResult' -count=1 -timeout=120s` 通过；仅提交代码，未部署或触发生产迁移。

生产容器仍为 `sha-0575582b`、healthy，启动时间 `2026-09-16T02:03:02.53813048Z`、重启次数 0；验证结束后生产字段仍为默认 NULL，表仍无记录。服务器临时应用、数据库容器、网络和目录均已清理。日志、无数据的表结构和小型验证源码保留在 `F:/Relocated/Users/Administrator/.codex/tmp/new-api-oauth-validation-20261001/`，不作为部署工具。本机隔离源码、二进制及上传分块仍在 `F:/DevCache/temp/newapi-oauth-validation-57b423ec8d3849b9b13ef3d9b70afdc2/`：已核对绝对路径的 PowerShell 递归清理被自动审批拒绝（`blocked by policy`），未绕过限制。

## 2026-09-13 rc.37 同步

从 rc.25 同步至 [v1.0.0-rc.37](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.37)，包含 111 个上游提交。合并提交 `d7dafb786`，另采纳上游认证测试 QueryClient 修复，应用代码提交 `c11a171f2277b45a11723a6bf19864660c21abd2`。后续提交仅整理记录，不改变已验证应用逻辑。

保留模型状态页、递归参数覆盖、独立 count_tokens、管理员上游账号日志、条件错误日志、余额专用套餐及同时生效上限、HTML no-store、版本与构建号展示、原有价格。没有视频模型，不转换价格表达式。

已通过前后端构建、认证/价格/订阅等相关测试、Linux 后端 785 个顶层测试，以及 SQLite/MySQL/PostgreSQL 新装和升级验证。生产库副本升级、重启及数据核对通过。全仓库 lint 的 194 个上游既有错误未扩大范围处理；未测试用户物理 Passkey 或第三方 SSO 实际账号。

首次上线曾采用本机编译、服务器临时镜像，现按用户要求恢复 GitHub 构建发布流程。首次生产验证中，HTTPS 登录/刷新/注销通过；Qwen3.8-27B 通过原有 qwen 分组返回 200，用户、令牌、日志各扣 2 quota，临时账号/令牌已停用，全部临时会话已撤销。原有用户、令牌、渠道、价格、订阅数据核对一致。

## 保留的备份与边界

- 切换前完整备份：服务器 `/opt/new-api-backups/20260912T200711Z-before-rc37/`。含主库和日志库、Redis、应用数据、原部署配置、镜像归档及校验清单。目录仅服务器可读，禁止打印或下载其中的密钥、Cookie、数据库内容。
- 验证和上线结果集中保存在该备份目录的 `validation-results.json`、`production-rc37-result.json`。临时验证库不得恢复至生产。
- rc.25 到 rc.37 有数据库及密码哈希迁移；生产已恢复业务，不能直接恢复旧数据库，否则会丢失后续账务数据。
- Gemini 的既有 AI Studio 上游返回 403 PERMISSION_DENIED，绕过 new-api 也同样失败，本次未修改其生产配置。
- SSH 使用 `ssh -F C:/Users/Administrator/.ssh/racknerd_latency_20260910.conf racknerd-latency`，依赖本机代理连通。不要修改服务器禁止 TCP 转发的策略。
