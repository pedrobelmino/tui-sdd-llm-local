VERSION ?= 0.1.0-dev
MODULE  := github.com/pedrobelmino/tui-sdd-llm-local
BINARY  := bin/tsll
LDFLAGS := -ldflags "-X $(MODULE)/internal/cmd.version=$(VERSION)"

.PHONY: build install test clean

build:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY) ./cmd/tsll

install: build
	@dest="$${GOBIN:-$$HOME/go/bin}"; \
	mkdir -p "$$dest"; \
	cp $(BINARY) "$$dest/tsll"

test:
	go test ./...

clean:
	rm -rf bin/
