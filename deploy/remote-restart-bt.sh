#!/usr/bin/env bash
# 在服务器上执行：通过宝塔进程守护管理器 / Supervisor 重启 marketd。
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
CONFIG="${ROOT}/config/config.yaml"
SUPERVISORCTL="${SUPERVISORCTL:-/www/server/panel/pyenv/bin/supervisorctl}"
TARGET="${MARKETPULSE_SUPERVISOR_TARGET:-}"

listen_port() {
  if [[ -f "${CONFIG}" ]]; then
    local addr
    addr="$(grep -E '^[[:space:]]*addr:' "${CONFIG}" | head -1 | sed -E 's/.*addr:[[:space:]]*"?([^"]+)"?.*/\1/')"
    if [[ "${addr}" == *:* ]]; then
      echo "${addr##*:}"
      return
    fi
  fi
  echo "8080"
}

if [[ ! -x "${SUPERVISORCTL}" ]]; then
  echo "ERROR: supervisorctl not found: ${SUPERVISORCTL}"
  echo "请确认已安装宝塔进程守护管理器，或改用 make ship"
  exit 1
fi

if [[ -z "${TARGET}" ]]; then
  TARGET="$("${SUPERVISORCTL}" status | awk '$1 ~ /^marketpulse(:|_|$)/ { print $1; exit }')"
fi

if [[ -z "${TARGET}" ]]; then
  echo "ERROR: 未找到 marketpulse 守护进程"
  echo "当前 supervisor 状态："
  "${SUPERVISORCTL}" status || true
  echo ""
  echo "如果宝塔里名称不是 marketpulse，请手动设置："
  echo "  MARKETPULSE_SUPERVISOR_TARGET=你的进程名 make ship-bt"
  exit 1
fi

echo "==> supervisor restart ${TARGET}"
"${SUPERVISORCTL}" restart "${TARGET}"
sleep 2

echo "==> supervisor status ${TARGET}"
"${SUPERVISORCTL}" status "${TARGET}"

if command -v curl >/dev/null 2>&1; then
  echo "==> health check"
  curl -fsS --max-time 5 "http://127.0.0.1:$(listen_port)/healthz" >/dev/null
fi

echo "marketpulse restarted by BT supervisor in ${ROOT}"
