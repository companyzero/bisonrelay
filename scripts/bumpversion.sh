#!/bin/bash

set -e

if [[ $1x = x ]]; then
    echo "Version not specified"
    exit 1
fi

VERSION="$1"
VERSION_NB_LIST=${VERSION//./,}

sed -Ei 's/Version = ".+"/Version = "'$VERSION'"/' brclient/internal/version/version.go
sed -Ei 's/Version = ".+"/Version = "'$VERSION'"/' brserver/internal/version/version.go
sed -Ei 's/version: .+/version: '$VERSION'/' bruig/flutterui/bruig/pubspec.yaml
sed -Ei 's/msix_version: .+/msix_version: '$VERSION'.0/' bruig/flutterui/bruig/pubspec.yaml
sed -Ei 's/VERSION_AS_STRING ".+"/VERSION_AS_STRING "'$VERSION'"/' bruig/flutterui/bruig/windows/runner/Runner.rc
sed -Ei 's/VERSION_AS_NUMBER [0-9,]+/VERSION_AS_NUMBER '$VERSION_NB_LIST'/' bruig/flutterui/bruig/windows/runner/Runner.rc
