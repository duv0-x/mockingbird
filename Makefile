# mockingbird — build targets (SPEC.md §10, T6).
GO      ?= /opt/homebrew/bin/go
TEMPL   ?= templ
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
IMAGE   ?= mockingbird:$(VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: generate build test lint run docker clean

# templ generate must run from the repo root (paths are baked into *_templ.go).
generate:
	$(TEMPL) generate

build: generate
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/mockingbird ./cmd/mockingbird

test:
	$(GO) vet ./...
	$(GO) test ./...

# gofmt check + vet; fix unformatted files with `gofmt -w .`.
lint:
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	$(GO) vet ./...

run: generate
	$(GO) run ./cmd/mockingbird

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

clean:
	rm -rf bin/
