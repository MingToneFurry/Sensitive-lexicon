# Sensitive-lexicon（中文敏感词库 + Go 检测网关）

本项目在原有词库基础上，新增了 **Go 高性能敏感词检测网关**，支持：

- Trie + DFS 精确识别（优先最长匹配，减少误判）
- 自定义替换符号
- 词库热加载（`/reload`）
- 异步检测（`/detect/async` + `/detect/result`）
- 流式读写检测（`/detect/stream`，降低内存占用）
- API Key 鉴权
- 基于运行时资源信息的自适应限流
- GitHub Actions 多系统多 Go 版本自动编译与 Release
- 每日自动同步上游词库并自动触发新版本构建

## 目录

- `Vocabulary/`：主词库
- `Organized/`：整理词库
- `ThirdPartyCompatibleFormats/`：第三方兼容格式
- `cmd/server`：Go 网关入口
- `internal/lexicon`：检测、加载、限流核心
- `docs/`：部署、调试、使用、配置文档

## 快速开始

```bash
go run ./cmd/server
```

### API 示例

```bash
curl -X POST http://127.0.0.1:8080/detect \
  -H 'Content-Type: application/json' \
  -d '{"text":"测试文本","replace":true,"symbol":"*"}'
```

## 文档

- 部署：`docs/DEPLOYMENT.md`
- 调试：`docs/DEBUG.md`
- 使用与请求示例：`docs/USAGE.md`
- 配置：`docs/CONFIGURATION.md`

## CI/CD 与自动同步

- `.github/workflows/main.yml`：多系统多版本构建 + 发布 Release
- `.github/workflows/sync-upstream.yml`：每日同步上游词库并自动提交与打 tag

## 开源许可

MIT
