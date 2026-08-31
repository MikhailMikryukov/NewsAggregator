APP_NAME=newsaggregator
BUILD_DIR=bin
MAIN_PACKAGE=./cmd/app
LINT_CONFIG=.golangci.yml

.PHONY: lint test build check clean

lint:
	golangci-lint run --config $(LINT_CONFIG) ./...

test:
	go test -race -cover ./...

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PACKAGE)

check: lint test build

clean:
	rm -rf $(BUILD_DIR)