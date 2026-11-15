BIN := lazynet
PKG := github.com/Camilo-ovalle/Lazy_Net/internal/cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(shell git rev-parse --short HEAD 2>/dev/null) -X $(PKG).Date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build test vet fmt demo run snapshot clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/lazynet

test:
	go test ./...

vet:
	go vet ./...
	test -z "$$(gofmt -l .)"

fmt:
	gofmt -w .

demo: build
	./$(BIN) --demo

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf $(BIN) dist completions
