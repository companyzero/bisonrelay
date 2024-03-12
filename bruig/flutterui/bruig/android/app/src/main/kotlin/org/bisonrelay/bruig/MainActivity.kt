package org.bisonrelay.bruig

import io.flutter.embedding.android.FlutterActivity

import golib.Golib

import android.app.NotificationManager
import android.content.Context

class MainActivity: FlutterActivity() {
    override fun onResume() {
        Golib.logInfo(0x12131400, "MainActivity: onResume()")
        super.onResume()
        closeAllNotifications();
    }

    override fun onStart() {
        Golib.logInfo(0x12131400, "MainActivity: onStart()")
        super.onStart()
    }

    private fun closeAllNotifications() {
        val notificationManager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.cancelAll()
    }

}