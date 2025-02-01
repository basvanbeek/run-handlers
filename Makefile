GO_MODULES := $(shell find . -mindepth 1 -maxdepth 1 -type d)

all: deps lint build test

deps:
	@for dir in $(GO_MODULES); do \
		if [ -f $$dir/go.mod ]; then \
			echo "Installing dependencies in $$dir"; \
			(cd $$dir && go mod tidy); \
		fi \
	done

lint:
	@for dir in $(GO_MODULES); do \
		if [ -f $$dir/go.mod ]; then \
			echo "Running golangci-lint in $$dir"; \
			(cd $$dir && golangci-lint run); \
		fi \
	done

build:
	@for dir in $(GO_MODULES); do \
		if [ -f $$dir/go.mod ]; then \
			echo "Building $$dir"; \
			(cd $$dir && go build); \
		fi \
	done

test:
	@for dir in $(GO_MODULES); do \
		if [ -f $$dir/go.mod ]; then \
			echo "Running tests in $$dir"; \
			(cd $$dir && go test); \
		fi \
	done