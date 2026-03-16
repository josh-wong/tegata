.PHONY: build test lint cross check-size clean

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

clean:
	rm -rf $(BUILD_DIR)
