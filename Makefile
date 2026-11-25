.PHONY: all build web site site-dev install dev test lint golangci-lint fmt fmt-check vet ci pre-commit hooks clean coverage help

all: build

help:
	@echo "Targets:"
	@echo "  all         Build frontend then Go binary (default)"
	@echo "  build       Build frontend then Go binary"
	@echo "  install     Install artex-cc to GOPATH/bin"
	@echo "  web         Build frontend to web/dist/"
	@echo "  site        Build the docs site to site/dist/"
	@echo "  site-dev    Run the docs site dev server"
	@echo "  dev         Print instructions for local development"
	@echo "  test        Run Go tests"
	@echo "  lint        Type-check frontend"
	@echo "  fmt         Format Go code"
	@echo "  fmt-check   Check Go formatting (CI)"
	@echo "  vet         Run go vet"
	@echo "  golangci-lint Run golangci-lint"
	@echo "  ci          Run fmt-check + vet + golangci-lint + test"
	@echo "  pre-commit  Run fmt-check + vet + lint (for git hook)"
	@echo "  hooks       Install git pre-commit hook"
	@echo "  coverage    Generate HTML coverage report"
	@echo "  clean       Remove build artifacts"

web: web/node_modules
	cd web && npm run build

web/node_modules: web/package.json web/package-lock.json
	cd web && npm ci
	@touch $@

site/node_modules: site/package.json site/package-lock.json
	cd site && npm ci
	@touch $@

site: site/node_modules
	cd site && npm run build

site-dev: site/node_modules
	cd site && npm run dev

build: web
	go build -o artex-cc ./cmd/artex-cc

install:
	go install ./cmd/artex-cc

dev:
	@echo "Start the Go server:  go run ./cmd/artex-cc serve --web --pg \$$ARTEX_PG --listen :9400"
	@echo "Start Vite dev:       cd web && npm run dev"

test:
	go test ./...

lint: web/node_modules
	cd web && npx tsc -b --noEmit

fmt:
	gofmt -s -w .

fmt-check:
	@out=$$(gofmt -s -l .); [ -z "$$out" ] || { echo "$$out"; exit 1; }

vet:
	go vet ./...

golangci-lint:
	golangci-lint run ./...

ci: fmt-check vet golangci-lint test

pre-commit: fmt-check vet lint

hooks:
	@mkdir -p .git/hooks
	@printf '#!/bin/sh\nmake pre-commit\n' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed."

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

clean:
	rm -rf web/dist site/dist artex-cc
