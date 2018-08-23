# Required for globs to work correctly
SHELL=/bin/bash

DEP_VERSION=0.3.2

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

# go options
GO        ?= go
PKG       := $(shell go list)
TAGS      :=
TESTS     := .
TESTFLAGS :=
LDFLAGS   :=
GOFLAGS   :=


.PHONY: all
all: build test

.PHONY: build
build:
	@mkdir -p _dist
	$(GO) build $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o _dist/$(APP) $(PKG)

.PHONY: run
run:
	$(GO) run $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' main.go $(OPTS)

.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross:
	gox -parallel=3 -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' $(PKG)

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
test: build
test: TESTFLAGS += -race -v
test: test-style
test: test-unit

.PHONY: test-unit
test-unit:
	@echo
	@echo "==> Running unit tests <=="
	$(GO) test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)

.PHONY: test-style
test-style:
	@scripts/validate-go.sh

.PHONY: clean
clean:
	@rm -rf ./_dist

.PHONY: dist-clean
dist-clean: clean
	@rm -rf vendor

HAS_DEP := $(shell command -v dep;)
HAS_GOX := $(shell command -v gox;)
HAS_GIT := $(shell command -v git;)

.PHONY: bootstrap
bootstrap:
ifndef HAS_DEP
	curl -L -s https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 -o ${GOPATH}/bin/dep
	chmod 755 ${GOPATH}/bin/dep
endif
ifndef HAS_GOX
	go get -u github.com/mitchellh/gox
endif
ifndef HAS_GIT
	$(error You must install Git)
endif
	dep ensure -vendor-only