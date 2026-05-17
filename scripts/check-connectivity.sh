#!/usr/bin/env bash
# 在目标 VPS 或本机检查外网数据源可达性
set -euo pipefail

echo "== Binance =="
curl -sf --max-time 10 https://api.binance.com/api/v3/ping && echo " OK"

echo "== Frankfurter USD/CNY =="
curl -sf --max-time 10 "https://api.frankfurter.app/latest?from=USD&to=CNY" | head -c 120 && echo "..."

echo "== Fear & Greed =="
curl -sf --max-time 10 "https://api.alternative.me/fng/?limit=1" | head -c 120 && echo "..."

echo "== CoinGecko global =="
curl -sf --max-time 15 "https://api.coingecko.com/api/v3/global" | head -c 120 && echo "..."

echo "== OKX public =="
curl -sf --max-time 10 "https://www.okx.com/api/v5/market/ticker?instId=BTC-USDT" | head -c 120 && echo "..."

echo "All checks passed."
