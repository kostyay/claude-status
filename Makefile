.PHONY: all lint test build clean security-check

all: lint test build

lint:
	golangci-lint run

test:
	go test -race -coverprofile=coverage.out ./...

build:
	go build -o claude-status ./cmd/claude-status

clean:
	rm -f claude-status coverage.out

security:
	gitleaks detect --source . --verbose
