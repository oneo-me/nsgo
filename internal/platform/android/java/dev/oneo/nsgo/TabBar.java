package dev.oneo.nsgo;

import android.content.Context;
import android.content.res.ColorStateList;
import android.content.res.Configuration;
import android.content.res.Resources;
import android.graphics.Typeface;
import android.graphics.drawable.Drawable;
import android.graphics.drawable.GradientDrawable;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

/**
 * core.TabBar's view, as NSH's TabBarComponent: Material 3's navigation
 * bar drawn with framework views, in its baseline colors for the
 * appearance shown and the selected tab in the theme's accent. Each tab is an icon in an active indicator over its
 * label; a press selects it at once and tells the listener. Native
 * components build it by reflection, so its constructor, setters and
 * listener are public.
 */
public final class TabBar extends LinearLayout {
    /** Hears the tab the user chose, by index. */
    public interface OnChangeListener {
        void onChange(int index);
    }

    private OnChangeListener listener;
    private int selected;
    /** The theme's accent as ARGB (0: none): the selected tab's icon, label and indicator. */
    private int tint;
    private final float density;

    public TabBar(Context context) {
        super(context);
        density = context.getResources().getDisplayMetrics().density;
        setOrientation(HORIZONTAL);
        setBackgroundColor(night() ? 0xff211f26 : 0xfff3edf7);
    }

    /**
     * Material's 80 dp tall, as wide as it is placed: measured with no
     * width of its own, it asks for its height alone.
     */
    @Override
    protected void onMeasure(int widthSpec, int heightSpec) {
        // Under the system's navigation bar, as Material's bar reaches,
        // its tabs above it.
        int inset = navigationInset();
        if (getPaddingBottom() != inset) setPadding(0, 0, 0, inset);
        int height = MeasureSpec.getMode(heightSpec) == MeasureSpec.EXACTLY ? MeasureSpec.getSize(heightSpec) : dp(80) + inset;
        int width = MeasureSpec.getMode(widthSpec) == MeasureSpec.UNSPECIFIED ? 0 : MeasureSpec.getSize(widthSpec);
        super.onMeasure(MeasureSpec.makeMeasureSpec(width, MeasureSpec.EXACTLY),
                MeasureSpec.makeMeasureSpec(height, MeasureSpec.EXACTLY));
    }

    public void setOnChangeListener(OnChangeListener l) {
        listener = l;
    }

    /** Makes the tabs from a JSON list of {"title", "icon"}. */
    public void setItems(String json) {
        removeAllViews();
        JSONArray items;
        try {
            items = new JSONArray(json);
        } catch (JSONException e) {
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject tab = items.optJSONObject(i);
            if (tab == null) continue;
            addView(tab(i, tab.optString("title"), tab.optString("icon")),
                    new LayoutParams(0, LayoutParams.MATCH_PARENT, 1));
        }
        restyle();
    }

    public void setSelectedIndex(int index) {
        selected = index;
        restyle();
    }

    /** Takes the theme's accent as ARGB, 0 for Material's colors. */
    public void setTint(long argb) {
        tint = (int) argb;
        restyle();
    }

    private View tab(int index, String title, String icon) {
        LinearLayout item = new LinearLayout(getContext());
        item.setOrientation(VERTICAL);
        item.setGravity(Gravity.CENTER);
        item.setPadding(0, dp(12), 0, dp(16));
        FrameLayout pill = new FrameLayout(getContext());
        GradientDrawable shape = new GradientDrawable();
        shape.setCornerRadius(dp(16));
        pill.setBackground(shape);
        ImageView image = new ImageView(getContext());
        Drawable d = drawable(icon);
        if (d != null) image.setImageDrawable(d.mutate());
        pill.addView(image, new FrameLayout.LayoutParams(dp(24), dp(24), Gravity.CENTER));
        item.addView(pill, new LayoutParams(dp(64), dp(32)));
        TextView label = new TextView(getContext());
        label.setText(title);
        label.setTextSize(TypedValue.COMPLEX_UNIT_SP, 12);
        label.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        label.setSingleLine(true);
        label.setGravity(Gravity.CENTER);
        LayoutParams lp = new LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT);
        lp.topMargin = dp(4);
        item.addView(label, lp);
        item.setContentDescription(title);
        item.setClickable(true);
        item.setOnClickListener(v -> {
            setSelectedIndex(index);
            if (listener != null) listener.onChange(index);
        });
        return item;
    }

    /** Colors each tab as selected or not. */
    private void restyle() {
        boolean night = night();
        int idle = night ? 0xffcac4d0 : 0xff49454f;
        int indicator = night ? 0xff4a4458 : 0xffe8def8;
        int activeIcon = night ? 0xffe8def8 : 0xff1d192b;
        int activeLabel = night ? 0xffe6e0e9 : 0xff1d1b20;
        if (tint != 0) {
            // The icon and label in the accent, over a faint pill of it.
            indicator = (tint & 0x00ffffff) | (((tint >>> 24) * 0x1f / 0xff) << 24);
            activeIcon = tint;
            activeLabel = tint;
        }
        for (int i = 0; i < getChildCount(); i++) {
            LinearLayout item = (LinearLayout) getChildAt(i);
            boolean on = i == selected;
            item.setSelected(on);
            FrameLayout pill = (FrameLayout) item.getChildAt(0);
            ((GradientDrawable) pill.getBackground()).setColor(on ? indicator : 0);
            ((ImageView) pill.getChildAt(0)).setImageTintList(ColorStateList.valueOf(on ? activeIcon : idle));
            TextView label = (TextView) item.getChildAt(1);
            label.setTextColor(on ? activeLabel : idle);
        }
    }

    /** The drawable an icon names: "android:name" the system's, else the app's. */
    private Drawable drawable(String name) {
        if (name == null || name.isEmpty()) return null;
        Resources res = getContext().getResources();
        int id;
        if (name.startsWith("android:")) {
            res = Resources.getSystem();
            id = res.getIdentifier(name.substring(8), "drawable", "android");
        } else {
            id = res.getIdentifier(name, "drawable", getContext().getPackageName());
        }
        if (id == 0) return null;
        try {
            return res.getDrawable(id, null);
        } catch (Resources.NotFoundException e) {
            return null;
        }
    }

    /** The height the system's navigation bar covers, in pixels. */
    private int navigationInset() {
        View root = getRootView();
        if (getContext() instanceof android.app.Activity) {
            root = ((android.app.Activity) getContext()).getWindow().getDecorView();
        }
        android.view.WindowInsets in = root.getRootWindowInsets();
        return in != null ? in.getSystemWindowInsetBottom() : 0;
    }

    private boolean night() {
        int mode = getContext().getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        return mode == Configuration.UI_MODE_NIGHT_YES;
    }

    private int dp(float v) {
        return Math.round(v * density);
    }
}
