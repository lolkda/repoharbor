.PHONY: dev web build run test fmt

web:
	cd web && pnpm install && pnpm build

build: web
	go build -o bin/repoharbor ./cmd/repoharbor

run:
	REPOHARBOR_DATA_DIR=./data REPOHARBOR_ADDR=:8080 go run ./cmd/repoharbor

dev:
	@echo "Run in two terminals:"
	@echo "  cd web && pnpm dev"
	@echo "  REPOHARBOR_DATA_DIR=./data REPOHARBOR_ADDR=:8080 go run ./cmd/repoharbor"

test:
	go test ./...
	cd web && pnpm build

fmt:
	gofmt -w cmd internal *.go
