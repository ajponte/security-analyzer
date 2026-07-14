# update app name. this is the name of binary
APP=security-analyzer
APP_EXECUTABLE="./out/$(APP)"

## Build
build: ## build the go application
	mkdir -p out/
	go build -o $(APP_EXECUTABLE)
	@echo "Build passed"

clean: ## cleans binary and other generated files
	go clean
	rm -rf out/
	rm -f coverage*.out




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
