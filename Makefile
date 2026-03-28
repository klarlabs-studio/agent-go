.PHONY: all build test lint clean coverage coverage-check coverage-report \
        security security-secrets security-diff \
        release-plan release-bump release-notes release-publish \
        check example help hooks \
        workspace-sync workspace-tidy contrib-build contrib-test

# Default target
all: build test lint

# Build (workspace-aware)
build:
	go build ./...

# Build core only
build-core:
	go build -mod=mod ./domain/... ./application/... ./interfaces/... ./infrastructure/...

# Build all contrib modules individually
contrib-build:
	@echo "Building contrib modules..."
	@for dir in contrib/*/; do \
		echo "Building $$dir"; \
		(cd "$$dir" && go build ./...) || exit 1; \
	done
	@echo "All contrib modules built successfully"

# Test (workspace-aware)
test:
	go test -race -v ./...

# Test core only
test-core:
	go test -race -v ./domain/... ./application/... ./interfaces/... ./infrastructure/... ./test/...

# Test all contrib modules individually
contrib-test:
	@echo "Testing contrib modules..."
	@for dir in contrib/*/; do \
		echo "Testing $$dir"; \
		(cd "$$dir" && go test -race ./...) || exit 1; \
	done
	@echo "All contrib module tests passed"

# Test with coverage
test-coverage:
	go test -race -coverprofile=coverage.out ./...

# Coverage analysis (coverctl)
coverage-check:
	coverctl check

coverage-report:
	coverctl report

coverage-debt:
	coverctl debt

coverage-suggest:
	coverctl suggest

# Security scanning (nox)
security:
	nox scan . --severity-threshold=high

security-secrets:
	nox scan . --history --history-depth=50 --severity-threshold=high

security-diff:
	nox diff --base=main --head=HEAD

# Release management (relicta)
release-status:
	relicta status

release-plan:
	relicta plan --analyze

release-bump:
	relicta bump --level=auto

release-notes:
	relicta notes --ai

release-approve:
	relicta approve

release-publish:
	relicta publish

release-validate:
	relicta validate-release

# Lint (workspace-aware)
lint:
	golangci-lint run ./...

# Lint core only
lint-core:
	golangci-lint run ./domain/... ./application/... ./interfaces/... ./infrastructure/...

# Clean
clean:
	rm -f coverage.out
	go clean ./...

# Run example
example:
	go run ./example/fileops

# Workspace management
workspace-sync:
	go work sync

workspace-tidy:
	@echo "Tidying all modules..."
	go mod tidy
	@for dir in contrib/*/; do \
		echo "Tidying $$dir"; \
		(cd "$$dir" && go mod tidy) || exit 1; \
	done
	@echo "All modules tidied"

# Verify workspace
workspace-verify:
	@echo "Verifying workspace..."
	go work sync
	go build ./...
	@echo "Workspace verified successfully"

# Install git hooks
hooks:
	@echo "Installing git hooks..."
	@cp scripts/pre-commit .git/hooks/pre-commit
	@cp scripts/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-commit .git/hooks/pre-push
	@echo "Git hooks installed successfully."
	@echo "  pre-commit: gofmt, go vet, golangci-lint, nox, build, core tests"
	@echo "  pre-push:   race tests, coverage check, nox security scan"

# All checks (CI/CD)
check: lint test-coverage coverage-check security

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "  Build & Test:"
	@echo "    build            - Build all packages (workspace)"
	@echo "    build-core       - Build core packages only"
	@echo "    contrib-build    - Build all contrib modules individually"
	@echo "    test             - Run tests with race detection (workspace)"
	@echo "    test-core        - Run core tests only"
	@echo "    contrib-test     - Test all contrib modules individually"
	@echo "    test-coverage    - Run tests with coverage profile"
	@echo ""
	@echo "  Coverage (coverctl):"
	@echo "    coverage-check   - Check coverage meets threshold (80%)"
	@echo "    coverage-report  - Generate coverage report"
	@echo "    coverage-debt    - Show coverage debt by domain"
	@echo "    coverage-suggest - Suggest optimal thresholds"
	@echo ""
	@echo "  Security (nox):"
	@echo "    security         - Run nox security scan (high+ severity)"
	@echo "    security-secrets - Scan git history for leaked secrets"
	@echo "    security-diff    - Show new findings vs main branch"
	@echo ""
	@echo "  Release (relicta):"
	@echo "    release-status   - Show current release state"
	@echo "    release-plan     - Analyze commits and suggest version"
	@echo "    release-bump     - Calculate and set next version"
	@echo "    release-notes    - Generate release notes"
	@echo "    release-approve  - Approve release for publishing"
	@echo "    release-publish  - Execute release (create tags, run plugins)"
	@echo "    release-validate - Run pre-flight validation"
	@echo ""
	@echo "  Workspace Management:"
	@echo "    workspace-sync   - Sync go.work with all modules"
	@echo "    workspace-tidy   - Run go mod tidy on all modules"
	@echo "    workspace-verify - Verify workspace builds correctly"
	@echo ""
	@echo "  Other:"
	@echo "    hooks            - Install pre-commit and pre-push git hooks"
	@echo "    lint             - Run golangci-lint (workspace)"
	@echo "    lint-core        - Run golangci-lint on core only"
	@echo "    clean            - Remove generated files"
	@echo "    example          - Run the fileops example"
	@echo "    check            - Run all CI checks"
	@echo "    help             - Show this help"
