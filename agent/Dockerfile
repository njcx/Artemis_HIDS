FROM golang:1.25-alpine AS builder

ARG PUBLIC_IP=127.0.0.1

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories && \
    apk --no-cache add gcc musl-dev

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /build

RUN go env -w GOPROXY=https://goproxy.cn,direct

COPY agent/go.mod agent/go.sum ./
RUN go mod download

COPY agent/. .

RUN go build -ldflags "-s -w" -o artemis_agent . && \
    mkdir -p workers && \
    go build -ldflags "-s -w" -o workers/file ./workers/file_monitor/ && \
    go build -ldflags "-s -w" -o workers/network ./workers/network_monitor/ && \
    go build -ldflags "-s -w" -o workers/log ./workers/security_log_monitor/ && \
    go build -ldflags "-s -w" -o workers/exec ./workers/exec_monitor/ && \
    go build -ldflags "-s -w" -o workers/system ./workers/system_info_monitor/

RUN sed 's|"server_addr": *"[^"]*"|"server_addr": "'"${PUBLIC_IP}"':50051"|' \
    config.example.json > config.json

RUN sed 's|ARTEMIS_SERVER:-http://localhost:8081|ARTEMIS_SERVER:-http://'"${PUBLIC_IP}"':8081|' \
    install-remote.sh > /tmp/install-remote.sh

RUN mkdir -p /out && \
    cp artemis_agent /out/ && \
    cp -r workers /out/ && \
    cp -r certs /out/ && \
    cp install.sh uninstall.sh artemis-agent.service config.example.json config.json /out/ && \
    tar -czf /out/artemis-agent.tar.gz -C /out artemis_agent workers certs install.sh uninstall.sh artemis-agent.service config.example.json config.json && \
    cp /tmp/install-remote.sh /out/install.sh

FROM alpine:latest

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories

WORKDIR /out

COPY --from=builder /out/artemis-agent.tar.gz /out/install.sh  /out/uninstall.sh ./

CMD ["true"]
