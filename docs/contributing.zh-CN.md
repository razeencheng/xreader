[English](contributing.md) | **简体中文**

# 为 xReader 做贡献

感谢你对 xReader 的贡献兴趣！本指南涵盖了搭建本地开发环境、运行测试以及提交 Pull Request 所需的一切内容。

> 本项目借助 AI 辅助开发。面向 AI agent 和人类贡献者的约定都记录在仓库根目录的 [AGENTS.md](../AGENTS.md) 中。请认真阅读——它是架构细节、命令速查表和注意事项的权威参考。

---

## 1. 架构概览

xReader 的设计刻意保持简单：**一个 Go 二进制文件**完成所有工作。

- **API 服务器** — 为前端和 Fever API 客户端提供 HTTP 接口
- **内嵌静态前端** — 编译后的 Next.js 产物在构建时打包进二进制文件
- **后台 Worker** — RSS 抓取循环和 AI 流水线在同一进程内的 goroutine 中运行
- **自动迁移** — 二进制文件启动时自动应用待执行的 Postgres 迁移，无需手动操作

AI 设置（模型、Base URL、API Key）以加密形式存储在数据库中，通过设置向导 UI 在运行时配置，没有配置文件。环境变量通过 `platform.ConfigResolver` 优先于数据库中的值生效。

完整的包级别详情和命令速查，请参见 [AGENTS.md](../AGENTS.md)。

---

## 2. 前提条件

| 工具 | 最低版本 | 备注 |
|------|---------|------|
| Go | 1.25+ | `go version` |
| Node.js | 20+ | `node --version` |
| pnpm | 最新版均可 | 如未安装：`npm i -g pnpm` |
| Docker | 最新版均可 | 后端测试必需（testcontainers-go 每个测试会启动真实 Postgres 容器） |

---

## 3. 本地开发

### 启动数据库

```bash
make up          # 启动 Postgres 16 容器
```

### 运行后端

```bash
cd server
go run ./cmd/xreader
```

首次运行时，服务器会在控制台打印 **SETUP TOKEN**。在浏览器中打开 `http://localhost:3000/setup`，输入该 Token 来配置 GitHub OAuth 和 AI 设置。

### 运行前端开发服务器

在另一个终端中：

```bash
cd web
pnpm install
pnpm dev         # Turbopack 开发服务器，监听 :3000
```

> 前端开发时，浏览器访问 Next.js 开发服务器（`:3000`）。生产环境中 Go 服务器和前端共用同一端口；本地同时运行两者时，建议将其中一个改为不同端口。

### 常用 Make 命令

```bash
make up          # 启动 Postgres 16 容器
make down        # 停止容器
make build       # 构建前端并编译 Go 二进制
make lint        # 运行所有 Linter（Go + TypeScript）
make test        # 运行所有后端和前端测试
```

---

## 4. 测试

### 运行全部测试

```bash
make test
```

### 后端测试（Go）

```bash
cd server
go test ./...           # 所有包——需要 Docker
go test ./... -race     # 开启竞态检测
```

后端测试使用 **testcontainers-go**：每个测试套件通过 `testutil.SetupTestDB()` 启动一个真实的 Postgres 16 容器。**不存在数据库 Mock**。运行测试前请确保 Docker 已启动。

运行单个测试：

```bash
cd server
go test ./internal/source/... -run TestNormalize -v -count=1
```

### 前端测试

```bash
cd web
pnpm vitest run                                        # 所有测试
pnpm vitest run src/lib/api-client.test.ts             # 单个文件
```

### 代码检查

```bash
make lint
# 或分别执行：
cd server && go vet ./...
cd web && pnpm lint
```

PR 合并前两者都必须通过。

---

## 5. 数据库变更

xReader 使用 **sqlc** 作为数据库访问层。工作流程如下：

1. 在 `server/db/queries/*.sql` 中编写或修改 SQL
2. 运行 `make sqlc-generate` 重新生成 `server/db/gen/` 中的 Go 代码
3. **绝对不要手动编辑** `server/db/gen/` 下的文件——每次生成都会覆盖它们

迁移文件位于 `server/db/migrations/`。新增迁移属于**安全门控**操作，详见第 7 节。

```bash
make sqlc-generate     # 从 SQL 查询重新生成 Go 代码
make migrate-up        # 运行所有待执行迁移（启动时也会自动执行）
make migrate-down      # 回滚一个迁移
```

---

## 6. 提交与 Pull Request 规范

### 提交信息格式

遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

| 前缀 | 适用场景 |
|------|---------|
| `feat:` | 用户可见的新功能 |
| `fix:` | 缺陷修复 |
| `docs:` | 仅文档变更 |
| `refactor:` | 代码重构，无行为变更 |
| `test:` | 添加或修复测试 |
| `chore:` | 构建脚本、依赖、配置等 |

建议加上 scope（范围）：`feat(reader):`、`fix(auth):`、`docs(contributing):` 等。

### Pull Request 检查清单

- [ ] 每个 PR 只包含一个功能或修复——不要捆绑无关变更
- [ ] 新功能必须附带测试；缺陷修复必须包含回归测试
- [ ] `make lint` 通过（Go vet + ESLint + TypeScript）
- [ ] `make test` 通过（后端测试需要 Docker）
- [ ] PR 描述说明**为什么**这样改，而不只是改了什么

---

## 7. 安全门控

以下操作请务必先**向维护者确认**：

- **新增数据库迁移**（`server/db/migrations/`）——Schema 变更影响所有部署环境
- **新增 Go 或 npm 依赖**——需要审查许可证兼容性和供应链安全
- **破坏性 Git 操作**——已推送提交上的 `push --force`、`reset --hard` 或 `commit --amend`

---

## 许可证

提交贡献即表示你同意你的贡献以 **AGPL-3.0** 许可证发布。
