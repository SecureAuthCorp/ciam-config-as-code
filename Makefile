
.PHONY: build
build:
	go build .

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: install
install:
	go install .

.PHONY: test
test:
	go test ./internal/...

.PHONY: test-e2e
test-e2e:
	go test -tags e2e -timeout 20m -v ./e2e/...

.PHONY: all
all: build lint test install