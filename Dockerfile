# 多阶段构建：产物是静态二进制（CGO_ENABLED=0），也可直接丢进 distroless / scratch
FROM golang:1.25-alpine AS build

ARG VERSION=dev

WORKDIR /src
# 先只拷贝依赖清单，让模块下载层可复用
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false \
	-ldflags "-s -w -X main.version=${VERSION}" \
	-o /out/cddns ./cmd/cddns

FROM alpine:3.21

# ca-certificates：ACME 与 DNS API 的 HTTPS 请求必需；tzdata 让日志时间带时区
RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /out/cddns /usr/local/bin/cddns

# 配置与证书目录：容器里靠挂载持久化
VOLUME ["/etc/cddns", "/var/lib/cddns"]
ENV CDDNS_CONFIG=/etc/cddns/config.yaml \
	CDDNS_DATA_DIR=/var/lib/cddns \
	XDG_DATA_HOME=/var/lib/cddns \
	XDG_CONFIG_HOME=/var/lib/cddns
# 反向代理与 ACME 需要真实监听 80/443，实际使用请用 host 网络
EXPOSE 80 443

ENTRYPOINT ["/usr/local/bin/cddns"]
CMD ["run", "--config", "/etc/cddns/config.yaml"]
