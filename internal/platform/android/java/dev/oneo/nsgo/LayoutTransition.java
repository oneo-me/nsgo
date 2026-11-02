package dev.oneo.nsgo;

import android.animation.TimeInterpolator;
import java.io.DataInputStream;
import java.io.IOException;

/**
 * One phase of a batch's LayoutAnimation, or a frame's own transition, as
 * core.LayoutTransition: its
 * progress over time, the same as core's At, durations in nanoseconds.
 */
final class LayoutTransition {
    // Curves, as core.LayoutCurve.
    static final int LINEAR = 0;
    static final int EASE_IN = 1;
    static final int EASE_OUT = 2;
    static final int EASE_IN_OUT = 3;
    static final int SPRING = 4;
    static final int KEYBOARD = 5;
    static final int BEZIER = 6;
    // What appearing and disappearing views animate, as core.LayoutProperty.
    static final int OPACITY = 0;
    static final int SCALE = 1;

    int curve, property;
    float damping;
    /** BEZIER's control points: x1, y1, x2, y2. */
    float[] bezier;
    long duration, delay;
    /**
     * On the keyboard's curve, the IME's interpolator, and how far the
     * keyboard moved before the batch came, in nanoseconds.
     */
    private TimeInterpolator ime;
    private long ahead;

    /** The keyboard's motion as WindowInsetsAnimation reports it: nanoseconds. */
    static final class Keyboard {
        TimeInterpolator interpolator;
        long duration, start;
    }

    /**
     * Moves a phase on the keyboard's curve onto the keyboard's motion
     * while it runs at now: its interpolator and duration, and as far in
     * as the keyboard is.
     */
    void alignToKeyboard(Keyboard k, long now) {
        if (curve != KEYBOARD || k.duration <= 0 || now - k.start >= k.duration) return;
        ime = k.interpolator;
        duration = k.duration;
        delay = 0;
        ahead = now - k.start;
    }

    /** Reads a transition as protocol.go's writeLayoutTransition writes it: null for none. */
    static LayoutTransition read(DataInputStream in) throws IOException {
        if (in.readUnsignedByte() == 0) return null;
        LayoutTransition t = new LayoutTransition();
        t.curve = in.readUnsignedByte();
        t.damping = in.readFloat();
        t.bezier = new float[] {in.readFloat(), in.readFloat(), in.readFloat(), in.readFloat()};
        t.property = in.readUnsignedByte();
        t.duration = in.readLong();
        t.delay = in.readLong();
        return t;
    }

    /** The progress at elapsed nanoseconds since the change; a spring may overshoot 1. */
    double at(long elapsed) {
        elapsed += ahead;
        double x = 1;
        if (duration > 0) x = (double) (elapsed - delay) / (double) duration;
        if (x <= 0) return 0;
        if (x >= 1) return 1;
        switch (curve) {
            case EASE_IN: return Animator.bezier(0.42f, 0, 1, 1, x);
            case EASE_OUT: return Animator.bezier(0, 0, 0.58f, 1, x);
            case EASE_IN_OUT: return Animator.bezier(0.42f, 0, 0.58f, 1, x);
            case SPRING: return spring(damping, x);
            case BEZIER: return Animator.bezier(bezier[0], bezier[1], bezier[2], bezier[3], x);
            // The IME's own interpolator, else InsetsController's.
            case KEYBOARD: return ime != null ? ime.getInterpolation((float) x) : Animator.bezier(0.2f, 0, 0, 1, x);
            default: return x;
        }
    }

    /** Whether the transition is done at elapsed nanoseconds. */
    boolean done(long elapsed) {
        return duration <= 0 || elapsed + ahead - delay >= duration;
    }

    /**
     * A damped spring from 0 to 1 at x, the fraction of the duration,
     * stiff enough to settle by the end, as core's layoutSpring.
     */
    private static double spring(double damping, double x) {
        final double settle = 6.907755; // ln(1000): the envelope falls to 1/1000
        double zeta = Math.min(Math.max(damping, 0.05), 1);
        if (zeta > 0.999) {
            double w = 9.233;
            return 1 - (1 + w * x) * Math.exp(-w * x);
        }
        double w = settle / zeta;
        double wd = w * Math.sqrt(1 - zeta * zeta);
        double env = Math.exp(-zeta * w * x);
        return 1 - env * (Math.cos(wd * x) + zeta * w / wd * Math.sin(wd * x));
    }

    /** A frame (x, y, width, height) from a to b at progress p, never of negative size, as core.LerpBounds. */
    static float[] lerp(float[] a, float[] b, double p) {
        float[] f = new float[4];
        for (int i = 0; i < 4; i++) f[i] = a[i] + (float) ((b[i] - a[i]) * p);
        f[2] = Math.max(f[2], 0);
        f[3] = Math.max(f[3], 0);
        return f;
    }
}
