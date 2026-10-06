# AI ENV 浏览器模式镜像：适合 NAS / Linux 服务器。
#   docker build -t aienv .
#   docker run -d --name aienv -p 3430:3430 -e AIENV_WEB_PASSWORD=换成你的口令 -v aienv-data:/data aienv
# 打开 http://<主机>:3430 登录。网关默认只听容器内的 127.0.0.1；要给其它机器用，
# 在“路由”页打开“局域网共享”并创建网关密钥，再加 -p 18790:18790。

FROM node:22-bookworm-slim AS web
WORKDIR /src/frontend
RUN corepack enable
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm build

FROM golang:1.23-bookworm AS app
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/frontend/dist ./frontend/dist
# 不带 Wails 的桌面构建标签，也不需要 CGO：只用其中的命令行与浏览器模式
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/aienv .

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=app /out/aienv /usr/local/bin/aienv
# 配置、日志与各工具的配置文件都在 /data（容器里的家目录）
ENV HOME=/data \
    AIENV_WEB_ADDR=0.0.0.0:3430
VOLUME /data
EXPOSE 3430 18790
ENTRYPOINT ["aienv"]
CMD ["web"]
