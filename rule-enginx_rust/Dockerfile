FROM rust:1-slim-bookworm AS builder

RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g; s|security.debian.org|mirrors.aliyun.com/debian-security|g' /etc/apt/sources.list.d/debian.sources

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    make \
    g++ \
    libssl-dev  libsasl2-dev librdkafka-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY docker/cargo-config.toml /usr/local/cargo/config.toml

COPY rule-enginx_rust/Cargo.toml rule-enginx_rust/Cargo.lock ./
COPY rule-enginx_rust/src ./src

RUN cargo build --release

FROM debian:bookworm-slim

RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g; s|security.debian.org|mirrors.aliyun.com/debian-security|g' /etc/apt/sources.list.d/debian.sources

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates  libsasl2-dev librdkafka-dev libssl-dev  \
    gettext-base \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/target/release/rule-enginx .
COPY rule-enginx_rust/rules ./rules
COPY docker/entrypoint-render-config.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./rule-enginx", "--config", "config.yaml"]
