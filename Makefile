SHELL = bash

# Environment

GIT_BRANCH = $(or $(CI_BUILD_REF_NAME) ,`git rev-parse --abbrev-ref HEAD 2>/dev/null`)
GIT_COMMIT = $(or $(CI_BUILD_REF), `git rev-parse HEAD 2>/dev/null`)
GIT_TAG = $(shell git describe --abbrev=0 --tags 2>/dev/null)
BUILD_DATE = $(or $(CI_BUILD_DATE), `date -u +%Y-%m-%dT%H:%M:%SZ`)

ifeq ($(GIT_BRANCH), $(GIT_TAG))
	TTN_VERSION = $(GIT_TAG)
	TAGS += prod
else
	TTN_VERSION = $(GIT_TAG)-dev
	TAGS += dev
endif

# All

.PHONY: all deps dev-deps test cover-clean cover-deps cover coveralls fmt vet build dev link docs clean docker

all: deps build

# Deps

deps:
	go mod download

dev-deps: deps
	@command -v forego > /dev/null || go get github.com/ddollar/forego

# Go Test

GO_FILES = $(shell find . -name "*.go" | grep -vE ".git|.env|vendor")

test: $(GO_FILES)
	go test -v ./...

GO_COVER_FILE ?= coverage.out

cover-clean:
	rm -f $(GO_COVER_FILE)

cover-deps:
	@command -v goveralls > /dev/null || go get github.com/mattn/goveralls

cover:
	go test -cover -coverprofile=$(GO_COVER_FILE) ./...

coveralls: cover-deps $(GO_COVER_FILE)
	goveralls -coverprofile=$(GO_COVER_FILE) -service=travis-ci -repotoken $$COVERALLS_TOKEN

fmt:
	go fmt ./...

vet:
	go vet ./...

# Go Build

RELEASE_DIR ?= release
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOEXE = $(shell GOOS=$(GOOS) GOARCH=$(GOARCH) go env GOEXE)
CGO_ENABLED ?= 0

splitfilename = $(subst ., ,$(subst -, ,$(subst $(RELEASE_DIR)/,,$1)))
GOOSfromfilename = $(word 4, $(call splitfilename, $1))
GOARCHfromfilename = $(word 5, $(call splitfilename, $1))
LDFLAGS = -ldflags "-w -X main.gitBranch=${GIT_BRANCH} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}"
GOBUILD = CGO_ENABLED=$(CGO_ENABLED) GOOS=$(call GOOSfromfilename, $@) GOARCH=$(call GOARCHfromfilename, $@) go build ${LDFLAGS} -tags "${TAGS}" -o "$@"

build: $(RELEASE_DIR)/gateway-connector-bridge-$(GOOS)-$(GOARCH)$(GOEXE)

$(RELEASE_DIR)/gateway-connector-bridge-%: $(GO_FILES)
	$(GOBUILD) .

install:
	go install -v

# Clean

clean:
	[ -d $(RELEASE_DIR) ] && rm -rf $(RELEASE_DIR) || [ ! -d $(RELEASE_DIR) ]

# Docker

docker: GOOS=linux
docker: GOARCH=amd64
docker: $(RELEASE_DIR)/gateway-connector-bridge-linux-amd64
	docker build -t thethingsnetwork/gateway-connector-bridge -f Dockerfile .
