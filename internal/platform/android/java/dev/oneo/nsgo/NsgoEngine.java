package dev.oneo.nsgo;

import android.content.Context;
import android.util.Log;

import java.io.BufferedInputStream;
import java.io.BufferedReader;
import java.io.BufferedOutputStream;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.io.InputStreamReader;

/**
 * Owns one NSGo process and its pipe. A host may keep this object in an Application or foreground service, so that the app runs on while the NsgoView showing it is detached from its Activity.
 *
 * Until the surface protocol in android-lifecycle.md lands, the pipe carries one surface: an engine serves the one NsgoView that first attaches to it, keeps it, detached or not, until the process ends, and refuses every other view. The process starts once; once it has ended or been destroyed, a host makes a new engine.
 */
public final class NsgoEngine {
    private static final String TAG = "nsgo";

    /** Receives messages from the Go process on the engine reader thread. */
    interface Listener {
        void onMessage(int kind, byte[] payload);

        /** The process ended, by itself or through destroy; called once, after the last message. */
        void onProcessEnded(IOException error);

        void onLog(String line);
    }

    private final Context application;
    private Process process;
    private DataOutputStream out;
    private Listener listener;
    private boolean readerStarted;
    private boolean closed;

    /** Creates an engine without retaining an Activity or another UI context. */
    public NsgoEngine(Context context) {
        if (context == null) throw new NullPointerException("context");
        application = context.getApplicationContext();
    }

    /** Starts the process; an engine starts one only. NsgoView supplies the immutable launch config. */
    synchronized void start(ProcessBuilder builder) throws IOException {
        if (!canStart()) throw new IllegalStateException(closed ? "engine is destroyed" : "engine has started its process");
        process = builder.start();
        out = new DataOutputStream(new BufferedOutputStream(process.getOutputStream()));
    }

    /** Says whether the engine has neither started a process nor been destroyed. */
    synchronized boolean canStart() {
        return !closed && process == null;
    }

    /**
     * Binds the view the engine serves and starts the pipe readers once the process runs. The view stays bound until the process ends; another view is refused, as the pipe carries one surface's tree and text.
     */
    synchronized void attach(Listener next) {
        if (closed) throw new IllegalStateException("engine is destroyed");
        if (listener != null && listener != next) {
            throw new IllegalStateException("engine serves another NsgoView: until the surface protocol lands, an engine serves the one view that first attached to it");
        }
        listener = next;
        if (readerStarted || process == null) return;
        readerStarted = true;
        Process running = process;
        Thread reader = new Thread(() -> read(running), "nsgo-reader");
        reader.setDaemon(true);
        reader.start();
        Thread logger = new Thread(() -> log(running), "nsgo-log");
        logger.setDaemon(true);
        logger.start();
    }

    /** Says whether the process is running and the engine has not been closed. */
    public synchronized boolean isAlive() {
        return !closed && process != null && process.isAlive();
    }

    /** Writes one protocol message, serialized with every other writer. */
    synchronized void write(int kind, byte[] payload) {
        if (out == null || closed) return;
        try {
            out.writeByte(kind);
            out.writeInt(payload.length);
            out.write(payload);
            out.flush();
        } catch (IOException e) {
            Log.w(TAG, "app process unreachable", e);
        }
    }

    /**
     * Stops the process and releases the pipe; it is safe to call twice. The bound view hears of it through onProcessEnded once the reader has passed it the last message, or at once if no reader runs.
     */
    public void destroy() {
        Listener idle;
        synchronized (this) {
            if (closed) return;
            closed = true;
            if (out != null) {
                try {
                    out.close();
                } catch (IOException ignored) {
                    // The process is being destroyed.
                }
                out = null;
            }
            if (process != null) process.destroy();
            // Without a reader, nothing else tells the view.
            idle = readerStarted ? null : listener;
            if (!readerStarted) listener = null;
        }
        if (idle != null) idle.onProcessEnded(null);
    }

    private void read(Process running) {
        IOException failure = null;
        try (DataInputStream in = new DataInputStream(new BufferedInputStream(running.getInputStream()))) {
            while (true) {
                int kind = in.readUnsignedByte();
                byte[] payload = new byte[in.readInt()];
                in.readFully(payload);
                Listener current;
                synchronized (this) {
                    current = listener;
                }
                // The bound view answers even while detached: Go waits for its text replies.
                if (current != null) current.onMessage(kind, payload);
            }
        } catch (IOException e) {
            failure = e;
        }
        Listener current;
        synchronized (this) {
            current = listener;
            listener = null;
            out = null;
        }
        if (current != null) current.onProcessEnded(failure);
    }

    private void log(Process running) {
        try (BufferedReader err = new BufferedReader(new InputStreamReader(running.getErrorStream()))) {
            for (String line; (line = err.readLine()) != null; ) {
                Listener current;
                synchronized (this) {
                    current = listener;
                }
                if (current != null) current.onLog(line);
            }
        } catch (IOException ignored) {
            // The process ended.
        }
    }
}
