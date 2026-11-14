package dev.oneo.nsgo;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;

/** Keeps an app process hosting a system overlay alive after its activity hides. */
public final class OverlayService extends Service {
    static final String STOP = "dev.oneo.nsgo.STOP_OVERLAY";
    private static final String CHANNEL = "nsgo-overlay";

    @Override public void onCreate() {
        super.onCreate();
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationManager manager = getSystemService(NotificationManager.class);
            manager.createNotificationChannel(new NotificationChannel(CHANNEL, "NSGo overlay",
                    NotificationManager.IMPORTANCE_LOW));
        }
        Notification.Builder builder = Build.VERSION.SDK_INT >= 26
                ? new Notification.Builder(this, CHANNEL)
                : new Notification.Builder(this);
        builder.setSmallIcon(android.R.drawable.ic_dialog_info)
                .setContentTitle("NSGo overlay")
                .setContentText("A floating window is active")
                .setOngoing(true);
        startForeground(0x4e53474f, builder.build());
    }

    @Override public int onStartCommand(Intent intent, int flags, int id) {
        if (intent != null && STOP.equals(intent.getAction())) stopSelf();
        return START_NOT_STICKY;
    }

    @Override public IBinder onBind(Intent intent) { return null; }
}
