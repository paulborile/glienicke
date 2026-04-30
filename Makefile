.PHONY: test integrationtest build

test:
	go test ./pkg/... ./internal/...

integrationtest:
	go test ./test/integration/...

build:
	go build -o bin/relay ./cmd/relay
	cp bin/relay glienicke-relay
