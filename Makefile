.PHONY: build clean

# Build the cluster health monitor
build:
	go build -o bin/clusterhealthmonitor ./cmd/clusterhealthmonitor

# Clean build artifacts
clean:
	rm -rf bin/

# Create bin directory if it doesn't exist
bin:
	mkdir -p bin
