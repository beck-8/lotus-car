subdir = $(shell ls -l | grep ^d | awk '{print $$9}')

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X github.com/minerdao/lotus-car/version.Version=$(VERSION) \
           -X github.com/minerdao/lotus-car/version.Commit=$(COMMIT) \
           -X github.com/minerdao/lotus-car/version.Date=$(DATE) \
           -s -w

.PHONY: build all test clean release

all: build build-unix build-ubuntu

build:
	go build -ldflags "$(LDFLAGS)" -o lotus-car ./*.go

build-unix:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o lotus-car-darwin-amd64 ./*.go

build-ubuntu:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o lotus-car-linux-amd64 ./*.go

install:
	install -C ./lotus-car /usr/local/bin/lotus-car

test:
	bundle2.7 exec rspec -f d

clean:
	rm -rfv ./lotus-car ./lotus-car-linux-amd64 ./lotus-car-darwin-amd64

# Release targets
.PHONY: release-patch release-minor release-major
release-patch: ## Release a patch version (0.0.X)
	@echo "Releasing patch version..."
	$(eval NEW_VERSION := $(shell scripts/bump_version.sh patch))
	@make release-common NEW_VERSION=$(NEW_VERSION)

release-minor: ## Release a minor version (0.X.0)
	@echo "Releasing minor version..."
	$(eval NEW_VERSION := $(shell scripts/bump_version.sh minor))
	@make release-common NEW_VERSION=$(NEW_VERSION)

release-major: ## Release a major version (X.0.0)
	@echo "Releasing major version..."
	$(eval NEW_VERSION := $(shell scripts/bump_version.sh major))
	@make release-common NEW_VERSION=$(NEW_VERSION)

release-common:
	@echo "Creating release $(NEW_VERSION)..."
	@git tag -a $(NEW_VERSION) -m "Release $(NEW_VERSION)"
	@echo "Building release binaries..."
	@make clean
	@make all
	@echo "Creating release archive..."
	@mkdir -p releases/$(NEW_VERSION)
	@cp lotus-car-darwin-amd64 releases/$(NEW_VERSION)/
	@cp lotus-car-linux-amd64 releases/$(NEW_VERSION)/
	@cd releases/$(NEW_VERSION) && \
		tar -czf lotus-car-$(NEW_VERSION)-darwin-amd64.tar.gz lotus-car-darwin-amd64 && \
		tar -czf lotus-car-$(NEW_VERSION)-linux-amd64.tar.gz lotus-car-linux-amd64
	@echo "Release $(NEW_VERSION) created!"
