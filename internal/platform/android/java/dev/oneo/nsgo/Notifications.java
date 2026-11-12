package dev.oneo.nsgo;

import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.os.Build;

/**
 * System notifications through NotificationManager, as core's
 * ShowNotification: a channel for those with the system's sound and one
 * for silent ones, POST_NOTIFICATIONS asked from API 33 where the app's
 * manifest asks for it, and a press that brings the app's activity back
 * with the notification's ID. Android has no badge count of its own:
 * launchers mark an app with notifications showing, and the count goes
 * with those shown after it is set.
 */
final class Notifications {
    private Notifications() {}

    static final String PERMISSION = "android.permission.POST_NOTIFICATIONS";
    static final int PERMISSION_REQUEST = 0x6e79;
    /** The extra of the intent a press starts the activity with: the notification's ID. */
    static final String EXTRA = "dev.oneo.nsgo.notification";
    private static final String CHANNEL = "nsgo", SILENT = "nsgo.silent";
    private static final String PREFS = "nsgo.notifications", ASKED = "asked";

    /** The count ITEM_BADGE set, which the notifications shown after it carry. */
    static int badge;

    /** Whether the app may show notifications, as core.NotificationPermission. */
    static int permission(Context context) {
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager == null) return Protocol.NOTIFICATIONS_UNAVAILABLE;
        if (Build.VERSION.SDK_INT >= 33
                && context.checkSelfPermission(PERMISSION) != PackageManager.PERMISSION_GRANTED) {
            return asked(context) ? Protocol.NOTIFICATIONS_DENIED : Protocol.NOTIFICATIONS_NOT_ASKED;
        }
        return manager.areNotificationsEnabled() ? Protocol.NOTIFICATIONS_GRANTED : Protocol.NOTIFICATIONS_DENIED;
    }

    /**
     * Whether a request for the permission needs the system's prompt: from
     * API 33, not asked yet, with the permission in the app's manifest.
     * Without it in the manifest the system refuses at once.
     */
    static boolean needsPrompt(Context context) {
        if (permission(context) != Protocol.NOTIFICATIONS_NOT_ASKED) return false;
        if (!declares(context)) {
            markAsked(context);
            return false;
        }
        return true;
    }

    static void markAsked(Context context) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putBoolean(ASKED, true).apply();
    }

    private static boolean asked(Context context) {
        SharedPreferences prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
        return prefs.getBoolean(ASKED, false);
    }

    private static boolean declares(Context context) {
        try {
            String[] asked = context.getPackageManager()
                    .getPackageInfo(context.getPackageName(), PackageManager.GET_PERMISSIONS).requestedPermissions;
            return asked != null && java.util.Arrays.asList(asked).contains(PERMISSION);
        } catch (PackageManager.NameNotFoundException e) {
            return false;
        }
    }

    /** Shows a notification, replacing one of its ID; whether the system took it. */
    static boolean show(Context context, Activity activity, String id, String title, String body, boolean silent) {
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager == null || permission(context) != Protocol.NOTIFICATIONS_GRANTED) return false;
        String channel = silent ? SILENT : CHANNEL;
        if (manager.getNotificationChannel(channel) == null) {
            NotificationChannel c = new NotificationChannel(channel, silent ? "Silent notifications" : "Notifications",
                    silent ? NotificationManager.IMPORTANCE_LOW : NotificationManager.IMPORTANCE_DEFAULT);
            manager.createNotificationChannel(c);
        }
        int icon = context.getApplicationInfo().icon;
        Notification.Builder b = new Notification.Builder(context, channel)
                .setSmallIcon(icon != 0 ? icon : android.R.drawable.ic_dialog_info)
                .setContentTitle(title)
                .setAutoCancel(true)
                .setContentIntent(pressIntent(context, activity, id));
        if (!body.isEmpty()) b.setContentText(body);
        if (badge > 0) b.setNumber(badge);
        try {
            manager.notify(id, 0, b.build());
        } catch (RuntimeException e) {
            return false;
        }
        return true;
    }

    /** Removes the notification called id, if it still shows. */
    static void close(Context context, String id) {
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager != null) manager.cancel(id, 0);
    }

    /**
     * The intent a press sends: the app's activity, brought back if it
     * runs, with the notification's ID.
     */
    private static PendingIntent pressIntent(Context context, Activity activity, String id) {
        Intent intent = activity != null ? new Intent(context, activity.getClass())
                : context.getPackageManager().getLaunchIntentForPackage(context.getPackageName());
        if (intent == null) intent = new Intent();
        intent.setPackage(context.getPackageName());
        intent.putExtra(EXTRA, id);
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        return PendingIntent.getActivity(context, id.hashCode(), intent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }
}
