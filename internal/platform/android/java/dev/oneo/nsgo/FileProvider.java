package dev.oneo.nsgo;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.content.Context;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import java.io.File;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.util.List;

/**
 * Serves the app's files to the apps it hands them to: the image it put
 * on the clipboard, a file it opens in another app and the files it
 * shares; and lets the camera app write the photo it takes into the
 * file of the cache's nsgo-camera made for it. The manifest declares it unexported with grantUriPermissions,
 * so only an app granted a URI reads its file, and names its authority
 * after the app's package.
 *
 * <p>A URI names a root, the path under it and the name the other app
 * sees, with the file's type as a parameter. The roots are the app's
 * data directory, which holds its files and cache, and its external
 * one, so the provider serves nothing outside the app's own storage.
 */
public final class FileProvider extends ContentProvider {
    static String authority(Context context) {
        return context.getPackageName() + ".nsgo.files";
    }

    /** The directories files are served from, by a URI's first segment; null for one the device has not. */
    private static File[] roots(Context context) {
        File external = context.getExternalFilesDir(null);
        return new File[] {context.getDataDir(), external != null ? external.getParentFile() : null};
    }

    /**
     * Returns the URI another app reads file through under name, of type
     * mime; it fails for a file outside the app's storage.
     */
    static Uri uri(Context context, File file, String name, String mime) throws IOException {
        String path = file.getCanonicalPath();
        File[] roots = roots(context);
        for (int i = 0; i < roots.length; i++) {
            if (roots[i] == null) continue;
            String root = roots[i].getCanonicalPath() + File.separator;
            if (path.startsWith(root)) {
                return new Uri.Builder().scheme("content").authority(authority(context))
                        .appendPath(String.valueOf(i)).appendPath(path.substring(root.length())).appendPath(name)
                        .appendQueryParameter("type", mime).build();
            }
        }
        throw new IOException(file + " is not in the app's storage, which other apps are handed files from");
    }

    @Override
    public boolean onCreate() {
        return true;
    }

    /** The file uri names, which must still be under its root. */
    private File file(Uri uri) throws FileNotFoundException {
        List<String> segments = uri.getPathSegments();
        File[] roots = roots(getContext());
        try {
            int i = Integer.parseInt(segments.get(0));
            if (segments.size() == 3 && i >= 0 && i < roots.length && roots[i] != null) {
                String root = roots[i].getCanonicalPath() + File.separator;
                File f = new File(root, segments.get(1)).getCanonicalFile();
                if (f.getPath().startsWith(root) && f.isFile()) return f;
            }
        } catch (IOException | RuntimeException e) {
            // Not a URI this provider made.
        }
        throw new FileNotFoundException(String.valueOf(uri));
    }

    @Override
    public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        File f = file(uri);
        if (mode.contains("w")) {
            if (!photo(f)) throw new FileNotFoundException(uri + " is not writable");
            return ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_WRITE_ONLY
                    | ParcelFileDescriptor.MODE_TRUNCATE);
        }
        return ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_READ_ONLY);
    }

    /** Whether f is a file Services.startPhoto made for the camera to write. */
    private boolean photo(File f) {
        try {
            String dir = new File(getContext().getCacheDir(), Services.CAMERA_DIR).getCanonicalPath() + File.separator;
            return f.getCanonicalPath().startsWith(dir);
        } catch (IOException e) {
            return false;
        }
    }

    @Override
    public String getType(Uri uri) {
        String type = uri.getQueryParameter("type");
        return type == null || type.isEmpty() ? "application/octet-stream" : type;
    }

    @Override
    public Cursor query(Uri uri, String[] projection, String selection, String[] args, String order) {
        File f;
        try {
            f = file(uri);
        } catch (FileNotFoundException e) {
            return null;
        }
        if (projection == null) projection = new String[] {OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE};
        // Only the columns asked for that the provider knows, as apps
        // that ask for more expect.
        String[] columns = new String[projection.length];
        Object[] row = new Object[projection.length];
        int n = 0;
        for (String c : projection) {
            if (OpenableColumns.DISPLAY_NAME.equals(c)) {
                columns[n] = c;
                row[n++] = uri.getLastPathSegment();
            } else if (OpenableColumns.SIZE.equals(c)) {
                columns[n] = c;
                row[n++] = f.length();
            }
        }
        MatrixCursor cursor = new MatrixCursor(java.util.Arrays.copyOf(columns, n), 1);
        cursor.addRow(java.util.Arrays.copyOf(row, n));
        return cursor;
    }

    @Override
    public Uri insert(Uri uri, ContentValues values) {
        return null;
    }

    @Override
    public int delete(Uri uri, String selection, String[] args) {
        return 0;
    }

    @Override
    public int update(Uri uri, ContentValues values, String selection, String[] args) {
        return 0;
    }
}
