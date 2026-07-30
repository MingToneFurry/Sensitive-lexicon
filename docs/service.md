# 敏感词服务文档

## 部署

### 直接编译部署

```bash
# 1. 编译
go build -o sensitive-server ./cmd/server

# 2. 复制配置模板并编辑
cp config.example.json config.json
# 编辑 config.json，至少设置 api_key 和 lexicon_dir

# 3. 启动
./sensitive-server

# 或指定配置文件
./sensitive-server -config /etc/myapp/config.json
```

### Docker 部署

```bash
# 拉取最新官方镜像
docker pull ghcr.io/mingtoneyfurry/sensitive-lexicon:latest

# 运行（使用内置词库）
docker run -d -p 8080:8080 ghcr.io/mingtoneyfurry/sensitive-lexicon:latest

# 挂载外部词库目录
docker run -d -p 8080:8080 \
  -v /path/to/Vocabulary:/app/Vocabulary \
  ghcr.io/mingtoneyfurry/sensitive-lexicon:latest

# 挂载配置文件
docker run -d -p 8080:8080 \
  -v /path/to/config.json:/app/config.json \
  ghcr.io/mingtoneyfurry/sensitive-lexicon:latest \
  ./sensitive-server -config /app/config.json

# 通过环境变量配置
docker run -d -p 8080:8080 \
  -e API_KEY=my-secret-key \
  -e BASE_RPS=1000 \
  -e SKIP_NOISE_CHARS=true \
  ghcr.io/mingtoneyfurry/sensitive-lexicon:latest
```

### docker-compose

```yaml
services:
  sensitive-server:
    image: ghcr.io/mingtoneyfurry/sensitive-lexicon:latest
    ports:
      - "8080:8080"
    volumes:
      - ./Vocabulary:/app/Vocabulary
      - ./config.json:/app/config.json
    command: ["./sensitive-server", "-config", "/app/config.json"]
    healthcheck:
      test: ["CMD", "wget", "-qO-", "--method=HEAD", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
    restart: unless-stopped
```

### 自行构建 Docker 镜像

```bash
docker build -t sensitive-lexicon:local .
docker run -d -p 8080:8080 sensitive-lexicon:local
```

## 调试

```bash
go test ./...
curl http://127.0.0.1:8080/health
```

## 配置文件（config.json）

配置优先级（由低到高）：内置默认值 → `config.json` → 环境变量。
环境变量始终覆盖文件，方便 Docker / CI 注入。

### 完整字段说明

| 字段（JSON）| 等效环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `listen_addr` | `LISTEN_ADDR` | `:8080` | 监听地址 |
| `lexicon_dir` | `LEXICON_DIR` | `./Vocabulary` | 词库目录（读取全部 `.txt`） |
| `replace_symbol` | `REPLACE_SYMBOL` | `*` | 替换符号 |
| `enable_boundary` | `ENABLE_BOUNDARY` | `true` | 启用边界识别（减少误判） |
| `api_key` | `API_KEY` | 空 | 若设置则除 `/health` 外需要 `X-API-Key` 请求头 |
| `base_rps` | `BASE_RPS` | `600` | 自适应限流基础吞吐 |
| `max_body_bytes` | `MAX_BODY_BYTES` | `1048576` | 请求体上限（字节） |
| `async_queue_length` | `ASYNC_QUEUE_LENGTH` | `128` | 异步检测队列长度 |
| `async_workers` | `ASYNC_WORKERS` | `4` | 异步检测工作协程数 |
| `block_score_threshold` | `BLOCK_SCORE_THRESHOLD` | `0.2` | 默认拦截阈值（0~1；score=匹配字符数/总字符数，score≥threshold 时 blocked=true） |
| `skip_noise_chars` | `SKIP_NOISE_CHARS` | `true` | 匹配时透明跳过空格及常见干扰字符（防规避检测） |
| `ocr.enable` | `ENABLE_OCR` | `false` | 启用 OCR 扩展（手动开启） |
| `ocr.use_gpu` | `OCR_USE_GPU` | `false` | OCR 使用 GPU |
| `ocr.gpu_device` | `OCR_GPU_DEVICE` | `"0"` | GPU 设备 ID（多卡场景） |
| `ocr.auto_download` | `OCR_AUTO_DOWNLOAD` | `true` | 启动时自动下载/更新 OCR 仓库 |
| `ocr.repo_url` | `OCR_REPO_URL` | `https://github.com/DayBreak-u/chineseocr_lite.git` | OCR 仓库地址 |
| `ocr.model_dir` | `OCR_MODEL_DIR` | `./ThirdPartyCompatibleFormats/chineseocr_lite` | OCR 仓库本地目录 |
| `ocr.python_bin` | `OCR_PYTHON_BIN` | `python3` | Python 可执行路径 |
| `ocr.bridge_script` | `OCR_BRIDGE_SCRIPT` | `./internal/ocr/bridge.py` | OCR Python bridge 脚本 |
| `ocr.timeout_sec` | `OCR_TIMEOUT_SEC` | `30` | OCR 单次识别超时（秒） |

### 示例 config.json

```json
{
  "listen_addr": ":8080",
  "lexicon_dir": "./Vocabulary",
  "replace_symbol": "*",
  "enable_boundary": true,
  "api_key": "my-secret-key",
  "base_rps": 600,
  "max_body_bytes": 1048576,
  "async_queue_length": 128,
  "async_workers": 4,
  "block_score_threshold": 0.2,
  "skip_noise_chars": true,
  "ocr": {
    "enable": false,
    "use_gpu": false,
    "gpu_device": "0",
    "auto_download": true,
    "repo_url": "https://github.com/DayBreak-u/chineseocr_lite.git",
    "model_dir": "./ThirdPartyCompatibleFormats/chineseocr_lite",
    "python_bin": "python3",
    "bridge_script": "./internal/ocr/bridge.py",
    "timeout_sec": 30
  }
}
```

> `config.json` 已加入 `.gitignore`，请勿将包含 `api_key` 的配置提交到代码库。

## 规避检测防护

`skip_noise_chars: true` 启用后，检测引擎在匹配前会对输入文本做两步规范化：

1. **全角→半角**：将 Unicode 全角字符（`ａ–ｚ`、`Ａ–Ｚ`、`０–９` 等）映射为 ASCII 等价字符。
2. **透明噪声跳过**：匹配时跳过空格、`*`、`-`、`_`、`.` 等常见干扰字符，同时始终去除零宽字符（`U+200B` 等）。

匹配结果的位置会映射回原始字符串，替换结果覆盖包括噪声字符在内的完整区间。

## OCR 启用示例

在 `config.json` 中：

```json
{
  "ocr": {
    "enable": true,
    "use_gpu": false,
    "gpu_device": "0",
    "auto_download": true
  }
}
```

或直接通过环境变量（适合 Docker）：

```bash
ENABLE_OCR=true OCR_USE_GPU=false ./sensitive-server -config config.json
```

## 接口

### `GET` / `HEAD /health`

返回服务状态、词条数量、OCR 是否启用。`HEAD` 请求仅返回头部（适合 K8s 存活探测、Docker HEALTHCHECK）。

```json
{"status":"ok","words":12345,"ocr_enabled":false}
```

### `GET /contains?text=...`

返回文本是否包含敏感词。

```json
{"contains":true}
```

### `GET` / `POST /detect`

文本检测并替换，附带拦截权重。

**GET 请求（query params）：**

```
GET /detect?text=这是坏词&replace_symbol=%23&block_threshold=0.15
```

**POST 请求体：**

```json
{
  "text": "这是坏词",
  "replace_symbol": "#",
  "block_threshold": 0.15
}
```

**响应体：**

```json
{
  "contains": true,
  "replaced": "这是##",
  "count": 1,
  "score": 0.5,
  "blocked": true,
  "threshold": 0.15,
  "category_scores": {
    "政治类型": 0.5
  }
}
```

字段说明：
- `score` = 匹配字符数 / 总字符数（四舍五入到 4 位小数）
- `blocked` = `contains && score >= threshold`
- `category_scores` = 按词库分类返回分数（可返回多个分类，分数=该分类匹配字符数/总字符数，且同样四舍五入到 4 位小数）

### `POST /detect/image`

图片 OCR + 敏感词检测（仅 `ocr.enable=true` 时可用）。

支持三种输入格式：

**1. JSON + base64（推荐）**

```bash
curl -s http://127.0.0.1:8080/detect/image \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"image_base64":"<base64>","block_threshold":0.2}'
```

**2. multipart/form-data（适合前端表单上传）**

```bash
curl -s http://127.0.0.1:8080/detect/image \
  -H 'X-API-Key: demo-key' \
  -F 'image=@/path/to/test.png' \
  -F 'replace_symbol=#' \
  -F 'block_threshold=0.2'
```

**3. 原始二进制 body**

```bash
curl -s http://127.0.0.1:8080/detect/image \
  -H 'X-API-Key: demo-key' \
  --data-binary @/path/to/test.png
```

**响应体：**

```json
{
  "contains": true,
  "replaced": "****",
  "count": 1,
  "score": 0.33,
  "blocked": true,
  "threshold": 0.2,
  "category_scores": {
    "色情类型": 0.33
  },
  "ocr_text": "识别出的原始文本"
}
```

### `GET` / `POST /detect/async`

异步提交检测任务，参数与 `/detect` 完全相同，立即返回 `job_id`。

```json
{"job_id":"job-1"}
```

### `GET /detect/async/result?job_id=...`

查询异步检测结果；任务未完成时返回 `202 Accepted`。

### `GET` / `POST /sanitize-stream`

按行流式替换，适合大文件场景（POST 请求体为换行分隔的文本流；GET 处理 `?text=` 单行）。

### `GET` / `POST /reload`

热加载词库目录（不重启服务）。

## 完整请求示例

### 检测并替换（POST）

```bash
curl -s http://127.0.0.1:8080/detect \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","replace_symbol":"#","block_threshold":0.2}'
```

### 检测并替换（GET）

```bash
curl -s 'http://127.0.0.1:8080/detect?text=这是坏词&block_threshold=0.2' \
  -H 'X-API-Key: demo-key'
```

### 健康检查（存活探测）

```bash
# HEAD — 仅验证服务在线，不返回 body
curl -I http://127.0.0.1:8080/health

# GET — 返回详细状态
curl http://127.0.0.1:8080/health
```

### 异步检测

```bash
# 提交
curl -s http://127.0.0.1:8080/detect/async \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","block_threshold":0.2}'

# 查询
curl -s 'http://127.0.0.1:8080/detect/async/result?job_id=job-1' \
  -H 'X-API-Key: demo-key'
```

### 流式文件处理

```bash
cat input.txt | curl -s http://127.0.0.1:8080/sanitize-stream \
  -H 'X-API-Key: demo-key' --data-binary @-
```

### 热加载词库

```bash
curl -s -X POST http://127.0.0.1:8080/reload -H 'X-API-Key: demo-key'
```

## 压测（图片路径）

```bash
go test -run '^$' -bench BenchmarkDetect -benchmem -count=5 ./internal/server
```

| 场景 | ns/op 范围 | 最高 req/s | 平均 req/s |
|---|---|---|---|
| `/detect`（纯文本） | 18,638 ~ 19,957 | **~53,654** | **~52,279** |
| `/detect/image`（OCR 启用，真实 PNG）| 20,020 ~ 21,411 | **~49,950** | **~48,301** |

测试环境：AMD EPYC 9V74 / 4 vCPU / 15 GiB / Ubuntu 24.04 / Go 1.24.x

压测使用真实 10×10 像素 PNG（由 `image/png` 生成），OCR 使用 stub 以消除外部 Python 进程影响，测量纯 Go 服务层吞吐。

