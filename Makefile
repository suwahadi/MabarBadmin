GO ?= go
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
TEST_DATABASE_URL ?= postgres://mabar:mabar@localhost:5432/mabar_test?sslmode=disable

.PHONY: help hooks assets sqlc fmt lint test test-integration cover dev build release check-commits

help:
	@echo "make hooks | assets | sqlc | lint | test | test-integration | cover | dev | build | release"

hooks:
	git config core.hooksPath .githooks

frontend/node_modules: frontend/package.json frontend/bun.lock
	cd frontend && bun install --frozen-lockfile
	@touch frontend/node_modules

assets: frontend/node_modules
	cd frontend && bun run build

sqlc:
	sqlc generate

fmt:
	gofmt -w cmd internal web

lint:
	@test -z "$$(gofmt -l cmd internal web)" || (gofmt -l cmd internal web; echo "gofmt needed"; exit 1)
	$(GO) vet ./...
	staticcheck ./...

test:
	$(GO) test ./...

test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO) test -tags nodynamic -p 1 -count=1 ./...

cover:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO) test -p 1 -count=1 -coverprofile=coverage.out ./internal/domain/... ./internal/service/...
	$(GO) tool cover -func=coverage.out | tail -1

dev: assets
	set -a; [ -f .env ] && . ./.env; set +a; $(GO) run ./cmd/mabar serve

build: assets
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -tags nodynamic -o dist/mabar ./cmd/mabar

release: assets lint test
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -tags nodynamic -o dist/mabar ./cmd/mabar
	@echo "dist/mabar $(VERSION)"

check-commits:
	sh scripts/check-commit-messages.sh $(BASE) $(HEAD)
