# 加密货币收款方案评估（2026-10-27）

## 目标与当前状态

- 用户希望自部署，自己掌握资金；USDT 充值入口支持 TRC20 / BEP-20，并尽量减少链上维护。
- 最终取舍：约 10～20 笔/天，继续项目内自建；不引入 Xcash 多地址合约归集或 BEpusdt 服务。
- 最终精度规则：BSC 原始 uint256 必须被 `10^12` 严格整除，才换算成整数 micro-USDT。余数非零的到账完整保存并进入待核对，不截断、不四舍五入，不阻塞其他交易。
- 生产 New API 使用 MySQL 8.0.46。Mac 已有 Colima、本地独立 MySQL 测试实例和 Bun。
- 已实现 BSC 扫描、完整原始金额保存、统一 USDT 入口与创建订单前选链，以及后台 BSC 地址/RPC 配置；完成本地测试与构建，尚未部署或做真实主网付款验收。
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

用户已决定采用现有自建扩展，保留六位订单精度和永久金额预留，接受不匹配到账人工核对。付款页面在创建订单前提示渠道须支持六位小数，并核对扣费后的实际到账金额；没有验证的交易所不能宣称兼容。BEpusdt 保留为将来多链、多项目共用收银台时的候选。

## 本地验证与上线准备

- `TEST_MYSQL_DSN='…本机专用测试库…' go test ./controller -run 'TestCrypto' -count=1 -v`：SQLite 3.50.4 和 MySQL 8.0.46 通过，PostgreSQL 未配置并跳过。覆盖旧表新增原始金额字段、重复迁移保留数据、金额精度、链隔离、幂等充值、分页恢复和扫描故障隔离。
- `bun run typecheck`：通过。
- `bun run test` 指定钱包付款、选链、金额计算及加密支付后台的五个测试文件：14 项通过。
- 修改的十个 TS/TSX 文件单独运行 `oxlint -c .oxlintrc.json <files>`：通过。全仓 `bun run lint` 仍有未改动文件的既有报错，未扩大范围修复。
- `bun run build` 与 `go build -o /tmp/new-api-bsc-development .`：通过。
- 实际 `BSCScanner` 对 `https://bsc-rpc.publicnode.com` 的主网只读扫描通过，验证链 ID、最终确认区块、历史区块查询及日志接口；测试窗口内该查询地址没有到账，不代表完成真实付款验收。此公共 RPC 未写入默认配置。
- 上线前仍需提供实际 BSC 收款地址、支持 `eth_getLogs` 和 `finalized` 区块的 HTTPS 主网 RPC，并做小额真实付款验收。程序仅查询链上数据，不保存钱包私钥或自动归集。
- 每个网络/收款地址/基准价格有 9999 个六位识别尾数，永久预留。耗尽会明确报错，需要调整价格或收款地址；不能通过复用旧金额来增加容量，否则会引入迟付归属歧义。

## 主要资料

- [BEpusdt 源码](https://github.com/v03413/BEpusdt/tree/07164853c38ed43700d3abcaab78e832782d36fa)
- [BEpusdt 匹配与地址分配](https://github.com/v03413/BEpusdt/blob/07164853c38ed43700d3abcaab78e832782d36fa/app/model/order.go#L374)
- [BEpusdt 到账匹配](https://github.com/v03413/BEpusdt/blob/07164853c38ed43700d3abcaab78e832782d36fa/app/task/transfer.go#L142)
- [BEpusdt 用户反馈与维护者对误差匹配的说明](https://github.com/v03413/BEpusdt/issues/214)
- [Xcash](https://github.com/xca-sh/xcash)
- [PayRam 支持网络](https://docs.payram.com/support/supported-networks-and-coins)
