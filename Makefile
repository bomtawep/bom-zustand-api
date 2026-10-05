.PHONY: help dev run build test test-cover vet lint fmt tidy clean install-tools swagger ci all

APP_NAME := api
BIN_DIR := bin
MAIN := cmd/api/main.go

help:
	@echo "Available targets:"
	@echo "  make dev            - run dev server with hot reload (air)"
	@echo "  make run            - run server with go run"
	@echo "  make build          - build binary to bin/$(APP_NAME)"
	@echo "  make test           - run all tests"
	@echo "  make test-cover     - run tests with coverage report (coverage.out, coverage.html)"
	@echo "  make vet            - run go vet"
	@echo "  make lint           - run golangci-lint"
	@echo "  make fmt            - run gofmt and goimports"
	@echo "  make tidy           - run go mod tidy"
	@echo "  make install-tools  - install air, golangci-lint, goimports, swag"
	@echo "  make swagger        - regenerate OpenAPI docs from handler annotations (docs/swagger)"
	@echo "  make ci             - run vet, lint, and test (what CI should run)"
	@echo "  make all            - fmt, vet, test, then build"
	@echo "  make clean          - remove build artifacts"

dev:
	air

run:
	go run $(MAIN)

build:
	go build -o $(BIN_DIR)/$(APP_NAME) $(MAIN)

test:
	go test ./...

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .
	goimports -w .

tidy:
	go mod tidy

install-tools:
	go install github.com/air-verse/air@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest

swagger:
	swag init -g $(MAIN) -o docs/swagger --parseInternal --parseDependency

ci: vet lint test

all: fmt vet test build

clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html
