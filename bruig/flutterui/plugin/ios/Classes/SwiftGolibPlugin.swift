import Flutter
import UIKit
import Golib

private class ReadStreamHandler: NSObject, FlutterStreamHandler {
    var eventSink: FlutterEventSink?

    override init() {
        super.init()
        Golib.GolibReadLoop(self)
    }

    public func onListen(withArguments args: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? {
        // TODO: support multiple listeners?
        self.eventSink = events
        return nil
        }

    public func onCancel(withArguments args: Any?) -> FlutterError? {
        return nil
    }
}

extension ReadStreamHandler : GolibReadLoopCBProtocol {
    func f(_ s: String?) {
        self.eventSink?(s)
    }
}

extension DispatchQueue {
    static func background(f: (()->Void)? = nil, done: (()->Void)? = nil) {
        DispatchQueue.global(qos: .background).async {
            f?()
            if let done = done {
                DispatchQueue.main.asyncAfter(deadline: .now(), execute: done)
            }
        }
    }
}

public class SwiftGolibPlugin: NSObject, FlutterPlugin {
  public static func register(with registrar: FlutterPluginRegistrar) {
    let channel = FlutterMethodChannel(name: "golib_plugin", binaryMessenger: registrar.messenger())
    let readStream = FlutterEventChannel(name: "readStream", binaryMessenger: registrar.messenger())
    readStream.setStreamHandler(ReadStreamHandler())
    let instance = SwiftGolibPlugin()
    registrar.addMethodCallDelegate(instance, channel: channel)
  }

  public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
    switch call.method {
    case "getPlatformVersion":
      result("iOS " + UIDevice.current.systemVersion)
    case "hello":
      Golib.GolibHello()
      result(nil) 
    case "setTag":
      let args = call.arguments as? Dictionary<String, Any>
      let tag = args?["tag"] as? String
      Golib.GolibSetTag(tag)
      result(nil) 
    case "nextTime":
      result(Golib.GolibNextTime())
    case "getURL":
      let args = call.arguments as? Dictionary<String, Any>
      let url = args?["url"] as? String
      var err: NSError?
      var res: Any?
      DispatchQueue.background(f: {
        res = Golib.GolibGetURL(url, &err)
      }, done: {
        if err != nil {
          result(FlutterError(code: "GolibError", message: err!.localizedDescription, details: nil))
          return
        }
        result(res)
      })
    case "writeStr":
      let args = call.arguments as? Dictionary<String, Any>
      let s = args?["s"] as? String
      Golib.GolibWriteStr(s)
      result(nil) 
    default:
      result(FlutterMethodNotImplemented)
    }
  }
}
