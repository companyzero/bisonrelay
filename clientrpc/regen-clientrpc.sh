#!/bin/sh

BINDIR=$(mktemp -d)

build_protoc_gen_go() {
    mkdir -p $BINDIR
    export GOBIN=$BINDIR
    go install github.com/golang/protobuf/protoc-gen-go
    go install ./internal/protoc-gen-go-svcintf
}

generate() {
    protoc -I. clientrpc.proto --go_out=types --go_opt=paths=source_relative
    protoc -I. clientrpc.proto --go-svcintf_out=types --go-svcintf_opt=paths=source_relative
}


# Build the bins from the main module, so that clientrpc doesn't need to
# require all client and tool dependencies.
(cd .. && build_protoc_gen_go)
GENPATH="$BINDIR:$PATH"
PATH=$GENPATH generate
