package org.bisonrelay.bruig

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.MethodChannel.MethodCallHandler
import io.flutter.plugin.common.MethodChannel.Result

import golib.Golib

import android.app.NotificationManager
import android.content.Context
import android.content.Intent



class MainActivity: MethodCallHandler, FlutterActivity()  {
    private val METHOD_CHANNEL = "org.bisonrelay.bruig.mainActivityChannel"
    private var lastIntent: Map<String, Any?>? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, METHOD_CHANNEL).setMethodCallHandler(this)
    }

    override fun onMethodCall(call: MethodCall, result: Result) {
        when (call.method) {
            "getLastIntent" -> {
                var res = lastIntent
                lastIntent = null
                result.success(res)
            }
            "closeAllNotifications" -> {
                closeAllNotifications()
                result.success(true)
            }
            else -> {
                result.notImplemented()
            }
        }
    }

    override fun onResume() {
        Golib.logInfo(0x12131400, "MainActivity: onResume()")
        super.onResume()
        closeAllNotifications();
    }

    override fun onStart() {
        Golib.logInfo(0x12131400, "MainActivity: onStart()")
        super.onStart()        
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)

        Golib.logInfo(0x12131400, "MainActivity: onNewIntent() action ${intent.action} data ${intent.dataString}")
        
        lastIntent = mapOf<String, Any?>(
            "action" to intent.action,
            "data" to intent.dataString,
            "sessRV" to intent.extras?.get("sessRV"),
            "inviter" to intent.extras?.get("inviter"),
            // "extra" to intent.extras?.let { bundleToJSON(it).toString() }
        )

        if (lastIntent != null && intent.action == Intent.ACTION_ANSWER) {
            closeAllNotifications()
        }        
    }

    private fun closeAllNotifications() {
        Golib.logInfo(0x12131400, "MainActivity: Closing all notifications")
        val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.cancelAll()
    }
}