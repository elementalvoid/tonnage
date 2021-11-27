# Required for globs to work correctly
SHELL=/bin/bash

TARGETS           ?= darwin/amd64 linux/amd64 linux/386 windows/amd64
DIST_DIRS         = find * -type d -exec
APP               = tonnage

GIT_SHA    = $(shell git rev-parse --short HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

VERSION ?= ${GIT_TAG}

# hack for including VERSION in the release filename only when set
FILE_VERSION :=
ifneq ($(VERSION),)
	FILE_VERSION = $(VERSION)-
endif

# Tool versions
GOX_VERSION=v1.0.1
GOLANGCI_LINT_VERSION=v1.43.0

# go options
PKG       := $(shell go list)
TAGS      :=
TESTS     := .
TESTFLAGS :=
LDFLAGS   :=
GOFLAGS   :=


.PHONY: all
all: build

.PHONY: build
build:
	go build

.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross:
	$$(go env GOPATH)/bin/gox -parallel=3 -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' $(PKG)

.PHONY: dist
dist: build-cross
	( \
		cd _dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) tar -zcf $(APP)-$(FILE_VERSION){}.tar.gz {} \; \
	)

.PHONY: checksum
checksum: dist
	for f in _dist/*.gz ; do \
		shasum -a 256 "$${f}"  | awk '{print $$1}' > "$${f}.sha256" ; \
	done

.PHONY: test
test: TESTFLAGS += -race -v
test: test-linter
test: test-unit

.PHONY: test-unit
test-unit:
	@echo
	@echo "==> Running unit tests <=="
	go test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)

.PHONY: test-linter
test-linter:
	@echo "==> Running linter <=="
	$(shell go env GOPATH)/bin/golangci-lint run

.PHONY: clean
clean:
	@rm -rf ./_dist

HAS_GIT := $(shell command -v git;)

.PHONY: bootstrap
bootstrap:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	go install github.com/mitchellh/gox@$(GOX_VERSION)
ifndef HAS_GIT
	$(error You must install Git)
endif
