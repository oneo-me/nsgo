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
 * Owns one NSGo process and its pipe. A host may keep this object in an
 * Application or foreground service while Activities attach and detach their
 * NsgoView. The current protocol still carries one surface; surface IDs are
 * a later protocol stage documented in android-lifecycle.md.
 */
public final class NsgoEngine {
    private static final String TAG = "nsgo";

    /** Receives messages from the Go process on the engine reader thread. */
    interface Listener {
        void onMessage(int kind, byte[] payload);

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

    /** Starts the process once. NsgoView supplies the immutable launch config. */
    synchronized void start(ProcessBuilder builder) throws IOException {
        if (closed) throw new IllegalStateException("engine is destroyed");
        if (process != null && process.isAlive()) return;
        process = builder.start();
        out = new DataOutputStream(new BufferedOutputStream(process.getOutputStream()));
        readerStarted = false;
    }

    /** Attaches the current surface listener and starts the pipe readers. */
    synchronized void attach(Listener next) {
        if (closed) throw new IllegalStateException("engine is destroyed");
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

    /** Detaches a listener without stopping the process. */
    synchronized void detach(Listener old) {
        if (listener == old) listener = null;
    }

    /** Says whether the process is running and the engine has not been closed. */
    public synchronized boolean isAlive() {
        return !closed && process != null && process.isAlive();
    }

    /** Says whether this engine can be started again. */
    synchronized boolean isDestroyed() {
        return closed;
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

    /** Stops the process and releases the pipe. It is safe to call twice. */
    public synchronized void destroy() {
        if (closed) return;
        closed = true;
        listener = null;
        if (out != null) {
            try {
                out.close();
            } catch (IOException ignored) {
                // The process is being destroyed.
            }
            out = null;
        }
        if (process != null) {
            process.destroy();
            process = null;
        }
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
                    current = process == running ? listener : null;
                }
                if (current != null) current.onMessage(kind, payload);
            }
        } catch (IOException e) {
            failure = e;
        }
        Listener current;
        synchronized (this) {
            current = process == running ? listener : null;
            if (process == running) {
                process = null;
                out = null;
                readerStarted = false;
            }
            listener = null;
        }
        if (current != null) current.onProcessEnded(failure);
    }

    private void log(Process running) {
        try (BufferedReader err = new BufferedReader(new InputStreamReader(running.getErrorStream()))) {
            for (String line; (line = err.readLine()) != null; ) {
                Listener current;
                synchronized (this) {
                    current = process == running ? listener : null;
                }
                if (current != null) current.onLog(line);
            }
        } catch (IOException ignored) {
            // The process ended.
        }
    }
}
