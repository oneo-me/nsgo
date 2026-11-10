package imageload

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The disk cache keeps the bytes of every http(s) image on disk, so an
// app opened again shows what it showed without a download, and offline,
// as NSH's image/cache.rs does. Its policy is RFC 9111's for a private
// cache, simplified:
//
//   - 200 and 203 responses are kept unless no-store, Vary: * or larger
//     than an eighth of the cache.
//   - A copy is fresh for max-age, else Expires minus Date, else a tenth
//     of the time since Last-Modified, at most 7 days, less its age; with
//     no-cache or no freshness it is stale at once.
//   - A stale copy is revalidated with its ETag and Last-Modified: 304
//     refreshes its headers, 4xx drops it, and a failed request or a 5xx
//     serves it unless it said must-revalidate or no-cache.
//
// Entries are keyed by URL alone, since sources carry no request
// headers. Each is one file named by a digest of its URL, written under
// a temporary name and renamed, so readers on any goroutine or process
// see a whole entry or none; one that does not read back whole is a miss
// and is deleted. The least recently used go first: a hit refreshes the
// file's time at most every 10 minutes, and past the limit a scan
// deletes the oldest down to three quarters of it, and temporary files
// over an hour old.

// DiskCacheBytes bounds the disk cache.
const DiskCacheBytes = 256 << 20

const (
	maxHeuristic = 7 * 24 * time.Hour
	touchEvery   = 10 * time.Minute
	staleTemp    = time.Hour
	entryMagic   = "NSGOIMG1"
)

// The headers an entry keeps: those the policy and revalidation read.
var keptHeaders = []string{"Cache-Control", "Expires", "Date", "Last-Modified", "Etag", "Age", "Vary", "Content-Type"}

var disk atomic.Pointer[diskCache]

// UseDiskCache keeps downloaded images in an images directory under
// cacheDir, the app's cache directory, at most DiskCacheBytes; "" keeps
// them in memory only. Backends call it as they start.
func UseDiskCache(cacheDir string) {
	if cacheDir == "" {
		disk.Store(nil)
		return
	}
	disk.Store(newDiskCache(filepath.Join(cacheDir, "images"), DiskCacheBytes))
}

type diskCache struct {
	dir   string
	limit int64
	now   func() time.Time

	mu      sync.Mutex
	size    int64 // the bytes of the entries, -1 until counted
	scanned bool
}

func newDiskCache(dir string, limit int64) *diskCache {
	return &diskCache{dir: dir, limit: limit, now: time.Now, size: -1}
}

// cached is an entry read back from disk.
type cached struct {
	url    string
	stored time.Time // when the response was received
	header Header
	body   []byte
	path   string
	mtime  time.Time
}

func (c *diskCache) path(url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:16]))
}

// fetch returns the bytes at url, from the cache when a fresh copy is
// there, else from the network through the cache.
func (c *diskCache) fetch(ctx context.Context, url string, progress Progress) ([]byte, error) {
	e := c.read(url)
	now := c.now()
	if e != nil && e.fresh(now) {
		c.touch(e, now)
		return e.body, nil
	}
	header := Header{}
	if e != nil {
		if tag := e.header.Get("Etag"); tag != "" {
			header.Set("If-None-Match", tag)
		}
		if lm := e.header.Get("Last-Modified"); lm != "" {
			header.Set("If-Modified-Since", lm)
		}
	}
	resp, err := get(ctx, url, header, progress)
	if err != nil {
		if e != nil && e.servesStale() && ctx.Err() == nil {
			return e.body, nil
		}
		return nil, err
	}
	switch {
	case resp.Status == 304 && e != nil:
		for _, k := range keptHeaders {
			if v := resp.Header.Values(k); len(v) > 0 {
				e.header[k] = v
			}
		}
		e.stored = c.now()
		c.write(e)
		return e.body, nil
	case resp.Status == 200 || resp.Status == 203:
		body := resp.Body
		fresh := &cached{url: url, stored: c.now(), header: Header{}, body: body}
		for _, k := range keptHeaders {
			if v := resp.Header.Values(k); len(v) > 0 {
				fresh.header[k] = v
			}
		}
		if fresh.storable(c.limit) {
			c.write(fresh)
		} else if e != nil {
			c.remove(e.path)
		}
		return body, nil
	case resp.Status >= 500 && e != nil && e.servesStale():
		return e.body, nil
	case resp.Status >= 400 && resp.Status < 500 && e != nil:
		c.remove(e.path)
	}
	return nil, fmt.Errorf("imageload: %s: status %d", url, resp.Status)
}

// directives returns the Cache-Control directives of h, lower-cased, with
// their values.
func directives(h Header) map[string]string {
	d := map[string]string{}
	for _, line := range h.Values("Cache-Control") {
		for part := range strings.SplitSeq(line, ",") {
			k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
			if k != "" {
				d[strings.ToLower(k)] = strings.Trim(v, `"`)
			}
		}
	}
	return d
}

// storable reports whether e may be kept in a cache of limit bytes.
func (e *cached) storable(limit int64) bool {
	if _, ok := directives(e.header)["no-store"]; ok {
		return false
	}
	for _, v := range e.header.Values("Vary") {
		if strings.TrimSpace(v) == "*" {
			return false
		}
	}
	return int64(len(e.body)) <= limit/8
}

// lifetime is how long e is fresh for from when it was made.
func (e *cached) lifetime() time.Duration {
	d := directives(e.header)
	if _, ok := d["no-cache"]; ok {
		return 0
	}
	if v, ok := d["max-age"]; ok {
		if s, err := strconv.ParseInt(v, 10, 64); err == nil && s > 0 {
			return time.Duration(s) * time.Second
		}
		return 0
	}
	date, dateErr := parseTime(e.header.Get("Date"))
	if dateErr != nil {
		date = e.stored
	}
	if v := e.header.Get("Expires"); v != "" {
		expires, err := parseTime(v)
		if err != nil {
			return 0
		}
		return max(expires.Sub(date), 0)
	}
	if lm, err := parseTime(e.header.Get("Last-Modified")); err == nil && lm.Before(date) {
		return min(date.Sub(lm)/10, maxHeuristic)
	}
	return 0
}

// fresh reports whether e may be served at now without asking the server.
func (e *cached) fresh(now time.Time) bool {
	age := now.Sub(e.stored)
	if s, err := strconv.ParseInt(e.header.Get("Age"), 10, 64); err == nil && s > 0 {
		age += time.Duration(s) * time.Second
	}
	life := e.lifetime()
	return life > 0 && age < life
}

// servesStale reports whether e may be served once stale when the
// server cannot be reached or fails.
func (e *cached) servesStale() bool {
	d := directives(e.header)
	_, must := d["must-revalidate"]
	_, noCache := d["no-cache"]
	return !must && !noCache
}

// read returns the entry for url, or nil when there is none whole.
func (c *diskCache) read(url string) *cached {
	path := c.path(url)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	e, err := decodeEntry(data)
	if err != nil || e.url != url {
		c.remove(path)
		return nil
	}
	e.path = path
	if info, err := os.Stat(path); err == nil {
		e.mtime = info.ModTime()
	}
	return e
}

// touch marks e used at now, at most every touchEvery.
func (c *diskCache) touch(e *cached, now time.Time) {
	if now.Sub(e.mtime) >= touchEvery {
		os.Chtimes(e.path, now, now)
	}
}

func (c *diskCache) remove(path string) {
	if info, err := os.Stat(path); err == nil && os.Remove(path) == nil {
		c.mu.Lock()
		if c.size >= 0 {
			c.size -= info.Size()
		}
		c.mu.Unlock()
	}
}

// write keeps e, replacing its old entry whole.
func (c *diskCache) write(e *cached) {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return
	}
	path := c.path(e.url)
	data := encodeEntry(e)
	f, err := os.CreateTemp(c.dir, "tmp-")
	if err != nil {
		return
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return
	}
	var old int64
	if info, err := os.Stat(path); err == nil {
		old = info.Size()
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return
	}
	c.mu.Lock()
	if c.size < 0 {
		c.size = c.count()
	} else {
		c.size += int64(len(data)) - old
	}
	trim := c.size > c.limit || !c.scanned
	c.scanned = true
	c.mu.Unlock()
	if trim {
		go c.trim()
	}
}

// count returns the bytes of the entries on disk.
func (c *diskCache) count() int64 {
	var n int64
	entries, _ := os.ReadDir(c.dir)
	for _, d := range entries {
		if info, err := d.Info(); err == nil && !strings.HasPrefix(d.Name(), "tmp-") {
			n += info.Size()
		}
	}
	return n
}

// trim deletes the least recently used entries until the cache holds
// three quarters of its limit, and temporary files left behind.
func (c *diskCache) trim() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
		at   time.Time
	}
	var files []file
	var total int64
	now := c.now()
	for _, d := range entries {
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(c.dir, d.Name())
		if strings.HasPrefix(d.Name(), "tmp-") {
			if now.Sub(info.ModTime()) > staleTemp {
				os.Remove(path)
			}
			continue
		}
		files = append(files, file{path, info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total > c.limit {
		slices.SortFunc(files, func(a, b file) int { return a.at.Compare(b.at) })
		for _, f := range files {
			if total <= c.limit*3/4 {
				break
			}
			if os.Remove(f.path) == nil {
				total -= f.size
			}
		}
	}
	c.mu.Lock()
	c.size = total
	c.mu.Unlock()
}

// An entry is entryMagic, the CRC32 of what follows, then the URL, the
// time the response came in, its kept headers as MIME header lines, a
// blank line and the body.
func encodeEntry(e *cached) []byte {
	var b bytes.Buffer
	b.WriteString(e.url)
	b.WriteByte('\n')
	b.WriteString(strconv.FormatInt(e.stored.UnixNano(), 10))
	b.WriteByte('\n')
	for _, k := range keptHeaders {
		for _, v := range e.header.Values(k) {
			b.WriteString(k + ": " + strings.NewReplacer("\r", " ", "\n", " ").Replace(v) + "\r\n")
		}
	}
	b.WriteString("\r\n")
	b.Write(e.body)
	out := make([]byte, 0, len(entryMagic)+4+b.Len())
	out = append(out, entryMagic...)
	out = binary.LittleEndian.AppendUint32(out, crc32.ChecksumIEEE(b.Bytes()))
	return append(out, b.Bytes()...)
}

var errEntry = errors.New("imageload: a broken cache entry")

func decodeEntry(data []byte) (*cached, error) {
	head := len(entryMagic) + 4
	if len(data) < head || string(data[:len(entryMagic)]) != entryMagic {
		return nil, errEntry
	}
	rest := data[head:]
	if crc32.ChecksumIEEE(rest) != binary.LittleEndian.Uint32(data[len(entryMagic):]) {
		return nil, errEntry
	}
	r := bufio.NewReader(bytes.NewReader(rest))
	url, err := r.ReadString('\n')
	if err != nil {
		return nil, errEntry
	}
	at, err := r.ReadString('\n')
	if err != nil {
		return nil, errEntry
	}
	nanos, err := strconv.ParseInt(strings.TrimSpace(at), 10, 64)
	if err != nil {
		return nil, errEntry
	}
	header := Header{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, errEntry
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, errEntry
		}
		header.Add(k, strings.TrimSpace(v))
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, errEntry
	}
	return &cached{url: strings.TrimSuffix(url, "\n"), stored: time.Unix(0, nanos), header: header, body: body}, nil
}
