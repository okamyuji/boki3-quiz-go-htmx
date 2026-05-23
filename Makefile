.PHONY: all build run test lint quality-gate clean fmt

GO ?= go
BIN := bin/boki3-quiz

all: quality-gate

build:
	mkdir -p bin
	$(GO) build -o $(BIN) ./cmd/boki3-quiz

run: build
	./$(BIN)

test:
	$(GO) test --count=1 --shuffle=on -race -cover ./...

lint:
	golangci-lint run --timeout 5m ./...

fmt:
	$(GO) fmt ./...

quality-gate:
	bash scripts/quality-gate.sh

clean:
	rm -rf bin coverage.txt coverage.html
