GO ?= go
VERSION := $(shell cat VERSION)

.PHONY: build test check demo install package

build:
	$(GO) build -trimpath -ldflags '-X main.version=$(VERSION)' -o bin/storepath ./cmd/storepath

test:
	$(GO) test -race ./...

check:
	test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))"
	python3 -m unittest discover -s scripts -p 'test_*.py'
	python3 scripts/check-public-source.py
	$(GO) vet ./...
	$(GO) test -race ./...

demo: build
	./bin/storepath demo

install:
	./scripts/install.sh

package:
	python3 scripts/package.py
