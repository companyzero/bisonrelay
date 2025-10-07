package org.bisonrelay.golib_plugin

import golib.Golib

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.graphics.drawable.Icon
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.net.Uri
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.Person
import androidx.core.graphics.drawable.IconCompat

// Singleton object for building notifications.
object NtfnBuilder {
  const val FGSVC_NTFN_ID : Int = 123482823

  const val CHANNEL_FGSVC = "fg_svc"
  const val CHANNEL_INSTANT_CALLS = "instant_calls2"
  const val CHANNEL_NEW_MESSAGES = "new_messages"        

  const val ACTION_DECLINE_CALL = "org.bisonrelay.bruig.ACTION_DECLINE_CALL"

  // Sets up the notification channels with the OS.
  fun setUpNotificationChannels(context: Context)  {
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


      if (notificationManager.getNotificationChannel(CHANNEL_INSTANT_CALLS) == null) {
        val defaultRingtoneUri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_RINGTONE)
        val audioAttributes = AudioAttributes.Builder()
            .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
            .setUsage(AudioAttributes.USAGE_NOTIFICATION_RINGTONE) 
            .build()

        val channel = NotificationChannel(
            CHANNEL_INSTANT_CALLS,
            "Instant Calls",
            NotificationManager.IMPORTANCE_HIGH,
        ).apply{
          enableVibration(true)
          enableLights(true)
          setSound(defaultRingtoneUri, audioAttributes)
          vibrationPattern = longArrayOf(1000, 1000, 1000, 1000) 
        }
        notificationManager.createNotificationChannel(channel)
      }
  }  

  
  private fun buildFgServiceNtfn(context: Context) : Notification {
      val targetComp = ComponentName("org.bisonrelay.bruig", "org.bisonrelay.bruig.MainActivity")
    //var actionIntent = Intent("org.bisonrelay.bruig.NTFN")
    var actionIntent = Intent("android.intent.action.MAIN")
      .addCategory("android.intent.category.LAUNCHER")
      .setComponent(targetComp)
      .setFlags(/*Intent.FLAG_ACTIVITY_NEW_TASK*/ 0x30000000)
    val pendingIntent = PendingIntent.getActivity(context, 0, actionIntent, 
      PendingIntent.FLAG_IMMUTABLE)

    val iconID = context.resources.getIdentifier(
            "ic_launcher",
            "mipmap",
            context.packageName
    ) // 0x01080067

    return NotificationCompat.Builder(context, CHANNEL_FGSVC)
      .setContentTitle("Bison Relay")
      .setContentText("BR background service is waiting for messages")
      .setContentIntent(pendingIntent)
      .setPriority(NotificationCompat.PRIORITY_MIN)
      .setWhen(0)
      .setSmallIcon(iconID)
      .setSilent(true)
      .build()
  }

  // Show/replace the foreground service notification with the standard one.
  fun showFgSvcNtfn(context: Context) {
    var ntfn = buildFgServiceNtfn(context)
    var notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
    notificationManager.notify(FGSVC_NTFN_ID, ntfn)
  }

  // Start the foreground service with the standard notification.
  fun startFgSvcWithNtfn(svc: Service) {
      var ntfn = buildFgServiceNtfn(svc.getApplication())
      
      svc.startForeground(NtfnBuilder.FGSVC_NTFN_ID, ntfn,
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
          ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
        } else {
            0
        },
      )
  }

  // Show a notification about an instant (RTDT) call.
  fun showCallNotification(context : Context, nick: String, uid : String, sessRV: String) {
    val appActity = ComponentName("org.bisonrelay.bruig", "org.bisonrelay.bruig.MainActivity")    
    val declineActivity = ComponentName("org.bisonrelay.bruig", "org.bisonrelay.golib_plugin.CallDeclineActivity")

    // 1. PendingIntent for content click (your basic tap on the notification)
    val contentIntent = Intent(Intent.ACTION_MAIN)
        .setComponent(appActity)
        .setFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_RESET_TASK_IF_NEEDED)
    val pendingContentIntent = PendingIntent.getActivity(context,
        0, // Unique request code
        contentIntent,
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT // Use UPDATE_CURRENT
    )

    // 2. PendingIntent for the "Answer" action
    val answerFlags = 0 //or 
      //Intent.FLAG_ACTIVITY_NEW_TASK or 
      //Intent.FLAG_ACTIVITY_RESET_TASK_IF_NEEDED or
      //Intent.FLAG_ACTIVITY_SINGLE_TOP or
      //Intent.FLAG_ACTIVITY_CLEAR_TOP
    val answerIntent = Intent(Intent.ACTION_ANSWER) 
        .setComponent(appActity)
        .setFlags(answerFlags)
        .putExtra("sessRV", sessRV) 
        .putExtra("inviter", uid)
    val pendingAnswerIntent = PendingIntent.getActivity(context,
        1, // Unique request code
        answerIntent,
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
    )

    // 3. PendingIntent for the "Decline" action
    val declineIntent = Intent(ACTION_DECLINE_CALL)
        .setComponent(declineActivity)
        .putExtra("action", "declineCall") // Add an extra to differentiate
    val pendingDeclineIntent = PendingIntent.getActivity(context,
        2, // Unique request code
        declineIntent,
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
    )

    val iconID = context.resources.getIdentifier(
              "ic_launcher",
              "mipmap",
              context.packageName
    ) // 0x01080077
    val icon = Icon.createWithResource(context, iconID)
    val iconCompat = IconCompat.createFromIcon(icon)
    val caller = Person.Builder().setName(nick).setIcon(iconCompat).setImportant(true).build()

    // Create a call style notification for an incoming call.
    val m = NotificationCompat.MessagingStyle.Message("test", 1000, caller)
    val messagingStyle = NotificationCompat.MessagingStyle(caller)
    messagingStyle.addMessage(m)

    val builder = NotificationCompat.Builder(context, CHANNEL_INSTANT_CALLS /* CHANNEL_FGSVC */)
      .setContentIntent(pendingContentIntent)
      .setSmallIcon(iconID)
      // .setDefaults(-1 /* Notification.DEFAULT_ALL */)
      .setOngoing(true)
      .setPriority(NotificationCompat.PRIORITY_MAX) 
      .setStyle(
          NotificationCompat.CallStyle.forIncomingCall(caller, pendingDeclineIntent, pendingAnswerIntent))
      .addPerson(caller)
      // .setFullScreenIntent(pendingAnswerIntent, true) // Pass your pending intent
    
    var notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager    
    notificationManager.notify(FGSVC_NTFN_ID, builder.build())
  }

  // Show a text message call.
  fun showMsgNotification(context: Context, nick: String, msg: String, ts: Long) {
    // Intent to open app when clicking the notification.
    val targetComp = ComponentName("org.bisonrelay.bruig", "org.bisonrelay.bruig.MainActivity")
    var actionIntent = Intent(/* "org.bisonrelay.bruig.NTFN" */"android.intent.action.MAIN")
      .setComponent(targetComp)
      .setFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_RESET_TASK_IF_NEEDED)
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
    val contactID: Int = 1000
    var notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager    
    notificationManager.notify(contactID, builder.build())
  }

  fun cancelFgSvcNtf(context: Context) {
    var notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager    
    notificationManager.cancel(FGSVC_NTFN_ID)
  }
}