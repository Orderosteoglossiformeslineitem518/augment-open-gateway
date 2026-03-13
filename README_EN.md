# Augment Gateway

[中文](./README.md) | English

> [!WARNING]
> This project was built with 90% AI + 10% human effort. It is for technical research purposes only. No issues will be accepted. Feel free to fork / modify. For commercial use, please credit the original source.

> [!NOTE]
> **From the author:** I disagree with AugmentCode's business philosophy and technical direction. The company is arrogant toward its users, dismissive of their needs, and lacks any sense of contractual integrity. Their product is shipped without proper testing, and the recently launched "Intent" feature is nothing but garbage. Its best outcome would be getting acquired and sidelined, or simply going out of business. I hope Augment's product team and leadership realize that your ACE is no longer your moat — shut down the company and go enjoy life with the money!

> AugmentCode API proxy gateway built with Go and Vue.js

Augment Gateway: A high-performance API proxy gateway for [AugmentCode](https://www.augmentcode.com/), providing TOKEN management, load balancing, user management, and real-time monitoring. Supports multi-user sharing, external channel forwarding, and OAuth 2.0 authorization.

[![Go Version](https://img.shields.io/badge/Go-1.23.6+-blue.svg)](https://golang.org)
[![Vue Version](https://img.shields.io/badge/Vue-3.4+-green.svg)](https://vuejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://docker.com)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## ✨ Features

- **Proxy Forwarding** — Direct TOKEN proxy and user token proxy modes, automatic tenant routing, streaming support
- **Load Balancing** — Round-robin, random, weighted, least-connections strategies with 24h sticky sessions
- **TOKEN Management** — Hierarchical system TOKEN and user token management with status monitoring and auto-expiry detection
- **User System** — User registration/login, invitation codes, quota management
- **External Channels** — Third-party AI model service integration (OpenAI, Anthropic, etc.) via unified gateway
- **Security** — JWT authentication, rate limiting, usage quotas, Cloudflare Turnstile, maintenance mode
- **Monitoring** — Real-time request monitoring, Telegram ban notifications, visual dashboards
- **Health Checks** — Configurable scheduled channel health checks for Token and external channel availability

## 📸 Screenshots

### User Dashboard

<table>
  <tr>
    <td><img src="screenshots/用户端-首页.png" alt="Home Page" /></td>
    <td><img src="screenshots/用户端-账号面板.png" alt="Account Panel" /></td>
  </tr>
  <tr>
    <td><img src="screenshots/用户端-数据面板.png" alt="Data Panel" /></td>
    <td><img src="screenshots/用户端-外部渠道.png" alt="External Channels" /></td>
  </tr>
  <tr>
    <td><img src="screenshots/用户端-插件下载.png" alt="Plugin Download" /></td>
    <td></td>
  </tr>
</table>

### Admin Panel

<table>
  <tr>
    <td><img src="screenshots/管理后台-仪表盘.png" alt="Admin Dashboard" /></td>
    <td><img src="screenshots/管理后台-Token管理.png" alt="Token Management" /></td>
  </tr>
</table>

## 🛠 Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.23.6 · Gin · GORM · JWT |
| Database | MySQL 8.0 · Redis 7 |
| Frontend | Vue.js 3 · Element Plus · ECharts · Pinia |
| Deployment | Docker · Docker Compose · GitHub Actions |

## 📁 Project Structure

```
augment-gateway/
├── cmd/
│   └── main.go                    # Application entry
├── internal/                      # Backend core
│   ├── config/                    # Configuration (env vars)
│   ├── database/                  # Database models & initialization
│   ├── handler/                   # HTTP request handlers
│   ├── service/                   # Business logic layer
│   ├── repository/                # Data access layer
│   ├── proxy/                     # Proxy core module
│   ├── middleware/                # Middleware (CORS, etc.)
│   ├── logger/                    # Logging module
│   └── utils/                     # Utility functions
├── web/                           # Frontend Vue.js app
│   ├── src/
│   │   ├── components/            # Vue components
│   │   ├── views/                 # Page views
│   │   ├── api/                   # API client
│   │   ├── store/                 # Pinia state management
│   │   └── router/                # Router configuration
│   └── public/                    # Static assets
├── .github/                       # GitHub Actions CI/CD
├── docker-compose.yml             # Docker orchestration
├── Dockerfile                     # Multi-stage build
└── .env.example                   # Environment variables example
```

## 🚀 Quick Start

### Docker Deployment (Recommended)

```bash
# Clone the project
git clone <repository-url>
cd augment-gateway

# Configure environment variables
cp .env.example .env
# Edit .env — at minimum change database password and JWT secret

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f app
```

After startup:
- 🌐 Application: `http://localhost:28080`
- 🔧 Admin Panel: `http://localhost:28080/admin`

Default admin credentials: `admin` / `admin123` (change immediately after first login)

### Local Development

**Requirements:** Go 1.23.6+ · MySQL 8.0+ · Redis 6.0+ · Node.js 18+

```bash
# Install backend dependencies
go mod tidy

# Install frontend dependencies
cd web && npm install && cd ..

# Configure environment variables
cp .env.example .env
# Edit .env with your database and Redis connection details

# Start backend
export $(cat .env | grep -v '^#' | xargs)
go run cmd/main.go

# Start frontend (new terminal)
cd web && npm run dev
```

Local access:
- Frontend: `http://localhost:23000`
- Backend API: `http://localhost:8080`

## ⚙️ Configuration

All configuration is injected via environment variables. See [.env.example](.env.example) for a complete example.

### Core (Required)

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `MYSQL_HOST` | MySQL host address | `localhost` | ✅ |
| `MYSQL_PORT` | MySQL port | `3306` | |
| `MYSQL_USERNAME` | MySQL username | `root` | ✅ |
| `MYSQL_PASSWORD` | MySQL password | _(empty)_ | ✅ |
| `MYSQL_DATABASE` | Database name | `augment_gateway` | |
| `REDIS_HOST` | Redis host address | `localhost` | ✅ |
| `REDIS_PORT` | Redis port | `6379` | |
| `REDIS_PASSWORD` | Redis password | _(empty)_ | |
| `REDIS_DB` | Redis database index | `2` | |

### Server

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `SERVER_HOST` | Listen address | `0.0.0.0` | |
| `SERVER_PORT` | Listen port | `8080` | |
| `SERVER_MODE` | Run mode: `debug` / `release` / `test` | `debug` | |

### Security

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `JWT_SECRET` | JWT signing key — **must change in production** | _(empty)_ | ✅ |
| `JWT_EXPIRES_IN` | JWT access token expiry (seconds) | `86400` (24h) | |
| `JWT_REFRESH_EXPIRES_IN` | JWT refresh token expiry (seconds) | `604800` (7d) | |
| `ADMIN_USERNAME` | Default admin username (created on first start) | `admin` | |
| `ADMIN_PASSWORD` | Default admin password (created on first start) | `admin123` | |
| `ADMIN_EMAIL` | Default admin email | _(empty)_ | |
| `BATCH_IMPORT_AUTH_TOKEN` | Auth token for batch TOKEN import API | _(empty)_ | |

### Proxy

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `FORWARD_DISABLED` | Disable forwarding (maintenance mode) | `false` | |
| `ENABLE_CUSTOM_USER_AGENT` | Enable custom User-Agent | `false` | |
| `SUBSCRIPTION_USER_AGENT` | Custom User-Agent value | _(empty)_ | |
| `ENABLE_VERSION_CHECK` | Enable VSCode Augment version check | `true` | |
| `MIN_VSCODE_AUGMENT_VERSION` | Minimum supported VSCode Augment version | `0.594.0` | |
| `ENABLE_CONVERSATION_ID_REPLACEMENT` | Enable conversation_id replacement | `true` | |
| `SCHEDULE_TASK_ENABLED` | Enable shared account credit deduction task | `true` | |

### Data Modification

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `ENABLE_MODELS_MODIFICATION` | Enable /get-models response modification | `false` | |
| `ENABLE_SUBSCRIPTION_MODIFICATION` | Enable /subscription-info response modification | `true` | |

### Frontend

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `FRONTEND_URL` | Frontend URL for OAuth callback redirects | _(empty)_ | |
| `FRONTEND_STATIC_PATH` | Frontend static files path | `./web/dist` | |

### Telegram Notifications

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `TELEGRAM_ENABLED` | Enable Telegram notifications | `false` | |
| `TELEGRAM_BOT_TOKEN` | Telegram Bot Token | _(empty)_ | If enabled |
| `TELEGRAM_CHAT_ID` | Telegram group/channel ID | _(empty)_ | If enabled |

### Cloudflare Turnstile

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `TURNSTILE_ENABLED` | Enable Turnstile captcha | `false` | |
| `TURNSTILE_SECRET_KEY` | Turnstile secret key | _(empty)_ | If enabled |

### Logging

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `LOG_LEVEL` | Log level: `debug` / `info` / `warn` / `error` | `info` | |
| `LOG_FORMAT` | Log format: `json` / `text` | `text` | |
| `LOG_ENABLED` | Enable log output | `true` | |

### User Authentication

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `USER_MIN_PASSWORD_LENGTH` | Minimum password length | `6` | |
| `USER_DEFAULT_RATE_LIMIT` | Default rate limit for new users (req/min) | `30` | |
| `USER_DEFAULT_MAX_REQUESTS` | Default max requests for new users (-1=unlimited) | `-1` | |

## 📖 Key Features

### Proxy Forwarding
- **User Token Proxy**: `Client → Gateway → Validate Token → Load Balance → Forward`
- **Direct TOKEN Proxy**: `Client → Gateway → Validate TOKEN → Forward`
- 24-hour sticky sessions, smart retry, automatic failover

### TOKEN Management
- Hierarchical system TOKEN and user token management
- TOKEN status monitoring (active/expired/disabled)
- User-submitted shared TOKENs with custom proxy addresses

### External Channels
- Integration with OpenAI, Anthropic, Google and other third-party AI model services
- Unified gateway forwarding with model mapping and weight configuration
- Channel health checks and automatic failover

### User System
- User registration/login with invitation code mechanism
- Admin can manually allocate quotas, ban/unban users

### Monitoring & Statistics
- Real-time request monitoring and success rate statistics
- TOKEN usage rankings and trend analysis
- System announcements and plugin notification management

## 📄 License

[MIT](LICENSE)
