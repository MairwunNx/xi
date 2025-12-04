FROM alpine:3.22 AS certs
RUN apk --no-cache add ca-certificates tzdata && update-ca-certificates

FROM alpine:3.22 AS pydeps
RUN apk --no-cache add python3 py3-pip
RUN python3 -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH" PIP_NO_CACHE_DIR=1
RUN pip install --upgrade pip && pip install telegramify-markdown

FROM golang:1.25.4-alpine3.21 AS builder
WORKDIR /src
RUN apk --no-cache add git build-base pkgconfig python3 python3-dev py3-pip
ENV CGO_ENABLED=1
COPY go.mod go.sum ./
RUN go mod download
COPY program.go ./
COPY sources/ ./sources/
COPY .version ./.version
ARG APP_VERSION
ARG BUILD_TIME
RUN : "${APP_VERSION:=$(cat ./.version 2>/dev/null || echo 0.0.0)}" && \
    : "${BUILD_TIME:=$(date -u +%Y%m%d-%H%M%S)}" && \
    go build -trimpath \
      -ldflags="-s -w -X main.version=${APP_VERSION} -X main.buildTime=${BUILD_TIME}" \
      -o /app/ximanager ./program.go
FROM alpine:3.22
WORKDIR /app
RUN apk --no-cache add python3 && adduser -D -s /bin/sh ximanager
COPY --from=certs  /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=certs  /usr/share/zoneinfo                /usr/share/zoneinfo
COPY --from=pydeps /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
COPY --from=builder /app/ximanager /usr/local/bin/ximanager
USER ximanager
EXPOSE 10000 9090 9091
ENTRYPOINT ["/usr/local/bin/ximanager"]