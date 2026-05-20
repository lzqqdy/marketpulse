#!/usr/bin/env bash
# 一键构建并部署到远程 Git 仓库目录（同步源码 + 产物，可选服务器端 git commit）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CFG="${DEPLOY_CFG:-${ROOT}/deploy/deploy.local.yaml}"
EXCLUDES="${ROOT}/deploy/rsync-excludes.txt"

# 可选：make ship SHIP_GIT_COMMIT=1  或  SHIP_GIT_MSG="fix: xxx"
SHIP_GIT_COMMIT="${SHIP_GIT_COMMIT:-}"
SHIP_GIT_MSG="${SHIP_GIT_MSG:-}"
SHIP_NO_SOURCE="${SHIP_NO_SOURCE:-}"

if [[ ! -f "${CFG}" ]]; then
  echo "缺少 ${CFG}"
  echo "请先: cp deploy/deploy.local.yaml.example deploy/deploy.local.yaml"
  exit 1
fi

get_yaml() {
  local key="$1" default="${2:-}"
  local line v
  line="$(grep -E "^${key}:" "${CFG}" 2>/dev/null | head -1 || true)"
  if [[ -z "${line}" ]]; then
    echo "${default}"
    return
  fi
  v="${line#*:}"
  v="${v#"${v%%[![:space:]]*}"}"
  v="${v%"${v##*[![:space:]]}"}"
  v="${v%\"}"
  v="${v#\"}"
  v="${v%\'}"
  v="${v#\'}"
  echo "${v}"
}

yaml_bool() {
  local v
  v="$(echo "$1" | tr '[:upper:]' '[:lower:]')"
  case "${v}" in
    1 | true | yes | on) return 0 ;;
    *) return 1 ;;
  esac
}

SSH_HOST="$(get_yaml ssh_host)"
SSH_PORT="$(get_yaml ssh_port 22)"
SSH_USER="$(get_yaml ssh_user root)"
SSH_KEY="$(get_yaml ssh_key)"
REMOTE_DIR="$(get_yaml remote_dir /home/lzqqdy/github/marketpulse)"
APP_ADDR="$(get_yaml app_addr "0.0.0.0:8080")"
GOOS="$(get_yaml goos linux)"
GOARCH="$(get_yaml goarch amd64)"
SYNC_SOURCE_CFG="$(get_yaml sync_source true)"
SYNC_CONFIG_CFG="$(get_yaml sync_config false)"
GIT_AUTO_COMMIT_CFG="$(get_yaml git_auto_commit false)"
GIT_COMMIT_MSG_CFG="$(get_yaml git_commit_message "")"

if [[ -z "${SSH_HOST}" || "${SSH_HOST}" == "1.2.3.4" ]]; then
  echo "请在 ${CFG} 中设置真实的 ssh_host"
  exit 1
fi

DO_GIT_COMMIT=0
if [[ "${SHIP_GIT_COMMIT}" == "1" ]] || yaml_bool "${GIT_AUTO_COMMIT_CFG}"; then
  DO_GIT_COMMIT=1
fi

DO_SYNC_SOURCE=1
if [[ "${SHIP_NO_SOURCE}" == "1" ]] || ! yaml_bool "${SYNC_SOURCE_CFG}"; then
  DO_SYNC_SOURCE=0
fi

DO_SYNC_CONFIG=0
if [[ "${SHIP_SYNC_CONFIG:-}" == "1" ]] || yaml_bool "${SYNC_CONFIG_CFG}"; then
  DO_SYNC_CONFIG=1
fi

GIT_MSG="${SHIP_GIT_MSG:-${GIT_COMMIT_MSG_CFG}}"
if [[ -z "${GIT_MSG}" ]]; then
  GIT_MSG="deploy: $(date '+%Y-%m-%d %H:%M:%S')"
fi

SSH_OPTS=(-p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new)
RSYNC_SSH="ssh ${SSH_OPTS[*]}"
if [[ -n "${SSH_KEY}" ]]; then
  SSH_OPTS+=(-i "${SSH_KEY/#\~/$HOME}")
  RSYNC_SSH="ssh ${SSH_OPTS[*]}"
fi
SSH_TARGET="${SSH_USER}@${SSH_HOST}"

echo "==> 构建前端"
cd "${ROOT}/web"
if [[ -f package-lock.json ]]; then
  npm ci
else
  npm install
fi
npm run build
cd "${ROOT}"

echo "==> 交叉编译后端 ${GOOS}/${GOARCH}"
mkdir -p bin
GOOS="${GOOS}" GOARCH="${GOARCH}" CGO_ENABLED=0 go build -buildvcs=false -o bin/marketd ./cmd/marketd

echo "==> 生成远程 config"
TMP_CFG="$(mktemp)"
trap 'rm -f "${TMP_CFG}"' EXIT
sed "s|0.0.0.0:8080|${APP_ADDR}|g" "${ROOT}/deploy/config.remote.yaml.example" >"${TMP_CFG}"

echo "==> 目标 ${SSH_TARGET}:${REMOTE_DIR}"
ssh "${SSH_OPTS[@]}" "${SSH_TARGET}" "mkdir -p '${REMOTE_DIR}/'{bin,config,web,log,scripts,deploy}"

if [[ "${DO_SYNC_SOURCE}" == "1" ]]; then
  echo "==> 同步源码（保留服务器 .git，排除 node_modules/bin/dist 等）"
  rsync -avz \
    --exclude-from="${EXCLUDES}" \
    -e "${RSYNC_SSH}" \
    "${ROOT}/" "${SSH_TARGET}:${REMOTE_DIR}/"
else
  echo "==> 跳过源码同步（SHIP_NO_SOURCE=1 或 sync_source: false）"
fi

echo "==> 同步运行产物"
rsync -azc --progress -e "${RSYNC_SSH}" bin/marketd "${SSH_TARGET}:${REMOTE_DIR}/bin/"
rsync -avz --delete -e "${RSYNC_SSH}" web/dist/ "${SSH_TARGET}:${REMOTE_DIR}/web/"
if [[ "${DO_SYNC_CONFIG}" == "1" ]]; then
  echo "==> 覆盖远程 config/config.yaml（SHIP_SYNC_CONFIG=1 或 sync_config: true）"
  rsync -avz -e "${RSYNC_SSH}" "${TMP_CFG}" "${SSH_TARGET}:${REMOTE_DIR}/config/config.yaml"
else
  echo "==> 保留远程 config/config.yaml（不存在时初始化）"
  rsync -avz -e "${RSYNC_SSH}" --ignore-existing "${TMP_CFG}" "${SSH_TARGET}:${REMOTE_DIR}/config/config.yaml"
fi
rsync -avz -e "${RSYNC_SSH}" deploy/remote-restart.sh "${SSH_TARGET}:${REMOTE_DIR}/scripts/restart.sh"
rsync -avz -e "${RSYNC_SSH}" deploy/remote-git-commit.sh "${SSH_TARGET}:${REMOTE_DIR}/deploy/remote-git-commit.sh"

echo "==> 重启服务"
ssh "${SSH_OPTS[@]}" "${SSH_TARGET}" \
  "chmod +x '${REMOTE_DIR}/scripts/restart.sh' '${REMOTE_DIR}/deploy/remote-git-commit.sh' && '${REMOTE_DIR}/scripts/restart.sh'"

if [[ "${DO_GIT_COMMIT}" == "1" ]]; then
  echo "==> 服务器 Git 提交"
  ssh "${SSH_OPTS[@]}" "${SSH_TARGET}" \
    "cd '${REMOTE_DIR}' && bash deploy/remote-git-commit.sh $(printf '%q' "${GIT_MSG}")"
else
  echo ""
  echo "提示: 未自动 git commit。可选手动："
  echo "  ssh -p ${SSH_PORT} ${SSH_TARGET} 'cd ${REMOTE_DIR} && git status && git add -A && git commit'"
  echo "  或下次: make ship SHIP_GIT_COMMIT=1"
fi

APP_PORT="$(get_yaml app_port)"
if [[ -z "${APP_PORT}" ]]; then
  APP_PORT="${APP_ADDR##*:}"
fi
echo ""
echo "部署完成: http://${SSH_HOST}:${APP_PORT}/"
echo "健康检查: curl -s http://${SSH_HOST}:${APP_PORT}/healthz"
echo "仓库目录: ${REMOTE_DIR}"
