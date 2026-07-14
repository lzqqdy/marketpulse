# MarketPulse — 统一构建入口（Vibe Coding 以 make help 为准）
.PHONY: help dev dev-api dev-api-log dev-web web api build deploy deploy-web deploy-api ship ship-bt ship-bt-commit ship-commit restart-remote check check-binance check-binance-remote test setup-config setup-deploy setup-log docker-build docker-up docker-up-db docker-down docker-logs

DEPLOY_MODE ?= nginx
DEPLOY_HOST ?=
DEPLOY_WEB_DIR ?= /var/www/marketpulse
DEPLOY_API_DIR ?= /opt/marketpulse
GO ?= go
BIN := bin/marketd

help:
	@echo "MarketPulse Makefile"
	@echo ""
	@echo "  开发"
	@echo "    make dev              同时启动后端 (:8080) 和前端 (:5173)"
	@echo "    make dev-api          启动后端 (:8080)"
	@echo "    make dev-api-log      启动后端并写入 log/local-api.log"
	@echo "    make dev-web          启动前端 Vite (:5173)"
	@echo ""
	@echo "  构建"
	@echo "    make web              构建前端 → web/dist"
	@echo "    make api              构建后端 → bin/marketd"
	@echo "    make build            web + api"
	@echo ""
	@echo "  Docker"
	@echo "    make docker-build     构建 marketpulse 镜像"
	@echo "    make docker-up        启动行情容器（默认）"
	@echo "    make docker-up-db     启动行情 + MySQL + Redis"
	@echo "    make docker-down      停止并移除容器"
	@echo "    make docker-logs      跟随 marketd 日志"
	@echo ""
	@echo "  部署"
	@echo "    make setup-deploy     复制 deploy.local.yaml 模板"
	@echo "    make ship             一键部署（同步源码到 remote_dir + 重启）"
	@echo "    make ship-bt          一键部署，并通过宝塔进程守护重启"
	@echo "    make ship-bt-commit   同 ship-bt，并在服务器 git commit"
	@echo "    make ship-commit      同上，并在服务器 git commit"
	@echo "    make restart-remote   只重启线上后端（不构建/不同步/不覆盖配置）"
	@echo "      SHIP_GIT_COMMIT=1   可选：本次部署后自动提交"
	@echo "      SHIP_GIT_MSG=...    可选：提交说明"
	@echo "      SHIP_NO_SOURCE=1    可选：不同步源码，只更新 bin/web"
	@echo "    make deploy-web       Nginx 模式：仅同步静态（需 DEPLOY_HOST）"
	@echo "    make deploy-api       Nginx 模式：仅同步二进制（需 DEPLOY_HOST）"
	@echo "    make deploy           Nginx 模式：web + api"
	@echo ""
	@echo "  其他"
	@echo "    make setup-config     复制 config.example.yaml（若不存在）"
	@echo "    make test             运行 Go 单元测试"
	@echo "    make check            外网数据源连通性检查（本机）"
	@echo "    make check-binance    本机 Binance REST+WS 检查"
	@echo "    make check-binance-remote  SSH 到 deploy.local 服务器检查"
	@echo ""
	@echo "  变量: DEPLOY_HOST=...  DEPLOY_CFG=...  SHIP_GIT_COMMIT=1  SHIP_GIT_MSG=..."
	@echo "  Docker 说明: deploy/docker.md"

# --- 开发 ---
setup-config:
	@test -f config/config.yaml || cp config/config.example.yaml config/config.yaml

setup-log:
	@mkdir -p log

setup-deploy:
	@test -f deploy/deploy.local.yaml || cp deploy/deploy.local.yaml.example deploy/deploy.local.yaml
	@echo "已创建 deploy/deploy.local.yaml，请填写 ssh_host 后执行 make ship"

dev-api: setup-config
	$(GO) run -buildvcs=false ./cmd/marketd -config config/config.yaml

dev-api-log: setup-config setup-log
	$(GO) run -buildvcs=false ./cmd/marketd -config config/config.yaml 2>&1 | tee log/local-api.log

test:
	$(GO) test -buildvcs=false ./...

dev-web:
	cd web && npm run dev

dev: setup-config
	@GO="$(GO)" scripts/dev-local.sh

# --- 构建 ---
web:
	cd web && npm ci && npm run build

api:
	@mkdir -p bin
	$(GO) build -buildvcs=false -o $(BIN) ./cmd/marketd

build: web api

# --- 部署 ---
deploy-web: web
ifndef DEPLOY_HOST
	$(error 请设置 DEPLOY_HOST，例如: make deploy-web DEPLOY_HOST=user@1.2.3.4)
endif
	rsync -avz --delete web/dist/ $(DEPLOY_HOST):$(DEPLOY_WEB_DIR)/

deploy-api: api
ifndef DEPLOY_HOST
	$(error 请设置 DEPLOY_HOST)
endif
	ssh $(DEPLOY_HOST) 'mkdir -p $(DEPLOY_API_DIR)/bin $(DEPLOY_API_DIR)/config'
	scp $(BIN) $(DEPLOY_HOST):$(DEPLOY_API_DIR)/bin/marketd
	ssh $(DEPLOY_HOST) 'test -f $(DEPLOY_API_DIR)/config/config.yaml || cat > $(DEPLOY_API_DIR)/config/config.yaml' < config/config.example.yaml
	ssh $(DEPLOY_HOST) 'sudo systemctl restart marketpulse || true'

deploy: deploy-web deploy-api

ship: setup-deploy
	@chmod +x scripts/deploy-remote.sh deploy/remote-restart.sh deploy/remote-git-commit.sh
	@./scripts/deploy-remote.sh

ship-bt: setup-deploy
	@chmod +x scripts/deploy-remote.sh deploy/remote-restart-bt.sh deploy/remote-git-commit.sh
	@DEPLOY_RESTART_SCRIPT=deploy/remote-restart-bt.sh ./scripts/deploy-remote.sh

ship-bt-commit: setup-deploy
	@chmod +x scripts/deploy-remote.sh deploy/remote-restart-bt.sh deploy/remote-git-commit.sh
	@DEPLOY_RESTART_SCRIPT=deploy/remote-restart-bt.sh SHIP_GIT_COMMIT=1 ./scripts/deploy-remote.sh

ship-commit: setup-deploy
	@chmod +x scripts/deploy-remote.sh deploy/remote-restart.sh deploy/remote-git-commit.sh
	@SHIP_GIT_COMMIT=1 ./scripts/deploy-remote.sh

restart-remote: setup-deploy
	@chmod +x scripts/restart-remote.sh
	@./scripts/restart-remote.sh

check:
	@./scripts/check-connectivity.sh

check-binance:
	@chmod +x scripts/check-binance.sh
	@./scripts/check-binance.sh

check-binance-remote:
	@chmod +x scripts/check-binance-remote.sh
	@./scripts/check-binance-remote.sh

# --- Docker ---
docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-up-db:
	@test -f .env || cp .env.example .env
	MYSQL_ENABLED=true REDIS_ENABLED=true docker compose --profile db up -d --build

docker-down:
	docker compose --profile db down

docker-logs:
	docker compose logs -f marketd
