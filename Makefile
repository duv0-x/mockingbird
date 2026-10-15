# Minimal targets from T0; T6 extends this file (docker, lint, ci).
GO    ?= /opt/homebrew/bin/go
TEMPL ?= templ

.PHONY: generate build test run

generate:
	$(TEMPL) generate

build: generate
	CGO_ENABLED=0 $(GO) build -o bin/mockingbird ./cmd/mockingbird

test:
	$(GO) vet ./...
	$(GO) test ./...

run: generate
	$(GO) run ./cmd/mockingbird
