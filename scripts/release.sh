#!/bin/bash

# If no tag specified, use date + version otherwise use tag.
if [[ $1x = x ]]; then
    DATE=`date +%Y%m%d`
    VERSION="01"
    TAG=$DATE-$VERSION
else
    TAG=$1
fi

PACKAGE=brclient

SYS="windows-amd64 linux-amd64 linux-arm64 darwin-amd64 darwin-arm64"

for i in $SYS; do
    OS=$(echo $i | cut -f1 -d-)
    ARCH=$(echo $i | cut -f2 -d-)
    echo "Building:" $OS $ARCH
    env CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH \
	go build -o build/$PACKAGE-$OS-$ARCH-$TAG -v -trimpath -tags 'safe,netgo' -ldflags 'buildid=' 
    if [[ $OS = "windows" ]]; then
	mv build/$PACKAGE-$i-$TAG build/$PACKAGE-$i-$TAG.exe
    fi
done

openssl sha256 -r build/* > build/manifest-$PACKAGE-$TAG.txt
