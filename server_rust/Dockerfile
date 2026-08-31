FROM rust:1-slim-bookworm AS builder

RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g; s|security.debian.org|mirrors.aliyun.com/debian-security|g' /etc/apt/sources.list.d/debian.sources

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    make \
    g++ \
    libssl-dev \
    pkg-config libsasl2-dev librdkafka-dev \
    protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY docker/cargo-config.toml /usr/local/cargo/config.toml

COPY server_rust/Cargo.toml server_rust/Cargo.lock ./
COPY server_rust/build.rs ./
COPY server_rust/proto ./proto
COPY server_rust/src ./src

RUN cargo build --release

FROM debian:bookworm-slim

RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g; s|security.debian.org|mirrors.aliyun.com/debian-security|g' /etc/apt/sources.list.d/debian.sources

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libsasl2-dev librdkafka-dev libssl-dev \
    gettext-base \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /var/log/artemis-hids

WORKDIR /app

COPY --from=builder /build/target/release/hids-server .
COPY server_rust/certs ./certs
COPY docker/entrypoint-render-config.sh /entrypoint.sh

EXPOSE 50051

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./hids-server", "--config", "config.yaml"]
