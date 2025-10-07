package org.bisonrelay.golib_plugin

import golib.Golib

import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder


class NtfFgSvc : Service() {    
    override fun onBind(intent: Intent?): IBinder? {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onBind")
      return null
    }

    override fun onCreate() {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onCreate")
      showNtfn()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
      Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onStartCommand")
      if (intent != null) {
        Golib.logInfo(0x12131400, "NativePlugin: FgSvc.onStartCommand ${intent.action} ${intent.getExtras().toString()}")
      }
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
      NtfnBuilder.startFgSvcWithNtfn(this)
    }
}
