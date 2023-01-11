#!/bin/sh

if [ $# -ne 3 ]; then
    echo "invalid arguments: logo-path cert-path cert-pass" >&2
    exit 2
fi

LOGOPATH=$1
CERTPATH=$2
CERTPASS=$3


go generate ../../golibbuilder/
flutter clean
flutter pub get
flutter pub run msix:create -l $LOGOPATH -c $CERTPATH -p $CERTPASS

exit 0

