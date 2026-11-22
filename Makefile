.PHONY: all build web install dev test lint fmt fmt-check vet ci clean coverage help

all: build

help:
	@echo "Targets:"
	@echo "  all         Build frontend then Go binary (default)"
	@echo "  build       Build frontend then Go binary"
	@echo "  install     Install cc-artex to GOPATH/bin"
	@echo "  web         Build frontend to web/dist/"
	@echo "  dev         Print instructions for local development"
	@echo "  test        Run Go tests"
	@echo "  lint        Type-check frontend"
	@echo "  fmt         Format Go code"
	@echo "  fmt-check   Check Go formatting (CI)"
	@echo "  vet         Run go vet"
	@echo "  ci          Run fmt-check + vet + test"
	@echo "  coverage    Generate HTML coverage report"
	@echo "  clean       Remove build artifacts"

web: web/node_modules
	cd web && npm run build

web/node_modules: web/package.json web/package-lock.json
	cd web && npm ci
	@touch $@

build: web
	go build -o cc-artex ./cmd/cc-artex

install:
	go install ./cmd/cc-artex

dev:
	@echo "Start the Go server:  go run ./cmd/cc-artex serve --web --pg \$$CC_ARTEX_PG --listen :9400"
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

ci: fmt-check vet test

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

clean:
	rm -rf web/dist cc-artex
