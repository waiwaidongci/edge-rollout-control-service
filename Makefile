.PHONY: fmt vet test build run verify lines

fmt:
	gofmt -w $$(find cmd internal api -type f -name '*.go')

vet:
	go vet ./...

test:
	go test ./...

build:
	go build ./...

run:
	go run ./cmd/edge-rollout -config configs/config.yaml

verify:
	./scripts/run-dev.sh

lines:
	find cmd internal api -type f -name '*.go' ! -name '*_test.go' -print0 | xargs -0 wc -l | tail -1
