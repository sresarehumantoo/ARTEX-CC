.PHONY: web build clean

web:
	cd web && npm run build

build: web
	go build ./cmd/cc-artex

clean:
	rm -rf web/dist
	rm -f cc-artex
