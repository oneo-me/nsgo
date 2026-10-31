package dev.oneo.nsgo;

import android.animation.ValueAnimator;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.ContextWrapper;
import android.content.DialogInterface;
import android.content.res.ColorStateList;
import android.content.res.Configuration;
import android.database.ContentObserver;
import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.graphics.ColorFilter;
import android.graphics.Insets;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.PixelFormat;
import android.graphics.drawable.Drawable;
import android.graphics.drawable.LayerDrawable;
import android.graphics.drawable.RippleDrawable;
import android.net.ConnectivityManager;
import android.net.LinkProperties;
import android.net.Network;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;
import android.text.Editable;
import android.text.InputType;
import android.text.StaticLayout;
import android.text.TextWatcher;
import android.util.DisplayMetrics;
import android.util.Log;
import android.util.SparseArray;
import android.util.TypedValue;
import android.view.Choreographer;
import android.view.DisplayCutout;
import android.view.Gravity;
import android.view.KeyCharacterMap;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.ScaleGestureDetector;
import android.view.VelocityTracker;
import android.view.View;
import android.view.ViewGroup;
import android.view.ViewGroupOverlay;
import android.view.ViewConfiguration;
import android.view.ViewParent;
import android.view.ViewTreeObserver;
import android.view.WindowInsets;
import android.view.WindowInsetsAnimation;
import android.view.WindowInsetsController;
import android.view.accessibility.AccessibilityManager;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.animation.DecelerateInterpolator;
import android.view.animation.LinearInterpolator;
import android.view.animation.PathInterpolator;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.window.OnBackInvokedCallback;
import android.window.OnBackInvokedDispatcher;
import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.BufferedReader;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.File;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.InetAddress;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.function.IntConsumer;

/**
 * A surface showing one NSGo app. It starts the app's Go executable
 * (libnsgo.so in the native library directory), serves its text engine on
 * a reader thread and mounts its batches on the UI thread, one Android view
 * per node placed at the frame the engine computed. Points are
 * density-independent pixels. It reports the environment as it changes,
 * consumes back while the app runs and decodes the images the app fetches.
 * It runs the app's animated values (Animator) and layout animations on
 * Choreographer frames, shows input accessory bars above the keyboard and
 * hosts native components' views (Embedded).
 * It needs API 28, and its activity should handle configuration changes
 * itself and draw edge to edge, as NsgoActivity does.
 */
public class NsgoView extends FrameLayout {
    private static final String TAG = "nsgo";
    private static final int DESTRUCTIVE = 0xffe53935;

    /** Pixels per point; the reader thread reads it too. */
    private volatile float density;
    private final Handler ui = new Handler(Looper.getMainLooper());
    private final InputMethodManager ime;
    /** Mounted nodes by ID, on the UI thread. */
    private final SparseArray<Node> nodes = new SparseArray<>();
    /** The node each node's view is, for a group to find its children's hit slop and areas. */
    private final java.util.HashMap<View, Node> byView = new java.util.HashMap<>();
    /** Prepared text by ID, on the reader thread. */
    private final SparseArray<Text.Prepared> prepared = new SparseArray<>();
    /** Laid-out text by ID: the reader thread adds it, the batch that draws it takes it. */
    private final ConcurrentHashMap<Integer, StaticLayout> layouts = new ConcurrentHashMap<>();
    /** Laid-out text a text view shows, by layout id, for hit testing on the reader thread. */
    private final ConcurrentHashMap<Integer, StaticLayout> shown = new ConcurrentHashMap<>();
    /** The lines a clipped layout shows, by layout id, until its ITEM_TEXT. */
    private final ConcurrentHashMap<Integer, Integer> clips = new ConcurrentHashMap<>();
    /** Decodes images off the UI thread, one at a time. */
    private final ExecutorService decoder = Executors.newSingleThreadExecutor(r -> {
        Thread t = new Thread(r, "nsgo-images");
        t.setDaemon(true);
        return t;
    });
    /** Nodes reporting presses outside them, on the UI thread. */
    private final ArrayList<Node> outside = new ArrayList<>();
    private final int[] location = new int[2];
    private Process process;
    private DataOutputStream out;
    // The test nsgo test -target android runs, and its agent.
    private String test;
    private TestAgent agent;
    // The alerts shown, by their ID, for the test agent to answer.
    final SparseArray<TestAgent.Alert> alerts = new SparseArray<>();

    // The environment, on the UI thread: the insets in pixels and what was last sent.
    private final AccessibilityManager accessibility;
    private final AccessibilityManager.TouchExplorationStateChangeListener screenReader = on -> environment();
    private final ContentObserver motion = new ContentObserver(ui) {
        @Override
        public void onChange(boolean self) {
            environment();
        }
    };
    private int safeTop, safeRight, safeBottom, safeLeft, keyboard;
    private byte[] environment;
    private ConnectivityManager.NetworkCallback network;
    /**
     * The predictive back gesture, on the UI thread: whether one runs, its
     * progress, the screen following it and the one it uncovers, their
     * translations and visibility before it, how far off the screen is and
     * the animation settling it.
     */
    private boolean backGesture;
    private float backProgress, backBase, backBelowBase, backAt;
    private Node backScreen, backBelow;
    private int backBelowVisibility;
    private ValueAnimator backMove;
    /** The OnBackInvokedDispatcher and OnBackInvokedCallback of back on API 33 and later. */
    private Object backDispatcher, backCallback;
    /** The keymap's bindings, oldest first: each key with its flags (1 fires in text, 2 repeats). */
    private String[] keys = new String[0];
    private int[] keyFlags = new int[0];
    private final Set<Integer> consumedKeys = new HashSet<>();
    private int statusBarStyle;
    private boolean statusBarHidden;
    /** The pan, pinch and rotation of the touch in progress, on the UI thread. */
    private final Gestures gestures;
    /** The view the root node shows in, among the accessory bar. */
    private View rootView;

    // Animation, on the UI thread: the graph, whether a frame callback is
    // posted and the app waits for it, the nodes whose frames move or which appear, and the views
    // deleted views leave fading out in their parents' overlays.
    private final Animator animator = new Animator((id, value, finished) ->
            event(Protocol.EVENT_ANIMATION, 0, id, 0, 0, "", 0, 0, 0, 0, 0, 0, value, null, 0, 0, false, "",
                    finished));
    private final Choreographer.FrameCallback tick = this::tick;
    private boolean ticking, frameWanted;
    private final LinkedHashSet<Node> moving = new LinkedHashSet<>();
    private final ArrayList<Ghost> ghosts = new ArrayList<>();
    private final Set<View> ghostViews = new HashSet<>();
    // The layout animation of the batch being mounted: its phases, when it
    // was applied and the nodes it created.
    private LayoutTransition layoutCreate, layoutUpdate, layoutDelete;
    private long batchTime;
    /** The keyboard's last motion, from its WindowInsetsAnimation (API 30). */
    private final LayoutTransition.Keyboard keyboardMotion = new LayoutTransition.Keyboard();
    private final Set<Integer> created = new HashSet<>();

    // The input accessory bar: the input with focus, and the accessory
    // node shown above the keyboard.
    private Node focusedInput, accessoryShown;

    /** The channel to the embedding app, and its settings over the system's. */
    private final Host host = new Host(this);
    private java.util.Locale hostLocale;
    private float hostFontScale;
    private int hostAppearance;
    private File storage;

    /** The appearances setAppearance takes. */
    public static final int APPEARANCE_SYSTEM = 0, APPEARANCE_LIGHT = 1, APPEARANCE_DARK = 2;

    /** Answers the app's calls of a name (core.Scope.CallHost), on the UI thread. */
    public interface HostHandler {
        /** Answers data, as maps, lists and plain values, with reply: then or later, from any thread. */
        void call(Object data, HostReply reply);
    }

    /** The one answer to a call; a post's goes nowhere. */
    public interface HostReply {
        void resolve(Object result);

        void reject(String error);
    }

    /** Sees the URLs the app opens before the system, on the UI thread: true takes one. */
    public interface OpenUrlHandler {
        boolean openUrl(String url);
    }

    /** Hears each failure the app does not catch, on the UI thread. */
    public interface ErrorListener {
        void onError(String message);
    }

    public NsgoView(Context context) {
        this(context, null);
    }

    /** The constructor layouts inflate. */
    public NsgoView(Context context, android.util.AttributeSet attrs) {
        super(context, attrs);
        density = context.getResources().getDisplayMetrics().density;
        ime = context.getSystemService(InputMethodManager.class);
        accessibility = context.getSystemService(AccessibilityManager.class);
        gestures = new Gestures(context);
        setOnApplyWindowInsetsListener((v, insets) -> {
            insets(insets);
            environment();
            updateAccessory();
            return insets;
        });
        // The keyboard's curve, duration and start as it begins to move,
        // before its insets arrive: layout animations on its curve keep
        // pace with it however late their batch comes.
        if (Build.VERSION.SDK_INT >= 30) {
            setWindowInsetsAnimationCallback(new WindowInsetsAnimation.Callback(
                    WindowInsetsAnimation.Callback.DISPATCH_MODE_CONTINUE_ON_SUBTREE) {
                @Override
                public void onPrepare(WindowInsetsAnimation animation) {
                    if ((animation.getTypeMask() & WindowInsets.Type.ime()) == 0) return;
                    keyboardMotion.interpolator = animation.getInterpolator();
                    keyboardMotion.duration = animation.getDurationMillis() * 1_000_000;
                    keyboardMotion.start = System.nanoTime();
                }

                @Override
                public WindowInsets onProgress(WindowInsets insets, List<WindowInsetsAnimation> running) {
                    return insets;
                }
            });
        }
    }

    @Override
    protected void onSizeChanged(int w, int h, int oldw, int oldh) {
        super.onSizeChanged(w, h, oldw, oldh);
        boolean first = process == null;
        if (first) start();
        environment();
        send(first ? Protocol.START : Protocol.RESIZE, w / density, h / density);
    }

    @Override
    protected void onConfigurationChanged(Configuration config) {
        super.onConfigurationChanged(config);
        // Text prepared before a change of density keeps its pixel size
        // until the engine prepares it again.
        float d = getResources().getDisplayMetrics().density;
        if (d != density && process != null) {
            density = d;
            send(Protocol.RESIZE, getWidth() / density, getHeight() / density);
        }
        environment();
        systemBars();
    }

    @Override
    protected void onAttachedToWindow() {
        super.onAttachedToWindow();
        accessibility.addTouchExplorationStateChangeListener(screenReader);
        getContext().getContentResolver().registerContentObserver(
                Settings.Global.getUriFor(Settings.Global.ANIMATOR_DURATION_SCALE), false, motion);
        if (Build.VERSION.SDK_INT >= 33) registerBack();
        systemBars();
    }

    @Override
    protected void onDetachedFromWindow() {
        accessibility.removeTouchExplorationStateChangeListener(screenReader);
        getContext().getContentResolver().unregisterContentObserver(motion);
        unregisterBack();
        super.onDetachedFromWindow();
    }

    /**
     * Runs the test the launch intent names, as nsgo test -target android
     * starts it: its arguments and environment go to the app's process.
     * Only a debuggable build takes one, so no other app can set them.
     */
    void setTest(String encoded) {
        boolean debuggable = (getContext().getApplicationInfo().flags
                & android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0;
        if (encoded != null && debuggable) test = encoded;
    }

    /**
     * Sets the launch parameters the app reads (core.Scope.HostParams), as
     * maps, lists and plain values; before the app starts, as the view
     * first gets a size.
     */
    public void setParams(java.util.Map<String, ?> params) {
        host.params = params == null ? "" : Host.json(params);
    }

    /** Keeps the app's data, its storage included, in dir; before the app starts. The app's files directory by default. */
    public void setStorageDirectory(File dir) {
        storage = dir;
    }

    /** Has the app format for locale in place of the system's; null follows the system again. */
    public void setLocale(java.util.Locale locale) {
        hostLocale = locale;
        environment();
    }

    /** Sets the text size multiplier in place of the system's; 0 follows the system again. */
    public void setFontScale(float scale) {
        hostFontScale = scale;
        environment();
    }

    /**
     * Shows the app light or dark in place of the system's appearance
     * (APPEARANCE_LIGHT, APPEARANCE_DARK); APPEARANCE_SYSTEM follows the
     * system again. An appearance the app forces still wins.
     */
    public void setAppearance(int appearance) {
        hostAppearance = appearance;
        environment();
    }

    /**
     * Gives the app the values the host themes it with (core.Env.Token):
     * CSS colors and other strings, tokens in both appearances and
     * darkTokens over them while the app shows dark.
     */
    public void setTokens(java.util.Map<String, String> tokens, java.util.Map<String, String> darkTokens) {
        host.tokens(tokens, darkTokens);
    }

    /** Sends the app an event (core.Scope.OnHostEvent), kept until it has a listener. */
    public void send(String name, Object data) {
        host.event(name, data);
    }

    /** Answers the app's calls and posts of name; null removes the handler. */
    public void handle(String name, HostHandler handler) {
        host.handle(name, handler);
    }

    public void removeHandler(String name) {
        host.handle(name, null);
    }

    /** Sees the URLs the app opens before the system; null removes the handler. */
    public void setOpenUrlHandler(OpenUrlHandler handler) {
        host.openUrl(handler);
    }

    public void setErrorListener(ErrorListener listener) {
        host.errors = listener;
    }

    /**
     * Registers a component of the host's for this view's app, which shows
     * it with a Native element of name. Names under nsgo/ are NSGo's own.
     */
    public void registerComponent(String name, NsgoComponent.Factory factory) {
        if (name.startsWith("nsgo/")) {
            Log.w(TAG, "component names under nsgo/ are NSGo's own: " + name + " is not registered");
            return;
        }
        host.components.put(name, factory);
    }

    /** The component the host registered as name, or null. */
    Embedded hostComponent(String name, NsgoComponent.Events events) {
        NsgoComponent.Factory factory = host.components.get(name);
        if (factory == null) {
            Log.w(TAG, "no native component " + name + " is registered: it shows an empty box");
            return null;
        }
        NsgoComponent c = factory.create(getContext(), events);
        return c == null ? null : Embedded.hosted(c);
    }

    private void start() {
        File exe = new File(getContext().getApplicationInfo().nativeLibraryDir, "libnsgo.so");
        try {
            ProcessBuilder builder = new ProcessBuilder(exe.getAbsolutePath());
            File data = storage != null ? storage : getContext().getFilesDir();
            builder.environment().put("NSGO_DATA_DIR", data.getAbsolutePath());
            builder.environment().put("NSGO_CACHE_DIR", getContext().getCacheDir().getAbsolutePath());
            if (!host.params.isEmpty()) builder.environment().put("NSGO_HOST_PARAMS", host.params);
            if (test != null) {
                agent = new TestAgent(this);
                TestAgent.apply(test, builder);
            }
            process = builder.start();
        } catch (IOException e) {
            throw new IllegalStateException("cannot start " + exe, e);
        }
        out = new DataOutputStream(new BufferedOutputStream(process.getOutputStream()));
        // The host's handlers and tokens come before the app starts.
        host.start();
        Thread reader = new Thread(this::read, "nsgo-reader");
        reader.setDaemon(true);
        reader.start();
        Thread logger = new Thread(this::log, "nsgo-log");
        logger.setDaemon(true);
        logger.start();
        watchNetwork();
    }

    /** Ends the app's process and destroys its native components. */
    public void stop() {
        if (network != null) {
            getContext().getSystemService(ConnectivityManager.class).unregisterNetworkCallback(network);
            network = null;
        }
        decoder.shutdownNow();
        if (process != null) process.destroy();
        destroyComponents();
    }

    /**
     * Destroys the native components still showing: a stopped app sends no
     * deletes, so this runs as its process ends, on the UI thread.
     */
    private void destroyComponents() {
        for (int i = 0; i < nodes.size(); i++) {
            Node n = nodes.valueAt(i);
            if (n.embedded != null) {
                n.embedded.destroy();
                n.embedded = null;
            }
        }
    }

    private void read() {
        try (DataInputStream in = new DataInputStream(new BufferedInputStream(process.getInputStream()))) {
            while (true) {
                int kind = in.readUnsignedByte();
                byte[] payload = new byte[in.readInt()];
                in.readFully(payload);
                DataInputStream message = new DataInputStream(new ByteArrayInputStream(payload));
                switch (kind) {
                    case Protocol.BATCH:
                        ui.post(() -> mount(payload));
                        break;
                    case Protocol.IMAGE:
                        image(message);
                        break;
                    case Protocol.CANVAS_IMAGE:
                        canvasImage(message);
                        break;
                    case Protocol.FONTS:
                        fonts(message);
                        break;
                    case Protocol.TEST:
                        if (agent != null) agent.handle(message);
                        break;
                    case Protocol.HOST_CALL:
                        host.message(message);
                        break;
                    case Protocol.WANT_FRAME:
                        ui.post(() -> {
                            frameWanted = true;
                            startTicking();
                        });
                        break;
                    default:
                        text(kind, message);
                }
            }
        } catch (IOException e) {
            Log.i(TAG, "app process ended: " + e);
        }
        // Leave back to the system from now on; the app's components go
        // with it.
        ui.post(this::unregisterBack);
        ui.post(this::destroyComponents);
    }

    private void log() {
        try (BufferedReader err = new BufferedReader(new InputStreamReader(process.getErrorStream()))) {
            for (String line; (line = err.readLine()) != null; ) Log.i(TAG, line);
        } catch (IOException ignored) {
            // The process ended.
        }
    }

    /**
     * Sends the environment if it changed: before START, then whenever the
     * insets, the configuration or an accessibility setting change. The
     * keyboard takes the system's default duration.
     */
    private void environment() {
        if (process == null) return;
        Configuration config = getResources().getConfiguration();
        boolean dark = dark();
        followAppearance(dark);
        // Android's reduce-motion setting turns animations off.
        boolean reduceMotion = Settings.Global.getFloat(getContext().getContentResolver(),
                Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f;
        boolean reader = accessibility.isTouchExplorationEnabled();
        boolean rtl = config.getLayoutDirection() == View.LAYOUT_DIRECTION_RTL;
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        DataOutputStream d = new DataOutputStream(bytes);
        try {
            d.writeFloat(safeTop / density);
            d.writeFloat(safeRight / density);
            d.writeFloat(safeBottom / density);
            d.writeFloat(safeLeft / density);
            d.writeFloat(keyboard / density);
            d.writeFloat(hostFontScale > 0 ? hostFontScale : config.fontScale);
            d.writeFloat(density);
            d.writeByte((dark ? 1 : 0) | (reduceMotion ? 2 : 0) | (reader ? 4 : 0) | (rtl ? 8 : 0));
            int idiom = config.smallestScreenWidthDp >= 600 ? Protocol.IDIOM_TABLET : Protocol.IDIOM_PHONE;
            if ((config.uiMode & Configuration.UI_MODE_TYPE_MASK) == Configuration.UI_MODE_TYPE_TELEVISION) {
                idiom = Protocol.IDIOM_TV;
            }
            d.writeByte(idiom);
            java.util.Locale locale = hostLocale != null ? hostLocale : config.getLocales().get(0);
            Protocol.writeString(d, locale.toLanguageTag());
            // The system header's heights, without and with a large title.
            d.writeFloat(Header.BAR);
            d.writeFloat(Header.BAR);
            d.writeInt(Build.VERSION.SDK_INT);
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        byte[] payload = bytes.toByteArray();
        if (Arrays.equals(payload, environment)) return;
        environment = payload;
        write(Protocol.ENVIRONMENT, payload);
    }

    /** The appearance the app forces, as driver.Appearance; 0 follows the system's. */
    private int appearance;

    /** The pick the photo picker answers. */
    private Services.Pick pick;

    /** Dark as shown: the app's forced appearance, else the host's, else the system's. */
    private boolean dark() {
        if (appearance != 0) return appearance == Protocol.APPEARANCE_DARK;
        if (hostAppearance != 0) return hostAppearance == APPEARANCE_DARK;
        int night = getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        return night == Configuration.UI_MODE_NIGHT_YES;
    }

    /** The appearance the theme and the web views follow; null before the first. */
    private Boolean themed;

    /**
     * Gives the standalone activity's theme and the web views the
     * appearance shown, as it changes: a host's activity keeps its own.
     */
    private void followAppearance(boolean dark) {
        if (themed != null && themed == dark) return;
        themed = dark;
        if (activity() instanceof NsgoActivity) ((NsgoActivity) activity()).followAppearance(dark);
        boolean forced = appearance != 0 || hostAppearance != 0;
        for (int i = 0; i < nodes.size(); i++) {
            Node n = nodes.valueAt(i);
            if (n.web != null) n.web.appearance(dark, forced);
            if (n.header != null) n.header.colors(dark);
        }
    }

    /** Hears the activity's results: the photo picker's images. */
    public void activityResult(int request, int result, android.content.Intent data) {
        if (request != Services.PICK_REQUEST || pick == null) return;
        Services.Pick p = pick;
        pick = null;
        java.util.List<android.net.Uri> uris = result == Activity.RESULT_OK ? Services.picked(data, p)
                : new java.util.ArrayList<>();
        Context context = getContext().getApplicationContext();
        // Reading and encoding photos takes a while: off the UI thread.
        new Thread(() -> answerPick(p, Services.prepare(context, uris, p)), "nsgo-pick").start();
    }

    private void answerPick(Services.Pick p, String files) {
        event(Protocol.EVENT_PICK, 0, p.count, 0, 0, "", 0, 0, 0, 0, 0, 0, 0, null, 0, 0, false, files, false);
    }

    /** Reads the safe area and the keyboard's height, in pixels, from the window's insets. */
    private void insets(WindowInsets insets) {
        if (Build.VERSION.SDK_INT >= 30) {
            Insets bars = insets.getInsets(WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout());
            safeTop = bars.top;
            safeRight = bars.right;
            safeBottom = bars.bottom;
            safeLeft = bars.left;
            keyboard = insets.getInsets(WindowInsets.Type.ime()).bottom;
            return;
        }
        // Before API 30 the bottom inset grows by the keyboard while it
        // shows, and the stable one never does. The decor view consumes
        // the stable insets it passes down, so they come from the window.
        WindowInsets root = getRootWindowInsets();
        int bottom = insets.getSystemWindowInsetBottom();
        int stable = (root != null ? root : insets).getStableInsetBottom();
        safeTop = insets.getSystemWindowInsetTop();
        safeRight = insets.getSystemWindowInsetRight();
        safeBottom = Math.min(bottom, stable);
        safeLeft = insets.getSystemWindowInsetLeft();
        keyboard = bottom > stable ? bottom : 0;
        DisplayCutout cutout = insets.getDisplayCutout();
        if (cutout != null) {
            safeTop = Math.max(safeTop, cutout.getSafeInsetTop());
            safeRight = Math.max(safeRight, cutout.getSafeInsetRight());
            safeBottom = Math.max(safeBottom, cutout.getSafeInsetBottom());
            safeLeft = Math.max(safeLeft, cutout.getSafeInsetLeft());
        }
    }

    /**
     * Tells the app the default network's DNS servers as they change: Go
     * outside the runtime has no resolver of the system's to ask. It needs
     * the ACCESS_NETWORK_STATE permission.
     */
    private void watchNetwork() {
        ConnectivityManager connectivity = getContext().getSystemService(ConnectivityManager.class);
        network = new ConnectivityManager.NetworkCallback() {
            @Override
            public void onLinkPropertiesChanged(Network n, LinkProperties properties) {
                dns(properties.getDnsServers());
            }
        };
        try {
            connectivity.registerDefaultNetworkCallback(network);
        } catch (SecurityException e) {
            Log.w(TAG, "the app resolves no host names without ACCESS_NETWORK_STATE", e);
            network = null;
        }
    }

    private void dns(List<InetAddress> servers) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        DataOutputStream d = new DataOutputStream(bytes);
        try {
            d.writeInt(servers.size());
            for (InetAddress server : servers) Protocol.writeString(d, server.getHostAddress());
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        write(Protocol.DNS, bytes.toByteArray());
    }

    /**
     * Serves the engine's text engine. A measurement has an answer, and so
     * has the layout of text with attachments, where they went; the engine
     * waits for both.
     */
    private void text(int kind, DataInputStream in) throws IOException {
        int id = in.readInt();
        switch (kind) {
            case Protocol.PREPARE: {
                String text = Protocol.readString(in);
                Text.Style style = new Text.Style(in, density);
                int maxLines = in.readInt();
                List<Text.Span> spans = new ArrayList<>();
                for (int i = 0, n = in.readInt(); i < n; i++) spans.add(new Text.Span(in, density));
                List<Text.Attachment> attachments = new ArrayList<>();
                for (int i = 0, n = in.readInt(); i < n; i++) attachments.add(new Text.Attachment(in, density));
                int ellipsize = in.readUnsignedByte();
                Text.Shadow shadow = new Text.Shadow(in, density);
                style.readDecoration(in, density);
                for (Text.Span span : spans) span.style.readDecoration(in, density);
                prepared.put(id, new Text.Prepared(text, style, maxLines, ellipsize, shadow, spans, attachments));
                break;
            }
            case Protocol.MEASURE: {
                float maxWidth = in.readFloat(); // NaN does not wrap
                float[] size = prepared.get(id).measure(maxWidth * density);
                send(Protocol.MEASURED, size[0] / density, size[1] / density);
                break;
            }
            case Protocol.LAYOUT: {
                int layout = in.readInt();
                Text.Prepared p = prepared.get(id);
                StaticLayout l = p.layout(in.readFloat() * density);
                layouts.put(layout, l);
                if (p.clipLines() > 0) clips.put(layout, p.clipLines());
                if (p.hasAttachments()) laidOut(Text.attachments(l));
                break;
            }
            case Protocol.LINES:
                lines(Text.lines(layouts.get(id)));
                break;
            case Protocol.CARETS: {
                StaticLayout l = layouts.get(id);
                if (l == null) l = shown.get(id);
                write(Protocol.CARET_LINES, Text.carets(l, density));
                break;
            }
            case Protocol.RELEASE:
                // A prepared text, or a layout no batch will name.
                prepared.remove(id);
                layouts.remove(id);
                clips.remove(id);
                break;
            default:
                Log.w(TAG, "unknown message " + kind);
        }
    }

    /**
     * Shows the bytes Go fetched for an image node's request: decoded on
     * the decoder thread at about the node's size, and dropped once the
     * node has moved on to another request.
     */
    /** Loads the app's font files from the assets as Typefaces of their families, before any text. */
    private void fonts(DataInputStream in) throws IOException {
        for (int i = 0, n = in.readInt(); i < n; i++) {
            String path = Protocol.readString(in), family = Protocol.readString(in);
            int weight = in.readUnsignedShort();
            boolean italic = in.readUnsignedByte() != 0;
            try {
                Text.register(family, weight, italic,
                        android.graphics.Typeface.createFromAsset(getContext().getAssets(), path));
            } catch (RuntimeException e) {
                Log.w(TAG, "font " + path + ": " + e);
            }
        }
    }

    /** Decodes an image canvases draw and draws the canvases again. */
    private void canvasImage(DataInputStream in) throws IOException {
        String key = Protocol.readString(in);
        float scale = in.readFloat();
        byte[] data = new byte[in.readInt()];
        in.readFully(data);
        decoder.execute(() -> {
            android.graphics.Bitmap bitmap = android.graphics.BitmapFactory.decodeByteArray(data, 0, data.length);
            if (bitmap == null) {
                Log.w(TAG, "canvas: image " + key + " cannot be decoded");
                return;
            }
            ui.post(() -> {
                CanvasNode.pictures.put(key, new CanvasNode.Picture(bitmap, scale));
                for (int i = 0; i < nodes.size(); i++) {
                    Node n = nodes.valueAt(i);
                    if (n.canvas != null) n.canvas.invalidate();
                    if (n.shader != null) n.shader.draw();
                }
            });
        });
    }

    private void image(DataInputStream in) throws IOException {
        int id = in.readInt(), request = in.readInt(), part = in.readUnsignedByte();
        float scale = in.readFloat();
        // The size EventLoad reports, as an SVG's in points; 0 for the bitmap's pixels.
        float reportWidth = in.readFloat(), reportHeight = in.readFloat();
        byte[] data = new byte[in.readInt()];
        in.readFully(data);
        ui.post(() -> {
            Node n = nodes.get(id);
            if (n == null || n.image == null || n.image.request != request || decoder.isShutdown()) return;
            ViewGroup.LayoutParams lp = n.view.getLayoutParams();
            DisplayMetrics screen = getResources().getDisplayMetrics();
            int width = lp != null && lp.width > 0 ? lp.width : screen.widthPixels;
            int height = lp != null && lp.height > 0 ? lp.height : screen.heightPixels;
            decoder.execute(() -> {
                ImageNode.Decoded image = ImageNode.decodeDrawable(getContext(), data, scale, width, height);
                ui.post(() -> {
                    if (nodes.get(id) != n || n.image.request != request) return;
                    if (part == Protocol.IMAGE_PLACEHOLDER) {
                        if (image != null) n.image.showPlaceholder(image.drawable);
                        return;
                    }
                    if (image == null) {
                        report(n, Protocol.EVENT_ERROR, 0, 0, "the image cannot be decoded");
                    } else {
                        n.image.showSource(image.drawable);
                        if (n.has(Protocol.EVENT_LOAD)) {
                            boolean given = reportWidth > 0 && reportHeight > 0;
                            event(Protocol.EVENT_LOAD, id, 0, 0, 0, "", given ? reportWidth : image.width,
                                    given ? reportHeight : image.height, 0);
                        }
                    }
                    report(n, Protocol.EVENT_LOAD_END);
                });
            });
        });
    }

    /** Answers a layout with where its attachments went: x and y in points, NaN for ones cut off. */
    private void laidOut(float[] places) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream(4 + places.length * 4);
        DataOutputStream d = new DataOutputStream(bytes);
        try {
            d.writeInt(places.length / 2);
            for (float f : places) d.writeFloat(f / density);
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        write(Protocol.LAID_OUT, bytes.toByteArray());
    }

    /** Answers a LINES message: per line its UTF-8 range, box and metrics, in points. */
    private void lines(float[][] lines) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream(4 + lines.length * 40);
        DataOutputStream d = new DataOutputStream(bytes);
        try {
            d.writeInt(lines.length);
            for (float[] line : lines) {
                d.writeInt((int) line[0]);
                d.writeInt((int) line[1]);
                for (int i = 2; i < line.length; i++) d.writeFloat(line[i] / density);
            }
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        write(Protocol.TEXT_LINES, bytes.toByteArray());
    }

    synchronized void write(int kind, byte[] payload) {
        if (out == null) return;
        try {
            out.writeByte(kind);
            out.writeInt(payload.length);
            out.write(payload);
            out.flush();
        } catch (IOException e) {
            Log.w(TAG, "app process unreachable", e);
        }
    }

    /** Answers the test step id: an error, else the step's data. */
    void testReply(int id, String error, String data) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        DataOutputStream d = new DataOutputStream(bytes);
        try {
            d.writeInt(id);
            Protocol.writeString(d, error);
            Protocol.writeString(d, data);
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        write(Protocol.TEST_REPLY, bytes.toByteArray());
    }

    float density() {
        return density;
    }

    private void send(int kind, float a, float b) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream(8);
        DataOutputStream d = new DataOutputStream(bytes);
        try {
            d.writeFloat(a);
            d.writeFloat(b);
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        write(kind, bytes.toByteArray());
    }

    private void event(int kind, int id, long count, float x, float y, String text, float width, float height,
            int index) {
        event(kind, id, count, x, y, text, width, height, index, 0, 0, 0, 0, null);
    }

    private void event(int kind, int id, long count, float x, float y, String text, float width, float height,
            int index, int phase, float vx, float vy, float value, WebNode.Navigation nav) {
        event(kind, id, count, x, y, text, width, height, index, phase, vx, vy, value, nav, 0, 0, false, "", false);
    }

    private void event(int kind, int id, long count, float x, float y, String text, float width, float height,
            int index, int phase, float vx, float vy, float value, WebNode.Navigation nav,
            long time, long duration, boolean buffering, String data, boolean finished) {
        event(kind, id, count, x, y, text, width, height, index, phase, vx, vy, value, nav, time, duration, buffering,
                data, finished, 0, 0, 0);
    }

    /**
     * Sends an event to the engine: its kind, node, count, point, text,
     * size and index, then a gesture's phase, velocity and value, then a
     * web view's navigation: flags of whether there is one, loading,
     * canGoBack and canGoForward, and if there is, its URL and title; then
     * a video's time and duration in milliseconds and whether it buffers;
     * then a native view's callback arguments as JSON and whether an
     * animation finished; then a pointer's position in its root view and
     * the modifier keys (modifiers' bits).
     */
    private void event(int kind, int id, long count, float x, float y, String text, float width, float height,
            int index, int phase, float vx, float vy, float value, WebNode.Navigation nav,
            long time, long duration, boolean buffering, String data, boolean finished,
            float pageX, float pageY, int mods) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        DataOutputStream d = new DataOutputStream(bytes);
        try {
            d.writeByte(kind);
            d.writeInt(id);
            d.writeInt((int) count);
            d.writeFloat(x);
            d.writeFloat(y);
            Protocol.writeString(d, text);
            d.writeFloat(width);
            d.writeFloat(height);
            d.writeInt(index);
            d.writeByte(phase);
            d.writeFloat(vx);
            d.writeFloat(vy);
            d.writeFloat(value);
            if (nav == null) {
                d.writeByte(0);
            } else {
                d.writeByte(1 | (nav.loading ? 2 : 0) | (nav.canGoBack ? 4 : 0) | (nav.canGoForward ? 8 : 0));
                Protocol.writeString(d, nav.url);
                Protocol.writeString(d, nav.title);
            }
            d.writeLong(time);
            d.writeLong(duration);
            d.writeByte(buffering ? 1 : 0);
            Protocol.writeString(d, data);
            d.writeByte(finished ? 1 : 0);
            d.writeFloat(pageX);
            d.writeFloat(pageY);
            d.writeByte(mods);
        } catch (IOException impossible) {
            throw new AssertionError(impossible);
        }
        write(Protocol.EVENT, bytes.toByteArray());
    }

    private void event(int kind, int id, long count, float x, float y, String text) {
        event(kind, id, count, x, y, text, 0, 0, 0);
    }

    /** Sends an event of kind about n if n reports that kind. */
    private void report(Node n, int kind, float x, float y, String text) {
        if (n.has(kind)) event(kind, n.id, 0, x, y, text);
    }

    private void report(Node n, int kind) {
        report(n, kind, 0, 0, "");
    }

    /** Sends a pointer event of kind about n at e if n reports that kind. */
    private void report(Node n, int kind, MotionEvent e) {
        if (!n.has(kind)) return;
        float[] page = page(n.view, e);
        event(kind, n.id, 0, e.getX() / density, e.getY() / density, "", 0, 0, 0, 0, 0, 0, 0, null, 0, 0, false, "",
                false, page[0], page[1], modifiers(e.getMetaState()));
    }

    /** Sends a press event of kind about n, with where its touch went down. */
    private void pressed(Node n, int kind, int index) {
        if (!n.has(kind)) return;
        event(kind, n.id, n.pressClicks, n.pressX, n.pressY, "", 0, 0, index, 0, 0, 0, 0, null, 0, 0, false, "",
                false, n.pressPageX, n.pressPageY, n.pressMods);
    }

    /** Records where a touch went down on n, for its presses. */
    private void pressedAt(Node n, MotionEvent e) {
        float[] page = page(n.view, e);
        n.pressX = e.getX() / density;
        n.pressY = e.getY() / density;
        n.pressPageX = page[0];
        n.pressPageY = page[1];
        n.pressMods = modifiers(e.getMetaState());
        n.pressClicks = 1;
    }

    /** Stops timing n's long press. */
    private static void stopLongPress(Node n) {
        if (n.longTimer != null) {
            n.view.removeCallbacks(n.longTimer);
            n.longTimer = null;
        }
    }

    /** Forgets n's touch once its press is reported, so a screen reader's click reports none. */
    private static void pressDone(Node n) {
        n.pressX = n.pressY = n.pressPageX = n.pressPageY = 0;
        n.pressMods = n.pressClicks = 0;
    }

    /**
     * Presses at a screen point as a tap there would, for a canvas part a
     * screen reader activates: on the innermost shown view under it that
     * takes presses.
     */
    private boolean pressAtScreen(float rawX, float rawY) {
        Node best = null;
        int depth = -1;
        for (int i = 0; i < nodes.size(); i++) {
            Node o = nodes.valueAt(i);
            if (!(o.has(Protocol.EVENT_PRESS) || o.has(Protocol.EVENT_PRESS_DOWN)) || !under(o.view, rawX, rawY)) continue;
            int d = 0;
            for (Object p = o.view.getParent(); p instanceof View; p = ((View) p).getParent()) d++;
            if (d > depth) {
                best = o;
                depth = d;
            }
        }
        if (best == null) return false;
        int[] at = new int[2];
        best.view.getLocationOnScreen(at);
        float x = (rawX - at[0]) / density, y = (rawY - at[1]) / density;
        int[] page = new int[2];
        getLocationOnScreen(page);
        for (int kind : new int[] {Protocol.EVENT_PRESS_DOWN, Protocol.EVENT_PRESS_UP, Protocol.EVENT_PRESS}) {
            if (best.has(kind)) {
                event(kind, best.id, 1, x, y, "", 0, 0, 0, 0, 0, 0, 0, null, 0, 0, false, "", false,
                        (rawX - page[0]) / density, (rawY - page[1]) / density, 0);
            }
        }
        return true;
    }

    /** Where e is in the root view v is in, the modal around it or this surface, in points. */
    private float[] page(View v, MotionEvent e) {
        View root = this;
        for (View p = v; p != null && p != this; p = p.getParent() instanceof View ? (View) p.getParent() : null) {
            if (p instanceof Layer) {
                root = p;
                break;
            }
        }
        int[] at = new int[2];
        root.getLocationOnScreen(at);
        return new float[] {(e.getRawX() - at[0]) / density, (e.getRawY() - at[1]) / density};
    }

    /** The bits core.Modifiers is sent as: control, alt, shift and meta (the platform key). */
    private static int modifiers(int meta) {
        return ((meta & KeyEvent.META_CTRL_ON) != 0 ? 1 : 0) | ((meta & KeyEvent.META_ALT_ON) != 0 ? 2 : 0)
                | ((meta & KeyEvent.META_SHIFT_ON) != 0 ? 4 : 0) | ((meta & KeyEvent.META_META_ON) != 0 ? 8 : 0);
    }

    private int px(float points) {
        return Math.round(points * density);
    }

    /**
     * Reports back to the engine, which routes it to the topmost overlay,
     * modal or stack. While the app runs it takes back even when nothing
     * there does, for the engine cannot answer; false once the app ended.
     */
    public boolean back() {
        if (process == null || !process.isAlive()) return false;
        event(Protocol.EVENT_BACK, 0, 0, 0, 0, "");
        return true;
    }

    /** Takes back on API 33 and later, where predictive back skips Activity.onBackPressed. */
    private void registerBack() {
        OnBackInvokedDispatcher dispatcher = findOnBackInvokedDispatcher();
        if (dispatcher == null) return;
        OnBackInvokedCallback callback = Build.VERSION.SDK_INT >= 34 ? new BackGesture(this) : this::back;
        dispatcher.registerOnBackInvokedCallback(OnBackInvokedDispatcher.PRIORITY_DEFAULT, callback);
        backDispatcher = dispatcher;
        backCallback = callback;
    }

    /**
     * A predictive back gesture began: the engine answers which screen it
     * pops (ITEM_BACK_PREVIEW), which then follows the gesture.
     */
    void backStarted(float progress) {
        backGesture = true;
        backProgress = progress;
        event(Protocol.EVENT_BACK_STARTED, 0, 0, 0, 0, "");
    }

    void backProgressed(float progress) {
        backProgress = progress;
        if (backScreen != null) placeBack(backDrag(progress));
    }

    void backCancelled() {
        backGesture = false;
        if (backScreen != null) settleBack(false);
        event(Protocol.EVENT_BACK_CANCELLED, 0, 0, 0, 0, "");
    }

    /** The gesture went through: the screen leaves, then the engine pops it. */
    void backInvoked() {
        backGesture = false;
        if (backScreen != null) {
            settleBack(true);
        } else {
            back();
        }
    }

    /** How far off its screen the gesture's progress moves: part of the way, as Android's own does. */
    private static float backDrag(float progress) {
        return progress * 0.4f;
    }

    /** Answers backStarted with screen n, or null when back goes elsewhere first. */
    private void previewBack(Node n) {
        if (!backGesture || n == null || n.view.getParent() == null || backScreen != null || backMove != null) return;
        ViewGroup up = (ViewGroup) n.view.getParent();
        Node below = null;
        for (int i = up.indexOfChild(n.view) - 1; i >= 0 && below == null; i--) {
            View v = up.getChildAt(i);
            for (int j = 0; j < nodes.size(); j++) {
                Node o = nodes.valueAt(j);
                if (o.view == v && o.header != null) {
                    below = o;
                    break;
                }
            }
        }
        backScreen = n;
        backBelow = below;
        backBase = n.view.getTranslationX();
        if (below != null) {
            backBelowBase = below.view.getTranslationX();
            backBelowVisibility = below.view.getVisibility();
            below.view.setVisibility(VISIBLE);
        }
        placeBack(backDrag(backProgress));
    }

    /** Moves the screen a fraction t of the way off, and the one below from a quarter of the way in. */
    private void placeBack(float t) {
        backAt = t;
        float w = backScreen.view.getWidth();
        if (getLayoutDirection() == LAYOUT_DIRECTION_RTL) w = -w;
        backScreen.view.setTranslationX(backBase + t * w);
        if (backBelow != null) backBelow.view.setTranslationX(backBelowBase - (1 - t) * w * 0.25f);
    }

    /** Ends the gesture's motion: off, reporting the back, or back in place. */
    private void settleBack(boolean pop) {
        Node screen = backScreen, below = backBelow;
        float base = backBase, belowBase = backBelowBase;
        int belowVisibility = backBelowVisibility;
        ValueAnimator a = ValueAnimator.ofFloat(backAt, pop ? 1 : 0);
        a.setDuration(Math.round((pop ? 300 : 200) * Math.max(Math.abs((pop ? 1 : 0) - backAt), 0.2f)));
        a.setInterpolator(new DecelerateInterpolator(1.5f));
        a.addUpdateListener(x -> placeBack((float) x.getAnimatedValue()));
        a.addListener(new android.animation.AnimatorListenerAdapter() {
            @Override
            public void onAnimationEnd(android.animation.Animator animation) {
                backMove = null;
                backScreen = backBelow = null;
                screen.view.setTranslationX(base);
                if (below != null) {
                    below.view.setTranslationX(belowBase);
                    if (!pop) below.view.setVisibility(belowVisibility);
                }
                if (pop) {
                    // Until the engine hides it, as it pops it.
                    screen.view.setVisibility(GONE);
                    event(Protocol.EVENT_BACK, screen.id, 0, 0, 0, "");
                }
            }
        });
        backMove = a;
        a.start();
    }

    private void unregisterBack() {
        if (backCallback == null || Build.VERSION.SDK_INT < 33) return;
        OnBackInvokedDispatcher dispatcher = (OnBackInvokedDispatcher) backDispatcher;
        dispatcher.unregisterOnBackInvokedCallback((OnBackInvokedCallback) backCallback);
        backDispatcher = null;
        backCallback = null;
    }

    /**
     * Sees each touch before the views below: a touch starting reports
     * presses outside overlays and collects the gesture views under it as
     * they see it; each later one moves the gestures first, so a pan that
     * activates stops the scroll views around it from taking the move.
     */
    @Override
    public boolean dispatchTouchEvent(MotionEvent e) {
        boolean down = e.getActionMasked() == MotionEvent.ACTION_DOWN;
        if (selectTouch(e)) return true;
        touches(e);
        if (down) {
            pressOutside(e);
            gestures.reset();
        } else {
            gestures.track(e);
        }
        boolean handled = super.dispatchTouchEvent(e);
        if (down) gestures.down(e);
        // While a node listens, the surface keeps the fingers though no
        // view took the down: a responder that claims on a move, as a
        // toast's swipe, hears the moves only then.
        return handled || touching;
    }

    /**
     * The selection of an area, and its touch: a long press waited for at
     * the down, and whether the touch made the selection and now moves its
     * end (Selection.java).
     */
    private Selection selection;
    private Runnable selectLater;
    private boolean selectingTouch, selectMoved;
    private float selectDownX, selectDownY;
    private final ViewTreeObserver.OnScrollChangedListener selectionScrolled = this::refreshSelection;

    /**
     * Follows a touch for selections: a down elsewhere clears the
     * selection, and one held on an area selects the word there, then
     * takes the rest of the touch, which moves the selection's end, from
     * the views. It reports whether it took the event.
     */
    private boolean selectTouch(MotionEvent e) {
        float x = e.getX(), y = e.getY();
        switch (e.getActionMasked()) {
            case MotionEvent.ACTION_DOWN: {
                if (selection != null && !selection.onHandle(x, y)) clearSelection();
                View area = selection == null ? areaAt(x, y) : null;
                if (area == null) return false;
                selectDownX = x;
                selectDownY = y;
                long downTime = e.getDownTime();
                selectLater = () -> {
                    selectLater = null;
                    Selection s = new Selection(this, area);
                    if (!s.selectWordAt(selectDownX, selectDownY)) return;
                    selection = s;
                    selectingTouch = true;
                    selectMoved = false;
                    getViewTreeObserver().addOnScrollChangedListener(selectionScrolled);
                    performHapticFeedback(android.view.HapticFeedbackConstants.LONG_PRESS);
                    // The views lose the touch the selection took.
                    MotionEvent cancel = MotionEvent.obtain(downTime, android.os.SystemClock.uptimeMillis(),
                            MotionEvent.ACTION_CANCEL, selectDownX, selectDownY, 0);
                    super.dispatchTouchEvent(cancel);
                    cancel.recycle();
                };
                postDelayed(selectLater, ViewConfiguration.getLongPressTimeout());
                return false;
            }
            case MotionEvent.ACTION_MOVE:
                if (selectingTouch) {
                    // The word stays until the finger moves on from it.
                    if (selectMoved || Math.hypot(x - selectDownX, y - selectDownY)
                            > ViewConfiguration.get(getContext()).getScaledTouchSlop()) {
                        selectMoved = true;
                        selection.moveHandle(1, x, y);
                    }
                    return true;
                }
                if (selectLater != null && Math.hypot(x - selectDownX, y - selectDownY)
                        > ViewConfiguration.get(getContext()).getScaledTouchSlop()) {
                    removeCallbacks(selectLater);
                    selectLater = null;
                }
                return false;
            case MotionEvent.ACTION_UP:
            case MotionEvent.ACTION_CANCEL:
                if (selectLater != null) {
                    removeCallbacks(selectLater);
                    selectLater = null;
                }
                if (selectingTouch) {
                    selectingTouch = false;
                    if (selection != null) selection.showToolbar();
                    return true;
                }
                return false;
            default:
                return selectingTouch;
        }
    }

    /**
     * The selection area a touch at x, y goes to: the outermost area around
     * the view hit, or the selectable text hit, unless a field, a page, a
     * native view, a pressable view or one that follows pans keeps it.
     */
    private View areaAt(float x, float y) {
        if (rootView == null) return null;
        List<View> under = new ArrayList<>();
        int[] at = new int[2], self = new int[2];
        getLocationInWindow(self);
        View v = rootView;
        while (v != null) {
            under.add(v);
            View next = null;
            if (v instanceof ViewGroup) {
                ViewGroup g = (ViewGroup) v;
                for (int i = g.getChildCount() - 1; i >= 0; i--) {
                    View c = g.getChildAt(i);
                    if (c.getVisibility() != VISIBLE) continue;
                    c.getLocationInWindow(at);
                    float cx = x + self[0] - at[0], cy = y + self[1] - at[1];
                    if (cx >= 0 && cy >= 0 && cx < c.getWidth() && cy < c.getHeight()) {
                        next = c;
                        break;
                    }
                }
            }
            v = next;
        }
        View area = null;
        boolean taken = false;
        for (int i = under.size() - 1; i >= 0; i--) {
            View u = under.get(i);
            Node n = byView.get(u);
            if (u instanceof EditText || u instanceof android.webkit.WebView) {
                taken = true;
                continue;
            }
            if (n == null) continue;
            boolean keeps = n.embedded != null || n.has(Protocol.EVENT_PRESS) || n.has(Protocol.EVENT_LONG_PRESS)
                    || n.has(Protocol.EVENT_PRESS_DOWN) || n.has(Protocol.EVENT_PAN);
            if (keeps) {
                if (area == null) taken = true;
            } else if (n.selectionArea) {
                if (area != null || !taken) area = u;
            } else if (u instanceof TextNode && ((TextNode) u).selectable == Protocol.SELECTABLE_ON && !taken && area == null) {
                area = u;
            }
        }
        return area;
    }

    /** Follows the selection's texts after a batch or a scroll. */
    private void refreshSelection() {
        if (selection != null && !selection.refresh()) clearSelection();
    }

    private void clearSelection() {
        if (selection == null) return;
        selection.clear();
        selection = null;
        getViewTreeObserver().removeOnScrollChangedListener(selectionScrolled);
    }

    /** Whether the fingers are reported as EventTouch: from a first finger down while a node listens. */
    private boolean touching;
    /** The last fingers reported, for a cancel when a scroll view takes them. */
    private String fingers = "";
    /** Whether the runtime itself is stopping the scroll views, for the touch responder. */
    private boolean blockingNative;

    /**
     * Reports the fingers of e as EventTouch while a node listens: each as
     * "id x y changed" in points in this view, joined by ";", with the
     * phase as core.GesturePhase.
     */
    private void touches(MotionEvent e) {
        int action = e.getActionMasked();
        if (action == MotionEvent.ACTION_DOWN) touching = listensTouches();
        if (!touching) return;
        int phase;
        int changed = -1; // every finger for a move
        switch (action) {
            case MotionEvent.ACTION_DOWN:
            case MotionEvent.ACTION_POINTER_DOWN:
                phase = 1;
                changed = e.getActionIndex();
                break;
            case MotionEvent.ACTION_MOVE:
                phase = 2;
                break;
            case MotionEvent.ACTION_UP:
            case MotionEvent.ACTION_POINTER_UP:
                phase = 3;
                changed = e.getActionIndex();
                break;
            case MotionEvent.ACTION_CANCEL:
                phase = 4;
                break;
            default:
                return;
        }
        StringBuilder b = new StringBuilder();
        for (int i = 0; i < e.getPointerCount(); i++) {
            if (i > 0) b.append(';');
            b.append(e.getPointerId(i)).append(' ').append(e.getX(i) / density).append(' ')
                    .append(e.getY(i) / density).append(' ').append(changed < 0 || changed == i ? 1 : 0);
        }
        fingers = b.toString();
        event(Protocol.EVENT_TOUCH, 0, 0, 0, 0, "", 0, 0, 0, phase, 0, 0, 0, null, e.getEventTime(), 0, false,
                fingers, false);
        if (action == MotionEvent.ACTION_UP || action == MotionEvent.ACTION_CANCEL) touching = false;
    }

    /** A scroll view took the touches: the nodes hear them cancelled. */
    private void touchesTaken() {
        if (!touching || blockingNative) return;
        touching = false;
        event(Protocol.EVENT_TOUCH, 0, 0, 0, 0, "", 0, 0, 0, 4, 0, 0, 0, null, android.os.SystemClock.uptimeMillis(), 0,
                false, fingers.replaceAll(" [01](;|$)", " 1$1"), false);
    }

    private boolean listensTouches() {
        for (int i = 0; i < nodes.size(); i++) {
            if (nodes.valueAt(i).has(Protocol.EVENT_TOUCH)) return true;
        }
        return false;
    }

    /** Reports a touch starting outside a node that listens for one, at its point in points. */
    private void pressOutside(MotionEvent e) {
        int x = Math.round(e.getRawX()), y = Math.round(e.getRawY());
        for (Node n : outside) {
            if (n.view.isShown() && !contains(n.view, x, y)) {
                event(Protocol.EVENT_PRESS_OUTSIDE, n.id, 0, e.getX() / density, e.getY() / density, "");
            }
        }
    }

    /** Whether the screen point (x, y) is on view or on a descendant it lets draw outside it. */
    private boolean contains(View view, int x, int y) {
        if (view.getVisibility() != VISIBLE) return false;
        view.getLocationOnScreen(location);
        if (x >= location[0] && y >= location[1]
                && x < location[0] + view.getWidth() && y < location[1] + view.getHeight()) {
            return true;
        }
        if (!(view instanceof ViewGroup) || view.getClipToOutline() || ((ViewGroup) view).getClipChildren()) {
            return false;
        }
        ViewGroup group = (ViewGroup) view;
        for (int i = 0; i < group.getChildCount(); i++) {
            if (contains(group.getChildAt(i), x, y)) return true;
        }
        return false;
    }

    private void mount(byte[] batch) {
        DataInputStream in = new DataInputStream(new ByteArrayInputStream(batch));
        try {
            int root = in.readInt();
            layoutCreate = layoutUpdate = layoutDelete = null;
            if (in.readUnsignedByte() != 0) {
                LayoutTransition create = LayoutTransition.read(in), update = LayoutTransition.read(in);
                LayoutTransition delete = LayoutTransition.read(in);
                // Reduced motion turns layout animations off.
                if (!reduceMotion()) {
                    layoutCreate = create;
                    layoutUpdate = update;
                    layoutDelete = delete;
                    batchTime = System.nanoTime();
                    for (LayoutTransition t : new LayoutTransition[] {create, update, delete}) {
                        if (t != null) t.alignToKeyboard(keyboardMotion, batchTime);
                    }
                }
            }
            for (int i = 0, count = in.readInt(); i < count; i++) {
                int kind = in.readUnsignedByte();
                int id = in.readInt();
                if (kind == Protocol.ITEM_CREATE) {
                    Node n = create(id, in.readUnsignedByte());
                    if (layoutCreate != null || layoutUpdate != null) created.add(id);
                    // Show the root at once, so text fields can take focus
                    // while the rest of its first batch mounts.
                    if (id == root) show(n.view);
                } else {
                    apply(kind, nodes.get(id), in);
                }
            }
        } catch (IOException e) {
            Log.e(TAG, "bad batch", e);
        }
        layoutCreate = layoutUpdate = layoutDelete = null;
        created.clear();
        // Bindings and values change in batches too.
        applyAnimated();
        if (animator.running()) startTicking();
        // The selection keeps to its texts, and goes with its area.
        if (selection != null) post(this::refreshSelection);
    }

    private void show(View root) {
        if (root.getParent() == this) return;
        if (rootView != null && rootView.getParent() == this) removeView(rootView);
        detach(root);
        // Under the accessory bar.
        addView(root, 0);
        rootView = root;
    }

    /** Whether Android's reduce-motion setting, no animations, is on. */
    private boolean reduceMotion() {
        return Settings.Global.getFloat(getContext().getContentResolver(),
                Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f;
    }

    private void apply(int kind, Node n, DataInputStream in) throws IOException {
        switch (kind) {
            case Protocol.ITEM_INSERT: {
                Node parent = nodes.get(in.readInt());
                Node before = nodes.get(in.readInt()); // none for ID 0: last
                n.up = parent;
                detach(n.view);
                ViewGroup group = parent.children;
                group.addView(n.view, before == null ? -1 : group.indexOfChild(before.view));
                if (n.modal >= 0 && !n.presented) {
                    n.presented = true;
                    present(n);
                }
                // The top view of each subtree the batch creates appears.
                if (layoutCreate != null && created.contains(n.id) && !created.contains(parent.id)) {
                    n.appear = layoutCreate;
                    n.appearStart = batchTime;
                    moving.add(n);
                    paint(n);
                    startTicking();
                }
                break;
            }
            case Protocol.ITEM_REMOVE:
                detach(n.view);
                break;
            case Protocol.ITEM_DELETE:
                // Dropping the node drops its views, its text layout and its
                // bitmaps. A view inside one fading out goes with it.
                if (!vanish(n) && !inGhost(n.view)) detach(n.view);
                nodes.remove(n.id);
                byView.remove(n.view);
                if (n.laid != 0) shown.remove(n.laid);
                outside.remove(n);
                moving.remove(n);
                animator.forget(n.id);
                if (n == focusedInput) focusedInput = null;
                if (n == accessoryShown) {
                    if (n.view.getParent() == this) removeView(n.view);
                    accessoryShown = null;
                }
                if (n.animator != null) n.animator.cancel();
                if (n.keyframer != null) n.keyframer.cancel();
                if (n.web != null) n.web.destroy();
                if (n.holder != null) detach(n.holder);
                if (n.video != null) n.video.destroy();
                if (n.embedded != null) n.embedded.destroy();
                break;
            case Protocol.ITEM_FRAME:
                setFrame(n, new float[] {in.readFloat(), in.readFloat(), in.readFloat(), in.readFloat()});
                break;
            case Protocol.ITEM_OVERFLOW: {
                float[] area = {in.readFloat(), in.readFloat(), in.readFloat(), in.readFloat()};
                n.area = area[2] > 0 && area[3] > 0 ? area : null;
                break;
            }
            case Protocol.ITEM_VIEW_STYLE:
                style(n, in);
                break;
            case Protocol.ITEM_TEXT: {
                n.text = Protocol.readString(in);
                int laid = in.readInt();
                StaticLayout layout = layouts.remove(laid);
                if (n.laid != 0) shown.remove(n.laid);
                n.laid = laid;
                if (layout != null) shown.put(laid, layout);
                Integer clip = clips.remove(laid);
                int x = px(in.readFloat()), y = px(in.readFloat());
                ((TextNode) n.view).setLayout(layout, x, y, clip != null ? clip : 0);
                describe(n);
                break;
            }
            case Protocol.ITEM_INPUT:
                input(n, in);
                updateAccessory();
                break;
            case Protocol.ITEM_INPUT_VALUE: {
                String value = Protocol.readString(in);
                long count = in.readInt() & 0xffffffffL;
                n.field.setValue(value, count);
                break;
            }
            case Protocol.ITEM_SELECTION: {
                int start = in.readInt(), end = in.readInt();
                if (n.field != null) n.field.select(start, end);
                break;
            }
            case Protocol.ITEM_EVENTS:
                events(n, in.readLong());
                break;
            case Protocol.ITEM_VIEW_PROPS:
                props(n, in);
                break;
            case Protocol.ITEM_SCROLL_CONFIG: {
                Scrolling.Config c = Scrolling.Config.read(in);
                scroller(n, c.horizontal, c.zooms());
                ((Scrolling.Scroller) n.scroller).configure(c);
                break;
            }
            case Protocol.ITEM_SCROLL_CONTENT: {
                int w = px(in.readFloat()), h = px(in.readFloat());
                n.children.setLayoutParams(new LayoutParams(w, h, Gravity.LEFT | Gravity.TOP));
                break;
            }
            case Protocol.ITEM_FOCUS:
                if (n.field != null && n.field.requestFocus()) {
                    EditText field = n.field;
                    field.post(() -> ime.showSoftInput(field, InputMethodManager.SHOW_IMPLICIT));
                }
                break;
            case Protocol.ITEM_BLUR:
                if (n.field != null && n.field.hasFocus()) {
                    n.field.clearFocus();
                    ime.hideSoftInputFromWindow(n.field.getWindowToken(), 0);
                }
                break;
            case Protocol.ITEM_IMAGE: {
                int request = in.readInt();
                int mode = in.readUnsignedByte();
                float blur = in.readFloat() * density;
                int fade = in.readInt();
                // Each axis of the position: a fraction of the free space plus points.
                float xFraction = in.readFloat(), x = in.readFloat() * density;
                float yFraction = in.readFloat(), y = in.readFloat() * density;
                n.image.configure(request, mode, blur, fade);
                n.image.position(xFraction, x, yFraction, y);
                break;
            }
            case Protocol.ITEM_MODAL:
                n.modal = in.readUnsignedByte();
                break;
            case Protocol.ITEM_ALERT:
                alert(in);
                break;
            case Protocol.ITEM_DISMISS_KEYBOARD: {
                View focused = findFocus();
                if (focused != null) {
                    ime.hideSoftInputFromWindow(focused.getWindowToken(), 0);
                    focused.clearFocus();
                }
                break;
            }
            case Protocol.ITEM_SCROLL_TO: {
                float x = in.readFloat(), y = in.readFloat();
                boolean animated = Protocol.flag(in.readUnsignedByte(), 0);
                Scrolling.Scroller scroller = (Scrolling.Scroller) n.scroller;
                // Scroll views clamp to their content, which takes its new
                // size at the next layout. Counted as applied, the scroll
                // reports carry it.
                afterLayout(n.scroller, () -> {
                    n.applied++;
                    scroller.scrollToPoints(x, y, animated);
                });
                break;
            }
            case Protocol.ITEM_SCROLL_BY: {
                float x = in.readFloat(), y = in.readFloat();
                Scrolling.Scroller scroller = (Scrolling.Scroller) n.scroller;
                afterLayout(n.scroller, () -> {
                    n.applied++;
                    scroller.scrollByPoints(x, y);
                });
                break;
            }
            case Protocol.ITEM_STICKY_CELLS: {
                // Each cell: its id, start and end, the animated node its
                // inset follows (0: none) and the inset start counts.
                float[] cells = new float[in.readInt() * 5];
                n.stickyBound = false;
                for (int i = 0; i < cells.length; i += 5) {
                    cells[i] = in.readInt();
                    cells[i + 1] = in.readFloat();
                    cells[i + 2] = in.readFloat();
                    cells[i + 3] = in.readInt();
                    cells[i + 4] = in.readFloat();
                    n.stickyBound |= cells[i + 3] != 0;
                }
                if (n.sticky != null) {
                    for (int i = 0; i < n.sticky.length; i += 5) stick(nodes.get((int) n.sticky[i]), 0, false);
                }
                n.sticky = cells;
                pinSticky(n);
                break;
            }
            case Protocol.ITEM_ZOOM_TO: {
                float scale = in.readFloat();
                boolean centered = Protocol.flag(in.readUnsignedByte(), 0);
                float x = in.readFloat(), y = in.readFloat();
                boolean animated = Protocol.flag(in.readUnsignedByte(), 0);
                if (n.scroller instanceof Scrolling.ZoomScroll) {
                    Scrolling.ZoomScroll z = (Scrolling.ZoomScroll) n.scroller;
                    afterLayout(z, () -> z.zoomTo(scale, centered, x, y, animated));
                }
                break;
            }
            case Protocol.ITEM_CANVAS: {
                byte[] ops = new byte[in.readInt()];
                in.readFully(ops);
                int slots = in.readInt();
                int[] offsets = new int[slots], nodes = new int[slots];
                boolean[] colors = new boolean[slots];
                for (int i = 0; i < slots; i++) {
                    offsets[i] = in.readInt();
                    colors[i] = in.readUnsignedByte() != 0;
                    nodes[i] = in.readInt();
                }
                CanvasNode.Part[] parts = new CanvasNode.Part[in.readInt()];
                for (int i = 0; i < parts.length; i++) {
                    float x = in.readFloat(), y = in.readFloat(), w = in.readFloat(), h = in.readFloat();
                    int role = in.readUnsignedByte();
                    String label = Protocol.readString(in), testID = Protocol.readString(in);
                    boolean selected = in.readUnsignedByte() != 0;
                    parts[i] = new CanvasNode.Part(x, y, w, h, role, label, testID, selected);
                }
                n.canvas.setOps(ops, offsets, colors, parts);
                animator.bindCanvas(n.id, nodes);
                applyAnimated();
                break;
            }
            case Protocol.ITEM_SHADER:
                n.shader.set(in);
                animator.bindCanvas(n.id, n.shader.boundNodes);
                applyAnimated();
                break;
            case Protocol.ITEM_SET_CLIPBOARD:
                clipboard().setPrimaryClip(ClipData.newPlainText("", Protocol.readString(in)));
                break;
            case Protocol.ITEM_READ_CLIPBOARD: {
                long count = in.readInt() & 0xffffffffL;
                int what = in.readUnsignedByte();
                // Android answers null unless the app has focus.
                String[] text = {""}, data = {""};
                boolean[] has = {false};
                Services.readClipboard(getContext(), clipboard(), what, text, data, has);
                event(Protocol.EVENT_CLIPBOARD, 0, count, 0, 0, text[0], 0, 0, 0, 0, 0, 0, 0, null, 0, 0, false,
                        data[0], has[0]);
                break;
            }
            case Protocol.ITEM_SET_CLIPBOARD_IMAGE:
                Services.setClipboardImage(getContext(), clipboard(), Protocol.readBytes(in));
                break;
            case Protocol.ITEM_OPEN_URL: {
                long count = in.readInt() & 0xffffffffL;
                Context context = activity() != null ? activity() : getContext();
                boolean opened = Services.openURL(context, Protocol.readString(in));
                event(Protocol.EVENT_OPEN_URL, 0, count, 0, 0, "", 0, 0, 0, 0, 0, 0, 0, null, 0, 0, false, "", opened);
                break;
            }
            case Protocol.ITEM_PICK_IMAGES: {
                Services.Pick p = new Services.Pick(in.readInt() & 0xffffffffL, in.readInt(), in.readInt(), in.readFloat(),
                        Protocol.flag(in.readUnsignedByte(), 0));
                if (pick != null) answerPick(pick, "");
                Activity activity = activity();
                if (activity != null && Services.startPick(activity, p)) pick = p;
                else answerPick(p, "");
                break;
            }
            case Protocol.ITEM_APPEARANCE:
                appearance = in.readUnsignedByte();
                systemBars();
                environment();
                break;
            case Protocol.ITEM_QUIT: {
                Activity activity = activity();
                if (activity != null) activity.finishAndRemoveTask();
                break;
            }
            case Protocol.ITEM_POPUP_MENU:
                // Desktops' only; Go writes no fields.
                break;
            case Protocol.ITEM_ANNOUNCE:
                announceForAccessibility(Protocol.readString(in));
                break;
            case Protocol.ITEM_BLOCK_NATIVE:
                // The touch responder stops the scroll views around it.
                if (n != null && n.view.getParent() != null) {
                    blockingNative = true;
                    n.view.getParent().requestDisallowInterceptTouchEvent(true);
                    blockingNative = false;
                }
                break;
            case Protocol.ITEM_KEYMAP: {
                int count = in.readInt();
                String[] bindings = new String[count];
                int[] flags = new int[count];
                for (int i = 0; i < count; i++) {
                    bindings[i] = Protocol.readString(in);
                    flags[i] = in.readUnsignedByte();
                }
                keys = bindings;
                keyFlags = flags;
                break;
            }
            case Protocol.ITEM_STATUS_BAR:
                statusBarStyle = in.readUnsignedByte();
                // Hidden, then animated, which the system does as it likes.
                statusBarHidden = Protocol.flag(in.readUnsignedByte(), 0);
                systemBars();
                break;
            case Protocol.ITEM_TITLE: {
                // Phones show no title; the window's names it to TalkBack.
                String title = Protocol.readString(in);
                Activity activity = activity();
                if (activity != null) activity.setTitle(title);
                break;
            }
            case Protocol.ITEM_WEB:
                n.web.configure(in);
                break;
            case Protocol.ITEM_WEB_COMMAND: {
                int command = in.readUnsignedByte();
                n.web.command(command, Protocol.readString(in));
                break;
            }
            case Protocol.ITEM_VIDEO:
                n.video.configure(in);
                break;
            case Protocol.ITEM_VIDEO_SEEK:
                n.video.seek(in.readLong());
                break;
            case Protocol.ITEM_NATIVE:
                embed(n, in);
                break;
            case Protocol.ITEM_NATIVE_COMMAND: {
                String name = Protocol.readString(in);
                Object args = Embedded.parse(Protocol.readString(in));
                if (n.embedded != null) {
                    n.embedded.command(name, args);
                    nativeSize(n);
                }
                break;
            }
            case Protocol.ITEM_ACCESSIBILITY_LABEL:
                n.label = Protocol.readString(in);
                describe(n);
                break;
            case Protocol.ITEM_ACCESSIBILITY_FOCUS: {
                View view = n.accessible();
                view.performAccessibilityAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS, null);
                break;
            }
            case Protocol.ITEM_ANIMATION:
                for (int i = 0, count = in.readInt(); i < count; i++) animator.apply(Animator.readOp(in));
                break;
            case Protocol.ITEM_SCREEN:
                screen(n, in);
                break;
            case Protocol.ITEM_WINDOW:
            case Protocol.ITEM_MENU:
            case Protocol.ITEM_HISTORY:
                // Desktop windows' and menus', and the web's history; Go
                // writes no fields.
                break;
            case Protocol.ITEM_BACK_PREVIEW:
                previewBack(n);
                break;
            case Protocol.ITEM_BACK_UNHANDLED: {
                // As the system's back does from API 31: a task's root
                // activity goes to the background, keeping the app; any
                // other finishes.
                Activity activity = activity();
                if (activity != null && !(activity.isTaskRoot() && activity.moveTaskToBack(true))) {
                    activity.finish();
                }
                break;
            }
            default:
                // Go writes no fields for the kinds it does not encode.
                Log.w(TAG, "unknown mount item " + kind);
        }
    }

    /**
     * Draws n's look: its style, or the transition moving it, with its
     * animated values in place of their properties and a layout animation's
     * appearance over them.
     */
    private void paint(Node n) {
        if (n.shown == null) {
            if (n.values == null && n.appear == null) return;
            n.shown = new Look();
        }
        if (n.background == null) {
            n.background = new Box();
            n.view.setBackground(n.background);
        }
        Look l = n.shown;
        if (n.values != null) l = l.animated(n.values, density);
        if (n.keyframer != null) {
            l = n.keyframes.apply(l, (float) n.keyframer.getAnimatedValue(), n.view.getHeight());
        }
        if (l.cameraDistance == 0 && n.up != null && n.up.shown != null && n.up.shown.childPerspective > 0) {
            // Seen in its parent's perspective, about its own pivot.
            if (l == n.shown) l = l.copy();
            l.cameraDistance = n.up.shown.childPerspective;
        }
        if (n.appear != null) {
            if (l == n.shown) l = l.copy();
            float p = (float) n.appear.at(System.nanoTime() - n.appearStart);
            if (n.appear.property == LayoutTransition.SCALE) {
                l.scaleX *= p;
                l.scaleY *= p;
            } else {
                l.opacity *= p;
            }
        }
        l.apply(n.view, n.background);
        if (n.shift != 0) {
            if (n.shiftAcross) n.view.setTranslationX(n.view.getTranslationX() + n.shift * density);
            else n.view.setTranslationY(n.view.getTranslationY() + n.shift * density);
        }
    }

    /** Moves scroll node n's sticky cells for its offset: each by clamp(offset - start, 0, end - start) points. */
    private void pinSticky(Node n) {
        if (n.sticky == null || !(n.scroller instanceof Scrolling.Scroller)) return;
        Scrolling.Scroller s = (Scrolling.Scroller) n.scroller;
        boolean horizontal = n.scroller instanceof Scrolling.HScroll;
        float at = horizontal ? s.offsetX() : s.offsetY();
        for (int i = 0; i < n.sticky.length; i += 5) {
            float start = n.sticky[i + 1], end = n.sticky[i + 2], offset = at;
            if (n.sticky[i + 3] != 0) {
                // The inset follows its animated value.
                double v = animator.valueOf((int) n.sticky[i + 3]);
                if (!Double.isNaN(v)) offset += (float) v - n.sticky[i + 4];
            }
            stick(nodes.get((int) n.sticky[i]), Math.min(Math.max(offset - start, 0), Math.max(end - start, 0)), horizontal);
        }
    }

    /** Moves sticky cell c by d points along its list from its frame. */
    private void stick(Node c, float d, boolean horizontal) {
        if (c == null || c.shift == d && c.shiftAcross == horizontal) return;
        c.shift = d;
        c.shiftAcross = horizontal;
        if (c.shown != null) {
            paint(c);
        } else {
            c.view.setTranslationX(horizontal ? d * density : 0);
            c.view.setTranslationY(horizontal ? 0 : d * density);
        }
    }

    /** Draws the views whose animated values changed. */
    private void applyAnimated() {
        animator.changed((id, values) -> {
            Node n = nodes.get(id);
            if (n == null) return;
            n.values = values.set != 0 ? values : null;
            if (n.view instanceof TextNode) {
                ((TextNode) n.view).setTint(values.has(Animator.PROP_TEXT_COLOR) ? Text.argb(values.textColor) : 0);
            }
            paint(n);
        });
        for (int i = 0; i < nodes.size(); i++) {
            Node s = nodes.valueAt(i);
            if (s.stickyBound) pinSticky(s);
        }
        animator.changedCanvases((id, c) -> {
            Node n = nodes.get(id);
            if (n == null) return;
            if (n.canvas != null) n.canvas.patch(c.nums, c.colors);
            if (n.shader != null) n.shader.patch(c.nums);
            if (n.bound != null && n.embedded != null) n.embedded.animate(n.bound, c.nums, c.colors);
        });
    }

    private void startTicking() {
        if (ticking) return;
        ticking = true;
        Choreographer.getInstance().postFrameCallback(tick);
    }

    /**
     * Tells the app a frame began if it waits for one, steps every animation
     * once a frame, and stops when none runs.
     */
    private void tick(long now) {
        ticking = false;
        if (frameWanted) {
            frameWanted = false;
            write(Protocol.FRAME, new byte[0]);
        }
        if (animator.running()) animator.step(now);
        applyAnimated();
        boolean more = animator.running();
        for (Node n : new ArrayList<>(moving)) {
            if (nodes.get(n.id) != n) {
                moving.remove(n);
                continue;
            }
            if (n.motion != null) {
                long elapsed = now - n.motionStart;
                if (n.motion.done(elapsed)) {
                    n.motion = null;
                    place(n, n.to);
                } else {
                    place(n, LayoutTransition.lerp(n.from, n.to, n.motion.at(elapsed)));
                }
            }
            if (n.appear != null) {
                if (n.appear.done(now - n.appearStart)) n.appear = null;
                paint(n);
            }
            if (n.motion == null && n.appear == null) moving.remove(n);
        }
        more |= !moving.isEmpty();
        for (int i = ghosts.size() - 1; i >= 0; i--) {
            Ghost g = ghosts.get(i);
            long elapsed = now - g.start;
            float p = (float) g.transition.at(elapsed);
            if (g.transition.property == LayoutTransition.SCALE) {
                float f = Math.max(1 - p, 0.0001f);
                g.view.setScaleX(g.scaleX * f);
                g.view.setScaleY(g.scaleY * f);
            } else {
                g.view.setAlpha(Math.max(0, g.alpha * (1 - p)));
            }
            if (g.transition.done(elapsed)) {
                g.overlay.remove(g.view);
                ghostViews.remove(g.view);
                ghosts.remove(i);
            }
        }
        more |= !ghosts.isEmpty();
        if (more) startTicking();
    }

    /**
     * Moves n to frame f in points; under a layout animation's Update a
     * node that showed before the batch moves there from where it shows.
     */
    private void setFrame(Node n, float[] f) {
        float[] target = n.motion != null ? n.to : n.frame;
        if (target != null && Arrays.equals(target, f)) return;
        if (layoutUpdate != null && n.frame != null && !created.contains(n.id) && n != accessoryShown) {
            n.from = n.frame.clone();
            n.to = f;
            n.motion = layoutUpdate;
            n.motionStart = batchTime;
            moving.add(n);
            startTicking();
            return;
        }
        n.motion = null;
        place(n, f);
    }

    /**
     * Applies an ItemScreen: screen n's system header, over its content,
     * holding the header views the core sized.
     */
    private void screen(Node n, DataInputStream in) throws IOException {
        if (n.header == null) {
            n.header = new Header(getContext(), density, () -> event(Protocol.EVENT_BACK, n.id, 0, 0, 0, ""));
            n.header.colors(dark());
            n.children.addView(n.header.toolbar);
        }
        n.header.configure(in, id -> {
            Node item = nodes.get(id);
            if (item == null) return null;
            if (item.holder == null) {
                item.holder = new Header.Holder(getContext());
                detach(item.view);
                item.holder.addView(item.view);
                float[] f = item.frame;
                Header.size(item.holder, f != null ? px(f[2]) : 1, f != null ? px(f[3]) : 1);
            }
            return item.holder;
        });
        // The bar as the toolbar lays out what it holds, for the core to
        // keep the content below.
        if (n.header.toolbar.getVisibility() == View.VISIBLE) {
            float bar = n.header.measure(n.frame != null ? px(n.frame[2]) : getWidth());
            if (bar != n.header.reported) {
                n.header.reported = bar;
                event(Protocol.EVENT_SCREEN_HEADER, n.id, 0, 0, 0, "", 0, 0, 0, 0, 0, 0, bar, null);
            }
        }
    }

    /** Places n's view at frame f in points, rounding its edges to pixels. */
    private void place(Node n, float[] f) {
        n.frame = f;
        if (n.holder != null) {
            Header.size(n.holder, px(f[0] + f[2]) - px(f[0]), px(f[1] + f[3]) - px(f[1]));
        }
        if (n == accessoryShown) {
            placeAccessory();
            return;
        }
        int left = px(f[0]), top = px(f[1]);
        LayoutParams lp = new LayoutParams(px(f[0] + f[2]) - left, px(f[1] + f[3]) - top, Gravity.LEFT | Gravity.TOP);
        lp.leftMargin = left;
        lp.topMargin = top;
        n.view.setLayoutParams(lp);
    }

    /** A deleted view fading or shrinking out in its parent's overlay, its node gone. */
    private static final class Ghost {
        View view;
        ViewGroupOverlay overlay;
        float alpha, scaleX, scaleY;
        LayoutTransition transition;
        long start;
    }

    /**
     * Keeps a view the batch deletes drawn where it was while it fades or
     * shrinks out under the layout animation's Delete, and reports whether
     * it does. Only the top view of a deleted subtree does; the views it
     * holds go with it.
     */
    private boolean vanish(Node n) {
        if (layoutDelete == null || !(n.view.getParent() instanceof ViewGroup) || inGhost(n.view)) return false;
        ViewGroup parent = (ViewGroup) n.view.getParent();
        if (!parent.isAttachedToWindow() || n.view.getVisibility() != VISIBLE) return false;
        Ghost g = new Ghost();
        g.view = n.view;
        g.overlay = parent.getOverlay();
        g.alpha = n.view.getAlpha();
        g.scaleX = n.view.getScaleX();
        g.scaleY = n.view.getScaleY();
        g.transition = layoutDelete;
        g.start = batchTime;
        // The overlay draws it where it was and passes it no touches.
        g.overlay.add(n.view);
        ghosts.add(g);
        ghostViews.add(n.view);
        startTicking();
        return true;
    }

    /** Whether view is inside a view fading out. */
    private boolean inGhost(View view) {
        if (ghostViews.isEmpty()) return false;
        for (ViewParent p = view.getParent(); p instanceof View; p = p.getParent()) {
            if (ghostViews.contains(p)) return true;
        }
        return false;
    }

    /**
     * Shows the accessory bar of the input with focus above the keyboard
     * while the keyboard shows, as wide as the surface; hides it otherwise.
     * The core lays accessories out but never inserts them.
     */
    private void updateAccessory() {
        Node bar = null;
        if (focusedInput != null && focusedInput.accessory != 0 && keyboard > 0) {
            bar = nodes.get(focusedInput.accessory);
        }
        if (accessoryShown != null && accessoryShown != bar && accessoryShown.view.getParent() == this) {
            removeView(accessoryShown.view);
        }
        accessoryShown = bar;
        keepFocusedClear();
        if (bar == null) return;
        if (bar.view.getParent() != this) {
            detach(bar.view);
            addView(bar.view);
        }
        placeAccessory();
    }

    /** The scroll view keeping the focused field clear of the keyboard. */
    private Scrolling.VScroll keyboardScroll;

    /**
     * Keeps the focused field clear of the keyboard and the accessory bar
     * above it, through the innermost vertical scroll view around it.
     */
    private void keepFocusedClear() {
        View field = focusedInput != null && keyboard > 0 ? focusedInput.field : null;
        Scrolling.VScroll scroll = null;
        for (android.view.ViewParent p = field != null ? field.getParent() : null; p != null; p = p.getParent()) {
            if (p instanceof Scrolling.VScroll) {
                scroll = (Scrolling.VScroll) p;
                break;
            }
        }
        if (keyboardScroll != null && keyboardScroll != scroll) keyboardScroll.d.keepClear(null, 0);
        keyboardScroll = scroll;
        if (scroll == null) return;
        int[] at = new int[2];
        getLocationInWindow(at);
        int bar = accessoryShown != null && accessoryShown.frame != null ? px(accessoryShown.frame[3]) : 0;
        scroll.d.keepClear(field, at[1] + getHeight() - keyboard - bar);
    }

    /** Places the accessory bar on the keyboard: its top keyboard pixels plus its height above the bottom. */
    private void placeAccessory() {
        Node bar = accessoryShown;
        int height = bar.frame != null ? px(bar.frame[3]) : 0;
        LayoutParams lp = new LayoutParams(getWidth(), height, Gravity.LEFT | Gravity.TOP);
        lp.topMargin = getHeight() - keyboard - height;
        bar.view.setLayoutParams(lp);
    }

    /**
     * Applies an ItemNative: the first one names the View class to build
     * by reflection, which fills the node's view; each sets the props that
     * changed through their setters. An empty class keeps the view.
     */
    private void embed(Node n, DataInputStream in) throws IOException {
        String className = Protocol.readString(in);
        if (!className.isEmpty() && !n.embedTried) {
            n.embedTried = true;
            Embedded.Emitter emit = (name, args) ->
                    event(Protocol.EVENT_NATIVE, n.id, 0, 0, 0, name, 0, 0, 0, 0, 0, 0, 0, null, 0, 0, false, args,
                            false);
            if (className.startsWith("host:")) {
                // A component of the embedding app's own: it emits one value.
                n.embedded = hostComponent(className.substring(5), (name, data) -> emit.emit(name, Host.json(data)));
            } else {
                n.embedded = Embedded.build(getContext(), className, emit);
            }
            if (n.embedded != null) {
                n.children.addView(n.embedded.view, new LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT));
            }
        }
        for (int i = 0, count = in.readInt(); i < count; i++) {
            String name = Protocol.readString(in);
            Object value = Embedded.parse(Protocol.readString(in));
            if (n.embedded != null) n.embedded.setProp(name, value);
        }
        if (Protocol.flag(in.readUnsignedByte(), 0)) {
            // Animated props, driven as a canvas's arguments are.
            int count = in.readInt();
            n.bound = new Embedded.Bound(count);
            int[] nodes = new int[count];
            for (int i = 0; i < count; i++) {
                n.bound.names[i] = Protocol.readString(in);
                nodes[i] = in.readInt();
                n.bound.colors[i] = Protocol.flag(in.readUnsignedByte(), 0);
            }
            animator.bindCanvas(n.id, nodes);
            applyAnimated();
        }
        nativeSize(n);
    }

    /** Reports a native view's natural size when it changed, for the element's open dimensions. */
    private void nativeSize(Node n) {
        if (n.embedded == null) return;
        int[] size = n.embedded.measure();
        if (size[0] == n.embedded.width && size[1] == n.embedded.height) return;
        n.embedded.width = size[0];
        n.embedded.height = size[1];
        event(Protocol.EVENT_NATIVE_SIZE, n.id, 0, 0, 0, "", size[0] / density, size[1] / density, 0);
    }

    private static void detach(View view) {
        ViewParent parent = view.getParent();
        if (parent instanceof ViewGroup) ((ViewGroup) parent).removeView(view);
    }

    /** Runs r once view has been laid out with what it was last given. */
    private static void afterLayout(View view, Runnable r) {
        if (view.isLaidOut() && !view.isLayoutRequested()) {
            r.run();
            return;
        }
        view.getViewTreeObserver().addOnGlobalLayoutListener(new ViewTreeObserver.OnGlobalLayoutListener() {
            @Override
            public void onGlobalLayout() {
                view.getViewTreeObserver().removeOnGlobalLayoutListener(this);
                r.run();
            }
        });
    }

    private ClipboardManager clipboard() {
        return getContext().getSystemService(ClipboardManager.class);
    }

    private Activity activity() {
        for (Context c = getContext(); c instanceof ContextWrapper; c = ((ContextWrapper) c).getBaseContext()) {
            if (c instanceof Activity) return (Activity) c;
        }
        return null;
    }

    /**
     * Lays the view out behind the system bars and draws their icons to
     * contrast with the status bar style or, for Auto, the appearance; and
     * hides the status bar when asked.
     */
    private void systemBars() {
        boolean dark = dark();
        boolean darkIcons = statusBarStyle == Protocol.STATUS_BAR_DARK
                || statusBarStyle != Protocol.STATUS_BAR_LIGHT && !dark;
        if (Build.VERSION.SDK_INT >= 30) {
            WindowInsetsController bars = getWindowInsetsController();
            if (bars == null) return;
            int light = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS
                    | WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;
            int appearance = (darkIcons ? WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS : 0)
                    | (dark ? 0 : WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS);
            bars.setSystemBarsAppearance(appearance, light);
            bars.setSystemBarsBehavior(WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            if (statusBarHidden) bars.hide(WindowInsets.Type.statusBars());
            else bars.show(WindowInsets.Type.statusBars());
            return;
        }
        int flags = SYSTEM_UI_FLAG_LAYOUT_STABLE | SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                | SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION;
        if (darkIcons) flags |= SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
        if (!dark) flags |= SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
        if (statusBarHidden) flags |= SYSTEM_UI_FLAG_FULLSCREEN;
        getRootView().setSystemUiVisibility(flags);
    }

    /**
     * Consumes a key press matching a keymap binding, named with "mod" for
     * Control or with the modifiers as they are, and reports it as EventKey.
     */
    @Override
    public boolean dispatchKeyEvent(KeyEvent e) {
        int code = e.getKeyCode();
        if (e.getAction() == KeyEvent.ACTION_UP && consumedKeys.remove(code)) return true;
        if (e.getAction() == KeyEvent.ACTION_DOWN && keys.length > 0) {
            String[] names = names(e);
            // The newest binding of the key that fires here, as core.FindKey decides.
            for (int i = keys.length - 1; names != null && i >= 0; i--) {
                if (!keys[i].equals(names[0]) && !keys[i].equals(names[1])) continue;
                if (editing() && (keyFlags[i] & 1) == 0) continue;
                consumedKeys.add(code);
                if (e.getRepeatCount() == 0 || (keyFlags[i] & 2) != 0) event(Protocol.EVENT_KEY, 0, 0, 0, 0, keys[i]);
                return true;
            }
        }
        return super.dispatchKeyEvent(e);
    }

    /** The press's names: with "mod" for Control, and with the modifiers as they are. */
    private static String[] names(KeyEvent e) {
        String key = keyName(e);
        if (key == null) return null;
        String rest = (e.isAltPressed() ? "alt+" : "") + (e.isShiftPressed() ? "shift+" : "")
                + (e.isMetaPressed() ? "meta+" : "") + key;
        return e.isCtrlPressed() ? new String[] {"mod+" + rest, "ctrl+" + rest} : new String[] {rest, rest};
    }

    /** Whether a text field or a web view has the keyboard focus. */
    private boolean editing() {
        View focused = findFocus();
        return focused instanceof android.widget.EditText || focused instanceof android.webkit.WebView;
    }

    /** The key's name as core.NormalizeKey writes it, or null for keys it has none for. */
    private static String keyName(KeyEvent e) {
        int code = e.getKeyCode();
        if (code >= KeyEvent.KEYCODE_A && code <= KeyEvent.KEYCODE_Z) {
            return String.valueOf((char) ('a' + code - KeyEvent.KEYCODE_A));
        }
        if (code >= KeyEvent.KEYCODE_0 && code <= KeyEvent.KEYCODE_9) {
            return String.valueOf((char) ('0' + code - KeyEvent.KEYCODE_0));
        }
        if (code >= KeyEvent.KEYCODE_F1 && code <= KeyEvent.KEYCODE_F12) return "f" + (code - KeyEvent.KEYCODE_F1 + 1);
        switch (code) {
            case KeyEvent.KEYCODE_ENTER: case KeyEvent.KEYCODE_NUMPAD_ENTER: return "enter";
            case KeyEvent.KEYCODE_ESCAPE: return "escape";
            case KeyEvent.KEYCODE_TAB: return "tab";
            case KeyEvent.KEYCODE_SPACE: return "space";
            case KeyEvent.KEYCODE_DEL: return "backspace";
            case KeyEvent.KEYCODE_FORWARD_DEL: return "delete";
            case KeyEvent.KEYCODE_DPAD_UP: return "up";
            case KeyEvent.KEYCODE_DPAD_DOWN: return "down";
            case KeyEvent.KEYCODE_DPAD_LEFT: return "left";
            case KeyEvent.KEYCODE_DPAD_RIGHT: return "right";
            case KeyEvent.KEYCODE_MOVE_HOME: return "home";
            case KeyEvent.KEYCODE_MOVE_END: return "end";
            case KeyEvent.KEYCODE_PAGE_UP: return "pageup";
            case KeyEvent.KEYCODE_PAGE_DOWN: return "pagedown";
            default: break;
        }
        // Other keys by the character they type unmodified, such as "/".
        int c = e.getKeyCharacterMap().get(code, 0);
        if (c <= ' ' || (c & KeyCharacterMap.COMBINING_ACCENT) != 0) return null;
        return new String(Character.toChars(c)).toLowerCase(Locale.ROOT);
    }

    /**
     * Animates a modal in as it is first inserted. Android's reduce-motion
     * setting, no animations, makes either animation instant.
     */
    private void present(Node n) {
        View view = n.view;
        if (n.modal == Protocol.MODAL_SLIDE) {
            ViewGroup.LayoutParams lp = view.getLayoutParams();
            view.setTranslationY(lp != null && lp.height > 0 ? lp.height : getHeight());
            view.animate().translationY(0).setDuration(300).setInterpolator(new DecelerateInterpolator());
        } else if (n.modal == Protocol.MODAL_FADE) {
            float opacity = view.getAlpha();
            view.setAlpha(0);
            view.animate().alpha(opacity).setDuration(200);
        }
    }

    /**
     * Shows an alert without waiting for it and answers with the index of
     * the action chosen, or, when dismissed, of the cancel action or -1.
     */
    private void alert(DataInputStream in) throws IOException {
        int id = in.readInt();
        String title = Protocol.readString(in), message = Protocol.readString(in);
        int count = in.readInt();
        String[] labels = new String[count];
        int[] roles = new int[count];
        int cancel = -1;
        for (int i = 0; i < count; i++) {
            labels[i] = Protocol.readString(in);
            roles[i] = in.readUnsignedByte();
            if (cancel < 0 && roles[i] == Protocol.ALERT_CANCEL) cancel = i;
        }
        boolean sheet = in.readUnsignedByte() != 0, cancelable = in.readUnsignedByte() != 0;
        boolean[] answered = {false};
        IntConsumer answer = index -> {
            if (answered[0]) return;
            answered[0] = true;
            alerts.remove(id);
            event(Protocol.EVENT_ALERT, 0, id, 0, 0, "", 0, 0, index);
        };
        AlertDialog.Builder b = new AlertDialog.Builder(getContext(), dark()
                ? android.R.style.Theme_DeviceDefault_Dialog_Alert
                : android.R.style.Theme_DeviceDefault_Light_Dialog_Alert);
        int[] buttons = count <= 3 && !sheet ? buttons(count, cancel) : null;
        if (buttons == null) {
            // A message hides a list, so the title takes it.
            String heading = title.isEmpty() || message.isEmpty() ? title + message : title + "\n\n" + message;
            if (!heading.isEmpty()) b.setTitle(heading);
            // An action sheet's cancel action is its dismissal.
            int[] indices = new int[count];
            int n = 0;
            for (int i = 0; i < count; i++) {
                if (!sheet || roles[i] != Protocol.ALERT_CANCEL) indices[n++] = i;
            }
            String[] items = new String[n];
            for (int i = 0; i < n; i++) items[i] = labels[indices[i]];
            b.setItems(items, (d, which) -> answer.accept(indices[which]));
        } else {
            if (!title.isEmpty()) b.setTitle(title);
            if (!message.isEmpty()) b.setMessage(message);
            for (int i = 0; i < count; i++) {
                int index = i;
                DialogInterface.OnClickListener click = (d, which) -> answer.accept(index);
                switch (buttons[i]) {
                    case DialogInterface.BUTTON_POSITIVE: b.setPositiveButton(labels[i], click); break;
                    case DialogInterface.BUTTON_NEGATIVE: b.setNegativeButton(labels[i], click); break;
                    default: b.setNeutralButton(labels[i], click);
                }
            }
        }
        // Back and touches outside choose the cancel action, or close it
        // without one when it is cancelable.
        int dismissed = cancel;
        b.setCancelable(sheet || cancelable || cancel >= 0);
        b.setOnCancelListener(d -> answer.accept(dismissed));
        AlertDialog dialog = b.show();
        alerts.put(id, new TestAgent.Alert(dialog, buttons, listIndices(count, sheet, roles)));
        for (int i = 0; buttons != null && i < count; i++) {
            if (roles[i] == Protocol.ALERT_DESTRUCTIVE) dialog.getButton(buttons[i]).setTextColor(DESTRUCTIVE);
        }
    }

    /** The action of each item of an alert shown as a list. */
    private static int[] listIndices(int count, boolean sheet, int[] roles) {
        int[] indices = new int[count];
        int n = 0;
        for (int i = 0; i < count; i++) {
            if (!sheet || roles[i] != Protocol.ALERT_CANCEL) indices[n++] = i;
        }
        return java.util.Arrays.copyOf(indices, n);
    }

    /**
     * The button of each of count actions: the cancel action's is the
     * negative one, and the others take the positive, the neutral, then the
     * negative one in order.
     */
    private static int[] buttons(int count, int cancel) {
        int[] order = {
            DialogInterface.BUTTON_POSITIVE, DialogInterface.BUTTON_NEUTRAL, DialogInterface.BUTTON_NEGATIVE,
        };
        int[] buttons = new int[count];
        for (int i = 0, next = 0; i < count; i++) {
            buttons[i] = i == cancel ? DialogInterface.BUTTON_NEGATIVE : order[next++];
        }
        return buttons;
    }

    /** Reads a frame in points into layout params in pixels, rounding its edges. */
    private LayoutParams frame(DataInputStream in) throws IOException {
        float x = in.readFloat(), y = in.readFloat(), w = in.readFloat(), h = in.readFloat();
        int left = px(x), top = px(y);
        LayoutParams lp = new LayoutParams(px(x + w) - left, px(y + h) - top, Gravity.LEFT | Gravity.TOP);
        lp.leftMargin = left;
        lp.topMargin = top;
        return lp;
    }

    private Node create(int id, int kind) {
        Context context = getContext();
        // A scroll view leaves clipping its content to its parent, so a
        // scroll node clips it.
        View view = kind == Protocol.KIND_TEXT ? new TextNode(context)
                : kind == Protocol.KIND_IMAGE ? new ImageNode(context)
                : kind == Protocol.KIND_CANVAS ? new CanvasNode(context)
                : kind == Protocol.KIND_SCROLL ? new FrameLayout(context)
                : kind == Protocol.KIND_MODAL ? new Layer(context) : new Group(context);
        Node n = new Node(id, view);
        if (view instanceof CanvasNode) ((CanvasNode) view).presser = this::pressAtScreen;
        byView.put(view, n);
        if (view instanceof Group) ((Group) view).node = n;
        switch (kind) {
            case Protocol.KIND_TEXT:
                // Its children are the views of its attachments.
                n.children = (ViewGroup) view;
                break;
            case Protocol.KIND_IMAGE:
                n.image = (ImageNode) view;
                break;
            case Protocol.KIND_CANVAS:
                n.canvas = (CanvasNode) view;
                break;
            case Protocol.KIND_INPUT:
                n.field = field(n);
                ((ViewGroup) view).addView(n.field);
                break;
            case Protocol.KIND_WEB:
                // The WebView fills the node's view, which draws its style.
                n.web = new WebNode(context, (k, text, nav) -> {
                    if (n.has(k)) event(k, n.id, 0, 0, 0, text, 0, 0, 0, 0, 0, 0, 0, nav);
                });
                n.web.appearance(dark(), appearance != 0 || hostAppearance != 0);
                n.children = (ViewGroup) view;
                n.children.addView(n.web.view, new LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT));
                break;
            case Protocol.KIND_VIDEO:
                // The player fills the node's view, which draws its style
                // and takes its presses.
                n.video = new VideoNode(context, new VideoNode.Reporter() {
                    @Override
                    public boolean wants(int k) {
                        return n.has(k);
                    }

                    @Override
                    public void report(int k, String text, long time, long duration, int width, int height,
                            boolean buffering) {
                        event(k, n.id, 0, 0, 0, text, width / density, height / density, 0, 0, 0, 0, 0, null,
                                time, duration, buffering, "", false);
                    }
                });
                n.children = (ViewGroup) view;
                n.children.addView(n.video, new LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT));
                break;
            case Protocol.KIND_SHADER:
                // A TextureView draws no background, so the shader fills the
                // node's view, which draws its style.
                n.shader = new ShaderNode(context);
                n.children = (ViewGroup) view;
                n.children.addView(n.shader, new LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT));
                break;
            case Protocol.KIND_SCROLL:
                n.children = new Content(context);
                scroller(n, false, false);
                break;
            default:
                n.children = (ViewGroup) view;
        }
        n.accessible().setAccessibilityDelegate(new View.AccessibilityDelegate() {
            @Override
            public void onInitializeAccessibilityNodeInfo(View host, AccessibilityNodeInfo info) {
                super.onInitializeAccessibilityNodeInfo(host, info);
                describe(n, info);
            }

            @Override
            public boolean performAccessibilityAction(View host, int action, android.os.Bundle args) {
                String name = actionName(n, action);
                if (name != null && n.offers(name) && n.has(Protocol.EVENT_ACCESSIBILITY_ACTION)) {
                    event(Protocol.EVENT_ACCESSIBILITY_ACTION, n.id, 0, 0, 0, name);
                    return true;
                }
                return super.performAccessibilityAction(host, action, args);
            }
        });
        nodes.put(id, n);
        return n;
    }

    private InputField field(Node n) {
        InputField field = new InputField(getContext(), new InputField.Listener() {
            @Override public boolean has(int kind) {
                return n.has(kind);
            }

            @Override public void send(int kind, long count, int index, String text, boolean finished) {
                // Text changes always go: the engine keeps the text.
                if (kind != Protocol.EVENT_CHANGE_TEXT && !n.has(kind)) return;
                event(kind, n.id, count, 0, 0, text, 0, 0, index, 0, 0, 0, 0, null, 0, 0, false, "", finished);
            }

            @Override public void caret(float x, float y, float w, float h) {
                if (n.has(Protocol.EVENT_CARET_MOVE)) {
                    event(Protocol.EVENT_CARET_MOVE, n.id, 0, x, y, "", w, h, 0);
                }
            }

            @Override public void paste(String text, String data) {
                if (n.has(Protocol.EVENT_PASTE)) {
                    event(Protocol.EVENT_PASTE, n.id, 0, 0, 0, text, 0, 0, 0, 0, 0, 0, 0, null, 0, 0, false, data, false);
                }
            }

            @Override public void blur() {
                if (n.field.hasFocus()) {
                    n.field.clearFocus();
                    ime.hideSoftInputFromWindow(n.field.getWindowToken(), 0);
                }
            }
        });
        field.setOnFocusChangeListener((v, focused) -> {
            if (focused) focusedInput = n;
            else if (focusedInput == n) focusedInput = null;
            updateAccessory();
            report(n, focused ? Protocol.EVENT_FOCUS : Protocol.EVENT_BLUR);
        });
        return field;
    }

    /**
     * Gives scroll node n a scroll view of the orientation, or one that
     * zooms, moving its content into it.
     */
    private void scroller(Node n, boolean horizontal, boolean zooms) {
        if (n.scroller != null && (n.scroller instanceof Scrolling.ZoomScroll) == zooms
                && (zooms || (n.scroller instanceof HorizontalScrollView) == horizontal)) return;
        Scrolling.Host host = scrollHost(n);
        ViewGroup frame = (ViewGroup) n.view;
        FrameLayout scroller = zooms ? new Scrolling.ZoomScroll(getContext(), host, density, n.children)
                : horizontal ? new Scrolling.HScroll(getContext(), host, density, n.children, frame)
                : new Scrolling.VScroll(getContext(), host, density, n.children, frame);
        // A scroll view inside another one scrolls first, then hands the
        // rest to its parent (else the parent takes every drag along its axis).
        scroller.setNestedScrollingEnabled(true);
        if (!zooms) {
            Scrolling.Scroller s = (Scrolling.Scroller) scroller;
            scroller.setOnScrollChangeListener((v, x, y, oldX, oldY) -> host.scrolled(s.offsetX(), s.offsetY(), 1));
        } else {
            n.children.setScaleX(1);
            n.children.setScaleY(1);
        }
        n.children.setTranslationX(0);
        n.children.setTranslationY(0);
        if (n.scroller != null) {
            n.scroller.removeView(n.children);
            ((ViewGroup) n.view).removeView(n.scroller);
        }
        scroller.addView(n.children);
        ((ViewGroup) n.view).addView(scroller, new LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT));
        n.scroller = scroller;
    }

    /**
     * Applies an ItemViewStyle. With a transition, the properties it lists
     * move from what the view shows to the new look on a ValueAnimator,
     * which Android's animator scale, its reduce-motion setting, also
     * scales; the rest change at once, as on a view's first style.
     */
    private void style(Node n, DataInputStream in) throws IOException {
        View view = n.view;
        Look look = new Look();
        look.background = Text.argb(in.readInt());
        look.radius = in.readFloat() * density;
        look.border = in.readFloat() * density;
        look.borderColor = Text.argb(in.readInt());
        for (int i = 0; i < 4; i++) look.borders[i] = in.readFloat() * density;
        for (int i = 0; i < 4; i++) look.borderColors[i] = Text.argb(in.readInt());
        look.sides = look.borders[0] > 0 || look.borders[1] > 0 || look.borders[2] > 0 || look.borders[3] > 0;
        look.opacity = in.readFloat();
        look.shadowColor = Text.argb(in.readInt());
        look.shadowRadius = in.readFloat() * density;
        int tint = in.readInt();
        look.translateX = in.readFloat() * density;
        look.translateY = in.readFloat() * density;
        look.rawScale = in.readFloat();
        look.rawScaleX = in.readFloat();
        look.rawScaleY = in.readFloat();
        look.combineScales();
        look.rotation = in.readFloat();
        long duration = in.readInt() & 0xffffffffL, delay = in.readInt() & 0xffffffffL;
        float x1 = in.readFloat(), y1 = in.readFloat(), x2 = in.readFloat(), y2 = in.readFloat();
        int properties = in.readUnsignedByte();
        int flags = in.readUnsignedByte();
        look.continuous = Protocol.flag(flags, 2);
        look.backfaceHidden = Protocol.flag(flags, 3);
        readDecor(look, in);
        Keyframes keyframes = Keyframes.read(in);

        if (n.image != null) n.image.setTint(Text.argb(tint));
        // The outline is the background's, corners included.
        view.setClipToOutline(Protocol.flag(flags, 0));
        view.setVisibility(Protocol.flag(flags, 1) ? GONE : VISIBLE);
        if (n.background == null) {
            n.background = new Box();
            view.setBackground(n.background);
        }
        if (n.animator != null) n.animator.cancel(); // n.shown is where it stopped
        runKeyframes(n, keyframes);
        if (duration == 0 || n.shown == null) {
            n.shown = look;
            paint(n);
            return;
        }
        Look from = n.shown;
        n.shown = Look.mix(from, look, 0, properties);
        paint(n);
        ValueAnimator animator = ValueAnimator.ofFloat(0, 1);
        animator.setDuration(duration);
        animator.setStartDelay(delay);
        boolean linear = x1 == 0 && y1 == 0 && x2 == 0 && y2 == 0;
        animator.setInterpolator(linear ? new LinearInterpolator() : new PathInterpolator(x1, y1, x2, y2));
        animator.addUpdateListener(a -> {
            n.shown = Look.mix(from, look, (float) a.getAnimatedValue(), properties);
            paint(n);
        });
        n.animator = animator;
        animator.start();
    }

    /** Reads what android.go's writeDecor writes into look, in pixels. */
    private void readDecor(Look look, DataInputStream in) throws IOException {
        float[] corners = new float[4];
        boolean any = false;
        for (int i = 0; i < 4; i++) {
            corners[i] = in.readFloat() * density;
            any |= corners[i] > 0;
        }
        look.corners = any ? corners : null;
        look.borderStyle = in.readUnsignedByte();
        Look.Outline o = new Look.Outline();
        o.width = in.readFloat() * density;
        o.offset = in.readFloat() * density;
        o.color = Text.argb(in.readInt());
        o.style = in.readUnsignedByte();
        look.outline = o.width > 0 ? o : null;
        look.rotationX = in.readFloat();
        look.rotationY = in.readFloat();
        look.cameraDistance = in.readFloat() * density;
        look.originX = in.readFloat() * density;
        look.originY = in.readFloat() * density;
        look.childPerspective = in.readFloat() * density;
        look.blend = in.readUnsignedByte();
        Look.Shadow[] shadows = new Look.Shadow[in.readUnsignedByte()];
        for (int i = 0; i < shadows.length; i++) {
            Look.Shadow sh = new Look.Shadow();
            sh.dx = in.readFloat() * density;
            sh.dy = in.readFloat() * density;
            sh.blur = in.readFloat() * density;
            sh.spread = in.readFloat() * density;
            sh.color = Text.argb(in.readInt());
            sh.inset = Protocol.flag(in.readUnsignedByte(), 0);
            shadows[i] = sh;
        }
        look.shadows = shadows;
        Box.Gradient[] gradients = new Box.Gradient[in.readUnsignedByte()];
        for (int i = 0; i < gradients.length; i++) {
            Box.Gradient g = new Box.Gradient();
            g.kind = in.readUnsignedByte();
            g.x0 = in.readFloat();
            g.y0 = in.readFloat();
            g.x1 = in.readFloat();
            g.y1 = in.readFloat();
            g.cx = in.readFloat();
            g.cy = in.readFloat();
            g.rx = in.readFloat();
            g.ry = in.readFloat();
            g.angle = in.readFloat();
            int n = in.readInt();
            g.offsets = new float[n];
            g.colors = new int[n];
            for (int k = 0; k < n; k++) {
                g.offsets[k] = in.readFloat();
                g.colors[k] = Text.argb(in.readInt());
            }
            gradients[i] = g;
        }
        look.gradients = gradients;
        if (Protocol.flag(in.readUnsignedByte(), 0)) {
            look.colorMatrix = new float[20];
            for (int i = 0; i < 20; i++) look.colorMatrix[i] = in.readFloat();
        }
        look.blur = in.readFloat() * density;
        if (in.readUnsignedByte() != 0) {
            look.dropShadow = new Look.Shadow();
            look.dropShadow.dx = in.readFloat() * density;
            look.dropShadow.dy = in.readFloat() * density;
            look.dropShadow.blur = in.readFloat() * density;
            look.dropShadow.color = Text.argb(in.readInt());
        }
    }

    /** Runs keyframes over n's look, or stops what ran when they change. */
    private void runKeyframes(Node n, Keyframes keyframes) {
        if (keyframes.equals(n.keyframes)) return;
        if (n.keyframer != null) n.keyframer.cancel();
        n.keyframer = null;
        n.keyframes = keyframes.steps.length > 0 ? keyframes : null;
        if (n.keyframes == null) return;
        ValueAnimator a = ValueAnimator.ofFloat(0, 1);
        a.setDuration(keyframes.duration);
        a.setStartDelay(keyframes.delay);
        a.setInterpolator(new LinearInterpolator());
        a.setRepeatCount(keyframes.iterations > 0 ? keyframes.iterations - 1 : ValueAnimator.INFINITE);
        a.addUpdateListener(v -> paint(n));
        n.keyframer = a;
        a.start();
    }

    private void input(Node n, DataInputStream in) throws IOException {
        InputField field = n.field;
        Text.Style style = new Text.Style(in, density);
        String placeholder = Protocol.readString(in);
        int keyboard = in.readUnsignedByte();
        int flags = in.readUnsignedByte();
        boolean masked = Protocol.flag(flags, 0), readonly = Protocol.flag(flags, 1);
        boolean disabled = Protocol.flag(flags, 2);
        LayoutParams content = frame(in);
        n.accessory = in.readInt();
        field.configure(in, keyboard, masked, readonly, disabled, density);
        field.setHint(placeholder);
        field.setTypeface(style.typeface());
        field.setTextSize(TypedValue.COMPLEX_UNIT_PX, style.size);
        field.setTextColor(style.color);
        field.setLetterSpacing(style.letterSpacingEm());
        field.setFontFeatureSettings(style.features);
        field.setPaintFlags(style.paintFlags(field.getPaintFlags()));
        field.setGravity(style.gravity() | (field.multiline() ? Gravity.TOP : Gravity.CENTER_VERTICAL));
        field.setLayoutParams(content);
    }

    static int inputType(int keyboard, boolean masked) {
        boolean decimal = keyboard == Protocol.KEYBOARD_DECIMAL;
        boolean numeric = decimal || keyboard == Protocol.KEYBOARD_NUMBER || keyboard == Protocol.KEYBOARD_PHONE;
        if (masked) {
            return numeric
                    ? InputType.TYPE_CLASS_NUMBER | InputType.TYPE_NUMBER_VARIATION_PASSWORD
                            | (decimal ? InputType.TYPE_NUMBER_FLAG_DECIMAL : 0)
                    : InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD;
        }
        switch (keyboard) {
            case Protocol.KEYBOARD_EMAIL:
                return InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS;
            case Protocol.KEYBOARD_NUMBER:
                return InputType.TYPE_CLASS_NUMBER;
            case Protocol.KEYBOARD_DECIMAL:
                return InputType.TYPE_CLASS_NUMBER | InputType.TYPE_NUMBER_FLAG_DECIMAL;
            case Protocol.KEYBOARD_PHONE:
                return InputType.TYPE_CLASS_PHONE;
            case Protocol.KEYBOARD_URL:
                return InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI;
            default:
                return InputType.TYPE_CLASS_TEXT;
        }
    }

    /**
     * Listens for the events in mask. A view takes touches only while it
     * reports presses or follows gestures (Group), so other touches reach
     * the pressable view under it.
     */
    private void events(Node n, long mask) {
        n.events = mask;
        View view = n.view;
        boolean press = n.has(Protocol.EVENT_PRESS);
        boolean longPress = n.has(Protocol.EVENT_LONG_PRESS);
        boolean downUp = n.has(Protocol.EVENT_PRESS_DOWN) || n.has(Protocol.EVENT_PRESS_UP);
        boolean hover = n.has(Protocol.EVENT_HOVER_IN) || n.has(Protocol.EVENT_HOVER_OUT)
                || n.has(Protocol.EVENT_HOVER_MOVE);
        // A text view's press carries the byte offset of the character
        // under it, for its link spans; every press where its touch went
        // down.
        view.setOnClickListener(press ? v -> {
            // A press held as a long press of the node's own delay ends there.
            if (!n.longFired) pressed(n, Protocol.EVENT_PRESS, v instanceof TextNode ? ((TextNode) v).pressedIndex() : 0);
            pressDone(n);
        } : null);
        // With a delay of its own the node times its long presses; the
        // system's, which comes at its fixed delay, only consumes the press.
        view.setOnLongClickListener(longPress ? v -> {
            if (n.delayLongPress > 0 && n.touching) {
                n.swallowed = true;
                return true;
            }
            pressed(n, Protocol.EVENT_LONG_PRESS, 0);
            pressDone(n);
            return true;
        } : null);
        view.setOnTouchListener(press || longPress || downUp ? (v, e) -> {
            if (v instanceof Group && !((Group) v).ownTouch()) return false;
            int action = e.getActionMasked();
            if (action == MotionEvent.ACTION_DOWN) {
                pressedAt(n, e);
                n.touching = true;
                n.longFired = n.swallowed = false;
                n.downX = e.getX();
                n.downY = e.getY();
                if (n.delayLongPress > 0 && n.has(Protocol.EVENT_LONG_PRESS)) {
                    n.longTimer = () -> {
                        n.longTimer = null;
                        n.longFired = true;
                        pressed(n, Protocol.EVENT_LONG_PRESS, 0);
                    };
                    v.postDelayed(n.longTimer, n.delayLongPress);
                }
                pressed(n, Protocol.EVENT_PRESS_DOWN, 0);
            } else if (action == MotionEvent.ACTION_MOVE) {
                int slop = ViewConfiguration.get(getContext()).getScaledTouchSlop();
                if (Math.hypot(e.getX() - n.downX, e.getY() - n.downY) > slop) stopLongPress(n);
            } else if (action == MotionEvent.ACTION_UP || action == MotionEvent.ACTION_CANCEL) {
                n.touching = false;
                boolean timing = n.longTimer != null;
                stopLongPress(n);
                pressed(n, Protocol.EVENT_PRESS_UP, 0);
                // The system's long press took the click, but the node's own
                // had not come yet: the release is a press.
                if (action == MotionEvent.ACTION_UP && n.swallowed && timing && !n.longFired) {
                    pressed(n, Protocol.EVENT_PRESS, v instanceof TextNode ? ((TextNode) v).pressedIndex() : 0);
                    pressDone(n);
                }
            }
            return false;
        } : null);
        view.setOnHoverListener(hover ? (v, e) -> {
            int action = e.getActionMasked();
            if (action == MotionEvent.ACTION_HOVER_ENTER) {
                report(n, Protocol.EVENT_HOVER_IN);
            } else if (action == MotionEvent.ACTION_HOVER_MOVE) {
                report(n, Protocol.EVENT_HOVER_MOVE, e);
            } else if (action == MotionEvent.ACTION_HOVER_EXIT) {
                report(n, Protocol.EVENT_HOVER_OUT);
            }
            return false;
        } : null);
        // A secondary click is no touch; it comes as a generic motion.
        view.setOnGenericMotionListener(n.has(Protocol.EVENT_CONTEXT_MENU) ? (v, e) -> {
            if (e.getActionMasked() != MotionEvent.ACTION_BUTTON_PRESS
                    || e.getActionButton() != MotionEvent.BUTTON_SECONDARY) {
                return false;
            }
            report(n, Protocol.EVENT_CONTEXT_MENU, e);
            return true;
        } : null);
        view.setClickable(press || longPress || downUp);
        view.setLongClickable(longPress);
        outside.remove(n);
        if (n.has(Protocol.EVENT_PRESS_OUTSIDE)) outside.add(n);
    }

    private void props(Node n, DataInputStream in) throws IOException {
        n.role = in.readUnsignedByte();
        int flags = in.readUnsignedByte();
        n.disabled = Protocol.flag(flags, 0);
        n.selected = Protocol.flag(flags, 1);
        n.checked = Protocol.flag(flags, 2);
        n.mixed = Protocol.flag(flags, 3);
        n.label = Protocol.readString(in);
        n.testID = Protocol.readString(in);
        n.activeX = in.readFloat();
        n.activeY = in.readFloat();
        n.failX = in.readFloat();
        n.failY = in.readFloat();
        n.hint = Protocol.readString(in);
        int a11y = in.readUnsignedByte();
        n.expandable = Protocol.flag(a11y, 0);
        n.expanded = Protocol.flag(a11y, 1);
        n.hidden = Protocol.flag(a11y, 2);
        n.range = Protocol.flag(a11y, 3);
        n.accessibleMode = in.readUnsignedByte();
        n.liveRegion = in.readUnsignedByte();
        n.min = in.readFloat();
        n.max = in.readFloat();
        n.now = in.readFloat();
        n.valueText = Protocol.readString(in);
        int actions = in.readInt();
        n.actionNames = new String[actions];
        n.actionLabels = new String[actions];
        for (int i = 0; i < actions; i++) {
            n.actionNames[i] = Protocol.readString(in);
            n.actionLabels[i] = Protocol.readString(in);
        }
        n.pointerEvents = in.readUnsignedByte();
        n.slop = new float[] {in.readFloat(), in.readFloat(), in.readFloat(), in.readFloat()};
        n.delayLongPress = in.readInt();
        int ripple = in.readUnsignedByte();
        int rippleColor = ripple != 0 ? in.readInt() : 0;
        float rippleRadius = ripple != 0 ? in.readFloat() : 0;
        ripple(n, ripple, rippleColor, rippleRadius);
        n.selectionArea = in.readUnsignedByte() != 0;
        int selectable = in.readUnsignedByte(), selectionColor = in.readInt();
        if (n.view instanceof TextNode) ((TextNode) n.view).selectable(selectable, Text.argb(selectionColor));
        View view = n.accessible();
        // A text field's ItemInput enables it.
        if (n.field == null) view.setEnabled(!n.disabled);
        view.setSelected(n.selected);
        view.setTag(n.testID.isEmpty() ? null : n.testID);
        // A dialog announces its label as it appears, an alert its content.
        view.setAccessibilityPaneTitle(n.role == Protocol.ROLE_DIALOG && !n.label.isEmpty() ? n.label : null);
        int live = n.liveRegion == 2 || n.role == Protocol.ROLE_ALERT ? View.ACCESSIBILITY_LIVE_REGION_ASSERTIVE
                : n.liveRegion == 1 ? View.ACCESSIBILITY_LIVE_REGION_POLITE : View.ACCESSIBILITY_LIVE_REGION_NONE;
        view.setAccessibilityLiveRegion(live);
        if (n.hidden) {
            view.setImportantForAccessibility(View.IMPORTANT_FOR_ACCESSIBILITY_NO_HIDE_DESCENDANTS);
        } else if (n.accessibleMode == Protocol.ACCESSIBLE_NO) {
            view.setImportantForAccessibility(View.IMPORTANT_FOR_ACCESSIBILITY_NO);
        } else if (n.accessibleMode == Protocol.ACCESSIBLE_YES) {
            view.setImportantForAccessibility(View.IMPORTANT_FOR_ACCESSIBILITY_YES);
        } else {
            view.setImportantForAccessibility(View.IMPORTANT_FOR_ACCESSIBILITY_AUTO);
        }
        // A group is one stop for TalkBack, which reads its label.
        view.setScreenReaderFocusable(n.accessibleMode == Protocol.ACCESSIBLE_YES);
        describe(n);
    }

    /**
     * Shows n's ripple, flags 1 set, 2 borderless and 4 foreground, as
     * Android's own press handling of a clickable view drives it: behind
     * the content in a layer over the background, or over it.
     */
    private void ripple(Node n, int flags, int color, float radius) {
        if (flags == n.rippleFlags && color == n.rippleColor && radius == n.rippleRadius) return;
        n.rippleFlags = flags;
        n.rippleColor = color;
        n.rippleRadius = radius;
        if (n.background == null) {
            n.background = new Box();
            n.view.setBackground(n.background);
        }
        if (n.view.getForeground() instanceof RippleDrawable) n.view.setForeground(null);
        if (!(n.view.getBackground() instanceof Box)) n.view.setBackground(n.background);
        if ((flags & 1) == 0) return;
        Box box = n.background;
        Drawable mask = (flags & 2) != 0 ? null : new Drawable() {
            private final Path path = new Path();
            private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);

            @Override
            public void draw(Canvas canvas) {
                paint.setColor(0xffffffff);
                canvas.drawPath(box.shape(getBounds(), path), paint);
            }

            @Override
            public void setAlpha(int alpha) {}

            @Override
            public void setColorFilter(ColorFilter filter) {}

            @Override
            public int getOpacity() {
                return PixelFormat.TRANSLUCENT;
            }
        };
        RippleDrawable drawable = new RippleDrawable(
                ColorStateList.valueOf(color != 0 ? Text.argb(color) : themeHighlight()), null, mask);
        if (radius > 0) drawable.setRadius(px(radius));
        if ((flags & 4) != 0) {
            n.view.setForeground(drawable);
        } else {
            n.view.setBackground(new LayerDrawable(new Drawable[] {box, drawable}));
        }
    }

    /** The theme's ripple color. */
    private int themeHighlight() {
        TypedValue value = new TypedValue();
        if (getContext().getTheme().resolveAttribute(android.R.attr.colorControlHighlight, value, true)) {
            return value.resourceId != 0 ? getContext().getColor(value.resourceId) : value.data;
        }
        return 0x1f000000;
    }

    /** The standard action a TalkBack action is, if any. */
    private static String actionName(Node n, int action) {
        if (action >= CUSTOM_ACTION && action < CUSTOM_ACTION + n.actionNames.length) {
            return n.actionNames[action - CUSTOM_ACTION];
        }
        switch (action) {
            case AccessibilityNodeInfo.ACTION_CLICK: return "activate";
            case AccessibilityNodeInfo.ACTION_LONG_CLICK: return "longpress";
            case AccessibilityNodeInfo.ACTION_SCROLL_FORWARD: return "increment";
            case AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD: return "decrement";
            case AccessibilityNodeInfo.ACTION_EXPAND: return "expand";
            case AccessibilityNodeInfo.ACTION_COLLAPSE: return "collapse";
            default: return null;
        }
    }

    /** Ids of custom accessibility actions, outside the framework's. */
    private static final int CUSTOM_ACTION = 0x3f000000;

    /** Actions TalkBack performs through its own gestures, not its actions menu. */
    private static boolean isStandardAction(String name) {
        switch (name) {
            case "activate": case "increment": case "decrement": case "longpress":
            case "expand": case "collapse": case "escape": case "magicTap":
                return true;
            default:
                return false;
        }
    }

    /** Sets what assistive technology reads: the label, else a text node's text. */
    private static void describe(Node n) {
        String d = !n.label.isEmpty() ? n.label : n.text;
        n.accessible().setContentDescription(d.isEmpty() ? null : d);
    }

    private static void describe(Node n, AccessibilityNodeInfo info) {
        String className = null, role = null;
        boolean checkable = false;
        switch (n.role) {
            case Protocol.ROLE_BUTTON: className = "android.widget.Button"; break;
            case Protocol.ROLE_LINK: role = "link"; break;
            case Protocol.ROLE_HEADER: info.setHeading(true); break;
            case Protocol.ROLE_IMAGE: className = "android.widget.ImageView"; break;
            case Protocol.ROLE_CHECKBOX: className = "android.widget.CheckBox"; checkable = true; break;
            case Protocol.ROLE_SWITCH: className = "android.widget.Switch"; checkable = true; break;
            case Protocol.ROLE_RADIO: className = "android.widget.RadioButton"; checkable = true; break;
            case Protocol.ROLE_TAB: role = "tab"; break;
            case Protocol.ROLE_TEXT: className = "android.widget.TextView"; break;
            case Protocol.ROLE_TEXT_FIELD: className = "android.widget.EditText"; break;
            case Protocol.ROLE_RADIO_GROUP: className = "android.widget.RadioGroup"; break;
            case Protocol.ROLE_TAB_LIST: className = "android.widget.TabWidget"; break;
            case Protocol.ROLE_MENU: role = "menu"; break;
            case Protocol.ROLE_MENU_ITEM: role = "menu item"; break;
            case Protocol.ROLE_DIALOG: role = "dialog"; break;
            case Protocol.ROLE_ALERT: role = "alert"; break;
            case Protocol.ROLE_TABLE: role = "table"; break;
            case Protocol.ROLE_GRID: role = "grid"; break;
            case Protocol.ROLE_ROW: role = "row"; break;
            case Protocol.ROLE_COLUMN_HEADER: role = "column header"; info.setHeading(true); break;
            case Protocol.ROLE_CELL: role = "cell"; break;
            default: break;
        }
        if (className != null) info.setClassName(className);
        // The key AndroidX and TalkBack read a role description under.
        if (role != null) info.getExtras().putCharSequence("AccessibilityNodeInfo.roleDescription", role);
        if (checkable) {
            info.setCheckable(true);
            info.setChecked(n.checked && !n.mixed);
            // The checked state has no mixed value to set, so describe it.
            if (n.mixed && Build.VERSION.SDK_INT >= 30) info.setStateDescription("partially checked");
        }
        info.setSelected(n.selected);
        if (!n.testID.isEmpty()) info.setViewIdResourceName(n.testID);
        if (!n.hint.isEmpty()) info.setHintText(n.hint);
        StringBuilder state = new StringBuilder();
        if (n.expandable) {
            state.append(n.expanded ? "expanded" : "collapsed");
            if (n.offers(n.expanded ? "collapse" : "expand")) {
                info.addAction(n.expanded ? AccessibilityNodeInfo.AccessibilityAction.ACTION_COLLAPSE
                        : AccessibilityNodeInfo.AccessibilityAction.ACTION_EXPAND);
            }
        }
        if (!n.valueText.isEmpty()) {
            if (state.length() > 0) state.append(", ");
            state.append(n.valueText);
        } else if (n.range) {
            info.setRangeInfo(AccessibilityNodeInfo.RangeInfo.obtain(
                    AccessibilityNodeInfo.RangeInfo.RANGE_TYPE_FLOAT, n.min, n.max, n.now));
        }
        if (n.role == Protocol.ROLE_ADJUSTABLE) info.setClassName("android.widget.SeekBar");
        if (n.role == Protocol.ROLE_PROGRESS_BAR) info.setClassName("android.widget.ProgressBar");
        if (state.length() > 0 && Build.VERSION.SDK_INT >= 30) info.setStateDescription(state);
        if (n.offers("increment")) info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_FORWARD);
        if (n.offers("decrement")) info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_BACKWARD);
        if (n.offers("activate")) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_CLICK);
            info.setClickable(true);
        }
        if (n.offers("longpress")) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_LONG_CLICK);
            info.setLongClickable(true);
        }
        for (int i = 0; i < n.actionNames.length; i++) {
            if (isStandardAction(n.actionNames[i])) continue;
            String label = n.actionLabels[i].isEmpty() ? n.actionNames[i] : n.actionLabels[i];
            info.addAction(new AccessibilityNodeInfo.AccessibilityAction(CUSTOM_ACTION + i, label));
        }
    }

    /**
     * A view node's container, whose children may draw outside it, as
     * without Clip. While its node follows gestures it takes the touches
     * its children leave, so the gestures see them, and while one of them
     * is active it takes its children's too, cancelling their presses.
     */
    private final class Group extends FrameLayout {
        Node node;
        /** The child a touch outside every child went to by its hit slop or area, until it lifts. */
        private View routed;
        /** Whether the touch reached the group outside its bounds and slop: only children may take it. */
        boolean childrenOnly;

        Group(Context context) {
            super(context);
            setClipChildren(false);
        }

        /** Whether the group itself may take the touch it is handed, as core.Reach says. */
        boolean ownTouch() {
            int pe = node == null ? 0 : node.pointerEvents;
            return !childrenOnly && pe != Protocol.POINTER_EVENTS_NONE && pe != Protocol.POINTER_EVENTS_BOX_NONE;
        }

        @Override
        public boolean dispatchTouchEvent(MotionEvent e) {
            int action = e.getActionMasked();
            if (node != null && node.pointerEvents == Protocol.POINTER_EVENTS_NONE) return false;
            if (action == MotionEvent.ACTION_DOWN) routed = route(e);
            if (routed != null) {
                deliver(routed, e);
                if (action == MotionEvent.ACTION_UP || action == MotionEvent.ACTION_CANCEL) routed = null;
                return true;
            }
            if (node != null && node.followsGestures()) {
                if (action == MotionEvent.ACTION_DOWN) gestures.candidate(node);
                // An ancestor, such as a scroll view, took the touch.
                else if (action == MotionEvent.ACTION_CANCEL) gestures.lost(node, true);
            }
            boolean handled = super.dispatchTouchEvent(e);
            if (action == MotionEvent.ACTION_UP || action == MotionEvent.ACTION_CANCEL) childrenOnly = false;
            return handled;
        }

        /**
         * Hands a touch starting outside every child's bounds to the topmost
         * child whose hit slop or area holds it, as UIKit's and AppKit's
         * hit tests do in NSGo; null when a child's bounds hold it, or
         * none takes it.
         */
        private View route(MotionEvent e) {
            if (node != null && node.pointerEvents == Protocol.POINTER_EVENTS_BOX_ONLY) return null;
            for (int i = getChildCount() - 1; i >= 0; i--) {
                View child = getChildAt(i);
                if (child.getVisibility() != VISIBLE) continue;
                float x = e.getX() + getScrollX() - child.getLeft(), y = e.getY() + getScrollY() - child.getTop();
                if (x >= 0 && y >= 0 && x < child.getWidth() && y < child.getHeight()) return null;
                Node c = byView.get(child);
                if (c == null) continue;
                boolean slop = c.slop != null && x >= -c.slop[3] * density && y >= -c.slop[0] * density
                        && x < child.getWidth() + c.slop[1] * density && y < child.getHeight() + c.slop[2] * density;
                boolean area = c.area != null && x >= c.area[0] * density && y >= c.area[1] * density
                        && x < (c.area[0] + c.area[2]) * density && y < (c.area[1] + c.area[3]) * density;
                if (!slop && !area) continue;
                if (child instanceof Group) ((Group) child).childrenOnly = !slop;
                if (deliver(child, e)) return child;
            }
            return null;
        }

        /** Dispatches e to child, in its coordinates. */
        private boolean deliver(View child, MotionEvent e) {
            MotionEvent local = MotionEvent.obtain(e);
            local.offsetLocation(getScrollX() - child.getLeft(), getScrollY() - child.getTop());
            boolean taken = child.dispatchTouchEvent(local);
            local.recycle();
            return taken;
        }

        @Override
        public void requestDisallowInterceptTouchEvent(boolean disallow) {
            // A descendant, such as a scroll view, took the touch first.
            if (disallow && !gestures.claiming) touchesTaken();
            if (disallow && node != null && !gestures.claiming) gestures.lost(node, false);
            super.requestDisallowInterceptTouchEvent(disallow);
        }

        @Override
        public boolean onInterceptTouchEvent(MotionEvent e) {
            // A box-only group takes its children's touches.
            if (node != null && node.pointerEvents == Protocol.POINTER_EVENTS_BOX_ONLY) return true;
            return gestures.owns(node) || super.onInterceptTouchEvent(e);
        }

        @Override
        public boolean onTouchEvent(MotionEvent e) {
            if (!ownTouch() && !gestures.owns(node)) return false;
            if (gestures.owns(node)) {
                // The gesture took the press: end the view's own once.
                if (gestures.takePress(node)) {
                    MotionEvent cancel = MotionEvent.obtain(e);
                    cancel.setAction(MotionEvent.ACTION_CANCEL);
                    super.onTouchEvent(cancel);
                    cancel.recycle();
                }
                return true;
            }
            boolean handled = super.onTouchEvent(e);
            return handled || node != null && node.followsGestures();
        }
    }

    /**
     * Recognizes the pan, pinch and rotation of one touch from the root's
     * motion events, all pointers in its coordinates, and reports them.
     * The innermost view under the first finger that follows each kind is
     * its target. A pan activates as its view's PanActivation decides
     * (core.PanActivation.Decide), on the move before the views below see
     * it; until then a scroll view that takes the touch fails it. An
     * active gesture keeps the scroll views around it from taking the
     * touch and takes it from the views inside its own.
     */
    private final class Gestures implements ScaleGestureDetector.OnScaleGestureListener {
        /** The distance a pan without active offsets moves before it activates, in points. */
        private static final float PAN_SLOP = 10;
        /** The angle a rotation turns before it begins, in radians. */
        private static final float ROTATION_SLOP = (float) (Math.PI / 36);

        /** Set while a gesture disallows its ancestors' interception, which is no other claim. */
        boolean claiming;
        private boolean collecting;
        private final ArrayList<Node> candidates = new ArrayList<>();
        private final ScaleGestureDetector scaler;
        private VelocityTracker velocity;
        private boolean cancelling;

        // The pan: its node, its pointer, where that went down, its last
        // translation in points, its state (0 undecided, 1 active, -1
        // failed or over) and whether it took the view's own press.
        private Node pan;
        private int panPointer;
        private float downX, downY, panX, panY;
        private int panState;
        private boolean panTookPress;

        // The pinch: its node, whether it is in progress, its scale and velocity.
        private Node pinch;
        private boolean pinching, pinchTookPress;
        private float scale = 1, scaleVelocity;

        // The rotation: its node, its two pointers (-1: none), its state as
        // a pan's, its angle so far, the last angle between the pointers,
        // its velocity and the time of the last move.
        private Node rotation;
        private int first = -1, second = -1;
        private int rotationState;
        private boolean rotationTookPress;
        private float angle, lastAngle, angleVelocity;
        private long angleTime;

        Gestures(Context context) {
            scaler = new ScaleGestureDetector(context, this);
            scaler.setQuickScaleEnabled(false);
        }

        /** Forgets the last touch as a new one starts. */
        void reset() {
            candidates.clear();
            collecting = true;
            pan = pinch = rotation = null;
            panState = rotationState = 0;
            panTookPress = pinchTookPress = rotationTookPress = false;
            pinching = false;
            first = second = -1;
            if (velocity != null) {
                velocity.recycle();
                velocity = null;
            }
        }

        /** Notes a gesture view the new touch went down on, outermost first. */
        void candidate(Node n) {
            if (collecting) candidates.add(n);
        }

        /** Takes the targets of the touch that went down. */
        void down(MotionEvent e) {
            collecting = false;
            for (Node n : candidates) {
                if (n.has(Protocol.EVENT_PAN)) pan = n;
                if (n.has(Protocol.EVENT_PINCH)) pinch = n;
                if (n.has(Protocol.EVENT_ROTATION)) rotation = n;
            }
            candidates.clear();
            if (pan == null && pinch == null && rotation == null) return;
            panPointer = e.getPointerId(0);
            downX = e.getX(0);
            downY = e.getY(0);
            panX = panY = 0;
            velocity = VelocityTracker.obtain();
            velocity.addMovement(e);
            if (pinch != null) scaler.onTouchEvent(e);
        }

        /** Follows a touch after it went down, before the views below see it. */
        void track(MotionEvent e) {
            if (velocity == null) return; // no gesture views under it
            int action = e.getActionMasked();
            cancelling = action == MotionEvent.ACTION_CANCEL;
            velocity.addMovement(e);
            trackPan(e, action);
            trackRotation(e, action);
            if (pinch != null) scaler.onTouchEvent(e);
            if (action == MotionEvent.ACTION_UP || action == MotionEvent.ACTION_CANCEL) {
                velocity.recycle();
                velocity = null;
                pan = pinch = rotation = null;
            }
        }

        private void trackPan(MotionEvent e, int action) {
            if (pan == null || panState < 0) return;
            int i = e.findPointerIndex(panPointer);
            boolean lifted = action == MotionEvent.ACTION_UP
                    || action == MotionEvent.ACTION_POINTER_UP && e.getPointerId(e.getActionIndex()) == panPointer;
            if (i >= 0 && action != MotionEvent.ACTION_CANCEL) {
                panX = (e.getX(i) - downX) / density;
                panY = (e.getY(i) - downY) / density;
            }
            if (i < 0 || action == MotionEvent.ACTION_CANCEL || lifted) {
                if (panState > 0) panEvent(lifted ? Protocol.GESTURE_ENDED : Protocol.GESTURE_CANCELLED);
                panState = -1;
                return;
            }
            if (action != MotionEvent.ACTION_MOVE) return;
            if (panState > 0) {
                panEvent(Protocol.GESTURE_CHANGED);
                return;
            }
            int decision = decide(pan, panX, panY);
            if (decision < 0) {
                panState = -1;
            } else if (decision > 0) {
                panState = 1;
                claim(pan);
                panEvent(Protocol.GESTURE_BEGAN);
            }
        }

        private void panEvent(int phase) {
            float vx = 0, vy = 0;
            if (velocity != null) {
                velocity.computeCurrentVelocity(1000);
                vx = velocity.getXVelocity(panPointer) / density;
                vy = velocity.getYVelocity(panPointer) / density;
            }
            gesture(pan, Protocol.EVENT_PAN, phase, panX, panY, vx, vy, 0);
        }

        private void trackRotation(MotionEvent e, int action) {
            if (rotation == null || rotationState < 0) return;
            if (action == MotionEvent.ACTION_POINTER_DOWN && second < 0) {
                int index = e.getActionIndex();
                first = e.getPointerId(index == 0 ? 1 : 0);
                second = e.getPointerId(index);
                lastAngle = angleOf(e);
                angle = angleVelocity = 0;
                angleTime = e.getEventTime();
                return;
            }
            if (second < 0) return;
            int a = e.findPointerIndex(first), b = e.findPointerIndex(second);
            int up = action == MotionEvent.ACTION_POINTER_UP ? e.getPointerId(e.getActionIndex()) : -1;
            boolean lifted = action == MotionEvent.ACTION_UP || up == first || up == second;
            if (a < 0 || b < 0 || action == MotionEvent.ACTION_CANCEL || lifted) {
                if (rotationState > 0) {
                    rotationEvent(lifted ? Protocol.GESTURE_ENDED : Protocol.GESTURE_CANCELLED);
                    rotationState = 0; // another pair may rotate again
                    rotationTookPress = false;
                }
                first = second = -1;
                return;
            }
            if (action != MotionEvent.ACTION_MOVE) return;
            float now = angleOf(e);
            float delta = now - lastAngle;
            // The shorter way round, across the atan2 seam.
            if (delta > Math.PI) delta -= (float) (2 * Math.PI);
            else if (delta < -Math.PI) delta += (float) (2 * Math.PI);
            lastAngle = now;
            angle += delta;
            long time = e.getEventTime();
            if (time > angleTime) angleVelocity = delta * 1000f / (time - angleTime);
            angleTime = time;
            if (rotationState > 0) {
                rotationEvent(Protocol.GESTURE_CHANGED);
            } else if (Math.abs(angle) >= ROTATION_SLOP) {
                rotationState = 1;
                claim(rotation);
                rotationEvent(Protocol.GESTURE_BEGAN);
            }
        }

        /** The angle of the line from the first pointer to the second, clockwise on screen. */
        private float angleOf(MotionEvent e) {
            int a = e.findPointerIndex(first), b = e.findPointerIndex(second);
            return (float) Math.atan2(e.getY(b) - e.getY(a), e.getX(b) - e.getX(a));
        }

        private void rotationEvent(int phase) {
            gesture(rotation, Protocol.EVENT_ROTATION, phase, 0, 0, angleVelocity, 0, angle);
        }

        @Override
        public boolean onScaleBegin(ScaleGestureDetector d) {
            if (pinch == null) return false;
            pinching = true;
            scale = 1;
            scaleVelocity = 0;
            claim(pinch);
            gesture(pinch, Protocol.EVENT_PINCH, Protocol.GESTURE_BEGAN, 0, 0, 0, 0, scale);
            return true;
        }

        @Override
        public boolean onScale(ScaleGestureDetector d) {
            if (!pinching) return true;
            float next = scale * d.getScaleFactor();
            long dt = d.getTimeDelta();
            if (dt > 0) scaleVelocity = (next - scale) * 1000f / dt;
            scale = next;
            gesture(pinch, Protocol.EVENT_PINCH, Protocol.GESTURE_CHANGED, 0, 0, scaleVelocity, 0, scale);
            return true;
        }

        @Override
        public void onScaleEnd(ScaleGestureDetector d) {
            if (!pinching) return;
            pinching = false;
            pinchTookPress = false;
            int phase = cancelling ? Protocol.GESTURE_CANCELLED : Protocol.GESTURE_ENDED;
            gesture(pinch, Protocol.EVENT_PINCH, phase, 0, 0, scaleVelocity, 0, scale);
        }

        /**
         * Notes that something else took n's touch: its gestures that have
         * not begun fail, and with active, those in progress are cancelled.
         */
        void lost(Node n, boolean active) {
            if (pan == n && (panState == 0 || active && panState > 0)) {
                if (panState > 0) panEvent(Protocol.GESTURE_CANCELLED);
                panState = -1;
            }
            if (pinch == n && (!pinching || active)) {
                if (pinching) {
                    gesture(pinch, Protocol.EVENT_PINCH, Protocol.GESTURE_CANCELLED, 0, 0, scaleVelocity, 0, scale);
                }
                pinching = false;
                pinch = null;
            }
            if (rotation == n && (rotationState == 0 || active && rotationState > 0)) {
                if (rotationState > 0) rotationEvent(Protocol.GESTURE_CANCELLED);
                rotationState = -1;
            }
        }

        /** Whether an active gesture of n's holds the touch. */
        boolean owns(Node n) {
            return n != null && (pan == n && panState > 0 || pinch == n && pinching
                    || rotation == n && rotationState > 0);
        }

        /** Whether n's own press is still to cancel for the gesture holding the touch, once. */
        boolean takePress(Node n) {
            boolean took = panTookPress && pan == n || pinchTookPress && pinch == n
                    || rotationTookPress && rotation == n;
            if (took) return false;
            if (pan == n) panTookPress = true;
            if (pinch == n) pinchTookPress = true;
            if (rotation == n) rotationTookPress = true;
            return true;
        }

        /** Keeps the scroll views and gesture views around n from taking the touch. */
        private void claim(Node n) {
            ViewParent parent = n.view.getParent();
            if (parent == null) return;
            claiming = true;
            parent.requestDisallowInterceptTouchEvent(true);
            claiming = false;
        }

        /**
         * Reports a gesture of n's, after moving the values bound to it in
         * the frame it moves; a pan's start and end carry those values as
         * the event's size.
         */
        private void gesture(Node n, int kind, int phase, float x, float y, float vx, float vy, float value) {
            float[] values = animator.gesture(kind, n.id, phase, x, y, value);
            if (values != null) applyAnimated();
            float w = values != null && values.length == 2 ? values[0] : 0;
            float h = values != null && values.length == 2 ? values[1] : 0;
            if (n.has(kind)) event(kind, n.id, 0, x, y, "", w, h, 0, phase, vx, vy, value, null);
        }
    }

    /** Whether a drag of n translated by (dx, dy) points activates its pan (1), fails it (-1) or is undecided (0). */
    private static int decide(Node n, float dx, float dy) {
        if (past(dx, n.failX) || past(dy, n.failY)) return -1;
        if (n.activeX == 0 && n.activeY == 0) {
            return Math.abs(dx) >= Gestures.PAN_SLOP || Math.abs(dy) >= Gestures.PAN_SLOP ? 1 : 0;
        }
        return past(dx, n.activeX) || past(dy, n.activeY) ? 1 : 0;
    }

    /** What scroll node n's scroll view asks of the runtime. */
    private Scrolling.Host scrollHost(Node n) {
        return new Scrolling.Host() {
            @Override
            public void scrollEvent(int kind, float x, float y, float zoom) {
                if (kind == Protocol.EVENT_SCROLL_DRAG_END || kind == Protocol.EVENT_MOMENTUM_END) {
                    // At rest, unless a glide follows, which scrolls on.
                    animator.settle(n.id);
                    if (animator.running()) startTicking();
                }
                if (n.has(kind)) event(kind, n.id, n.applied, x, y, "", 0, 0, 0, 0, 0, 0, zoom, null);
            }

            @Override
            public void scrolled(float x, float y, float zoom) {
                // Values following the offset move in the same frame, and
                // so do sticky cells.
                boolean moved = animator.scroll(n.id, x, y);
                if (animator.pulls(n.id) && n.scroller instanceof Scrolling.Scroller
                        && animator.pull(n.id, ((Scrolling.Scroller) n.scroller).pulled())) {
                    moved = true;
                }
                if (moved) applyAnimated();
                if (n.sticky != null) pinSticky(n);
                scrollEvent(Protocol.EVENT_SCROLL, x, y, zoom);
            }

            @Override
            public boolean dismissKeyboard() {
                View focused = findFocus();
                if (!(focused instanceof EditText)) return false;
                ime.hideSoftInputFromWindow(focused.getWindowToken(), 0);
                focused.clearFocus();
                return true;
            }

            @Override
            public boolean onInput(float rawX, float rawY) {
                for (int i = 0; i < nodes.size(); i++) {
                    Node o = nodes.valueAt(i);
                    if (o.field != null && under(o.field, rawX, rawY)) return true;
                }
                return false;
            }

            @Override
            public boolean handlesTap(float rawX, float rawY) {
                for (int i = 0; i < nodes.size(); i++) {
                    Node o = nodes.valueAt(i);
                    if (o != n && (o.has(Protocol.EVENT_PRESS) || o.has(Protocol.EVENT_LONG_PRESS)
                            || o.has(Protocol.EVENT_PRESS_DOWN) || o.followsGestures()) && under(o.view, rawX, rawY)) {
                        return true;
                    }
                }
                return false;
            }
        };
    }

    /** Whether a shown view covers the raw screen point. */
    private static boolean under(View v, float rawX, float rawY) {
        if (!v.isShown()) return false;
        int[] at = new int[2];
        v.getLocationOnScreen(at);
        return rawX >= at[0] && rawY >= at[1] && rawX < at[0] + v.getWidth() * v.getScaleX()
                && rawY < at[1] + v.getHeight() * v.getScaleY();
    }

    private static boolean past(float v, float limit) {
        return limit != 0 && Math.abs(v) >= Math.abs(limit);
    }

    /**
     * A scroll view's content. Scroll views leave its length along their
     * axis unbounded, so it takes the size ItemScrollContent gave it.
     */
    private static final class Content extends FrameLayout {
        Content(Context context) {
            super(context);
            setClipChildren(false);
        }

        @Override
        protected void onMeasure(int widthSpec, int heightSpec) {
            ViewGroup.LayoutParams lp = getLayoutParams();
            super.onMeasure(MeasureSpec.makeMeasureSpec(Math.max(lp.width, 0), MeasureSpec.EXACTLY),
                    MeasureSpec.makeMeasureSpec(Math.max(lp.height, 0), MeasureSpec.EXACTLY));
        }
    }

    /** A modal's layer: it takes the touches its children leave, so none reach the views it covers. */
    private static final class Layer extends FrameLayout {
        Layer(Context context) {
            super(context);
            setClipChildren(false);
        }

        @Override
        public boolean onTouchEvent(MotionEvent e) {
            super.onTouchEvent(e);
            return true;
        }
    }

    /** A mounted node: its views and what batches told it. UI thread only. */
    private static final class Node {
        final int id;
        /** The view placed at the node's frame in its parent. */
        final View view;
        /** Where children go: the view, or a scroll view's content. */
        ViewGroup children;
        FrameLayout scroller;
        /** The id of the text layout a text node shows. */
        int laid;
        InputField field;
        ImageNode image;
        CanvasNode canvas;
        ShaderNode shader;
        WebNode web;
        VideoNode video;
        /** The view's background, the look it shows and the transition moving it. */
        Box background;
        Look shown;
        /** Whether the node is a selection area (driver.ViewProps.SelectionArea). */
        boolean selectionArea;
        /** The ripple's flags, color and radius, as ripple() takes them. */
        int rippleFlags, rippleColor;
        float rippleRadius;
        ValueAnimator animator;
        /** The keyframe animation running over the look, and what it runs. */
        Keyframes keyframes;
        ValueAnimator keyframer;
        /** The node the view went into, whose perspective its turns are seen in. */
        Node up;
        /** A modal's animation, -1 for other nodes, and whether it was presented. */
        int modal = -1;
        boolean presented;
        long events;
        int role;
        boolean disabled, selected, checked, mixed;
        String label = "", text = "", testID = "";
        /** Accessibility beyond the role, label and states, as core.ViewProps has it. */
        String hint = "", valueText = "";
        boolean expandable, expanded, hidden, range;
        int accessibleMode, liveRegion;
        float min, max, now;
        String[] actionNames = new String[0], actionLabels = new String[0];
        /** The pan's activation offsets in points, as in core.PanActivation; 0 is unset. */
        float activeX, activeY, failX, failY;
        /** Where its touch went down, in it and its root view in points, with the modifier bits and clicks. */
        float pressX, pressY, pressPageX, pressPageY;
        int pressMods, pressClicks;
        /** core.PointerEvents, the hit slop (top, right, bottom, left) and areas in points, the long press delay in ms. */
        int pointerEvents;
        float[] slop, area;
        int delayLongPress;
        /** A touch is down; the long press it is timed for fired; where it went down, in pixels. */
        boolean touching, longFired, swallowed;
        float downX, downY;
        Runnable longTimer;
        /** The frame shown, in points (x, y, width, height), or null before the first. */
        float[] frame;
        /**
         * A scroll node's ItemScrollTo and ItemScrollBy applied, which its
         * reports carry, and its sticky cells (id, start, end); a sticky
         * cell's shift from its frame in points, across for a horizontal list.
         */
        long applied;
        float[] sticky;
        // stickyBound says a sticky cell's inset follows an animated value.
        boolean stickyBound;
        float shift;
        boolean shiftAcross;
        /** A layout animation's move: from and to, its phase and when it began. */
        float[] from, to;
        LayoutTransition motion;
        long motionStart;
        /** A layout animation's appearance, and when it began. */
        LayoutTransition appear;
        long appearStart;
        /** The animated values bound to it, drawn over its style, or null. */
        Animator.Values values;
        /** An input's InputAccessoryView, 0 for none. */
        int accessory;
        /** A native view's component view, and whether one was asked for. */
        Embedded embedded;
        boolean embedTried;
        /** A native view's animated props. */
        Embedded.Bound bound;
        /** A screen's system header, and the holder of a header view in one. */
        Header header;
        Header.Holder holder;

        Node(int id, View view) {
            this.id = id;
            this.view = view;
        }

        boolean has(int kind) {
            return (events & (1L << kind)) != 0;
        }

        /** Whether the node offers the accessibility action name. */
        boolean offers(String name) {
            for (String a : actionNames) {
                if (a.equals(name)) return true;
            }
            return false;
        }

        /** Whether the node follows a pan, a pinch or a rotation. */
        boolean followsGestures() {
            return has(Protocol.EVENT_PAN) || has(Protocol.EVENT_PINCH) || has(Protocol.EVENT_ROTATION);
        }

        /** The view assistive technology sees: an input's text field, else the view. */
        View accessible() {
            return field != null ? field : view;
        }
    }
}
