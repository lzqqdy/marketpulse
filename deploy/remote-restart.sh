#!/usr/bin/env bash
# 在服务器上执行：重启 marketd（无 systemd 时的简易方案）
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
PIDFILE="${ROOT}/log/marketd.pid"
BIN="${ROOT}/bin/marketd"
CONFIG="${ROOT}/config/config.yaml"
mkdir -p "${ROOT}/log"

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

PORT="$(listen_port)"

stop_all() {
  if [[ -f "${PIDFILE}" ]]; then
    local old
    old="$(cat "${PIDFILE}")"
    if kill -0 "${old}" 2>/dev/null; then
      kill "${old}" 2>/dev/null || true
      sleep 1
      kill -9 "${old}" 2>/dev/null || true
    fi
  fi
  pkill -f "${BIN}" 2>/dev/null || true
  sleep 1
  if command -v fuser >/dev/null 2>&1; then
    fuser -k "${PORT}/tcp" 2>/dev/null || true
  elif command -v lsof >/dev/null 2>&1; then
    local pids
    pids="$(lsof -ti :"${PORT}" 2>/dev/null || true)"
    [[ -n "${pids}" ]] && kill -9 ${pids} 2>/dev/null || true
  fi
  sleep 1
}

stop_all

if command -v ss >/dev/null 2>&1 && ss -ltn | grep -q ":${PORT} "; then
  echo "ERROR: port ${PORT} still in use, abort"
  ss -ltnp | grep ":${PORT} " || true
  exit 1
fi

nohup "${BIN}" -config "${CONFIG}" >/dev/null 2>&1 &
echo $! >"${PIDFILE}"
sleep 2
if ! kill -0 "$(cat "${PIDFILE}")" 2>/dev/null; then
  echo "ERROR: marketd exited immediately, see ${ROOT}/log/YYYY-MM-DD/error.log"
  find "${ROOT}/log" -maxdepth 2 -type f -name 'error.log' 2>/dev/null | sort | tail -1 | xargs -r tail -20 || true
  exit 1
fi
echo "marketd started pid=$(cat "${PIDFILE}") port=${PORT}"
