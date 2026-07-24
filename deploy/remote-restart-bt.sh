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

# supervisorctl status：只要有任一进程非 RUNNING 就会返回非 0（常见为 3）。
# 不能在 pipefail 下直接管道，否则会静默失败。
supervisor_status() {
  "${SUPERVISORCTL}" status 2>/dev/null || true
}

wait_health() {
  local port="$1"
  local i
  for i in 1 2 3 4 5 6 7 8 9 10; do
    if curl -fsS --max-time 3 "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

if [[ ! -x "${SUPERVISORCTL}" ]]; then
  echo "ERROR: supervisorctl not found: ${SUPERVISORCTL}"
  echo "请确认已安装宝塔进程守护管理器，或改用 make ship"
  exit 1
fi

if [[ -z "${TARGET}" ]]; then
  TARGET="$(supervisor_status | awk '$1 ~ /^marketpulse(:|_|$)/ { print $1; exit }')"
fi

if [[ -z "${TARGET}" ]]; then
  echo "ERROR: 未找到 marketpulse 守护进程"
  echo "当前 supervisor 状态："
  supervisor_status || true
  echo ""
  echo "如果宝塔里名称不是 marketpulse，请手动设置："
  echo "  MARKETPULSE_SUPERVISOR_TARGET=你的进程名 make ship-bt"
  exit 1
fi

echo "==> supervisor restart ${TARGET}"
"${SUPERVISORCTL}" restart "${TARGET}"
sleep 1

echo "==> supervisor status ${TARGET}"
# 单进程 status 在 STOPPED 时也可能非 0，仅作展示
"${SUPERVISORCTL}" status "${TARGET}" || true

if command -v curl >/dev/null 2>&1; then
  echo "==> health check"
  if ! wait_health "$(listen_port)"; then
    echo "ERROR: healthz 未在超时内就绪"
    "${SUPERVISORCTL}" status "${TARGET}" || true
    exit 1
  fi
fi

echo "marketpulse restarted by BT supervisor in ${ROOT}"
