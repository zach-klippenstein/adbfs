#!/bin/sh

export GO15VENDOREXPERIMENT=1

go install -v ./cmd/...
