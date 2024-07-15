#!/bin/sh

BINDIR=$(mktemp -d)

build_protoc_gen_go() {
    mkdir -p $BINDIR
    export GOBIN=$BINDIR
    go install github.com/golang/protobuf/protoc-gen-go
}

generate() {
    protoc -I. pluginrpc.proto \
      --go_out=grpctypes --go_opt=paths=source_relative \
      --go-grpc_out=grpctypes --go-grpc_opt=paths=source_relative
}


# Build the bins from the main module, so that clientrpc doesn't need to
# require all client and tool dependencies.
(cd .. && build_protoc_gen_go)
GENPATH="$BINDIR:$PATH"
PATH=$GENPATH generate
