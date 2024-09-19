package org.bisonrelay.golib_plugin

import androidx.annotation.NonNull

import androidx.core.app.NotificationCompat
import android.app.NotificationManager
import android.app.ActivityManager;
import androidx.core.app.Person
import android.content.Context
import android.app.NotificationChannel
import android.content.Intent
import android.content.ComponentName
import android.app.PendingIntent
import android.app.Service
import android.content.pm.ServiceInfo
import android.os.IBinder
import android.os.Build
import android.graphics.drawable.Icon
import androidx.core.graphics.drawable.IconCompat

import golib.Golib

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

  companion object {
    private const val CHANNEL_NEW_MESSAGES = "new_messages"
    private const val CHANNEL_FGSVC = "fg_svc"
  }

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


  fun setUpNotificationChannels(): NotificationManager  {
      var notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
      if (notificationManager.getNotificationChannel(CHANNEL_NEW_MESSAGES) == null) {
          notificationManager.createNotificationChannel(
              NotificationChannel(
                  CHANNEL_NEW_MESSAGES,
                  "New Messages",
                  // The importance must be IMPORTANCE_HIGH to show Bubbles.
                  NotificationManager.IMPORTANCE_HIGH
              )
          )
      }
      if (notificationManager.getNotificationChannel(CHANNEL_FGSVC) == null) {
        notificationManager.createNotificationChannel(
            NotificationChannel(
                CHANNEL_FGSVC,
                "Foreground Svc",
                // The importance must be IMPORTANCE_HIGH to show Bubbles.
                NotificationManager.IMPORTANCE_DEFAULT,
            )
        )
    }
      return notificationManager;
  }

  fun showNotification(notificationManager: NotificationManager, nick: String, msg: String, ts: Long) {
    // Intent to open app when clicking the notification.
    val targetComp = ComponentName("org.bisonrelay.bruig", ".MainActivity")
    var actionIntent = Intent(/* "org.bisonrelay.bruig.NTFN" */"android.intent.action.MAIN")
      .setComponent(targetComp)
      .setFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_RESET_TASK_IF_NEEDED or Intent.FLAG_FROM_BACKGROUND)
    val pendingIntent = PendingIntent.getActivity(context, 0, actionIntent, PendingIntent.FLAG_IMMUTABLE)

    val iconID = context.resources.getIdentifier(
              "ic_launcher",
              "mipmap",
              context.packageName
    ) // 0x01080077
    val icon = Icon.createWithResource(context, iconID)
    val iconCompat = IconCompat.createFromIcon(icon)

    // Sender styling.
    val user = Person.Builder().setName(nick).setIcon(iconCompat).build()
    val person: Person? = null

    // Create message.
    val m = NotificationCompat.MessagingStyle.Message(msg, ts*1000, person)
    val messagingStyle = NotificationCompat.MessagingStyle(user)
    messagingStyle.addMessage(m)
    val builder = NotificationCompat.Builder(context, CHANNEL_NEW_MESSAGES)
      .setStyle(messagingStyle)
      .setSmallIcon(iconID)
      .setWhen(ts*1000)
      .setContentIntent(pendingIntent)


    // Send notification.
    val contactID: Long = 1000
    notificationManager.notify(contactID.toInt(), builder.build())
  }

  override fun onAttachedToEngine(@NonNull flutterPluginBinding: FlutterPlugin.FlutterPluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: attached to engine") // 0x88 == CTLogInfo
    context = flutterPluginBinding.applicationContext
    channel = MethodChannel(flutterPluginBinding.binaryMessenger, "golib_plugin")
    channel.setMethodCallHandler(this)
    this.initReadStream(flutterPluginBinding)
    this.initCmdResultLoop(flutterPluginBinding)
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
      Golib.asyncCallStr(typ, id, handle, payload)
    } else if (call.method == "startFgSvc") {
      fgSvcEnabled = true;
    } else if (call.method == "stopFgSvc") {
      fgSvcEnabled = false;
      context.stopService(Intent(context, FgSvc::class.java))
    } else if (call.method == "setNtfnsEnabled") {
      val enabled: Boolean = call.argument("enabled") ?: false
      Golib.logInfo(0x12131400, "NativePlugin: toggling notifications to $enabled")
      ntfnsEnabled = enabled;
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
    var ntfManager = setUpNotificationChannels();

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
        showNotification(ntfManager, nick, text, ts)
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
    var ntfManager = setUpNotificationChannels();
    var id = Golib.cmdResultLoop(object : golib.CmdResultLoopCB {
      override fun f(id: Int, typ: Int, payload: String, err: String) {
        // Ignored because the flutter engine is detached.
      }

      // UI notification called when app was in background _and_ flutter engine
      // was detached.
      override fun uiNtfn(text: String, nick: String, ts: Long) {
        Golib.logInfo(0x12131400, "NativePlugin: background UI ntfn from $nick") // 0x88 == CTLogInfo
        showNotification(ntfManager, nick, text, ts)
      }
    }, true)
    loopsIds.add(id);
    Golib.logInfo(0x12131400, "NativePlugin: Started new loop id $id")

    // Run the foreground service.
    if (fgSvcEnabled) {
      context.startService(Intent(context, FgSvc::class.java))
    }
  }


  override fun onAttachedToActivity(binding: ActivityPluginBinding) {
    (binding.lifecycle as HiddenLifecycleReference)
            .lifecycle
            .addObserver(LifecycleEventObserver { source, event ->
              Golib.logInfo(0x12131400, "NativePlugin: state changed to ${event}")
              logProcessState()
              if (event == Lifecycle.Event.ON_STOP) {
                // App went into background.
                Golib.asyncCallStr(0x84, 0, 0, null) // 0x84 == CTEnableBackgroundNtfs
                if (ntfnsEnabled && fgSvcEnabled) {
                  context.startService(Intent(context, FgSvc::class.java))
                }
              } else if (event == Lifecycle.Event.ON_START) {
                // App came back from background.
                Golib.asyncCallStr(0x85, 0, 0, null) // 0x84 == CTDisableBackgroundNtfs
                if (ntfnsEnabled && fgSvcEnabled) {
                  context.stopService(Intent(context, FgSvc::class.java))
                }
              }
            });
  }

  override fun onDetachedFromActivity() {
    Golib.logInfo(0x12131400, "NativePlugin: onDetachedFromActivity")
  }

  override fun onDetachedFromActivityForConfigChanges() {
    Golib.logInfo(0x12131400, "NativePlugin: onDetachedFromActivityForConfigChanges")
  }

  override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: onReattachedToActivityForConfigChanges")
  }

  override fun onAttachedToService(binding: ServicePluginBinding) {
    Golib.logInfo(0x12131400, "NativePlugin: onAttachedToService")
  }

  override fun onDetachedFromService() {
    Golib.logInfo(0x12131400, "NativePlugin: onDetachedFromService")
  }

  class FgSvc : Service() {
    override fun onBind(intent: Intent?): IBinder? {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onBind")
      return null
    }

    override fun onCreate() {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onCreate")
      showNtfn()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
      super.onStartCommand(intent, flags, startId)
      showNtfn()
      return START_STICKY
    }

    override fun onLowMemory() {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onLowMemory")
    }

    override fun onTrimMemory(level: Int) {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onTrimMemory level $level")
    }

    override fun onDestroy() {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onDestroy")
      super.onDestroy()
    }

    private fun showNtfn() {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.showNtfn")
      val targetComp = ComponentName("org.bisonrelay.bruig", ".MainActivity")
      //var actionIntent = Intent("org.bisonrelay.bruig.NTFN")
      var actionIntent = Intent("android.intent.action.MAIN")
        .addCategory("android.intent.category.LAUNCHER")
        .setComponent(targetComp)
        .setFlags(/*Intent.FLAG_ACTIVITY_NEW_TASK*/ 0x30000000)
      val pendingIntent = PendingIntent.getActivity(getApplication(), 0, actionIntent, 
        PendingIntent.FLAG_IMMUTABLE)

      val iconID = getApplication().resources.getIdentifier(
              "ic_launcher",
              "mipmap",
              getApplication().packageName
      ) // 0x01080067

      val notification = NotificationCompat.Builder(getApplication(), CHANNEL_FGSVC)
        .setContentTitle("Bison Relay")
        .setContentText("BR background service is waiting for messages")
        .setContentIntent(pendingIntent)
        .setPriority(NotificationCompat.PRIORITY_MIN)
        .setWhen(0)
        .setSmallIcon(iconID)
        .setSilent(true)
        .build()

      var foreground_id = 123482823
      startForeground(foreground_id, notification,
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
          ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
        } else {
            0
        },
      )
    }
  }
}
