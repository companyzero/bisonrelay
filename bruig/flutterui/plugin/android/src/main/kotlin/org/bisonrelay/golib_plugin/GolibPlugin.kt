package org.bisonrelay.golib_plugin

import androidx.annotation.NonNull

import android.app.ActivityManager;
import android.content.Context
import android.content.Intent
import android.media.AudioManager
import android.media.AudioDeviceInfo
import android.os.Build
import org.json.JSONObject

import golib.Golib
import org.bisonrelay.golib_plugin.NtfFgSvc

import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.plugins.activity.ActivityAware
import io.flutter.embedding.engine.plugins.service.ServiceAware
import io.flutter.embedding.engine.plugins.service.ServicePluginBinding
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding
import io.flutter.embedding.engine.plugins.lifecycle.HiddenLifecycleReference
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.Lifecycle
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
class GolibPlugin: FlutterPlugin, MethodCallHandler, ActivityAware, ServiceAware {
  /// The MethodChannel that will the communication between Flutter and native Android
  ///
  /// This local reference serves to register the plugin with the Flutter Engine and unregister it
  /// when the Flutter Engine is detached from the Activity
  private lateinit var channel : MethodChannel

  private lateinit var context : Context

  private val executorService: ExecutorService = Executors.newFixedThreadPool(2)

  private val loopsIds = mutableListOf<Int>()

  private var fgSvcEnabled: Boolean = false;
  private var ntfnsEnabled : Boolean = false;

  private var lastIntent: Map<String, Any?>? = null

  
  fun logProcessState() {
    var activityManager = context.getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
    var runningProcesses = activityManager.getRunningAppProcesses();
    for (processInfo in runningProcesses) {
      if (processInfo.pid != android.os.Process.myPid()) {
        continue;
      }

      // Here 'importance' is an integer that represents the process state
      var imp = processInfo.importance
      var impReason = processInfo.importanceReasonCode
      var compName = processInfo.importanceReasonComponent
      var lastTrim = processInfo.lastTrimLevel
      Golib.logInfo(0x12131400, "NativePlugin: process state imp=$imp reason=$impReason comp=$compName trim=$lastTrim")
    }
  }
  
  override fun onAttachedToEngine(@NonNull flutterPluginBinding: FlutterPlugin.FlutterPluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: attached to engine") // 0x88 == CTLogInfo
    context = flutterPluginBinding.applicationContext
    channel = MethodChannel(flutterPluginBinding.binaryMessenger, "golib_plugin")
    channel.setMethodCallHandler(this)
    this.initReadStream(flutterPluginBinding)
    this.initCmdResultLoop(flutterPluginBinding)
    NtfnBuilder.setUpNotificationChannels(context)    
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
          handler.post{ result.error(e::class.qualifiedName ?: "UnknownClass", e.toString(), null); }
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
    } else if (call.method == "asyncCall") {
      val typ: Int = call.argument("typ") ?: 0
      val id: Int = call.argument("id") ?: 0
      val handle: Int = call.argument("handle") ?: 0
      val payload: String? = call.argument("payload")
      Golib.asyncCallStr(typ.toLong(), id.toLong(), handle.toLong(), payload)
    } else if (call.method == "startFgSvc") {
      Golib.logInfo(0x12131400, "NativePlugin: toggling enabling FgSvc")
      fgSvcEnabled = true;

      // Run the foreground service. This is needed to get rid of old notifications.
      context.startService(Intent(context, NtfFgSvc::class.java))
    } else if (call.method == "stopFgSvc") {
      Golib.logInfo(0x12131400, "NativePlugin: toggling disabling FgSvc")
      fgSvcEnabled = false;
      context.stopService(Intent(context, NtfFgSvc::class.java))
    } else if (call.method == "setNtfnsEnabled") {
      val enabled: Boolean = call.argument("enabled") ?: false
      Golib.logInfo(0x12131400, "NativePlugin: toggling notifications to $enabled")
      ntfnsEnabled = enabled;      
    } else if (call.method == "listAudioDevices") {
      result.success(listAudioDevices())
    } else if (call.method == "getLastIntent") {
      var res = lastIntent
      lastIntent = null
      result.success(res)
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

  fun detachExistingLoops() {
    // Stop all async goroutines.
    val iterator = loopsIds.iterator()
    while (iterator.hasNext()) {
      var id = iterator.next()
      Golib.stopCmdResultLoop(id);
      iterator.remove();
    }
  }

  fun initCmdResultLoop(@NonNull flutterPluginBinding: FlutterPlugin.FlutterPluginBinding) {
    detachExistingLoops()
    Golib.stopAllCmdResultLoops() // Remove background ntfn loop from prior engine
    Golib.logInfo(0x12131400, "NativePlugin: Initing new CmdResultLoop")
    logProcessState()

    val handler = Handler(Looper.getMainLooper())
    val channel : EventChannel = EventChannel(flutterPluginBinding.binaryMessenger, "cmdResultLoop")
    var sink : EventChannel.EventSink? = null;

    channel.setStreamHandler(object : EventChannel.StreamHandler {
      override fun onListen(listener: Any?, newSink: EventChannel.EventSink?) {
        // TODO: support multiple readers?
        sink = newSink;
      }

      override fun onCancel(listener: Any?) {
        sink?.endOfStream();
        sink = null;
      }
    });

    var id = Golib.cmdResultLoop(object : golib.CmdResultLoopCB {
      override fun f(id: Int, typ: Int, payload: String, err: String) {
        val res: Map<String,Any> = mapOf("id" to id, "type" to typ, "payload" to payload, "error" to err)
        handler.post{ sink?.success(res) }
      }

      // UI notification called when app was in background but still attached to
      // flutter engine.
      override fun uiNtfn(text: String, nick: String, ts: Long) {
        Golib.logInfo(0x12131400, "NativePlugin: background UI ntfn from $nick") // 0x88 == CTLogInfo
        NtfnBuilder.showMsgNotification(context, nick, text, ts)
      }

      override fun callNtfn(nick: String, uid : String, sessRV: String) {
        Golib.logInfo(0x12131400, "NativePlugin: Call ntfn from $nick") // 0x88 == CTLogInfo
        NtfnBuilder.showCallNotification(context, nick, uid, sessRV)
      }
    }, false)
    loopsIds.add(id);
  }

  override fun onDetachedFromEngine(@NonNull binding: FlutterPlugin.FlutterPluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: detached from engine")
    logProcessState()
    channel.setMethodCallHandler(null)

    detachExistingLoops()
    Golib.stopAllCmdResultLoops()

    if (!ntfnsEnabled) {
      Golib.logInfo(0x12131400, "NativePlugin: all done after detach (ntfns disabled)")
      return;
    }

    // Attach background notification loop.
    var id = Golib.cmdResultLoop(object : golib.CmdResultLoopCB {
      override fun f(id: Int, typ: Int, payload: String, err: String) {
        // Ignored because the flutter engine is detached.
      }

      // UI notification called when app was in background _and_ flutter engine
      // was detached.
      override fun uiNtfn(text: String, nick: String, ts: Long) {
        Golib.logInfo(0x12131400, "NativePlugin: background UI ntfn from $nick") // 0x88 == CTLogInfo
        NtfnBuilder.showMsgNotification(context, nick, text, ts)
      }

      override fun callNtfn(nick: String, uid : String, sessRV: String) {
        Golib.logInfo(0x12131400, "NativePlugin: Call ntfn from $nick") // 0x88 == CTLogInfo
        NtfnBuilder.showCallNotification(context, nick, uid, sessRV)
      }
    }, true)
    loopsIds.add(id);
    Golib.logInfo(0x12131400, "NativePlugin: Started new loop id $id")

    // Run the foreground service.
    if (fgSvcEnabled) {
      context.startService(Intent(context, NtfFgSvc::class.java))
    }
  }


  override fun onAttachedToActivity(binding: ActivityPluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: onAttachedToActivity")
    (binding.lifecycle as HiddenLifecycleReference)
            .lifecycle
            .addObserver(LifecycleEventObserver { source, event ->
              Golib.logInfo(0x12131400, "NativePlugin: state changed to ${event}")
              logProcessState()
              if (event == Lifecycle.Event.ON_STOP) {
                // App went into background.
                Golib.asyncCallStr(0x84, 0, 0, null) // 0x84 == CTEnableBackgroundNtfs
                if (ntfnsEnabled && fgSvcEnabled) {
                  context.startService(Intent(context, NtfFgSvc::class.java))
                }
              } else if (event == Lifecycle.Event.ON_START) {
                // App came back from background.
                Golib.asyncCallStr(0x85, 0, 0, null) // 0x84 == CTDisableBackgroundNtfs
                if (ntfnsEnabled && fgSvcEnabled) {
                  context.stopService(Intent(context, NtfFgSvc::class.java))
                }
              }
            });
    binding.addOnNewIntentListener(fun(intent: Intent?): Boolean {
        Golib.logInfo(0x12131400, "NativePlugin: newIntent action ${binding.activity.intent.action} data ${binding.activity.intent.dataString}")
        return false;
    })

    val intent = binding.activity.intent
    lastIntent = mapOf<String, Any?>(
        "action" to intent.action,
        "data" to intent.dataString,        
        "sessRV" to intent.extras?.get("sessRV"),
        "inviter" to intent.extras?.get("inviter"),
        // "extra" to intent.extras?.let { bundleToJSON(it).toString() }
    )
    Golib.logInfo(0x12131400, "NativePlugin: action ${intent.action} data ${intent.dataString} extraAction ${intent.getExtras().toString()}  ")

    NtfnBuilder.cancelFgSvcNtf(context)
  }

  override fun onDetachedFromActivity() {
    Golib.logInfo(0x12131400, "NativePlugin: onDetachedFromActivity")
  }

  override fun onDetachedFromActivityForConfigChanges() {
    Golib.logInfo(0x12131400, "NativePlugin: onDetachedFromActivityForConfigChanges")
  }

  override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: onReattachedToActivityForConfigChanges")
    binding.addOnNewIntentListener(fun(intent: Intent?): Boolean {
        Golib.logInfo(0x12131400, "NativePlugin: newIntent action ${binding.activity.intent.action} data ${binding.activity.intent.dataString}")
        return false;
    })
  }

  override fun onAttachedToService(binding: ServicePluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: onAttachedToService")
  }

  override fun onDetachedFromService() {
    Golib.logInfo(0x12131400, "NativePlugin: onDetachedFromService")
  }

  
  // Lists the available audio devices and returns a json-encoded object. The
  // object is in the following format:
  // {"playback": [device], "capture": [device]}
  // Where each device is
  // {"id": "id of the device", "name": "name or description of device", is_default: false}
  fun listAudioDevices() : String {
    try {
      val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
      
      // For Android 6.0 (API 23) and above, we can use getDevices()
      val playbackDevices = mutableListOf<Map<String, Any>>()
      val captureDevices = mutableListOf<Map<String, Any>>()

      val supportedDeviceTypes = mapOf( // Note: These constants are used for switching speaker type
        26  to "Bluetooth Headset", // AudioDeviceInfo.TYPE_BLE_HEADSET
        27  to "Bluetooth Speaker", // AudioDeviceInfo.TYPE_BLE_SPEAKER
        // AudioDeviceInfo.TYPE_TELEPHONY to "Telephony",
        AudioDeviceInfo.TYPE_AUX_LINE to "Aux Line",
        AudioDeviceInfo.TYPE_BLUETOOTH_A2DP to "Bluetooth A2DP",
        AudioDeviceInfo.TYPE_BLUETOOTH_SCO to "Bluetooth SCO",
        AudioDeviceInfo.TYPE_BUILTIN_EARPIECE to "Internal Earpiece",
        AudioDeviceInfo.TYPE_BUILTIN_MIC to "Internal Microphone",
        AudioDeviceInfo.TYPE_BUILTIN_SPEAKER to "Internal Speaker",
        AudioDeviceInfo.TYPE_HEARING_AID to "Hearing Aid",
        AudioDeviceInfo.TYPE_USB_HEADSET to "USB Headset",
        AudioDeviceInfo.TYPE_WIRED_HEADPHONES to "Wired Headphones",
        AudioDeviceInfo.TYPE_WIRED_HEADSET to "Wired Headset",
      )

      val addedOutputs = mutableSetOf<Int>();
      val addedInputs = mutableSetOf<Int>();
      
      if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
        // Get output (playback) devices
        val outputDevices = audioManager.getDevices(AudioManager.GET_DEVICES_OUTPUTS)
        for (device in outputDevices) {
          if (!supportedDeviceTypes.containsKey(device.type)) {
            Golib.logInfo(0x12131400, "NativePlugin: Ignoring output device ${device.productName.toString()} type ${device.type} id ${device.id.toString()}");
            continue
          }

          if (addedOutputs.contains(device.id)) {
            // Avoid duplicates.
            continue
          }
          addedOutputs.add(device.id)

          Golib.logInfo(0x12131400, "NativePlugin: Output device ${device.productName.toString()} type ${device.type} id ${device.id.toString()} addr ${device.getAddress()} str ${device.toString()}");

          var addr = device.getAddress()
          if (addr != "") {
            addr = " (${addr})"
          }

          // Note: Determining the *actual* default device is complex and often
          // requires checking routing or specific API levels. This is a basic check.
          val isDefault = device.type == AudioDeviceInfo.TYPE_BUILTIN_SPEAKER || device.type == AudioDeviceInfo.TYPE_WIRED_HEADSET || device.type == AudioDeviceInfo.TYPE_WIRED_HEADPHONES
          playbackDevices.add(mapOf(
            "id" to device.id.toString(),
            "name" to "${device.productName.toString()} ${supportedDeviceTypes[device.type]}$addr",
            "is_default" to isDefault // Simplified default check
          ))
        }
        
        // Get input (capture) devices
        val inputDevices = audioManager.getDevices(AudioManager.GET_DEVICES_INPUTS)
        for (device in inputDevices) {
          if (!supportedDeviceTypes.containsKey(device.type)) {
            Golib.logInfo(0x12131400, "NativePlugin: Ignoring input device ${device.productName.toString()} type ${device.type} id ${device.id.toString()}");
            continue
          }

          if (addedInputs.contains(device.id)) {
            // Avoid duplicates.
            continue
          }
          addedInputs.add(device.id)

          Golib.logInfo(0x12131400, "NativePlugin: Input device ${device.productName.toString()} type ${device.type} id ${device.id.toString()} addr ${device.getAddress()} str ${device.toString()}");

          var addr = device.getAddress()
          if (addr != "") {
            addr = " (${addr})"
          }

          val isDefault = device.type == AudioDeviceInfo.TYPE_BUILTIN_MIC
          captureDevices.add(mapOf(
            "id" to device.id.toString(),
            "name" to "${device.productName.toString()} ${supportedDeviceTypes[device.type]}$addr",
            "is_default" to isDefault // Simplified default check
          ))
        }
      } else {
        // For older Android versions, add placeholder default devices
        // Checking isSpeakerphoneOn() or isWiredHeadsetOn() might give clues but isn't definitive.
        playbackDevices.add(mapOf(
          "id" to "default_output",
          "name" to "Default Output Device",
          "is_default" to true
        ))
        
        captureDevices.add(mapOf(
          "id" to "default_input",
          "name" to "Default Input Device",
          "is_default" to true
        ))
      }
      
      // Create the result JSON structure
      val result = mapOf(
        "playback" to playbackDevices,
        "capture" to captureDevices
      )
      
      // Convert to JSON string
      return JSONObject(result).toString()
    } catch (e: Exception) {
      Golib.logInfo(0x12131400, "NativePlugin: Error listing audio devices: ${e.message}")
      // Return empty structure in case of error
      return "{\"playback\": [], \"capture\": []}"
    }
  }
}
