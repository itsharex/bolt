.PHONY: build test test-race test-v clean

BINARY = bolt

build:
	go build -o $(BINARY) ./cmd/bolt/

test:
	go test ./... -count=1 -timeout 120s

test-race:
	go test ./... -race -count=1 -timeout 120s

test-v:
	go test ./... -v -count=1 -timeout 120s

clean:
	rm -f $(BINARY)
	go clean -testcache
