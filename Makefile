.PHONY: build build-darwin build-darwin-arm64 build-darwin-amd64 run release clean

BIN := ../build/llvm-configure
GO_DIR := src
CMD := ./cmd/llvm-configure

build:
	cd $(GO_DIR) && go build -o $(BIN) $(CMD)

# Cross-build the CLI binary for macOS (CGO_ENABLED=0 keeps this possible).
# This builds the llvm-configure tool itself for macOS hosts; the LLVM
# projects it generates still build natively on a Mac.
build-darwin: build-darwin-arm64 build-darwin-amd64

build-darwin-arm64:
	cd $(GO_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -ldflags="-s -w" -o ../build/llvm-configure-darwin-arm64 $(CMD)

build-darwin-amd64:
	cd $(GO_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build -ldflags="-s -w" -o ../build/llvm-configure-darwin-amd64 $(CMD)

run:
	cd $(GO_DIR) && go run $(CMD) $(ARGS)

release:
	./scripts/build_release.sh

clean:
	rm -f build/llvm-configure build/llvm-configure-darwin-arm64 build/llvm-configure-darwin-amd64