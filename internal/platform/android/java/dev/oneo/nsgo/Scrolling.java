package dev.oneo.nsgo;

import android.content.Context;
import android.os.SystemClock;
import android.view.Gravity;
import android.view.MotionEvent;
import android.view.ScaleGestureDetector;
import android.view.View;
import android.view.ViewConfiguration;
import android.view.ViewGroup;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.widget.OverScroller;
import android.widget.ProgressBar;
import android.widget.ScrollView;

import java.io.DataInputStream;
import java.io.IOException;

/**
 * Scroll views, as NSH's ScrollBox scrolls: Android's ScrollView and
 * HorizontalScrollView, whose flings glide by core.ScrollBehavior's
 * deceleration or end at the snap point snapTarget (a port of
 * core.ScrollBehavior.SnapTarget) picks; drag and momentum events; the
 * content inset as padding; keyboard dismissal on drags and taps; pull to
 * refresh with a spinner, or the app's own indicator, above the content;
 * and ZoomScroll, which a
 * zoomable view switches to: it scales the content around its top left and
 * scrolls both ways itself.
 */
final class Scrolling {
    private Scrolling() {}

    /** How far past the top a pull refreshes, and the spinner's room, in points. */
    static final float REFRESH_PULL = 64, REFRESH_SPACE = 56;
    /** Releases slower than this (points per millisecond) settle at the nearest snap point. */
    private static final float SLOW = 0.3f;
    /** A glide stops below this speed, in points per millisecond. */
    private static final float STOP = 0.01f;
    /** Duration of animated scrolls, in milliseconds. */
    private static final long ANIMATION = 300;

    /** A scroll view's configuration (core.ScrollConfig), lengths in points. */
    static final class Config {
        boolean horizontal, hideIndicator, paging, oneStep, noScroll, noBounces, refreshable, refreshing;
        boolean noBouncesZoom, noPinchZoom, centerContent;
        float interval, rate = 0.998f;
        int alignment, dismiss, taps;
        float[] offsets = new float[0];
        float insetTop, insetRight, insetBottom, insetLeft;
        float minZoom = 1, maxZoom = 1, zoom = 1;
        /** The height of the app's own refresh indicator; 0 shows the spinner. */
        float refreshIndicator;

        boolean snaps() {
            return paging || interval > 0 || offsets.length > 0;
        }

        boolean zooms() {
            return maxZoom > minZoom;
        }

        /** How far past the top a released pull refreshes, in points. */
        float refreshPull() {
            return refreshIndicator > 0 ? refreshIndicator : REFRESH_PULL;
        }

        /** The room kept above the content while it refreshes, in points. */
        float refreshSpace() {
            return refreshIndicator > 0 ? refreshIndicator : REFRESH_SPACE;
        }

        /** Reads ItemScrollConfig as internal/platform/android/android.go writes it. */
        static Config read(DataInputStream in) throws IOException {
            Config c = new Config();
            int flags = in.readUnsignedByte();
            c.horizontal = Protocol.flag(flags, 0);
            c.hideIndicator = Protocol.flag(flags, 1);
            flags = in.readUnsignedByte();
            c.paging = Protocol.flag(flags, 0);
            c.oneStep = Protocol.flag(flags, 1);
            c.noScroll = Protocol.flag(flags, 2);
            c.noBounces = Protocol.flag(flags, 3);
            c.refreshable = Protocol.flag(flags, 4);
            c.refreshing = Protocol.flag(flags, 5);
            c.noBouncesZoom = Protocol.flag(flags, 6);
            c.noPinchZoom = Protocol.flag(flags, 7);
            c.centerContent = Protocol.flag(in.readUnsignedByte(), 0);
            c.interval = in.readFloat();
            c.alignment = in.readUnsignedByte();
            c.offsets = new float[in.readInt()];
            for (int i = 0; i < c.offsets.length; i++) c.offsets[i] = in.readFloat();
            java.util.Arrays.sort(c.offsets);
            c.rate = in.readFloat();
            c.insetTop = in.readFloat();
            c.insetRight = in.readFloat();
            c.insetBottom = in.readFloat();
            c.insetLeft = in.readFloat();
            c.dismiss = in.readUnsignedByte();
            c.taps = in.readUnsignedByte();
            c.minZoom = in.readFloat();
            c.maxZoom = in.readFloat();
            c.zoom = in.readFloat();
            c.refreshIndicator = in.readFloat();
            return c;
        }
    }

    /** The snap points of a view snapping along a viewport and content of those lengths. */
    static float[] snapPoints(Config c, float viewport, float content) {
        float limit = Math.max(content - viewport, 0);
        if (c.interval > 0) {
            float shift = c.alignment == 1 ? (viewport - c.interval) / 2 : c.alignment == 2 ? viewport - c.interval : 0;
            int n = Math.max((int) Math.ceil((limit + shift) / c.interval), 0);
            float[] out = new float[n + 1];
            for (int i = 0; i <= n; i++) out[i] = Math.min(Math.max(i * c.interval - shift, 0), limit);
            return out;
        }
        if (c.offsets.length > 0) {
            float[] out = new float[c.offsets.length];
            for (int i = 0; i < out.length; i++) out[i] = Math.min(Math.max(c.offsets[i], 0), limit);
            return out;
        }
        if (c.paging) {
            float page = Math.max(viewport, 1);
            int n = (int) Math.ceil(limit / page);
            float[] out = new float[n + 1];
            for (int i = 0; i <= n; i++) out[i] = Math.min(i * page, limit);
            return out;
        }
        return new float[0];
    }

    /**
     * core.ScrollBehavior.SnapTarget: the snap point a fling released at
     * offset, which would glide to projected at velocity (points per
     * millisecond), ends at; NaN when the view does not snap.
     */
    static float snapTarget(Config c, float offset, float projected, float velocity, float viewport, float content) {
        float[] points = snapPoints(c, viewport, content);
        if (points.length == 0) return Float.NaN;
        float limit = Math.max(content - viewport, 0);
        boolean step = c.oneStep || c.paging && c.interval <= 0 && c.offsets.length == 0;
        float from = step ? offset : projected;
        if (velocity > SLOW) {
            for (float p : points) if (step ? p > from + 0.5f : p >= from - 0.5f) return p;
            return limit;
        }
        if (velocity < -SLOW) {
            for (int i = points.length - 1; i >= 0; i--) {
                float p = points[i];
                if (step ? p < from - 0.5f : p <= from + 0.5f) return p;
            }
            return 0;
        }
        float best = points[0];
        for (float p : points) if (Math.abs(p - from) < Math.abs(best - from)) best = p;
        return best;
    }

    /** Where a glide at velocity from offset comes to rest. */
    static float projected(float offset, float velocity, float rate) {
        return offset - velocity / (float) Math.log(rate);
    }

    /** How long (ms) an eased move of distance starting at velocity takes, as NSH's snap_duration. */
    static long snapDuration(float distance, float velocity) {
        if (Math.abs(distance) < 0.5f) return 0;
        if (velocity * distance > 0) return (long) Math.min(Math.max(3 * Math.abs(distance) / Math.abs(velocity), 180), 600);
        return ANIMATION;
    }

    /** The rubber band of iOS: how far content pulled over points past an edge of a dim-point viewport moves. */
    static float rubber(float over, float dim) {
        dim = Math.max(dim, 1);
        return (1 - 1 / (over * 0.55f / dim + 1)) * dim;
    }

    /** What a scroll view asks of its runtime. */
    interface Host {
        /** Reports kind with the offset (points, UIKit's contentOffset) and zoom, if the node listens. */
        void scrollEvent(int kind, float x, float y, float zoom);

        /** The offset moved: values bound to it follow, and EventScroll goes out. */
        void scrolled(float x, float y, float zoom);

        /** Takes keyboard focus away, hiding the keyboard; false when nothing had it. */
        boolean dismissKeyboard();

        /** Whether a tap at the raw screen point lands on a text field. */
        boolean onInput(float rawX, float rawY);

        /** Whether a tap at the raw screen point lands on a view that takes presses. */
        boolean handlesTap(float rawX, float rawY);
    }

    /** A scroll view of the runtime: VScroll, HScroll or ZoomScroll. */
    interface Scroller {
        void configure(Config c);

        /** Scrolls to the offset in points (UIKit's contentOffset), clamped. */
        void scrollToPoints(float x, float y, boolean animated);

        /**
         * Moves the offset by points, carrying a glide or snap in flight
         * along: the core keeps the visible content in place as rows above
         * it change size.
         */
        void scrollByPoints(float dx, float dy);

        float offsetX();

        float offsetY();

        /** How far the view is pulled past its top, its refresh room included, in points. */
        float pulled();
    }

    /**
     * What VScroll and HScroll share: the drag and its events, the glide
     * or snap after a fling, the pull to refresh and the keyboard.
     */
    static final class Driver {
        final FrameLayout view;
        final boolean horizontal;
        final Host host;
        final float density;
        final View content;
        final ViewGroup frame;
        Config config = new Config();
        private final int slop;
        // The touch: where it went down, and whether it dragged.
        private float downX, downY, lastY;
        private boolean touching, dragging, flung;
        // The glide or eased move running, with its momentum events.
        private Runnable motion;
        private boolean momentum;
        // What ItemScrollBy moved the motion running by, in pixels.
        private int carry;
        // The pull to refresh, in pixels, and its spinner.
        private float pull, pullTarget;
        private boolean pulling, refreshing;
        private ProgressBar spinner;

        Driver(FrameLayout view, boolean horizontal, Host host, float density, View content, ViewGroup frame) {
            this.view = view;
            this.horizontal = horizontal;
            this.host = host;
            this.density = density;
            this.content = content;
            this.frame = frame;
            slop = ViewConfiguration.get(view.getContext()).getScaledTouchSlop();
        }

        int pos() {
            return horizontal ? view.getScrollX() : view.getScrollY();
        }

        /** The offset's range in pixels: the content and its padding (the inset) less the viewport. */
        int range() {
            if (horizontal) return Math.max(content.getWidth() + view.getPaddingLeft() + view.getPaddingRight() - view.getWidth(), 0);
            return Math.max(content.getHeight() + view.getPaddingTop() + view.getPaddingBottom() - view.getHeight(), 0);
        }

        float offsetX() {
            return view.getScrollX() / density - config.insetLeft;
        }

        float offsetY() {
            return (view.getScrollY() - pull) / density - config.insetTop;
        }

        float pulled() {
            return horizontal ? 0 : Math.max(pull - view.getScrollY(), 0) / density;
        }

        /** What the keyboard covers of the view, added to its bottom inset, in pixels. */
        private int keyboardPad;

        void configure(Config c) {
            config = c;
            pad();
            view.setClipToPadding(false);
            view.setOverScrollMode(c.noBounces ? View.OVER_SCROLL_NEVER : View.OVER_SCROLL_IF_CONTENT_SCROLLS);
            view.setVerticalScrollBarEnabled(!c.hideIndicator && !horizontal);
            view.setHorizontalScrollBarEnabled(!c.hideIndicator && horizontal);
            setRefreshing(c.refreshing && !horizontal);
        }

        private void pad() {
            Config c = config;
            view.setPadding(Math.round(c.insetLeft * density), Math.round(c.insetTop * density),
                    Math.round(c.insetRight * density), Math.round(c.insetBottom * density) + keyboardPad);
        }

        /**
         * Keeps field clear of the keyboard whose top is at keyboardTop in
         * the window: what it covers of the view adds to the bottom inset,
         * and the view scrolls the field above it. A null field gives the
         * inset back.
         */
        void keepClear(View field, int keyboardTop) {
            int[] at = new int[2];
            int covered = 0;
            if (field != null) {
                view.getLocationInWindow(at);
                covered = Math.max(0, at[1] + view.getHeight() - keyboardTop);
            }
            if (covered != keyboardPad) {
                keyboardPad = covered;
                pad();
            }
            if (field == null || horizontal) return;
            field.getLocationInWindow(at);
            int margin = Math.round(8 * density);
            int over = at[1] + field.getHeight() + margin - keyboardTop;
            if (over > 0) {
                stop();
                // The new inset takes a layout pass to widen the range.
                view.post(() -> ease(Math.min(pos() + over, range()), ANIMATION, false));
            }
        }

        void scrollTo(float x, float y, boolean animated) {
            stop();
            int target = Math.round((horizontal ? x + config.insetLeft : y + config.insetTop) * density);
            target = Math.min(Math.max(target, 0), range());
            if (animated) ease(target, ANIMATION, false);
            else move(target);
        }

        private void move(int px) {
            if (horizontal) view.scrollTo(px, 0);
            else view.scrollTo(0, px);
        }

        /** ItemScrollBy: a motion in flight carries the shift along; a drag goes on from where it is. */
        void scrollBy(float dx, float dy) {
            int d = Math.round((horizontal ? dx : dy) * density);
            if (d == 0) return;
            if (motion != null) carry += d;
            move(pos() + d);
        }

        private void event(int kind) {
            host.scrollEvent(kind, offsetX(), offsetY(), 1);
        }

        /** Stops a glide or an eased move, which ends its momentum. */
        void stop() {
            if (motion != null) {
                view.removeCallbacks(motion);
                motion = null;
            }
            if (momentum) {
                momentum = false;
                event(Protocol.EVENT_MOMENTUM_END);
            }
        }

        /** Whether the view takes the touch: a vertical refreshable view pulled down at its top. */
        boolean intercept(MotionEvent e) {
            if (e.getActionMasked() == MotionEvent.ACTION_DOWN) {
                downX = e.getX();
                downY = lastY = e.getY();
                return false;
            }
            return e.getActionMasked() == MotionEvent.ACTION_MOVE && pullable() && e.getY() - downY > slop
                    && Math.abs(e.getY() - downY) > Math.abs(e.getX() - downX);
        }

        private boolean pullable() {
            return !horizontal && config.refreshable && !refreshing && !config.noScroll && view.getScrollY() <= 0;
        }

        /**
         * Follows the view's touches before ScrollView sees them; true
         * when the pull to refresh takes this one.
         */
        boolean before(MotionEvent e) {
            switch (e.getActionMasked()) {
                case MotionEvent.ACTION_DOWN:
                    stop();
                    touching = true;
                    dragging = flung = false;
                    downX = e.getX();
                    downY = lastY = e.getY();
                    return false;
                case MotionEvent.ACTION_MOVE: {
                    float dy = e.getY() - lastY;
                    lastY = e.getY();
                    if (pulling || pullable() && dy > 0 && e.getY() - downY > slop) {
                        if (!pulling) {
                            pulling = true;
                            dragStart();
                        }
                        pull = Math.max(rubber(Math.max(e.getY() - downY - slop, 0) / density, view.getHeight() / density) * density, 0);
                        content.setTranslationY(pull);
                        showSpinner(pull > 0);
                        host.scrolled(offsetX(), offsetY(), 1);
                        if (pull <= 0 && dy < 0) pulling = false; // back to scrolling
                        return pulling;
                    }
                    return false;
                }
                case MotionEvent.ACTION_UP:
                case MotionEvent.ACTION_CANCEL:
                    if (!pulling) return false;
                    pulling = touching = false;
                    dragging = false;
                    event(Protocol.EVENT_SCROLL_DRAG_END);
                    if (pull >= config.refreshPull() * density && e.getActionMasked() == MotionEvent.ACTION_UP) {
                        // Shown right away; the next config says whether the app keeps it.
                        event(Protocol.EVENT_REFRESH);
                        refreshing = false;
                        setRefreshing(true);
                    } else {
                        settlePull(0);
                    }
                    return true;
            }
            return false;
        }

        /** Follows the view's touches after ScrollView handled them. */
        void after(MotionEvent e) {
            switch (e.getActionMasked()) {
                case MotionEvent.ACTION_MOVE:
                    if (!dragging && touching && Math.hypot(e.getX() - downX, e.getY() - downY) > slop) dragStart();
                    break;
                case MotionEvent.ACTION_UP:
                case MotionEvent.ACTION_CANCEL:
                    boolean tap = touching && !dragging && e.getActionMasked() == MotionEvent.ACTION_UP;
                    touching = false;
                    if (dragging) {
                        dragging = false;
                        if (!flung) {
                            event(Protocol.EVENT_SCROLL_DRAG_END);
                            settle(0);
                            if (!momentum) event(Protocol.EVENT_SCROLL_END);
                        }
                    }
                    if (tap) tapped(e);
                    break;
            }
        }

        private void dragStart() {
            dragging = true;
            event(Protocol.EVENT_SCROLL_DRAG_START);
            if (config.dismiss != 0) host.dismissKeyboard();
        }

        /** A tap while the keyboard shows dismisses it, as KeyboardShouldPersistTaps says. */
        private void tapped(MotionEvent e) {
            float x = e.getRawX(), y = e.getRawY();
            if (config.taps == 1 || host.onInput(x, y)) return; // always
            if (config.taps == 2 && host.handlesTap(x, y)) return; // handled
            host.dismissKeyboard();
        }

        /** ScrollView flings at velocity, in pixels per second as the offset grows. */
        void fling(int velocity) {
            flung = true;
            if (dragging) {
                dragging = false;
                event(Protocol.EVENT_SCROLL_DRAG_END);
            }
            settle(velocity / 1000f / density);
            if (!momentum) event(Protocol.EVENT_SCROLL_END);
        }

        /** The user let go at v points per millisecond: a snap, a glide or nothing. */
        void settle(float v) {
            float pos = pos() / density;
            float max = range() / density;
            if (config.snaps()) {
                float lead = horizontal ? config.insetLeft : config.insetTop;
                float viewport = (horizontal ? view.getWidth() : view.getHeight()) / density;
                float length = max + viewport - lead; // the content and its trailing inset
                float to = snapTarget(config, pos - lead, projected(pos, v, config.rate) - lead, v, viewport, length);
                if (!Float.isNaN(to)) {
                    int target = Math.min(Math.max(Math.round((to + lead) * density), 0), range());
                    long duration = snapDuration(target / density - pos, v);
                    if (duration > 0) {
                        momentum = true;
                        event(Protocol.EVENT_MOMENTUM_START);
                        ease(target, duration, true);
                    }
                    return;
                }
            }
            if (Math.abs(v) > STOP * 5 && (v > 0 && pos < max || v < 0 && pos > 0)) glide(v);
        }

        private void glide(float velocity) {
            momentum = true;
            event(Protocol.EVENT_MOMENTUM_START);
            final float rate = config.rate;
            carry = 0;
            final float[] state = {pos(), velocity * density, SystemClock.uptimeMillis()};
            motion = new Runnable() {
                @Override
                public void run() {
                    if (motion != this) return;
                    long now = SystemClock.uptimeMillis();
                    float dt = Math.max(now - state[2], 0);
                    double k = Math.pow(rate, dt);
                    state[0] += (float) (state[1] * (k - 1) / Math.log(rate));
                    state[1] *= (float) k;
                    state[2] = now;
                    int max = range();
                    float at = state[0] + carry;
                    boolean done = Math.abs(state[1]) < STOP * density || at <= 0 || at >= max;
                    move(Math.round(Math.min(Math.max(at, 0), max)));
                    if (done) finish();
                    else view.postOnAnimation(this);
                }
            };
            view.postOnAnimation(motion);
        }

        private void ease(int target, long duration, boolean isMomentum) {
            final int from = pos();
            final long start = SystemClock.uptimeMillis();
            carry = 0;
            motion = new Runnable() {
                @Override
                public void run() {
                    if (motion != this) return;
                    float t = Math.min((SystemClock.uptimeMillis() - start) / (float) duration, 1);
                    float k = 1 - (1 - t) * (1 - t) * (1 - t);
                    move(Math.round(from + carry + (target - from) * k));
                    if (t >= 1) finish();
                    else view.postOnAnimation(this);
                }
            };
            view.postOnAnimation(motion);
        }

        /** A glide, a snap or an eased scroll ran to its end: the view is at rest. */
        private void finish() {
            motion = null;
            if (momentum) {
                momentum = false;
                event(Protocol.EVENT_MOMENTUM_END);
            }
            event(Protocol.EVENT_SCROLL_END);
        }

        /** Shows the spinner, or the app's indicator, and its room above the content, or takes them away. */
        void setRefreshing(boolean on) {
            float room = config.refreshSpace() * density;
            if (on == refreshing && !(on && !pulling && pullTarget != room)) return;
            refreshing = on;
            if (pulling) return;
            settlePull(on ? room : 0);
        }

        private void settlePull(float to) {
            pullTarget = to;
            showSpinner(to > 0 || pull > 0);
            content.animate().translationY(to).setDuration(ANIMATION).setUpdateListener(a -> {
                pull = content.getTranslationY();
                host.scrolled(offsetX(), offsetY(), 1);
            }).withEndAction(() -> {
                pull = to;
                if (to == 0) showSpinner(false);
                host.scrolled(offsetX(), offsetY(), 1);
            }).start();
        }

        private void showSpinner(boolean on) {
            on = on && config.refreshIndicator <= 0;
            if (on && spinner == null) {
                spinner = new ProgressBar(view.getContext());
                spinner.setIndeterminate(true);
                int side = Math.round(28 * density);
                FrameLayout.LayoutParams lp = new FrameLayout.LayoutParams(side, side, Gravity.TOP | Gravity.CENTER_HORIZONTAL);
                lp.topMargin = Math.round((config.refreshSpace() * density - side) / 2 + config.insetTop * density);
                frame.addView(spinner, lp);
            } else if (!on && spinner != null) {
                frame.removeView(spinner);
                spinner = null;
            }
        }
    }

    /** A vertical scroll view. */
    static final class VScroll extends ScrollView implements Scroller {
        final Driver d;

        VScroll(Context context, Host host, float density, View content, ViewGroup frame) {
            super(context);
            d = new Driver(this, false, host, density, content, frame);
        }

        @Override
        public boolean onInterceptTouchEvent(MotionEvent e) {
            if (d.config.noScroll) return false;
            boolean pull = d.intercept(e);
            return super.onInterceptTouchEvent(e) || pull;
        }

        @Override
        public boolean onTouchEvent(MotionEvent e) {
            if (d.config.noScroll) return false;
            if (d.before(e)) return true;
            boolean r = super.onTouchEvent(e);
            d.after(e);
            return r;
        }

        @Override
        public void fling(int velocityY) {
            d.fling(velocityY);
        }

        @Override
        public void configure(Config c) {
            d.configure(c);
        }

        @Override
        public void scrollToPoints(float x, float y, boolean animated) {
            d.scrollTo(x, y, animated);
        }

        @Override
        public void scrollByPoints(float dx, float dy) {
            d.scrollBy(dx, dy);
        }

        @Override
        public float offsetX() {
            return d.offsetX();
        }

        @Override
        public float offsetY() {
            return d.offsetY();
        }

        @Override
        public float pulled() {
            return d.pulled();
        }
    }

    /** A horizontal scroll view. */
    static final class HScroll extends HorizontalScrollView implements Scroller {
        final Driver d;

        HScroll(Context context, Host host, float density, View content, ViewGroup frame) {
            super(context);
            d = new Driver(this, true, host, density, content, frame);
        }

        @Override
        public boolean onInterceptTouchEvent(MotionEvent e) {
            if (d.config.noScroll) return false;
            d.intercept(e);
            return super.onInterceptTouchEvent(e);
        }

        @Override
        public boolean onTouchEvent(MotionEvent e) {
            if (d.config.noScroll) return false;
            d.before(e);
            boolean r = super.onTouchEvent(e);
            d.after(e);
            return r;
        }

        @Override
        public void fling(int velocityX) {
            d.fling(velocityX);
        }

        @Override
        public void configure(Config c) {
            d.configure(c);
        }

        @Override
        public void scrollToPoints(float x, float y, boolean animated) {
            d.scrollTo(x, y, animated);
        }

        @Override
        public void scrollByPoints(float dx, float dy) {
            d.scrollBy(dx, dy);
        }

        @Override
        public float offsetX() {
            return d.offsetX();
        }

        @Override
        public float offsetY() {
            return d.offsetY();
        }

        @Override
        public float pulled() {
            return 0;
        }
    }

    /**
     * A zoomable scroll view: Android has none, so it scales its content
     * around its top left and scrolls both ways itself, pinched around the
     * fingers' focal point, panned with any number of fingers and flung
     * with an OverScroller. Pinches past the limits spring back unless
     * NoBouncesZoom.
     */
    static final class ZoomScroll extends FrameLayout implements Scroller {
        private final Host host;
        private final float density;
        private final View content;
        private final ScaleGestureDetector scale;
        private final OverScroller flinger;
        private final android.view.VelocityTracker[] tracker = {null};
        private final int slop;
        private Config config = new Config();
        private float zoom = 1;
        private float lastX, lastY, downX, downY;
        private boolean dragging, pinching, momentum;

        ZoomScroll(Context context, Host host, float density, View content) {
            super(context);
            this.host = host;
            this.density = density;
            this.content = content;
            content.setPivotX(0);
            content.setPivotY(0);
            flinger = new OverScroller(context);
            slop = ViewConfiguration.get(context).getScaledTouchSlop();
            scale = new ScaleGestureDetector(context, new ScaleGestureDetector.SimpleOnScaleGestureListener() {
                @Override
                public boolean onScaleBegin(ScaleGestureDetector g) {
                    if (config.noPinchZoom) return false;
                    pinching = true;
                    if (!dragging) {
                        dragging = true;
                        event(Protocol.EVENT_SCROLL_DRAG_START);
                    }
                    return true;
                }

                @Override
                public boolean onScale(ScaleGestureDetector g) {
                    zoomAround(limit(zoom * g.getScaleFactor(), true), g.getFocusX(), g.getFocusY());
                    return true;
                }
            });
            setClipChildren(true);
        }

        private void event(int kind) {
            host.scrollEvent(kind, offsetX(), offsetY(), zoom);
        }

        private float limit(float z, boolean rubber) {
            float lo = config.minZoom, hi = config.maxZoom;
            if (z > hi) return rubber && !config.noBouncesZoom ? hi * (float) Math.pow(z / hi, 0.3) : hi;
            if (z < lo) return rubber && !config.noBouncesZoom ? lo * (float) Math.pow(z / lo, 0.3) : lo;
            return z;
        }

        /** Where the content starts: inside the inset, centered while smaller than the viewport. */
        private float originX() {
            float in = config.insetLeft * density;
            if (!config.centerContent) return in;
            float free = getWidth() - in - config.insetRight * density - content.getWidth() * zoom;
            return in + Math.max(free / 2, 0);
        }

        private float originY() {
            float in = config.insetTop * density;
            if (!config.centerContent) return in;
            float free = getHeight() - in - config.insetBottom * density - content.getHeight() * zoom;
            return in + Math.max(free / 2, 0);
        }

        private int maxX() {
            return (int) Math.max(content.getWidth() * zoom + (config.insetLeft + config.insetRight) * density - getWidth(), 0);
        }

        private int maxY() {
            return (int) Math.max(content.getHeight() * zoom + (config.insetTop + config.insetBottom) * density - getHeight(), 0);
        }

        private void place() {
            content.setScaleX(zoom);
            content.setScaleY(zoom);
            content.setTranslationX(originX());
            content.setTranslationY(originY());
        }

        private void moveTo(int x, int y) {
            scrollTo(Math.min(Math.max(x, 0), maxX()), Math.min(Math.max(y, 0), maxY()));
        }

        @Override
        protected void onScrollChanged(int l, int t, int oldl, int oldt) {
            super.onScrollChanged(l, t, oldl, oldt);
            host.scrolled(offsetX(), offsetY(), zoom);
        }

        @Override
        protected void onLayout(boolean changed, int l, int t, int r, int b) {
            super.onLayout(changed, l, t, r, b);
            place();
            moveTo(getScrollX(), getScrollY());
        }

        /** Zooms to z keeping the content under the focal point fx, fy (in the view) in place. */
        private void zoomAround(float z, float fx, float fy) {
            float cx = (getScrollX() + fx - originX()) / zoom, cy = (getScrollY() + fy - originY()) / zoom;
            zoom = z;
            place();
            moveTo(Math.round(cx * zoom + originX() - fx), Math.round(cy * zoom + originY() - fy));
            host.scrolled(offsetX(), offsetY(), zoom);
        }

        void zoomTo(float z, boolean hasCenter, float x, float y, boolean animated) {
            final float target = limit(z, false);
            final float fx = getWidth() / 2f, fy = getHeight() / 2f;
            final float cx = hasCenter ? x * density : (getScrollX() + fx - originX()) / zoom;
            final float cy = hasCenter ? y * density : (getScrollY() + fy - originY()) / zoom;
            if (!animated) {
                zoomCentered(target, cx, cy);
                return;
            }
            final float z0 = zoom;
            final float x0 = (getScrollX() + fx - originX()) / zoom, y0 = (getScrollY() + fy - originY()) / zoom;
            final long start = SystemClock.uptimeMillis();
            postOnAnimation(new Runnable() {
                @Override
                public void run() {
                    float t = Math.min((SystemClock.uptimeMillis() - start) / 250f, 1);
                    float k = 1 - (1 - t) * (1 - t) * (1 - t);
                    zoomCentered(z0 + (target - z0) * k, x0 + (cx - x0) * k, y0 + (cy - y0) * k);
                    if (t < 1) postOnAnimation(this);
                }
            });
        }

        /** Zooms to z with content point cx, cy (pixels, unzoomed) in the middle. */
        private void zoomCentered(float z, float cx, float cy) {
            zoom = z;
            place();
            moveTo(Math.round(cx * zoom + originX() - getWidth() / 2f), Math.round(cy * zoom + originY() - getHeight() / 2f));
            host.scrolled(offsetX(), offsetY(), zoom);
        }

        /**
         * Content that pans keeps the drag from a scroll view around it,
         * which takes what is left at its edges.
         */
        private void nest() {
            if (maxX() > 0 || maxY() > 0) startNestedScroll(SCROLL_AXIS_HORIZONTAL | SCROLL_AXIS_VERTICAL);
        }

        @Override
        public boolean onInterceptTouchEvent(MotionEvent e) {
            if (config.noScroll) return false;
            switch (e.getActionMasked()) {
                case MotionEvent.ACTION_DOWN:
                    downX = lastX = e.getX();
                    downY = lastY = e.getY();
                    if (!flinger.isFinished()) {
                        flinger.forceFinished(true);
                        endMomentum();
                    }
                    scale.onTouchEvent(e);
                    nest();
                    return false;
                case MotionEvent.ACTION_POINTER_DOWN:
                    return true;
                case MotionEvent.ACTION_MOVE:
                    return Math.hypot(e.getX() - downX, e.getY() - downY) > slop;
            }
            return false;
        }

        @Override
        public boolean onTouchEvent(MotionEvent e) {
            if (config.noScroll) return false;
            scale.onTouchEvent(e);
            if (tracker[0] == null) tracker[0] = android.view.VelocityTracker.obtain();
            tracker[0].addMovement(e);
            float x = e.getX(), y = e.getY();
            if (e.getPointerCount() > 1) {
                // The fingers' middle pans while they pinch.
                x = (e.getX(0) + e.getX(1)) / 2;
                y = (e.getY(0) + e.getY(1)) / 2;
            }
            switch (e.getActionMasked()) {
                case MotionEvent.ACTION_DOWN:
                    downX = lastX = x;
                    downY = lastY = y;
                    nest(); // View's dispatch stopped the one onInterceptTouchEvent started
                    return true;
                case MotionEvent.ACTION_POINTER_DOWN:
                case MotionEvent.ACTION_POINTER_UP:
                    lastX = x;
                    lastY = y;
                    return true;
                case MotionEvent.ACTION_MOVE:
                    if (!dragging && Math.hypot(x - downX, y - downY) > slop) {
                        dragging = true;
                        event(Protocol.EVENT_SCROLL_DRAG_START);
                        if (config.dismiss != 0) host.dismissKeyboard();
                        getParent().requestDisallowInterceptTouchEvent(true);
                    }
                    if (dragging) {
                        int fromX = getScrollX(), fromY = getScrollY();
                        int dx = Math.round(-(x - lastX)), dy = Math.round(-(y - lastY));
                        moveTo(fromX + dx, fromY + dy);
                        int movedX = getScrollX() - fromX, movedY = getScrollY() - fromY;
                        dispatchNestedScroll(movedX, movedY, dx - movedX, dy - movedY, null);
                    }
                    lastX = x;
                    lastY = y;
                    return true;
                case MotionEvent.ACTION_UP:
                case MotionEvent.ACTION_CANCEL:
                    stopNestedScroll();
                    tracker[0].computeCurrentVelocity(1000);
                    float vx = tracker[0].getXVelocity(), vy = tracker[0].getYVelocity();
                    tracker[0].recycle();
                    tracker[0] = null;
                    if (pinching) {
                        pinching = false;
                        float settled = limit(zoom, false);
                        if (settled != zoom) zoomTo(settled, false, 0, 0, true);
                    }
                    if (dragging) {
                        dragging = false;
                        event(Protocol.EVENT_SCROLL_DRAG_END);
                        flinger.fling(getScrollX(), getScrollY(), -Math.round(vx), -Math.round(vy), 0, maxX(), 0, maxY());
                        if (!flinger.isFinished()) {
                            momentum = true;
                            event(Protocol.EVENT_MOMENTUM_START);
                            postInvalidateOnAnimation();
                        } else {
                            event(Protocol.EVENT_SCROLL_END);
                        }
                    }
                    return true;
            }
            return true;
        }

        @Override
        public void computeScroll() {
            if (flinger.computeScrollOffset()) {
                moveTo(flinger.getCurrX(), flinger.getCurrY());
                postInvalidateOnAnimation();
            } else if (momentum) {
                // The fling ran to its end: at rest.
                endMomentum();
                event(Protocol.EVENT_SCROLL_END);
            }
        }

        private void endMomentum() {
            if (momentum) {
                momentum = false;
                event(Protocol.EVENT_MOMENTUM_END);
            }
        }

        @Override
        public void configure(Config c) {
            float old = config.zoom;
            boolean first = !config.zooms();
            config = c;
            setVerticalScrollBarEnabled(!c.hideIndicator);
            setHorizontalScrollBarEnabled(!c.hideIndicator);
            if (first || c.zoom != old) {
                if (first) zoom = limit(c.zoom, false);
                else zoomTo(c.zoom, false, 0, 0, true);
            } else {
                zoom = limit(zoom, false);
            }
            place();
            moveTo(getScrollX(), getScrollY());
        }

        @Override
        public void scrollToPoints(float x, float y, boolean animated) {
            moveTo(Math.round((x + config.insetLeft) * density), Math.round((y + config.insetTop) * density));
        }

        @Override
        public void scrollByPoints(float dx, float dy) {
            moveTo(getScrollX() + Math.round(dx * density), getScrollY() + Math.round(dy * density));
        }

        @Override
        public float offsetX() {
            return getScrollX() / density - config.insetLeft;
        }

        @Override
        public float offsetY() {
            return getScrollY() / density - config.insetTop;
        }

        @Override
        public float pulled() {
            return 0;
        }
    }
}
