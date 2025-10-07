package org.bisonrelay.golib_plugin

import golib.Golib

import android.app.Activity
import android.os.Bundle
import android.os.PersistableBundle

import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat

import org.bisonrelay.golib_plugin.NtfFgSvc


class CallDeclineActivity: Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState);
        replaceNtfn();
        finish();
    }
      
    private fun replaceNtfn() {
      Golib.logInfo(0x12131400, "CallDeclineActivity replacing fg svc notification")
      NtfnBuilder.showFgSvcNtfn(getApplication())
    }
}