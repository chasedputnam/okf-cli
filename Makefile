.PHONY: build build-all test lint clean demo-bundle

VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY := memphis

# Build for current platform
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/memphis

# Build for all platforms
build-all: clean
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/memphis
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/memphis
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/memphis
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/memphis
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/memphis

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/

# Run the demo
demo: build
	./$(BINARY) demo
