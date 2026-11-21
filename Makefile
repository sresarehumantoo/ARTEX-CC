.PHONY: web build clean dev test lint help

help:
	@echo "Targets:"
	@echo "  build   Build frontend then Go binary (default)"
	@echo "  web     Build frontend to web/dist/"
	@echo "  dev     Print instructions for local development"
	@echo "  test    Run Go tests"
	@echo "  lint    Type-check frontend"
	@echo "  clean   Remove build artifacts"

web: web/node_modules
	cd web && npm run build

web/node_modules: web/package.json web/package-lock.json
	cd web && npm ci
	@touch $@

build: web
	go build -o cc-artex ./cmd/cc-artex

dev:
	@echo "Start the Go server:  go run ./cmd/cc-artex serve --web --pg \$$CC_ARTEX_PG --listen :9400"
	@echo "Start Vite dev:       cd web && npm run dev"

test:
	go test ./...

lint: web/node_modules
	cd web && npx tsc -b --noEmit

clean:
	rm -rf web/dist cc-artex
