#!/bin/sh
#
# Wraps the go command to specify the current git SHA.

IMPORT_PATH=github.com/zach-klippenstein/adbfs
BUILD_SHA=$(git rev-parse --verify HEAD | cut -c -7)
GO_CMD="$1"
shift

set -x
go "$GO_CMD" -ldflags "-X $IMPORT_PATH/cli.buildSHA=$BUILD_SHA" "$@"