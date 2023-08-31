#!/usr/bin/make

# SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

-include .makerc

ifeq (, $(shell which git))
VERSION ?= dev
DATE ?= $(shell date --rfc-3339=seconds)
else
VERSION := $(shell git describe --tags)
DATE := $(shell git log -1 --format=%cI)
endif

BUILD_TAGS := netgo osusergo sqlite_omit_load_extension sqlite_foreign_keys sqlite_json1 sqlite_fts5 sqlite_secure_delete
VERSION_FLAGS := \
	-X 'codeberg.org/readeck/readeck/configs.version=$(VERSION)' \
	-X 'codeberg.org/readeck/readeck/configs.buildTimeStr=$(DATE)'

OUTFILE_NAME ?= readeck
LDFLAGS ?= -s -w
export CGO_ENABLED ?= 0
export CGO_CFLAGS ?= -D_LARGEFILE64_SOURCE

SITECONFIG_SRC=./ftr-site-config
SITECONFIG_DEST=src/pkg/extract/fftr/site-config/standard

# Build the app
.PHONY: all
all: web-build docs-build build

# Build the server
.PHONY: build
build:
	@echo "CC: $(CC)"
	@echo "CXX: $(CXX)"
	@echo "CGO_ENABLED": $$CGO_ENABLED
	@echo "CGO_CFLAGS": $$CGO_CFLAGS
	@echo "GOOS": $$GOOS
	@echo "GOARCH": $$GOARCH
	go build \
		-v \
		-tags "$(BUILD_TAGS)" \
		-ldflags="$(VERSION_FLAGS) $(LDFLAGS)" -trimpath \
		-o dist/$(OUTFILE_NAME) \
		./src

# Clean the build
.PHONY: clean
clean:
	rm -rf dist
	rm -rf src/assets/www/*
	make -C src/web clean

list:
	CGO_ENABLED=$(CGO_ENABLED) CGO_CFLAGS=$(CGO_CFLAGS) \
	go list \
		-tags "$(BUILD_TAGS)" \
		-ldflags="$(VERSION_FLAGS) $(LDFLAGS)" \
		-f "{{ .GoFiles }}" \
		./src

# Linting
.PHONY: lint
lint:
	cd src && golangci-lint run

# SLOC
.PHONY: sloc
sloc:
	scc -i go,js,sass,html,md

# Launch tests
.PHONY: test
test: CC:=
test: CXX:=
test: CGO_ENABLED=1
test: LDFLAGS:=-s -w
test: docs-build web-build
	@echo "CC: $(CC)"
	@echo "CXX: $(CXX)"
	@echo "CGO_ENABLED": $$CGO_ENABLED
	@echo "CGO_CFLAGS": $$CGO_CFLAGS
	go test \
		-tags "$(BUILD_TAGS)" \
		-ldflags="$(VERSION_FLAGS) $(LDFLAGS)" -trimpath \
		-cover -count=1 ./src/...

# Start the HTTP server
.PHONY: serve
serve:
	modd -f modd.conf

# Update site-config folder with definitions from
# graby git repository
.PHONY: update-site-config
update-site-config:
	rm -rf $(SITECONFIG_DEST)
	go run ./tools/ftr $(SITECONFIG_SRC) $(SITECONFIG_DEST)

.PHONY: dev
dev:
	${MAKE} -j2 web-watch serve

.PHONY: help-build
docs-build:
	${MAKE} -C src/docs all

.PHONY: web-build
web-build:
	@$(MAKE) -C src/web build

.PHONY: web-watch
web-watch:
	@$(MAKE) -C src/web watch


# Setup the development env
.PHONY: setup
setup:
	${MAKE} -C src/web setup
	go install github.com/cortesi/modd/cmd/modd@latest
	go install github.com/boyter/scc/v3@latest


#
# Release targets
#
.PHONY: release-all
release-all: all
	${MAKE} release-linux-amd64
	${MAKE} release-linux-arm
	${MAKE} release-linux-arm64
	${MAKE} release-darwin-amd64
	${MAKE} release-darwin-arm64
	${MAKE} release-windows-amd64

.PHONY: compress_release
compress_release:
	upx -v --best --lzma dist/$(OUTFILE_NAME)
	upx -v -t dist/$(OUTFILE_NAME)

.PHONY: checksum_release
checksum_release:
	sha256sum dist/$(OUTFILE_NAME) > dist/$(OUTFILE_NAME).sha256

.PHONY: release-linux-amd64
release-linux-amd64: CC:=zig cc -target x86_64-linux-musl
release-linux-amd64: CXX:=zig cc -target x86_64-linux-musl
release-linux-amd64: CGO_ENABLED=1
release-linux-amd64: LDFLAGS:=-s -w -linkmode 'external' -extldflags '-static'
release-linux-amd64: export GOOS=linux
release-linux-amd64: export GOARCH=amd64
release-linux-amd64: OUTFILE_NAME:=readeck-$(VERSION)-$(GOOS)-$(GOARCH)
release-linux-amd64: build compress_release checksum_release

.PHONY: release-linux-arm64
release-linux-arm64: CC:=
release-linux-arm64: CXX:=
release-linux-arm64: CGO_ENABLED=0
release-linux-arm64: LDFLAGS:=-s -w
release-linux-arm64: export GOOS=linux
release-linux-arm64: export GOARCH=arm64
release-linux-arm64: OUTFILE_NAME:=readeck-$(VERSION)-$(GOOS)-$(GOARCH)
release-linux-arm64: build compress_release checksum_release

.PHONY: release-linux-arm
release-linux-arm: CC:=
release-linux-arm: CXX:=
release-linux-arm: CGO_ENABLED=0
release-linux-arm: LDFLAGS:=-s -w
release-linux-arm: export GOOS=linux
release-linux-arm: export GOARCH=arm
release-linux-arm: OUTFILE_NAME:=readeck-$(VERSION)-$(GOOS)-$(GOARCH)
release-linux-arm: build compress_release checksum_release

.PHONY: release-darwin-amd64
release-darwin-amd64: CC:=
release-darwin-amd64: CXX:=
release-darwin-amd64: CGO_ENABLED=0
release-darwin-amd64: LDFLAGS:=-s -w
release-darwin-amd64: export GOOS=darwin
release-darwin-amd64: export GOARCH=amd64
release-darwin-amd64: OUTFILE_NAME:=readeck-$(VERSION)-$(GOOS)-$(GOARCH)
release-darwin-amd64: build checksum_release

.PHONY: release-darwin-arm64
release-darwin-arm64: CC:=
release-darwin-arm64: CXX:=
release-darwin-arm64: CGO_ENABLED=0
release-darwin-arm64: LDFLAGS:=-s -w
release-darwin-arm64: export GOOS=darwin
release-darwin-arm64: export GOARCH=arm64
release-darwin-arm64: OUTFILE_NAME:=readeck-$(VERSION)-$(GOOS)-$(GOARCH)
release-darwin-arm64: build checksum_release

.PHONY: release-windows-amd64
release-windows-amd64: CC:=
release-windows-amd64: CXX:=
release-windows-amd64: CGO_ENABLED=0
release-windows-amd64: LDFLAGS:=-s -w
release-windows-amd64: export GOOS=windows
release-windows-amd64: export GOARCH=amd64
release-windows-amd64: OUTFILE_NAME:=readeck-$(VERSION)-$(GOOS)-$(GOARCH).exe
release-windows-amd64: build compress_release checksum_release

.PHONY: release-container-amd64
release-container-amd64:
	docker build \
		--ulimit=nofile=4000 \
		-f Containerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg DATE=$(DATE) \
		-t readeck-release:$(VERSION)
