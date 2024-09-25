import 'package:ffi/ffi.dart';
import 'dart:ffi';
import 'dart:io';
import 'package:path/path.dart' as path;

// The following definitions are for the functions exported by the dynamic
// library. The first one is the dart function (suffix *Func) and the second one
// is the native function (suffice *Native).
//
// When a single definition is sufficient, only the *Native version is defined.

typedef SetTagFunc = void Function(Pointer<Utf8>);
typedef SetTagNative = Void Function(Pointer<Utf8>);

typedef HelloFunc = void Function();
typedef HelloNative = Void Function();

final class GetURLResultNative extends Struct {
  external Pointer<Utf8> res;
  external Pointer<Utf8> err;
}

typedef GetURLNative = GetURLResultNative Function(Pointer<Utf8>);

typedef NextTimeNative = Pointer<Utf8> Function();

typedef WriteStrFunc = void Function(Pointer<Utf8>);
typedef WriteStrNative = Void Function(Pointer<Utf8>);

typedef ReadStrNative = Pointer<Utf8> Function();

typedef AsyncCallFunc = void Function(
    int typ, int id, int clientHandle, Pointer<Utf8> payload, int payloadLen);
typedef AsyncCallNative = Void Function(Uint32 typ, Uint32 id,
    Uint32 clientHandle, Pointer<Utf8> payload, Uint32 payloadLen);

final class NextCallResultReturnType extends Struct {
  @Uint64()
  external int handle;
  @Uint64()
  external int payloadLen;
  @Uint64()
  external int cmdType;
  @Uint64()
  external int isErr;
}

typedef NextCallResultNative = NextCallResultReturnType Function();
typedef CopyCallResultFunc = int Function(int handle, Pointer<Utf8> buff);
typedef CopyCallResultNative = Uint32 Function(
    IntPtr handle, Pointer<Utf8> buff);

// desktopLibName returns the platform-dependent name of the dynamic library.
String desktopLibPath() {
  var exePath = path.dirname(Platform.resolvedExecutable);
  if (Platform.isLinux) {
    return path.join(exePath, "lib", "golib.so");
  } else if (Platform.isMacOS) {
    return "golib.dylib";
  } else if (Platform.isWindows) {
    return path.join(exePath, "golib.dll");
  }

  throw "Platform without desktopLibName()";
}
