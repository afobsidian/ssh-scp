APP_NAME := ssh-scp
MAIN     := ./cmd/main.go
PREFIX   ?= /usr/local

.PHONY: build install lint fmt test coverage clean

## build: compile the binary
build:
	go build -o ./bin/$(APP_NAME) $(MAIN)

## install: install to PREFIX/bin (default /usr/local/bin)
install: build
	install -d $(PREFIX)/bin
	install -m 755 ./bin/$(APP_NAME) $(PREFIX)/bin/$(APP_NAME)

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## fmt: format all Go source files
fmt:
	go fmt ./...

## test: run all tests
test:
	go test ./...

## coverage: run tests with coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML report: go tool cover -html=coverage.out"

## clean: remove build artifacts
clean:
	rm -f ./bin/$(APP_NAME) coverage.out
