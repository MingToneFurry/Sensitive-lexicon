# Sensitive-lexicon

中文敏感词库 + Go 高性能检测服务（低占用、可热加载、支持自定义替换、异步和流式处理）。

## 新增能力（本次实现）

- Trie/DFA 风格匹配引擎，支持边界识别（减少误判）
- 自定义替换符号（全局配置 + 单请求覆盖）
- 热加载词库（`/reload`）
- 异步检测接口（`/detect/async`）
- 流式处理接口（`/sanitize-stream`，按行读写）
- API Key 鉴权（`X-API-Key`）
- 基于系统负载/内存的自适应限流
- GitHub Actions：多系统多 Go 版本构建 + tag 发布 + 每日同步上游

## 目录结构

```text
Sensitive-lexicon/
├── cmd/server                 # Go 服务入口
├── internal/                  # 核心实现（词典、限流、服务）
├── Vocabulary/                # 词库
├── docs/service.md            # 部署/调试/配置/API 文档
└── .github/workflows/         # CI/CD 与每日同步
```

## 快速开始

```bash
go build -o sensitive-server ./cmd/server
API_KEY=demo-key LISTEN_ADDR=:8080 LEXICON_DIR=./Vocabulary ./sensitive-server
```

## API 概览

- `GET /health`：健康检查
- `GET /contains?text=...`：是否包含敏感词
- `POST /detect`：检测并替换
- `POST /reload`：热加载词库
- `POST /detect/async` + `GET /detect/async/result`：异步检测
- `POST /sanitize-stream`：流式替换

详细部署、调试、配置、请求示例见：[`docs/service.md`](docs/service.md)

## 请求示例

```bash
curl -s http://127.0.0.1:8080/detect \
  -H 'X-API-Key: demo-key' \
  -H 'Content-Type: application/json' \
  -d '{"text":"这是坏词","replace_symbol":"#"}'
```

## CI/CD 与自动同步

- `.github/workflows/build-release.yml`
  - 在 `v*` tag 上构建 Ubuntu/Windows/macOS，Go 1.22/1.23，并发布到 Release
- `.github/workflows/daily-sync-upstream.yml`
  - 每天 UTC 02:00 同步上游并重新构建

## 压力测试与分析（本地实测）

测试环境：
- CPU: 2 vCPU
- 内存: 7GB
- 系统: Ubuntu 24.04 (GitHub Actions runner)
- Go: 1.22

方法：
- 使用 `go test -bench BenchmarkDetectParallel -benchmem ./internal/server`

结果：
- `BenchmarkDetectParallel-4    49609    23764 ns/op    11966 B/op    43 allocs/op`
- 估算服务层可稳定承载约 **4.0~4.2 万次检测/秒**（受网关、JSON、网络与 API Key 校验影响会波动）

优化说明：
- 使用原子热切换词典，避免检测时加锁
- 复用 Trie 结构进行前缀扫描，降低误判（边界识别）
- 按系统负载动态降速，避免高压下雪崩
- 流式读写接口降低大文本场景内存峰值

## 兼容与注意事项

- 请遵守当地法律法规与平台政策。
- 敏感词识别会受上下文影响，建议结合业务白名单/人工审核。
- 若需极限吞吐，可在网关层启用连接复用 + 批量请求。

## License

MIT
