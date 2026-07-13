当前项目结构如下：
```bash
/
├── apps/
│   ├── api/         # API 服务（Fastify）
│   ├── web/         # 前端应用（Astro + Svelte）
│   ├── cloudflare/  # Cloudflare 相关能力目录
│   └── status/      # 状态监控相关目录
├── packages/
│   ├── db/          # Drizzle ORM schema 与数据库工具
│   ├── configs/     # 共享配置（ESLint/Prettier/TS 等）
│   └── utils/       # 通用工具库
└── scripts/         # 仓库级脚本（hooks 等）
```
需要对模块化的项目结构进行修改：
整体的请求流程结构如下：
设立一个整体的后端进程，用于对接所有的请求和数据库侧的操作，使用数据库的线程池对数据库的请求进行统一接管

```
/
├── apps/
│   ├── web/                  # 前端应用，不变，依旧使用 Astro + Svelte，采用 SSR 渲染，由 server 拦截请求并转发到后端处理后返回页面
│   ├── api/                  # 统一后端服务，使用 GO 语言进行编写，包含 http 的前端 api 请求和其他服务的 rpc 请求调用，并统一管理数据库定义、数据库操作和连接池
│   ├── shell/                # 运维与部署服务，使用 GO 语言进行编写，包含 webhook 触发之后的部署相关内容、nginx 的动态 ip 配置，以及定时清理日志、定时备份数据库、定时升级操作系统依赖、定时刷新源站保护 ip 等操作
│   ├── edge/                 # Edge 服务，包含需要使用 Cloudflare Workers 和 Tencent EdgeOne 进行边缘部署的服务
│   │   ├── cloudflare/       # Cloudflare Workers 相关服务
│   │   └── tencent/          # Tencent EdgeOne 相关服务
│   └── worker/               # 独立于整体后端应用的任务执行服务，包含一些需要独立于整体后端应用的任务执行的服务，例如定时任务等
├── packages/
│   ├── protocol/             # 定义 http / rpc 请求调用的协议、接口契约以及跨语言共享的数据结构
│   ├── configs/              # 共享配置（ESLint/Prettier/TS 等）
│   └── utils/
│       ├── go/               # Go 侧共享工具库
│       └── node/             # Node / TypeScript 侧共享工具库
└── scripts/                  # 仓库级脚本（hooks 等）
```
其中，对于整体后端的 api 服务，分为以下部分：
- http 的前端 api 请求：对接前端的请求，进行数据的处理和返回
- rpc 请求调用：对接其他内部服务的 rpc 请求，对常用的方法和和数据接口进行统一的封装和处理，并以 rpc 的方式提供给其他服务进行调用
- 数据库定义与操作：迁移后统一在 GO 语言侧进行数据库定义、连接池管理和数据库读写操作，不再沿用 Node 侧的 Drizzle 作为长期数据库定义入口

这一版本的改动主要针对前一版本中，所有服务都会建立和数据库的请求连接池，导致数据库连接数过多的问题进行改动，改为在整体后端服务中建立一个数据库连接池，并对所有的数据库请求进行统一的接管和处理，避免了数据库连接数过多的问题，同时也提高了数据库请求的效率和稳定性。

需要注意的是，数据库中的主键、外键、唯一约束、索引、非空约束等关系约束，应当以下沉到数据库本身的 migration / schema 定义为准，由 GO 侧负责维护和映射；前端和 edge 侧不直接共享数据库关系模型，而是只通过 `packages/protocol` 中定义的 HTTP / RPC 契约来共享请求与响应的数据结构。也就是说，GO 侧负责数据库关系和领域实现，TypeScript 侧负责接口消费，两端共享的是协议层的 DTO，而不是数据库实体本身。

对于不同服务使用的调用方法之间的关系如下：
```
用户侧浏览器请求 -> 前端应用（Astro + Svelte）拦截转发 --http--> 统一后端服务（GO） --> 数据库
shell 服务（GO） <--rpc--> 统一后端服务（GO）
edge 服务（Cloudflare Workers / Tencent EdgeOne） <--http--> 统一后端服务（GO）
worker 服务（GO） <--rpc--> 统一后端服务（GO）
```
