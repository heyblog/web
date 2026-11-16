# 本地演示数据

在仓库根目录运行：

```sh
sh apps/api/scripts/seed-demo.sh
```

也可使用 `mise run //apps/api:seed:demo`。脚本可从任意目录调用，不需要安装本机 psql。
需要 Docker Compose，并先启动 `infra/docker/docker-compose.env.yaml` 中的开发依赖，
启动 API 完成数据库迁移。脚本不会自动启动服务或迁移数据库。

## 数据范围

| 内容 | 数量 / 场景 |
| --- | --- |
| 虚拟博客 | 36 个；32 个公开、2 个隐藏、2 个已移除 |
| 分类 | 复用初始化分类，覆盖开发、工具、摄影、旅行、饮食、写作、数学、艺术、生活 |
| 标签 | 每站一个三级标签；其中 5 站含警告，4 个公开 |
| 软件信息 | 6 种建站程序、语言与部分运行时关联 |
| 订阅源 | 36 个 RSS / Atom / JSON Feed，9 站无订阅源 |
| 资源地址 | 36 个友链页面、18 个站点地图 |
| 友链 | 68 条有向关系，包含 6 组互链、2 个未收录目标、2 个公开孤立站点 |
| 统计 | 每站不同的虚拟累计计数 |
| 边界情况 | 长标题、子路径、区域限制、隐藏与移除状态 |

名称、简介、地址和统计均为虚构；全部站点使用保留的 `.test` 域名，不可用于真实访问、
抓取或验证站点所有权。未创建登录账号、审核申请、公告或文章。

导入后刷新 Web 首页、站点列表和友链图谱即可查看。
示例标识：`Demo00001` / `demo-01`；域名 `blog-01.heyblog.test`。
隐藏站点为 `Demo00033`、`Demo00034`，已移除为 `Demo00035`、`Demo00036`。

## 重复执行与冲突

数据来源为 `LOCAL_DEMO`，固定 UUID 标记整个演示集。再次执行会跳过整批数据，保留已有编辑
和新增计数，不会恢复你修改或移除的演示内容。来源标记不一致或关联数量不完整时拒绝导入。

首次导入不会覆盖已有站点或软件、标签元数据；保留标识、域名冲突或约束失败时整批回滚，
包括图谱写入。脚本不提供清空或强制覆盖选项。不应手工删除来源标记后重试。

脚本固定连接本机 Docker 的 `heyblog-dev-env` 项目、`postgres` 服务、`heyblog` 数据库，
使用容器内本地连接，不读取应用环境文件，也不接受数据库连接参数。远程 Docker 端点会被拒绝。

## 验证

```sh
mise exec -- shellcheck apps/api/scripts/seed-demo.sh
mise --cd apps/api exec -- go test -count=1 -tags=integration ./internal/integration -run TestDemoSeed -v
mise run //apps/api:verify
```

集成测试使用独立 PostgreSQL/AGE 容器，检查数据结构、可见图谱、重复导入保留编辑、来源冲突
和整批失败回滚，不改动开发数据库。
