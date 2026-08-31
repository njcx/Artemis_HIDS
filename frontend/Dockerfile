FROM node:16-alpine AS frontend-builder

WORKDIR /build

COPY frontend/package.json frontend/yarn.lock* ./
RUN sed -i 's|https://registry\.npm\.taobao\.org|https://registry.npmmirror.com|g' yarn.lock && \
    yarn config set registry https://registry.npmmirror.com && \
    yarn install --frozen-lockfile

COPY frontend/. .

RUN NODE_OPTIONS=--max-old-space-size=768 npx vue-cli-service build

FROM artemis-agent:latest AS agent-builder

FROM nginx:alpine

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories

COPY --from=frontend-builder /build/dist /usr/share/nginx/html
COPY --from=agent-builder /out/artemis-agent.tar.gz /usr/share/nginx/agent-download/artemis-agent.tar.gz
COPY --from=agent-builder /out/install.sh /usr/share/nginx/agent-download/install.sh
COPY --from=agent-builder /out/uninstall.sh /usr/share/nginx/agent-download/uninstall.sh
COPY docker/nginx.conf /etc/nginx/nginx.conf

EXPOSE 80 8081

CMD ["nginx", "-g", "daemon off;"]
