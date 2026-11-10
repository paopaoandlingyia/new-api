# 自用 New API

RN 上的部署目录是 `/opt/new-api-personal/`，访问地址为
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
