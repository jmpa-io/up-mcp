.PHONY: build run help

## build: compile the MCP server binary into dist/
build:
	mkdir -p dist
	go build -o dist/up-mcp .

## run: run the MCP server directly (requires UP_TOKEN env var)
run:
	go run .

## test: run all tests
test:
	go test ./...

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
