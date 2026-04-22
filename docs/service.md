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
| `BLOCK_SCORE_THRESHOLD` | `0.2` | 默认句子拦截阈值（0~1） |
| `ENABLE_OCR` | `false` | 是否启用 OCR 扩展（手动开启） |
| `OCR_USE_GPU` | `false` | OCR 是否启用 GPU |
| `OCR_GPU_DEVICE` | `0` | GPU 设备 ID（多 GPU 场景可指定） |
| `OCR_AUTO_DOWNLOAD` | `true` | 启动时是否自动下载/更新 OCR 仓库 |
| `OCR_REPO_URL` | `https://github.com/DayBreak-u/chineseocr_lite.git` | OCR 仓库地址 |
| `OCR_MODEL_DIR` | `./ThirdPartyCompatibleFormats/chineseocr_lite` | OCR 仓库落地目录 |
| `OCR_PYTHON_BIN` | `python3` | OCR Python 可执行文件 |
| `OCR_BRIDGE_SCRIPT` | `./internal/ocr/bridge.py` | OCR Python bridge 脚本 |
| `OCR_TIMEOUT_SEC` | `30` | OCR 单次识别超时（秒） |

## OCR 启用示例

```bash
ENABLE_OCR=true \
OCR_USE_GPU=false \
OCR_GPU_DEVICE=0 \
OCR_AUTO_DOWNLOAD=true \
OCR_MODEL_DIR=./ThirdPartyCompatibleFormats/chineseocr_lite \
OCR_PYTHON_BIN=python3 \
OCR_BRIDGE_SCRIPT=./internal/ocr/bridge.py \
./sensitive-server
```

## 接口

### `GET /health`

返回：服务状态、词条数量、OCR 是否启用。

### `GET /contains?text=...`

返回是否包含敏感词。

### `POST /detect`

文本检测并替换，返回拦截权重。

请求体：

```json
{
  "text": "这是坏词",
  "replace_symbol": "#",
  "block_threshold": 0.15
}
```

响应体示例：

```json
{
  "contains": true,
  "replaced": "这是##",
  "count": 1,
  "score": 0.5,
  "blocked": true,
  "threshold": 0.15
}
```

### `POST /detect/image`

图片 OCR + 敏感词检测（仅 `ENABLE_OCR=true` 时可用）。

支持三种输入：

1) JSON base64

```json
{
  "image_base64": "<base64>",
  "replace_symbol": "#",
  "block_threshold": 0.2
}
```

2) `multipart/form-data`
- 文件字段：`image`
- 可选字段：`replace_symbol`、`block_threshold`

3) 原始二进制 POST body（`image/*` 或 `application/octet-stream`）

响应体示例：

```json
{
  "contains": true,
  "replaced": "****",
  "count": 1,
  "score": 0.33,
  "blocked": true,
  "threshold": 0.2,
  "ocr_text": "识别出的文本"
}
```

### `POST /detect/async`

异步提交检测任务，参数与 `/detect` 相同。

### `GET /detect/async/result?job_id=...`

查询异步检测结果。

### `POST /sanitize-stream`

按行流式替换。

### `POST /reload`

热加载词库。

## 请求示例

### 检测并替换

```bash
curl -s http://127.0.0.1:8080/detect \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","replace_symbol":"#","block_threshold":0.2}'
```

### 图片检测（base64）

```bash
curl -s http://127.0.0.1:8080/detect/image \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"image_base64":"<base64>","block_threshold":0.2}'
```

### 图片检测（POST 文件）

```bash
curl -s http://127.0.0.1:8080/detect/image \
  -H 'X-API-Key: demo-key' \
  -F 'image=@/path/to/test.png' \
  -F 'replace_symbol=#' \
  -F 'block_threshold=0.2'
```

### 异步检测

```bash
curl -s http://127.0.0.1:8080/detect/async \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","block_threshold":0.2}'

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
