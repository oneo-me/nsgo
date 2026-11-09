package dev.oneo.nsgo;

import java.io.DataInputStream;
import java.io.IOException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.function.BiConsumer;

/**
 * The animation graph on the UI thread: a faithful port of core/animator.go,
 * same formulas and semantics, so a run plays without a round trip to Go per
 * frame. NsgoView feeds it each ItemAnimation as android.go's
 * writeAnimation encodes it, steps it on Choreographer frames while it runs
 * and draws what changed reports over each view's style. Finished runs and
 * reads go back to Go as EventAnimation. Times are seconds, as in Go.
 */
final class Animator {
    // Op kinds, as core.AnimationOpKind.
    static final int CREATE_VALUE = 1;
    static final int CREATE_INTERPOLATE = 2;
    static final int CREATE_MATH = 3;
    static final int CREATE_DIFF_CLAMP = 4;
    static final int SET = 5;
    static final int SET_OFFSET = 6;
    static final int FLATTEN_OFFSET = 7;
    static final int EXTRACT_OFFSET = 8;
    static final int STOP_VALUE = 9;
    static final int READ = 10;
    static final int START = 11;
    static final int STOP = 12;
    static final int BIND = 13;
    static final int BIND_PAN = 14;
    static final int BIND_SCROLL = 15;
    static final int RELEASE = 16;
    static final int BIND_PINCH = 17;
    static final int BIND_ROTATION = 18;
    static final int RESET = 19;
    static final int FORGET_RUN = 20;
    static final int BIND_PULL = 21;
    static final int CREATE_RUBBER_BAND = 22;

    // Bound properties, as core.AnimatedProp.
    static final int PROP_OPACITY = 0;
    static final int PROP_TRANSLATE_X = 1;
    static final int PROP_TRANSLATE_Y = 2;
    static final int PROP_SCALE = 3;
    static final int PROP_SCALE_X = 4;
    static final int PROP_SCALE_Y = 5;
    static final int PROP_ROTATE = 6;
    static final int PROP_BACKGROUND = 7;
    static final int PROP_BORDER_COLOR = 8;
    static final int PROP_SHADOW_COLOR = 9;
    static final int PROP_SHADOW_RADIUS = 10;
    static final int PROP_SHADOW_OPACITY = 11;
    static final int PROP_TEXT_COLOR = 12;
    static final int PROP_ROTATE_X = 13;
    static final int PROP_ROTATE_Y = 14;
    static final int PROP_COUNT = 15;

    // Math ops, as core.MathOp.
    static final int MATH_ADD = 1;
    static final int MATH_SUB = 2;
    static final int MATH_MUL = 3;
    static final int MATH_DIV = 4;
    static final int MATH_MOD = 5;

    // Extrapolation, as core.Extrapolate; 0 is unset.
    static final int EXTRAPOLATE_CLAMP = 1;
    static final int EXTRAPOLATE_EXTEND = 2;
    static final int EXTRAPOLATE_IDENTITY = 3;

    // Easing kinds and modes, as core.EasingKind and core.EaseMode.
    static final int EASING_BEZIER = 0;
    static final int EASING_QUAD = 1;
    static final int EASING_CUBIC = 2;
    static final int EASING_POLY = 3;
    static final int EASING_SIN = 4;
    static final int EASING_CIRCLE = 5;
    static final int EASING_EXP = 6;
    static final int EASING_ELASTIC = 7;
    static final int EASING_BACK = 8;
    static final int EASING_BOUNCE = 9;
    static final int EASING_STEPS = 10;
    // Step positions, as core.StepPosition.
    static final int JUMP_END = 0;
    static final int JUMP_START = 1;
    static final int JUMP_NONE = 2;
    static final int JUMP_BOTH = 3;
    static final int EASE_IN = 0;
    static final int EASE_OUT = 1;
    static final int EASE_IN_OUT = 2;

    // Animation kinds, as core.AnimationKind.
    static final int TIMING = 1;
    static final int SPRING = 2;
    static final int DECAY = 3;
    static final int SEQUENCE = 4;
    static final int PARALLEL = 5;
    static final int STAGGER = 6;
    static final int DELAY = 7;
    static final int LOOP = 8;

    /** Receives finished runs and reads, as EventAnimation's Count, Value and Finished. */
    interface Reporter {
        void report(int id, float value, boolean finished);
    }

    private final Map<Integer, ANode> nodes = new HashMap<>();
    private final Map<Integer, Run> runs = new LinkedHashMap<>();
    private final Map<Integer, AView> views = new HashMap<>();
    private final Map<Integer, int[]> pans = new HashMap<>();
    private final Map<Integer, int[]> scrolls = new HashMap<>();
    /** The values pinches and rotations on a view move, and where a gesture's values were as it began: pan x, y, pinch, rotation. */
    private final Map<Integer, Integer> pinches = new HashMap<>(), rotations = new HashMap<>();
    /** The values scroll views' pulls past their tops set. */
    private final Map<Integer, Integer> pulls = new HashMap<>();
    private final Map<Integer, double[]> starts = new HashMap<>();
    private final LinkedHashSet<Integer> changed = new LinkedHashSet<>();
    /** Canvas views' bound drawing arguments, and whether values changed since changedCanvases looked. */
    private final Map<Integer, ACanvas> canvases = new HashMap<>();
    private boolean canvasesStale;
    /** Where the values each run moves were as it started, for a reset: node, value pairs. */
    private final Map<Integer, double[]> runStarts = new HashMap<>();
    /** The snapping diff clamps easing to a bound. */
    private final LinkedHashSet<ANode> settling = new LinkedHashSet<>();
    /** How long a snapping diff clamp takes to its bound, in seconds. */
    private static final double SETTLE = 0.2;
    private final Reporter report;
    private long gen; // bumped when any value changes
    private double now; // seconds of the last step

    Animator(Reporter report) {
        this.report = report;
    }

    /** A node of the graph. */
    private static final class ANode {
        int kind; // the op that made it
        double value, offset;
        double vel; // units per second, from the driver moving it
        Leaf driver; // the animation moving a value
        Op op;
        // The node's output in generation gen.
        long gen;
        double num;
        int color;
        // Diff-clamp state, and a snapping one's settling.
        double last, lastIn;
        boolean primed;
        double settleFrom, settleTo, settleStarted;
    }

    /** A canvas's bound nodes and the values changedCanvases last reported. */
    static final class ACanvas {
        final int[] nodes;
        final double[] nums;
        final int[] colors;
        boolean sent;

        ACanvas(int[] nodes) {
            this.nodes = nodes;
            nums = new double[nodes.length];
            colors = new int[nodes.length];
        }
    }

    private static final class AView {
        final int[] nodes = new int[PROP_COUNT];
        Values sent = new Values();

        boolean empty() {
            for (int n : nodes) if (n != 0) return false;
            return true;
        }
    }

    /** An op's fields, as core.AnimationOp. */
    static final class Op {
        int kind, node, input, math, id, view, prop, x, y;
        double value, min, max;
        boolean snap;
        int aNode, bNode;
        double aConst, bConst;
        Interpolation interpolation;
        Spec animation;
        /** A run's AtFrameRate, 0 for every frame. */
        double frameRate;
    }

    /**
     * The bound properties of a view as last evaluated, as
     * core.AnimatedValues: translations and the shadow radius in points,
     * rotation in degrees and colors as core's RGBA.
     */
    static final class Values {
        int set;
        float opacity, translateX, translateY, scale, scaleX, scaleY, rotate, shadowRadius;
        float shadowOpacity, rotateX, rotateY;
        int background, borderColor, shadowColor, textColor;

        boolean has(int prop) {
            return (set & (1 << prop)) != 0;
        }

        @Override
        public boolean equals(Object o) {
            if (!(o instanceof Values)) return false;
            Values v = (Values) o;
            return set == v.set && opacity == v.opacity && translateX == v.translateX
                    && translateY == v.translateY && scale == v.scale && scaleX == v.scaleX
                    && scaleY == v.scaleY && rotate == v.rotate && shadowRadius == v.shadowRadius
                    && background == v.background && borderColor == v.borderColor && shadowColor == v.shadowColor
                    && shadowOpacity == v.shadowOpacity && rotateX == v.rotateX && rotateY == v.rotateY
                    && textColor == v.textColor;
        }

        @Override
        public int hashCode() {
            return set;
        }
    }

    /** An easing, as core.Easing. */
    static final class Easing {
        int kind, mode, steps;
        float x1, y1, x2, y2;
        double param;

        static Easing read(DataInputStream in) throws IOException {
            Easing e = new Easing();
            e.kind = in.readUnsignedByte();
            e.mode = in.readUnsignedByte();
            e.x1 = in.readFloat();
            e.y1 = in.readFloat();
            e.x2 = in.readFloat();
            e.y2 = in.readFloat();
            e.param = in.readDouble();
            e.steps = in.readUnsignedByte();
            return e;
        }

        /** The eased progress at time fraction x from 0 to 1. */
        double at(double x) {
            if (kind == EASING_BEZIER) return bezier(x1, y1, x2, y2, x);
            if (kind == EASING_STEPS) return steps(x);
            switch (mode) {
                case EASE_OUT:
                    return 1 - in(1 - x);
                case EASE_IN_OUT:
                    if (x < 0.5) return in(x * 2) / 2;
                    return 1 - in((1 - x) * 2) / 2;
                default:
                    return in(x);
            }
        }

        /** CSS's steps(): param steps jumping at the steps' positions. */
        private double steps(double x) {
            double n = Math.max(Math.round(param), 1);
            double step = Math.floor(x * n);
            if (steps == JUMP_START || steps == JUMP_BOTH) step++;
            double jumps = n;
            if (steps == JUMP_BOTH) jumps++;
            else if (steps == JUMP_NONE) jumps = Math.max(n - 1, 1);
            if (x >= 0 && step < 0) step = 0;
            if (x <= 1 && step > jumps) step = jumps;
            return step / jumps;
        }

        /** The curve's "in" form, React Native's Easing functions. */
        private double in(double t) {
            switch (kind) {
                case EASING_QUAD:
                    return t * t;
                case EASING_CUBIC:
                    return t * t * t;
                case EASING_POLY: {
                    double n = param == 0 ? 2 : param;
                    return Math.pow(t, n);
                }
                case EASING_SIN:
                    return 1 - Math.cos(t * Math.PI / 2);
                case EASING_CIRCLE:
                    return 1 - Math.sqrt(Math.max(1 - t * t, 0));
                case EASING_EXP:
                    if (t <= 0) return 0;
                    return Math.pow(2, 10 * (t - 1));
                case EASING_ELASTIC: {
                    double b = param == 0 ? 1 : param;
                    double p = b * Math.PI;
                    return 1 - Math.pow(Math.cos(t * Math.PI / 2), 3) * Math.cos(t * p);
                }
                case EASING_BACK: {
                    double s = param == 0 ? 1.70158 : param;
                    return t * t * ((s + 1) * t - s);
                }
                case EASING_BOUNCE:
                    return bounce(t);
                default:
                    return t;
            }
        }

        private static double bounce(double t) {
            if (t < 1 / 2.75) return 7.5625 * t * t;
            if (t < 2 / 2.75) {
                t -= 1.5 / 2.75;
                return 7.5625 * t * t + 0.75;
            }
            if (t < 2.5 / 2.75) {
                t -= 2.25 / 2.75;
                return 7.5625 * t * t + 0.9375;
            }
            t -= 2.625 / 2.75;
            return 7.5625 * t * t + 0.984375;
        }
    }

    /** core.Curve.At: a cubic Bézier's progress at time fraction x; linear when all are 0. */
    static double bezier(float cx1, float cy1, float cx2, float cy2, double x) {
        if (cx1 == 0 && cy1 == 0 && cx2 == 0 && cy2 == 0 || x <= 0 || x >= 1) {
            return Math.min(Math.max(x, 0), 1);
        }
        double x1 = cx1, y1 = cy1, x2 = cx2, y2 = cy2;
        // Find t with bez(t) == x on the x axis by bisection, then map to y.
        double lo = 0, hi = 1;
        for (int i = 0; i < 40; i++) {
            double mid = (lo + hi) / 2;
            if (bez(mid, x1, x2) < x) lo = mid;
            else hi = mid;
        }
        return bez((lo + hi) / 2, y1, y2);
    }

    private static double bez(double t, double p1, double p2) {
        double u = 1 - t;
        return 3 * u * u * t * p1 + 3 * u * t * t * p2 + t * t * t;
    }

    /** An interpolation, as core.Interpolation. */
    static final class Interpolation {
        double[] input, output;
        int[] colors;
        int extrapolate, left, right;
        Easing easing;
        /** The color of the last at, when it has colors. */
        int color;

        static Interpolation read(DataInputStream in) throws IOException {
            Interpolation r = new Interpolation();
            r.input = new double[in.readInt()];
            for (int i = 0; i < r.input.length; i++) r.input[i] = in.readDouble();
            r.output = new double[in.readInt()];
            for (int i = 0; i < r.output.length; i++) r.output[i] = in.readDouble();
            r.colors = new int[in.readInt()];
            for (int i = 0; i < r.colors.length; i++) r.colors[i] = in.readInt();
            r.extrapolate = in.readUnsignedByte();
            r.left = in.readUnsignedByte();
            r.right = in.readUnsignedByte();
            r.easing = Easing.read(in);
            return r;
        }

        /** Maps x: a number, and in color a color when it has colors. */
        double at(double x) {
            color = 0;
            double[] in = input;
            int k = in.length;
            if (k == 0) return x;
            if (k == 1) return output(0, 0, 0);
            int i = 0;
            while (i < k - 2 && x > in[i + 1]) i++;
            double lo = in[i], hi = in[i + 1];
            double t = 0;
            if (hi != lo) t = (x - lo) / (hi - lo);
            if (x < in[0]) {
                switch (left != 0 ? left : extrapolate) {
                    case EXTRAPOLATE_IDENTITY:
                        return x;
                    case 0:
                    case EXTRAPOLATE_CLAMP:
                        t = 0;
                        break;
                    default:
                        break;
                }
            } else if (x > in[k - 1]) {
                switch (right != 0 ? right : extrapolate) {
                    case EXTRAPOLATE_IDENTITY:
                        return x;
                    case 0:
                    case EXTRAPOLATE_CLAMP:
                        t = 1;
                        break;
                    default:
                        break;
                }
            }
            if (t >= 0 && t <= 1) t = easing.at(t);
            return output(i, i + 1, t);
        }

        private double output(int i, int j, double t) {
            if (colors.length > j) {
                color = lerpColor(colors[i], colors[j], t);
                return t;
            }
            if (output.length > j) return output[i] + (output[j] - output[i]) * t;
            return 0;
        }
    }

    static int lerpColor(int a, int b, double t) {
        t = Math.min(Math.max(t, 0), 1);
        int out = 0;
        for (int shift = 0; shift < 32; shift += 8) {
            double x = (a >>> shift) & 0xff, y = (b >>> shift) & 0xff;
            out |= ((int) Math.round(x + (y - x) * t)) << shift;
        }
        return out;
    }

    /** An animation, as core.Animation; durations in nanoseconds. */
    static final class Spec {
        int kind, value;
        double to;
        long duration, timingDelay;
        Easing easing;
        double stiffness, damping, mass, springVelocity;
        double decayVelocity, deceleration, min, max, bounce;
        boolean clamp;
        Spec[] steps;
        long delay;
        int iterations;
        boolean noReset;

        static Spec read(DataInputStream in) throws IOException {
            Spec s = new Spec();
            s.kind = in.readUnsignedByte();
            s.value = in.readInt();
            s.to = in.readDouble();
            s.duration = in.readLong();
            s.timingDelay = in.readLong();
            s.easing = Easing.read(in);
            s.stiffness = in.readDouble();
            s.damping = in.readDouble();
            s.mass = in.readDouble();
            s.springVelocity = in.readDouble();
            s.decayVelocity = in.readDouble();
            s.deceleration = in.readDouble();
            s.clamp = in.readUnsignedByte() != 0;
            s.min = in.readDouble();
            s.max = in.readDouble();
            s.bounce = in.readDouble();
            s.delay = in.readLong();
            int iterations = in.readInt();
            // A loop's NoReset is the word's top bit.
            s.noReset = iterations < 0;
            s.iterations = iterations & 0x7fffffff;
            s.steps = new Spec[in.readInt()];
            for (int i = 0; i < s.steps.length; i++) s.steps[i] = read(in);
            return s;
        }
    }

    /** time.Duration.Seconds of nanoseconds d. */
    static double seconds(long d) {
        return (double) (d / 1_000_000_000L) + (double) (d % 1_000_000_000L) / 1e9;
    }

    /** Reads one op as android.go's writeAnimation writes it. */
    static Op readOp(DataInputStream in) throws IOException {
        Op o = new Op();
        o.kind = in.readUnsignedByte();
        o.node = in.readInt();
        switch (o.kind) {
            case CREATE_VALUE:
            case SET:
            case SET_OFFSET:
                o.value = in.readDouble();
                break;
            case CREATE_INTERPOLATE:
                o.input = in.readInt();
                o.interpolation = Interpolation.read(in);
                break;
            case CREATE_MATH:
                o.math = in.readUnsignedByte();
                o.aNode = in.readInt();
                o.aConst = in.readDouble();
                o.bNode = in.readInt();
                o.bConst = in.readDouble();
                break;
            case CREATE_DIFF_CLAMP:
                o.input = in.readInt();
                o.min = in.readDouble();
                o.max = in.readDouble();
                o.snap = in.readUnsignedByte() != 0;
                break;
            case CREATE_RUBBER_BAND:
                o.input = in.readInt();
                o.min = in.readDouble();
                o.max = in.readDouble();
                o.value = in.readDouble();
                break;
            case READ:
            case STOP:
            case RESET:
            case FORGET_RUN:
                o.id = in.readInt();
                break;
            case START:
                o.id = in.readInt();
                if (in.readUnsignedByte() != 0) {
                    o.animation = Spec.read(in);
                    o.frameRate = in.readDouble();
                }
                break;
            case BIND:
                o.view = in.readInt();
                o.prop = in.readUnsignedByte();
                break;
            case BIND_PAN:
            case BIND_SCROLL:
                o.view = in.readInt();
                o.x = in.readInt();
                o.y = in.readInt();
                break;
            case BIND_PINCH:
            case BIND_ROTATION:
            case BIND_PULL:
                o.view = in.readInt();
                o.node = in.readInt();
                break;
            default:
                break;
        }
        return o;
    }

    /** Applies one change to the graph. */
    void apply(Op o) {
        switch (o.kind) {
            case CREATE_VALUE: {
                ANode n = new ANode();
                n.kind = o.kind;
                n.value = o.value;
                nodes.put(o.node, n);
                touch();
                break;
            }
            case CREATE_INTERPOLATE:
            case CREATE_MATH:
            case CREATE_DIFF_CLAMP:
            case CREATE_RUBBER_BAND: {
                ANode n = new ANode();
                n.kind = o.kind;
                n.op = o;
                nodes.put(o.node, n);
                touch();
                break;
            }
            case SET:
            case SET_OFFSET:
            case FLATTEN_OFFSET:
            case EXTRACT_OFFSET:
            case STOP_VALUE: {
                ANode n = value(o.node);
                if (n == null) return;
                if (o.kind != SET_OFFSET) interrupt(n);
                switch (o.kind) {
                    case SET:
                        n.value = o.value;
                        break;
                    case SET_OFFSET:
                        n.offset = o.value;
                        break;
                    case FLATTEN_OFFSET:
                        n.value += n.offset;
                        n.offset = 0;
                        break;
                    case EXTRACT_OFFSET:
                        n.offset += n.value;
                        n.value = 0;
                        break;
                    default:
                        break;
                }
                touch();
                break;
            }
            case READ: {
                double v = 0;
                ANode n = value(o.node);
                if (n != null) v = n.value + n.offset;
                report.report(o.id, (float) v, true);
                break;
            }
            case START: {
                if (o.animation == null) return;
                Run r = new Run(o.id, build(o.animation), o.frameRate);
                List<Integer> moved = new ArrayList<>();
                r.root.values(moved);
                double[] starts = new double[moved.size() * 2];
                int k = 0;
                for (int id : moved) {
                    ANode n = value(id);
                    if (n == null) continue;
                    starts[k++] = id;
                    starts[k++] = n.value;
                }
                runStarts.put(o.id, java.util.Arrays.copyOf(starts, k));
                runs.put(o.id, r);
                r.root.bind(r);
                r.root.start(this, -1);
                break;
            }
            case STOP: {
                Run r = runs.get(o.id);
                if (r != null) end(r, false);
                break;
            }
            case RESET: {
                Run r = runs.get(o.id);
                if (r != null) end(r, false);
                double[] starts = runStarts.get(o.id);
                if (starts != null) {
                    for (int i = 0; i + 1 < starts.length; i += 2) {
                        ANode n = value((int) starts[i]);
                        if (n == null) continue;
                        interrupt(n);
                        n.value = starts[i + 1];
                        n.vel = 0;
                    }
                }
                touch();
                break;
            }
            case FORGET_RUN:
                runStarts.remove(o.id);
                break;
            case BIND: {
                AView v = views.get(o.view);
                if (v == null) {
                    if (o.node == 0) return;
                    v = new AView();
                    views.put(o.view, v);
                }
                v.nodes[o.prop] = o.node;
                changed.add(o.view);
                if (v.empty()) views.remove(o.view);
                break;
            }
            case BIND_PAN:
                pans.put(o.view, new int[] {o.x, o.y});
                break;
            case BIND_SCROLL:
                scrolls.put(o.view, new int[] {o.x, o.y});
                break;
            case BIND_PINCH:
                pinches.put(o.view, o.node);
                break;
            case BIND_ROTATION:
                rotations.put(o.view, o.node);
                break;
            case BIND_PULL:
                pulls.put(o.view, o.node);
                break;
            case RELEASE: {
                ANode n = nodes.get(o.node);
                if (n != null) {
                    interrupt(n);
                    settling.remove(n);
                    nodes.remove(o.node);
                    touch();
                }
                break;
            }
            default:
                break;
        }
    }

    /** Notes that values changed, so bound views are evaluated again. */
    private void touch() {
        gen++;
        changed.addAll(views.keySet());
        canvasesStale = true;
    }

    /** Has nodes drive arguments of canvas view's drawing, replacing what it bound. */
    void bindCanvas(int view, int[] nodes) {
        if (nodes.length == 0) {
            canvases.remove(view);
            return;
        }
        canvases.put(view, new ACanvas(nodes));
        canvasesStale = true;
    }

    /** Calls fn with each canvas whose bound values changed since the last call, its nodes' numbers and colors. */
    void changedCanvases(BiConsumer<Integer, ACanvas> fn) {
        if (!canvasesStale) return;
        canvasesStale = false;
        for (Map.Entry<Integer, ACanvas> e : canvases.entrySet()) {
            ACanvas c = e.getValue();
            boolean changed = !c.sent;
            c.sent = true;
            for (int i = 0; i < c.nodes.length; i++) {
                ANode n = nodes.get(c.nodes[i]);
                double num = n != null ? eval(n) : 0;
                int color = n != null ? n.color : 0;
                if (num != c.nums[i] || color != c.colors[i]) {
                    c.nums[i] = num;
                    c.colors[i] = color;
                    changed = true;
                }
            }
            if (changed) fn.accept(e.getKey(), c);
        }
    }

    private ANode value(int id) {
        ANode n = nodes.get(id);
        return n != null && n.kind == CREATE_VALUE ? n : null;
    }

    /** Stops the animation moving n, ending its run unfinished. */
    private void interrupt(ANode n) {
        Leaf l = n.driver;
        if (l != null) {
            n.driver = null;
            Run r = l.run;
            if (r != null && runs.get(r.id) == r) end(r, false);
        }
    }

    /** Finishes run r, reporting whether it ran to its end. */
    private void end(Run r, boolean finished) {
        runs.remove(r.id);
        r.root.stop(this);
        report.report(r.id, 0, finished);
    }

    /** Whether an animation runs or a snapping diff clamp settles, so the view keeps its frame callback. */
    boolean running() {
        return !runs.isEmpty() || !settling.isEmpty();
    }

    /** Advances the running animations to now, monotonic nanoseconds. */
    void step(long nanos) {
        now = seconds(nanos);
        for (Run r : new ArrayList<>(runs.values())) {
            if (runs.get(r.id) != r) continue;
            // A run with a frame rate moves only when its next frame is due.
            if (r.rate > 0) {
                if (r.moved && now - r.last < 1 / r.rate - FRAME_SLACK) continue;
                r.last = now;
                r.moved = true;
            }
            if (r.root.step(this, now)) end(r, true);
        }
        for (ANode n : new ArrayList<>(settling)) {
            // Settling starts with the first frame after the rest.
            if (n.settleStarted < 0) n.settleStarted = now;
            double p = (now - n.settleStarted) / SETTLE;
            if (p >= 1) {
                n.last = n.settleTo;
                settling.remove(n);
            } else {
                double e = 1 - Math.pow(1 - Math.max(p, 0), 3);
                n.last = n.settleFrom + (n.settleTo - n.settleFrom) * e;
            }
            touch();
        }
    }

    /**
     * Starts the snapping diff clamps that follow scroll view view's values
     * toward their nearer bound, as core.Animator.Settle does: the view
     * calls it where a scroll comes to rest.
     */
    void settle(int view) {
        int[] ids = scrolls.get(view);
        if (ids == null) return;
        for (ANode n : nodes.values()) {
            if (n.kind != CREATE_DIFF_CLAMP || !n.op.snap || !dependsOn(n.op.input, ids)) continue;
            eval(n);
            double lo = n.op.min, hi = n.op.max, to = lo;
            if (n.lastIn >= hi && n.last - lo >= hi - n.last) to = hi;
            if (n.last == to) continue;
            n.settleFrom = n.last;
            n.settleTo = to;
            n.settleStarted = -1;
            settling.add(n);
        }
    }

    /** Whether node id reads one of the values ids. */
    private boolean dependsOn(int id, int[] ids) {
        if (id == 0) return false;
        if (id == ids[0] || id == ids[1]) return true;
        ANode n = nodes.get(id);
        if (n == null) return false;
        switch (n.kind) {
            case CREATE_INTERPOLATE:
            case CREATE_DIFF_CLAMP:
            case CREATE_RUBBER_BAND:
                return dependsOn(n.op.input, ids);
            case CREATE_MATH:
                return dependsOn(n.op.aNode, ids) || dependsOn(n.op.bNode, ids);
            default:
                return false;
        }
    }

    /** Node id's output now, or NaN without one. */
    double valueOf(int id) {
        ANode n = nodes.get(id);
        return n == null ? Double.NaN : eval(n);
    }

    /** Calls fn with each bound view whose values may have changed since the last call, and their values. */
    void changed(BiConsumer<Integer, Values> fn) {
        List<Integer> ids = new ArrayList<>(changed);
        changed.clear();
        for (int id : ids) {
            AView vw = views.get(id);
            Values vals = new Values();
            if (vw != null) {
                vals = evalView(vw);
                if (vals.equals(vw.sent)) continue;
                vw.sent = vals;
            }
            fn.accept(id, vals);
        }
    }

    /** View's bound values, or null when it has none. */
    Values values(int view) {
        AView vw = views.get(view);
        return vw == null ? null : evalView(vw);
    }

    private Values evalView(AView vw) {
        Values v = new Values();
        for (int p = 0; p < PROP_COUNT; p++) {
            int id = vw.nodes[p];
            if (id == 0) continue;
            ANode n = nodes.get(id);
            if (n == null) continue;
            double num = eval(n);
            int col = n.color;
            v.set |= 1 << p;
            switch (p) {
                case PROP_OPACITY: v.opacity = (float) Math.min(Math.max(num, 0), 1); break;
                case PROP_TRANSLATE_X: v.translateX = (float) num; break;
                case PROP_TRANSLATE_Y: v.translateY = (float) num; break;
                case PROP_SCALE: v.scale = (float) num; break;
                case PROP_SCALE_X: v.scaleX = (float) num; break;
                case PROP_SCALE_Y: v.scaleY = (float) num; break;
                case PROP_ROTATE: v.rotate = (float) num; break;
                case PROP_BACKGROUND: v.background = col; break;
                case PROP_BORDER_COLOR: v.borderColor = col; break;
                case PROP_SHADOW_COLOR: v.shadowColor = col; break;
                case PROP_SHADOW_RADIUS: v.shadowRadius = (float) Math.max(num, 0); break;
                case PROP_SHADOW_OPACITY: v.shadowOpacity = (float) Math.min(Math.max(num, 0), 1); break;
                case PROP_TEXT_COLOR: v.textColor = col; break;
                case PROP_ROTATE_X: v.rotateX = (float) num; break;
                case PROP_ROTATE_Y: v.rotateY = (float) num; break;
                default: break;
            }
        }
        return v;
    }

    /**
     * Moves the values bound to a gesture of kind on view, as
     * core.Animator.Gesture does: a pan's by its translation (x, y), a
     * pinch's scaled by value, a rotation's turned by value, from where
     * they were as it began. Returns null when none is bound, else a pan's
     * values as it begins and ends ({} otherwise).
     */
    float[] gesture(int kind, int view, int phase, float x, float y, float value) {
        double[] s = starts.computeIfAbsent(view, k -> new double[4]);
        boolean began = phase == Protocol.GESTURE_BEGAN;
        float[] out = {};
        if (kind == Protocol.EVENT_PAN) {
            int[] ids = pans.get(view);
            if (ids == null) return null;
            if (began) {
                s[0] = current(ids[0], 0);
                s[1] = current(ids[1], 0);
            }
            set(ids[0], s[0] + x);
            set(ids[1], s[1] + y);
            if (began || phase == Protocol.GESTURE_ENDED || phase == Protocol.GESTURE_CANCELLED) {
                out = new float[] {(float) current(ids[0], 0), (float) current(ids[1], 0)};
            }
        } else if (kind == Protocol.EVENT_PINCH || kind == Protocol.EVENT_ROTATION) {
            boolean pinch = kind == Protocol.EVENT_PINCH;
            Integer id = (pinch ? pinches : rotations).get(view);
            if (id == null) return null;
            int slot = pinch ? 2 : 3;
            if (began) s[slot] = current(id, pinch ? 1 : 0);
            set(id, pinch ? s[slot] * value : s[slot] + value);
        } else {
            return null;
        }
        touch();
        return out;
    }

    /** Value id's value, stopping what animates it; unset without one. */
    private double current(int id, double unset) {
        ANode n = value(id);
        if (n == null) return unset;
        interrupt(n);
        return n.value;
    }

    private void set(int id, double v) {
        ANode n = value(id);
        if (n != null) n.value = v;
    }

    /** Sets the values bound to scroll view view to its offset, in points. */
    boolean scroll(int view, float x, float y) {
        return drive(scrolls, view, x, y);
    }

    private boolean drive(Map<Integer, int[]> m, int view, float x, float y) {
        int[] ids = m.get(view);
        if (ids == null) return false;
        float[] p = {x, y};
        for (int i = 0; i < 2; i++) {
            ANode n = value(ids[i]);
            if (n != null) {
                interrupt(n);
                n.value = p[i];
            }
        }
        // Scrolling again ends the settling where it is.
        settling.removeIf(n -> dependsOn(n.op.input, ids));
        touch();
        return true;
    }

    /** Whether a value follows view's pull. */
    boolean pulls(int view) {
        return pulls.containsKey(view);
    }

    /** Sets the value bound to scroll view view's pull to how far it is pulled past its top, in points. */
    boolean pull(int view, float distance) {
        Integer id = pulls.get(view);
        ANode n = id == null ? null : value(id);
        if (n == null || n.value == distance) return false;
        interrupt(n);
        n.value = distance;
        touch();
        return true;
    }

    /** Drops what is bound to a deleted view. */
    void forget(int view) {
        pulls.remove(view);
        canvases.remove(view);
        views.remove(view);
        pans.remove(view);
        scrolls.remove(view);
        pinches.remove(view);
        rotations.remove(view);
        starts.remove(view);
        changed.remove(view);
    }

    /** N's output in the current generation; a color's goes to n.color. */
    private double eval(ANode n) {
        if (n.gen == gen && n.gen != 0) return n.num;
        n.gen = gen;
        switch (n.kind) {
            case CREATE_VALUE:
                n.num = n.value + n.offset;
                break;
            case CREATE_INTERPOLATE: {
                double in = 0;
                ANode src = nodes.get(n.op.input);
                if (src != null) in = eval(src);
                n.num = n.op.interpolation.at(in);
                n.color = n.op.interpolation.color;
                break;
            }
            case CREATE_MATH: {
                double x = operand(n.op.aNode, n.op.aConst), y = operand(n.op.bNode, n.op.bConst);
                switch (n.op.math) {
                    case MATH_ADD: n.num = x + y; break;
                    case MATH_SUB: n.num = x - y; break;
                    case MATH_MUL: n.num = x * y; break;
                    case MATH_DIV: n.num = y != 0 ? x / y : 0; break;
                    case MATH_MOD: n.num = y != 0 ? fmod(fmod(x, y) + y, y) : 0; break;
                    default: break;
                }
                break;
            }
            case CREATE_DIFF_CLAMP: {
                double in = 0;
                ANode src = nodes.get(n.op.input);
                if (src != null) in = eval(src);
                if (!n.primed) {
                    n.primed = true;
                    n.last = Math.min(Math.max(in, n.op.min), n.op.max);
                } else if (!settling.contains(n)) {
                    n.last = Math.min(Math.max(n.last + in - n.lastIn, n.op.min), n.op.max);
                }
                n.lastIn = in;
                n.num = n.last;
                break;
            }
            case CREATE_RUBBER_BAND: {
                double in = 0;
                ANode src = nodes.get(n.op.input);
                if (src != null) in = eval(src);
                n.num = rubberBand(in, n.op.min, n.op.max, n.op.value);
                break;
            }
            default:
                break;
        }
        return n.num;
    }

    /** driver.RubberBand: x resisted past lo and hi, by at most dimension. */
    static double rubberBand(double x, double lo, double hi, double dimension) {
        if (x >= lo && x <= hi) return x;
        double over = x < lo ? lo - x : x - hi;
        double band = dimension <= 0 ? 0 : (1 - 1 / (over * 0.55 / dimension + 1)) * dimension;
        return x < lo ? lo - band : hi + band;
    }

    /** Go's math.Mod: the remainder with x's sign, as Java's % on doubles. */
    private static double fmod(double x, double y) {
        return x % y;
    }

    private double operand(int node, double constant) {
        if (node == 0) return constant;
        ANode n = nodes.get(node);
        return n != null ? eval(n) : 0;
    }

    // Runs and their steps.

    private static final class Run {
        final int id;
        final Step root;
        /** Its frame rate, 0 for every step; last is when it last moved, once moved. */
        final double rate;
        double last;
        boolean moved;

        Run(int id, Step root, double rate) {
            this.id = id;
            this.root = root;
            this.rate = rate;
        }
    }

    /**
     * How early a run's frame may land, in seconds, as core's frameSlack: a
     * display frame a little before the rate's own would otherwise wait for
     * the one after it, halving the rate.
     */
    private static final double FRAME_SLACK = 0.002;

    /** A node of a running animation's tree. */
    private abstract static class Step {
        /** When the step ended, set by a step that reports it is done. */
        double end;

        /** Begins at time at, in seconds, or on the next step when at is negative. */
        abstract void start(Animator a, double at);

        /** Advances to now and reports whether the step is done; end says when it ended. */
        abstract boolean step(Animator a, double now);

        /** Releases the values it moves. */
        abstract void stop(Animator a);

        /** Gives the leaves their run, so an interruption ends it. */
        abstract void bind(Run r);

        /** Lists the values the step moves, for loops to reset. */
        abstract void values(List<Integer> out);
    }

    private Step build(Spec an) {
        switch (an.kind) {
            case TIMING:
            case SPRING:
            case DECAY:
                return new Leaf(an);
            case DELAY:
                return new Wait(seconds(an.delay));
            case SEQUENCE: {
                Seq s = new Seq();
                for (Spec c : an.steps) s.steps.add(build(c));
                return s;
            }
            case PARALLEL:
            case STAGGER: {
                Par p = new Par();
                for (int i = 0; i < an.steps.length; i++) {
                    Step st = build(an.steps[i]);
                    if (an.kind == STAGGER && i > 0) {
                        Seq s = new Seq();
                        s.steps.add(new Wait(seconds(an.delay) * i));
                        s.steps.add(st);
                        st = s;
                    }
                    p.steps.add(st);
                }
                return p;
            }
            case LOOP: {
                Loop l = new Loop(an.iterations, an.noReset);
                if (an.steps.length > 0) l.body = build(an.steps[0]);
                return l;
            }
            default:
                return new Wait(0);
        }
    }

    /** Moves one value. */
    private static final class Leaf extends Step {
        final Spec spec;
        Run run;
        ANode node;
        double from;
        double vel; // initial velocity, units per second
        double t0;
        boolean started; // has its start time
        // Decay segments: a bounce starts a new one at a bound.
        double segFrom, segVel, segT0;
        boolean done;

        Leaf(Spec spec) {
            this.spec = spec;
        }

        @Override
        void bind(Run r) {
            run = r;
        }

        @Override
        void values(List<Integer> out) {
            out.add(spec.value);
        }

        @Override
        void start(Animator a, double at) {
            ANode n = a.value(spec.value);
            node = n;
            done = false;
            if (n == null) {
                done = true;
                return;
            }
            if (n.driver != null && n.driver != this) a.interrupt(n);
            n.driver = this;
            from = n.value;
            vel = n.vel;
            if (spec.kind == SPRING && spec.springVelocity != 0) vel = spec.springVelocity;
            if (spec.kind == DECAY && spec.decayVelocity != 0) vel = spec.decayVelocity;
            t0 = at;
            started = at >= 0;
            segFrom = from;
            segVel = vel;
            segT0 = at;
        }

        @Override
        void stop(Animator a) {
            if (node != null && node.driver == this) {
                node.driver = null;
                node.vel = 0;
            }
        }

        @Override
        boolean step(Animator a, double now) {
            ANode n = node;
            if (done || n == null || n.driver != this) {
                // Something else took the value: the run was ended already.
                end = now;
                return true;
            }
            if (!started) {
                t0 = now;
                segT0 = now;
                started = true;
            }
            double[] r;
            switch (spec.kind) {
                case TIMING:
                    r = timing(now);
                    break;
                case SPRING:
                    r = spring(now);
                    break;
                default:
                    r = decay(now);
                    break;
            }
            boolean finished = r[2] != 0;
            end = spec.kind == TIMING ? r[3] : now;
            n.value = r[0];
            n.vel = r[1];
            a.touch();
            if (finished) {
                done = true;
                n.driver = null;
                n.vel = 0;
            }
            return finished;
        }

        /** Value, velocity, done (1) and end. */
        private double[] timing(double now) {
            double d = seconds(spec.duration), delay = seconds(spec.timingDelay);
            double end = t0 + delay + d;
            // Steps of a composite start at exact ends; a nanosecond's
            // slack keeps float error from costing a frame.
            if (now >= end - 1e-9) return new double[] {spec.to, 0, 1, end};
            double x = 1;
            if (d > 0) x = (now - t0 - delay) / d;
            if (x <= 0) return new double[] {from, 0, 0, end};
            return new double[] {from + (spec.to - from) * spec.easing.at(x), 0, 0, end};
        }

        /** React Native's closed-form damped spring. */
        private double[] spring(double now) {
            double k = spec.stiffness, damp = spec.damping, m = spec.mass;
            double to = spec.to;
            double t = now - t0;
            double v0 = -vel;
            double zeta = damp / (2 * Math.sqrt(k * m));
            double w0 = Math.sqrt(k / m);
            double x0 = to - from;
            double pos, v;
            if (zeta < 1) {
                double w1 = w0 * Math.sqrt(1 - zeta * zeta);
                double env = Math.exp(-zeta * w0 * t);
                double sin = Math.sin(w1 * t), cos = Math.cos(w1 * t);
                pos = to - env * ((v0 + zeta * w0 * x0) / w1 * sin + x0 * cos);
                v = zeta * w0 * env * (sin * (v0 + zeta * w0 * x0) / w1 + x0 * cos)
                        - env * (cos * (v0 + zeta * w0 * x0) - w1 * x0 * sin);
            } else {
                double env = Math.exp(-w0 * t);
                pos = to - env * (x0 + (v0 + w0 * x0) * t);
                v = env * (v0 * (t * w0 - 1) + t * x0 * w0 * w0);
            }
            final double rest = 0.001;
            if (Math.abs(v) <= rest && Math.abs(to - pos) <= rest || t > 60) return new double[] {to, 0, 1};
            return new double[] {pos, v, 0};
        }

        /** Slows the value exponentially, bouncing off or stopping at its bounds. */
        private double[] decay(double now) {
            double k = 1 - spec.deceleration; // per millisecond
            final double restSpeed = 0.0005; // units per millisecond
            double pos = 0;
            for (int i = 0; i < 16; i++) {
                double t = (now - segT0) * 1000;
                double v = segVel / 1000;
                pos = segFrom + v / k * (1 - Math.exp(-k * t));
                double speed = v * Math.exp(-k * t);
                if (spec.clamp && (pos < spec.min || pos > spec.max)) {
                    double bound = pos < spec.min ? spec.min : spec.max;
                    // When it got there: x(t) = bound.
                    double f = 1 - (bound - segFrom) * k / v;
                    if (f <= 0 || f >= 1 || spec.bounce <= 0) return new double[] {bound, 0, 1};
                    double tb = -Math.log(f) / k;
                    double hit = v * Math.exp(-k * tb);
                    segFrom = bound;
                    segVel = -hit * spec.bounce * 1000;
                    segT0 = segT0 + tb / 1000;
                    continue;
                }
                if (Math.abs(speed) < restSpeed) return new double[] {pos, 0, 1};
                return new double[] {pos, speed * 1000, 0};
            }
            return new double[] {pos, 0, 1};
        }
    }

    /** Waits. */
    private static final class Wait extends Step {
        final double d;
        double t0;
        boolean started;

        Wait(double d) {
            this.d = d;
        }

        @Override
        void start(Animator a, double at) {
            t0 = at;
            started = at >= 0;
        }

        @Override
        void stop(Animator a) {}

        @Override
        void bind(Run r) {}

        @Override
        void values(List<Integer> out) {}

        @Override
        boolean step(Animator a, double now) {
            if (!started) {
                t0 = now;
                started = true;
            }
            end = t0 + d;
            return now >= end - 1e-9;
        }
    }

    /** Runs its steps in turn, each from the exact end of the last. */
    private static final class Seq extends Step {
        final List<Step> steps = new ArrayList<>();
        int i;

        @Override
        void start(Animator a, double at) {
            i = 0;
            if (!steps.isEmpty()) steps.get(0).start(a, at);
        }

        @Override
        void stop(Animator a) {
            if (i < steps.size()) steps.get(i).stop(a);
        }

        @Override
        void bind(Run r) {
            for (Step st : steps) st.bind(r);
        }

        @Override
        void values(List<Integer> out) {
            for (Step st : steps) st.values(out);
        }

        @Override
        boolean step(Animator a, double now) {
            double e = now;
            while (i < steps.size()) {
                Step st = steps.get(i);
                if (!st.step(a, now)) return false;
                e = st.end;
                i++;
                if (i < steps.size()) steps.get(i).start(a, e);
            }
            end = e;
            return true;
        }
    }

    /** Runs its steps at once. */
    private static final class Par extends Step {
        final List<Step> steps = new ArrayList<>();
        boolean[] done;

        @Override
        void start(Animator a, double at) {
            done = new boolean[steps.size()];
            for (Step st : steps) st.start(a, at);
        }

        @Override
        void stop(Animator a) {
            for (int i = 0; i < steps.size(); i++) {
                if (!done[i]) steps.get(i).stop(a);
            }
        }

        @Override
        void bind(Run r) {
            for (Step st : steps) st.bind(r);
        }

        @Override
        void values(List<Integer> out) {
            for (Step st : steps) st.values(out);
        }

        @Override
        boolean step(Animator a, double now) {
            boolean all = true;
            double e = 0;
            for (int i = 0; i < steps.size(); i++) {
                if (done[i]) continue;
                Step st = steps.get(i);
                if (st.step(a, now)) {
                    done[i] = true;
                    e = Math.max(e, st.end);
                } else {
                    all = false;
                }
            }
            if (!all) return false;
            if (e == 0) e = now;
            end = e;
            return true;
        }
    }

    /** Runs its body again, its values put back where they started. */
    private static final class Loop extends Step {
        Step body;
        final int n;
        final boolean noReset;
        int i;
        final List<Integer> ids = new ArrayList<>();
        final List<Double> initial = new ArrayList<>();

        Loop(int n, boolean noReset) {
            this.n = n;
            this.noReset = noReset;
        }

        private void startAt(Animator a, double at) {
            if (!noReset) {
                for (int k = 0; k < ids.size(); k++) {
                    ANode v = a.value(ids.get(k));
                    if (v != null) v.value = initial.get(k);
                }
                a.touch();
            }
            body.start(a, at);
        }

        @Override
        void start(Animator a, double at) {
            i = 0;
            if (body == null) return;
            ids.clear();
            body.values(ids);
            initial.clear();
            for (int id : ids) {
                ANode v = a.value(id);
                initial.add(v != null ? v.value : 0.0);
            }
            body.start(a, at);
        }

        @Override
        void stop(Animator a) {
            if (body != null) body.stop(a);
        }

        @Override
        void bind(Run r) {
            if (body != null) body.bind(r);
        }

        @Override
        void values(List<Integer> out) {
            if (body != null) body.values(out);
        }

        @Override
        boolean step(Animator a, double now) {
            if (body == null) {
                end = now;
                return true;
            }
            // A body that ends within this frame starts again at its end,
            // up to a bound so a zero-length body cannot spin.
            for (int k = 0; k < 64; k++) {
                if (!body.step(a, now)) return false;
                double e = body.end;
                i++;
                if (n > 0 && i >= n) {
                    end = e;
                    return true;
                }
                startAt(a, e);
                if (e >= now) return false;
            }
            return false;
        }
    }
}
