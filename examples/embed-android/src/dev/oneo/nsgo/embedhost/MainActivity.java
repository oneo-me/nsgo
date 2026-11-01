package dev.oneo.nsgo.embedhost;

import android.app.Activity;
import android.net.Uri;
import android.os.Bundle;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;
import dev.oneo.nsgo.NsgoComponent;
import dev.oneo.nsgo.NsgoView;
import java.text.DateFormat;
import java.util.ArrayList;
import java.util.Date;
import java.util.List;
import java.util.Map;

/**
 * An Android app of its own that embeds the NSGo app in examples/embedded:
 * its screen shows a status line, a button and two NsgoViews, each an app
 * of its own. The host greets each view's user through its launch
 * parameters, sends both a message from the button, answers their "time"
 * calls, hears their "tapped" posts, routes their order links and gives
 * each a badge of its own as a native component. README.md builds it.
 */
public class MainActivity extends Activity {
    private TextView status;
    private final List<NsgoView> apps = new ArrayList<>();
    private int sent;

    /** The host's badge: a button showing the app's count, which emits "tap". */
    static final class Badge implements NsgoComponent {
        private final Button button;

        Badge(android.content.Context context, NsgoComponent.Events events) {
            button = new Button(context);
            button.setText("Badge 0");
            button.setAllCaps(false);
            button.setOnClickListener(v -> events.emit("tap", null));
        }

        @Override
        public View view() {
            return button;
        }

        @Override
        public void setProp(String name, Object value) {
            if (name.equals("count")) button.setText("Badge " + value);
        }
    }

    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        LinearLayout column = new LinearLayout(this);
        column.setOrientation(LinearLayout.VERTICAL);
        column.setFitsSystemWindows(true);
        LinearLayout bar = new LinearLayout(this);
        status = new TextView(this);
        status.setText("The host's own views are above; NSGo's below.");
        bar.addView(status, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        Button send = new Button(this);
        send.setText("Send a message");
        send.setAllCaps(false);
        send.setOnClickListener(v -> sendMessage());
        bar.addView(send);
        column.addView(bar);

        String[][] users = {{"Ada", null}, {"Grace", "#cf222e"}};
        for (String[] u : users) {
            String user = u[0];
            NsgoView app = new NsgoView(this);
            app.setParams(Map.of("user", user));
            if (u[1] != null) app.setTokens(Map.of("accent", u[1]), null);
            app.handle("time", (data, reply) ->
                    reply.resolve(DateFormat.getTimeInstance(DateFormat.MEDIUM).format(new Date())));
            app.handle("tapped", (data, reply) ->
                    status.setText(user + " tapped " + ((Map<?, ?>) data).get("count") + " times"));
            app.setOpenUrlHandler(url -> {
                // The host's own router takes its order pages.
                Uri uri = Uri.parse(url);
                if (!"example.com".equals(uri.getHost()) || !uri.getPath().startsWith("/orders/")) return false;
                status.setText(user + " opened order " + uri.getLastPathSegment() + " in the host");
                return true;
            });
            app.setErrorListener(message -> android.util.Log.w("embedhost", "the app failed: " + message));
            app.registerComponent("host.badge", Badge::new);
            column.addView(app, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1));
            apps.add(app);
        }
        setContentView(column);
    }

    private void sendMessage() {
        sent++;
        for (NsgoView app : apps) app.send("message", Map.of("text", "Message " + sent + " from the host"));
    }

    @Override
    protected void onActivityResult(int request, int result, android.content.Intent data) {
        super.onActivityResult(request, result, data);
        // The photo and file pickers and the save dialog answer here.
        for (NsgoView app : apps) app.activityResult(request, result, data);
    }

    @Override
    protected void onDestroy() {
        for (NsgoView app : apps) app.stop();
        super.onDestroy();
    }
}
