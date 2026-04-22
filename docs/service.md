# 敏感词服务文档

## 部署

```bash
go build -o sensitive-server ./cmd/server
API_KEY=demo-key LISTEN_ADDR=:8080 LEXICON_DIR=./Vocabulary ./sensitive-server
```

## 调试

```bash
go test ./...
curl http://127.0.0.1:8080/health
```

## 配置

| 变量 | 默认值 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | 监听地址 |
| `LEXICON_DIR` | `./Vocabulary` | 词库目录（读取全部 `.txt`） |
| `REPLACE_SYMBOL` | `*` | 替换符号 |
| `ENABLE_BOUNDARY` | `true` | 是否启用边界识别（减少误判） |
| `API_KEY` | 空 | 若设置则除 `/health` 外需要 `X-API-Key` |
| `BASE_RPS` | `600` | 自适应限流基础吞吐 |
| `MAX_BODY_BYTES` | `1048576` | 请求体限制 |
| `ASYNC_QUEUE_LENGTH` | `128` | 异步队列长度 |

## 请求示例

### 检测并替换

```bash
curl -s http://127.0.0.1:8080/detect \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","replace_symbol":"#"}'
```

### 异步检测

```bash
curl -s http://127.0.0.1:8080/detect/async \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词"}'

curl -s 'http://127.0.0.1:8080/detect/async/result?job_id=job-1' -H 'X-API-Key: demo-key'
```

### 流式文件处理

```bash
cat input.txt | curl -s http://127.0.0.1:8080/sanitize-stream -H 'X-API-Key: demo-key' --data-binary @-
```

### 热加载

```bash
curl -s -X POST http://127.0.0.1:8080/reload -H 'X-API-Key: demo-key'
```
