# Sensitive-lexicon

中文敏感词库 + Go 高性能检测服务（低占用、可热加载、本地配置文件、支持 OCR 图片识别扩展）。

## 功能一览

- Trie/DFA 风格匹配引擎，支持边界识别（减少误判）
- **规避检测防护**：自动规范化全角→半角字符、透明跳过零宽字符及常见干扰符（`色*情`→`色情`），防止插入特殊字符绕过过滤
- 自定义替换符号（全局配置 + 单请求覆盖）
- 句子拦截权重：返回 `score`、`threshold`、`blocked`，并返回多分类分数 `category_scores`
- **本地 JSON 配置文件**（`config.json`），env var 仍可作为覆盖层
- 热加载词库（`/reload`）
- 异步检测接口（`/detect/async`）
- 流式处理接口（`/sanitize-stream`，按行读写）
- API Key 鉴权（`X-API-Key`）
- 基于系统负载/内存的自适应限流
- **多方法支持**：POST 接口同时支持 GET（via query params），`/health` 支持 HEAD（满足 K8s/Docker 存活探测）
- **OCR 扩展**（手动启用）：对接 `DayBreak-u/chineseocr_lite`，支持模型仓库自动下载、GPU 可选
- 图片检测接口（`/detect/image`）：支持 `base64`、`multipart/form-data`、原始二进制 POST
- GitHub Actions：构建发布、每日双源上游同步（Konsheng + houbb/sensitive-word-data 自动去重）、每周 OCR 模型库同步
- **Docker 部署**：官方镜像发布至 GitHub Container Registry（`ghcr.io`）

## 目录结构

```text
Sensitive-lexicon/
├── cmd/server                 # 服务入口
├── internal/                  # 核心实现（词典、限流、OCR、服务）
├── Vocabulary/                # 词库（含自动同步的 houbb-sensitive-word.txt）
├── Dockerfile                 # Docker 部署镜像
├── config.example.json        # 配置文件模板
├── ThirdPartyCompatibleFormats/chineseocr_lite  # OCR 模型/代码同步目录
├── docs/service.md            # 详细 API 与部署文档
└── .github/workflows/
```

## 快速开始

### 方式一：直接编译

```bash
# 1. 编译
go build -o sensitive-server ./cmd/server

# 2. 复制配置文件模板并按需修改
cp config.example.json config.json

# 3. 启动（自动读取 config.json）
./sensitive-server

# 或者指定配置文件路径
./sensitive-server -config /etc/myapp/config.json
```

### 方式二：Docker 部署

```bash
# 拉取最新镜像
docker pull ghcr.io/mingtoneyfurry/sensitive-lexicon:latest

# 运行（直接使用内置词库）
docker run -d -p 8080:8080 ghcr.io/mingtoneyfurry/sensitive-lexicon:latest

# 挂载自定义词库目录
docker run -d -p 8080:8080 \
  -v /path/to/your/Vocabulary:/app/Vocabulary \
  ghcr.io/mingtoneyfurry/sensitive-lexicon:latest

# 挂载自定义配置文件
docker run -d -p 8080:8080 \
  -v /path/to/config.json:/app/config.json \
  ghcr.io/mingtoneyfurry/sensitive-lexicon:latest \
  ./sensitive-server -config /app/config.json

# 通过环境变量覆盖配置
docker run -d -p 8080:8080 \
  -e API_KEY=my-secret-key \
  -e BASE_RPS=1000 \
  ghcr.io/mingtoneyfurry/sensitive-lexicon:latest
```

### 方式三：docker-compose

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

环境变量仍被支持，且会覆盖配置文件中的对应字段（方便 Docker / CI 注入）。

## 配置文件（config.json）

```json
{
  "listen_addr": ":8080",
  "lexicon_dir": "./Vocabulary",
  "replace_symbol": "*",
  "enable_boundary": true,
  "api_key": "",
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

完整字段说明见 [`docs/service.md`](docs/service.md)。

## 规避检测防护

启用 `skip_noise_chars: true`（默认）后，以下输入均会被检测为敏感词：

| 输入 | 效果 |
|------|------|
| `色情` | 直接匹配 |
| `色 情` | 忽略空格 |
| `色*情` | 忽略星号 |
| `色_情` | 忽略下划线 |
| `ｓｅｘ` | 全角→半角转换后匹配 |
| `色\u200B情` | 零宽字符自动过滤 |

如需关闭（精确匹配模式）：

```json
{ "skip_noise_chars": false }
```

或：`SKIP_NOISE_CHARS=false`

## OCR 扩展启用

在 `config.json` 中将 `ocr.enable` 改为 `true`：

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

> **注意**：启用 OCR 需自行准备 Python 依赖（`numpy`、`Pillow`、`torch` 等）。

## API 概览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET / HEAD | `/health` | 健康检查（HEAD 仅返回头部，适合存活探测） |
| GET  | `/contains?text=…` | 是否包含敏感词 |
| GET / POST | `/detect` | 文本检测 + 替换 + 拦截权重 |
| POST | `/detect/image` | 图片 OCR + 敏感词检测（需 OCR 启用） |
| GET / POST | `/detect/async` | 异步提交检测任务 |
| GET  | `/detect/async/result?job_id=…` | 查询异步结果 |
| GET / POST | `/sanitize-stream` | 流式替换（POST 按行；GET 处理单行 `?text=`） |
| GET / POST | `/reload` | 热加载词库 |

详细见：[`docs/service.md`](docs/service.md)

## 请求示例

### 文本检测（POST）

```bash
curl -s http://127.0.0.1:8080/detect \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","replace_symbol":"#","block_threshold":0.15}'
```

### 文本检测（GET）

```bash
curl -s 'http://127.0.0.1:8080/detect?text=这是坏词&replace_symbol=%23&block_threshold=0.15' \
  -H 'X-API-Key: demo-key'
```

### 健康检查（HEAD，用于存活探测）

```bash
curl -I http://127.0.0.1:8080/health
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

| Workflow | 触发 | 说明 |
|---|---|---|
| `build-release.yml` | `main` 分支推送 / 手动 | 多平台（Linux/Windows/macOS）构建并发布两个压缩包；同时构建 Docker 镜像推送至 `ghcr.io` |
| `daily-sync-upstream.yml` | 每天 UTC 02:00 | 同步两个上游词库（Konsheng + houbb/sensitive-word-data）并自动去重 |
| `weekly-sync-ocr-models.yml` | 每周一 UTC 02:00 | 同步 `chineseocr_lite` 代码快照 |

## 压测结果（使用真实 PNG 图片）

### 服务器配置

| 项目 | 值 |
|---|---|
| CPU | AMD EPYC 9V74（4 vCPU 可用） |
| 内存 | 15 GiB |
| 系统 | Ubuntu 24.04（Linux 6.17） |
| Go | 1.24.x |
| 测试方法 | `go test -run '^$' -bench BenchmarkDetect -benchmem -count=5 ./internal/server` |

### /detect（纯文本）

| 指标 | 值 |
|---|---|
| ns/op 范围 | 18,638 ~ 19,957 |
| 最高吞吐 | **~53,654 req/s** |
| 5 轮平均 | **~52,279 req/s** |

### /detect/image（OCR 启用，真实 PNG）

| 指标 | 值 |
|---|---|
| ns/op 范围 | 20,020 ~ 21,411 |
| 最高吞吐 | **~49,950 req/s** |
| 5 轮平均 | **~48,301 req/s** |

### 优化说明

- 检测阶段复用匹配结果进行替换，避免重复 Trie 扫描（减少 ~1 次全量扫描）
- OCR 图片接口复用统一权重判定逻辑
- 基于系统负载和内存使用的自适应限流，防止高压雪崩
- 规避检测使用 O(n) 文本规范化预处理，不影响热路径吞吐

## 兼容与注意事项

- 请遵守当地法律法规与平台政策。
- 敏感词识别受上下文影响，建议结合业务白名单 / 人工审核。
- `config.json` 可能包含 `api_key` 等敏感信息，已加入 `.gitignore`，请勿提交。

## License

MIT

