COMPILEFLAGS=-race -v

# -w reduces binary size.
LDFLAGS=-ldflags="-w"

TEST_PACKAGES=$(shell go list ./... | grep -v /vendor/)

build:
	go build ${COMPILEFLAGS} ${LDFLAGS} ./cmd/...

install:
	go install ${COMPILEFLAGS} ${LDFLAGS} ./cmd/...

test:
	go test ${COMPILEFLAGS} ${TEST_PACKAGES}

godep-save:
	godep save . ./cmd/... ./internal/...

show-env:
	go env

# Ignore any files in the current dir that happen to be named like the targets above.
.PHONY: build install test godep-save show-env
