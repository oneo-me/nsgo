package dev.oneo.nsgo;

import android.os.SystemClock;
import java.io.ByteArrayOutputStream;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Downloads the app's images for its Go process with HttpURLConnection,
 * a few at a time, as fetch.go asks: a FETCH message starts a download or
 * cancels it, and FETCHED messages report its progress, at most every
 * 100 ms, then its status, headers and body, or what failed.
 */
final class Downloads {
    interface Writer {
        void write(int kind, byte[] payload);
    }

    private static final class Download {
        volatile boolean cancelled;
        volatile HttpURLConnection connection;
    }

    private final Writer out;
    private final ConcurrentHashMap<Integer, Download> running = new ConcurrentHashMap<>();
    private final ExecutorService pool = Executors.newFixedThreadPool(6, r -> {
        Thread t = new Thread(r, "nsgo-downloads");
        t.setDaemon(true);
        return t;
    });

    Downloads(Writer out) {
        this.out = out;
    }

    /** Reads a FETCH message on the reader thread. */
    void message(DataInputStream in) throws IOException {
        int id = in.readInt();
        if (in.readUnsignedByte() == Protocol.FETCH_CANCEL) {
            Download d = running.get(id);
            if (d != null) {
                d.cancelled = true;
                HttpURLConnection c = d.connection;
                if (c != null) c.disconnect();
            }
            return;
        }
        String url = Protocol.readString(in);
        String[] headers = new String[2 * in.readInt()];
        for (int i = 0; i < headers.length; i++) headers[i] = Protocol.readString(in);
        Download d = new Download();
        running.put(id, d);
        pool.execute(() -> run(id, d, url, headers));
    }

    private void run(int id, Download d, String url, String[] headers) {
        HttpURLConnection c = null;
        try {
            c = (HttpURLConnection) new URL(url).openConnection();
            d.connection = c;
            if (d.cancelled) return;
            // imageload keeps its own cache under HTTP's rules.
            c.setUseCaches(false);
            for (int i = 0; i < headers.length; i += 2) c.addRequestProperty(headers[i], headers[i + 1]);
            int status = c.getResponseCode();
            long total = Math.max(c.getContentLengthLong(), 0);
            ByteArrayOutputStream body = new ByteArrayOutputStream((int) Math.min(total, 64 << 20));
            InputStream stream = status >= 400 ? c.getErrorStream() : c.getInputStream();
            if (stream != null) {
                try (InputStream s = stream) {
                    byte[] buf = new byte[64 << 10];
                    long last = SystemClock.uptimeMillis();
                    for (int n; (n = s.read(buf)) > 0; ) {
                        body.write(buf, 0, n);
                        long now = SystemClock.uptimeMillis();
                        if (now - last >= 100) {
                            last = now;
                            progress(id, body.size(), total);
                        }
                    }
                }
            }
            done(id, "", status, c.getHeaderFields(), body);
        } catch (IOException | RuntimeException e) {
            if (!d.cancelled) done(id, String.valueOf(e.getMessage()), 0, null, null);
        } finally {
            running.remove(id);
            if (c != null) c.disconnect();
        }
    }

    private void progress(int id, long loaded, long total) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        DataOutputStream b = new DataOutputStream(bytes);
        try {
            b.writeInt(id);
            b.writeByte(Protocol.FETCHED_PROGRESS);
            b.writeLong(loaded);
            b.writeLong(total);
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        out.write(Protocol.FETCHED, bytes.toByteArray());
    }

    /** Reports the download's response, or its error when not "". */
    private void done(int id, String error, int status, Map<String, List<String>> headers, ByteArrayOutputStream body) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream((body == null ? 0 : body.size()) + 1024);
        DataOutputStream b = new DataOutputStream(bytes);
        try {
            b.writeInt(id);
            b.writeByte(Protocol.FETCHED_DONE);
            Protocol.writeString(b, error);
            if (error.isEmpty()) {
                b.writeInt(status);
                int count = 0;
                for (Map.Entry<String, List<String>> h : headers.entrySet()) {
                    // The null key is the status line.
                    if (h.getKey() != null) count += h.getValue().size();
                }
                b.writeInt(count);
                for (Map.Entry<String, List<String>> h : headers.entrySet()) {
                    if (h.getKey() == null) continue;
                    for (String v : h.getValue()) {
                        Protocol.writeString(b, h.getKey());
                        Protocol.writeString(b, v);
                    }
                }
                b.writeInt(body.size());
                body.writeTo(b);
            }
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        out.write(Protocol.FETCHED, bytes.toByteArray());
    }
}
