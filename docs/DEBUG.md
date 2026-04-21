# 调试文档

## 健康检查

```bash
curl -s http://127.0.0.1:8080/health | jq .
```

## 热加载验证

1. 修改 `Vocabulary/` 下词库。
2. 调用：

```bash
curl -X POST http://127.0.0.1:8080/reload -H 'X-API-Key: your-key'
```

## 常见问题

- `401 invalid api key`：检查 `API_KEY` 与请求头 `X-API-Key`。
- `429 rate limit exceeded`：触发自适应限流，降低并发或提高 `BASE_RPS/MAX_RPS`。
- `no lexicon words loaded`：确认 `LEXICON_DIR` 指向有效词库目录。
