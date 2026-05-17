#!/usr/bin/env bash
# Start local API and web dev server, and clean them up reliably on Ctrl+C.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO="${GO:-go}"
API_PID=""
WEB_PID=""

kill_port() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    local pids
    pids="$(lsof -ti :"${port}" 2>/dev/null || true)"
    if [[ -n "${pids}" ]]; then
      echo "==> 清理占用端口 ${port}: ${pids}"
      kill ${pids} 2>/dev/null || true
      sleep 0.5
      pids="$(lsof -ti :"${port}" 2>/dev/null || true)"
      [[ -n "${pids}" ]] && kill -9 ${pids} 2>/dev/null || true
    fi
  fi
}

cleanup() {
  trap - INT TERM EXIT
  echo ""
  echo "==> 停止本地开发进程"
  [[ -n "${WEB_PID}" ]] && kill "${WEB_PID}" 2>/dev/null || true
  [[ -n "${API_PID}" ]] && kill "${API_PID}" 2>/dev/null || true
  sleep 0.8
  [[ -n "${WEB_PID}" ]] && kill -9 "${WEB_PID}" 2>/dev/null || true
  [[ -n "${API_PID}" ]] && kill -9 "${API_PID}" 2>/dev/null || true
  kill_port 8080
  kill_port 5173
}

trap cleanup INT TERM EXIT

kill_port 8080
kill_port 5173

echo "==> 启动后端 http://localhost:8080"
cd "${ROOT}"
"${GO}" run -buildvcs=false ./cmd/marketd -config config/config.yaml &
API_PID="$!"

echo "==> 启动前端 http://localhost:5173"
cd "${ROOT}/web"
npm run dev &
WEB_PID="$!"

while true; do
  if ! kill -0 "${API_PID}" 2>/dev/null; then
    wait "${API_PID}" 2>/dev/null || true
    exit 1
  fi
  if ! kill -0 "${WEB_PID}" 2>/dev/null; then
    wait "${WEB_PID}" 2>/dev/null || true
    exit 1
  fi
  sleep 1
done
