.PHONY: test
test:
	go test ./...

.PHONY: integration-test
integration-test:
	go run cmd/integration/main.go
