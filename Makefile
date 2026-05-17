.PHONY: build install test lint cross check-size clean gui gui-dev release-cli checksums

BINARY_NAME := tegata
BUILD_DIR := bin
PREFIX ?= /usr/local
LDFLAGS := -s -w
MAX_SIZE := 20971520

ifeq ($(OS),Windows_NT)
BINARY_SUFFIX := .exe
ENV_CGO_DISABLED := set "CGO_ENABLED=0" &&
ENV_WIN_AMD64 := set "CGO_ENABLED=0" && set "GOOS=windows" && set "GOARCH=amd64" &&
ENV_DARWIN_ARM64 := set "CGO_ENABLED=0" && set "GOOS=darwin" && set "GOARCH=arm64" &&
ENV_DARWIN_AMD64 := set "CGO_ENABLED=0" && set "GOOS=darwin" && set "GOARCH=amd64" &&
ENV_LINUX_AMD64 := set "CGO_ENABLED=0" && set "GOOS=linux" && set "GOARCH=amd64" &&
else
BINARY_SUFFIX :=
ENV_CGO_DISABLED := CGO_ENABLED=0
ENV_WIN_AMD64 := CGO_ENABLED=0 GOOS=windows GOARCH=amd64
ENV_DARWIN_ARM64 := CGO_ENABLED=0 GOOS=darwin GOARCH=arm64
ENV_DARWIN_AMD64 := CGO_ENABLED=0 GOOS=darwin GOARCH=amd64
ENV_LINUX_AMD64 := CGO_ENABLED=0 GOOS=linux GOARCH=amd64
endif

build:
	$(ENV_CGO_DISABLED) go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_SUFFIX) ./cmd/tegata/

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_SUFFIX) $(PREFIX)/bin/$(BINARY_NAME)

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

cross:
	$(ENV_WIN_AMD64) go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/tegata/
	$(ENV_DARWIN_ARM64) go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/tegata/
	$(ENV_DARWIN_AMD64) go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/tegata/
	$(ENV_LINUX_AMD64) go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/tegata/

check-size: build
	@SIZE=$$(wc -c < $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_SUFFIX)); \
	echo "Binary size: $$SIZE bytes"; \
	if [ "$$SIZE" -gt "$(MAX_SIZE)" ]; then \
		echo "ERROR: Binary exceeds 20MB limit"; \
		exit 1; \
	fi; \
	echo "OK: Binary is under 20MB limit"

gui:
	cd cmd/tegata-gui && wails build -clean
	mv cmd/tegata-gui/build/bin/tegata-gui.app cmd/tegata-gui/build/bin/Tegata.app

gui-dev:
	cd cmd/tegata-gui && wails dev

release-cli:
	@VERSION=$${VERSION:-dev}; \
	for target in "windows/amd64/.exe" "darwin/arm64/" "darwin/amd64/" "linux/amd64/"; do \
		IFS='/' read -r goos goarch ext <<< "$$target"; \
		echo "Building tegata-$$goos-$$goarch$$ext"; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build \
			-ldflags="-s -w -X main.version=$$VERSION" \
			-o $(BUILD_DIR)/tegata-$$goos-$$goarch$$ext ./cmd/tegata/; \
	done

checksums:
	cd $(BUILD_DIR) && sha256sum * > SHA256SUMS.txt

clean:
	rm -rf $(BUILD_DIR)
