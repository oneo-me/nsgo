package dev.oneo.nsgo;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Context;
import android.content.ContextWrapper;
import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;
import android.util.Base64;
import android.view.InputDevice;
import android.view.KeyCharacterMap;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.View;
import android.view.ViewGroup;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import java.io.ByteArrayOutputStream;
import java.io.DataInputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

/**
 * The test agent of nsgo test -target android: the test in the app's
 * process asks it for input over the pipe (Protocol.TEST), and it acts on
 * the UI thread as a user would: MotionEvents dispatched to the window as
 * a finger's, drags that rest before they lift, KeyEvents as a keyboard's,
 * and what no key types committed through the focused field's input
 * connection. It answers each step with Protocol.TEST_REPLY once done.
 */
final class TestAgent {
    /** The launch intent's extra that names the test: base64 JSON. */
    static final String EXTRA = "nsgo.test";

    /** An alert on screen: its dialog, buttons and list items' actions. */
    static final class Alert {
        final AlertDialog dialog;
        final int[] buttons;
        final int[] items;

        Alert(AlertDialog dialog, int[] buttons, int[] items) {
            this.dialog = dialog;
            this.buttons = buttons;
            this.items = items;
        }
    }

    private final NsgoView view;
    private final Handler ui = new Handler(Looper.getMainLooper());

    TestAgent(NsgoView view) {
        this.view = view;
    }

    /** Sets the test's arguments and environment on the app's process. */
    static void apply(String encoded, ProcessBuilder builder) {
        try {
            JSONObject test = new JSONObject(new String(Base64.decode(encoded, Base64.DEFAULT), StandardCharsets.UTF_8));
            JSONArray args = test.optJSONArray("args");
            for (int i = 0; args != null && i < args.length(); i++) builder.command().add(args.getString(i));
            JSONObject env = test.optJSONObject("env");
            if (env != null) {
                for (Iterator<String> keys = env.keys(); keys.hasNext(); ) {
                    String k = keys.next();
                    builder.environment().put(k, env.getString(k));
                }
            }
        } catch (JSONException | IllegalArgumentException e) {
            android.util.Log.w("nsgo", "unreadable test: " + e);
        }
    }

    /** Takes a step from the pipe's reader and runs it on the UI thread. */
    void handle(DataInputStream in) throws IOException {
        int id = in.readInt();
        String op = Protocol.readString(in);
        ui.post(() -> {
            try {
                run(id, new JSONObject(op));
            } catch (Exception e) {
                view.testReply(id, e.toString(), "");
            }
        });
    }

    private Activity activity() {
        Context c = view.getContext();
        while (c instanceof ContextWrapper && !(c instanceof Activity)) c = ((ContextWrapper) c).getBaseContext();
        return (Activity) c;
    }

    /** x, y of the surface's points, in the window's pixels. */
    private float[] window(double x, double y) {
        int[] at = new int[2];
        view.getLocationInWindow(at);
        float d = view.density();
        return new float[] {at[0] + (float) x * d, at[1] + (float) y * d};
    }

    private void run(int id, JSONObject op) throws Exception {
        switch (op.getString("op")) {
            case "press": {
                float[] p = window(op.getDouble("x"), op.getDouble("y"));
                long hold = Math.max(op.optLong("hold"), 0) + 50;
                long down = SystemClock.uptimeMillis();
                touch(down, MotionEvent.ACTION_DOWN, p);
                ui.postDelayed(() -> {
                    touch(down, MotionEvent.ACTION_UP, p);
                    view.testReply(id, "", "");
                }, hold);
                return;
            }
            case "drag": {
                JSONArray path = op.getJSONArray("path");
                List<float[]> points = new ArrayList<>();
                for (int i = 0; i < path.length(); i++) {
                    JSONObject p = path.getJSONObject(i);
                    points.add(window(p.getDouble("X"), p.getDouble("Y")));
                }
                finger(id, points);
                return;
            }
            case "scroll": {
                float[] from = window(op.getDouble("x"), op.getDouble("y"));
                float d = view.density();
                float[] to = {from[0] - (float) op.getDouble("dx") * d, from[1] - (float) op.getDouble("dy") * d};
                List<float[]> points = new ArrayList<>();
                points.add(from);
                points.add(to);
                finger(id, points);
                return;
            }
            case "type":
                type(op.getString("text"));
                break;
            case "compose":
                compose(op.getString("text"), false);
                break;
            case "commit":
                compose(op.getString("text"), true);
                break;
            case "key":
                key(op.getString("key"));
                break;
            case "back":
                view.back();
                break;
            case "answer": {
                int alert = op.getInt("id");
                answer(alert, op.getInt("index"));
                // The dialog calls its listener from its handler: the reply
                // waits for the answer, which then reaches the app first.
                long until = SystemClock.uptimeMillis() + 2000;
                Runnable[] check = new Runnable[1];
                check[0] = () -> {
                    if (view.alerts.get(alert) != null && SystemClock.uptimeMillis() < until) {
                        ui.postDelayed(check[0], 16);
                    } else {
                        view.testReply(id, "", "");
                    }
                };
                ui.post(check[0]);
                return;
            }
            case "screenshot":
                view.testReply(id, "", screenshot());
                return;
            case "signature":
                view.testReply(id, "", signature());
                return;
            default:
                throw new IllegalArgumentException("unknown step " + op);
        }
        view.testReply(id, "", "");
    }

    private void touch(long down, int action, float[] p) {
        MotionEvent e = MotionEvent.obtain(down, SystemClock.uptimeMillis(), action, p[0], p[1], 0);
        e.setSource(InputDevice.SOURCE_TOUCHSCREEN);
        activity().dispatchTouchEvent(e);
        e.recycle();
    }

    /**
     * A finger along points: it rests where it lands, moves 10 points a
     * frame, and rests still reporting where it is before it lifts, so it
     * ends without a fling.
     */
    private void finger(int id, List<float[]> points) {
        if (points.isEmpty()) {
            view.testReply(id, "an empty drag", "");
            return;
        }
        long down = SystemClock.uptimeMillis();
        List<float[]> moves = new ArrayList<>();
        float step = 10 * view.density();
        for (int i = 1; i < points.size(); i++) {
            float[] a = points.get(i - 1), b = points.get(i);
            int n = (int) (Math.max(Math.abs(b[0] - a[0]), Math.abs(b[1] - a[1])) / step) + 1;
            for (int k = 1; k <= n; k++) {
                float f = (float) k / n;
                moves.add(new float[] {a[0] + (b[0] - a[0]) * f, a[1] + (b[1] - a[1]) * f});
            }
        }
        float[] last = points.get(points.size() - 1);
        for (int i = 0; i < 6; i++) moves.add(last);
        touch(down, MotionEvent.ACTION_DOWN, points.get(0));
        long at = 100;
        for (float[] p : moves) {
            ui.postDelayed(() -> touch(down, MotionEvent.ACTION_MOVE, p), at);
            at += 16;
        }
        ui.postDelayed(() -> {
            touch(down, MotionEvent.ACTION_UP, last);
            view.testReply(id, "", "");
        }, at);
    }

    /** Types text as a keyboard's keys, or commits what no key types. */
    private void type(String text) {
        Activity a = activity();
        KeyCharacterMap keys = KeyCharacterMap.load(KeyCharacterMap.VIRTUAL_KEYBOARD);
        for (int i = 0; i < text.length(); ) {
            int cp = text.codePointAt(i);
            String s = new String(Character.toChars(cp));
            i += s.length();
            KeyEvent[] events = cp == '\n' ? null : keys.getEvents(s.toCharArray());
            if (cp == '\n') {
                key("enter");
            } else if (events != null) {
                for (KeyEvent e : events) a.dispatchKeyEvent(e);
            } else {
                View f = a.getCurrentFocus();
                InputConnection ic = f == null ? null : f.onCreateInputConnection(new EditorInfo());
                if (ic != null) ic.commitText(s, 1);
            }
        }
    }

    /**
     * Composes text in the focused field through its InputConnection, as an
     * input method does, or commits it; composing nothing ends the
     * composition.
     */
    private void compose(String text, boolean commit) {
        View f = activity().getCurrentFocus();
        InputConnection ic = f == null ? null : f.onCreateInputConnection(new EditorInfo());
        if (ic == null) throw new IllegalStateException("no field has the keyboard");
        ic.beginBatchEdit();
        if (commit) {
            ic.commitText(text, 1);
        } else if (text.isEmpty()) {
            ic.setComposingText("", 1);
            ic.finishComposingText();
        } else {
            ic.setComposingText(text, 1);
        }
        ic.endBatchEdit();
    }

    private static int keyCode(String name) {
        switch (name) {
            case "enter": case "return": return KeyEvent.KEYCODE_ENTER;
            case "backspace": return KeyEvent.KEYCODE_DEL;
            case "delete": return KeyEvent.KEYCODE_FORWARD_DEL;
            case "escape": return KeyEvent.KEYCODE_ESCAPE;
            case "tab": return KeyEvent.KEYCODE_TAB;
            case "space": return KeyEvent.KEYCODE_SPACE;
            case "left": return KeyEvent.KEYCODE_DPAD_LEFT;
            case "right": return KeyEvent.KEYCODE_DPAD_RIGHT;
            case "up": return KeyEvent.KEYCODE_DPAD_UP;
            case "down": return KeyEvent.KEYCODE_DPAD_DOWN;
            case "home": return KeyEvent.KEYCODE_MOVE_HOME;
            case "end": return KeyEvent.KEYCODE_MOVE_END;
            default:
                if (name.length() == 1) {
                    char c = name.charAt(0);
                    if (c >= 'a' && c <= 'z') return KeyEvent.KEYCODE_A + (c - 'a');
                    if (c >= '0' && c <= '9') return KeyEvent.KEYCODE_0 + (c - '0');
                }
                throw new IllegalArgumentException("unknown key " + name);
        }
    }

    /** Presses a key ("enter", "mod+a") as a hardware keyboard's. */
    private void key(String key) {
        String[] parts = key.toLowerCase().split("\\+");
        int meta = 0;
        for (int i = 0; i < parts.length - 1; i++) {
            switch (parts[i]) {
                case "mod": case "ctrl": case "control": meta |= KeyEvent.META_CTRL_ON | KeyEvent.META_CTRL_LEFT_ON; break;
                case "shift": meta |= KeyEvent.META_SHIFT_ON | KeyEvent.META_SHIFT_LEFT_ON; break;
                case "alt": case "option": meta |= KeyEvent.META_ALT_ON | KeyEvent.META_ALT_LEFT_ON; break;
                case "meta": case "super": meta |= KeyEvent.META_META_ON; break;
                default: throw new IllegalArgumentException("unknown modifier " + parts[i]);
            }
        }
        int code = keyCode(parts[parts.length - 1]);
        long now = SystemClock.uptimeMillis();
        Activity a = activity();
        KeyEvent down = new KeyEvent(now, now, KeyEvent.ACTION_DOWN, code, 0, meta, KeyCharacterMap.VIRTUAL_KEYBOARD, 0);
        // A key with Control is offered as a shortcut first, as the
        // window's input stage offers it: a field's Control+A selects all.
        if ((meta & KeyEvent.META_CTRL_ON) != 0 && a.dispatchKeyShortcutEvent(down)) return;
        a.dispatchKeyEvent(down);
        a.dispatchKeyEvent(new KeyEvent(now, now, KeyEvent.ACTION_UP, code, 0, meta, KeyCharacterMap.VIRTUAL_KEYBOARD, 0));
    }

    /** Taps the alert's button or list item for the action, or cancels it. */
    private void answer(int id, int index) {
        Alert a = view.alerts.get(id);
        if (a == null) throw new IllegalStateException("no alert " + id + " shows");
        if (index < 0) {
            a.dialog.cancel();
            return;
        }
        if (a.buttons != null && index < a.buttons.length) {
            a.dialog.getButton(a.buttons[index]).performClick();
            return;
        }
        for (int i = 0; i < a.items.length; i++) {
            if (a.items[i] == index) {
                android.widget.ListView list = a.dialog.getListView();
                list.performItemClick(list.getChildAt(i - list.getFirstVisiblePosition()), i, list.getItemIdAtPosition(i));
                return;
            }
        }
        throw new IllegalArgumentException("the alert has no action " + index);
    }

    /** The window drawn into a bitmap, as a PNG in base64. */
    private String screenshot() {
        View root = activity().getWindow().getDecorView();
        Bitmap bitmap = Bitmap.createBitmap(Math.max(root.getWidth(), 1), Math.max(root.getHeight(), 1), Bitmap.Config.ARGB_8888);
        root.draw(new Canvas(bitmap));
        ByteArrayOutputStream png = new ByteArrayOutputStream();
        bitmap.compress(Bitmap.CompressFormat.PNG, 100, png);
        bitmap.recycle();
        return Base64.encodeToString(png.toByteArray(), Base64.NO_WRAP);
    }

    /**
     * Where every shown view is, how far it scrolled and how opaque it
     * is: the test compares two of these to see whether anything moves.
     */
    private String signature() {
        StringBuilder b = new StringBuilder();
        walk(activity().getWindow().getDecorView(), b);
        return Integer.toHexString(b.toString().hashCode());
    }

    private static void walk(View v, StringBuilder b) {
        if (v.getVisibility() != View.VISIBLE) return;
        b.append(v.getLeft()).append(',').append(v.getTop()).append(',')
                .append(v.getTranslationX()).append(',').append(v.getTranslationY()).append(',')
                .append(v.getScrollX()).append(',').append(v.getScrollY()).append(',')
                .append(v.getAlpha()).append(',').append(v.getScaleX()).append(';');
        if (v instanceof ViewGroup) {
            ViewGroup g = (ViewGroup) v;
            for (int i = 0; i < g.getChildCount(); i++) walk(g.getChildAt(i), b);
        }
    }
}
