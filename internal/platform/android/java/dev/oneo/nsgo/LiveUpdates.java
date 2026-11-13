package dev.oneo.nsgo;

import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.graphics.drawable.Icon;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.service.notification.StatusBarNotification;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

/**
 * Package widget's Live Activities as notifications, which Go starts,
 * updates and ends with LIVE_UPDATE's JSON (widget/activity_android.go):
 * Live Updates from Android 16, promoted to the top of the shade, the
 * Lock Screen and a chip of the status bar with Status, a ProgressStyle
 * bar and a chronometer, and ongoing notifications with a bar before.
 * Each is tagged with its activity's ID, so the runtime finds those of
 * earlier runs, and reports LIVE_REPORT's state, whether they show and
 * which run, as it starts and after each change, and the presses of their
 * actions, which bring the app's activity back.
 */
final class LiveUpdates {
    private LiveUpdates() {}

    private static final String CHANNEL = "nsgo.live";
    private static final String TAG = "nsgo.live:";
    private static final String KIND = "dev.oneo.nsgo.live.kind";
    private static final String PROMOTE = "android.requestPromotedOngoing";
    /** The extras of the intent an action's press starts the activity with. */
    static final String EXTRA_ID = "dev.oneo.nsgo.live.id", EXTRA_ACTION = "dev.oneo.nsgo.live.action";

    /** Carries out a LIVE_UPDATE's message, on the UI thread. */
    static void apply(Context context, Activity activity, byte[] message) {
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager == null) return;
        try {
            JSONObject m = new JSONObject(new String(message, StandardCharsets.UTF_8));
            String tag = TAG + m.getString("id");
            boolean end = m.getString("op").equals("end");
            long dismiss = m.optLong("dismiss");
            if (end && (dismiss <= System.currentTimeMillis() || !m.has("title"))) {
                manager.cancel(tag, 0);
                return;
            }
            Notification.Builder b = builder(context, activity, manager, m, !end);
            if (end) b.setTimeoutAfter(dismiss - System.currentTimeMillis());
            manager.notify(tag, 0, b.build());
        } catch (JSONException | RuntimeException e) {
            android.util.Log.w("nsgo", "Live Activity: " + e);
        }
    }

    private static Notification.Builder builder(Context context, Activity activity, NotificationManager manager,
            JSONObject m, boolean ongoing) throws JSONException {
        if (manager.getNotificationChannel(CHANNEL) == null) {
            // Live Updates need an importance of default or more; they
            // alert only as Alert asks.
            NotificationChannel c = new NotificationChannel(CHANNEL, "Live Activities", NotificationManager.IMPORTANCE_DEFAULT);
            c.setSound(null, null);
            c.enableVibration(false);
            manager.createNotificationChannel(c);
        }
        String id = m.getString("id");
        int icon = context.getApplicationInfo().icon;
        if (icon == 0) icon = android.R.drawable.ic_dialog_info;
        Bundle extras = new Bundle();
        extras.putString(KIND, m.getString("kind"));
        Notification.Builder b = new Notification.Builder(context, CHANNEL)
                .setSmallIcon(icon)
                .setContentTitle(m.optString("title"))
                .setOngoing(ongoing)
                .setAutoCancel(false)
                .setOnlyAlertOnce(!m.optBoolean("alert"))
                .setCategory(Notification.CATEGORY_PROGRESS)
                .addExtras(extras)
                .setContentIntent(press(context, activity, id, null, m.optString("url")));
        String body = m.optString("body");
        if (!body.isEmpty()) b.setContentText(body);
        long tint = m.optLong("tint");
        if (tint != 0) b.setColor((int) tint);
        long timer = m.optLong("timer");
        if (timer != 0) {
            b.setWhen(timer).setShowWhen(true).setUsesChronometer(true);
            b.setChronometerCountDown(timer > System.currentTimeMillis());
        }
        double progress = m.optDouble("progress", -1);
        String status = m.optString("status");
        if (Build.VERSION.SDK_INT >= 36) {
            // Notification.Builder.setRequestPromotedOngoing from 36.1,
            // which reads this extra.
            if (ongoing) extras.putBoolean(PROMOTE, true);
            if (!status.isEmpty()) b.setShortCriticalText(status);
            if (progress >= 0) {
                Notification.ProgressStyle.Segment segment = new Notification.ProgressStyle.Segment(100);
                if (tint != 0) segment.setColor((int) tint);
                b.setStyle(new Notification.ProgressStyle()
                        .setProgressSegments(Collections.singletonList(segment))
                        .setProgress((int) Math.round(progress * 100)));
            }
        } else {
            if (progress >= 0) b.setProgress(100, (int) Math.round(progress * 100), false);
            if (!status.isEmpty()) b.setSubText(status);
        }
        b.addExtras(extras);
        JSONArray actions = m.optJSONArray("actions");
        for (int i = 0; actions != null && i < actions.length(); i++) {
            JSONObject a = actions.getJSONObject(i);
            b.addAction(new Notification.Action.Builder(Icon.createWithResource(context, icon), a.getString("title"),
                    press(context, activity, id, a.getString("id"), null)).build());
        }
        return b;
    }

    /**
     * The intent a press sends: the app's activity, brought back if it
     * runs, with the activity's ID and the action pressed, if any, and the
     * content's URL as its data.
     */
    private static PendingIntent press(Context context, Activity activity, String id, String action, String url) {
        Intent intent = activity != null ? new Intent(context, activity.getClass())
                : context.getPackageManager().getLaunchIntentForPackage(context.getPackageName());
        if (intent == null) intent = new Intent();
        intent.setPackage(context.getPackageName());
        if (action != null) {
            intent.putExtra(EXTRA_ID, id);
            intent.putExtra(EXTRA_ACTION, action);
        }
        if (url != null && !url.isEmpty()) intent.setData(Uri.parse(url));
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        String key = id + "\u0000" + (action == null ? "" : action);
        return PendingIntent.getActivity(context, key.hashCode(), intent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }

    /** LIVE_REPORT's state: whether activities show and which run. */
    static byte[] state(Context context) {
        JSONObject r = new JSONObject();
        try {
            NotificationManager manager = context.getSystemService(NotificationManager.class);
            JSONArray running = new JSONArray();
            if (manager != null) {
                for (StatusBarNotification n : manager.getActiveNotifications()) {
                    String tag = n.getTag();
                    if (tag == null || !tag.startsWith(TAG) || (n.getNotification().flags & Notification.FLAG_ONGOING_EVENT) == 0) continue;
                    running.put(new JSONObject().put("id", tag.substring(TAG.length()))
                            .put("kind", n.getNotification().extras.getString(KIND, "")));
                }
            }
            r.put("type", "state")
                    .put("enabled", Notifications.permission(context) == Protocol.NOTIFICATIONS_GRANTED)
                    .put("running", running);
        } catch (JSONException e) {
            throw new IllegalStateException(e);
        }
        return r.toString().getBytes(StandardCharsets.UTF_8);
    }

    /** LIVE_REPORT's press of an action, or null for an intent of none. */
    static byte[] press(Intent intent) {
        String id = intent.getStringExtra(EXTRA_ID), action = intent.getStringExtra(EXTRA_ACTION);
        if (id == null || action == null) return null;
        try {
            return new JSONObject().put("type", "press").put("id", id).put("action", action).toString()
                    .getBytes(StandardCharsets.UTF_8);
        } catch (JSONException e) {
            throw new IllegalStateException(e);
        }
    }
}
