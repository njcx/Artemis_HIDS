FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY mongo_syncer/go.mod mongo_syncer/go.sum ./

RUN go env -w GO111MODULE=on
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY mongo_syncer/. .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /build/mongo_syncer .

FROM alpine:latest

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories && \
    apk --no-cache add ca-certificates tzdata gettext

WORKDIR /app

COPY --from=builder /build/mongo_syncer .
COPY docker/entrypoint-render-config.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./mongo_syncer", "-config", "config.yaml"]
