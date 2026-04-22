# Sensitive-lexicon

中文敏感词库 + Go 高性能检测服务（低占用、可热加载、支持 OCR 图片识别扩展）。

## 新增能力（本次实现）

- Trie/DFA 风格匹配引擎，支持边界识别（减少误判）
- 自定义替换符号（全局配置 + 单请求覆盖）
- 句子拦截权重：返回 `score`、`threshold`、`blocked`
- 热加载词库（`/reload`）
- 异步检测接口（`/detect/async`）
- 流式处理接口（`/sanitize-stream`，按行读写）
- API Key 鉴权（`X-API-Key`）
- 基于系统负载/内存的自适应限流
- OCR 扩展（手动启用）：对接 `DayBreak-u/chineseocr_lite`，支持模型仓库自动下载、GPU 可选
- 图片检测接口（`/detect/image`）：支持 `base64`、`multipart/form-data`、原始二进制 POST
- GitHub Actions：构建发布、每日上游同步、每周 OCR 模型库同步

## 目录结构

```text
Sensitive-lexicon/
├── cmd/server
├── internal/
├── Vocabulary/
├── ThirdPartyCompatibleFormats/chineseocr_lite  # OCR 模型/代码同步目录
├── docs/service.md
└── .github/workflows/
```

## 快速开始

```bash
go build -o sensitive-server ./cmd/server
API_KEY=demo-key LISTEN_ADDR=:8080 LEXICON_DIR=./Vocabulary ./sensitive-server
```

## OCR 扩展启用

默认关闭。仅在手动开启时初始化并下载 OCR 模型库。

```bash
ENABLE_OCR=true \
OCR_AUTO_DOWNLOAD=true \
OCR_USE_GPU=false \
OCR_GPU_DEVICE=0 \
OCR_MODEL_DIR=./ThirdPartyCompatibleFormats/chineseocr_lite \
OCR_REPO_URL=https://github.com/DayBreak-u/chineseocr_lite.git \
OCR_PYTHON_BIN=python3 \
OCR_BRIDGE_SCRIPT=./internal/ocr/bridge.py \
./sensitive-server
```

> 注意：启用 OCR 需自行准备 Python 依赖（如 `numpy`、`Pillow`、`torch` 等，以及 chineseocr_lite 运行依赖）。

## API 概览

- `GET /health`：健康检查
- `GET /contains?text=...`：是否包含敏感词
- `POST /detect`：文本检测并替换（含拦截权重）
- `POST /detect/image`：图片 OCR + 敏感词检测
- `POST /reload`：热加载词库
- `POST /detect/async` + `GET /detect/async/result`：异步检测
- `POST /sanitize-stream`：流式替换

详细见：[`docs/service.md`](docs/service.md)

## 请求示例

### 文本检测（带阈值）

```bash
curl -s http://127.0.0.1:8080/detect \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","replace_symbol":"#","block_threshold":0.15}'
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
  -F 'block_threshold=0.2'
```

## CI/CD 与自动同步

- `.github/workflows/build-release.yml`
  - 在 `v*` tag 上构建 Ubuntu/Windows/macOS，Go 1.22/1.23，并发布到 Release
- `.github/workflows/daily-sync-upstream.yml`
  - 每天 UTC 02:00 同步上游并重新构建
- `.github/workflows/weekly-sync-ocr-models.yml`
  - 每周 UTC 02:00（周一）同步 `DayBreak-u/chineseocr_lite` 到 `ThirdPartyCompatibleFormats/chineseocr_lite`

## 压测与优化（启用 OCR 路径）

### 服务器配置

- CPU: 4 vCPU（Intel Xeon Platinum 8370C）
- 内存: 15 GiB
- 系统: Ubuntu 24.04 (Linux 6.17)
- Go: 1.24.13

### 压测命令

```bash
go test -run '^$' -bench BenchmarkDetectImageWithOCRParallel -benchmem -count=5 ./internal/server
```

### 结果（OCR 启用路径）

- ns/op：`17344 ~ 19118`
- 单次最高吞吐：约 **57,656 req/s**
- 5 轮平均吞吐：约 **55,569 req/s**

### 本轮优化

- 检测阶段复用匹配结果进行替换，避免重复 Trie 扫描
- OCR 图片接口复用统一权重判定逻辑，减少重复计算分支

## License

MIT
