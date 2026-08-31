FROM golang:1.24-alpine AS builder

WORKDIR /build

RUN go env -w GO111MODULE=on
RUN go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
COPY manager/go.mod manager/go.sum ./
RUN go mod download

COPY manager/. .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /build/bin/manager-api ./cmd/api

FROM alpine:latest

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata gettext

WORKDIR /app

COPY --from=builder /build/bin/manager-api .
COPY docker/entrypoint-render-config.sh /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./manager-api"]
