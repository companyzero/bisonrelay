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

private class CmdResultLoop: NSObject, FlutterStreamHandler {
    var eventSink: FlutterEventSink?

    override init() {
        super.init()
        Golib.GolibCmdResultLoop(self, false)
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

extension CmdResultLoop : GolibCmdResultLoopCBProtocol {
    func f(_ s: Int32, typ: Int32, payload: String?, err: String?) {
      var d: [String: Any] = ["id":s, "type":typ, "payload": payload, "error": err]
        self.eventSink?(d)
    }

    func pm(_: String?, nick: String?, msg: String?, ts: Int64) {
      // TODO: show background notification.
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

    let cmdResultLoop = FlutterEventChannel(name: "cmdResultLoop", binaryMessenger: registrar.messenger())
    cmdResultLoop.setStreamHandler(CmdResultLoop())

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
    case "asyncCall":
      let args = call.arguments as? Dictionary<String, Any>
      let typ = (args?["typ"] as? Int32) ?? 0
      let id = (args?["id"] as? Int32) ?? 0
      let handle = (args?["handle"] as? Int32) ?? 0
      let payload = (args?["payload"] as? String) ?? ""
      var err: NSError?
      var res: Any?
      Golib.GolibAsyncCallStr(typ, id, handle, payload)
      result(nil)
    default:
      result(FlutterMethodNotImplemented)
    }
  }
}
