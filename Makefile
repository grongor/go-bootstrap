.PHONY: check
check: static-analysis test

.PHONY: static-analysis
static-analysis:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run

.PHONY: fix
fix:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fix

.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	go test --timeout 5m --count 1 ./...
	go test --timeout 5m --count 1 --race ./...
	go test --timeout 5m --count 100 ./...
