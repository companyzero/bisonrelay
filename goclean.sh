#!/bin/bash

set -ex

# run tests
env go test ./...

# check linters
golangci-lint run

# check client protobuf linters
(cd clientrpc && protolint lint .)

