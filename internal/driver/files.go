package driver

import (
	"bytes"
	"io"
	"mime"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The system's file and folder pickers and save dialog, which NSH has
// not: the user picks files of any type or folders, which reach the app
// as paths on disk, or chooses where a file the app made goes. The
// platform answers each with EventFiles.

// FilesRequest is a file pick as the platform carries it out.
type FilesRequest struct {
	// Limit is the most files the user picks: 1 for a single pick, at
	// most MaxPicked.
	Limit int
	// Types are the files offered: lower-case media types
	// (application/pdf), families (image/*) and extensions (.zip). None
	// offers every file.
	Types []string
}

// File is a file the user picked.
type File struct {
	// Name is its name as the user saw it.
	Name string
	// MIME is its media type, application/octet-stream when nothing says.
	MIME string
	Size int64
	// Modified is when it last changed, zero when the platform does not
	// say.
	Modified time.Time
	// Path is where it is on disk, "" on the web, where Opener reads it.
	Path string
	// Temporary says Path is a copy the platform made in the app's own
	// storage, as iOS and Android do, which the app deletes once it is
	// done with it; elsewhere Path is the user's file.
	Temporary bool
	// Opener opens a file that has no Path.
	Opener func() (io.ReadCloser, error)
}

// Open opens the file for reading.
func (f File) Open() (io.ReadCloser, error) {
	if f.Opener != nil {
		return f.Opener()
	}
	return os.Open(f.Path)
}

// DirectoriesRequest is a folder pick as the platform carries it out.
type DirectoriesRequest struct {
	// Limit is the most folders the user picks: 1 for a single pick, at
	// most MaxPicked.
	Limit int
}

// Directory is a folder the user picked.
type Directory struct {
	// Name is its name as the user saw it.
	Name string
	// Path is where it is on disk.
	Path string
}

// NoDirectoryPicker is what a platform without a folder picker answers
// ItemPickDirectories with.
const NoDirectoryPicker = "this platform has no folder picker"

// SaveRequest is a save as the platform carries it out: the file the
// dialog suggests and what goes in it, the file at Path or else Data.
type SaveRequest struct {
	// Name is a file name without directories, never empty.
	Name string
	MIME string
	Path string
	Data []byte
}

// Open opens what r saves.
func (r SaveRequest) Open() (io.ReadCloser, error) {
	if r.Path != "" {
		return os.Open(r.Path)
	}
	return io.NopCloser(bytes.NewReader(r.Data)), nil
}

// SaveTo writes what r saves to path through a temporary file beside
// it, so a write that fails leaves path as it was.
func (r SaveRequest) SaveTo(path string) error {
	src, err := r.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, src)
	if err == nil {
		err = tmp.Chmod(0o644) // CreateTemp's file is the owner's alone
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// Stage returns a file named r.Name, in a new directory under dir, that
// holds what r saves, for platforms that save only files on disk under
// the name they show: a link to Path where the system allows one, else
// a copy. The caller removes the directory once the save ended.
func (r SaveRequest) Stage(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(dir, "save")
	if err != nil {
		return "", err
	}
	path := filepath.Join(tmp, r.Name)
	if r.Path != "" && os.Link(r.Path, path) == nil {
		return path, nil
	}
	if err := r.SaveTo(path); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return path, nil
}

// SharedFile is a file the app hands another app: one it opens in the
// app the system associates with it, or one of a share.
type SharedFile struct {
	// Path is where it is on disk, absolute once the core resolved it.
	Path string
	// Name is the file name the other app sees, Path's own unless the app
	// gave another; the core fills it in.
	Name string
	// MIME is its media type, which the core fills in from Name.
	MIME string
}

// ShareRequest is a share as the platform carries it out: files, text,
// or both, for the system's share sheet.
type ShareRequest struct {
	Files []SharedFile
	Text  string
	// From is the area the sheet points at where it shows as a popover,
	// as on macOS and iPads, in the root's points; zero for the root's
	// center.
	From Bounds
}

// What a platform that cannot open a file or share answers ItemOpenFile
// or ItemShare with.
const (
	NoFileOpener = "this platform opens no files in other apps"
	NoShareSheet = "this platform has no share sheet"
)

// ShareStageAge is how long a file Stage made stays before a later stage
// in the same directory removes it: long enough for the app the user
// chose to read it.
const ShareStageAge = time.Hour

// Stage returns a path to f under its Name, for platforms that hand
// files on by path and show a file's own name: Path itself when its name
// is Name, else a link to it, or a copy where the system allows none, in
// a new directory under dir. Directories a stage made over ShareStageAge
// ago go first.
func (f SharedFile) Stage(dir string) (string, error) {
	if filepath.Base(f.Path) == f.Name {
		return f.Path, nil
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if info, err := e.Info(); err == nil && e.IsDir() && time.Since(info.ModTime()) > ShareStageAge {
				os.RemoveAll(filepath.Join(dir, e.Name()))
			}
		}
	}
	return SaveRequest{Name: f.Name, Path: f.Path}.Stage(dir)
}

// FinishFiles returns files as the app gets them: at most r.Limit, each
// with a name, a type and, for one on disk, its size and modification
// time.
func FinishFiles(r FilesRequest, files []File) []File {
	if n := max(r.Limit, 1); len(files) > n {
		files = files[:n]
	}
	for i := range files {
		f := &files[i]
		if f.Name == "" {
			f.Name = filepath.Base(f.Path)
		}
		if f.Path != "" {
			if info, err := os.Stat(f.Path); err == nil {
				f.Size, f.Modified = info.Size(), info.ModTime()
			}
		}
		if f.MIME == "" {
			f.MIME = MIMEForFile(f.Name)
		}
	}
	return files
}

// FinishDirectories returns dirs as the app gets them: at most r.Limit,
// each with a name.
func FinishDirectories(r DirectoriesRequest, dirs []Directory) []Directory {
	if n := max(r.Limit, 1); len(dirs) > n {
		dirs = dirs[:n]
	}
	for i := range dirs {
		if d := &dirs[i]; d.Name == "" {
			d.Name = filepath.Base(d.Path)
		}
	}
	return dirs
}

// fileTypes are the media types of common extensions, so every platform
// names a file's type alike; mime's table, which differs by system,
// covers the rest.
var fileTypes = map[string]string{
	"7z": "application/x-7z-compressed", "aac": "audio/aac", "apk": "application/vnd.android.package-archive",
	"avi": "video/x-msvideo", "bz2": "application/x-bzip2", "csv": "text/csv", "doc": "application/msword",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"epub": "application/epub+zip", "flac": "audio/flac", "gz": "application/gzip", "htm": "text/html",
	"html": "text/html", "ics": "text/calendar", "js": "text/javascript", "json": "application/json",
	"key": "application/x-iwork-keynote-sffkey", "log": "text/plain", "m4a": "audio/mp4", "m4v": "video/mp4",
	"md": "text/markdown", "mkv": "video/x-matroska", "mov": "video/quicktime", "mp3": "audio/mpeg",
	"mp4": "video/mp4", "numbers": "application/x-iwork-numbers-sffnumbers", "odp": "application/vnd.oasis.opendocument.presentation",
	"ods": "application/vnd.oasis.opendocument.spreadsheet", "odt": "application/vnd.oasis.opendocument.text",
	"oga": "audio/ogg", "ogg": "audio/ogg", "ogv": "video/ogg", "opus": "audio/opus", "otf": "font/otf",
	"pages": "application/x-iwork-pages-sffpages", "pdf": "application/pdf", "ppt": "application/vnd.ms-powerpoint",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation", "rar": "application/vnd.rar",
	"rtf": "application/rtf", "svg": "image/svg+xml", "tar": "application/x-tar", "tgz": "application/gzip",
	"ttf": "font/ttf", "txt": "text/plain", "wav": "audio/wav", "weba": "audio/webm", "webm": "video/webm",
	"woff": "font/woff", "woff2": "font/woff2", "xls": "application/vnd.ms-excel",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xml": "application/xml",
	"xz": "application/x-xz", "yaml": "application/yaml", "yml": "application/yaml", "zip": "application/zip",
	"3gp": "video/3gpp",
}

// mimeForExtension returns the media type of ext (zip, without a dot),
// or "".
func mimeForExtension(ext string) string {
	ext = strings.ToLower(ext)
	if t := imageExtensions[ext]; t != "" {
		return t
	}
	if t := fileTypes[ext]; t != "" {
		return t
	}
	t, _, _ := strings.Cut(mime.TypeByExtension("."+ext), ";")
	return strings.TrimSpace(t)
}

// MIMEForFile returns the media type a file name's extension says,
// application/octet-stream for one it does not know.
func MIMEForFile(name string) string {
	if ext := strings.TrimPrefix(filepath.Ext(name), "."); ext != "" {
		if t := mimeForExtension(ext); t != "" {
			return t
		}
	}
	return "application/octet-stream"
}

// MIMETypes returns r's types as media types, for pickers that filter
// only by them: an extension becomes its type, and any reports that one
// has none, so the picker must offer every file.
func (r FilesRequest) MIMETypes() (types []string, any bool) {
	for _, t := range r.Types {
		if ext, ok := strings.CutPrefix(t, "."); ok {
			if t = mimeForExtension(ext); t == "" {
				return nil, true
			}
		}
		if !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	return types, len(types) == 0
}

// Extensions returns r's types as extensions without dots, for dialogs
// that filter only by them: a media type becomes the extensions of its
// files, and any reports that one has none, so the dialog must offer
// every file.
func (r FilesRequest) Extensions() (exts []string, any bool) {
	add := func(e string) {
		if !slices.Contains(exts, e) {
			exts = append(exts, e)
		}
	}
	for _, t := range r.Types {
		if ext, ok := strings.CutPrefix(t, "."); ok {
			add(ext)
			continue
		}
		n := len(exts)
		family, wild := strings.CutSuffix(t, "/*")
		for _, table := range []map[string]string{imageExtensions, fileTypes} {
			for ext, m := range table {
				if m == t || wild && strings.HasPrefix(m, family+"/") {
					add(ext)
				}
			}
		}
		if !wild {
			sys, _ := mime.ExtensionsByType(t)
			for _, e := range sys {
				add(strings.TrimPrefix(e, "."))
			}
		}
		if len(exts) == n {
			return nil, true
		}
	}
	slices.Sort(exts)
	return exts, len(exts) == 0
}
