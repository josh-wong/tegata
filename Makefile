.PHONY: build test lint cross check-size clean gui gui-dev release-cli checksums

BINARY_NAME := tegata
BUILD_DIR := bin
LDFLAGS := -s -w
MAX_SIZE := 20971520

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/tegata/

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

cross:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/tegata/
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/tegata/
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/tegata/
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/tegata/

check-size: build
	@SIZE=$$(wc -c < $(BUILD_DIR)/$(BINARY_NAME)); \
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
