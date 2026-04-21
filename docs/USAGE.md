# 使用与请求示例

## 同步检测

```bash
curl -X POST http://127.0.0.1:8080/detect \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: your-key' \
  -d '{"text":"这段文本含有敏感词","replace":true,"symbol":"*"}'
```

## 异步检测

```bash
curl -X POST http://127.0.0.1:8080/detect/async \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: your-key' \
  -d '{"text":"这段文本含有敏感词","replace":true}'
```

查询结果：

```bash
curl -s 'http://127.0.0.1:8080/detect/result?id=<job_id>' -H 'X-API-Key: your-key'
```

## 流式检测

```bash
cat input.txt | curl -X POST http://127.0.0.1:8080/detect/stream \
  -H 'X-API-Key: your-key' \
  --data-binary @-
```
