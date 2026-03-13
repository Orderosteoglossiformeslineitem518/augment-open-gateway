# 多阶段构建：前端构建阶段
FROM node:18-alpine AS frontend-builder

# 设置工作目录
WORKDIR /app/web

# 复制前端依赖文件
COPY web/package*.json ./

# 清除npm缓存并安装前端依赖（包括开发依赖，构建需要）
RUN npm cache clean --force && npm ci --no-cache

# 复制前端源代码
COPY web/ .

# 构建前端
RUN npm run build

# 多阶段构建：后端构建阶段
FROM golang:1.23-alpine AS backend-builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git

# 复制Go模块文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制后端源代码
COPY . .

# 构建后端应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o augment-gateway cmd/main.go

# 生产阶段：创建最终镜像
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 设置静态文件路径环境变量
ENV FRONTEND_STATIC_PATH=/app/web/dist

# 创建非root用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 从构建阶段复制文件
COPY --from=backend-builder /app/augment-gateway .
COPY --from=frontend-builder /app/web/dist ./web/dist

# 创建必要的目录
RUN mkdir -p /app/logs && \
    chown -R appuser:appgroup /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 28080



# 启动应用
CMD ["./augment-gateway"]