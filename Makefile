.PHONY: build run release clean

BIN := ../build/llvm-configure
GO_DIR := src
CMD := ./cmd/llvm-configure

build:
	cd $(GO_DIR) && go build -o $(BIN) $(CMD)

run:
	go run $(CMD)

release:
	./scripts/build_release.sh

clean:
	rm -f $(BIN)
