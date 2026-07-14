# syntax=docker/dockerfile:1

# --- frontend ---
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- backend ---
FROM golang:1.22-alpine AS api
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o /out/marketd ./cmd/marketd

# --- runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
  && adduser -D -u 1000 marketpulse \
  && mkdir -p /app/log /app/config /app/web/dist \
  && chown -R marketpulse:marketpulse /app
WORKDIR /app

COPY --from=api /out/marketd /app/marketd
COPY --from=web /src/web/dist /app/web/dist
COPY config/config.docker.yaml /app/config/config.yaml

USER marketpulse
ENV TZ=Asia/Shanghai
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/marketd"]
CMD ["-config", "/app/config/config.yaml"]
