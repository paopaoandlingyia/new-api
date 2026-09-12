# rc.37 同步、验证与上线（2026-09-13）

## 范围与当前状态

用户授权同步上游、保留个人定制，并在 RackNerd 上用生产库副本验证；**2026-09-13 用户回复“是的开始吧”，已授权按下述步骤生产切换，无需再次询问上线许可**。没有视频模型，保留既有价格，不执行表达式价格转换。

**已于北京时间 2026-09-13 04:21 恢复生产访问，公网确认运行 rc.37 / `c11a171f2`，容器 healthy、restart count=0。上线完成，不能再次执行维护或数据库回退脚本。**

- 原工作区：`F:/github-fork-pr/new-api`，`main` / `975ad458cce7c54926854d20b04715913d69eb79`，未改动。
- 验证工作区：`F:/github-fork-pr/new-api-rc37-validation`，分支 `codex/upstream-rc37-validation`。
- 上游目标：`v1.0.0-rc.37` / `385d2dfd10d821b25c8a6766bd16eea248cb1652`，相对 rc.25 有 111 个提交。
- 合并提交 `d7dafb7860d0278d0e76c10dfb8a4cd5ac5fab99`；另采纳上游 `be36cbb8f` 的两处认证测试 QueryClient 修复，得到应用构建提交 **`c11a171f2277b45a11723a6bf19864660c21abd2`**。未引入上游 main 的其余发布后改动。
- 已验证镜像在服务器提升为生产标签 `ghcr.io/paopaoandlingyia/new-api:sha-c11a171f2`，镜像 ID `sha256:ecb16170859ff256de55fca1eaf2a4254e33b1aa572d26add2e5f4b438f2f493`；没有推送镜像或代码。生产 `.env` 的 `NEW_API_IMAGE` 指向此标签，compose 的 `pull_policy` 从 `always` 改为 `never`，其余解析后的部署配置及应用环境变量一致。
- 后续记录提交仅添加此文档，不改变上述已验证应用代码。

## 更新内容与合并取舍

主要更新：统一登录二次验证与敏感操作授权、账号绑定及审计、多 RP ID Passkey；数据库迁移和唯一约束修复；表达式定价及转换草稿；转发协议/计量修复；任务插件架构；渠道编辑器、价格展示、日志及管理员更新提醒。

[上游发布说明](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.37)仍明确说“不推荐用于生产环境”。GitHub 的 prerelease 标记为 false，不代表发布说明撤回了这条警告。视频价格重新配置要求对当前站点不适用。

合并保留：模型状态页与来源配置、公开模型卡片不显示性能徽章、递归 `exists` 参数覆盖、独立 `count_tokens`、上游账号记录及管理员可见性、条件错误日志、余额专用套餐和同时生效上限、HTML no-store、版本号与构建提交分别展示。新日志结构和手机布局接入了原有上游账号字段；保留数据库非自动迁移字段的补列逻辑。

## 已完成的验证

- Go 1.26.2，Linux amd64 / CGO=0，Bun 1.4.0；前端锁文件安装、类型检查、生产构建成功，后端生产构建成功。`relaykit` 在 `GOWORK=off` 下独立构建和全模块测试成功。
- 本次相对 rc.37 有差异的前端文件 lint、保留版权头的格式检查、`git diff --check` 通过。
- 全仓库 lint 有 **194 个错误，分布于 105 个文件**；这些文件与 rc.37 完全一致，未为了升级扩大范围修复。
- 前端相关功能测试分批通过；其中认证相关 18 个文件 / 154 项全通过；价格卡片、审计查看器、手机日志三文件复查 69 项通过。首次并发运行的审计异步等待失败在单 worker 下通过。
- Linux 后端 `model` / `controller` / `service` / `relay/channel` 分别通过 191 / 349 / 226 / 19 个顶层测试，合计 **785**；配置真实 MySQL、PostgreSQL 及独立日志数据库 DSN。其他 common、middleware、relay providers/helper、性能指标和状态配置相关包已在本机通过。
- Windows 的 audit.db 临时文件删除、服务时间戳及 HTTP/2 连接关闭测试问题未在 Linux 重现，没有为此修改产品逻辑或放宽断言。
- **SQLite、MySQL 8.0.46、PostgreSQL 18：全新安装和 rc.25 升级共六组场景全部通过**；每组重复启动，比较数据和表结构/索引，验证旧账号登录与令牌记录。MySQL、PostgreSQL 包含独立日志库。SQLite 的应用配置不支持指定另一份 SQLite 日志文件，因此该场景使用同库，独立日志路径另由 MySQL/PostgreSQL 验证。
- 生产完整一致性快照（主库和日志库）恢复到隔离 MySQL；先启动原镜像建立基线，再升级、重启。用户余额/密码哈希、令牌及额度、渠道密钥/模型/覆盖参数、价格选项、订阅记录的校验值全部一致；第二次新版启动表结构不变。
- 隔离 HTTP API：登录、刷新、注销后令牌失效、未登录/普通用户权限、HTML no-store；普通对话、流式、工具调用、Claude、Gemini 每次按固定用量精确扣 20 quota，用户和令牌额度同步；失败退款、count_tokens 不收费；递归覆盖生效；上游账号仅管理员可见，普通用户日志无 admin_info。
- 隔离 HTTPS：启用与生产一致的 Secure Cookie 模式，使用只绑定服务器回环地址的临时 TLS 入口；验证证书、Secure / HttpOnly / SameSite=Strict、无 Origin 与错误 Origin 被拒绝、可信来源刷新和注销后失效，全部通过。测试后已恢复隔离环境配置，没有更改生产证书或信任设置。
- 隔离订阅：余额购买精确扣费、重复购买被同时生效上限拦截、余额专用套餐拒绝外部支付入口、订阅优先扣费且不扣钱包。
- 浏览器验证同一生产构建：价格搜索、详情打开/关闭、自定义状态页和模型展开；390px 手机视口布局正常。SSH 服务禁止 TCP 转发，因此浏览器使用本机只读 API 快照预览，**不是浏览器连接远程实例的端到端测试**。服务器 API 集成测试独立完成。

## 实际上游结果

- OpenAI 兼容入口：Qwen3.8-27B 返回 200。
- Claude：原专用入口拒绝普通请求（`official ingress requires a Claude Code-shaped request`），符合原有访问策略；现有兼容入口的 Haiku 返回 200。没有修改原入口限制。
- Gemini：经新版转发返回 403 `PERMISSION_DENIED`；绕过 new-api、直接请求当前 AI Studio 上游也同样返回 403。这是当前上游权限问题，本次未修改其生产配置。
- 实际出站共 6 次（包含失败和重试），均为简短验证提示。模拟请求不出站。

## 安全验证与边界

参考 [ASVS 5.0.0 V6](https://github.com/OWASP/ASVS/blob/v5.0.0_release/5.0/en/0x15-V6-Authentication.md)、[V7](https://github.com/OWASP/ASVS/blob/v5.0.0_release/5.0/en/0x16-V7-Session-Management.md)、Authentication / Session Management Cheat Sheets。重点涉及旧密码校验、敏感操作证明的作用域/单次使用、服务端会话验证及撤销（6.2.3、6.5.1、7.2.1、7.4.1 等）。这不是完整 ASVS 合规审计；第三方 SSO 回调和用户物理 Passkey 未做生产实机验证。

不得把测试环境数据覆盖到生产：其中新增了测试用户、令牌、渠道、套餐和价格项，并清除了监控源、邮件/注册等可能产生副作用的设置。

## 继续工作入口

- 本机详细证据、构建与测试脚本：`F:/github-fork-pr/new-api-rc37-artifacts/`。
- SSH 必须显式使用 `ssh -F C:/Users/Administrator/.ssh/racknerd_latency_20260910.conf racknerd-latency`。默认 SSH 配置有 Windows 所有权问题；不需要改权限。服务端禁止端口转发；不要修改其 SSH 策略。
- 服务器验证目录：`/opt/new-api-upgrade-test`，权限 700；`compose.json`、数据库快照、环境文件、日志及结果 JSON 都在这里。密钥仅留在服务器私有文件中，禁止打印/提交/复制进报告。
- 生产配置：`/opt/new-api/docker-compose.yml`；生产容器 `new-api`，现为上文的 rc.37 固定镜像。旧镜像 `ghcr.io/paopaoandlingyia/new-api:sha-975ad458` 和升级前数据库备份仍保留。
- 只使用验证目录下的 compose 管理测试服务；停止测试资源可执行 `docker compose -f /opt/new-api-upgrade-test/compose.json stop`，不会删除数据或生产资源。

## 上线结果与备份

已执行维护、请求排空和批量计费写入等待、停止旧应用、新备份、固定镜像切换、生产验证和恢复访问。实际维护约 10 分钟（UTC 20:10:56 至 20:21:18），其中多数时间耗在一次性运维检查的修正；数据库备份和新版启动约一分钟。

- 本次切换前新备份：`/opt/new-api-backups/20260912T200711Z-before-rc37/`，仅服务器可读。包含主库和日志库一致性备份 `databases.sql.gz`（32,688,225 字节，gzip 完整性校验通过）、部署配置、Redis RDB、应用数据、旧版和候选镜像归档（113,252,864 字节）及校验清单。早先隔离测试快照不是这次备份。
- 升级前后原有用户余额/密码、令牌额度、渠道密钥/模型/覆盖配置、价格、订阅的哈希一致；生产环境变量和端口映射保持一致。
- 生产 HTTPS 登录、刷新、注销和权限检查通过。refresh Cookie 为 Secure / HttpOnly / SameSite=Strict；另有正常的、供前端读取的 `new_api_has_session` 标记 Cookie。
- 真实 Qwen3.8-27B 请求通过原有 `qwen` 分组返回 200（0.73 秒）；用户、令牌、消费日志均扣 2 quota。临时账号软删除并停用、临时令牌撤销并停用，全部临时会话已撤销。测试未改变任何已有账号余额。
- 原站点 Nginx 配置已原样恢复，四条临时维护防火墙规则全部撤除；隔离测试服务保持停止。公网 `/api/status` 确认 rc.37 / `c11a171f2`。
- 安全结果文件：服务器 `/opt/new-api-upgrade-test/production-rc37-result.json`，本机同名文件在 artifacts 目录。账号和 Cookie 文件仅留在服务器私有备份中，禁止打印或下载。

升级包含数据库迁移和新密码哈希写入；**不能假定只换回旧镜像就能安全回滚**。维护窗口内失败时恢复匹配的旧镜像和数据库备份；一旦开放业务，再回滚数据库需要先处理升级后的新账务数据，防止丢失。既有价格继续保留，不顺便转换表达式。Gemini 权限问题另行征得授权后处理。

## 后续工作方式

用户指出此次脚本与验证流程过重。后续默认沿用其已有的“合并代码 → GitHub 构建镜像 → 备份 → 升级检查”流程，只为具体风险增加专项验证，不重复本次全套验证或把一次性脚本建设成长期系统。本次代码仍在本机验证分支、原 main 未改动，镜像仅在服务器；尚未发布到 GitHub/GHCR，不要假定此标签能从仓库拉取。

连接阻塞在用户开启虚拟网卡后消失，继续使用原 SSH 配置及主机指纹。运维检查曾因内核缺少规则 comment 模块、Cloudflare 拒绝服务器公网自检、脚本未跟随 `/model-status/` 的正常重定向及丢失重复 Set-Cookie 头而中断；修正的均为临时脚本，没有改产品逻辑或服务器模块。首次临时令牌误选 default 分组，请求在分发前被拒；改选现有 qwen 分组后成功，无渠道配置变更。
