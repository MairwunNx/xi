FROM golang:1.24.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && go mod verify

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -o /app/ximanager \
    -installsuffix cgo \
    -gcflags="all=-B -C" \
    -ldflags="-s -w -X main.version=2.0.1 -X main.buildTime=$(date -u +%Y%m%d-%H%M%S)" \
    ./program.go

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/ximanager /ximanager
EXPOSE 10000 10001
CMD ["/ximanager"]