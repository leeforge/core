.PHONY: generate test lint tidy

GREEN  = \033[0;32m
YELLOW = \033[0;33m
NC     = \033[0m

GO_ENV         = GOWORK=off GOCACHE=/tmp/go-build-cache
GO_GENERATE_ENV = GOFLAGS=-mod=mod

## generate: Run Ent code generation (commit the output)
generate:
	@echo "$(GREEN)Generating core Ent code...$(NC)"
	@$(GO_ENV) $(GO_GENERATE_ENV) go generate ./...
	@echo "$(GREEN)Done. Commit the generated files.$(NC)"

## test: Run all tests
test:
	@echo "$(GREEN)Running core tests...$(NC)"
	@$(GO_ENV) go test ./...

## lint: Run go vet
lint:
	@echo "$(GREEN)Running go vet...$(NC)"
	@$(GO_ENV) go vet ./...

## tidy: Tidy go.mod / go.sum
tidy:
	@echo "$(GREEN)Tidying modules...$(NC)"
	@$(GO_ENV) go mod tidy

help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
