# Makefile for audiovisual

.PHONY: build build-audio test fmt lint clean

BINARY := audiovisual
MAIN   := ./cmd/audiovisual

# Build without audio capture (no PortAudio required).
build:
	go build -o $(BINARY) $(MAIN)

# Build with audio capture. Requires PortAudio and pkg-config (e.g. brew install pkg-config portaudio).
build-audio:
	go build -tags=portaudio -o $(BINARY) $(MAIN)

test:
	go test ./...

fmt:
	go fmt ./...

# Run golangci-lint. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
