package com.example.golib_plugin

import androidx.annotation.NonNull

import golib.Golib

import io.flutter.embedding.engine.plugins.FlutterPlugin
import android.os.Handler
import android.os.Looper
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.MethodChannel.MethodCallHandler
import io.flutter.plugin.common.MethodChannel.Result

/** GolibPlugin */
class GolibPlugin: FlutterPlugin, MethodCallHandler {
  /// The MethodChannel that will the communication between Flutter and native Android
  ///
  /// This local reference serves to register the plugin with the Flutter Engine and unregister it
  /// when the Flutter Engine is detached from the Activity
  private lateinit var channel : MethodChannel

  private val executorService: ExecutorService = Executors.newFixedThreadPool(2)


  override fun onAttachedToEngine(@NonNull flutterPluginBinding: FlutterPlugin.FlutterPluginBinding) {
    channel = MethodChannel(flutterPluginBinding.binaryMessenger, "golib_plugin")
    channel.setMethodCallHandler(this)
    this.initReadStream(flutterPluginBinding)
  }

  override fun onMethodCall(@NonNull call: MethodCall, @NonNull result: Result) {
    if (call.method == "getPlatformVersion") {
      result.success("Android ${android.os.Build.VERSION.RELEASE}")
    } else if (call.method == "hello") {
      Golib.hello()
      result.success(null);
    } else if (call.method == "getURL") {
      // Will perform a network access, so launch on separate coroutine.
      val url: String? = call.argument("url");
      executorService.execute {
        val handler = Handler(Looper.getMainLooper())
        try {
          val res = Golib.getURL(url);
          handler.post{ result.success(res) }
        } catch (e: Exception) {
          handler.post{ result.error(e::class.qualifiedName, e.toString(), null); }
        }
      }
    } else if (call.method == "setTag") {
      val tag: String? = call.argument("tag");
      Golib.setTag(tag);
      result.success(null);
    } else if (call.method == "nextTime") {
      val nt: String? = Golib.nextTime()
      result.success(nt);
    } else if (call.method == "writeStr") {
      val s: String? = call.argument("s");
      Golib.writeStr(s);
      result.success(null);
    } else if (call.method == "readStr") {
      val s: String? = Golib.readStr()
      result.success(s);
    } else {
      result.notImplemented()
    }
  }

  fun initReadStream(@NonNull flutterPluginBinding: FlutterPlugin.FlutterPluginBinding) {
    val handler = Handler(Looper.getMainLooper())
    val channel : EventChannel = EventChannel(flutterPluginBinding.binaryMessenger, "readStream")
    var sink : EventChannel.EventSink? = null;

    channel.setStreamHandler(object : EventChannel.StreamHandler {
      override fun onListen(listener: Any?, newSink: EventChannel.EventSink?) {
        // TODO: support multiple readers?
        sink = newSink;
      }    

      override fun onCancel(listener: Any?) {
        sink = null;
      }
    });

    Golib.readLoop(object : golib.ReadLoopCB {
      override fun f(msg: String) {
        handler.post{ sink?.success(msg) }
      }
    })
  }
  
  override fun onDetachedFromEngine(@NonNull binding: FlutterPlugin.FlutterPluginBinding) {
    channel.setMethodCallHandler(null)

    // TODO: stop readstream?
  }
}
