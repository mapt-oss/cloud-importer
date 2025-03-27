VERSION ?= 0.0.1
CONTAINER_MANAGER ?= podman
# Image URL to use all building/pushing image targets
IMG ?= quay.io/devtools-qe-incubator/cloud-importer:v${VERSION}

# Go and compilation related variables
GOPATH ?= $(shell go env GOPATH)
BUILD_DIR ?= out
SOURCE_DIRS = cmd pkg
# https://golang.org/cmd/link/
LDFLAGS := $(VERSION_VARIABLES) ${GO_EXTRA_LDFLAGS}
GCFLAGS := all=-N -l

# Add default target
.PHONY: default
default: install

# Create and update the vendor directory
.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

.PHONY: check
check: build test lint

# Start of the actual build targets

.PHONY: install
install: $(SOURCES)
	go install -ldflags="$(LDFLAGS)" $(GO_EXTRA_BUILDFLAGS) ./cmd/importer

$(BUILD_DIR)/cloud-importer: $(SOURCES)
	GOOS=linux GOARCH=amd64 go build -gcflags="$(GCFLAGS)" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/cloud-importer $(GO_EXTRA_BUILDFLAGS) ./cmd/importer

.PHONY: build 
build: clean $(BUILD_DIR)/cloud-importer

.PHONY: test
test:
	go test -race --tags build -v -ldflags="$(VERSION_VARIABLES)" ./pkg/... ./cmd/...

.PHONY: clean ## Remove all build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(GOPATH)/bin/cloud-import

.PHONY: fmt
fmt:
	@gofmt -l -w $(SOURCE_DIRS)

$(GOPATH)/bin/golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.6

# Run golangci-lint against code
.PHONY: lint
lint: $(GOPATH)/bin/golangci-lint
	$(GOPATH)/bin/golangci-lint run

# Build for amd64 architecture only
.PHONY: oci-build-amd64
oci-build-amd64: clean
	# Build the container image for amd64
	${CONTAINER_MANAGER} build --platform linux/amd64 --manifest $(IMG)-amd64 -f oci/Containerfile .

# Build for arm64 architecture only
.PHONY: oci-build-arm64
oci-build-arm64: clean
	# Build the container image for arm64
	${CONTAINER_MANAGER} build --platform linux/arm64 --manifest $(IMG)-arm64 -f oci/Containerfile .

