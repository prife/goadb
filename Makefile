# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO ?= go

# test for go module support
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
export GO_BUILD=GO111MODULE=on $(GO) build -mod=mod
export GO_TEST=GO111MODULE=on $(GO) test -mod=mod
else
export GO_BUILD=$(GO) build
export GO_TEST=$(GO) test
endif


GOOS := $(shell $(GO) env GOOS)
ifeq ($(GOOS),windows)
	BIN_EXT := .exe
endif

 #git.woa.com/CloudTesting/CloudServer/taskd
PROJECT := github.com/prife/goadb
BINDIR := /usr/local/bin

VERSION := $(shell git describe --tags --dirty --always)
VERSION := $(VERSION:v%=%)
TARGET := adb

# -s -w
GO_LDFLAGS := -X $(PROJECT)/internal/version.Version=$(VERSION)

BUILD_PATH := $(shell pwd)/build
BUILD_BIN_PATH := $(BUILD_PATH)/bin

define go-build
	$(shell cd `pwd` && $(GO_BUILD) -o $(BUILD_BIN_PATH)/$(shell basename $(1)) $(1))
	@echo > /dev/null
endef

GINKGO := $(BUILD_BIN_PATH)/ginkgo
GOLANGCI_LINT := $(BUILD_BIN_PATH)/golangci-lint
OUTDIR := $(CURDIR)/_output

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'install' - Install binaries to system locations."
	@echo " * 'binaries' - Build emud."
	@echo " * 'clean' - Clean artifacts."


.PHONY: all
all: | goadb_mac goadb_linux_x86 goadb_linux_arm64 goadb.exe ## Build binary

.PHONY: goadb_mac
goadb_mac: goadb_mac_x86 goadb_mac_arm64
	lipo $(patsubst %, $(OUTDIR)/%, $^) -create -output $(CURDIR)/_output/$@

.PHONY: goadb_mac_x86
goadb_mac_x86:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(CURDIR)/_output/$@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(PROJECT)/cmd/$(TARGET)

.PHONY: goadb_mac_arm64
goadb_mac_arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO_BUILD) -o $(CURDIR)/_output/$@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(PROJECT)/cmd/$(TARGET)

## for linux
.PHONY: goadb_linux_x86
goadb_linux_x86:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO_BUILD) -o $(CURDIR)/_output/$@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(PROJECT)/cmd/$(TARGET)

.PHONY: goadb_linux_arm64
goadb_linux_arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(CURDIR)/_output/$@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(PROJECT)/cmd/$(TARGET)

# for windows
.PHONY: goadb.exe
goadb.exe:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(CURDIR)/_output/$@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(PROJECT)/cmd/$(TARGET)

clean:
	find . -name \*~ -delete
	find . -name \#\* -delete
	rm -rf _output/*


tidy:
	export GO111MODULE=on \
		&& $(GO) mod tidy
