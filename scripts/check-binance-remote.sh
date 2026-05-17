#!/usr/bin/env bash
# 从本机 SSH 到 VPS 验证 Binance（REST + TLS + healthz + WS 探针）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CFG="${DEPLOY_CFG:-${ROOT}/deploy/deploy.local.yaml}"

if [[ ! -f "${CFG}" ]]; then
  echo "缺少 ${CFG}"
  exit 1
fi

get_yaml() {
  local key="$1" default="${2:-}"
  local line v
  line="$(grep -E "^${key}:" "${CFG}" 2>/dev/null | head -1 || true)"
  [[ -z "${line}" ]] && { echo "${default}"; return; }
  v="${line#*:}"
  v="${v#"${v%%[![:space:]]*}"}"
  v="${v%"${v##*[![:space:]]}"}"
  v="${v%\"}"; v="${v#\"}"; v="${v%\'}"; v="${v#\'}"
  echo "${v}"
}

SSH_HOST="$(get_yaml ssh_host)"
SSH_PORT="$(get_yaml ssh_port 22)"
SSH_USER="$(get_yaml ssh_user root)"
SSH_KEY="$(get_yaml ssh_key)"
APP_PORT="$(get_yaml app_port 8080)"
GOOS="$(get_yaml goos linux)"
GOARCH="$(get_yaml goarch amd64)"

SSH_OPTS=(-p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new)
[[ -n "${SSH_KEY}" ]] && SSH_OPTS+=(-i "${SSH_KEY/#\~/$HOME}")

echo ">>> 编译 Linux WS 探针..."
mkdir -p "${ROOT}/bin"
GOOS="${GOOS}" GOARCH="${GOARCH}" CGO_ENABLED=0 \
  go build -buildvcs=false -o "${ROOT}/bin/binance-ws-check" ./scripts/binance_ws_check

echo ">>> SSH ${SSH_USER}@${SSH_HOST}:${SSH_PORT}"
ssh "${SSH_OPTS[@]}" "${SSH_USER}@${SSH_HOST}" "mkdir -p /tmp/marketpulse-check"
scp -P "${SSH_PORT}" ${SSH_KEY:+-i "${SSH_KEY/#\~/$HOME}"} \
  "${ROOT}/scripts/check-binance.sh" \
  "${ROOT}/bin/binance-ws-check" \
  "${SSH_USER}@${SSH_HOST}:/tmp/marketpulse-check/"

ssh "${SSH_OPTS[@]}" "${SSH_USER}@${SSH_HOST}" \
  "chmod +x /tmp/marketpulse-check/check-binance.sh /tmp/marketpulse-check/binance-ws-check && \
   ROOT=/tmp/marketpulse-check APP_PORT=${APP_PORT} /tmp/marketpulse-check/check-binance.sh"
