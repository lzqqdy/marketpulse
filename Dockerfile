# syntax=docker/dockerfile:1
# 微信云托管：仓库根目录 Dockerfile，控制台监听端口填 80。

# --- frontend ---
FROM node:22-alpine AS web
WORKDIR /src/web
ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- backend ---
FROM golang:1.22-alpine AS api
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w" -o /out/marketd ./cmd/marketd

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
  && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
  && echo Asia/Shanghai > /etc/timezone \
  && mkdir -p /app/log /app/config /app/web/dist /app/data/uploads
WORKDIR /app

COPY --from=api /out/marketd /app/marketd
COPY --from=web /src/web/dist /app/web/dist
COPY config/config.wxcloudrun.yaml /app/config/config.yaml

ENV TZ=Asia/Shanghai
ENV PORT=80
EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
  CMD wget -qO- http://127.0.0.1:80/health >/dev/null || exit 1

ENTRYPOINT ["/app/marketd"]
CMD ["-config", "/app/config/config.yaml"]
