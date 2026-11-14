# 自用 New API

## 当前部署：Northflank

2026-11-14 已将 RN 私人实例的完整 SQLite 数据和原有会话、加密密钥迁入
Northflank `paop47s-team / first / new-api`。访问地址：
<https://p01--new-api--5wqvlnwqsnwm.code.run>。

- 镜像：`ghcr.io/paopaoandlingyia/new-api:sha-7e7d993a`，与 RN 实例相同。
- 单实例，0.1 vCPU / 256 MB，`GOMEMLIMIT=192MiB`，不使用 Redis 或独立数据库。
- 免费持久卷 `new-api-data`：6 GB NVMe，挂载 `/data`。
- HTTP 容器端口 `3000`，公网端口名 `p01`，平台提供 HTTPS。
- HTTP 就绪检查：`/api/status`。

运行时变量：

```dotenv
TZ=Asia/Shanghai
DEFAULT_LANGUAGE=zh-CN
NODE_NAME=new-api-personal
GOMEMLIMIT=192MiB
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://p01--new-api--5wqvlnwqsnwm.code.run
SQLITE_PATH=/data/new-api.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate
```

另需保留原有 `SESSION_SECRET` 和 `CRYPTO_SECRET`，不可重新生成；它们保存在
Northflank 运行时变量中，不应提交到 Git。管理员账号、渠道和历史记录沿用原站，
注册仍关闭。Claude Relay 渠道地址改为 `https://cr.etgpao.de`，Vercel 渠道保持原配置。

迁移时停止 RN 私人容器，以 SQLite backup API 生成一致快照；只在快照内更新
`ServerAddress`，导入到云端未使用的 `/data/new-api.db`，核对 SHA-256 后切换路径并重启。
完整性检查通过，备份包含 2 个用户和 2 条渠道；导入前后哈希一致。已确认 HTTPS、
原管理员登录、注册关闭及匿名接口鉴权，模型调用由用户自行验证。

RN 私人容器 `new-api-personal` 保持停止，原站和 Claude Relay 未改动。
原始目录保留在 `/opt/new-api-personal/`，迁移快照和配置位于
`/opt/new-api-personal/backups/northflank-20261114/`。
本机敏感备份位于 `~/.codex/private/northflank-new-api-paop47/`，不纳入 Git。

后续迁移应先停止写入，再制作一致快照，或在停机后保留整个 `/data` 目录（包括
可能存在的 WAL/SHM 文件）。同时迁移两个原密钥，更新服务器地址和 HTTPS Cookie
受信任地址。云端已有新写入时，回迁 RN 前必须先同步最新数据，不能直接启动旧库。

## RN 备用部署

此目录的 Compose 文件用于独立服务器部署。RN 上的目录是 `/opt/new-api-personal/`，旧地址为
`http://172.245.225.254:3211`。该实例使用与现有站点相同的镜像，数据独立；
SQLite 数据库及请求日志保存在 `data/`，不依赖 MySQL 或 Redis。
容器最多使用 1 核 CPU、768 MiB 内存，控制台日志最多保留约 30 MiB。

## 首次部署

将此目录复制到服务器的 `/opt/new-api-personal/`，复制 `.env.example` 为 `.env`，
分别生成并填写两个密钥，设置 `.env` 权限为 `600`。私有镜像需先登录 GHCR；
服务器已有同版本镜像时可直接复用。

先保持 `NEW_API_PERSONAL_BIND_ADDRESS=127.0.0.1`，运行：

```sh
cd /opt/new-api-personal
docker compose up -d
```

通过 SSH 转发 `3211` 端口完成管理员初始化、启用自用模式并关闭注册。
完成后将 `.env` 的 `NEW_API_PERSONAL_BIND_ADDRESS` 改为 `0.0.0.0`，再次运行
`docker compose up -d` 开放 IP 访问。当前部署的初始管理员凭据保存在服务器的
`/opt/new-api-personal/admin-credentials.txt`，权限为 `600`。

IP 直连使用 HTTP，密码、会话和 API 请求没有 TLS 传输加密；需要加密访问时可使用
SSH 隧道。此配置为 HTTP 登录设置了 `SESSION_COOKIE_SECURE=false`。

## 检查状态

```sh
cd /opt/new-api-personal
docker compose ps
docker compose logs --tail=100
```

## 迁移

在旧服务器停止实例后打包整个目录，确保 SQLite 文件一致：

```sh
cd /opt/new-api-personal
docker compose stop
tar -C /opt -czf /root/new-api-personal.tar.gz new-api-personal
```

通过 SSH 将压缩包复制到新服务器，登录 GHCR 后解压并启动：

```sh
tar -C /opt -xzf /root/new-api-personal.tar.gz
cd /opt/new-api-personal
docker compose up -d
```

保留 `.env` 中的两个密钥及整个 `data/` 目录，包含可能存在的 `-wal`、`-shm` 文件。
更新客户端中的服务器 IP；同时在管理后台更新服务器地址设置。
迁移后保持旧实例停止，避免两份数据库各自记录用量。压缩包包含密钥、渠道信息和
管理员凭据，应按敏感文件保存。
