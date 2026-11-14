package dev.oneo.nsgo;

import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.ClipData;
import android.content.ClipDescription;
import android.content.ClipboardManager;
import android.content.ContentResolver;
import android.content.Context;
import android.content.Intent;
import android.database.Cursor;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.ImageDecoder;
import android.media.ExifInterface;
import android.net.Uri;
import android.os.Build;
import android.os.ext.SdkExtensions;
import android.provider.DocumentsContract;
import android.provider.MediaStore;
import android.provider.OpenableColumns;
import android.provider.Settings;
import android.util.Base64;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.DataOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.ByteBuffer;
import java.nio.channels.FileChannel;
import java.util.ArrayList;
import java.util.List;

/**
 * The services NsgoView performs for the app beyond its views: the
 * clipboard's images and files, opening URLs, the photo picker, whose
 * images it prepares as Go's driver.PickRequest says, the documents UI's
 * file and folder picks and saves, reading the folders picked for Go, and
 * opening and sharing the app's files, which FileProvider serves.
 */
final class Services {
    private Services() {}

    // What a clipboard read asks for, as driver.ClipboardRead.
    static final int CLIPBOARD_TEXT = 0;
    static final int CLIPBOARD_HAS_TEXT = 1;
    static final int CLIPBOARD_IMAGE = 2;
    static final int CLIPBOARD_HAS_IMAGE = 3;
    static final int CLIPBOARD_FILES = 4;

    /** The largest PNG a clipboard read answers with, as driver.MaxClipboardImage. */
    private static final int MAX_CLIPBOARD_IMAGE = 48 << 20;

    /** The request codes of the photo picker's, the file picker's, the save dialog's and the folder picker's activities. */
    static final int PICK_REQUEST = 0x6e73, FILES_REQUEST = 0x6e74, SAVE_REQUEST = 0x6e75, FOLDER_REQUEST = 0x6e76;

    /** The request codes of the camera's activity and of the camera permission's prompt. */
    static final int PHOTO_REQUEST = 0x6e77, CAMERA_PERMISSION_REQUEST = 0x6e78;

    /** The directory in the app's cache the camera writes photos to, which FileProvider lets it write. */
    static final String CAMERA_DIR = "nsgo-camera";

    static final String CAMERA_PERMISSION = "android.permission.CAMERA";

    /** The directory in the app's cache picked files are copied to, as Go's File.Temporary says. */
    static final String FILES_DIR = "nsgo-files";

    /** The directory in the app's cache the clipboard's image is kept in. */
    static final String CLIP_DIR = "nsgo-clip";

    static byte[] read(ContentResolver resolver, Uri uri) throws IOException {
        try (InputStream s = resolver.openInputStream(uri)) {
            if (s == null) throw new IOException("no stream for " + uri);
            ByteArrayOutputStream b = new ByteArrayOutputStream();
            byte[] buf = new byte[65536];
            for (int n; (n = s.read(buf)) > 0; ) b.write(buf, 0, n);
            return b.toByteArray();
        }
    }

    /**
     * Answers a clipboard read: the text, or in data "image", the PNG's
     * size and its base64 ("toolarge" and its bytes past the limit, "error"
     * and why for an image it cannot read), or "uri" lines of local files' file: URLs; and
     * in finished whether it has text or an image. Asking whether it has
     * one reads only the clip's description, so Android shows no paste
     * notice.
     */
    static void readClipboard(Context context, ClipboardManager cm, int what, String[] text, String[] data,
            boolean[] has) {
        ClipDescription d = cm == null ? null : cm.getPrimaryClipDescription();
        switch (what) {
            case CLIPBOARD_HAS_TEXT:
                has[0] = d != null && (d.hasMimeType("text/*") || d.hasMimeType(ClipDescription.MIMETYPE_TEXT_URILIST));
                return;
            case CLIPBOARD_HAS_IMAGE:
                has[0] = d != null && d.hasMimeType("image/*");
                return;
        }
        ClipData clip = cm == null ? null : cm.getPrimaryClip();
        if (clip == null) return;
        StringBuilder out = new StringBuilder();
        for (int i = 0; i < clip.getItemCount(); i++) {
            ClipData.Item item = clip.getItemAt(i);
            Uri uri = item.getUri();
            if (what == CLIPBOARD_TEXT) {
                CharSequence t = item.coerceToText(context);
                if (t != null) text[0] = t.toString();
                return;
            }
            if (what == CLIPBOARD_FILES && uri != null && "file".equals(uri.getScheme())) {
                out.append("uri\t").append(uri).append('\n');
            }
            if (what == CLIPBOARD_IMAGE && uri != null) {
                String mime = context.getContentResolver().getType(uri);
                if (mime == null || !mime.startsWith("image/")) continue;
                try {
                    byte[] bytes = read(context.getContentResolver(), uri);
                    Bitmap bitmap = BitmapFactory.decodeByteArray(bytes, 0, bytes.length);
                    if (bitmap == null) {
                        out.setLength(0);
                        out.append("error\tno decoder reads its ").append(mime);
                        continue;
                    }
                    ByteArrayOutputStream png = new ByteArrayOutputStream();
                    bitmap.compress(Bitmap.CompressFormat.PNG, 100, png);
                    out.setLength(0);
                    if (png.size() > MAX_CLIPBOARD_IMAGE) {
                        out.append("toolarge\t").append(png.size());
                        break;
                    }
                    out.append("image\t").append(bitmap.getWidth()).append('\t').append(bitmap.getHeight()).append('\t')
                            .append(Base64.encodeToString(png.toByteArray(), Base64.NO_WRAP));
                    break;
                } catch (IOException | SecurityException e) {
                    // An image the app may not read fails its read, unless a later item reads.
                    out.setLength(0);
                    out.append("error\t").append(String.valueOf(e.getMessage()).replace('\n', ' '));
                }
            }
        }
        data[0] = out.toString();
    }

    /**
     * Puts the image bytes encode on the clipboard: a PNG in the app's
     * cache that FileProvider serves, which the clipboard lets the app
     * pasting it read.
     */
    static void setClipboardImage(Context context, ClipboardManager cm, byte[] bytes) {
        Bitmap bitmap = BitmapFactory.decodeByteArray(bytes, 0, bytes.length);
        if (bitmap == null || cm == null) return;
        File dir = new File(context.getCacheDir(), CLIP_DIR);
        dir.mkdirs();
        File[] old = dir.listFiles();
        if (old != null) for (File f : old) f.delete();
        File file = new File(dir, "image-" + System.currentTimeMillis() + ".png");
        try (FileOutputStream s = new FileOutputStream(file)) {
            bitmap.compress(Bitmap.CompressFormat.PNG, 100, s);
        } catch (IOException e) {
            return;
        }
        Uri uri;
        try {
            uri = FileProvider.uri(context, file, file.getName(), "image/png");
        } catch (IOException e) {
            return;
        }
        cm.setPrimaryClip(ClipData.newUri(context.getContentResolver(), "Image", uri));
    }

    /** Opens url in the app that handles it; false when none does. */
    static boolean openURL(Context context, String url) {
        Intent intent = new Intent(Intent.ACTION_VIEW, Uri.parse(url));
        if (!(context instanceof Activity)) intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        try {
            context.startActivity(intent);
            return true;
        } catch (ActivityNotFoundException | SecurityException e) {
            return false;
        }
    }

    /**
     * Opens the file at path in the app the system associates with mime,
     * which reads it as name through a URI granted to it; false when no
     * app opens it. error says what failed otherwise.
     */
    static boolean openFile(Context context, String path, String name, String mime, String[] error) {
        Intent intent = new Intent(Intent.ACTION_VIEW);
        try {
            intent.setDataAndType(FileProvider.uri(context, new File(path), name, mime), mime);
        } catch (IOException e) {
            error[0] = e.getMessage();
            return false;
        }
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        if (!(context instanceof Activity)) intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        try {
            context.startActivity(intent);
            return true;
        } catch (ActivityNotFoundException e) {
            return false;
        } catch (RuntimeException e) {
            error[0] = String.valueOf(e.getMessage());
            return false;
        }
    }

    /**
     * Shows the system's share sheet for text and the files at paths,
     * which the app chosen reads under names through URIs granted to it;
     * it returns what failed, "" once the sheet shows.
     */
    static String share(Context context, String text, String[] paths, String[] names, String[] mimes) {
        ArrayList<Uri> uris = new ArrayList<>();
        try {
            for (int i = 0; i < paths.length; i++) {
                uris.add(FileProvider.uri(context, new File(paths[i]), names[i], mimes[i]));
            }
        } catch (IOException e) {
            return e.getMessage();
        }
        Intent intent = new Intent(uris.size() > 1 ? Intent.ACTION_SEND_MULTIPLE : Intent.ACTION_SEND);
        intent.setType(uris.isEmpty() ? "text/plain" : commonType(mimes));
        if (!text.isEmpty()) intent.putExtra(Intent.EXTRA_TEXT, text);
        if (uris.size() == 1) intent.putExtra(Intent.EXTRA_STREAM, uris.get(0));
        else if (uris.size() > 1) intent.putParcelableArrayListExtra(Intent.EXTRA_STREAM, uris);
        if (!uris.isEmpty()) {
            // The clip carries the grants through the chooser to the app
            // chosen, and the sheet's preview of the files.
            ClipData clip = new ClipData(names[0], mimes, new ClipData.Item(uris.get(0)));
            for (int i = 1; i < uris.size(); i++) clip.addItem(new ClipData.Item(uris.get(i)));
            intent.setClipData(clip);
            intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        }
        Intent chooser = Intent.createChooser(intent, null);
        if (!(context instanceof Activity)) chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        try {
            context.startActivity(chooser);
            return "";
        } catch (RuntimeException e) {
            return String.valueOf(e.getMessage());
        }
    }

    /** The permission an app installing packages asks for in its manifest. */
    private static final String INSTALL_PERMISSION = "android.permission.REQUEST_INSTALL_PACKAGES";

    /**
     * Hands the APK at path to the package installer, which reads it as
     * name through a URI granted to it and asks the user to confirm; it
     * returns what failed, "" once the installer starts.
     */
    static String installPackage(Context context, String path, String name) {
        if (!asksFor(context, INSTALL_PERMISSION)) {
            return "the app's manifest does not ask for " + INSTALL_PERMISSION
                    + ": add it to nsgo.yaml's android.permissions, or an embedding app's manifest";
        }
        String mime = "application/vnd.android.package-archive";
        Intent intent = new Intent(Intent.ACTION_VIEW);
        try {
            intent.setDataAndType(FileProvider.uri(context, new File(path), name, mime), mime);
        } catch (IOException e) {
            return e.getMessage();
        }
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        if (!(context instanceof Activity)) intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        try {
            context.startActivity(intent);
            return "";
        } catch (RuntimeException e) {
            return String.valueOf(e.getMessage());
        }
    }

    /** Whether the app's manifest asks for permission. */
    private static boolean asksFor(Context context, String permission) {
        try {
            String[] asked = context.getPackageManager()
                    .getPackageInfo(context.getPackageName(), android.content.pm.PackageManager.GET_PERMISSIONS)
                    .requestedPermissions;
            if (asked != null) for (String p : asked) if (permission.equals(p)) return true;
        } catch (android.content.pm.PackageManager.NameNotFoundException e) {
            // The app's own package is always there.
        }
        return false;
    }

    /** Opens the setting that lets the app install packages; false when it cannot. */
    static boolean installSettings(Context context) {
        Intent intent = new Intent(android.provider.Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                Uri.parse("package:" + context.getPackageName()));
        if (!(context instanceof Activity)) intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        try {
            context.startActivity(intent);
            return true;
        } catch (RuntimeException e) {
            return false;
        }
    }

    /** Whether Android lets this package draw windows above other apps. */
    static boolean canDrawOverlays(Context context) {
        return Build.VERSION.SDK_INT < 23 || Settings.canDrawOverlays(context);
    }

    /** Opens the setting that lets this package draw windows above other apps. */
    static boolean overlaySettings(Context context) {
        Intent intent = new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                Uri.parse("package:" + context.getPackageName()));
        if (!(context instanceof Activity)) intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        try {
            context.startActivity(intent);
            return true;
        } catch (RuntimeException e) {
            return false;
        }
    }

    /** The type a share of files of types says: theirs, their family's (image/*) or any. */
    private static String commonType(String[] types) {
        String type = types[0];
        for (String t : types) {
            if (t.equals(type)) continue;
            String family = type.substring(0, type.indexOf('/') + 1);
            if (family.isEmpty() || !t.startsWith(family)) return "*/*";
            type = family + "*";
        }
        return type;
    }

    /** A pick the photo picker answers: Go's request and how to prepare its images. */
    static final class Pick {
        final long count;
        final int limit, maxDimension;
        final float quality;
        final boolean current;

        Pick(long count, int limit, int maxDimension, float quality, boolean current) {
            this.count = count;
            this.limit = Math.max(limit, 1);
            this.maxDimension = maxDimension;
            this.quality = quality;
            this.current = current;
        }
    }

    /**
     * Starts the system's photo picker, which needs no permission: the
     * Photo Picker where the system has it, else the documents UI for
     * images. It reports false when no activity can pick.
     */
    static boolean startPick(Activity activity, Pick p) {
        Intent intent;
        boolean photoPicker = Build.VERSION.SDK_INT >= 33
                || Build.VERSION.SDK_INT >= 30 && SdkExtensions.getExtensionVersion(Build.VERSION_CODES.R) >= 2;
        if (photoPicker) {
            intent = new Intent(MediaStore.ACTION_PICK_IMAGES);
            if (p.limit > 1) intent.putExtra(MediaStore.EXTRA_PICK_IMAGES_MAX, Math.min(p.limit, MediaStore.getPickImagesMaxLimit()));
        } else {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            intent.setType("image/*");
            intent.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, p.limit > 1);
        }
        try {
            activity.startActivityForResult(intent, PICK_REQUEST);
            return true;
        } catch (ActivityNotFoundException e) {
            return false;
        }
    }

    /** Whether the device has a camera an app takes photos with. */
    static boolean hasCamera(Context context) {
        return context.getPackageManager().hasSystemFeature(android.content.pm.PackageManager.FEATURE_CAMERA_ANY);
    }

    /**
     * Whether the camera app must wait for the user to allow the camera:
     * an app whose manifest asks for CAMERA, and has not been granted it,
     * cannot start one. One that does not ask needs no permission.
     */
    static boolean needsCameraPermission(Context context) {
        if (context.checkSelfPermission(CAMERA_PERMISSION) == android.content.pm.PackageManager.PERMISSION_GRANTED) {
            return false;
        }
        try {
            String[] asked = context.getPackageManager()
                    .getPackageInfo(context.getPackageName(), android.content.pm.PackageManager.GET_PERMISSIONS)
                    .requestedPermissions;
            return asked != null && java.util.Arrays.asList(asked).contains(CAMERA_PERMISSION);
        } catch (android.content.pm.PackageManager.NameNotFoundException e) {
            return false;
        }
    }

    /**
     * Starts the camera app to take one photo into a new file of the
     * cache's nsgo-camera, which it writes through FileProvider, and
     * returns the file; it fails when no activity takes photos.
     */
    static File startPhoto(Activity activity) throws IOException {
        File dir = new File(activity.getCacheDir(), CAMERA_DIR);
        if (!dir.isDirectory() && !dir.mkdirs()) throw new IOException("cannot make " + dir);
        String name = new java.text.SimpleDateFormat("'IMG_'yyyyMMdd_HHmmss", java.util.Locale.ROOT)
                .format(new java.util.Date());
        File photo = new File(dir, name + ".jpg");
        for (int n = 2; !photo.createNewFile(); n++) photo = new File(dir, name + "_" + n + ".jpg");
        Uri uri = photoURI(activity, photo);
        Intent intent = new Intent(MediaStore.ACTION_IMAGE_CAPTURE);
        intent.putExtra(MediaStore.EXTRA_OUTPUT, uri);
        // The clip carries the grant to camera apps that read it from there.
        intent.setClipData(ClipData.newRawUri("", uri));
        intent.addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION | Intent.FLAG_GRANT_READ_URI_PERMISSION);
        try {
            activity.startActivityForResult(intent, PHOTO_REQUEST);
        } catch (ActivityNotFoundException | SecurityException e) {
            photo.delete();
            throw new IOException("no camera app takes photos", e);
        }
        return photo;
    }

    /** The URI the camera app writes photo through, and the app reads it back from. */
    static Uri photoURI(Context context, File photo) throws IOException {
        return FileProvider.uri(context, photo, photo.getName(), "image/jpeg");
    }

    /** The images a picker's result names, at most the pick's limit. */
    static List<Uri> picked(Intent result, Pick p) {
        return picked(result, p.limit);
    }

    /** The documents a picker's result names, at most limit. */
    static List<Uri> picked(Intent result, int limit) {
        List<Uri> uris = new ArrayList<>();
        if (result == null) return uris;
        ClipData clip = result.getClipData();
        if (clip != null) {
            for (int i = 0; i < clip.getItemCount() && uris.size() < limit; i++) {
                if (clip.getItemAt(i).getUri() != null) uris.add(clip.getItemAt(i).getUri());
            }
        } else if (result.getData() != null) {
            uris.add(result.getData());
        }
        return uris;
    }

    /** A file or folder pick or a save the documents UI answers: Go's request number and what it asked for. */
    static final class Documents {
        final long count;
        final int limit;
        /** The file a save copies, null for a pick. */
        final String from;
        /** Whether it picks a folder. */
        final boolean folder;

        Documents(long count, int limit, String from, boolean folder) {
            this.count = count;
            this.limit = Math.max(limit, 1);
            this.from = from;
            this.folder = folder;
        }

        /** The request code of the activity that answers it. */
        int request() {
            return folder ? FOLDER_REQUEST : from == null ? FILES_REQUEST : SAVE_REQUEST;
        }
    }

    /**
     * Starts the documents UI for the files of the media types given, or
     * any file for none; it needs no permission. It reports false when no
     * activity can pick.
     */
    static boolean startFiles(Activity activity, int limit, String[] types) {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.setType(types.length == 1 ? types[0] : "*/*");
        if (types.length > 1) intent.putExtra(Intent.EXTRA_MIME_TYPES, types);
        intent.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, limit > 1);
        return start(activity, intent, FILES_REQUEST);
    }

    /** Starts the documents UI to pick a folder, which grants its tree, not a path. */
    static boolean startFolder(Activity activity) {
        return start(activity, new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE), FOLDER_REQUEST);
    }

    /**
     * The folder picked as Go's readFolders reads it: a line of its name,
     * the tree's URI and its root document's ID.
     */
    static String pickedTree(Context context, Uri tree) {
        String root = DocumentsContract.getTreeDocumentId(tree);
        String name = displayName(context.getContentResolver(), DocumentsContract.buildDocumentUriUsingTree(tree, root));
        return name.replace('\t', ' ').replace('\n', ' ') + '\t' + tree + '\t' + root + '\n';
    }

    /**
     * Lists the folder document of a tree as Go's tree.List reads it: the
     * count, then each child's name, document ID, whether it is a folder,
     * size (-1 when its provider does not say) and modification time in
     * milliseconds (0: unknown).
     */
    static void listFolder(Context context, String tree, String document, DataOutputStream out) throws IOException {
        Uri children = DocumentsContract.buildChildDocumentsUriUsingTree(Uri.parse(tree), document);
        String[] columns = {DocumentsContract.Document.COLUMN_DISPLAY_NAME, DocumentsContract.Document.COLUMN_DOCUMENT_ID,
                DocumentsContract.Document.COLUMN_MIME_TYPE, DocumentsContract.Document.COLUMN_SIZE,
                DocumentsContract.Document.COLUMN_LAST_MODIFIED};
        try (Cursor c = context.getContentResolver().query(children, columns, null, null, null)) {
            if (c == null) throw new IOException("no listing for " + children);
            out.writeInt(c.getCount());
            while (c.moveToNext()) {
                Protocol.writeString(out, c.isNull(0) ? "" : c.getString(0));
                Protocol.writeString(out, c.getString(1));
                out.writeByte(DocumentsContract.Document.MIME_TYPE_DIR.equals(c.getString(2)) ? 1 : 0);
                out.writeLong(c.isNull(3) ? -1 : c.getLong(3));
                out.writeLong(c.isNull(4) ? 0 : c.getLong(4));
            }
        } catch (RuntimeException e) {
            throw new IOException(e.getMessage(), e);
        }
    }

    /**
     * Reads at most n bytes of a tree's document from off, fewer only at
     * its end: in place where its provider gives a file, else skipping
     * through its stream.
     */
    static byte[] readDocument(Context context, String tree, String document, long off, int n) throws IOException {
        ContentResolver resolver = context.getContentResolver();
        Uri uri = DocumentsContract.buildDocumentUriUsingTree(Uri.parse(tree), document);
        ByteBuffer buf = ByteBuffer.allocate(n);
        try {
            try (android.os.ParcelFileDescriptor fd = resolver.openFileDescriptor(uri, "r");
                    FileChannel ch = new FileInputStream(fd.getFileDescriptor()).getChannel()) {
                while (buf.hasRemaining() && ch.read(buf, off + buf.position()) > 0) {}
            } catch (IOException e) {
                // A pipe, which cannot read in place.
                buf.clear();
                try (InputStream in = resolver.openInputStream(uri)) {
                    if (in == null) throw new IOException("no stream for " + uri);
                    for (long skip = off; skip > 0; ) {
                        long s = in.skip(skip);
                        if (s <= 0) return new byte[0];
                        skip -= s;
                    }
                    for (int r; buf.hasRemaining() && (r = in.read(buf.array(), buf.position(), buf.remaining())) > 0; ) {
                        buf.position(buf.position() + r);
                    }
                }
            }
        } catch (RuntimeException e) {
            throw new IOException(e.getMessage(), e);
        }
        return java.util.Arrays.copyOf(buf.array(), buf.position());
    }

    /** Starts the documents UI to create a document named name of type mime. */
    static boolean startSave(Activity activity, String name, String mime) {
        Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.setType(mime.isEmpty() ? "application/octet-stream" : mime);
        intent.putExtra(Intent.EXTRA_TITLE, name);
        return start(activity, intent, SAVE_REQUEST);
    }

    private static boolean start(Activity activity, Intent intent, int request) {
        try {
            activity.startActivityForResult(intent, request);
            return true;
        } catch (ActivityNotFoundException e) {
            return false;
        }
    }

    /**
     * Copies the documents picked into a new directory of the cache, which
     * the Go process reads, streaming each: lines of the name, the MIME
     * type and the copy's path. A document that cannot be read removes
     * the copies and fails the pick.
     */
    static String copyPicked(Context context, List<Uri> uris) throws IOException {
        ContentResolver resolver = context.getContentResolver();
        File base = new File(context.getCacheDir(), FILES_DIR);
        if (!base.isDirectory() && !base.mkdirs()) throw new IOException("cannot make " + base);
        File dir = java.nio.file.Files.createTempDirectory(base.toPath(), "pick").toFile();
        StringBuilder out = new StringBuilder();
        try {
            for (Uri uri : uris) {
                String name = displayName(resolver, uri);
                File to = new File(dir, name);
                for (int n = 2; to.exists(); n++) to = new File(dir, n + " " + name);
                try (InputStream in = resolver.openInputStream(uri); OutputStream o = new FileOutputStream(to)) {
                    if (in == null) throw new IOException("no stream for " + uri);
                    copy(in, o);
                }
                String mime = resolver.getType(uri);
                out.append(name.replace('\t', ' ').replace('\n', ' ')).append('\t')
                        .append(mime == null ? "" : mime.toLowerCase()).append('\t').append(to.getAbsolutePath()).append('\n');
            }
        } catch (IOException | RuntimeException e) {
            remove(dir);
            throw e instanceof IOException ? (IOException) e : new IOException(e.getMessage(), e);
        }
        return out.toString();
    }

    /** A document's name as the user saw it, safe as a file name. */
    private static String displayName(ContentResolver resolver, Uri uri) {
        String name = null;
        try (Cursor c = resolver.query(uri, new String[] {OpenableColumns.DISPLAY_NAME}, null, null, null)) {
            if (c != null && c.moveToFirst() && !c.isNull(0)) name = c.getString(0);
        } catch (RuntimeException e) {
            // A provider without the column leaves the URI's last segment.
        }
        if (name == null) name = uri.getLastPathSegment();
        if (name != null) name = name.replace('/', '_').replace('\0', '_').trim();
        return name == null || name.isEmpty() || name.equals(".") || name.equals("..") ? "file" : name;
    }

    /**
     * Copies the file at from into the document the user created,
     * streaming it; a copy that fails deletes the document, which holds
     * only part of the file.
     */
    static void save(Context context, String from, Uri to) throws IOException {
        ContentResolver resolver = context.getContentResolver();
        try (InputStream in = new java.io.FileInputStream(from); OutputStream o = resolver.openOutputStream(to, "w")) {
            if (o == null) throw new IOException("no stream for " + to);
            copy(in, o);
        } catch (IOException | RuntimeException e) {
            try {
                android.provider.DocumentsContract.deleteDocument(resolver, to);
            } catch (Exception ignored) {
                // The provider keeps it; nothing more to do.
            }
            throw e instanceof IOException ? (IOException) e : new IOException(e.getMessage(), e);
        }
    }

    private static void copy(InputStream in, OutputStream out) throws IOException {
        byte[] buf = new byte[1 << 16];
        for (int n; (n = in.read(buf)) > 0; ) out.write(buf, 0, n);
    }

    private static void remove(File f) {
        File[] children = f.listFiles();
        if (children != null) for (File c : children) remove(c);
        f.delete();
    }

    /**
     * Reads and prepares the images picked as the pick asks, as Go's
     * PickRequest.Encoding decides: lines of the name, type, width,
     * height, modification time in milliseconds (0: unknown) and bytes in
     * base64.
     */
    static String prepare(Context context, List<Uri> uris, Pick p) {
        ContentResolver resolver = context.getContentResolver();
        StringBuilder out = new StringBuilder();
        for (Uri uri : uris) {
            try {
                byte[] bytes = read(resolver, uri);
                String mime = resolver.getType(uri);
                if (mime == null) mime = "";
                mime = mime.toLowerCase();
                String name = "image";
                long modified = 0;
                try (Cursor c = resolver.query(uri, null, null, null, null)) {
                    if (c != null && c.moveToFirst()) {
                        int n = c.getColumnIndex(OpenableColumns.DISPLAY_NAME);
                        if (n >= 0 && c.getString(n) != null) name = c.getString(n);
                        int m = c.getColumnIndex(MediaStore.MediaColumns.DATE_MODIFIED);
                        if (m >= 0) modified = c.getLong(m) * 1000;
                    }
                } catch (RuntimeException e) {
                    // A provider without these columns leaves the defaults.
                }
                BitmapFactory.Options bounds = new BitmapFactory.Options();
                bounds.inJustDecodeBounds = true;
                BitmapFactory.decodeByteArray(bytes, 0, bytes.length, bounds);
                int w = bounds.outWidth, h = bounds.outHeight;
                if (turned(bytes)) {
                    int t = w;
                    w = h;
                    h = t;
                }
                boolean png = mime.equals("image/png"), jpeg = mime.equals("image/jpeg");
                boolean foreign = !(jpeg || png || mime.equals("image/gif") || mime.equals("image/webp"));
                boolean scale = p.maxDimension > 0 && Math.max(w, h) > p.maxDimension;
                boolean encode = scale || !p.current && foreign || p.quality > 0 && jpeg;
                if (encode) {
                    final int max = p.maxDimension;
                    Bitmap bitmap = ImageDecoder.decodeBitmap(ImageDecoder.createSource(ByteBuffer.wrap(bytes)),
                            (decoder, info, source) -> {
                                decoder.setAllocator(ImageDecoder.ALLOCATOR_SOFTWARE);
                                int iw = info.getSize().getWidth(), ih = info.getSize().getHeight();
                                int longer = Math.max(iw, ih);
                                if (max > 0 && longer > max) {
                                    decoder.setTargetSize(Math.max(1, Math.round((float) iw * max / longer)),
                                            Math.max(1, Math.round((float) ih * max / longer)));
                                }
                            });
                    ByteArrayOutputStream b = new ByteArrayOutputStream();
                    boolean asPNG = png && scale;
                    float quality = p.quality > 0 ? Math.min(p.quality, 1f) : 0.9f;
                    bitmap.compress(asPNG ? Bitmap.CompressFormat.PNG : Bitmap.CompressFormat.JPEG,
                            Math.round(quality * 100), b);
                    bytes = b.toByteArray();
                    mime = asPNG ? "image/png" : "image/jpeg";
                    w = bitmap.getWidth();
                    h = bitmap.getHeight();
                }
                out.append(name.replace('\t', ' ').replace('\n', ' ')).append('\t').append(mime).append('\t')
                        .append(w).append('\t').append(h).append('\t').append(modified).append('\t')
                        .append(Base64.encodeToString(bytes, Base64.NO_WRAP)).append('\n');
            } catch (IOException | RuntimeException e) {
                // An image that cannot be read or decoded is left out.
            }
        }
        return out.toString();
    }

    /** Whether an image's EXIF orientation turns it a quarter. */
    private static boolean turned(byte[] bytes) {
        try {
            int o = new ExifInterface(new ByteArrayInputStream(bytes))
                    .getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL);
            return o >= ExifInterface.ORIENTATION_TRANSPOSE && o <= ExifInterface.ORIENTATION_ROTATE_270;
        } catch (IOException | RuntimeException e) {
            return false;
        }
    }
}
