package org.bisonrelay.golib_plugin

import androidx.annotation.NonNull

import androidx.core.app.NotificationCompat
import android.app.NotificationManager
import androidx.core.app.Person
import android.content.Context
import android.app.NotificationChannel
import android.content.Intent
import android.app.PendingIntent

import golib.Golib

import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.plugins.activity.ActivityAware
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
class GolibPlugin: FlutterPlugin, MethodCallHandler, ActivityAware {
  /// The MethodChannel that will the communication between Flutter and native Android
  ///
  /// This local reference serves to register the plugin with the Flutter Engine and unregister it
  /// when the Flutter Engine is detached from the Activity
  private lateinit var channel : MethodChannel

  private lateinit var context : Context

  private val executorService: ExecutorService = Executors.newFixedThreadPool(2)

  private val loopsIds = mutableListOf<Int>()

  companion object {
    private const val CHANNEL_NEW_MESSAGES = "new_messages"
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
      return notificationManager;
  }

  fun showNotification(notificationManager: NotificationManager, nick: String, msg: String, ts: Long) {
    // Intent to open app when clicking the notification.
    val resultIntent = Intent("org.bisonrelay.bruig.NTFN")// Intent(context, "MainActivity")
    val pendingIntent = PendingIntent.getActivity(context, 0, resultIntent, PendingIntent.FLAG_UPDATE_CURRENT)

    // Sender styling.
    val user = Person.Builder().setName(nick).build()
    // var icon = Icon(Icons.Rounded.Menu, contentDescription = "Localized description");
    val person: Person? = null

    // Create message.
    val m = NotificationCompat.MessagingStyle.Message(msg, ts*1000, person)
    val messagingStyle = NotificationCompat.MessagingStyle(user)
    messagingStyle.addMessage(m)
    val builder = NotificationCompat.Builder(context, CHANNEL_NEW_MESSAGES)
      .setStyle(messagingStyle)
      .setSmallIcon(/*R.drawable.stat_notify_chat*/ 0x01080077)
      .setWhen(ts*1000)
      .setContentIntent(pendingIntent)

    // Send notification.
    val contactID: Long = 1000
    notificationManager.notify(contactID.toInt(), builder.build())
  }

  override fun onAttachedToEngine(@NonNull flutterPluginBinding: FlutterPlugin.FlutterPluginBinding) {
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

      // PM notification called when app was in background but still attached to
      // flutter engine.
      override fun pm(uid: String, nick: String, msg: String, ts: Long) {
        showNotification(ntfManager, nick, msg, ts)
      }
    }, false)
    loopsIds.add(id);
  }

  
  override fun onDetachedFromEngine(@NonNull binding: FlutterPlugin.FlutterPluginBinding) {
    channel.setMethodCallHandler(null)

    detachExistingLoops()
    Golib.stopAllCmdResultLoops()

    // Attach background notification loop.
    var ntfManager = setUpNotificationChannels();
    var id = Golib.cmdResultLoop(object : golib.CmdResultLoopCB {
      override fun f(id: Int, typ: Int, payload: String, err: String) {
        // Ignored because the flutter engine is detached.
      }

      // PM notification called when app was in background _and_ flutter engine
      // was detached.
      override fun pm(uid: String, nick: String, msg: String, ts: Long) {
        showNotification(ntfManager, nick, msg, ts)
      }
    }, true)
    loopsIds.add(id);
  }


  override fun onAttachedToActivity(binding: ActivityPluginBinding) {
    (binding.lifecycle as HiddenLifecycleReference)
            .lifecycle
            .addObserver(LifecycleEventObserver { source, event ->
              if (event == Lifecycle.Event.ON_STOP) {
                // App went into background.
                Golib.asyncCallStr(0x84, 0, 0, null) // 0x84 == CTEnableBackgroundNtfs
              } else if (event == Lifecycle.Event.ON_START) {
                // App came back from background.
                Golib.asyncCallStr(0x85, 0, 0, null) // 0x84 == CTDisableBackgroundNtfs
              }
            });
  }

  override fun onDetachedFromActivity() {}

  override fun onDetachedFromActivityForConfigChanges() {}

  override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) {}
}
