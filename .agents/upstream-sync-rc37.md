# rc.37 同步与隔离验证（2026-09-12）

## 范围与当前状态

用户授权同步上游、保留个人定制，并在 RackNerd 上用生产库副本验证；**尚未授权生产切换**。没有视频模型，保留既有价格，不执行表达式价格转换。

- 原工作区：`F:/github-fork-pr/new-api`，`main` / `975ad458cce7c54926854d20b04715913d69eb79`，未改动。
- 验证工作区：`F:/github-fork-pr/new-api-rc37-validation`，分支 `codex/upstream-rc37-validation`。
- 上游目标：`v1.0.0-rc.37` / `385d2dfd10d821b25c8a6766bd16eea248cb1652`，相对 rc.25 有 111 个提交。
- 合并提交 `d7dafb7860d0278d0e76c10dfb8a4cd5ac5fab99`；另采纳上游 `be36cbb8f` 的两处认证测试 QueryClient 修复，得到应用构建提交 **`c11a171f2277b45a11723a6bf19864660c21abd2`**。未引入上游 main 的其余发布后改动。
- 已构建服务器本地镜像 `new-api-upgrade-test:rc37-c11a171f2`，镜像 ID `sha256:ecb16170859ff256de55fca1eaf2a4254e33b1aa572d26add2e5f4b438f2f493`；没有推送镜像或代码。
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
- 生产配置：`/opt/new-api/docker-compose.yml`；生产容器 `new-api`，旧镜像 `ghcr.io/paopaoandlingyia/new-api:sha-975ad458`。核查时仍 healthy、restart count=0、启动时间 `2026-08-31T15:57:21.270462112Z`。
- 只使用验证目录下的 compose 管理测试服务；停止测试资源可执行 `docker compose -f /opt/new-api-upgrade-test/compose.json stop`，不会删除数据或生产资源。

## 待批准的上线步骤

先确认是否接受 rc.37 发布说明中的生产风险，再批准生产切换。批准后：短暂维护并等待进行中的请求结束，重新备份生产配置、主库和日志库，固定使用上述已验证镜像，检查启动、登录和实际转发，再恢复访问。此次早先快照仅用于验证，不能替代切换时的新备份。

升级包含数据库迁移和新密码哈希写入；**不能假定只换回旧镜像就能安全回滚**。维护窗口内失败时恢复匹配的旧镜像和数据库备份；一旦开放业务，再回滚数据库需要先处理升级后的新账务数据，防止丢失。既有价格继续保留，不顺便转换表达式。Gemini 权限问题另行征得授权后处理。
