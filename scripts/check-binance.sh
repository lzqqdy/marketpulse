#!/usr/bin/env bash
# 在 VPS（如香港）上验证 Binance REST + WebSocket 与本地 marketd 状态
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT}"
WS_CHECK_BIN="${SCRIPT_DIR}/binance-ws-check"
[[ -x "${ROOT}/bin/binance-ws-check" ]] && WS_CHECK_BIN="${ROOT}/bin/binance-ws-check"

APP_PORT="${APP_PORT:-8080}"
WS_BASE="${MARKETPULSE_BINANCE_WS_BASE:-wss://stream.binance.com:9443/stream}"
STREAM_URL="${WS_BASE}?streams=btcusdt@miniTicker/ethusdt@miniTicker"

echo "=========================================="
echo " MarketPulse Binance 连通性检查"
echo " $(date -Iseconds 2>/dev/null || date)"
echo "=========================================="
echo ""

echo "== 1) DNS =="
for host in stream.binance.com api.binance.com data-stream.binance.vision; do
  if command -v dig >/dev/null 2>&1; then
    ip="$(dig +short "${host}" | head -1)"
    echo "  ${host} -> ${ip:-解析失败}"
  else
    getent hosts "${host}" 2>/dev/null || echo "  ${host} -> (无 dig/getent，跳过)"
  fi
done
echo ""

echo "== 2) REST api.binance.com =="
if curl -sf --max-time 10 https://api.binance.com/api/v3/ping >/dev/null; then
  echo "  OK  ping"
  price="$(curl -sf --max-time 10 "https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT" | sed -n 's/.*"price":"\([^"]*\)".*/\1/p')"
  echo "  BTCUSDT = ${price:-?}"
else
  echo "  FAIL  无法访问 REST（超时或被墙/防火墙）"
fi
echo ""

echo "== 3) TLS 9443 (stream.binance.com) =="
if timeout 8 bash -c 'echo | openssl s_client -connect stream.binance.com:9443 -servername stream.binance.com 2>/dev/null | grep -q "Verify return code"'; then
  echo "  OK  9443 端口 TLS 握手成功"
else
  echo "  FAIL  9443 不可达"
fi
echo ""

echo "== 4) WebSocket miniTicker（与 marketd 相同逻辑，约 15s）=="
echo "  URL: ${STREAM_URL}"
if [[ "${SKIP_GO_WS:-}" == "1" ]]; then
  echo "  SKIP  SKIP_GO_WS=1"
elif [[ -x "${WS_CHECK_BIN}" ]]; then
  if "${WS_CHECK_BIN}" "${STREAM_URL}"; then
    echo "  WS: OK  已收到 ticker"
  else
    echo "  WS: FAIL  见上方错误"
  fi
elif command -v go >/dev/null 2>&1; then
  if go run -buildvcs=false ./scripts/binance_ws_check "${STREAM_URL}"; then
    echo "  WS: OK  已收到 ticker"
  else
    echo "  WS: FAIL  见上方错误"
    echo ""
    echo "  可尝试镜像："
    echo "    MARKETPULSE_BINANCE_WS_BASE=wss://data-stream.binance.vision/stream $0"
  fi
else
  echo "  SKIP  未安装 go，跳过 WS 实测"
fi
echo ""

echo "== 5) 本地 marketd /healthz =="
if curl -sf --max-time 5 "http://127.0.0.1:${APP_PORT}/healthz" >/tmp/marketpulse-healthz.json 2>/dev/null; then
  if command -v python3 >/dev/null 2>&1; then
    python3 - <<'PY'
import json
d=json.load(open("/tmp/marketpulse-healthz.json"))
ing=d.get("ingest",{})
print("  binance_ws     =", ing.get("binance_ws"))
print("  last_quote_ms  =", ing.get("last_quote_ms"))
print("  stream_clients =", ing.get("stream_clients"))
PY
  else
    cat /tmp/marketpulse-healthz.json
  fi
  echo ""
  echo "  最近 Binance 日志："
  if [[ -f /opt/marketpulse/log/marketd.log ]]; then
    grep -i binance /opt/marketpulse/log/marketd.log 2>/dev/null | tail -5 | sed 's/^/    /' || echo "    (无匹配)"
  else
    echo "    /opt/marketpulse/log/marketd.log 不存在"
  fi
else
  echo "  SKIP  http://127.0.0.1:${APP_PORT}/healthz 不可达（进程未启动或端口不对）"
fi
echo ""
echo "=========================================="
echo " 判定："
echo "   REST+WS 都 OK，但 healthz 仍 disconnected → 看 marketd 配置/重启"
echo "   REST OK、WS FAIL → 出站 9443/WSS 被拦，试 vision 镜像"
echo "   都 FAIL → 机器出网或 DNS 问题"
echo "=========================================="
