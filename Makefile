##
## Variables
##

BUILD_DIR := _build
BUILDTOOL := $(BUILD_DIR)/build-tool
BUILDTOOL_MAIN := cmd/build-tool/main.go
BUILDTOOL_PACKAGE := \
		     internal/build-tool/bison.go \
		     internal/build-tool/bpf_asm.go \
		     internal/build-tool/capnproto.go \
		     internal/build-tool/common.go \
		     internal/build-tool/config.go \
		     internal/build-tool/downloads.go \
		     internal/build-tool/flex.go \
		     internal/build-tool/go-capnp.go \
		     internal/build-tool/generate/capnp.go \
		     internal/build-tool/linux.go \
		     internal/build-tool/tinygo.go \
		     internal/build-tool/toolchain.go \

TOOLCHAIN_DIR := ./toolchain
BISON_VERSION := 3.8.2
BISON := $(TOOLCHAIN_DIR)/bison-$(BISON_VERSION)/tests/bison
BPF_ASM_VERSION := 6.13.8
BPF_ASM := $(TOOLCHAIN_DIR)/bpf_asm-$(BPF_ASM_VERSION)/tools/bpf/bpf_asm
CAPNP_VERSION := 1.1.0
CAPNP := $(TOOLCHAIN_DIR)/capnproto-$(CAPNP_VERSION)/capnp
FLEX_VERSION := 2.6.4
FLEX := $(TOOLCHAIN_DIR)/flex-$(FLEX_VERSION)/src/flex
GO_VERSION := 1.25.3
GO := $(TOOLCHAIN_DIR)/go-$(GO_VERSION)/bin/go
GO_BUILD := $(GO) build
GO_GET := $(GO) get
GOCAPNP_VERSION := 3.1.0-alpha.1
GOCAPNP := $(TOOLCHAIN_DIR)/go-capnp-$(GOCAPNP_VERSION)/bin/go-capnp
# GOPATH_DIR to not collide with GOPATH
GOPATH_DIR := $(abspath $(TOOLCHAIN_DIR)/gopath-$(GO_VERSION))
TEMPEST_SANDBOX_LAUNCHER := $(BUILD_DIR)/tempest-sandbox-launcher
TEMPEST_SANDBOX_LAUNCHER_GEN := \
				$(BUILD_DIR)/bpf_filter.h \
				$(BUILD_DIR)/constants.h \
				$(BUILD_DIR)/filter_preproc.s \
				$(BUILD_DIR)/gen-clean-h
TEMPEST_SANDBOX_LAUNCHER_OBJ := c/sandbox-launcher.o
TINYGO_VERSION := 0.37.0
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
clean: clean-tempest-sandbox-launcher
	rm -rf _build
	rm -f \
		go/internal/server/embed/*.wasm \
		c/config.h  \
		go/internal/config/config.go
	# We decided that, for now, we're going to keep these files around.
	#find * -type f -name '*.capnp.go' -delete
	find * -type f -name '*.cgr' -delete
	find * -type d -empty -delete
	rm -f $(BUILDTOOL)

.PHONY: clean-build-tool-cache
clean-build-tool-cache:
	# Used by scripts/bootstrap-build-tool.sh, cmd/build-tool and
	# internal/build-tool
	if [ -n "${HOME}" ]; then rm -rf "${HOME}/.cache/tempest-build-tool"; fi

.PHONY: clean-tempest-sandbox-launcher
clean-tempest-sandbox-launcher:
	rm -f $(TEMPEST_SANDBOX_LAUNCHER)
	rm -f $(TEMPEST_SANDBOX_LAUNCHER_GEN)
	rm -f c/*.o c/*.d

.PHONY: clean-toolchain
clean-toolchain:
	# Go get tends to create files and directories that have a write bit
	# cleared, so rm -rf cannot do it's job.  Fix that with find.
	find "$(TOOLCHAIN_DIR)" ! -perm -u=w -exec chmod 700 \{\} \;
	rm -rf $(TOOLCHAIN_DIR)

.PHONY: nuke
nuke: clean clean-build-tool-cache clean-toolchain
	rm -f config.json

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
# Sandbox Launcher
#
c/%.o: c/%.c
	$(CC) $(CFLAGS) -I $(BUILD_DIR) -std=c11 -Wall -Wextra -MMD -c -o $@ $<

$(TEMPEST_SANDBOX_LAUNCHER): $(BPF_ASM) $(TEMPEST_SANDBOX_LAUNCHER_OBJ)
	$(CC) $(LDFLAGS) -o $@ $(TEMPEST_SANDBOX_LAUNCHER_OBJ)

$(TEMPEST_SANDBOX_LAUNCHER_OBJ): $(BUILD_DIR)/bpf_filter.h

$(BUILD_DIR)/gen-clean-h: c/gen-clean-h.o
	$(CC) $(LDFLAGS) -o $@ $<

$(BUILD_DIR)/constants.h: $(BUILD_DIR)/gen-clean-h
	$(BUILD_DIR)/gen-clean-h > $@

$(BUILD_DIR)/filter_preproc.s: $(BUILD_DIR)/constants.h c/filter.s
	cpp -I $(BUILD_DIR) c/filter.s -o $@

$(BUILD_DIR)/bpf_filter.h: $(BUILD_DIR)/filter_preproc.s
	echo $(BPF_ASM) -c \< $< \> $@
	$(BPF_ASM) -c < $< > $@

-include c/*.d

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
toolchain: $(BISON) $(BPF_ASM) $(CAPNP) $(FLEX) $(GO) $(GOCAPNP) $(TINYGO)

$(BISON): $(BUILDTOOL)
	@echo Building Bison $(BISON_VERSION)
	$(BUILDTOOL) bootstrap-bison

$(BPF_ASM): $(BISON) $(BUILDTOOL) $(FLEX)
	@echo Building bpf_asm from Linux $(BPF_ASM_VERSION)
	$(BUILDTOOL) bootstrap-bpf_asm

$(BUILDTOOL): $(BUILDTOOL_MAIN) $(BUILDTOOL_PACKAGE) $(GO) $(GOPATH_DIR)
	GOPATH="$(GOPATH_DIR)" $(GO_GET) ./internal/build-tool
	GOPATH="$(GOPATH_DIR)" $(GO_BUILD) -o $(BUILDTOOL) $(BUILDTOOL_MAIN)

$(CAPNP): $(BUILDTOOL)
	@echo Building Cap\'n Proto $(CAPNP_VERSION)
	$(BUILDTOOL) bootstrap-capnproto

$(FLEX): $(BUILDTOOL)
	@echo Building Flex $(FLEX_VERSION)
	$(BUILDTOOL) bootstrap-flex

$(GO):
	@echo Setting up Go $(GO_VERSION)
	./scripts/bootstrap-build-tool.sh

$(GOCAPNP): $(BUILDTOOL) $(GOPATH_DIR)
	@echo Setting up Cap\'n Proto for Go
	GOPATH="$(GOPATH_DIR)" $(BUILDTOOL) bootstrap-go-capnp

$(GOPATH_DIR):
	mkdir -p "$(GOPATH_DIR)"

$(TINYGO): $(BUILDTOOL)
	@echo Setting up TinyGo $(TINYGO_VERSION)
	$(BUILDTOOL) bootstrap-tinygo

capnp/%.capnp.go: $(BUILDTOOL) $(CAPNP) $(GOCAPNP) capnp/%.capnp
	$(BUILDTOOL) generate-capnp

internal/capnp/%.capnp.go: $(BUILDTOOL) $(CAPNP) $(GOCAPNP) internal/capnp/%.capnp
	$(BUILDTOOL) generate-capnp
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
