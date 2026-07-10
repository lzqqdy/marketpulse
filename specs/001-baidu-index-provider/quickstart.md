# Quickstart: 百度指数数据源

## 验证 Provider 切换

```bash
cd marketpulse
make dev-api

# 指数快照
curl -s http://127.0.0.1:8080/api/v1/market/snapshot | jq '.indices[:3]'

# Provider 健康
curl -s http://127.0.0.1:8080/api/v1/market/providers/status | jq '.providers[] | select(.category=="index")'

# 指数 K 线
curl -s 'http://127.0.0.1:8080/api/v1/market/index/klines?id=sh000001&interval=1d&limit=5' | jq '{source, count: (.candles|length)}'
```

期望：`baidu_index` role=primary 且 current_used=true；K 线 source 含 `baidu`。

## 模拟降级

```yaml
# config/config.yaml 临时测试
ingest:
  baidu:
    enabled: false
  equity:
    providers:
      - tencent
      - eastmoney
```

重启后应回到腾讯+东财，与切换前一致。

## 运行测试

```bash
go test -buildvcs=false ./internal/marketdata/ingest/baidu/...
go test -buildvcs=false ./internal/marketdata/ingest/equity/...
```
