##
## Variables
##

BUILDTOOL := _build/build-tool
BUILDTOOL_MAIN := cmd/build-tool/main.go
BUILDTOOL_PACKAGE := \
		     internal/build-tool/common.go \
		     internal/build-tool/config.go \
		     internal/build-tool/downloads.go \
		     internal/build-tool/tinygo.go \

TOOLCHAIN_DIR := ./toolchain
GO_VERSION := 1.23.3
GO := $(TOOLCHAIN_DIR)/go-$(GO_VERSION)/bin/go
GO_BUILD := $(GO) build
GO_GET := $(GO) get
TINYGO_VERSION := 0.35.0
TINYGO := $(TOOLCHAIN_DIR)/tinygo-$(TINYGO_VERSION)/bin/tinygo

##
## Targets
##

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo
	@echo Targets:
	@echo "    build        Build the project"
	@echo "    check        Run project tests"
	@echo "    clean        Remove build artifacts"
	@echo "    format       Format the source files"
	@echo "    lint         Run the linters"
	@echo "    nuke         Remove build artifacts and configuration"
	@echo "    toolchain    Download and set up the toolchain"
	@echo "    update-deps  Update depedencies"
	@echo

.PHONY: all
all: build

#
# Clean Targets
#

.PHONY: clean
clean:
	cd c && $(MAKE) clean
	rm -rf _build
	rm -f \
		go/internal/server/embed/*.wasm \
		c/config.h  \
		go/internal/config/config.go
	find * -type f -name '*.capnp.go' -delete
	find * -type f -name '*.cgr' -delete
	find * -type d -empty -delete
	rm -f $(BUILDTOOL)

.PHONY: clean-toolchain
clean-toolchain:
	rm -rf $(TOOLCHAIN_DIR)

.PHONY: nuke
nuke: clean clean-toolchain
	rm -f config.json
	# Used by scripts/bootstrap-build-tool.sh
	if [ -n "${HOME}" ]; then rm -rf "${HOME}/.cache/tempest-build-tool"; fi

#
# Development Targets
#

.PHONY: format
format:
	shfmt --write scripts/bootstrap-build-tool.sh
	gofmt -l -w $(BUILDTOOL_MAIN) $(BUILDTOOL_PACKAGE)

.PHONY: lint
lint:
	shellcheck scripts/bootstrap-build-tool.sh

#
# Tempest Target
#

.PHONY: build install dev test-app export-import
build install dev test-app export-import:
	@# Just shell out to make.go.
	go run internal/make/make.go $@

#
# Test Targets
#

.PHONY: check
check: all
	./scripts/run-tests.sh

#
# Toolchain Targets
#

.PHONY: toolchain
toolchain: $(GO) $(TINYGO)

$(BUILDTOOL): $(GO)
	$(GO_GET) ./internal/build-tool
	$(GO_BUILD) -o $(BUILDTOOL) $(BUILDTOOL_SOURCE)

$(GO):
	@echo Setting up Go $(GO_VERSION)
	./scripts/bootstrap-build-tool.sh

$(TINYGO): $(BUILDTOOL)
	@echo Setting up TinyGo $(TINYGO_VERSION)
	$(BUILDTOOL) bootstrap-tinygo

#
# Update Targets
#

.PHONY: update-deps
update-deps:
	# Update the versions of these in go.mod:
	go get capnproto.org/go/capnp/v3
	go get zenhack.net/go/util
	go get zenhack.net/go/tea
	go get zenhack.net/go/websocket-capnp
	# and clean up:
	go mod tidy
