BINARY      := bin/cddns
RELEASE_DIR := release
DIST_DIR    := dist

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
# -buildvcs=false：仓库状态异常（例如残缺 .git）时不阻断构建
BUILD_FLAGS := -trimpath -buildvcs=false

# 交叉编译矩阵：<发布名>/<GOARCH>/<GOARM>，GOOS 固定 linux
DIST_PLATFORMS := linux-amd64/amd64/ linux-arm64/arm64/ linux-armv7/arm/7

IMAGE ?= cddns:$(VERSION)

.PHONY: build test vet clean install release dist docker docker-run

build:
	mkdir -p bin
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/cddns

test:
	go test -buildvcs=false ./...

vet:
	go vet -buildvcs=false ./...

clean:
	rm -rf bin $(DIST_DIR) $(RELEASE_DIR)

install: build
	./scripts/install.sh $(BINARY)

# 扁平发布目录：二进制 + 安装脚本，可直接 sudo ./release/install.sh
release: build
	rm -rf $(RELEASE_DIR)
	mkdir -p $(RELEASE_DIR)
	cp $(BINARY) scripts/install.sh scripts/cddns.service LICENSE $(RELEASE_DIR)/
	chmod 755 $(RELEASE_DIR) $(RELEASE_DIR)/cddns $(RELEASE_DIR)/install.sh
	chmod 644 $(RELEASE_DIR)/cddns.service $(RELEASE_DIR)/LICENSE
	@echo "发布目录: $(RELEASE_DIR)/（cddns + install.sh + cddns.service）"

# 多架构发布包：linux/amd64、linux/arm64、linux/armv7
dist:
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	set -e; \
	for p in $(DIST_PLATFORMS); do \
		set -- $$(echo "$$p" | tr '/' ' '); \
		name=$$1; goarch=$$2; goarm=$$3; \
		dir=$(DIST_DIR)/cddns_$(VERSION)_$$name; \
		echo ">> $$name (GOOS=linux GOARCH=$$goarch GOARM=$$goarm)"; \
		mkdir -p "$$dir"; \
		GOOS=linux GOARCH=$$goarch GOARM=$$goarm CGO_ENABLED=0 \
			go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o "$$dir/cddns" ./cmd/cddns; \
		cp scripts/install.sh scripts/cddns.service README.md LICENSE "$$dir/"; \
		chmod 755 "$$dir" "$$dir/cddns" "$$dir/install.sh"; \
		chmod 644 "$$dir/cddns.service" "$$dir/README.md" "$$dir/LICENSE"; \
		tar -C $(DIST_DIR) -czf "$$dir.tar.gz" "$$(basename $$dir)"; \
		rm -rf "$$dir"; \
	done; \
	(cd $(DIST_DIR) && sha256sum *.tar.gz > checksums.txt); \
	ls -lh $(DIST_DIR)

# 多架构镜像（需要 buildx）；推送：make docker IMAGE=ghcr.io/<owner>/cddns
docker:
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
		--build-arg VERSION=$(VERSION) -t $(IMAGE) .

docker-run: docker
	docker run --rm -it --network host \
		-v /etc/cddns:/etc/cddns -v /var/lib/cddns:/var/lib/cddns \
		$(IMAGE) version
