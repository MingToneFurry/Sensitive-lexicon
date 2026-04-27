# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /sensitive-server ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.20
LABEL org.opencontainers.image.source="https://github.com/MingToneFurry/Sensitive-lexicon"
LABEL org.opencontainers.image.description="High-performance Chinese sensitive word detection service"
LABEL org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /sensitive-server ./sensitive-server
COPY Vocabulary/ ./Vocabulary/
COPY config.example.json ./

# Default configuration (override via volume or environment variables)
ENV LISTEN_ADDR=:8080
ENV LEXICON_DIR=/app/Vocabulary

EXPOSE 8080

# Health check using the /health endpoint (HEAD is supported)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- --method=HEAD http://localhost:8080/health || exit 1

ENTRYPOINT ["./sensitive-server"]
