# HeyBlog 生产部署

`Container` 工作流发布 API、Web 和 Worker 镜像到腾讯云 CCR。生产自动部署启用后，
工作流会在镜像发布成功且提交仍是 `main` 最新版本时，发送三个镜像的 digest。现有的
`zhblogs-deployer` 服务名、`/webhooks/deploy` 路径和 HMAC 签名头可以继续使用。

## 主机配置

在当前生产主机上复用现有 Compose 项目 `heyblog-production` 和原有数据库、Redis 与配置文件。
现有运行中的 API 和 Web 必须能从 Docker 读取 CCR 镜像 digest，供回滚使用。旧版没有运行
Worker 时，回滚会停止新 Worker。部署前确认生产 env 已包含新版所需的变量，包括
`API_TASK_WORKER_TOKEN`。不要在部署期间清理旧的本地镜像。

1. 从审核通过的提交构建
   `CGO_ENABLED=0 GOWORK=off go -C apps/deployer build -o /tmp/zhblogs-deployer ./`。
   将二进制安装到现有 systemd 单元的 `ExecStart` 路径。把新版
   `docker-compose.production.yaml` 安装到现有生产运维目录，保留 Compose 项目名称及
   原来的挂载和 env 文件。由管理员管理 Compose 文件和它引用的配置。
2. 如果现有单元以受限的 `zhblogs` 用户运行，安装
   `infra/docker/heyblog-deploy-docker.sh` 为 root 所有的
   `/usr/local/sbin/heyblog-deploy-docker`（模式 `0755`）。在 root 所有的
   `/etc/heyblog/deployer-docker.conf` 中设置绝对路径 `COMPOSE_FILE` 和 `ENV_FILE`，
   与单元配置一致。通过 `visudo` 授予该用户
   `zhblogs ALL=(root) NOPASSWD: /usr/local/sbin/heyblog-deploy-docker`。
   wrapper 只允许读取运行镜像的标识、拉取 HeyBlog 的 CCR 镜像和更新这三个服务。
   它使用宿主机 root 的 Docker 登录配置。若镜像仓库需要认证，配置 CCR 拉取凭证。
3. 保留原有 `zhblogs-deployer` systemd 单元并更新设置。以下路径按实际安装位置填写。

   ```ini
   [Service]
   StateDirectory=heyblog-deployer
   StateDirectoryMode=0700
   Environment=DEPLOYER_STATE_DIR=/var/lib/heyblog-deployer
   Environment=DEPLOYER_COMPOSE_FILE=/srv/zhblogs/core/infra/docker/docker-compose.production.yaml
   Environment=DEPLOYER_ENV_FILE=/srv/zhblogs/core/.env.prod
   Environment=DEPLOYER_DOCKER_BIN=/usr/local/sbin/heyblog-deploy-docker
   Environment=DEPLOYER_LIVE_URL=https://www.heyblog.net/
   EnvironmentFile=/etc/heyblog/deployer.env
   ```

   `/etc/heyblog/deployer.env` 由 root 管理（模式 `0600`），设置至少 32 字节的
   `DEPLOYER_NOTIFY_SECRET`。部署状态及回滚目标保存在 `StateDirectory`。
4. 应用 systemd 配置后，通过本机 `http://127.0.0.1:9401/health` 确认
   `service` 为 `zhblogs-deployer` 且 `api_version` 为 `2`。更新生产 Nginx
   配置中的签名 webhook 反代，执行 `nginx -t` 后再重载。若前面有 EdgeOne，
   确认 `/webhooks/deploy` 及其状态子路径回源到更新后的 Nginx，且绕过缓存。
   可以用无签名的只读请求访问 `/webhooks/deploy/1-1`，预期返回 `401`。

## GitHub 配置

在 `production` Environment 中限制部署来自 `main`，设置变量
`DEPLOYER_NOTIFY_URL=https://www.heyblog.net/webhooks/deploy`，并通过 Environment Secret
存入与主机一致的 `DEPLOYER_NOTIFY_SECRET`。完成主机与入口验证后，将仓库变量
`PRODUCTION_DEPLOY_ENABLED` 设为 `true`，后续成功的镜像发布才会触发部署。删除该变量
可暂停以后自动触发的部署。生产 API 独立运行，已受理的任务会继续完成或回滚。

## 检查与回滚

部署服务先记录实际运行中的旧镜像，然后预拉取新镜像。Compose 等三个容器的健康检查通过后，
从公网请求官网首页并核对页脚里的构建 commit。任一检查失败会恢复已记录的旧镜像，再检查
容器健康和官网是否可访问。工作流会把 `rolled_back` 和 `rollback_failed` 分开报告。
若部署服务在更新容器中途重启，它会读取保存的回滚目标并尝试恢复。

API 启动会应用数据库迁移。自动回滚恢复容器镜像，因此启用自动部署的版本需要让旧版继续
读写迁移后的数据库。破坏兼容性的迁移应单独安排生产上线。
