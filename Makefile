REGISTRY ?= ghcr.io
ifndef TAG
	TAG ?= $(shell git rev-parse --short=7 HEAD)
endif
CLUSTER_HEALTH_MONITOR_IMAGE_VERSION ?= $(TAG)
CLUSTER_HEALTH_MONITOR_IMAGE_NAME ?= cluster-health-monitor

## --------------------------------------
## Build
## --------------------------------------

.PHONY: build
build: ## Build binaries.
	go build -o bin/clusterhealthmonitor ./cmd/clusterhealthmonitor

.PHONY: run-clusterhealthmonitor
run-clusterhealthmonitor: ## Run a controllers from your host.
	go run ./cmd/clusterhealthmonitor/main.go

## --------------------------------------
## Images
## --------------------------------------

OUTPUT_TYPE ?= type=registry
BUILDX_BUILDER_NAME ?= img-builder
QEMU_VERSION ?= 7.2.0-1
BUILDKIT_VERSION ?= v0.18.1

.PHONY: push
push:
	$(MAKE) OUTPUT_TYPE="type=registry" docker-build-cluster-health-monitor

# By default, docker buildx create will pull image moby/buildkit:buildx-stable-1 and hit the too many requests error
.PHONY: docker-buildx-builder
docker-buildx-builder:
	@if ! docker buildx ls | grep $(BUILDX_BUILDER_NAME); then \
		docker run --rm --privileged mcr.microsoft.com/mirror/docker/multiarch/qemu-user-static:$(QEMU_VERSION) --reset -p yes; \
		docker buildx create --driver-opt image=mcr.microsoft.com/oss/v2/moby/buildkit:$(BUILDKIT_VERSION) --name $(BUILDX_BUILDER_NAME) --use; \
		docker buildx inspect $(BUILDX_BUILDER_NAME) --bootstrap; \
	fi

.PHONY: docker-build-cluster-health-monitor
docker-build-cluster-health-monitor: docker-buildx-builder
	docker buildx build \
		--file docker/$(CLUSTER_HEALTH_MONITOR_IMAGE_NAME).Dockerfile \
		--output=$(OUTPUT_TYPE) \
		--platform="linux/amd64" \
		--pull \
		--tag $(REGISTRY)/$(CLUSTER_HEALTH_MONITOR_IMAGE_NAME):$(CLUSTER_HEALTH_MONITOR_IMAGE_VERSION) .

## -----------------------------------
## Cleanup
## -----------------------------------

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf ./bin
