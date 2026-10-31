package dev.oneo.nsgo;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.BlendMode;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.DashPathEffect;
import android.graphics.LinearGradient;
import android.graphics.Matrix;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.PorterDuff;
import android.graphics.PorterDuffXfermode;
import android.graphics.RadialGradient;
import android.graphics.Rect;
import android.graphics.RectF;
import android.graphics.Shader;
import android.graphics.SweepGradient;
import android.graphics.Typeface;
import android.os.Build;
import android.os.Bundle;
import android.view.MotionEvent;
import android.view.View;
import android.view.accessibility.AccessibilityEvent;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityNodeProvider;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.ArrayDeque;
import java.util.HashMap;
import java.util.Map;

/**
 * A canvas node: it plays the latest drawing the engine recorded, as
 * android.go's writeDrawing writes it, in points over its background. The
 * ops play into a layer of their own, so clearing and the operators that
 * clear leave the background. Paths are built in the canvas's points, each
 * point through the transform current when it is added, as the web's
 * canvas builds them; strokes, text and images draw in the transform.
 * Arguments bound to animated nodes are patched into the ops each frame
 * they change. Named parts are virtual views for assistive technology.
 */
final class CanvasNode extends View {
    /** An image canvases draw, decoded once, with its pixels a point. */
    static final class Picture {
        final Bitmap bitmap;
        final float scale;

        Picture(Bitmap bitmap, float scale) {
            this.bitmap = bitmap;
            this.scale = scale > 0 ? scale : 1;
        }
    }

    /** The images canvases draw, by core.ImageSource.Key; UI thread only. */
    static final Map<String, Picture> pictures = new HashMap<>();

    /** A named part of the drawing, in points. */
    static final class Part {
        final float x, y, w, h;
        final int role;
        final String label, testID;
        final boolean selected;

        Part(float x, float y, float w, float h, int role, String label, String testID, boolean selected) {
            this.x = x;
            this.y = y;
            this.w = w;
            this.h = h;
            this.role = role;
            this.label = label;
            this.testID = testID;
            this.selected = selected;
        }
    }

    /** Presses at a point on the screen, on the view there that takes presses. */
    interface Presser {
        boolean pressAt(float rawX, float rawY);
    }

    private byte[] ops;
    private int[] offsets = new int[0];
    private boolean[] colors = new boolean[0];
    private Part[] parts = new Part[0];
    Presser presser;
    private int hovered = -1;

    CanvasNode(Context context) {
        super(context);
    }

    void setOps(byte[] ops, int[] offsets, boolean[] colors, Part[] parts) {
        this.ops = ops;
        this.offsets = offsets;
        this.colors = colors;
        boolean changed = parts.length != this.parts.length;
        this.parts = parts;
        if (changed) {
            setImportantForAccessibility(parts.length > 0 ? IMPORTANT_FOR_ACCESSIBILITY_YES
                    : IMPORTANT_FOR_ACCESSIBILITY_AUTO);
        }
        if (parts.length > 0) sendAccessibilityEvent(AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED);
        invalidate();
    }

    /** Writes the values of the bound nodes into the ops and draws again. */
    void patch(double[] nums, int[] cols) {
        if (ops == null) return;
        ByteBuffer b = ByteBuffer.wrap(ops);
        for (int i = 0; i < offsets.length && i < nums.length; i++) {
            if (offsets[i] < 0 || offsets[i] + 4 > ops.length) continue;
            if (colors[i]) {
                b.putInt(offsets[i], cols[i]);
            } else {
                b.putFloat(offsets[i], (float) nums[i]);
            }
        }
        invalidate();
    }

    @Override
    protected void onDraw(Canvas canvas) {
        if (ops == null || ops.length == 0) return;
        int layer = canvas.saveLayer(0, 0, getWidth(), getHeight(), null);
        float density = getResources().getDisplayMetrics().density;
        canvas.scale(density, density);
        new Player(canvas, getWidth() / density, getHeight() / density).play(ByteBuffer.wrap(ops));
        canvas.restoreToCount(layer);
    }

    // Assistive technology sees the parts as virtual views.

    @Override
    public AccessibilityNodeProvider getAccessibilityNodeProvider() {
        if (parts.length == 0) return super.getAccessibilityNodeProvider();
        return provider;
    }

    @Override
    public boolean dispatchHoverEvent(MotionEvent e) {
        if (parts.length == 0) return super.dispatchHoverEvent(e);
        float density = getResources().getDisplayMetrics().density;
        int at = -1;
        if (e.getAction() != MotionEvent.ACTION_HOVER_EXIT) {
            float x = e.getX() / density, y = e.getY() / density;
            for (int i = parts.length - 1; i >= 0; i--) {
                Part p = parts[i];
                if (x >= p.x && y >= p.y && x < p.x + p.w && y < p.y + p.h) {
                    at = i;
                    break;
                }
            }
        }
        if (at != hovered) {
            if (at >= 0) sendPartEvent(at, AccessibilityEvent.TYPE_VIEW_HOVER_ENTER);
            if (hovered >= 0) sendPartEvent(hovered, AccessibilityEvent.TYPE_VIEW_HOVER_EXIT);
            hovered = at;
        }
        return at >= 0 || super.dispatchHoverEvent(e);
    }

    private void sendPartEvent(int part, int type) {
        AccessibilityEvent e = AccessibilityEvent.obtain(type);
        e.setPackageName(getContext().getPackageName());
        e.setSource(this, part);
        if (getParent() != null) getParent().requestSendAccessibilityEvent(this, e);
    }

    private final AccessibilityNodeProvider provider = new AccessibilityNodeProvider() {
        @Override
        public AccessibilityNodeInfo createAccessibilityNodeInfo(int id) {
            float density = getResources().getDisplayMetrics().density;
            if (id == View.NO_ID) {
                AccessibilityNodeInfo info = AccessibilityNodeInfo.obtain(CanvasNode.this);
                onInitializeAccessibilityNodeInfo(info);
                for (int i = 0; i < parts.length; i++) info.addChild(CanvasNode.this, i);
                return info;
            }
            if (id < 0 || id >= parts.length) return null;
            Part p = parts[id];
            AccessibilityNodeInfo info = AccessibilityNodeInfo.obtain(CanvasNode.this, id);
            info.setParent(CanvasNode.this);
            info.setPackageName(getContext().getPackageName());
            info.setEnabled(true);
            info.setVisibleToUser(true);
            info.setContentDescription(p.label.isEmpty() ? null : p.label);
            info.setSelected(p.selected);
            if (!p.testID.isEmpty()) info.setViewIdResourceName(p.testID);
            switch (p.role) {
                case Protocol.ROLE_BUTTON: info.setClassName("android.widget.Button"); break;
                case Protocol.ROLE_IMAGE: info.setClassName("android.widget.ImageView"); break;
                case Protocol.ROLE_TEXT: info.setClassName("android.widget.TextView"); break;
                case Protocol.ROLE_HEADER: info.setHeading(true); break;
                default: break;
            }
            info.setClickable(true);
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_CLICK);
            Rect parent = new Rect((int) (p.x * density), (int) (p.y * density), (int) ((p.x + p.w) * density),
                    (int) ((p.y + p.h) * density));
            info.setBoundsInParent(parent);
            int[] at = new int[2];
            getLocationOnScreen(at);
            parent.offset(at[0], at[1]);
            info.setBoundsInScreen(parent);
            return info;
        }

        @Override
        public boolean performAction(int id, int action, Bundle arguments) {
            if (id == View.NO_ID) return performAccessibilityAction(action, arguments);
            if (id < 0 || id >= parts.length || action != AccessibilityNodeInfo.ACTION_CLICK || presser == null) {
                return false;
            }
            Part p = parts[id];
            float density = getResources().getDisplayMetrics().density;
            int[] at = new int[2];
            getLocationOnScreen(at);
            return presser.pressAt(at[0] + (p.x + p.w / 2) * density, at[1] + (p.y + p.h / 2) * density);
        }
    };

    /** A gradient as android.go's writeGradient writes it, made a shader in the space it paints in. */
    private static final class Gradient {
        int kind; // 0 linear, 1 radial, 2 conic
        float x1, y1, r1, x2, y2, r2, angle;
        final float[] m = new float[6];
        int spread;
        float[] offsets;
        int[] colors;

        static Gradient read(ByteBuffer b) {
            Gradient g = new Gradient();
            g.kind = b.get();
            g.x1 = b.getFloat();
            g.y1 = b.getFloat();
            g.r1 = b.getFloat();
            g.x2 = b.getFloat();
            g.y2 = b.getFloat();
            g.r2 = b.getFloat();
            g.angle = b.getFloat();
            for (int i = 0; i < 6; i++) g.m[i] = b.getFloat();
            g.spread = b.get();
            int n = b.getInt();
            g.offsets = new float[Math.max(n, 2)];
            g.colors = new int[Math.max(n, 2)];
            for (int i = 0; i < n; i++) {
                g.offsets[i] = b.getFloat();
                g.colors[i] = Text.argb(b.getInt());
            }
            if (n == 0) return null;
            if (n == 1) {
                g.offsets[1] = 1;
                g.colors[1] = g.colors[0];
            }
            return g;
        }

        /**
         * The shader, its gradient space mapped through the gradient's own
         * transform, then to; null for one Android cannot paint. Before API
         * 31 a radial gradient's focal circle is its center.
         */
        Shader shader(Matrix to) {
            Shader.TileMode tile = spread == Protocol.SPREAD_REFLECT ? Shader.TileMode.MIRROR
                    : spread == Protocol.SPREAD_REPEAT ? Shader.TileMode.REPEAT : Shader.TileMode.CLAMP;
            Shader shader;
            try {
                if (kind == 2) {
                    shader = new SweepGradient(x1, y1, colors, offsets);
                } else if (kind == 0) {
                    shader = new LinearGradient(x1, y1, x2, y2, colors, offsets, tile);
                } else if (Build.VERSION.SDK_INT >= 31) {
                    long[] packed = new long[colors.length];
                    for (int i = 0; i < colors.length; i++) packed[i] = Color.pack(colors[i]);
                    shader = new RadialGradient(x1, y1, Math.max(r1, 0), x2, y2, r2, packed, offsets, tile);
                } else {
                    if (r2 <= 0) return null;
                    // The focal circle's radius moves the stops outward.
                    float inner = Math.max(0, Math.min(r1 / r2, 1));
                    float[] stops = new float[offsets.length];
                    for (int i = 0; i < stops.length; i++) stops[i] = inner + offsets[i] * (1 - inner);
                    shader = new RadialGradient(x2, y2, r2, colors, stops, tile);
                }
            } catch (IllegalArgumentException e) {
                return null;
            }
            Matrix local = new Matrix(to);
            boolean identity = true;
            for (float v : m) identity &= v == 0;
            if (!identity) {
                Matrix own = new Matrix();
                own.setValues(new float[] {m[0], m[2], m[4], m[1], m[3], m[5], 0, 0, 1});
                local.preConcat(own);
            }
            // A sweep starts along the x axis: turn it to its angle.
            if (kind == 2) local.preRotate((float) Math.toDegrees(angle), x1, y1);
            shader.setLocalMatrix(local);
            return shader;
        }
    }

    /** The settings Save keeps and Restore brings back, besides the canvas's clip. */
    private static final class State {
        final Matrix ctm = new Matrix();
        int fill = Color.BLACK, stroke = Color.BLACK;
        float lineWidth = 1, alpha = 1, miter = 10, dashOffset;
        float[] dash;
        Paint.Cap cap = Paint.Cap.BUTT;
        Paint.Join join = Paint.Join.MITER;
        /** The gradients filling and stroking in place of the colors, or null. */
        Gradient fillGradient, strokeGradient;
        int composite;
        int shadowColor;
        float shadowX, shadowY, shadowBlur;

        State copy() {
            State s = new State();
            s.ctm.set(ctm);
            s.fill = fill;
            s.stroke = stroke;
            s.lineWidth = lineWidth;
            s.alpha = alpha;
            s.miter = miter;
            s.dash = dash;
            s.dashOffset = dashOffset;
            s.cap = cap;
            s.join = join;
            s.fillGradient = fillGradient;
            s.strokeGradient = strokeGradient;
            s.composite = composite;
            s.shadowColor = shadowColor;
            s.shadowX = shadowX;
            s.shadowY = shadowY;
            s.shadowBlur = shadowBlur;
            return s;
        }
    }

    // core.CompositeOp values.
    private static final int SOURCE_OVER = 0, SOURCE_IN = 1, SOURCE_OUT = 2, SOURCE_ATOP = 3, DESTINATION_OVER = 4,
            DESTINATION_IN = 5, DESTINATION_OUT = 6, DESTINATION_ATOP = 7, LIGHTER = 8, COPY = 9, XOR = 10,
            MULTIPLY = 11;

    private static final PorterDuff.Mode[] PORTER_DUFF = {PorterDuff.Mode.SRC_OVER, PorterDuff.Mode.SRC_IN,
            PorterDuff.Mode.SRC_OUT, PorterDuff.Mode.SRC_ATOP, PorterDuff.Mode.DST_OVER, PorterDuff.Mode.DST_IN,
            PorterDuff.Mode.DST_OUT, PorterDuff.Mode.DST_ATOP, PorterDuff.Mode.ADD, PorterDuff.Mode.SRC,
            PorterDuff.Mode.XOR, PorterDuff.Mode.MULTIPLY, PorterDuff.Mode.SCREEN, PorterDuff.Mode.OVERLAY,
            PorterDuff.Mode.DARKEN, PorterDuff.Mode.LIGHTEN};

    private static final class Player {
        private final Canvas canvas;
        private final float width, height;
        private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
        private final ArrayDeque<State> saved = new ArrayDeque<>();
        private final float[] pt = new float[6];
        private Path path = new Path();
        private boolean has;
        private float curX, curY, startX, startY; // in the canvas's points
        private State state = new State();

        Player(Canvas canvas, float width, float height) {
            this.canvas = canvas;
            this.width = width;
            this.height = height;
        }

        void play(ByteBuffer b) {
            while (b.hasRemaining()) {
                switch (b.get()) {
                    case Protocol.DRAW_SAVE:
                        canvas.save();
                        saved.push(state.copy());
                        break;
                    case Protocol.DRAW_RESTORE:
                        if (saved.isEmpty()) break;
                        canvas.restore();
                        state = saved.pop();
                        break;
                    case Protocol.DRAW_TRANSLATE:
                        state.ctm.preTranslate(b.getFloat(), b.getFloat());
                        break;
                    case Protocol.DRAW_SCALE:
                        state.ctm.preScale(b.getFloat(), b.getFloat());
                        break;
                    case Protocol.DRAW_ROTATE:
                        state.ctm.preRotate((float) Math.toDegrees(b.getFloat()));
                        break;
                    case Protocol.DRAW_TRANSFORM:
                        state.ctm.preConcat(matrix(b));
                        break;
                    case Protocol.DRAW_SET_TRANSFORM:
                        state.ctm.set(matrix(b));
                        break;
                    case Protocol.DRAW_BEGIN_PATH:
                        path = new Path();
                        has = false;
                        break;
                    case Protocol.DRAW_MOVE_TO:
                        moveTo(b.getFloat(), b.getFloat());
                        break;
                    case Protocol.DRAW_LINE_TO:
                        lineTo(b.getFloat(), b.getFloat());
                        break;
                    case Protocol.DRAW_QUAD_TO: {
                        float cx = b.getFloat(), cy = b.getFloat(), x = b.getFloat(), y = b.getFloat();
                        if (!has) moveTo(cx, cy);
                        map(cx, cy, x, y);
                        path.quadTo(pt[0], pt[1], pt[2], pt[3]);
                        curX = pt[2];
                        curY = pt[3];
                        break;
                    }
                    case Protocol.DRAW_CUBIC_TO: {
                        float x1 = b.getFloat(), y1 = b.getFloat(), x2 = b.getFloat(), y2 = b.getFloat();
                        float x = b.getFloat(), y = b.getFloat();
                        if (!has) moveTo(x1, y1);
                        cubic(x1, y1, x2, y2, x, y);
                        break;
                    }
                    case Protocol.DRAW_ARC:
                        arc(b.getFloat(), b.getFloat(), b.getFloat(), b.getFloat(), b.getFloat(), b.get() != 0);
                        break;
                    case Protocol.DRAW_RECT: {
                        float x = b.getFloat(), y = b.getFloat(), w = b.getFloat(), h = b.getFloat();
                        moveTo(x, y);
                        lineTo(x + w, y);
                        lineTo(x + w, y + h);
                        lineTo(x, y + h);
                        close();
                        moveTo(x, y);
                        break;
                    }
                    case Protocol.DRAW_ROUND_RECT: {
                        float x = b.getFloat(), y = b.getFloat(), w = b.getFloat(), h = b.getFloat(), r = b.getFloat();
                        r = Math.max(0, Math.min(r, Math.min(Math.abs(w), Math.abs(h)) / 2));
                        moveTo(x + r, y);
                        lineTo(x + w - r, y);
                        ellipseArc(x + w - r, y + r, r, r, -Math.PI / 2, Math.PI / 2);
                        lineTo(x + w, y + h - r);
                        ellipseArc(x + w - r, y + h - r, r, r, 0, Math.PI / 2);
                        lineTo(x + r, y + h);
                        ellipseArc(x + r, y + h - r, r, r, Math.PI / 2, Math.PI / 2);
                        lineTo(x, y + r);
                        ellipseArc(x + r, y + r, r, r, Math.PI, Math.PI / 2);
                        close();
                        break;
                    }
                    case Protocol.DRAW_ELLIPSE: {
                        float x = b.getFloat(), y = b.getFloat(), rx = b.getFloat(), ry = b.getFloat();
                        moveTo(x + rx, y);
                        ellipseArc(x, y, rx, ry, 0, 2 * Math.PI);
                        close();
                        break;
                    }
                    case Protocol.DRAW_CLOSE_PATH:
                        close();
                        break;
                    case Protocol.DRAW_FILL:
                        path.setFillType(b.get() != 0 ? Path.FillType.EVEN_ODD : Path.FillType.WINDING);
                        fill();
                        break;
                    case Protocol.DRAW_STROKE:
                        stroke();
                        break;
                    case Protocol.DRAW_CLIP:
                        path.setFillType(b.get() != 0 ? Path.FillType.EVEN_ODD : Path.FillType.WINDING);
                        canvas.clipPath(path);
                        break;
                    case Protocol.DRAW_FILL_COLOR:
                        state.fill = Text.argb(b.getInt());
                        state.fillGradient = null;
                        break;
                    case Protocol.DRAW_STROKE_COLOR:
                        state.stroke = Text.argb(b.getInt());
                        state.strokeGradient = null;
                        break;
                    case Protocol.DRAW_FILL_GRADIENT: {
                        // An invalid gradient keeps the color before it.
                        Gradient g = Gradient.read(b);
                        if (g != null) state.fillGradient = g;
                        break;
                    }
                    case Protocol.DRAW_STROKE_GRADIENT: {
                        Gradient g = Gradient.read(b);
                        if (g != null) state.strokeGradient = g;
                        break;
                    }
                    case Protocol.DRAW_LINE_WIDTH:
                        state.lineWidth = b.getFloat();
                        break;
                    case Protocol.DRAW_LINE_CAP: {
                        int cap = b.get();
                        state.cap = cap == Protocol.CAP_ROUND ? Paint.Cap.ROUND
                                : cap == Protocol.CAP_SQUARE ? Paint.Cap.SQUARE : Paint.Cap.BUTT;
                        break;
                    }
                    case Protocol.DRAW_LINE_JOIN: {
                        int join = b.get();
                        state.join = join == Protocol.JOIN_ROUND ? Paint.Join.ROUND
                                : join == Protocol.JOIN_BEVEL ? Paint.Join.BEVEL : Paint.Join.MITER;
                        break;
                    }
                    case Protocol.DRAW_MITER_LIMIT:
                        state.miter = b.getFloat();
                        break;
                    case Protocol.DRAW_LINE_DASH: {
                        int n = b.getInt();
                        float[] dash = new float[n];
                        for (int i = 0; i < n; i++) dash[i] = b.getFloat();
                        boolean any = false;
                        for (float d : dash) any |= d > 0;
                        state.dash = n >= 2 && any ? dash : null;
                        break;
                    }
                    case Protocol.DRAW_LINE_DASH_OFFSET:
                        state.dashOffset = b.getFloat();
                        break;
                    case Protocol.DRAW_ALPHA:
                        state.alpha = Math.max(0, Math.min(1, b.getFloat()));
                        break;
                    case Protocol.DRAW_COMPOSITE:
                        state.composite = b.get();
                        break;
                    case Protocol.DRAW_SHADOW:
                        state.shadowColor = Text.argb(b.getInt());
                        state.shadowX = b.getFloat();
                        state.shadowY = b.getFloat();
                        state.shadowBlur = Math.max(0, b.getFloat());
                        break;
                    case Protocol.DRAW_FILL_TEXT:
                        text(b, false);
                        break;
                    case Protocol.DRAW_STROKE_TEXT:
                        text(b, true);
                        break;
                    case Protocol.DRAW_IMAGE:
                        image(b);
                        break;
                    case Protocol.DRAW_CLEAR: {
                        float x = b.getFloat(), y = b.getFloat(), w = b.getFloat(), h = b.getFloat();
                        canvas.save();
                        canvas.concat(state.ctm);
                        canvas.clipRect(x, y, x + w, y + h);
                        canvas.drawColor(Color.TRANSPARENT, PorterDuff.Mode.CLEAR);
                        canvas.restore();
                        break;
                    }
                    default:
                        return; // an op this runtime does not know: its fields are unknown too
                }
            }
        }

        private static Matrix matrix(ByteBuffer b) {
            float a = b.getFloat(), bb = b.getFloat(), c = b.getFloat(), d = b.getFloat();
            float e = b.getFloat(), f = b.getFloat();
            Matrix m = new Matrix();
            m.setValues(new float[] {a, c, e, bb, d, f, 0, 0, 1});
            return m;
        }

        /** Maps up to three points through the transform into pt. */
        private void map(float... xy) {
            System.arraycopy(xy, 0, pt, 0, xy.length);
            state.ctm.mapPoints(pt, 0, pt, 0, xy.length / 2);
        }

        private void moveTo(float x, float y) {
            map(x, y);
            path.moveTo(pt[0], pt[1]);
            curX = startX = pt[0];
            curY = startY = pt[1];
            has = true;
        }

        private void lineTo(float x, float y) {
            if (!has) {
                moveTo(x, y);
                return;
            }
            map(x, y);
            path.lineTo(pt[0], pt[1]);
            curX = pt[0];
            curY = pt[1];
        }

        private void cubic(float x1, float y1, float x2, float y2, float x, float y) {
            map(x1, y1, x2, y2, x, y);
            path.cubicTo(pt[0], pt[1], pt[2], pt[3], pt[4], pt[5]);
            curX = pt[4];
            curY = pt[5];
        }

        private void close() {
            if (!has) return;
            path.close();
            curX = startX;
            curY = startY;
        }

        /** Adds cubic curves along an ellipse around (cx, cy) from angle a0 by sweep, from where the path is. */
        private void ellipseArc(double cx, double cy, double rx, double ry, double a0, double sweep) {
            int n = Math.max((int) Math.ceil(Math.abs(sweep) / (Math.PI / 2) - 1e-6), 1);
            double step = sweep / n, k = 4.0 / 3 * Math.tan(step / 4), a = a0;
            for (int i = 0; i < n; i++) {
                double s0 = Math.sin(a), c0 = Math.cos(a), s1 = Math.sin(a + step), c1 = Math.cos(a + step);
                cubic((float) (cx + rx * (c0 - k * s0)), (float) (cy + ry * (s0 + k * c0)),
                        (float) (cx + rx * (c1 + k * s1)), (float) (cy + ry * (s1 - k * c1)),
                        (float) (cx + rx * c1), (float) (cy + ry * s1));
                a += step;
            }
        }

        /**
         * Adds an arc as the web's canvas does: a line to its start, then
         * clockwise from a0 to a1 unless ccw, a full circle at most.
         */
        private void arc(float x, float y, float r, float a0, float a1, boolean ccw) {
            r = Math.max(r, 0);
            double tau = 2 * Math.PI;
            double d = ccw ? a0 - a1 : a1 - a0;
            double sweep = d >= tau ? tau : ((d % tau) + tau) % tau;
            if (ccw) sweep = -sweep;
            float sx = (float) (x + r * Math.cos(a0)), sy = (float) (y + r * Math.sin(a0));
            if (has) {
                lineTo(sx, sy);
            } else {
                moveTo(sx, sy);
            }
            if (sweep != 0) ellipseArc(x, y, r, r, a0, sweep);
        }

        /** Sets the paint to the color, or the gradient over it in to's space, under the global alpha. */
        private boolean paintWith(int color, Gradient g, Matrix to) {
            paint.reset();
            paint.setAntiAlias(true);
            if (g != null) {
                Shader s = g.shader(to);
                if (s == null) return false;
                // A shader takes only the paint's alpha.
                paint.setColor(fade(Color.BLACK));
                paint.setShader(s);
            } else {
                paint.setColor(fade(color));
            }
            return true;
        }

        /**
         * Draws with the paint as draw does, in the shadow when there is
         * one: its offset and blur in the canvas's points, whatever the
         * transform inTransform says the drawing is under. An operator
         * that clears outside the shape draws into a layer of the whole
         * canvas, which it mixes by.
         */
        private void draw(Runnable draw, boolean inTransform) {
            if (Color.alpha(state.shadowColor) != 0 && (state.shadowX != 0 || state.shadowY != 0 || state.shadowBlur != 0)) {
                float dx = state.shadowX, dy = state.shadowY, blur = state.shadowBlur;
                if (inTransform) {
                    Matrix inv = new Matrix();
                    if (state.ctm.invert(inv)) {
                        float[] v = {dx, dy};
                        inv.mapVectors(v);
                        dx = v[0];
                        dy = v[1];
                        blur = inv.mapRadius(blur);
                    }
                }
                // Skia's sigma is radius * 0.57735 + 0.5; the canvas's is
                // half the blur. A radius of 0 removes the shadow.
                float radius = Math.max((blur / 2 - 0.5f) / 0.57735f, 0.01f);
                paint.setShadowLayer(radius, dx, dy, state.shadowColor);
            }
            int c = state.composite;
            if (c == SOURCE_OVER) {
                draw.run();
                return;
            }
            Paint mix = new Paint();
            if (Build.VERSION.SDK_INT >= 29) {
                mix.setBlendMode(blendMode(c));
            } else if (c < PORTER_DUFF.length) {
                mix.setXfermode(new PorterDuffXfermode(PORTER_DUFF[c]));
            }
            boolean unbounded = c == SOURCE_IN || c == SOURCE_OUT || c == DESTINATION_IN || c == DESTINATION_ATOP
                    || c == COPY;
            if (unbounded) {
                int layer = canvas.saveLayer(null, mix);
                draw.run();
                canvas.restoreToCount(layer);
                return;
            }
            if (Build.VERSION.SDK_INT >= 29) {
                paint.setBlendMode(blendMode(c));
            } else if (c < PORTER_DUFF.length) {
                paint.setXfermode(new PorterDuffXfermode(PORTER_DUFF[c]));
            }
            draw.run();
        }

        private static BlendMode blendMode(int c) {
            switch (c) {
                case SOURCE_IN: return BlendMode.SRC_IN;
                case SOURCE_OUT: return BlendMode.SRC_OUT;
                case SOURCE_ATOP: return BlendMode.SRC_ATOP;
                case DESTINATION_OVER: return BlendMode.DST_OVER;
                case DESTINATION_IN: return BlendMode.DST_IN;
                case DESTINATION_OUT: return BlendMode.DST_OUT;
                case DESTINATION_ATOP: return BlendMode.DST_ATOP;
                case LIGHTER: return BlendMode.PLUS;
                case COPY: return BlendMode.SRC;
                case XOR: return BlendMode.XOR;
                case MULTIPLY: return BlendMode.MULTIPLY;
                case MULTIPLY + 1: return BlendMode.SCREEN;
                case MULTIPLY + 2: return BlendMode.OVERLAY;
                case MULTIPLY + 3: return BlendMode.DARKEN;
                case MULTIPLY + 4: return BlendMode.LIGHTEN;
                case MULTIPLY + 5: return BlendMode.COLOR_DODGE;
                case MULTIPLY + 6: return BlendMode.COLOR_BURN;
                case MULTIPLY + 7: return BlendMode.HARD_LIGHT;
                case MULTIPLY + 8: return BlendMode.SOFT_LIGHT;
                case MULTIPLY + 9: return BlendMode.DIFFERENCE;
                case MULTIPLY + 10: return BlendMode.EXCLUSION;
                case MULTIPLY + 11: return BlendMode.HUE;
                case MULTIPLY + 12: return BlendMode.SATURATION;
                case MULTIPLY + 13: return BlendMode.COLOR;
                case MULTIPLY + 14: return BlendMode.LUMINOSITY;
                default: return BlendMode.SRC_OVER;
            }
        }

        /** Fills the path, built in the canvas's points, in which the gradient is mapped by the transform. */
        private void fill() {
            if (!paintWith(state.fill, state.fillGradient, state.ctm)) return;
            paint.setStyle(Paint.Style.FILL);
            draw(() -> canvas.drawPath(path, paint), false);
        }

        /** Strokes the path in the transform, as the web's canvas does: its width in the drawing's units. */
        private void stroke() {
            Matrix inv = new Matrix();
            if (!state.ctm.invert(inv)) return;
            if (!paintWith(state.stroke, state.strokeGradient, new Matrix())) return;
            strokeStyle();
            Path local = new Path();
            path.transform(inv, local);
            canvas.save();
            canvas.concat(state.ctm);
            draw(() -> canvas.drawPath(local, paint), true);
            canvas.restore();
        }

        private void strokeStyle() {
            paint.setStyle(Paint.Style.STROKE);
            paint.setStrokeWidth(state.lineWidth);
            paint.setStrokeCap(state.cap);
            paint.setStrokeJoin(state.join);
            paint.setStrokeMiter(state.miter);
            if (state.dash != null) paint.setPathEffect(new DashPathEffect(state.dash, state.dashOffset));
        }

        private void text(ByteBuffer b, boolean stroke) {
            String text = string(b);
            float x = b.getFloat(), y = b.getFloat();
            String family = string(b);
            float size = b.getFloat();
            int weight = b.getShort() & 0xffff;
            boolean italic = b.get() != 0;
            String features = Text.features(b.get() & 0xff);
            if (stroke) {
                if (!paintWith(state.stroke, state.strokeGradient, new Matrix())) return;
                strokeStyle();
            } else {
                if (!paintWith(state.fill, state.fillGradient, new Matrix())) return;
                paint.setStyle(Paint.Style.FILL);
            }
            paint.setTypeface(Text.typeface(family, weight, italic));
            paint.setTextSize(size);
            paint.setFontFeatureSettings(features);
            canvas.save();
            canvas.concat(state.ctm);
            draw(() -> canvas.drawText(text, x, y, paint), true);
            canvas.restore();
        }

        private void image(ByteBuffer b) {
            String key = string(b);
            b.getFloat(); // the density its name gives, which the picture carries
            float x = b.getFloat(), y = b.getFloat(), w = b.getFloat(), h = b.getFloat();
            float sx = b.getFloat(), sy = b.getFloat(), sw = b.getFloat(), sh = b.getFloat();
            Picture p = pictures.get(key);
            if (p == null) return;
            float natW = p.bitmap.getWidth() / p.scale, natH = p.bitmap.getHeight() / p.scale;
            if (w == 0 && h == 0) {
                w = natW;
                h = natH;
            }
            Rect src = null;
            if (sw != 0 || sh != 0) {
                src = new Rect(Math.round(sx * p.scale), Math.round(sy * p.scale), Math.round((sx + sw) * p.scale),
                        Math.round((sy + sh) * p.scale));
            }
            paint.reset();
            paint.setAntiAlias(true);
            paint.setFilterBitmap(true);
            paint.setAlpha(Math.round(255 * state.alpha));
            RectF dst = new RectF(x, y, x + w, y + h);
            Rect from = src;
            canvas.save();
            canvas.concat(state.ctm);
            draw(() -> canvas.drawBitmap(p.bitmap, from, dst, paint), true);
            canvas.restore();
        }

        /** The color with the global alpha applied. */
        private int fade(int color) {
            int alpha = Math.round(Color.alpha(color) * state.alpha);
            return (color & 0x00ffffff) | (alpha << 24);
        }

        private static String string(ByteBuffer b) {
            byte[] bytes = new byte[b.getInt()];
            b.get(bytes);
            return new String(bytes, StandardCharsets.UTF_8);
        }
    }
}
