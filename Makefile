.PHONY: all build test cover cover-html lint fmt tidy clean release-snapshot install-tools help

GO              ?= go
GOLANGCI_LINT   ?= golangci-lint
GOFUMPT         ?= gofumpt
COVERAGE_FILE   ?= coverage.out
COVERAGE_MIN    ?= 90
BINARY          ?= envocabulary
PKGS            ?= ./...

all: lint test build  ## Run lint, tests, and build (default)

build:  ## Build the binary
	$(GO) build -o $(BINARY) ./cmd/envocabulary

test:  ## Run tests
	$(GO) test -race $(PKGS)

cover:  ## Run tests with coverage, print summary
	$(GO) test -race -covermode=atomic -coverprofile=$(COVERAGE_FILE) $(PKGS)
	@$(GO) tool cover -func=$(COVERAGE_FILE) | tail -1
	@total=$$($(GO) tool cover -func=$(COVERAGE_FILE) | awk '/total:/ {gsub(/%/,"",$$3); print int($$3)}'); \
		if [ "$$total" -lt "$(COVERAGE_MIN)" ]; then \
			echo ""; \
			echo "✗ coverage $$total% is below minimum $(COVERAGE_MIN)%"; \
			exit 1; \
		else \
			echo "✓ coverage $$total% meets minimum $(COVERAGE_MIN)%"; \
		fi

cover-html: cover  ## Generate HTML coverage report and open it
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "open coverage.html"

lint:  ## Run golangci-lint
	$(GOLANGCI_LINT) run $(PKGS)

fmt:  ## Format code with gofumpt
	$(GOFUMPT) -w .

tidy:  ## Tidy go.mod
	$(GO) mod tidy

clean:  ## Remove build artifacts
	rm -f $(BINARY) $(COVERAGE_FILE) coverage.html
	rm -rf dist/

release-snapshot:  ## Build a local snapshot release with goreleaser (requires goreleaser installed)
	goreleaser release --snapshot --clean --skip=publish

install-tools:  ## Install dev tooling (golangci-lint, gofumpt, goreleaser)
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install mvdan.cc/gofumpt@latest
	$(GO) install github.com/goreleaser/goreleaser/v2@latest

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'
