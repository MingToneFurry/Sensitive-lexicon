# 部署文档

## 本地运行

```bash
go run ./cmd/server
```

## 二进制部署

```bash
go build -o sensitive-gateway ./cmd/server
./sensitive-gateway
```

## Docker（示例）

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/sensitive-gateway ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/sensitive-gateway /app/sensitive-gateway
COPY Vocabulary /app/Vocabulary
ENV PORT=8080 LEXICON_DIR=/app/Vocabulary
EXPOSE 8080
ENTRYPOINT ["/app/sensitive-gateway"]
```
