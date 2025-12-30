.PHONY: all fmt lint test build clean security-check

all: lint test build

fmt:
	goimports -w .

lint:
	golangci-lint run

test:
	go test -race -coverprofile=coverage.out ./...

build:
	go build -o claude-status ./cmd/claude-status

clean:
	rm -f claude-status coverage.out

security:
	@echo "Running gitleaks..."
	@command -v gitleaks >/dev/null 2>&1 && gitleaks detect --source . --verbose || echo "⚠️  gitleaks not installed (brew install gitleaks)"
	@echo "Running trufflehog..."
	@command -v trufflehog >/dev/null 2>&1 && trufflehog git file://. --only-verified --fail || echo "⚠️  trufflehog not installed (brew install trufflehog)"
