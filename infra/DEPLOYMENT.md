# zhblogs Debian 部署操作指南

## 1. 目标结构

- 仓库：`/srv/zhblogs/core`
- deployer 二进制：`/srv/zhblogs/bin/zhblogs-deployer`
- 生产环境变量：`/srv/zhblogs/core/.env.prod`
- Docker Compose：`/srv/zhblogs/core/infra/docker/docker-compose.prod.yml`
- Nginx 源配置：`/srv/zhblogs/core/infra/nginx/zhblogs.conf`
- Nginx 生效配置：`/etc/nginx/sites-available/zhblogs.conf`

对外只开放 `22/80/443`。

## 2. 用户分级

```bash
adduser ops
usermod -aG sudo ops

adduser --system --group --home /srv/zhblogs --shell /usr/sbin/nologin zhblogs
install -d -o zhblogs -g zhblogs -m 0750 /srv/zhblogs
install -d -o zhblogs -g zhblogs -m 0750 /srv/zhblogs/bin
usermod -aG zhblogs myxx
```


- `root`：初始化、安装系统服务、维护 sudoers。
- `ops`：日常 SSH 登录和人工维护。
- `zhblogs`：只运行 deployer，不允许 SSH 登录，不加入 `docker` 组。

## 3. 安装系统依赖

```bash
```

## 4. PostgreSQL

```bash
sudo -u postgres psql
```

```sql
CREATE USER zhblogs_app WITH PASSWORD '<strong-password>';
CREATE DATABASE zhblogs OWNER zhblogs_app;
CREATE DATABASE zhblogs_old OWNER zhblogs_app;
\c zhblogs
GRANT ALL ON SCHEMA public TO zhblogs_app;
\c zhblogs_old
GRANT ALL ON SCHEMA public TO zhblogs_app;
```

`postgresql.conf` 只监听本机和 Docker 网关可访问地址；`pg_hba.conf` 只允许本机和 Docker 私网段访问 `zhblogs`。

## 5. Valkey

Valkey 是缓存服务，使用 Docker Compose 内部服务运行。

- 不映射宿主机端口。
- API 通过 compose 网络访问 `valkey://valkey:6379`。
- compose 使用无持久化启动参数：`--save "" --appendonly no`。

## 6. 拉取仓库和环境变量

```bash
sudo -u zhblogs git clone <repo-url> /srv/zhblogs/core
cd /srv/zhblogs/core
sudo -u zhblogs pnpm install --frozen-lockfile
```

创建 `/srv/zhblogs/core/.env.prod`。下面是部署相关变量，还需要补齐 `.env.example` 中标记为生产必须覆盖的业务变量。

```env
NODE_ENV=production
DATABASE_URL=postgresql://zhblogs_app:<strong-password>@host.docker.internal:5432/zhblogs
VALKEY_URL=valkey://valkey:6379

DOCKERHUB_NAMESPACE=<dockerhub-namespace>

WEB_HOST_PORT=127.0.0.1:9101
API_HOST_PORT=127.0.0.1:9201
WORKER_HOST_PORT=127.0.0.1:9301
WEB_PUBLIC_BASE_URL=https://www.zhblogs.net
API_CORS_ORIGINS=https://www.zhblogs.net

DEPLOYER_NOTIFY_SECRET=<hmac-secret>
DEPLOYER_REPO_DIR=/srv/zhblogs/core
DEPLOYER_COMPOSE_FILE=/srv/zhblogs/core/infra/docker/docker-compose.prod.yml
DEPLOYER_SYSTEMD_UNIT=zhblogs-deployer
DEPLOYER_HOST=127.0.0.1
DEPLOYER_PORT=9401
DEPLOYER_RULES_FILE=apps/deployer/deploy-rules.json
DEPLOYER_DOCKER_BIN=/usr/local/sbin/zhblogs-docker
DEPLOYER_SYSTEMCTL_BIN=/usr/local/sbin/zhblogs-systemctl
DEPLOYER_GO_BIN=go
DEPLOYER_BINARY_PATH=/srv/zhblogs/bin/zhblogs-deployer
DEPLOYER_CONFIG_SYNC_COMMAND=sudo /usr/local/sbin/zhblogs-apply-infra
```

`WEB_PUBLIC_BASE_URL` 是主站公开地址，也是认证邮件、密码重置邮件、站点审核通知邮件里的公开链接来源。`API_WEB_BASE_URL` 仅用于 API 需要生成 Web 重定向地址的场景，`/srv/zhblogs/core/.env.prod` 默认不重复设置。

`API_CORS_ORIGINS` 只保留真实公网来源；不要把 `127.0.0.1`、`localhost`、`web` 这类本地或容器内部地址带到生产环境。

CDN 使用 `origin.zhblogs.net` 回源时，回源到 Web 容器的 `Host` 和 `X-Forwarded-Host` 必须保持为 `www.zhblogs.net`，否则 Astro 会把认证表单 POST 判定为跨站提交。

```bash
chown root:zhblogs /srv/zhblogs/core/.env.prod
chmod 0640 /srv/zhblogs/core/.env.prod
```

## 7. 受限 wrapper

`/usr/local/sbin/zhblogs-docker`：

```sh
#!/bin/sh
set -eu

COMPOSE_FILE="/srv/zhblogs/core/infra/docker/docker-compose.prod.yml"

[ "${1:-}" = "compose" ] || exit 1
[ "${2:-}" = "-f" ] || exit 1
[ "${3:-}" = "$COMPOSE_FILE" ] || exit 1

case "${4:-}" in
  pull)
    service_start=5
    ;;
  up)
    [ "${5:-}" = "-d" ] || exit 1
    service_start=6
    ;;
  *)
    exit 1
    ;;
esac

index=1
for service in "$@"; do
  if [ "$index" -lt "$service_start" ]; then
    index=$((index + 1))
    continue
  fi
  case "$service" in
    api|web|worker|valkey) ;;
    *) exit 1 ;;
  esac
  index=$((index + 1))
done

exec sudo /usr/bin/docker "$@"
```

`/usr/local/sbin/zhblogs-systemctl`：

```sh
#!/bin/sh
set -eu

[ "${1:-}" = "restart" ] || exit 1
[ "${2:-}" = "--no-block" ] || exit 1
[ "${3:-}" = "zhblogs-deployer" ] || exit 1

exec sudo /bin/systemctl "$@"
```

`/usr/local/sbin/zhblogs-apply-infra`：

```sh
#!/bin/sh
set -eu

SOURCE="/srv/zhblogs/core/infra/nginx/zhblogs.conf"
TARGET="/etc/nginx/sites-available/zhblogs.conf"
ENABLED="/etc/nginx/sites-enabled/zhblogs.conf"
COMPOSE_FILE="/srv/zhblogs/core/infra/docker/docker-compose.prod.yml"

install -m 0644 "$SOURCE" "$TARGET"
ln -sfn "$TARGET" "$ENABLED"
nginx -t
sudo /usr/local/sbin/zhblogs-docker compose -f "$COMPOSE_FILE" pull valkey
sudo /usr/local/sbin/zhblogs-docker compose -f "$COMPOSE_FILE" up -d valkey
systemctl reload nginx
```

```bash
chown root:root /usr/local/sbin/zhblogs-docker
chown root:root /usr/local/sbin/zhblogs-systemctl
chown root:root /usr/local/sbin/zhblogs-apply-infra
chmod 0755 /usr/local/sbin/zhblogs-docker
chmod 0755 /usr/local/sbin/zhblogs-systemctl
chmod 0755 /usr/local/sbin/zhblogs-apply-infra
```

`/etc/sudoers.d/zhblogs-deployer`：

```sudoers
zhblogs ALL=(root) NOPASSWD: /usr/bin/docker compose -f /srv/zhblogs/core/infra/docker/docker-compose.prod.yml pull *
zhblogs ALL=(root) NOPASSWD: /usr/bin/docker compose -f /srv/zhblogs/core/infra/docker/docker-compose.prod.yml up -d *
zhblogs ALL=(root) NOPASSWD: /bin/systemctl restart --no-block zhblogs-deployer
zhblogs ALL=(root) NOPASSWD: /usr/local/sbin/zhblogs-apply-infra
```

```bash
chmod 0440 /etc/sudoers.d/zhblogs-deployer
visudo -cf /etc/sudoers.d/zhblogs-deployer
```

## 8. Nginx

确认 `infra/nginx/zhblogs.conf` 中：

- `server_name` 是生产域名。
- 证书路径存在。
- 主站反代到 `127.0.0.1:9101`。
- `/webhooks/deploy` 反代到 `127.0.0.1:9401/webhooks/deploy`。

首次生效：

```bash
/usr/local/sbin/zhblogs-apply-infra
```

GitHub Actions 的 `DEPLOYER_NOTIFY_URL` 使用：

```text
https://www.zhblogs.net/webhooks/deploy
```

## 9. deployer systemd

`/etc/systemd/system/zhblogs-deployer.service`：

```ini
[Unit]
Description=zhblogs deployer
After=network-online.target docker.service postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=zhblogs
Group=zhblogs
WorkingDirectory=/srv/zhblogs/core
EnvironmentFile=/srv/zhblogs/core/.env.prod
ExecStart=/srv/zhblogs/bin/zhblogs-deployer
Restart=always
RestartSec=5
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

安装二进制：

```bash
cd /srv/zhblogs/core/apps/deployer
sudo -u zhblogs go build -o /srv/zhblogs/bin/zhblogs-deployer ./
chmod 0750 /srv/zhblogs/bin/zhblogs-deployer
```

启动：

```bash
systemctl daemon-reload
systemctl enable --now zhblogs-deployer
```

## 10. 初始化部署

```bash
cd /srv/zhblogs/core
sudo -u zhblogs pnpm env:prod -- "pnpm -F @zhblogs/db run migrate"
sudo -u zhblogs /usr/local/sbin/zhblogs-docker compose -f /srv/zhblogs/core/infra/docker/docker-compose.prod.yml pull valkey api web worker
sudo -u zhblogs /usr/local/sbin/zhblogs-docker compose -f /srv/zhblogs/core/infra/docker/docker-compose.prod.yml up -d valkey api web worker
```

## 11. 验证

```bash
docker compose -f /srv/zhblogs/core/infra/docker/docker-compose.prod.yml config
curl http://127.0.0.1:9101
curl http://127.0.0.1:9201/health
curl http://127.0.0.1:9401/health
nginx -t
systemctl status zhblogs-deployer
```

外部只验证 HTTPS：

```bash
curl -I https://www.zhblogs.net/
```
