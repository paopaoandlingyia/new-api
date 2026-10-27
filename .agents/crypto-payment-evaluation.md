# 加密货币收款方案评估（更新于 2026-10-28）

## 目标与当前状态

- 用户希望自部署，自己掌握资金；USDT 充值入口支持 TRC20 / BEP-20，并尽量减少链上维护。
- 最终取舍：约 10～20 笔/天，继续项目内自建；不引入 Xcash 多地址合约归集或 BEpusdt 服务。
- 最终精度规则：BSC 原始 uint256 必须被 `10^12` 严格整除，才换算成整数 micro-USDT。余数非零的到账完整保存并进入待核对，不截断、不四舍五入，不阻塞其他交易。
- 生产 New API 使用 MySQL 8.0.46。Mac 已有 Colima、本地独立 MySQL 测试实例和 Bun。
- 已实现 BSC 扫描、完整原始金额保存、统一 USDT 入口与创建订单前选链，以及后台 BSC 地址/RPC 配置；完成本地测试与构建。用户已部署上线；2026-10-28 实测 0.160001 USDT 的 BSC 转账完成自动匹配、入账，线上只读核对订单为 success。旧策略从链上到账至入账约 2 分 24 秒。提示精简与后续优化均仅本地提交，按用户要求留待下次更新一起推送。
- 评估与实现期间没有修改服务器、真实钱包或现有线上支付配置。

## BEpusdt

核对源码：`v03413/BEpusdt`，commit `07164853c38ed43700d3abcaab78e832782d36fa`（2026-10-19）。GitHub 最新发布为 v1.24.3（2026-10-21）；发布制品本身未部署验证。

- 原生支持 USDT/TRON、USDT/BSC，有收银台、回调重试和易支付接口；默认 SQLite，无须另装数据库服务。
- 收款钱包模型只配置地址，不要求收款私钥；正常转账直接进入配置的钱包。不是自动生成每用户地址并归集资金的系统。
- 普通模式仍靠地址和金额匹配。USDT 默认金额步长为 0.01，可配置；这减少对六位小数的依赖，但并非保证所有交易所提币渠道都兼容。
- `CalcTradeAmount` 只避让待付款和确认中的订单，因此旧订单结束后金额可以复用。
- `LockTradeAddress` 只锁待付款和确认中的独占订单。独占是临时订单锁，不是永久用户地址。
- `orderTransferMatch` 检查区块时间落在订单有效期内。有效期内付款、之后才扫描到，与过期后才付款是两种情况；不能混为一谈。
- 默认 `block_offset_confirm=0`；EVM 确认代码在该开关开启时才等待配置的确认深度。上线前需要明确确认策略。

### 本地边界验证

在 `/tmp/clauderelay-bepusdt-evaluation-20261027` 的临时克隆中添加评估测试，使用临时 SQLite 和合成转账，无真实资金及外部链上写入：

```sh
go test ./app/task -run '^TestEvaluation' -v -count=1 -timeout 90s
```

结果两个测试通过，确认以下行为：

1. 旧用户的独占订单过期后，地址可被新订单分配。若旧用户此时才付款，转账会满足新订单的地址、金额不限和时间条件。系统无法识别付款人的原始订单意图。
2. 待付金额为 `10`、`10.01` 时，转账 `10.014` 在 `has_prefix` 和 `round_off` 两种模式下同时满足两者；`classic` 拒绝两者。宽松模式不能视为通用的自动补差解决方案。

这些是特定场景的行为验证，不是完整安全审计或主网验收。

## 与现有支付宝共存

当前 `controller/topup.go:GetEpayClient` 只有一组全局 `PayAddress/EpayId/EpayKey`，创建订单和验签共用它。把现有易支付地址直接改成 BEpusdt 会影响已有支付宝入口。

若选 BEpusdt，建议新增独立的网关配置与回调处理；可复用现有充值事务和幂等思路，不能直接覆盖支付宝配置。原生 API 的收银台选链能力比复用单一易支付配置更符合统一 USDT 入口的目标。

## 其他候选

- **Xcash (`xca-sh/xcash`)**：核对 commit `f4327a2`（2026-10-26）。支持 BSC/TRON；`ensure_deposit_address` 按客户和链返回稳定的专属合约地址，模型有客户/链及链/地址唯一约束，更贴合余额充值。需要 PostgreSQL、Redis、后台任务、系统 Gas 钱包和合约部署/归集；归集地址配置不可随意改动，备份包含数据库与加密配置。只做了源码核对，未启动、未审计合约、未测试网/主网验证，不能声称已可靠。
- **PayRam**：官方当前支持网络清单列出 Ethereum、Base、Polygon、Tron、Bitcoin，未列 BSC，暂不作为此次 TRC20+BEP-20 的首选。

## 结论

用户已排除 Xcash：稳定的专属地址更适合识别用户，但合约部署、归集 Gas、系统钱包及多服务运维不符合当前轻量目标。

项目内自建和 BEpusdt 均可使用每链一个固定收款地址，不需要多地址自动归集。接收转账本身不需要收款系统发起链上交易；以后转出资金仍需承担链上费用。自建复用 New API 的服务、数据库和管理界面；BEpusdt 需要另部署网关、备份其数据库，并接入独立配置与回调，但可复用现成多链维护。

用户已决定采用现有自建扩展，保留六位订单精度和永久金额预留，接受不匹配到账人工核对。付款页面保留准确金额、网络和有效期提示；按用户要求，移除了六位小数与提现操作的额外提示。后台仍严格匹配，没有验证的交易所不能宣称兼容。BEpusdt 保留为将来多链、多项目共用收银台时的候选。

## 扫描延迟与时间显示（2026-10-28）

- 用户同意取消 BSC 额外的两分钟缓冲。BSC 窗口截止于当前时间，但实际查询和持久化进度最多到 RPC 的 finalized 区块时间；节点落后不能把进度推进到尚未查询的数据。保留分页恢复、每分钟重叠及原有调度频率。TRON 仍保留两分钟缓冲。
- 付款页分链提示预计到账时间：BSC 约 1 分钟，TRON 约 2～3 分钟，网络确认可能更久；这是预期而非时限保证。上述策略与交易所提币处理和金额精度无关。
- 截止时间复用 `@/lib/format` 的 `formatTimestampToDate(..., 'milliseconds')`，遵循浏览器本地时区，不固定北京时间。剩余时间按绝对截止时间每秒重算，不在客户端判定订单过期；服务端继续使用 Unix 时间戳。
- `useCountdown` 是递减并归零后重置的验证码冷却工具，不符合付款绝对截止时间的语义；本次在现有付款组件内直接计算，未修改共享 hook。
- 本轮 `TEST_MYSQL_DSN=… go test ./controller -run 'TestCrypto' -count=1 -v` 通过（SQLite 3.50.4 / MySQL 8.0.46；PostgreSQL 未配置）。覆盖最近十秒内的到账、最终确认进度、分页恢复和链隔离。付款组件 5 项测试在本地时区及 `TZ=America/Los_Angeles` 下均通过；类型检查、修改文件 lint 和前端构建通过。

## 本地验证与上线准备

- `TEST_MYSQL_DSN='…本机专用测试库…' go test ./controller -run 'TestCrypto' -count=1 -v`：SQLite 3.50.4 和 MySQL 8.0.46 通过，PostgreSQL 未配置并跳过。覆盖旧表新增原始金额字段、重复迁移保留数据、金额精度、链隔离、幂等充值、分页恢复和扫描故障隔离。
- `bun run typecheck`：通过。
- `bun run test` 指定钱包付款、选链、金额计算及加密支付后台的五个测试文件：14 项通过。
- 修改的十个 TS/TSX 文件单独运行 `oxlint -c .oxlintrc.json <files>`：通过。全仓 `bun run lint` 仍有未改动文件的既有报错，未扩大范围修复。
- `bun run build` 与 `go build -o /tmp/new-api-bsc-development .`：通过。
- 实际 `BSCScanner` 对 `https://bsc-rpc.publicnode.com` 的主网只读扫描通过，验证链 ID、最终确认区块、历史区块查询及日志接口；测试窗口内该查询地址没有到账，不代表完成真实付款验收。此公共 RPC 未写入默认配置。
- 线上已配置 BSC 地址与 RPC，并通过小额真实付款验收；后续扫描优化尚未部署验证。RPC 仍需支持 `eth_getLogs` 和 `finalized` 区块。程序仅查询链上数据，不保存钱包私钥或自动归集。
- 每个网络/收款地址/基准价格有 9999 个六位识别尾数，永久预留。耗尽会明确报错，需要调整价格或收款地址；不能通过复用旧金额来增加容量，否则会引入迟付归属歧义。

## 主要资料

- [BEpusdt 源码](https://github.com/v03413/BEpusdt/tree/07164853c38ed43700d3abcaab78e832782d36fa)
- [BEpusdt 匹配与地址分配](https://github.com/v03413/BEpusdt/blob/07164853c38ed43700d3abcaab78e832782d36fa/app/model/order.go#L374)
- [BEpusdt 到账匹配](https://github.com/v03413/BEpusdt/blob/07164853c38ed43700d3abcaab78e832782d36fa/app/task/transfer.go#L142)
- [BEpusdt 用户反馈与维护者对误差匹配的说明](https://github.com/v03413/BEpusdt/issues/214)
- [Xcash](https://github.com/xca-sh/xcash)
- [PayRam 支持网络](https://docs.payram.com/support/supported-networks-and-coins)
