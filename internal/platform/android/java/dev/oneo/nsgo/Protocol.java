package dev.oneo.nsgo;

import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * The pipe protocol between the runtime and the app's Go process: messages
 * of a type byte, a big-endian length and a payload. Kept in sync with
 * internal/platform/android/android.go, whose writers define each payload.
 */
final class Protocol {
    // Go to Java.
    static final int BATCH = 1;
    static final int PREPARE = 2;
    static final int MEASURE = 3;
    static final int LAYOUT = 4;
    static final int RELEASE = 5;
    static final int IMAGE = 6;
    static final int LINES = 7;
    static final int CANVAS_IMAGE = 8;
    static final int FONTS = 9;
    static final int TEST = 10;
    static final int HOST_CALL = 11;
    static final int CARETS = 12;
    static final int WANT_FRAME = 13;
    static final int FONT_DATA = 14;

    // Java to Go.
    static final int START = 1;
    static final int RESIZE = 2;
    static final int EVENT = 3;
    static final int MEASURED = 4;
    static final int ENVIRONMENT = 5;
    static final int DNS = 6;
    static final int LAID_OUT = 7;
    static final int TEXT_LINES = 8;
    static final int TEST_REPLY = 9;
    static final int HOST = 10;
    static final int CARET_LINES = 11;
    static final int FRAME = 12;

    // The parts of a HOST message, and of a HOST_CALL, as host.go has them.
    static final int HOST_EVENT = 1;
    static final int HOST_HANDLE = 2;
    static final int HOST_REPLY = 3;
    static final int HOST_TOKENS = 4;
    static final int HOST_URL_HANDLER = 5;
    static final int HOST_URL_ANSWER = 6;
    static final int CALL_CALL = 1;
    static final int CALL_OPEN_URL = 2;
    static final int CALL_ERROR = 3;

    // The parts of an image request an IMAGE message carries.
    static final int IMAGE_PLACEHOLDER = 0;
    static final int IMAGE_SOURCE = 1;

    // Mount item kinds, as in core/mount.go.
    static final int ITEM_CREATE = 1;
    static final int ITEM_INSERT = 2;
    static final int ITEM_REMOVE = 3;
    static final int ITEM_DELETE = 4;
    static final int ITEM_FRAME = 5;
    static final int ITEM_VIEW_STYLE = 6;
    static final int ITEM_TEXT = 7;
    static final int ITEM_INPUT = 8;
    static final int ITEM_INPUT_VALUE = 9;
    static final int ITEM_EVENTS = 10;
    static final int ITEM_VIEW_PROPS = 11;
    static final int ITEM_SCROLL_CONFIG = 12;
    static final int ITEM_SCROLL_CONTENT = 13;
    static final int ITEM_FOCUS = 14;
    static final int ITEM_BLUR = 15;
    static final int ITEM_IMAGE = 16;
    static final int ITEM_MODAL = 17;
    static final int ITEM_ALERT = 18;
    static final int ITEM_DISMISS_KEYBOARD = 19;
    static final int ITEM_SCROLL_TO = 20;
    static final int ITEM_CANVAS = 21;
    static final int ITEM_SET_CLIPBOARD = 22;
    static final int ITEM_READ_CLIPBOARD = 23;
    static final int ITEM_ANNOUNCE = 24;
    static final int ITEM_KEYMAP = 26;
    static final int ITEM_STATUS_BAR = 27;
    static final int ITEM_TITLE = 28;
    static final int ITEM_BACK_UNHANDLED = 29;
    static final int ITEM_WINDOW = 30;
    static final int ITEM_MENU = 31;
    static final int ITEM_WEB = 32;
    static final int ITEM_WEB_COMMAND = 33;
    static final int ITEM_VIDEO = 34;
    static final int ITEM_VIDEO_SEEK = 35;
    static final int ITEM_NATIVE = 36;
    static final int ITEM_NATIVE_COMMAND = 37;
    static final int ITEM_ANIMATION = 38;
    static final int ITEM_ZOOM_TO = 39;
    static final int ITEM_SELECTION = 41;
    static final int ITEM_SCROLL_BY = 42;
    static final int ITEM_STICKY_CELLS = 43;
    static final int ITEM_ACCESSIBILITY_LABEL = 44;
    static final int ITEM_ACCESSIBILITY_FOCUS = 45;
    static final int ITEM_SCREEN = 46;
    static final int ITEM_OVERFLOW = 47;
    static final int ITEM_BLOCK_NATIVE = 48;
    static final int ITEM_SHADER = 49;
    static final int ITEM_SET_CLIPBOARD_IMAGE = 50;
    static final int ITEM_OPEN_URL = 51;
    static final int ITEM_PICK_IMAGES = 52;
    static final int ITEM_APPEARANCE = 53;
    static final int ITEM_POPUP_MENU = 54;
    static final int ITEM_QUIT = 55;
    static final int ITEM_BACK_PREVIEW = 56;
    static final int ITEM_HISTORY = 57;
    static final int ITEM_PICK_FILES = 58;
    static final int ITEM_SAVE_FILE = 59;
    static final int ITEM_DIRECTION = 60;
    static final int ITEM_OPEN_FILE = 63;
    static final int ITEM_SHARE = 64;
    static final int ITEM_INSTALL_PACKAGE = 65;
    static final int ITEM_CAN_INSTALL_PACKAGES = 66;
    static final int ITEM_INSTALL_SETTINGS = 67;

    // View kinds, as in core/mount.go.
    static final int KIND_VIEW = 1;
    static final int KIND_TEXT = 2;
    static final int KIND_INPUT = 3;
    static final int KIND_SCROLL = 4;
    static final int KIND_IMAGE = 5;
    static final int KIND_MODAL = 6;
    static final int KIND_CANVAS = 7;
    static final int KIND_WEB = 8;
    static final int KIND_VIDEO = 9;
    static final int KIND_NATIVE = 10;
    static final int KIND_SHADER = 11;

    // Event kinds, as in core/events.go. An event mask holds 1 << kind.
    static final int EVENT_PRESS = 1;
    static final int EVENT_PRESS_DOWN = 2;
    static final int EVENT_PRESS_UP = 3;
    static final int EVENT_LONG_PRESS = 4;
    static final int EVENT_HOVER_IN = 5;
    static final int EVENT_HOVER_OUT = 6;
    static final int EVENT_FOCUS = 7;
    static final int EVENT_BLUR = 8;
    static final int EVENT_CHANGE_TEXT = 9;
    static final int EVENT_SUBMIT_EDITING = 10;
    static final int EVENT_SCROLL = 11;
    static final int EVENT_LOAD_START = 12;
    static final int EVENT_PROGRESS = 13;
    static final int EVENT_LOAD = 14;
    static final int EVENT_ERROR = 15;
    static final int EVENT_LOAD_END = 16;
    static final int EVENT_PRESS_OUTSIDE = 17;
    static final int EVENT_BACK = 18;
    static final int EVENT_ALERT = 19;
    static final int EVENT_CLIPBOARD = 20;
    static final int EVENT_CONTEXT_MENU = 21;
    static final int EVENT_KEY = 22;
    static final int EVENT_PAN = 23;
    static final int EVENT_PINCH = 24;
    static final int EVENT_ROTATION = 25;
    static final int EVENT_MENU = 26;
    static final int EVENT_MESSAGE = 27;
    static final int EVENT_NAVIGATION_STATE_CHANGE = 28;
    static final int EVENT_END = 29;
    static final int EVENT_BUFFER = 30;
    static final int EVENT_FILE_DRAG_ENTER = 31;
    static final int EVENT_FILE_DRAG_LEAVE = 32;
    static final int EVENT_FILE_DROP = 33;
    static final int EVENT_NATIVE = 34;
    static final int EVENT_NATIVE_SIZE = 35;
    static final int EVENT_ANIMATION = 36;
    static final int EVENT_SCROLL_DRAG_START = 37;
    static final int EVENT_SCROLL_DRAG_END = 38;
    static final int EVENT_MOMENTUM_START = 39;
    static final int EVENT_MOMENTUM_END = 40;
    static final int EVENT_REFRESH = 41;
    static final int EVENT_SELECTION_CHANGE = 43;
    static final int EVENT_KEY_PRESS = 44;
    static final int EVENT_COMPOSITION_CHANGE = 45;
    static final int EVENT_ACCESSIBILITY_ACTION = 46;
    static final int EVENT_HOVER_MOVE = 49;
    static final int EVENT_TOUCH = 51;
    static final int EVENT_CARET_MOVE = 52;
    static final int EVENT_PASTE = 53;
    static final int EVENT_SCREEN_HEADER = 54;
    static final int EVENT_OPEN_URL = 55;
    static final int EVENT_PICK = 56;
    static final int EVENT_BACK_STARTED = 58;
    static final int EVENT_BACK_CANCELLED = 59;
    static final int EVENT_SCROLL_END = 60;
    static final int EVENT_FILES = 61;

    // core.PointerEvents.
    static final int POINTER_EVENTS_NONE = 1;
    static final int POINTER_EVENTS_BOX_NONE = 2;
    static final int POINTER_EVENTS_BOX_ONLY = 3;

    // Web view commands, as in core/webview.go.
    static final int WEB_RELOAD = 1;
    static final int WEB_GO_BACK = 2;
    static final int WEB_GO_FORWARD = 3;
    static final int WEB_STOP_LOADING = 4;
    static final int WEB_POST_MESSAGE = 5;
    static final int WEB_INJECT_JAVASCRIPT = 6;

    // Gesture phases, as in core/gesture.go.
    static final int GESTURE_BEGAN = 1;
    static final int GESTURE_CHANGED = 2;
    static final int GESTURE_ENDED = 3;
    static final int GESTURE_CANCELLED = 4;

    // Accessibility roles, as in core/mount.go.
    static final int ROLE_BUTTON = 1;
    static final int ROLE_LINK = 2;
    static final int ROLE_HEADER = 3;
    static final int ROLE_IMAGE = 4;
    static final int ROLE_CHECKBOX = 5;
    static final int ROLE_SWITCH = 6;
    static final int ROLE_RADIO = 7;
    static final int ROLE_TAB = 8;
    static final int ROLE_TEXT = 9;
    static final int ROLE_TEXT_FIELD = 10;
    static final int ROLE_RADIO_GROUP = 11;
    static final int ROLE_TAB_LIST = 12;
    static final int ROLE_MENU = 13;
    static final int ROLE_MENU_ITEM = 14;
    static final int ROLE_DIALOG = 15;
    static final int ROLE_ALERT = 16;
    static final int ROLE_ADJUSTABLE = 17;
    static final int ROLE_PROGRESS_BAR = 18;
    static final int ROLE_TABLE = 19;
    static final int ROLE_GRID = 20;
    static final int ROLE_ROW_GROUP = 21;
    static final int ROLE_ROW = 22;
    static final int ROLE_COLUMN_HEADER = 23;
    static final int ROLE_CELL = 24;

    // How a view is an accessibility element, as core.Accessible.
    static final int ACCESSIBLE_YES = 1;
    static final int ACCESSIBLE_NO = 2;

    // Keyboard types, as in core/mount.go.
    static final int KEYBOARD_EMAIL = 1;
    static final int KEYBOARD_NUMBER = 2;
    static final int KEYBOARD_DECIMAL = 3;
    static final int KEYBOARD_PHONE = 4;
    static final int KEYBOARD_URL = 5;

    // Resize modes, as in core/element.go.
    static final int RESIZE_COVER = 0;
    static final int RESIZE_CONTAIN = 1;
    static final int RESIZE_STRETCH = 2;
    static final int RESIZE_CENTER = 3;
    static final int RESIZE_SCALE_DOWN = 4;

    // Modal animations, as in core/element.go.
    static final int MODAL_SLIDE = 0;
    static final int MODAL_FADE = 1;

    // Alert action roles, as in core/services.go; Default is 0.
    static final int ALERT_CANCEL = 1;
    static final int ALERT_DESTRUCTIVE = 2;

    // Transition properties, as in core/animate.go; 0 is all of them.
    static final int TRANSITION_OPACITY = 1;
    static final int TRANSITION_BACKGROUND = 2;
    static final int TRANSITION_TRANSFORM = 4;
    static final int TRANSITION_BORDER = 8;
    static final int TRANSITION_SHADOW = 16;

    // Status bar styles, as in core/statusbar.go; Auto is 1.
    static final int STATUS_BAR_DARK = 2;
    static final int STATUS_BAR_LIGHT = 3;

    // Drawing operations, as in core/canvas.go.
    static final int DRAW_SAVE = 1;
    static final int DRAW_RESTORE = 2;
    static final int DRAW_TRANSLATE = 3;
    static final int DRAW_SCALE = 4;
    static final int DRAW_ROTATE = 5;
    static final int DRAW_BEGIN_PATH = 6;
    static final int DRAW_MOVE_TO = 7;
    static final int DRAW_LINE_TO = 8;
    static final int DRAW_QUAD_TO = 9;
    static final int DRAW_CUBIC_TO = 10;
    static final int DRAW_ARC = 11;
    static final int DRAW_RECT = 12;
    static final int DRAW_ROUND_RECT = 13;
    static final int DRAW_ELLIPSE = 14;
    static final int DRAW_CLOSE_PATH = 15;
    static final int DRAW_FILL = 16;
    static final int DRAW_STROKE = 17;
    static final int DRAW_CLIP = 18;
    static final int DRAW_FILL_COLOR = 19;
    static final int DRAW_STROKE_COLOR = 20;
    static final int DRAW_LINE_WIDTH = 21;
    static final int DRAW_LINE_CAP = 22;
    static final int DRAW_LINE_JOIN = 23;
    static final int DRAW_ALPHA = 24;
    static final int DRAW_FILL_TEXT = 25;
    static final int DRAW_CLEAR = 26;
    static final int DRAW_FILL_GRADIENT = 27;
    static final int DRAW_STROKE_GRADIENT = 28;
    static final int DRAW_TRANSFORM = 29;
    static final int DRAW_SET_TRANSFORM = 30;
    static final int DRAW_COMPOSITE = 31;
    static final int DRAW_LINE_DASH = 32;
    static final int DRAW_LINE_DASH_OFFSET = 33;
    static final int DRAW_MITER_LIMIT = 34;
    static final int DRAW_SHADOW = 35;
    static final int DRAW_STROKE_TEXT = 36;
    static final int DRAW_IMAGE = 37;

    // Gradient spreads, as in core/canvas.go; Pad is 0.
    static final int SPREAD_REFLECT = 1;
    static final int SPREAD_REPEAT = 2;

    // Line caps and joins, as in core/canvas.go; Butt and Miter are 0.
    static final int CAP_ROUND = 1;
    static final int CAP_SQUARE = 2;
    static final int JOIN_ROUND = 1;
    static final int JOIN_BEVEL = 2;

    // Device idioms, as in core/env.go.
    static final int IDIOM_PHONE = 0;
    static final int IDIOM_TABLET = 1;
    static final int IDIOM_TV = 4;

    // driver.Selectable's off.
    static final int SELECTABLE_ON = 1;
    static final int SELECTABLE_OFF = 2;

    // Appearances, as driver.Appearance; 0 is the system's.
    static final int APPEARANCE_LIGHT = 1;
    static final int APPEARANCE_DARK = 2;

    // Text alignment, as in core/style.go.
    static final int ALIGN_CENTER = 2;
    static final int ALIGN_END = 3;
    static final int ALIGN_JUSTIFY = 4;

    // Where a line limit cuts text, as in core/text.go; 0 and Tail end
    // the last line with an ellipsis.
    static final int ELLIPSIZE_HEAD = 2;
    static final int ELLIPSIZE_MIDDLE = 3;
    static final int ELLIPSIZE_CLIP = 4;

    // Decoration styles, as in core/style.go; 0 and Solid are solid.
    static final int DECORATION_SOLID = 1;
    static final int DECORATION_DOUBLE = 2;
    static final int DECORATION_DOTTED = 3;
    static final int DECORATION_DASHED = 4;
    static final int DECORATION_WAVY = 5;

    // Border and outline styles, as in driver/effects.go; 0 is solid.
    static final int BORDER_SOLID = 0;
    static final int BORDER_DASHED = 2;
    static final int BORDER_DOTTED = 3;

    // Gradient kinds, as in driver/gradient.go.
    static final int GRADIENT_LINEAR = 0;
    static final int GRADIENT_RADIAL = 1;
    static final int GRADIENT_CONIC = 2;

    // Vertical alignments of attachments, as in core/style.go; 0 and
    // Baseline are the baseline.
    static final int VALIGN_TEXT_TOP = 2;
    static final int VALIGN_TEXT_BOTTOM = 3;
    static final int VALIGN_MIDDLE = 4;

    private Protocol() {}

    static byte[] readBytes(DataInputStream in) throws IOException {
        byte[] bytes = new byte[in.readInt()];
        in.readFully(bytes);
        return bytes;
    }

    static String readString(DataInputStream in) throws IOException {
        byte[] bytes = new byte[in.readInt()];
        in.readFully(bytes);
        return new String(bytes, StandardCharsets.UTF_8);
    }

    static void writeString(DataOutputStream out, String s) throws IOException {
        byte[] bytes = s.getBytes(StandardCharsets.UTF_8);
        out.writeInt(bytes.length);
        out.write(bytes);
    }

    /** Whether bit i of a flags byte is set; Go writes the first flag lowest. */
    static boolean flag(int flags, int i) {
        return (flags & (1 << i)) != 0;
    }
}
