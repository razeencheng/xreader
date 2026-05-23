[English](deployment.md) | **简体中文**

# 部署指南

## 1. 概述

xReader 以单一 Go 二进制文件的形式分发，该二进制文件同时提供 API 服务、内嵌静态 Next.js 前端，并运行 RSS/AI 工作进程——一切都在一个进程中完成。唯一的外部依赖是 Postgres 16 数据库。

预构建的容器镜像发布在两个镜像仓库：

- **GitHub Container Registry：** `ghcr.io/razeencheng/xreader`
- **Docker Hub：** `razeencheng/xreader`

二进制文件在启动时会自动执行数据库迁移，生产环境无需手动迁移。首次运行时，xReader 会将 `SETUP TOKEN` 打印到标准输出；管理员使用该令牌在 `/setup` 页面完成初始配置向导。

> 营销着陆页位于 [`landing/`](../landing/)，单独部署到 Cloudflare Workers，详见 `landing/README.md`。

---

## 2. 使用预构建镜像快速启动

### 2.1 创建 `docker-compose.yml`

```yaml
services:
  xreader:
    image: ghcr.io/razeencheng/xreader:latest   # or razeencheng/xreader:latest (Docker Hub)
    ports:
      - "3000:3000"
    environment:
      DATABASE_URL: postgres://xreader:${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}@postgres:5432/xreader?sslmode=disable
      SESSION_SECRET: ${SESSION_SECRET:?set SESSION_SECRET (openssl rand -hex 32)}
      COOKIE_SECURE: "true"
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:3000/health"]
      interval: 10s
      timeout: 3s
      retries: 3

  postgres:
    image: postgres:16
    # No host port published — only the xreader service reaches it over the compose network.
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_USER: xreader
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      POSTGRES_DB: xreader
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U xreader"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  pgdata: {}
```

### 2.2 生成密钥并启动

```bash
export SESSION_SECRET=$(openssl rand -hex 32)
export POSTGRES_PASSWORD=your-strong-db-password

docker compose up -d
```

### 2.3 获取 Setup Token

首次运行时，xReader 会在日志中打印一次性配置令牌：

```bash
docker compose logs xreader | grep "SETUP TOKEN"
```

复制该令牌，然后打开 `http://your-host:3000/setup` 完成配置向导。

---

## 3. 环境变量

| 变量 | 必填 | 说明 |
|---|:---:|---|
| `DATABASE_URL` | 是 | Postgres 连接字符串，例如 `postgres://user:pass@host:5432/dbname?sslmode=disable` |
| `SESSION_SECRET` | 是 | 用于签名会话 Cookie 的随机密钥。使用 `openssl rand -hex 32` 生成。 |
| `GITHUB_CLIENT_ID` | 否 | GitHub OAuth 应用的 Client ID（也可通过配置向导设置） |
| `GITHUB_CLIENT_SECRET` | 否 | GitHub OAuth 应用的 Client Secret（也可通过配置向导设置） |
| `GITHUB_CALLBACK_URL` | 否 | 完整的回调 URL，例如 `https://your-domain/api/auth/github/callback` |
| `COOKIE_SECURE` | 否 | 在 HTTPS 环境下运行时设为 `true`。生产环境必须开启。 |
| `SETUP_TOKEN` | 否 | 将配置令牌固定为指定值，而不是每次启动时随机生成 |
| `XREADER_AI_ENCRYPTION_KEY` | 否 | 用于加密数据库中 AI 密钥的独立加密密钥 |
| `XREADER_GA_ID` | 否 | Google Analytics 测量 ID（例如 `G-XXXXXXXXXX`） |
| `PORT` | 否 | HTTP 监听端口，默认为 `3000` |
| `XREADER_DEV_MODE` | 否 | 设为 `true` 以在本地开发时允许使用弱 `SESSION_SECRET` |

所有 AI 设置（Base URL、模型、API Key）均经过 AES 加密存储在数据库的 `settings` 表中，并通过配置向导 UI 进行管理——无需通过环境变量配置。

---

## 4. 反向代理与 HTTPS

生产环境中请务必在反向代理后面运行 xReader，以终止 TLS 并将流量转发到 3000 端口。同时设置 `COOKIE_SECURE=true`，确保会话 Cookie 仅通过 HTTPS 发送。

### Caddy 示例

```caddyfile
your-domain.com {
    reverse_proxy xreader:3000
}
```

Caddy 会自动申请并续期 TLS 证书，同时转发 `X-Forwarded-Proto` 请求头，使 xReader 能够识别请求是否通过 HTTPS 到达。

### nginx 示例

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    # TLS 证书配置省略——建议使用 certbot / Let's Encrypt。

    location / {
        proxy_pass         http://127.0.0.1:3000;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$host$request_uri;
}
```

> **重要：** 请确保转发 `X-Forwarded-Proto` 请求头，以便 xReader 生成的重定向 URL 使用正确的协议。

---

## 5. GitHub OAuth 配置

xReader 使用带有允许列表的 GitHub OAuth 进行身份验证。配置步骤如下：

1. 前往 **GitHub → Settings → Developer settings → OAuth Apps → New OAuth App**。
2. 填写以下信息：
   - **Application name：** xReader（或任意名称）
   - **Homepage URL：** `https://your-domain`
   - **Authorization callback URL：** `https://your-domain/api/auth/github/callback`
3. 点击 **Register application**，然后生成 **Client Secret**。
4. 通过环境变量提供凭据：

   ```bash
   GITHUB_CLIENT_ID=Ov23li...
   GITHUB_CLIENT_SECRET=abc123...
   GITHUB_CALLBACK_URL=https://your-domain/api/auth/github/callback
   ```

   或在配置向导 UI 中填写（存储在数据库中，经过加密）。

5. 完成配置向导后，将允许登录的 GitHub 用户名添加到管理员允许列表。

---

## 6. 配置向导（Setup Wizard）

首次启动时，xReader 会在日志中打印一次性 `SETUP TOKEN`：

```
SETUP TOKEN: <token>
```

1. 打开 `https://your-domain/setup`（初始配置期间也可使用 `http://your-host:3000/setup`）。
2. 输入配置令牌进行身份验证。
3. 配置 **AI 提供商**：
   - **Base URL** — 任意 OpenAI 兼容的接口地址（例如 `https://api.openai.com/v1`）
   - **模型** — 例如 `gpt-4o-mini`
   - **API Key** — 加密存储在数据库中
4. 配置 **GitHub OAuth**（如果尚未通过环境变量设置）。
5. 添加初始**管理员允许列表** — 允许登录的 GitHub 用户名。

如果通过环境变量设置了 `SETUP_TOKEN`，向导将在每次启动时使用该固定令牌（适用于自动化部署）。若不设置，则每次服务启动时都会生成新的随机令牌。

---

## 7. 安全注意事项

**Postgres 暴露风险：** 仓库中的开发用 `docker-compose.yml` 为本地开发方便起见，将 Postgres 发布在 `127.0.0.1:5432`。**生产部署绝对不能将 Postgres 暴露给宿主机或公网。** 第 2 节中的生产 Compose 配置有意省略了 postgres 服务的 `ports` 配置项——Postgres 只能通过 compose 内部网络访问。

**SESSION_SECRET：** 始终生成强随机值：

```bash
openssl rand -hex 32
```

不同环境之间不要复用同一个值，也不要将其提交到版本控制系统。

**数据库密码：** 为 `POSTGRES_PASSWORD` 使用长随机密码，避免使用简单或基于字典的密码。

**COOKIE_SECURE：** 在任何 HTTPS 代理后面部署时都应将此项设为 `true`。在生产环境中不设置此项会导致会话 Cookie 通过明文 HTTP 发送。

**AI 密钥：** AI 提供商凭据（API Key、Base URL）使用从 `XREADER_AI_ENCRYPTION_KEY` 派生的密钥（或以 `SESSION_SECRET` 作为回退）进行 AES 加密，存储在数据库中。请妥善保护对数据库和这些环境变量的访问。

---

## 8. 升级

xReader 启动时会自动执行迁移。升级步骤：

```bash
docker compose pull
docker compose up -d
```

新镜像启动后会自动执行所有待处理的数据库迁移，然后开始提供服务。无需手动执行迁移步骤。

> **建议：** 在生产环境中固定到特定镜像标签（例如 `:v1.2.0`）而非 `:latest`，以便主动控制升级时机。

---

## 9. 备份与恢复

### 9.1 使用 pg_dump 进行夜间备份

在宿主机上通过 cron 运行夜间备份。示例脚本：

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR=/data/xreader/backups
mkdir -p "$BACKUP_DIR"

TIMESTAMP="$(date +%F-%H%M%S)"
BACKUP_FILE="$BACKUP_DIR/xreader-$TIMESTAMP.sql.gz"

docker compose -f /data/xreader/deploy/docker-compose.yml exec -T postgres \
  pg_dump -U xreader -d xreader \
  | gzip > "$BACKUP_FILE"
```

建议的 cron 配置（每晚 02:00 运行）：

```cron
0 2 * * * /usr/local/bin/xreader-backup.sh
```

请设置保留策略，防止备份目录无限增长。

### 9.2 备份验证

每天验证备份任务是否正常运行：

1. 确认备份文件存在且非空。
2. 检查 gzip 文件是否可读：

   ```bash
   gzip -t /data/xreader/backups/xreader-YYYY-MM-DD-HHMMSS.sql.gz
   ```

3. 检查最新备份的时间戳。
4. 定期将备份恢复到临时数据库以确认转储文件可用。

### 9.3 恢复流程

需要恢复数据库时：

1. 停止 xreader 服务，防止恢复期间发生写入：

   ```bash
   docker compose stop xreader
   ```

2. 删除并重建数据库：

   ```bash
   docker compose exec postgres psql -U xreader -c "DROP DATABASE xreader;"
   docker compose exec postgres psql -U xreader -d postgres -c "CREATE DATABASE xreader OWNER xreader;"
   ```

3. 将备份恢复到 PostgreSQL：

   ```bash
   gunzip -c /data/xreader/backups/xreader-YYYY-MM-DD-HHMMSS.sql.gz \
     | docker compose exec -T postgres psql -U xreader -d xreader
   ```

4. 重新启动 xreader（启动时会自动执行迁移）：

   ```bash
   docker compose start xreader
   ```

### 9.4 恢复后检查

恢复完成后，请验证以下内容：

- `curl -fsS http://localhost:3000/health` 返回 `{"status":"ok"}`
- xreader 日志显示数据库连接成功
- 文章、订阅源和管理员允许列表的数据行均存在
- Web 应用可正常加载，登录功能正常

### 9.5 灾难恢复注意事项

- 将备份存储在与 Postgres 数据目录不同的卷上。
- 在生产环境需要恢复之前，先在预发布主机上测试恢复流程。
- 如果本地存储本身是故障域，请至少保留一份异地备份。
