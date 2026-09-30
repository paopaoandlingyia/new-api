# new-api 上游同步与部署记录

## 当前生产边界

用户于 2026-09-30 明确要求：生产容器正在提供服务，暂时不得重启或切换镜像。代码整合、Git 推送、隔离验证不代表生产切换授权。下述部署步骤仅在用户后续明确允许切换时执行。

当前生产仍为 `ghcr.io/paopaoandlingyia/new-api:sha-0575582b`，对应 `codex/group-user-concurrency-limit` 的 rc.37 代码。删除源码分支不会改变正在运行的镜像。

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
- 保留独立 Claude `count_tokens`、模型状态页及来源管理、管理员上游账号日志、条件错误日志、统计隐私处理、HTML no-store、版本与构建号展示、1Panel 部署配置及个人镜像工作流。既有管理员定价配置不改写。
- 上游新增认证、请求策略、任务插件等功能随发布版整体同步，不另行重构。旧的 Custom OAuth 布尔字段迁移修复继续保留。

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
