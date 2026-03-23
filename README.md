# Augment Gateway

[English](./README_EN.md) | 中文

> [!WARNING]
> 本项目业务代码为 90% AI + 10% 人工开发完成，插件对接及接口调试均由人工完成，仅供技术研究参考。欢迎自行 Fork / 二次开发，商用请标明出处。本项目请搭配：[AugmentCode 补丁项目](https://github.com/linqiu919/augment-open-patch) 使用。

> 基于 Go 和 Vue.js 的 AugmentCode API 代理网关系统

Augment Gateway：一个高性能的 API 代理网关，为 [AugmentCode](https://www.augmentcode.com/) 提供 TOKEN 管理、负载均衡、用户管理和实时监控功能。支持多用户共享、外部渠道转发和 OAuth 2.0 授权接入。

[![Go Version](https://img.shields.io/badge/Go-1.23.6+-blue.svg)](https://golang.org)
[![Vue Version](https://img.shields.io/badge/Vue-3.4+-green.svg)](https://vuejs.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://docker.com)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## ✨ 核心特性

- **代理转发** — 支持直接 TOKEN 代理和用户令牌代理两种模式，自动路由到不同租户地址，支持流式传输
- **负载均衡** — 轮询、随机、加权、最少连接等多种策略，24 小时固定分配，TOKEN 失效自动重新分配
- **TOKEN 管理** — 系统 TOKEN 和用户令牌分层管理，支持状态监控、自动过期检测和共享账号管理
- **用户系统** — 用户注册登录、邀请码机制、额度管理
- **外部渠道** — 支持接入第三方 AI 模型服务（OpenAI、Anthropic 等），统一通过网关转发
- **安全控制** — JWT 认证、频率限制、使用额度管理、Cloudflare Turnstile 验证、维护模式
- **监控通知** — 实时请求监控和统计、Telegram 封禁通知、可视化数据展示
- **定时监测** — 用户可配置渠道健康检查，定时验证 Token 和外部渠道可用性

## 📸 系统截图

### 用户端

<table>
  <tr>
    <td><img src="screenshots/用户端-首页.png" alt="用户端首页" /></td>
    <td><img src="screenshots/用户端-账号面板.png" alt="账号面板" /></td>
  </tr>
  <tr>
    <td><img src="screenshots/用户端-数据面板.png" alt="数据面板" /></td>
    <td><img src="screenshots/用户端-外部渠道.png" alt="外部渠道" /></td>
  </tr>
  <tr>
    <td><img src="screenshots/用户端-插件下载.png" alt="插件下载" /></td>
    <td></td>
  </tr>
</table>

### 管理后台

<table>
  <tr>
    <td><img src="screenshots/管理后台-仪表盘.png" alt="管理后台仪表盘" /></td>
    <td><img src="screenshots/管理后台-Token管理.png" alt="Token 管理" /></td>
  </tr>
</table>
## 🛠 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.23.6 · Gin · GORM · JWT |
| 数据库 | MySQL 8.0 · Redis 7 |
| 前端 | Vue.js 3 · Element Plus · ECharts · Pinia |
| 部署 | Docker · Docker Compose · GitHub Actions |

## 📁 项目结构

```
augment-gateway/
├── cmd/
│   └── main.go                    # 应用入口
├── internal/                      # 后端核心代码
│   ├── config/                    # 配置管理（环境变量加载）
│   ├── database/                  # 数据库模型和初始化
│   ├── handler/                   # HTTP 请求处理器
│   ├── service/                   # 业务逻辑层
│   ├── repository/                # 数据访问层
│   ├── proxy/                     # 代理核心模块
│   ├── middleware/                # 中间件（CORS等）
│   ├── logger/                    # 日志模块
│   └── utils/                     # 工具函数
├── web/                           # 前端 Vue.js 应用
│   ├── src/
│   │   ├── components/            # Vue 组件
│   │   ├── views/                 # 页面视图
│   │   ├── api/                   # API 请求封装
│   │   ├── store/                 # Pinia 状态管理
│   │   └── router/                # 路由配置
│   └── public/                    # 静态资源
├── .github/                       # GitHub Actions CI/CD
├── docker-compose.yml             # Docker 编排
├── Dockerfile                     # 多阶段构建
└── .env.example                   # 环境变量示例
```

## 🚀 快速开始

### Docker 部署（推荐）

```bash
# 克隆项目
git clone <repository-url>
cd augment-gateway

# 配置环境变量
cp .env.example .env
# 编辑 .env 文件，至少修改数据库密码和 JWT 密钥

# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f app
```

启动后访问：
- 🌐 应用首页：`http://localhost:28080`
- 🔧 管理后台：`http://localhost:28080/admin`

默认管理员账号：`admin` / `admin123`（请在首次登录后立即修改密码）

### 本地开发

**环境要求：** Go 1.23.6+ · MySQL 8.0+ · Redis 6.0+ · Node.js 18+

```bash
# 安装后端依赖
go mod tidy

# 安装前端依赖
cd web && npm install && cd ..

# 配置环境变量
cp .env.example .env
# 编辑 .env 配置数据库和 Redis 连接

# 启动后端
export $(cat .env | grep -v '^#' | xargs)
go run cmd/main.go

# 启动前端（新终端）
cd web && npm run dev
```

本地访问：
- 前端：`http://localhost:23000`
- 后端 API：`http://localhost:8080`

## ⚙️ 环境变量配置

所有配置通过环境变量注入，完整示例见 [.env.example](.env.example)。

### 基础配置（必须）

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `MYSQL_HOST` | MySQL 主机地址 | `localhost` | ✅ |
| `MYSQL_PORT` | MySQL 端口 | `3306` | |
| `MYSQL_USERNAME` | MySQL 用户名 | `root` | ✅ |
| `MYSQL_PASSWORD` | MySQL 密码 | _(空)_ | ✅ |
| `MYSQL_DATABASE` | 数据库名 | `augment_gateway` | |
| `REDIS_HOST` | Redis 主机地址 | `localhost` | ✅ |
| `REDIS_PORT` | Redis 端口 | `6379` | |
| `REDIS_PASSWORD` | Redis 密码 | _(空)_ | |
| `REDIS_DB` | Redis 数据库索引 | `2` | |

### 服务器配置

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `SERVER_HOST` | 监听地址 | `0.0.0.0` | |
| `SERVER_PORT` | 监听端口 | `8080` | |
| `SERVER_MODE` | 运行模式：`debug` / `release` / `test` | `debug` | |

### 安全配置

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `JWT_SECRET` | JWT 签名密钥，**生产环境必须修改** | _(空)_ | ✅ |
| `JWT_EXPIRES_IN` | JWT 访问令牌过期时间（秒） | `86400`（24h） | |
| `JWT_REFRESH_EXPIRES_IN` | JWT 刷新令牌过期时间（秒） | `604800`（7d） | |
| `ADMIN_USERNAME` | 默认管理员用户名（首次启动创建） | `admin` | |
| `ADMIN_PASSWORD` | 默认管理员密码（首次启动创建） | `admin123` | |
| `ADMIN_EMAIL` | 默认管理员邮箱 | _(空)_ | |
| `BATCH_IMPORT_AUTH_TOKEN` | 批量导入 TOKEN 接口鉴权令牌 | _(空)_ | |

### 代理配置

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `FORWARD_DISABLED` | 禁用转发服务（维护模式） | `false` | |
| `ENABLE_CUSTOM_USER_AGENT` | 启用自定义 User-Agent | `false` | |
| `SUBSCRIPTION_USER_AGENT` | 自定义 User-Agent 值 | _(空)_ | |
| `ENABLE_VERSION_CHECK` | 启用 VSCode Augment 版本检测 | `true` | |
| `MIN_VSCODE_AUGMENT_VERSION` | 最低支持的 VSCode Augment 版本 | `0.594.0` | |
| `ENABLE_CONVERSATION_ID_REPLACEMENT` | 启用 conversation_id 替换 | `true` | |
| `SCHEDULE_TASK_ENABLED` | 启用共享账号积分消耗定时任务 | `true` | |

### 数据修改配置

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `ENABLE_MODELS_MODIFICATION` | 启用 /get-models 接口数据修改 | `false` | |
| `ENABLE_SUBSCRIPTION_MODIFICATION` | 启用 /subscription-info 接口数据修改 | `true` | |

### 前端配置

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `FRONTEND_URL` | 前端访问 URL，用于 OAuth 回调跳转 | _(空)_ | |
| `FRONTEND_STATIC_PATH` | 前端静态文件路径 | `./web/dist` | |

### Telegram 通知

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `TELEGRAM_ENABLED` | 启用 Telegram 通知 | `false` | |
| `TELEGRAM_BOT_TOKEN` | Telegram Bot Token | _(空)_ | 启用时必须 |
| `TELEGRAM_CHAT_ID` | Telegram 群组/频道 ID | _(空)_ | 启用时必须 |

### Cloudflare Turnstile

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `TURNSTILE_ENABLED` | 启用 Turnstile 人机验证 | `false` | |
| `TURNSTILE_SECRET_KEY` | Turnstile 私钥 | _(空)_ | 启用时必须 |

### 日志配置

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `LOG_LEVEL` | 日志级别：`debug` / `info` / `warn` / `error` | `info` | |
| `LOG_FORMAT` | 日志格式：`json` / `text` | `text` | |
| `LOG_ENABLED` | 是否启用日志输出 | `true` | |

### 用户认证配置

| 变量名 | 说明 | 默认值 | 必须 |
|--------|------|--------|:----:|
| `USER_MIN_PASSWORD_LENGTH` | 用户密码最小长度 | `6` | |
| `USER_DEFAULT_RATE_LIMIT` | 新用户默认频率限制（次/分钟） | `30` | |
| `USER_DEFAULT_MAX_REQUESTS` | 新用户默认最大请求次数（-1=无限制） | `-1` | |

## 📖 主要功能

### 代理转发
- **用户令牌代理**：`客户端 → 网关 → 验证令牌 → 负载均衡 → 转发`
- **直接 TOKEN 代理**：`客户端 → 网关 → 验证 TOKEN → 转发`
- 24 小时固定分配，智能重试和自动故障转移

### TOKEN 管理
- 系统 TOKEN 和用户令牌分层管理
- TOKEN 状态监控（正常/过期/禁用）
- 支持用户提交共享 TOKEN 和自定义代理地址

### 外部渠道
- 支持接入 OpenAI、Anthropic、Google 等第三方 AI 模型服务
- 统一通过网关转发，支持模型映射和权重配置
- 渠道健康检查和自动故障切换

### 用户系统
- 用户注册/登录，邀请码机制
- 管理员可手动分配额度、封禁/解封用户

### 监控统计
- 实时请求监控和成功率统计
- TOKEN 使用排行和趋势分析
- 系统公告和插件通知管理

## 📄 License

[MIT](LICENSE)
