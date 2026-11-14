package dev.oneo.nsgo;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.media.AudioFormat;
import android.media.AudioRecord;
import android.media.MediaRecorder;
import android.os.Build;
import java.util.HashMap;
import java.util.Map;

/**
 * Long-running microphone capture. It owns AudioRecord outside the activity
 * lifecycle and forwards bounded PCM chunks to the NSGo pipe through Callback.
 */
public final class RecordingService extends Service {
    static final String ACTION_START = "dev.oneo.nsgo.RECORDING_START";
    static final String ACTION_STOP = "dev.oneo.nsgo.RECORDING_STOP";
    static final String EXTRA_ID = "id";
    static final String EXTRA_RATE = "rate";
    private static final String CHANNEL = "nsgo-recording";
    private static volatile Callback callback;
    private final Map<Integer, Recorder> recorders = new HashMap<>();

    interface Callback {
        void opened(int id, int rate, int channels);
        void samples(int id, short[] data, int count, float level);
        void ended(int id, String error);
    }

    static void setCallback(Callback value) {
        callback = value;
    }

    static void start(Context context, int id, int rate) {
        Intent intent = new Intent(context, RecordingService.class).setAction(ACTION_START)
                .putExtra(EXTRA_ID, id).putExtra(EXTRA_RATE, rate);
        if (Build.VERSION.SDK_INT >= 26) context.startForegroundService(intent);
        else context.startService(intent);
    }

    static void stop(Context context, int id) {
        Intent intent = new Intent(context, RecordingService.class).setAction(ACTION_STOP)
                .putExtra(EXTRA_ID, id);
        if (Build.VERSION.SDK_INT >= 26) context.startForegroundService(intent);
        else context.startService(intent);
    }

    static void stopAll(Context context) {
        context.stopService(new Intent(context, RecordingService.class));
    }

    @Override public void onCreate() {
        super.onCreate();
        NotificationManager manager = getSystemService(NotificationManager.class);
        if (Build.VERSION.SDK_INT >= 26) {
            manager.createNotificationChannel(new NotificationChannel(CHANNEL, "Recording",
                    NotificationManager.IMPORTANCE_LOW));
        }
        Notification.Builder b = Build.VERSION.SDK_INT >= 26
                ? new Notification.Builder(this, CHANNEL) : new Notification.Builder(this);
        b.setContentTitle("NSGo recording").setContentText("Microphone is in use")
                .setSmallIcon(android.R.drawable.ic_btn_speak_now).setOngoing(true);
        if (Build.VERSION.SDK_INT >= 29) {
            startForeground(0x4e53474f, b.build(), ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE);
        } else {
            startForeground(0x4e53474f, b.build());
        }
    }

    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent == null) return START_STICKY;
        int id = intent.getIntExtra(EXTRA_ID, 0);
        if (ACTION_STOP.equals(intent.getAction())) {
            Recorder r;
            synchronized (recorders) { r = recorders.remove(id); }
            if (r != null) r.stop(null);
            stopIfIdle();
            return START_NOT_STICKY;
        }
        if (ACTION_START.equals(intent.getAction())) {
            Recorder old;
            synchronized (recorders) { old = recorders.remove(id); }
            if (old != null) old.stop("restarted");
            Recorder r = new Recorder(id, intent.getIntExtra(EXTRA_RATE, 44100));
            synchronized (recorders) { recorders.put(id, r); }
            r.start();
        }
        return START_STICKY;
    }

    private void stopIfIdle() {
        synchronized (recorders) { if (!recorders.isEmpty()) return; }
        stopForeground(STOP_FOREGROUND_REMOVE);
        stopSelf();
    }

    @Override public void onDestroy() {
        Recorder[] all;
        synchronized (recorders) { all = recorders.values().toArray(new Recorder[0]); recorders.clear(); }
        for (Recorder r : all) r.stop("service stopped");
        super.onDestroy();
    }

    @Override public android.os.IBinder onBind(Intent intent) { return null; }

    private final class Recorder implements Runnable {
        final int id, requested;
        volatile boolean stopped;
        boolean finished;
        AudioRecord audio;
        Thread thread;

        Recorder(int id, int requested) { this.id = id; this.requested = requested > 0 ? requested : 44100; }

        void start() {
            try {
                int rate = chooseRate(requested);
                int min = AudioRecord.getMinBufferSize(rate, AudioFormat.CHANNEL_IN_MONO,
                        AudioFormat.ENCODING_PCM_16BIT);
                if (min <= 0) throw new IllegalStateException("AudioRecord has no supported input rate");
                int buffer = Math.max(min, rate / 5 * 2);
                audio = new AudioRecord.Builder().setAudioSource(MediaRecorder.AudioSource.MIC)
                        .setAudioFormat(new AudioFormat.Builder().setSampleRate(rate)
                                .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                                .setChannelMask(AudioFormat.CHANNEL_IN_MONO).build())
                        .setBufferSizeInBytes(buffer).build();
                if (audio.getState() != AudioRecord.STATE_INITIALIZED) throw new IllegalStateException("AudioRecord did not initialize");
                audio.startRecording();
                Callback c = callback;
                if (c != null) c.opened(id, rate, 1);
                thread = new Thread(this, "nsgo-recording-" + id);
                thread.start();
            } catch (SecurityException e) {
                finish(e.getMessage() == null ? "microphone permission denied" : e.getMessage());
            } catch (RuntimeException e) {
                finish(e.getMessage() == null ? "cannot open microphone" : e.getMessage());
            }
        }

        @Override public void run() {
            short[] data = new short[Math.max(256, audio.getSampleRate() / 20)];
            try {
                while (!stopped) {
                    int n = audio.read(data, 0, data.length, AudioRecord.READ_BLOCKING);
                    if (n <= 0) { if (n == AudioRecord.ERROR_DEAD_OBJECT) break; continue; }
                    double sum = 0;
                    for (int i = 0; i < n; i++) { double v = data[i] / 32768.0; sum += v * v; }
                    Callback c = callback;
                    if (c != null) c.samples(id, data, n, (float)Math.sqrt(sum / n));
                }
            } finally {
                finish(null);
            }
        }

        void stop(String error) {
            stopped = true;
            if (audio != null) { try { audio.stop(); } catch (RuntimeException ignored) {} }
            if (thread == null) finish(error);
        }

        void finish(String error) {
            if (finished) return;
            finished = true;
            stopped = true;
            if (audio != null) { audio.release(); audio = null; }
            Callback c = callback;
            if (c != null) c.ended(id, error == null ? "" : error);
            synchronized (recorders) { recorders.remove(id); }
            stopIfIdle();
        }
    }

    private static int chooseRate(int requested) {
        int[] rates = {requested, 44100, 48000, 16000, 8000};
        for (int rate : rates) {
            if (rate <= 0) continue;
            int min = AudioRecord.getMinBufferSize(rate, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT);
            if (min > 0) return rate;
        }
        return requested;
    }
}
