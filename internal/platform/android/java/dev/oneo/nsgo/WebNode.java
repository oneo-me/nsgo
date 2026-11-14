package dev.oneo.nsgo;

import android.annotation.SuppressLint;
import android.content.Context;
import android.graphics.Bitmap;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
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
import android.webkit.URLUtil;
import org.json.JSONException;
import org.json.JSONObject;
import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
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

    /** Starts a page download into the app-owned directory. */
    private void download(String url, String userAgent, String contentDisposition, String mimetype,
            long contentLength) {
        if (downloadDir.isEmpty() || destroyed) return;
        String name = downloadName(url, contentDisposition, mimetype);
        String id = Long.toString(System.currentTimeMillis(), 36) + "-"
                + Long.toString(DOWNLOAD_SERIAL.incrementAndGet(), 36);
        File directory = new File(downloadDir, id);
        if ((!directory.exists() && !directory.mkdirs()) || !directory.isDirectory()) {
            reportDownload(id, url, name, new File(directory, name).getPath(), 2, 0,
                    "Cannot create the download directory.");
            return;
        }
        File path = new File(directory, name);
        reportDownload(id, url, name, path.getPath(), 0, 0, "");
        String cookies = CookieManager.getInstance().getCookie(url);
        DOWNLOADS.execute(() -> fetchDownload(id, url, userAgent, cookies, path));
    }

    private static String downloadName(String url, String contentDisposition, String mimetype) {
        String name = URLUtil.guessFileName(url, contentDisposition, mimetype);
        name = new File(name == null ? "" : name).getName();
        if (name.isEmpty() || ".".equals(name) || "..".equals(name)) return "download";
        return name;
    }

    private void fetchDownload(String id, String url, String userAgent, String cookies, File path) {
        File partial = new File(path.getParentFile(), "." + path.getName() + ".part");
        URLConnection connection = null;
        long bytes = 0;
        try {
            connection = new URL(url).openConnection();
            connection.setUseCaches(false);
            if (userAgent != null && !userAgent.isEmpty()) connection.setRequestProperty("User-Agent", userAgent);
            if (cookies != null && !cookies.isEmpty()) connection.setRequestProperty("Cookie", cookies);
            if (connection instanceof HttpURLConnection) {
                HttpURLConnection http = (HttpURLConnection) connection;
                int status = http.getResponseCode();
                if (status < 200 || status >= 300) throw new IOException("HTTP " + status);
            }
            try (InputStream input = new BufferedInputStream(connection.getInputStream());
                    OutputStream output = new BufferedOutputStream(new FileOutputStream(partial))) {
                byte[] buffer = new byte[64 << 10];
                for (int n; (n = input.read(buffer)) >= 0; ) {
                    if (n == 0) continue;
                    output.write(buffer, 0, n);
                    bytes += n;
                }
            }
            if (destroyed) {
                partial.delete();
                return;
            }
            if (!partial.renameTo(path)) throw new IOException("Cannot finish the download.");
            reportDownload(id, url, path.getName(), path.getPath(), 1, bytes, "");
        } catch (IOException | RuntimeException e) {
            if (!destroyed) {
                partial.delete();
                reportDownload(id, url, path.getName(), path.getPath(), 2, bytes, String.valueOf(e.getMessage()));
            } else {
                partial.delete();
            }
        } finally {
            if (connection instanceof HttpURLConnection) ((HttpURLConnection) connection).disconnect();
        }
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
        view.stopLoading();
        // A WebView is destroyed out of the view hierarchy.
        if (view.getParent() instanceof ViewGroup) ((ViewGroup) view.getParent()).removeView(view);
        view.setWebViewClient(new WebViewClient());
        view.removeJavascriptInterface(INTERFACE);
        view.destroy();
    }
}
