# 配置文档

| 变量 | 默认值 | 说明 |
|---|---:|---|
| `PORT` | `8080` | 服务监听端口 |
| `LEXICON_DIR` | `./Vocabulary` | 词库目录 |
| `API_KEY` | 空 | 非空时启用网关鉴权 |
| `REPLACE_SYMBOL` | `*` | 替换符号 |
| `KEEP_REPLACEMENT_LENGTH` | `true` | 是否按命中词长度替换 |
| `ASYNC_WORKERS` | CPU 核数 | 异步任务 worker 数 |
| `ASYNC_QUEUE_SIZE` | `1024` | 异步队列长度 |
| `BASE_RPS` | `300` | 自适应限流基础 RPS |
| `MIN_RPS` | `50` | 自适应限流最小 RPS |
| `MAX_RPS` | `1500` | 自适应限流最大 RPS |
| `ADAPT_INTERVAL_SECONDS` | `2` | 限流参数自适应调整周期 |
