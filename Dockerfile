FROM python:3.11-bookworm AS python-deps

RUN pip install --no-cache-dir telegramify-markdown

FROM golang:1.24-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y \
    python3 \
    python3-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

COPY --from=python-deps /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=1 go build \
    -trimpath \
    -o ximanager \
    -ldflags="-s -w -X main.version=2.2.3 -X main.buildTime=$(date -u +%Y%m%d-%H%M%S)" \
    ./program.go

FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir telegramify-markdown

COPY --from=builder /app/ximanager /ximanager

EXPOSE 10000 10001
CMD ["/ximanager"]