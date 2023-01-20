#!/bin/sh

if [ $# -ne 1 ]; then
    echo "invalid arguments: version" >&2
    exit 2
fi
VERSION=$1
TAR_NAME=bisonrelay-linux-amd64-$VERSION.tar.gz
BUILD_DIR=build/linux/x64/release/bundle
APPIMAGE_DIR=BisonRelayBuild/
APPRUN=../AppRun
DESKTOP_FILE=../BisonRelay.desktop
ICON_FILE=../assets/icons/icon_32x32.png

flutter clean
flutter build linux --release

if [ -d "$APPIMAGE_DIR" ]; then
    printf '%s\n' "Removing Lock ($APPIMAGE_DIR)"
    rm -rf "$APPIMAGE_DIR"
fi
mv $BUILD_DIR build/$APPIMAGE_DIR

cd build

tar -czf $TAR_NAME $APPIMAGE_DIR

cp $APPRUN $APPIMAGE_DIR

chmod +x $APPRUN

cp $DESKTOP_FILE $APPIMAGE_DIR

cp $ICON_FILE $APPIMAGE_DIR/bisonrelay.png

if ! type appimagetool-x86_64.AppImage > /dev/null; then
    printf '%s\n' "appimagetool-x86_64.AppImage (github.com/AppImage/AppImage/Kit) required to be installed for appimage building"
    exit 1
fi

appimagetool-x86_64.AppImage $APPIMAGE_DIR

mv Bison_Relay-x86_64.AppImage BisonRelay-$VERSION.AppImage
