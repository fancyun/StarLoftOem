# ========================
# Stage 1: Builder
# ========================
FROM golang:1.21-alpine AS builder

# 安装必要的编译工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# Go 模块代理（国内镜像，避免访问 proxy.golang.org 超时）
ENV GOPROXY=https://mirrors.tencent.com/go/,https://goproxy.cn,direct

# 复制源代码
COPY . .

# 按 go.mod 声明下载依赖（比 go mod tidy 更快且不改写 go.mod/go.sum）
RUN go mod download

# 编译应用（禁用CGO以获得静态二进制）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o oem_server \
    ./cmd/server

# ========================
# Stage 2: Runtime
# ========================
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非root用户
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

# 设置工作目录
WORKDIR /app

# 从builder复制二进制文件
COPY --from=builder /app/oem_server .

# 创建日志、上传与认证媒体目录并修改所有者为 app 用户
# 关键：镜像中若不存在 /app/logs、/app/uploads、/app/media，Docker 挂载命名卷时会以 root 属主创建该目录，
# 导致以非 root 用户（app）运行时无法写入日志/图片（permission denied）。
RUN mkdir -p /app/logs /app/uploads /app/media && chown -R app:app /app

# 切换到非root用户
USER app

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/console/config || exit 1

# 启动应用
CMD ["./oem_server"]
