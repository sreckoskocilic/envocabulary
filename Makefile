.PHONY: all build test cover cover-html lint fmt tidy clean release-snapshot install-tools help

GO              ?= go
GOLANGCI_LINT   ?= golangci-lint
GOFUMPT         ?= gofumpt
COVERAGE_FILE   ?= coverage.out
COVERAGE_MIN    ?= 95
BINARY          ?= envocabulary
PKGS            ?= ./...

all: lint test build  ## Run lint, tests, and build (default)

build:  ## Build the binary
	$(GO) build -o $(BINARY) ./cmd/envocabulary

test:  ## Run tests
	$(GO) test -race $(PKGS)

cover:  ## Run tests with coverage; gate on testable surface (excludes *_external.go)
	@$(GO) test -race -covermode=atomic -coverprofile=$(COVERAGE_FILE) $(PKGS) > /dev/null
	@grep -v '/external\.go:\|_external\.go:' $(COVERAGE_FILE) > $(COVERAGE_FILE).gated
	@full=$$($(GO) tool cover -func=$(COVERAGE_FILE) | awk '/total:/ {gsub(/%/,"",$$3); print $$3}'); \
		gated=$$($(GO) tool cover -func=$(COVERAGE_FILE).gated | awk '/total:/ {gsub(/%/,"",$$3); print $$3}'); \
		echo "Coverage:"; \
		echo "  full   (engineering truth, includes *_external.go):  $$full%"; \
		echo "  gated  (testable surface, matches codecov badge):    $$gated%"; \
		echo ""; \
		if awk -v g=$$gated -v m=$(COVERAGE_MIN) 'BEGIN { exit (g+0 < m+0) ? 1 : 0 }'; then \
			echo "✓ gated coverage $$gated% meets minimum $(COVERAGE_MIN)%"; \
		else \
			echo "✗ gated coverage $$gated% is below minimum $(COVERAGE_MIN)%"; \
			rm -f $(COVERAGE_FILE).gated; \
			exit 1; \
		fi
	@rm -f $(COVERAGE_FILE).gated

cover-html: cover  ## Generate HTML coverage report (full picture, includes *_external.go)
	@$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Open coverage.html in your browser to inspect line-by-line coverage."

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
