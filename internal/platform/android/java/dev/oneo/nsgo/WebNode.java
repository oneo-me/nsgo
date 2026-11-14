package dev.oneo.nsgo;

import android.annotation.SuppressLint;
import android.content.Context;
import android.graphics.Bitmap;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.util.Base64;
import android.view.ContextThemeWrapper;
import android.view.MotionEvent;
import android.view.ViewGroup;
import android.webkit.JavascriptInterface;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.CookieManager;
import android.webkit.MimeTypeMap;
import android.webkit.URLUtil;
import org.json.JSONException;
import org.json.JSONObject;
import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.ByteArrayOutputStream;
import java.io.DataInputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.net.URLConnection;
import java.util.Locale;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicLong;

/**
 * A web view node's android.webkit.WebView, filling the node's view, as
 * NSH's WebBox. Pages post to the app through window.nsgo.postMessage: the
 * JavaScript interface INTERFACE is there from the page's first script,
 * and the bridge Go puts first in the script before content wraps it to
 * pass any value as a string. The view loads its source again when it or
 * the scripts change.
 */
@SuppressLint({"SetJavaScriptEnabled", "AddJavascriptInterface", "ClickableViewAccessibility"})
final class WebNode {
    /** The JavaScript interface pages post to, as webInterface in android.go. */
    static final String INTERFACE = "nsgo";
    /** The JavaScript interface a page hands the bytes of a blob: download to, under the token Java gave it. */
    private static final String BLOBS = "nsgoBlobs";

    /**
     * Notes the name a page gives each link it downloads (the download attribute), by URL, as android.webkit reports a download without it: links the user clicks, and links scripts click or dispatch a click to, which are often never in the document. It runs as each page starts, before the page's own scripts as far as android.webkit allows.
     *
     * It also keeps the Blob behind a blob: link it notes, until saveBlob reads it: android.webkit reports the download only after the click returns, and pages commonly revoke the URL right after clicking it (URL.revokeObjectURL in the same task or a short timeout), after which the URL can no longer be fetched. So it remembers which Blob each object URL the page makes stands for, forgetting it when the page revokes the URL unless a download link already took it.
     */
    private static final String DOWNLOAD_NAMES = "(function () {"
            + " if (window.__nsgoDownloadNames) return;"
            + " var names = new Map(), made = new Map(), kept = new Map();"
            + " Object.defineProperty(window, '__nsgoDownloadNames', {value: names});"
            + " Object.defineProperty(window, '__nsgoDownloadBlobs', {value: kept});"
            + " var create = URL.createObjectURL, revoke = URL.revokeObjectURL;"
            + " URL.createObjectURL = function (o) { var u = create.apply(URL, arguments); if (o instanceof Blob) made.set(u, o); return u; };"
            + " URL.revokeObjectURL = function (u) { made.delete(String(u)); return revoke.apply(URL, arguments); };"
            + " var note = function (a) { if (!a || !a.href || !a.hasAttribute('download')) return;"
            + "  names.set(a.href, a.getAttribute('download')); if (made.has(a.href)) kept.set(a.href, made.get(a.href)); };"
            + " var click = HTMLAnchorElement.prototype.click;"
            + " HTMLAnchorElement.prototype.click = function () { note(this); return click.apply(this, arguments); };"
            + " var dispatch = HTMLAnchorElement.prototype.dispatchEvent;"
            + " HTMLAnchorElement.prototype.dispatchEvent = function (e) { if (e && e.type === 'click') note(this); return dispatch.apply(this, arguments); };"
            + " document.addEventListener('click', function (e) { note(e.target && e.target.closest ? e.target.closest('a[download]') : null); }, true);"
            + "})();";

    /** Receives a web view's events; navigation is null for messages and has only the URL for errors. UI thread. */
    interface Reporter {
        void report(int kind, String text, Navigation navigation);

        void download(String data);
    }

    /** A page and its history, as core.WebNavigation. */
    static final class Navigation {
        final String url, title;
        final boolean loading, canGoBack, canGoForward;

        Navigation(String url, String title, boolean loading, boolean canGoBack, boolean canGoForward) {
            this.url = url;
            this.title = title;
            this.loading = loading;
            this.canGoBack = canGoBack;
            this.canGoForward = canGoForward;
        }
    }

    final WebView view;
    private final Reporter reporter;
    private final Handler ui = new Handler(Looper.getMainLooper());
    private boolean configured;
    private String src = "", html = "", baseURL = "";
    private String injected = "", injectedBefore = "", userAgent = "";
    private Map<String, String> headers = new HashMap<>();
    private boolean noScroll;
    private boolean incognito;
    private String downloadDir = "";
    /** The blob: downloads waiting for their page's bytes, by the token the page answers with. UI thread. */
    private final Map<String, BlobAnswer> blobs = new HashMap<>();
    private volatile boolean destroyed;
    private static final AtomicLong DOWNLOAD_SERIAL = new AtomicLong();
    private static final ExecutorService DOWNLOADS = Executors.newFixedThreadPool(3, r -> {
        Thread t = new Thread(r, "nsgo-web-downloads");
        t.setDaemon(true);
        return t;
    });
    /** Whether top-level navigations go only to hosts, space-separated patterns (WebConfig.Navigable). */
    private boolean restricted;
    private String[] hosts = new String[0];

    /** The theme the page's prefers-color-scheme follows from Android 13. */
    private final ContextThemeWrapper themed;

    WebNode(Context context, Reporter reporter) {
        this.reporter = reporter;
        themed = new ContextThemeWrapper(context, android.R.style.Theme_DeviceDefault_NoActionBar);
        view = new WebView(themed);
        view.getSettings().setDomStorageEnabled(true);
        view.addJavascriptInterface(new Bridge(), INTERFACE);
        view.addJavascriptInterface(new Blobs(), BLOBS);
        view.setWebViewClient(new Client());
        view.setDownloadListener(this::download);
        view.setOnTouchListener((v, e) -> {
            if (e.getActionMasked() == MotionEvent.ACTION_DOWN) {
                // The page scrolls itself: keep enclosing scroll views out.
                v.getParent().requestDisallowInterceptTouchEvent(true);
            }
            // Without moves the page cannot scroll; taps still reach it.
            return noScroll && e.getActionMasked() == MotionEvent.ACTION_MOVE;
        });
    }

    /**
     * Shows pages in the app's appearance: from Android 13 they take it
     * from the view's theme as prefers-color-scheme; before it a forced
     * appearance darkens them, or not, as the system's dark theme would.
     */
    @SuppressWarnings("deprecation")
    void appearance(boolean dark, boolean forced) {
        int theme = dark ? android.R.style.Theme_DeviceDefault_NoActionBar
                : android.R.style.Theme_DeviceDefault_Light_NoActionBar;
        themed.getTheme().applyStyle(theme, true);
        if (Build.VERSION.SDK_INT >= 33) {
            // The page reads the theme again as the configuration changes.
            view.dispatchConfigurationChanged(view.getResources().getConfiguration());
        } else if (Build.VERSION.SDK_INT >= 29) {
            view.getSettings().setForceDark(!forced ? WebSettings.FORCE_DARK_AUTO
                    : dark ? WebSettings.FORCE_DARK_ON : WebSettings.FORCE_DARK_OFF);
        }
    }

    /** Called from the page on a binder thread. */
    private final class Bridge {
        @JavascriptInterface
        public void postMessage(String data) {
            ui.post(() -> reporter.report(Protocol.EVENT_MESSAGE, data != null ? data : "", null));
        }
    }

    /** What a blob: download does with its page's answer: the bytes, base64, or null and an error. */
    private interface BlobAnswer {
        void answer(String base64, String name, String type, String error);
    }

    /** Called from the page on a binder thread with the bytes of a blob: download Java asked it for. */
    private final class Blobs {
        @JavascriptInterface
        public void answer(String token, String base64, String name, String type, String error) {
            ui.post(() -> {
                BlobAnswer answer = blobs.remove(token);
                if (answer != null && !destroyed) answer.answer(base64, name, type, error);
            });
        }
    }

    /** Whether the page may go to url at the top level, as driver.WebConfig.Navigable says. */
    boolean navigable(String url) {
        if (!restricted) return true;
        android.net.Uri uri = android.net.Uri.parse(url);
        String scheme = uri.getScheme() == null ? "" : uri.getScheme().toLowerCase(Locale.ROOT);
        switch (scheme) {
            case "about": case "data": case "blob": return true;
            case "http": case "https": break;
            default: return false;
        }
        String host = uri.getHost() == null ? "" : uri.getHost().toLowerCase(Locale.ROOT);
        if (host.endsWith(".")) host = host.substring(0, host.length() - 1);
        for (String p : hosts) {
            p = p.toLowerCase(Locale.ROOT);
            if (p.equals("*") || p.equals(host) || p.startsWith("*.") && host.endsWith(p.substring(1))) return true;
        }
        return false;
    }

    private final class Client extends WebViewClient {
        @Override
        public boolean shouldOverrideUrlLoading(WebView web, WebResourceRequest request) {
            if (!request.isForMainFrame()) return false;
            String url = request.getUrl().toString();
            if (navigable(url)) return false;
            reporter.report(Protocol.EVENT_ERROR, "nsgo: the web view may not navigate to " + url
                    + " (the webview capability's hosts)", new Navigation(url, "", false, false, false));
            return true;
        }

        @Override
        public void onPageStarted(WebView web, String url, Bitmap favicon) {
            // android.webkit cannot run a script at document start: only
            // androidx's WebViewCompat.addDocumentStartJavaScript can, which
            // the runtime does without. The script before content (the bridge,
            // then the app's) runs as the page starts loading instead, so a
            // page's earliest scripts may run before it.
            if (!injectedBefore.isEmpty()) web.evaluateJavascript(injectedBefore, null);
            if (!downloadDir.isEmpty()) web.evaluateJavascript(DOWNLOAD_NAMES, null);
            // A blob: URL lives as long as its document: the page leaving takes the downloads still reading theirs.
            for (BlobAnswer waiting : new java.util.ArrayList<>(blobs.values())) {
                waiting.answer(null, "", "", "The page left before its blob: download was read.");
            }
            blobs.clear();
            state(Protocol.EVENT_LOAD_START, url, true);
        }

        @Override
        public void onPageFinished(WebView web, String url) {
            if (!injected.isEmpty()) web.evaluateJavascript(injected, null);
            state(Protocol.EVENT_LOAD, url, false);
        }

        @Override
        public void onReceivedError(WebView web, WebResourceRequest request, WebResourceError error) {
            if (!request.isForMainFrame()) return;
            Navigation nav = new Navigation(request.getUrl().toString(), "", false, false, false);
            reporter.report(Protocol.EVENT_ERROR, String.valueOf(error.getDescription()), nav);
        }
    }

    /** Starts a page download into the app-owned directory. UI thread, where android.webkit calls DownloadListener. */
    private void download(String url, String userAgent, String contentDisposition, String mimetype,
            long contentLength) {
        if (downloadDir.isEmpty() || destroyed) return;
        String id = Long.toString(System.currentTimeMillis(), 36) + "-"
                + Long.toString(DOWNLOAD_SERIAL.incrementAndGet(), 36);
        String scheme = schemeOf(url);
        boolean scripts = view.getSettings().getJavaScriptEnabled();
        switch (scheme) {
            case "http": case "https": {
                String name = downloadName(url, contentDisposition, mimetype);
                File path = start(id, url, name);
                if (path == null) return;
                String cookies = CookieManager.getInstance().getCookie(url);
                Map<String, String> request = requestHeaders(url, userAgent, cookies, Objects.toString(view.getUrl(), ""));
                DOWNLOADS.execute(() -> fetchDownload(id, url, request, path));
                return;
            }
            case "data":
                if (!scripts) {
                    saveData(id, url, "", mimetype);
                    return;
                }
                // The name the page gave the link, which android.webkit leaves out.
                view.evaluateJavascript("(window.__nsgoDownloadNames && window.__nsgoDownloadNames.get("
                        + JSONObject.quote(url) + ")) || ''", name -> saveData(id, url, unquote(name), mimetype));
                return;
            case "blob":
                if (!scripts) {
                    fail(id, url, localName("", mimetype), "A blob: download needs the page's JavaScript to read it.");
                    return;
                }
                saveBlob(id, url, mimetype);
                return;
            default:
                fail(id, url, downloadName(url, contentDisposition, mimetype),
                        "The web view does not save " + scheme + ": downloads.");
        }
    }

    /** Makes the download's directory and reports its start, returning where it saves, or null after reporting a failure. */
    private File start(String id, String url, String name) {
        File directory = new File(downloadDir, id);
        if ((!directory.exists() && !directory.mkdirs()) || !directory.isDirectory()) {
            reportDownload(id, url, name, new File(directory, name).getPath(), 2, 0,
                    "Cannot create the download directory.");
            return null;
        }
        File path = new File(directory, name);
        reportDownload(id, url, name, path.getPath(), 0, 0, "");
        return path;
    }

    /** Reports a download that failed before it started saving. */
    private void fail(String id, String url, String name, String message) {
        reportDownload(id, url, name, new File(new File(downloadDir, id), name).getPath(), 2, 0, message);
    }

    /** Saves a data: URL's bytes, decoded here, under the page's name for it or one after its type. */
    private void saveData(String id, String url, String pageName, String mimetype) {
        if (destroyed) return;
        String type = dataType(url);
        String name = localName(pageName, type.isEmpty() ? mimetype : type);
        File path = start(id, url, name);
        if (path == null) return;
        DOWNLOADS.execute(() -> {
            try {
                save(id, url, path, decodeDataURL(url));
            } catch (IOException | IllegalArgumentException e) {
                failed(id, url, path, 0, String.valueOf(e.getMessage()));
            }
        });
    }

    /**
     * Asks the page for a blob: URL's bytes, which only the document that made it can read: it reads the Blob DOWNLOAD_NAMES kept for the link, so a URL the page revoked right after the click still saves, else fetches the URL, and hands the bytes back base64 through the Blobs interface, under a token pages cannot guess. A page that leaves first fails the download.
     */
    private void saveBlob(String id, String url, String mimetype) {
        String token = UUID.randomUUID().toString();
        blobs.put(token, (base64, pageName, type, error) -> {
            String name = localName(pageName, type == null || type.isEmpty() ? mimetype : type);
            if (base64 == null || error != null && !error.isEmpty()) {
                fail(id, url, name, error != null && !error.isEmpty() ? error : "The page did not read the blob.");
                return;
            }
            File path = start(id, url, name);
            if (path == null) return;
            DOWNLOADS.execute(() -> {
                try {
                    save(id, url, path, Base64.decode(base64, Base64.DEFAULT));
                } catch (IllegalArgumentException e) {
                    failed(id, url, path, 0, String.valueOf(e.getMessage()));
                }
            });
        });
        String script = "(function (url, token) {"
                + " var names = window.__nsgoDownloadNames, name = (names && names.get(url)) || '';"
                + " var answer = function (data, blob, error) { window." + BLOBS + ".answer(token, data, name || (blob && blob.name) || '', (blob && blob.type) || '', error); };"
                + " var kept = window.__nsgoDownloadBlobs, held = kept && kept.get(url);"
                + " if (kept) kept.delete(url);"
                + " (held ? Promise.resolve(held) : fetch(url).then(function (r) { return r.blob(); })).then(function (blob) {"
                + "  var reader = new FileReader();"
                + "  reader.onload = function () { var s = String(reader.result); answer(s.slice(s.indexOf(',') + 1), blob, ''); };"
                + "  reader.onerror = function () { answer(null, blob, String(reader.error)); };"
                + "  reader.readAsDataURL(blob);"
                + " }).catch(function (e) { answer(null, null, String(e)); });"
                + "})(" + JSONObject.quote(url) + ", " + JSONObject.quote(token) + ");";
        view.evaluateJavascript(script, null);
    }

    private static String schemeOf(String url) {
        String scheme = android.net.Uri.parse(url).getScheme();
        return scheme == null ? "" : scheme.toLowerCase(Locale.ROOT);
    }

    /** The JSON string evaluateJavascript answers with, as text; "" for anything else. */
    private static String unquote(String json) {
        if (json == null || !json.startsWith("\"")) return "";
        try {
            return new org.json.JSONArray("[" + json + "]").getString(0);
        } catch (JSONException e) {
            return "";
        }
    }

    private static String downloadName(String url, String contentDisposition, String mimetype) {
        String name = URLUtil.guessFileName(url, contentDisposition, mimetype);
        name = new File(name == null ? "" : name).getName();
        if (name.isEmpty() || ".".equals(name) || "..".equals(name)) return "download";
        return name;
    }

    /** The name a data: or blob: download saves as: the page's, else "download" with its type's extension. Their URLs hold no name. */
    private static String localName(String pageName, String mimetype) {
        String name = new File(pageName == null ? "" : pageName).getName();
        if (!name.isEmpty() && !".".equals(name) && !"..".equals(name)) return name;
        String extension = mimetype == null ? null
                : MimeTypeMap.getSingleton().getExtensionFromMimeType(mimetype.toLowerCase(Locale.ROOT));
        return extension == null || extension.isEmpty() ? "download" : "download." + extension;
    }

    /** The media type a data: URL names, without its parameters, or "". */
    private static String dataType(String url) {
        int comma = url.indexOf(',');
        if (comma < 5) return "";
        String type = url.substring(5, comma);
        int semicolon = type.indexOf(';');
        return (semicolon >= 0 ? type.substring(0, semicolon) : type).trim();
    }

    /** Decodes a data: URL's bytes (RFC 2397): percent-encoded, then base64 when it says so. */
    static byte[] decodeDataURL(String url) throws IOException {
        int comma = url.indexOf(',');
        if (!url.regionMatches(true, 0, "data:", 0, 5) || comma < 0) throw new IOException("Malformed data: URL.");
        boolean base64 = false;
        for (String parameter : url.substring(5, comma).split(";")) {
            if (parameter.trim().equalsIgnoreCase("base64")) base64 = true;
        }
        byte[] bytes = percentDecode(url.substring(comma + 1));
        return base64 ? Base64.decode(bytes, Base64.DEFAULT) : bytes;
    }

    private static byte[] percentDecode(String s) throws IOException {
        byte[] in = s.getBytes(java.nio.charset.StandardCharsets.UTF_8);
        ByteArrayOutputStream out = new ByteArrayOutputStream(in.length);
        for (int i = 0; i < in.length; i++) {
            if (in[i] == '%' && i + 2 < in.length) {
                int high = Character.digit(in[i + 1], 16), low = Character.digit(in[i + 2], 16);
                if (high >= 0 && low >= 0) {
                    out.write(high << 4 | low);
                    i += 2;
                    continue;
                }
            }
            out.write(in[i]);
        }
        return out.toByteArray();
    }

    /**
     * The fields a download's own request sends, as the page's would have: the user agent, the cookies, the view's Headers when the download is on its source's origin, so they reach no other host, and a Referer as browsers send one by default (strict-origin-when-cross-origin): the page without its fragment and user name on its own origin, only its origin elsewhere, and none from https to http.
     */
    private Map<String, String> requestHeaders(String url, String userAgent, String cookies, String page) {
        Map<String, String> request = new HashMap<>();
        if (sameOrigin(url, src)) request.putAll(headers);
        if (userAgent != null && !userAgent.isEmpty()) request.put("User-Agent", userAgent);
        if (cookies != null && !cookies.isEmpty()) request.put("Cookie", cookies);
        android.net.Uri from = android.net.Uri.parse(page);
        String scheme = schemeOf(page);
        if ((scheme.equals("http") || scheme.equals("https")) && from.getHost() != null
                && !(scheme.equals("https") && schemeOf(url).equals("http"))) {
            String origin = scheme + "://" + from.getHost() + (from.getPort() >= 0 ? ":" + from.getPort() : "");
            String query = from.getEncodedQuery();
            request.put("Referer", !sameOrigin(url, page) ? origin + "/"
                    : origin + Objects.toString(from.getEncodedPath(), "") + (query != null ? "?" + query : ""));
        }
        return request;
    }

    private static boolean sameOrigin(String a, String b) {
        android.net.Uri x = android.net.Uri.parse(a), y = android.net.Uri.parse(b);
        return x.getScheme() != null && x.getScheme().equalsIgnoreCase(Objects.toString(y.getScheme(), ""))
                && x.getHost() != null && x.getHost().equalsIgnoreCase(Objects.toString(y.getHost(), ""))
                && port(x) == port(y);
    }

    private static int port(android.net.Uri u) {
        if (u.getPort() >= 0) return u.getPort();
        return "https".equalsIgnoreCase(u.getScheme()) ? 443 : 80;
    }

    /** Fetches an http(s) download again, outside the WebView, which hands its downloads to the app. */
    private void fetchDownload(String id, String url, Map<String, String> request, File path) {
        File partial = new File(path.getParentFile(), "." + path.getName() + ".part");
        HttpURLConnection connection = null;
        long bytes = 0;
        try {
            connection = (HttpURLConnection) new URL(url).openConnection();
            connection.setUseCaches(false);
            for (Map.Entry<String, String> field : request.entrySet()) {
                connection.setRequestProperty(field.getKey(), field.getValue());
            }
            int status = connection.getResponseCode();
            if (status < 200 || status >= 300) throw new IOException("HTTP " + status);
            try (InputStream input = new BufferedInputStream(connection.getInputStream());
                    OutputStream output = new BufferedOutputStream(new FileOutputStream(partial))) {
                byte[] buffer = new byte[64 << 10];
                for (int n; (n = input.read(buffer)) >= 0; ) {
                    if (n == 0) continue;
                    output.write(buffer, 0, n);
                    bytes += n;
                }
            }
            finish(id, url, partial, path, bytes);
        } catch (IOException | RuntimeException e) {
            failed(id, url, path, bytes, String.valueOf(e.getMessage()));
        } finally {
            if (connection != null) connection.disconnect();
            partial.delete();
        }
    }

    /** Writes bytes a data: or blob: download decoded to path and reports it. Download thread. */
    private void save(String id, String url, File path, byte[] data) {
        File partial = new File(path.getParentFile(), "." + path.getName() + ".part");
        try {
            try (OutputStream output = new FileOutputStream(partial)) {
                output.write(data);
            }
            finish(id, url, partial, path, data.length);
        } catch (IOException e) {
            failed(id, url, path, 0, String.valueOf(e.getMessage()));
        } finally {
            partial.delete();
        }
    }

    /** Moves a complete download into place and reports it, unless the view is gone. */
    private void finish(String id, String url, File partial, File path, long bytes) throws IOException {
        if (destroyed) return;
        if (!partial.renameTo(path)) throw new IOException("Cannot finish the download.");
        reportDownload(id, url, path.getName(), path.getPath(), 1, bytes, "");
    }

    private void failed(String id, String url, File path, long bytes, String message) {
        reportDownload(id, url, path.getName(), path.getPath(), 2, bytes, message);
    }

    private void reportDownload(String id, String url, String name, String path, int state, long bytes, String message) {
        if (destroyed) return;
        try {
            JSONObject data = new JSONObject();
            data.put("state", state);
            data.put("id", id);
            data.put("url", url);
            data.put("filename", name);
            data.put("path", path);
            data.put("bytes", bytes);
            data.put("message", message);
            ui.post(() -> {
                if (!destroyed) reporter.download(data.toString());
            });
        } catch (JSONException impossible) {
            throw new AssertionError(impossible);
        }
    }

    /** Reports the page's state as kind and as a navigation state change. */
    private void state(int kind, String url, boolean loading) {
        Navigation nav = new Navigation(url != null ? url : "", Objects.toString(view.getTitle(), ""),
                loading, view.canGoBack(), view.canGoForward());
        reporter.report(kind, "", nav);
        reporter.report(Protocol.EVENT_NAVIGATION_STATE_CHANGE, "", nav);
    }

    /** Applies an ItemWeb's fields, in the order android.go writes them. */
    void configure(DataInputStream in) throws IOException {
        String src = Protocol.readString(in), html = Protocol.readString(in), baseURL = Protocol.readString(in);
        String injected = Protocol.readString(in), injectedBefore = Protocol.readString(in);
        String userAgent = Protocol.readString(in);
        int flags = in.readUnsignedByte();
        boolean noScroll = Protocol.flag(flags, 0), noJavaScript = Protocol.flag(flags, 1);
        restricted = Protocol.flag(flags, 2);
        boolean incognito = Protocol.flag(flags, 3);
        String hosts = Protocol.readString(in).trim();
        this.hosts = hosts.isEmpty() ? new String[0] : hosts.split(" +");
        this.downloadDir = Protocol.readString(in);
        int headerCount = in.readInt();
        Map<String, String> headers = new HashMap<>();
        for (int i = 0; i < headerCount; i++) {
            headers.put(Protocol.readString(in), Protocol.readString(in));
        }

        boolean reload = !configured || !src.equals(this.src) || !html.equals(this.html)
                || !baseURL.equals(this.baseURL) || !injected.equals(this.injected)
                || !injectedBefore.equals(this.injectedBefore) || !headers.equals(this.headers);
        boolean first = !configured;
        configured = true;
        this.src = src;
        this.html = html;
        this.baseURL = baseURL;
        this.injected = injected;
        this.injectedBefore = injectedBefore;
        this.headers = headers;
        this.noScroll = noScroll;
        this.incognito = incognito;

        view.setVerticalScrollBarEnabled(!noScroll);
        view.setHorizontalScrollBarEnabled(!noScroll);
        WebSettings settings = view.getSettings();
        if (first) {
            settings.setCacheMode(incognito ? WebSettings.LOAD_NO_CACHE : WebSettings.LOAD_DEFAULT);
            settings.setSaveFormData(!incognito);
            settings.setDomStorageEnabled(!incognito);
        }
        settings.setJavaScriptEnabled(!noJavaScript);
        if (!userAgent.equals(this.userAgent)) {
            this.userAgent = userAgent;
            settings.setUserAgentString(userAgent.isEmpty() ? null : userAgent); // null: the default
        }
        if (reload) load();
    }

    private void load() {
        // Apps load their own files; pages from elsewhere may not.
        WebSettings settings = view.getSettings();
        if (!html.isEmpty()) {
            settings.setAllowFileAccess(baseURL.startsWith("file:"));
            view.loadDataWithBaseURL(baseURL.isEmpty() ? null : baseURL, html, "text/html", "UTF-8", null);
        } else if (!src.isEmpty()) {
            settings.setAllowFileAccess(src.startsWith("file:"));
            view.loadUrl(src, headers);
        }
    }

    /** Does a core.WebCommand, with value for messages and scripts. */
    void command(int command, String value) {
        switch (command) {
            case Protocol.WEB_RELOAD: view.reload(); break;
            case Protocol.WEB_GO_BACK: view.goBack(); break;
            case Protocol.WEB_GO_FORWARD: view.goForward(); break;
            case Protocol.WEB_STOP_LOADING: view.stopLoading(); break;
            // Go writes the script dispatching a message as the value.
            case Protocol.WEB_POST_MESSAGE:
            case Protocol.WEB_INJECT_JAVASCRIPT:
                view.evaluateJavascript(value, null);
                break;
            default:
        }
    }

    /** Stops the page and frees the WebView. */
    void destroy() {
        destroyed = true;
        blobs.clear();
        view.stopLoading();
        // A WebView is destroyed out of the view hierarchy.
        if (view.getParent() instanceof ViewGroup) ((ViewGroup) view.getParent()).removeView(view);
        view.setWebViewClient(new WebViewClient());
        view.removeJavascriptInterface(INTERFACE);
        view.removeJavascriptInterface(BLOBS);
        view.destroy();
    }
}
