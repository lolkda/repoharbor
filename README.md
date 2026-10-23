# RepoHarbor

RepoHarbor 是一个局域网自托管的公开 GitHub 仓库本地镜像管理器。它把目标仓库完整克隆为 bare mirror，按计划自动拉取，并提供手机、平板和桌面都可用的只读代码浏览界面。

## 功能

- 管理公开 GitHub 仓库的本地镜像
- 每个仓库独立设置 cron 同步计划（每 15 分钟 / 每 30 分钟 / 每小时 / 每 12 小时 / 每天，支持自定义）
- 手动同步、暂停/恢复、强制重新镜像
- 完整镜像所有分支、标签和提交历史
- 只读代码浏览：文件树、文件内容、下载、分支/标签切换
- 提交历史、提交详情和 unified diff
- 单管理员密码登录，Argon2id 哈希 + 服务端会话 + CSRF
- 响应式 Web 界面，默认简体中文
- SSE 实时同步状态

## 技术栈

- 后端：Go 1.27、chi、`modernc.org/sqlite`、`sqlc`、`goose`
- 前端：React 18、Vite、TypeScript、Tailwind CSS、shadcn/ui 主题变量（Claude +）、TanStack Query、Shiki
- 数据库：SQLite（WAL）
- 部署：Docker 多阶段构建，前端静态文件内嵌到 Go 二进制

设计文档见 [docs/specs/repoharbor.md](docs/specs/repoharbor.md)。

## 本地开发

需要 Go 1.27+、Node 24+、Git。

```bash
# 终端 1：前端
cd web
pnpm install
pnpm dev

# 终端 2：后端
REPOHARBOR_DATA_DIR=./data REPOHARBOR_ADDR=:8080 go run ./cmd/repoharbor
```

前端开发服务器会把 `/api` 代理到 `http://127.0.0.1:8080`。

首次打开 `http://127.0.0.1:8080`（或 Vite 的 5173）会进入管理员密码设置页。

## Docker 部署

```bash
docker compose up -d --build
```

访问 `http://<host>:8080`。数据保存在命名卷 `repoharbor_data` 中，重启容器不会丢失。

如果使用 bind mount，请确保宿主机目录对容器用户 UID 10001 可写：

```bash
mkdir -p data
chown -R 10001:10001 data
# 然后把 compose 中的 repoharbor_data:/data 改为 ./data:/data
```

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `REPOHARBOR_ADDR` | `:8080` | HTTP 监听地址 |
| `REPOHARBOR_DATA_DIR` | `./data` | 数据目录 |

## 数据目录

```text
/data/
├── repoharbor.db          # SQLite：配置、同步记录、会话
├── repos/                 # bare mirror
│   └── owner__repo.git
└── logs/                  # 同步错误日志
```

## 限制

- 仅支持 `https://github.com/<owner>/<repo>` 形式的公开仓库
- 不下载 Git LFS 对象，不递归同步子模块
- 单实例部署，不支持多副本并发写 SQLite
- 不提供内置备份、代码搜索、通知或统计
- 局域网默认 HTTP，HTTPS 请在外层反向代理配置

## 测试

```bash
go test ./...
cd web && pnpm build
```
