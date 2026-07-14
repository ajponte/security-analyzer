# update app name. this is the name of binary
APP=security-analyzer
APP_EXECUTABLE="./out/$(APP)"

# run goimports formatting from url.
GO_IMPORTS_FMT := $(shell go env GOPATH)/bin/goimports
GOLANGCI_LINT ?= golangci-lint


## Build
build: ## build the go application
	mkdir -p out/
	go build -o $(APP_EXECUTABLE)
	@echo "Build passed"

run: build ## build and run the go application
	$(APP_EXECUTABLE)

fmt: ## runs go formatters
	$(GO_IMPORTS_FMT) -w .
	# go fmt ./...

lint: ## lint the go code using golangci-lint
	${GOLANGCI_LINT} run


clean: ## cleans binary and other generated files
	go clean
	rm -rf out/
	rm -f coverage*.out

vet: ## go vet
	go vet ./...


tidy: ## runs tidy to fix go.mod dependencies
	go mod tidy




.PHONY: help
## Help
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)
